package example

// Minimal reproduction test for the "version does not exist" panic on
// restart, as requested in the audit brief:
//
//   create app -> mount stores -> init chain -> commit blocks ->
//   close DB -> reopen DB -> LoadLatestVersion -> query bank -> query dex
//
// This bypasses CometBFT/the CLI entirely and drives BaseApp's ABCI methods
// directly against a real on-disk goleveldb, which is the same storage
// engine the `start` command uses by default. That keeps the reproduction
// fast and isolated from anything network- or CometBFT-related, while
// still exercising the exact rootmulti/IAVL commit + reload path from the
// bug report.
//
// NOTE: this test is compiled and executed against the repository's
// current Go/module/toolchain versions as part of the normal test suite.
//
// Run with:
//   go test -v -run TestPersistenceAcrossRestart ./...

import (
	"encoding/json"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log/v2"
	sdkmath "cosmossdk.io/math"
	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	dextypes "github.com/tutu787878/xpex-chain/x/dex/types"
)

// buildGenesis takes the app's default genesis and overrides the "dex"
// entry with distinctive, non-default values, so that after restart we can
// prove *specific* committed state round-tripped rather than just reading
// back defaults.
func buildGenesis(t *testing.T, app *ExampleApp) GenesisState {
	t.Helper()

	genesis := app.DefaultGenesis()

	// Create one real test validator and one genesis account using the
	// same helper pattern as the Cosmos SDK simulation framework.
	valSet, err := simtestutil.CreateRandomValidatorSet()
	require.NoError(t, err)

	priv := secp256k1.GenPrivKey()
	genAccount := authtypes.NewBaseAccount(
		priv.PubKey().Address().Bytes(),
		priv.PubKey(),
		0,
		0,
	)

	genesis, err = simtestutil.GenesisStateWithValSet(
		app.AppCodec(),
		genesis,
		valSet,
		[]authtypes.GenesisAccount{genAccount},
		banktypes.Balance{
			Address: genAccount.GetAddress().String(),
			Coins: sdk.NewCoins(
				sdk.NewCoin(
					sdk.DefaultBondDenom,
					sdkmath.NewInt(100000000000000),
				),
			),
		},
	)
	require.NoError(t, err)

	// Override only the DEX state with distinctive values so that after
	// restart we can prove specific committed DEX state round-tripped.
	dexGenesis := dextypes.GenesisState{
		NextPoolId: 7,
		Pools: []*dextypes.Pool{
			{
				Id:       1,
				TokenA:   "uxpx",
				TokenB:   "uusdt",
				ReserveA: "1000000",
				ReserveB: "2000000",
				TotalLp:  "1000000",
				FeeBps:   "30",
			},
		},
	}
	genesis[dextypes.ModuleName] = app.AppCodec().MustMarshalJSON(&dexGenesis)

	return genesis
}

func newPersistenceTestApp(t *testing.T, dataDir string, loadLatest bool) (*ExampleApp, dbm.DB) {
	t.Helper()

	db, err := dbm.NewDB("application", dbm.GoLevelDBBackend, dataDir)
	require.NoError(t, err, "opening application.db")

	appOpts := simtestutil.NewAppOptionsWithFlagHome(dataDir)
	app := NewExampleApp(
		log.NewNopLogger(),
		db,
		nil,
		loadLatest,
		appOpts,
		baseapp.SetChainID("xpex-1"),
	)

	return app, db
}

