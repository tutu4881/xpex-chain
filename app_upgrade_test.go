package example

// TestUpgradeAddsDexStoreToExistingChain reproduces, end-to-end, the exact
// production problem described in the audit brief: a chain whose existing
// block history predates x/dex and x/upgrade needs the `dex` store added
// via a proper Cosmos SDK store upgrade, without losing any pre-existing
// chain state -- and proves the fix in app.go resolves it.
//
// Phase 1 fabricates that pre-existing chain history directly against
// cosmossdk.io/store/rootmulti, mounting only the store keys app.go mounted
// *before* this fix (i.e. everything except "upgrade" and "dex"). This
// stands in for a real node's data directory produced by a binary that had
// never heard of dex or x/upgrade -- exactly the situation described in the
// brief ("приложение уже содержит существующую историю блоков").
//
// Phase 2a proves the failure mode this fix addresses still exists for
// anyone who skips the documented bootstrap step: booting today's
// example.NewExampleApp (which always mounts "upgrade" and "dex" now)
// against that same data, with no upgrade-info.json bootstrap file
// present, panics with the exact "new stores should be added using
// StoreUpgrades" error from the bug report. This is the negative control --
// it shows adding the store keys to app.go alone does not fix anything by
// itself, the documented bootstrap step is load-bearing.
//
// Phase 2b proves the fix: writing the documented upgrade-info.json
// bootstrap file (name=AddDexStoreUpgradeName, height=lastHeight+1) before
// booting lets the SAME binary come up cleanly. Driving one more block
// through FinalizeBlock+Commit at that exact height (mirroring what
// CometBFT does the moment the swapped-in binary comes back online) then
// shows: the dex store exists and was initialized via its own
// DefaultGenesis, AND the pre-existing (pre-dex) chain state from phase 1
// is completely intact -- the check that would fail if the fromVM
// back-fill in the upgrade handler were missing or wrong.
//
// Run with:
//
//	go test -v -run TestUpgradeAddsDexStoreToExistingChain ./...
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cosmossdk.io/log/v2"
	"cosmossdk.io/store/rootmulti"
	storetypes "cosmossdk.io/store/types"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"

	countertypes "github.com/tutu787878/xpex-chain/x/counter/types"
)