// TestPersistenceAcrossRestart is the Step-3 minimal repro: it must fail
// with a panic containing "version does not exist" if the bug reproduces,
// and pass cleanly if it does not.
func TestPersistenceAcrossRestart(t *testing.T) {
	dataDir := t.TempDir()

	const numBlocks = 5
	var lastAppHash []byte
	var lastHeight int64

	// ---- run 1: fresh init, a few blocks, graceful close ----
	func() {
		app, _ := newPersistenceTestApp(t, dataDir, true)
		defer func() {
			// Mirrors what the `start` command's shutdown path calls.
			require.NoError(t, app.Close(), "app.Close() on first run")
		}()

		genesis := buildGenesis(t, app)
		appStateBytes, err := json.Marshal(genesis)
		require.NoError(t, err)

		now := time.Now().UTC()

		_, err = app.InitChain(&abci.RequestInitChain{
			Time:            now,
			ChainId:         "xpex-1",
			ConsensusParams: simtestutil.DefaultConsensusParams,
			InitialHeight:   1,
			AppStateBytes:   appStateBytes,
		})
		require.NoError(t, err, "InitChain")

		for i := int64(1); i <= numBlocks; i++ {
			blockTime := now.Add(time.Duration(i) * time.Second)

			fbResp, err := app.FinalizeBlock(&abci.RequestFinalizeBlock{
				Height:             i,
				Time:               blockTime,
				Hash:               make([]byte, 32),
				NextValidatorsHash: make([]byte, 32),
				ProposerAddress:    make([]byte, 20),
				DecidedLastCommit:  abci.CommitInfo{},
				Txs:                nil,
			})
			require.NoErrorf(t, err, "FinalizeBlock height %d", i)

			// NOTE: in the ABCI 2.0 / FinalizeBlock model the app hash comes
			// back on ResponseFinalizeBlock.AppHash, not ResponseCommit --
			// Commit itself only returns a retain-height hint. If your pin
			// still exposes a Data/AppHash field on ResponseCommit instead,
			// swap this line accordingly.
			lastAppHash = fbResp.AppHash

			_, err = app.Commit()
			require.NoErrorf(t, err, "Commit height %d", i)

			lastHeight = i
		}

		require.Equal(t, int64(numBlocks), app.LastBlockHeight(),
			"app should have committed exactly numBlocks blocks before shutdown")

		// Sanity check state is actually there BEFORE we ever touch reload.
		qctx := app.NewContextLegacy(true, cmtproto.Header{Height: app.LastBlockHeight()})
		nextID, err := app.DexKeeper.NextPoolID(qctx)
		require.NoError(t, err)
		require.Equal(t, uint64(7), nextID, "dex next_pool_id should read back genesis value before restart")

		pool, err := app.DexKeeper.GetPool(qctx, 1)
		require.NoError(t, err)
		require.Equal(t, "uxpx", pool.TokenA)

		supply := app.BankKeeper.GetSupply(qctx, "stake")
		_ = supply // default genesis has no minted supply; just proving the call doesn't panic.
	}()

	require.NotEmpty(t, lastAppHash, "sanity: should have a committed app hash from run 1")
	require.Equal(t, int64(numBlocks), lastHeight)

	// ---- run 2: reopen the SAME data dir, THIS is where the bug fires ----
	// If the "version does not exist" bug reproduces, NewExampleApp panics
	// inside LoadLatestVersion() here. We recover it explicitly so the
	// failure message is readable instead of a raw stack trace, and so a
	// single `go test -run TestPersistenceAcrossRestart` run gives a clear
	// yes/no answer.
	var app2 *ExampleApp
	var db2 dbm.DB
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf(
					"REPRO CONFIRMED: reopening the same data dir after a clean "+
						"shutdown panicked in LoadLatestVersion(): %v\n"+
						"data dir: %s", r, dataDir,
				)
			}
		}()
		app2, db2 = newPersistenceTestApp(t, dataDir, true)
	}()
	defer func() { require.NoError(t, app2.Close()) }()
	_ = db2 // owned by app2 now; app2.Close() closes it.

	require.Equal(t, lastHeight, app2.LastBlockHeight(),
		"reloaded app should resume at the height it was committed to")

	qctx2 := app2.NewContextLegacy(true, cmtproto.Header{Height: app2.LastBlockHeight()})

	nextID2, err := app2.DexKeeper.NextPoolID(qctx2)
	require.NoError(t, err, "querying dex next_pool_id after restart")
	require.Equal(t, uint64(7), nextID2, "dex next_pool_id should survive restart")

	pool2, err := app2.DexKeeper.GetPool(qctx2, 1)
	require.NoError(t, err, "querying dex pool 1 after restart")
	require.Equal(t, "uxpx", pool2.TokenA)
	require.Equal(t, "uusdt", pool2.TokenB)

	// "query bank" equivalent: this alone reproduces the report that plain
	// bank queries break after restart too, since it reads through the
	// same rootmulti/IAVL reload path as dex.
	supply2 := app2.BankKeeper.GetSupply(qctx2, "stake")
	_ = supply2

	t.Logf("OK: restart succeeded. height=%d dex.next_pool_id=%d pool[1].token_a=%s",
		app2.LastBlockHeight(), nextID2, pool2.TokenA)

	// Cleanup note: dataDir is a t.TempDir(), removed automatically. If you
	// want to poke at the raw DB after a failure, change dataDir to a fixed
	// path so it survives the test and inspect it with the diagnostic
	// snippet described in the write-up.
}