func TestUpgradeAddsDexStoreToExistingChain(t *testing.T) {
	dataDir := t.TempDir()

	// ------------------------------------------------------------------
	// Phase 1: fabricate a pre-dex, pre-upgrade chain history.
	//
	// This mounts exactly the store keys app.go used to mount before this
	// fix -- everything the chain already has, minus "upgrade" and "dex" --
	// commits a handful of blocks, and writes one marker value into an
	// already-existing module's store (counter) standing in for real
	// historical account/staking/etc. data. This is the same low-level
	// cosmossdk.io/store/rootmulti API tools/storediag already uses to
	// inspect this exact class of problem.
	// ------------------------------------------------------------------
	oldKeys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey,
		banktypes.StoreKey,
		stakingtypes.StoreKey,
		distrtypes.StoreKey,
		slashingtypes.StoreKey,
		govtypes.StoreKey,
		consensusparamtypes.StoreKey,
		countertypes.StoreKey,
	)

	db, err := dbm.NewDB("application", dbm.GoLevelDBBackend, dataDir)
	require.NoError(t, err, "opening application.db for the fabricated pre-upgrade chain")

	rs := rootmulti.NewStore(db, log.NewNopLogger(), nil)
	for _, key := range oldKeys {
		rs.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	}
	require.NoError(t, rs.LoadLatestVersion())

	counterStore := rs.GetKVStore(oldKeys[countertypes.StoreKey])
	counterStore.Set([]byte("marker"), []byte("pre-upgrade-value"))

	const numOldBlocks = 5
	var lastHeight int64
	for i := 0; i < numOldBlocks; i++ {
		cid := rs.Commit()
		lastHeight = cid.Version
	}
	require.Equal(t, int64(numOldBlocks), lastHeight, "sanity: fabricated chain history height")

	// Close cleanly -- this is exactly the "old binary was cleanly shut
	// down" state a real operator would hand off to the new binary.
	require.NoError(t, db.Close(), "closing the fabricated pre-upgrade data dir")

	upgradeHeight := lastHeight + 1

	// ------------------------------------------------------------------
	// Phase 2a (negative control): boot today's binary with NO
	// upgrade-info.json bootstrap present. Must panic with the exact
	// store-version-mismatch error from the bug report.
	// ------------------------------------------------------------------
	func() {
		db2a, err := dbm.NewDB("application", dbm.GoLevelDBBackend, dataDir)
		require.NoError(t, err, "re-opening the fabricated data dir (phase 2a)")
		defer func() { require.NoError(t, db2a.Close()) }()

		defer func() {
			r := recover()
			require.NotNil(t, r, "expected NewExampleApp to panic without the upgrade-info.json bootstrap")
			msg := fmt.Sprintf("%v", r)
			require.Contains(t, msg, "StoreUpgrades",
				"panic should be the store-version-mismatch error this fix addresses, got: %s", msg)
		}()

		// A separate, empty home dir: ReadUpgradeInfoFromDisk() finds
		// nothing here, so app.go never calls SetStoreLoader, and the
		// default loader hits the fabricated (pre-dex) data untouched --
		// this isolates "was the bootstrap file written" as the only
		// variable under test.
		emptyHome := t.TempDir()
		appOpts := simtestutil.NewAppOptionsWithFlagHome(emptyHome)
		_ = NewExampleApp(log.NewNopLogger(), db2a, nil, true, appOpts, baseapp.SetChainID("xpex-1"))
	}()

	// ------------------------------------------------------------------
	// Phase 2b (the fix): write the documented upgrade-info.json bootstrap
	// file, then boot the SAME binary against the SAME fabricated data.
	// ------------------------------------------------------------------
	dataSubDir := filepath.Join(dataDir, "data")
	require.NoError(t, os.MkdirAll(dataSubDir, 0o755))

	planBytes, err := json.Marshal(upgradetypes.Plan{
		Name:   AddDexStoreUpgradeName,
		Height: upgradeHeight,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataSubDir, upgradetypes.UpgradeInfoFilename), planBytes, 0o600),
		"writing the coordinated upgrade-info.json bootstrap file")

	db2b, err := dbm.NewDB("application", dbm.GoLevelDBBackend, dataDir)
	require.NoError(t, err, "re-opening the fabricated data dir (phase 2b)")

	appOpts := simtestutil.NewAppOptionsWithFlagHome(dataDir)
	app := NewExampleApp(log.NewNopLogger(), db2b, nil, true, appOpts, baseapp.SetChainID("xpex-1"))
	defer func() { require.NoError(t, app.Close()) }()

	require.Equal(t, lastHeight, app.LastBlockHeight(),
		"booting with the store-upgrade loader must not itself commit a block")

	// Drive the one block at the exact coordinated upgrade height -- this
	// is what actually triggers PreBlocker's self-scheduled ScheduleUpgrade
	// -> x/upgrade's ApplyUpgrade -> our RunMigrations handler.
	now := time.Now().UTC()
	fbResp, err := app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height:             upgradeHeight,
		Time:               now,
		Hash:               make([]byte, 32),
		NextValidatorsHash: make([]byte, 32),
		ProposerAddress:    make([]byte, 20),
		DecidedLastCommit:  abci.CommitInfo{},
		Txs:                nil,
	})
	require.NoError(t, err, "FinalizeBlock at the upgrade height")
	require.NotEmpty(t, fbResp.AppHash, "sanity: block at the upgrade height should produce a real app hash")

	_, err = app.Commit()
	require.NoError(t, err, "Commit at the upgrade height")
	require.Equal(t, upgradeHeight, app.LastBlockHeight())

	qctx := app.NewContextLegacy(true, cmtproto.Header{Height: app.LastBlockHeight()})

	// dex must now exist and be usable, initialized via its own
	// DefaultGenesis (NextPoolId starts at 1, no pools) through the
	// upgrade handler's RunMigrations call -- NOT through InitChain, which
	// never ran against this data directory.
	nextID, err := app.DexKeeper.NextPoolID(qctx)
	require.NoError(t, err, "dex store should exist and be queryable after the upgrade")
	require.Equal(t, uint64(1), nextID,
		"dex should have been initialized via its own DefaultGenesis, not left empty or wiped")

	// Pre-existing (pre-dex) chain state must be completely intact. This
	// is the check that would fail if the fromVM back-fill in the upgrade
	// handler were missing or wrong: RunMigrations would otherwise have
	// treated every already-existing module, including counter, as "new"
	// and reset it via InitGenesis with default (empty) state.
	gotCounterStore := app.CommitMultiStore().GetKVStore(app.GetKey(countertypes.StoreKey))
	require.Equal(t, []byte("pre-upgrade-value"), gotCounterStore.Get([]byte("marker")),
		"pre-existing chain state must survive the store upgrade untouched")

	t.Logf("OK: dex store added at height %d; dex.next_pool_id=%d; pre-existing counter state intact",
		upgradeHeight, nextID)
}
