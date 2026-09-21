package example

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	"cosmossdk.io/client/v2/autocli"
	"cosmossdk.io/client/v2/helpers"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/log/v2"
	storetypes "cosmossdk.io/store/types"
	abci "github.com/cometbft/cometbft/abci/types"
	db "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	nodeservice "github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	runtimeservices "github.com/cosmos/cosmos-sdk/runtime/services"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensus "github.com/cosmos/cosmos-sdk/x/consensus"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/cosmos-sdk/x/tx/signing"
	upgrade "github.com/cosmos/cosmos-sdk/x/upgrade"
	upgradekeeper "github.com/cosmos/cosmos-sdk/x/upgrade/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/spf13/cast"

	// counter tutorial app wiring 1: add counter imports below
	counter "github.com/tutu787878/xpex-chain/x/counter"
	counterkeeper "github.com/tutu787878/xpex-chain/x/counter/keeper"
	countertypes "github.com/tutu787878/xpex-chain/x/counter/types"
	dex "github.com/tutu787878/xpex-chain/x/dex"
	dexkeeper "github.com/tutu787878/xpex-chain/x/dex/keeper"
	dextypes "github.com/tutu787878/xpex-chain/x/dex/types"
)

const (
	appName = "exampleApp"

	// AddDexStoreUpgradeName is the governance-scheduled x/upgrade Plan name
	// that adds the `dex` KVStore to a chain whose existing block history
	// predates x/dex. It must match exactly what is submitted on-chain via
	// MsgSoftwareUpgrade (or written by hand to upgrade-info.json for the
	// one-time bootstrap described in the deployment runbook, since x/upgrade
	// itself did not exist on this chain before this release). Exported so
	// both app.go and its tests reference the same literal.
	AddDexStoreUpgradeName = "v2-add-dex-store"
)

var (
	DefaultNodeHome string

	_ runtime.AppI            = (*ExampleApp)(nil)
	_ servertypes.Application = (*ExampleApp)(nil)

	// module account permissions
	maccPerms = map[string][]string{
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		countertypes.ModuleName:        nil,
		dextypes.ModuleName:            nil,
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:            {authtypes.Burner},
	}
)

func init() {
	var err error
	DefaultNodeHome, err = helpers.GetNodeHomeDirectory(".exampleapp")
	if err != nil {
		panic(err)
	}
}

type ExampleApp struct {
	*baseapp.BaseApp
	legacyAmino       *codec.LegacyAmino
	appCodec          codec.Codec
	txConfig          client.TxConfig
	interfaceRegistry codectypes.InterfaceRegistry

	keys map[string]*storetypes.KVStoreKey

	// Keepers
	AccountKeeper         authkeeper.AccountKeeper
	BankKeeper            bankkeeper.Keeper
	DistrKeeper           distrkeeper.Keeper
	GovKeeper             govkeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	SlashingKeeper        slashingkeeper.Keeper
	ConsensusParamsKeeper consensusparamkeeper.Keeper
	// counter tutorial app wiring 2: add the counter keeper field below
	CounterKeeper *counterkeeper.Keeper
	DexKeeper     *dexkeeper.Keeper
	UpgradeKeeper *upgradekeeper.Keeper

	// the module manager
	ModuleManager      *module.Manager
	BasicModuleManager module.BasicManager

	// simulation manager
	sm *module.SimulationManager

	// module configurator
	configurator module.Configurator

	// addDexStoreUpgradeInfo is what ReadUpgradeInfoFromDisk() returned at
	// boot time, captured so PreBlocker (below) can self-schedule the
	// AddDexStoreUpgradeName plan on-chain at exactly that height. See the
	// comment on PreBlocker for why this bootstrap step is necessary.
	addDexStoreUpgradeInfo upgradetypes.Plan
}

func NewExampleApp(
	logger log.Logger,
	db db.DB,
	traceStore io.Writer,
	loadLatest bool,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) *ExampleApp {
	interfaceRegistry, _ := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: signing.Options{
			AddressCodec: address.Bech32Codec{
				Bech32Prefix: sdk.GetConfig().GetBech32AccountAddrPrefix(),
			},
			ValidatorAddressCodec: address.Bech32Codec{
				Bech32Prefix: sdk.GetConfig().GetBech32ValidatorAddrPrefix(),
			},
		},
	})
	appCodec := codec.NewProtoCodec(interfaceRegistry)
	legacyAmino := codec.NewLegacyAmino()
	txConfig := authtx.NewTxConfig(appCodec, authtx.DefaultSignModes)
	if err := interfaceRegistry.SigningContext().Validate(); err != nil {
		panic(err)
	}

	std.RegisterLegacyAminoCodec(legacyAmino)
	std.RegisterInterfaces(interfaceRegistry)
	baseAppOptions = append(baseAppOptions, baseapp.SetOptimisticExecution())

	bApp := baseapp.NewBaseApp(appName, logger, db, txConfig.TxDecoder(), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)
	bApp.SetTxEncoder(txConfig.TxEncoder())

	keys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey,
		banktypes.StoreKey,
		stakingtypes.StoreKey,
		distrtypes.StoreKey,
		slashingtypes.StoreKey,
		govtypes.StoreKey,
		consensusparamtypes.StoreKey,
		upgradetypes.StoreKey,
		// counter tutorial app wiring 3: add the counter store key below
		countertypes.StoreKey,
		dextypes.StoreKey,
	)

	if err := bApp.RegisterStreamingServices(appOpts, keys); err != nil {
		panic(err)
	}

	app := &ExampleApp{
		BaseApp:           bApp,
		legacyAmino:       legacyAmino,
		appCodec:          appCodec,
		txConfig:          txConfig,
		interfaceRegistry: interfaceRegistry,
		keys:              keys,
	}

	// set the BaseApp's parameter store
	app.ConsensusParamsKeeper = consensusparamkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[consensusparamtypes.StoreKey]),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		runtime.EventService{},
	)

	// UpgradeKeeper must exist before LoadLatestVersion() is called (further
	// below) so that its store-upgrade wiring can intercept how the store is
	// loaded. skipUpgradeHeights/homePath come from the same appOpts the
	// `start`/`export` commands already populate via cosmos-sdk's standard
	// server.AddCommandsWithStartCmdOptions flags (--unsafe-skip-upgrades,
	// --home); nothing else in the CLI needs to change for this to work.
	skipUpgradeHeights := make(map[int64]bool)
	for _, h := range cast.ToIntSlice(appOpts.Get(server.FlagUnsafeSkipUpgrades)) {
		skipUpgradeHeights[int64(h)] = true
	}
	homePath := cast.ToString(appOpts.Get(flags.FlagHome))

	app.UpgradeKeeper = upgradekeeper.NewKeeper(
		skipUpgradeHeights,
		runtime.NewKVStoreService(keys[upgradetypes.StoreKey]),
		appCodec,
		homePath,
		app.BaseApp,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	bApp.SetParamStore(app.ConsensusParamsKeeper.ParamsStore)

	app.AccountKeeper = authkeeper.NewAccountKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[authtypes.StoreKey]),
		authtypes.ProtoBaseAccount,
		maccPerms,
		authcodec.NewBech32Codec(sdk.Bech32MainPrefix),
		sdk.Bech32MainPrefix,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		authkeeper.WithUnorderedTransactions(true),
	)

	app.BankKeeper = bankkeeper.NewBaseKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[banktypes.StoreKey]),
		app.AccountKeeper,
		BlockedAddresses(),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		logger,
	)

	app.StakingKeeper = stakingkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[stakingtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		authcodec.NewBech32Codec(sdk.Bech32PrefixValAddr),
		authcodec.NewBech32Codec(sdk.Bech32PrefixConsAddr),
	)
	app.DistrKeeper = distrkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[distrtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		authtypes.FeeCollectorName,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	app.SlashingKeeper = slashingkeeper.NewKeeper(
		appCodec,
		legacyAmino,
		runtime.NewKVStoreService(keys[slashingtypes.StoreKey]),
		app.StakingKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	app.StakingKeeper.SetHooks(
		stakingtypes.NewMultiStakingHooks(
			app.DistrKeeper.Hooks(),
			app.SlashingKeeper.Hooks(),
		),
	)
	// Register the proposal types
	// Deprecated: Avoid adding new handlers, instead use the new proposal flow
	// by granting the governance module the right to execute the message.
	// See: https://docs.cosmos.network/main/modules/gov#proposal-messages
	govRouter := govv1beta1.NewRouter()
	govRouter.AddRoute(govtypes.RouterKey, govv1beta1.ProposalHandler)
	govConfig := govtypes.DefaultConfig()
	/*
		Example of setting gov params:
		govConfig.MaxMetadataLen = 10000
	*/
	govKeeper := govkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[govtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		app.DistrKeeper,
		app.MsgServiceRouter(),
		govConfig,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		govkeeper.NewDefaultCalculateVoteResultsAndVotingPower(app.StakingKeeper),
	)

	// Set legacy router for backwards compatibility with gov v1beta1
	govKeeper.SetLegacyRouter(govRouter)

	app.GovKeeper = *govKeeper.SetHooks(
		govtypes.NewMultiGovHooks(
		// register the governance hooks
		),
	)

	// counter tutorial app wiring 4: create the counter keeper below
	app.CounterKeeper = counterkeeper.NewKeeper(runtime.NewKVStoreService(keys[countertypes.StoreKey]), appCodec, app.BankKeeper)

	app.DexKeeper = dexkeeper.NewKeeper(
		runtime.NewKVStoreService(keys[dextypes.StoreKey]),
		appCodec,
		app.BankKeeper,
	)

	app.ModuleManager = module.NewManager(
		genutil.NewAppModule(
			app.AccountKeeper, app.StakingKeeper, app,
			txConfig,
		),
		auth.NewAppModule(appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, nil),
		bank.NewAppModule(appCodec, app.BankKeeper, app.AccountKeeper, nil),
		gov.NewAppModule(appCodec, &app.GovKeeper, app.AccountKeeper, app.BankKeeper, nil),
		consensus.NewAppModule(appCodec, app.ConsensusParamsKeeper),
		slashing.NewAppModule(appCodec, app.SlashingKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, nil, app.interfaceRegistry),
		distr.NewAppModule(appCodec, app.DistrKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, nil),
		staking.NewAppModule(appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, nil),
		vesting.NewAppModule(app.AccountKeeper, app.BankKeeper),
		// counter tutorial app wiring 5: register the counter module below
		counter.NewAppModule(appCodec, app.CounterKeeper),
		dex.NewAppModule(appCodec, app.DexKeeper),
		upgrade.NewAppModule(app.UpgradeKeeper, authcodec.NewBech32Codec(sdk.Bech32MainPrefix)),
	)

	// Record the module manager's own version map as the "initial" version
	// map x/upgrade's InitGenesis will persist for chains that start fresh
	// from genesis with x/upgrade already wired in. This is what lets a
	// *future* new module be added the same way dex is being added now,
	// without needing the fromVM back-fill done in the upgrade handler
	// below (that back-fill exists only because x/upgrade did not exist on
	// this chain prior to this upgrade).
	app.UpgradeKeeper.SetInitVersionMap(app.ModuleManager.GetVersionMap())

	// BasicModuleManager defines the module BasicManager is in charge of setting up basic,
	// non-dependent module elements, such as codec registration and genesis verification.
	// By default it is composed of all the module from the module manager.
	// Additionally, app module basics can be overwritten by passing them as argument.
	app.BasicModuleManager = module.NewBasicManagerFromManager(
		app.ModuleManager,
		map[string]module.AppModuleBasic{
			genutiltypes.ModuleName: genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
			dextypes.ModuleName:     dex.NewAppModule(appCodec, app.DexKeeper).AppModuleBasic,
			govtypes.ModuleName: gov.NewAppModuleBasic(
				[]govclient.ProposalHandler{},
			),
		},
	)

	app.BasicModuleManager.RegisterLegacyAminoCodec(legacyAmino)
	app.BasicModuleManager.RegisterInterfaces(interfaceRegistry)

	// create the simulation manager and define the order of the modules for deterministic simulations
	overrideModules := map[string]module.AppModuleSimulation{
		authtypes.ModuleName: auth.NewAppModule(appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, nil),
	}
	app.sm = module.NewSimulationManagerFromAppModules(app.ModuleManager.Modules, overrideModules)
	app.sm.RegisterStoreDecoders()

	// upgrade MUST be first: its PreBlock hook is what applies a scheduled
	// upgrade (including any pending store migration) before any other
	// module's BeginBlock/PreBlock logic touches state for this height.
	app.ModuleManager.SetOrderPreBlockers(
		upgradetypes.ModuleName,
		authtypes.ModuleName,
	)
	app.ModuleManager.SetOrderBeginBlockers(
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		stakingtypes.ModuleName,
		// counter tutorial app wiring 6: add the counter module to genesis order below
		countertypes.ModuleName,
		dextypes.ModuleName,
		genutiltypes.ModuleName,
	)
	app.ModuleManager.SetOrderEndBlockers(
		banktypes.ModuleName,
		govtypes.ModuleName,
		stakingtypes.ModuleName,
		// counter tutorial app wiring 7: add the counter module to export order below
		countertypes.ModuleName,
		dextypes.ModuleName,
		genutiltypes.ModuleName,
	)
	// NOTE: The genutils module must occur after staking so that pools are
	// properly initialized with tokens from genesis accounts.
	// NOTE: The genutils module must also occur after auth so that it can access the params from auth.
	genesisModuleOrder := []string{
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		consensusparamtypes.ModuleName,
		upgradetypes.ModuleName,
		vestingtypes.ModuleName,
		countertypes.ModuleName,
		dextypes.ModuleName,
		genutiltypes.ModuleName,
	}

	exportModuleOrder := []string{
		consensusparamtypes.ModuleName,
		upgradetypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		vestingtypes.ModuleName,
		countertypes.ModuleName,
		dextypes.ModuleName,
		genutiltypes.ModuleName,
	}

	app.ModuleManager.SetOrderInitGenesis(genesisModuleOrder...)
	app.ModuleManager.SetOrderExportGenesis(exportModuleOrder...)

	app.configurator = module.NewConfigurator(app.appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	err := app.ModuleManager.RegisterServices(app.configurator)
	if err != nil {
		panic(err)
	}
	autocliv1.RegisterQueryServer(app.GRPCQueryRouter(), runtimeservices.NewAutoCLIQueryService(app.ModuleManager.Modules))

	// ------------------------------------------------------------------
	// Store-upgrade wiring for adding the `dex` (and `upgrade`) stores to
	// an existing chain.
	//
	// Root cause this fixes: `dex` is a KVStoreKey that did not exist when
	// earlier blocks of this chain were committed, so it has no entry in
	// any historical CommitInfo -- and neither does `upgrade`, since this
	// release is also the first one to wire x/upgrade in at all. cosmossdk.io/
	// store/rootmulti's loadVersion only tolerates a store whose on-disk
	// version doesn't match the root store's committed height when that
	// store is explicitly named in a *StoreUpgrades{Added: [...]} passed to
	// LoadLatestVersionAndUpgrade. A plain app.LoadLatestVersion() (what
	// this app called before this fix) always passes a nil StoreUpgrades,
	// so on any data directory that predates `dex` it fails with:
	//
	//   version of store dex mismatch root store's version; expected N got 0;
	//   new stores should be added using StoreUpgrades
	//
	// The supported fix is x/upgrade's UpgradeStoreLoader: an
	// app.SetStoreLoader override that is a no-op on every block except the
	// one immediately following a named, scheduled upgrade height, where it
	// runs LoadLatestVersionAndUpgrade with the given StoreUpgrades exactly
	// once. This must be set up before LoadLatestVersion() runs below.
	//
	// Getting the physical stores mounted is only half of it. The handler
	// registered below (which actually initializes dex's genesis state and
	// records the module version map) is invoked by x/upgrade's own
	// PreBlocker, which only fires it once it finds a matching Plan already
	// scheduled *on-chain*, in x/upgrade's own store. On a chain that
	// already had x/upgrade, that Plan is put there ahead of time by a
	// passed MsgSoftwareUpgrade governance proposal. That path does not
	// exist here: nothing could have scheduled anything in x/upgrade's
	// store before this block, because that store itself did not exist
	// before this block. See PreBlocker below for how this is bridged.
	// ------------------------------------------------------------------

	app.UpgradeKeeper.SetUpgradeHandler(
		AddDexStoreUpgradeName,
		func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			// fromVM is read from x/upgrade's own on-chain VersionMap store.
			// Because x/upgrade is being wired into this chain for the
			// first time in this very release (it did not exist in any
			// earlier binary), that store starts out completely empty --
			// fromVM has NO entries at all, not just a missing "dex" entry.
			//
			// Handing an empty fromVM straight to RunMigrations would make
			// it treat every already-existing module (auth, bank, staking,
			// distribution, slashing, gov, consensus, counter, and upgrade
			// itself) as brand new too, and call each one's InitGenesis
			// with its DEFAULT (empty) genesis -- which would wipe real
			// account balances, delegations and every other bit of
			// existing chain state.
			//
			// So: back-fill fromVM with the CURRENT ConsensusVersion of
			// every module that already exists on this chain (upgrade
			// included -- it has no InitGenesis work of its own to do;
			// ApplyUpgrade sets its version-map state explicitly right
			// after this handler returns), and deliberately leave out only
			// `dex`, which really is new. RunMigrations then runs an
			// ordinary version-diff migration (a no-op, since nothing
			// actually changed) for every existing module, and calls
			// InitGenesis with DefaultGenesis only for dex -- which is
			// exactly the "add a new module's store to a running chain"
			// pattern documented for module.Manager.RunMigrations.
			for name, mod := range app.ModuleManager.Modules {
				if name == dextypes.ModuleName {
					continue
				}
				if _, exists := fromVM[name]; exists {
					continue
				}
				if hasCV, ok := mod.(module.HasConsensusVersion); ok {
					fromVM[name] = hasCV.ConsensusVersion()
				}
			}

			return app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		},
	)

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Errorf("failed to read upgrade info from disk: %w", err))
	}
	app.addDexStoreUpgradeInfo = upgradeInfo

	if upgradeInfo.Name == AddDexStoreUpgradeName && !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		storeUpgrades := storetypes.StoreUpgrades{
			// Both stores are new to this chain's history: `upgrade`'s
			// store is created by this very release, same as `dex`'s.
			Added: []string{upgradetypes.StoreKey, dextypes.StoreKey},
		}
		// UpgradeStoreLoader only actually runs LoadLatestVersionAndUpgrade
		// on the single block where upgradeInfo.Height == last committed
		// version + 1; every other startup (before or long after that
		// height) it transparently falls back to the normal store loader.
		// It is therefore safe to set this unconditionally on every start
		// once upgrade-info.json records this plan name.
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}

	// initialize stores
	app.MountKVStores(keys)
	// initialize BaseApp
	app.SetInitChainer(app.InitChainer)
	app.SetPreBlocker(app.PreBlocker)
	app.SetBeginBlocker(app.BeginBlocker)
	app.SetEndBlocker(app.EndBlocker)
	app.setAnteHandler(txConfig)

	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			panic(fmt.Errorf("error loading last version: %w", err))
		}
	}

	return app
}

// AutoCliOpts returns the autocli options for the app.
func (app *ExampleApp) AutoCliOpts() autocli.AppOptions {
	modules := make(map[string]appmodule.AppModule)
	for _, m := range app.ModuleManager.Modules {
		if moduleWithName, ok := m.(module.HasName); ok {
			moduleName := moduleWithName.Name()
			if appModule, ok := moduleWithName.(appmodule.AppModule); ok {
				modules[moduleName] = appModule
			}
		}
	}

	return autocli.AppOptions{
		Modules:               modules,
		ModuleOptions:         runtimeservices.ExtractAutoCLIOptions(app.ModuleManager.Modules),
		AddressCodec:          authcodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
		ValidatorAddressCodec: authcodec.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix()),
		ConsensusAddressCodec: authcodec.NewBech32Codec(sdk.GetConfig().GetBech32ConsensusAddrPrefix()),
	}
}

func (app *ExampleApp) setAnteHandler(txConfig client.TxConfig) {
	anteHandler, err := ante.NewAnteHandler(
		ante.HandlerOptions{
			AccountKeeper:   app.AccountKeeper,
			BankKeeper:      app.BankKeeper,
			SignModeHandler: txConfig.SignModeHandler(),
			SigGasConsumer:  ante.DefaultSigVerificationGasConsumer,
			SigVerifyOptions: []ante.SigVerificationDecoratorOption{
				// change below as needed.
				ante.WithUnorderedTxGasCost(ante.DefaultUnorderedTxGasCost),
				ante.WithMaxUnorderedTxTimeoutDuration(ante.DefaultMaxTimeoutDuration),
			},
		},
	)
	if err != nil {
		panic(err)
	}

	// Set the AnteHandler for the app
	app.SetAnteHandler(anteHandler)
}

// RegisterAPIRoutes registers all application module routes with the provided
// API server.
func (app *ExampleApp) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
	clientCtx := apiSvr.ClientCtx
	// Register new tx routes from grpc-gateway.
	authtx.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register new CometBFT queries routes from grpc-gateway.
	cmtservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register node gRPC service for grpc-gateway.
	nodeservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register grpc-gateway routes for all modules.
	app.BasicModuleManager.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// register swagger API from root so that other applications can override easily
	if err := server.RegisterSwaggerAPI(apiSvr.ClientCtx, apiSvr.Router, apiConfig.Swagger); err != nil {
		panic(err)
	}
}

// RegisterTxService implements the Application.RegisterTxService method.
func (app *ExampleApp) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.GRPCQueryRouter(), clientCtx, app.Simulate, app.interfaceRegistry)
}

// RegisterTendermintService implements the Application.RegisterTendermintService method.
func (app *ExampleApp) RegisterTendermintService(clientCtx client.Context) {
	cmtApp := server.NewCometABCIWrapper(app)
	cmtservice.RegisterTendermintService(
		clientCtx,
		app.BaseApp.GRPCQueryRouter(),
		app.interfaceRegistry,
		cmtApp.Query,
	)
}

func (app *ExampleApp) RegisterNodeService(clientCtx client.Context, cfg config.Config) {
	nodeservice.RegisterNodeService(clientCtx, app.GRPCQueryRouter(), cfg, func() int64 {
		return app.CommitMultiStore().EarliestVersion()
	})
}

func (app *ExampleApp) Name() string {
	return appName
}

func (app *ExampleApp) LegacyAmino() *codec.LegacyAmino {
	return app.legacyAmino
}

func (app *ExampleApp) BeginBlocker(ctx sdk.Context) (sdk.BeginBlock, error) {
	return app.ModuleManager.BeginBlock(ctx)
}

func (app *ExampleApp) EndBlocker(ctx sdk.Context) (sdk.EndBlock, error) {
	return app.ModuleManager.EndBlock(ctx)
}

// GenesisState of the blockchain is represented here as a map of raw json
// messages key'd by a identifier string.
// The identifier is used to determine which module genesis information belongs
// to so it may be appropriately routed during init chain.
// Within this application default genesis information is retrieved from
// the ModuleBasicManager which populates json from each BasicModule
// object provided to it during init.
type GenesisState map[string]json.RawMessage

func (app *ExampleApp) InitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var genesisState GenesisState
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}
	return app.ModuleManager.InitGenesis(ctx, app.appCodec, genesisState)
}

// PreBlocker runs the module manager's PreBlock hooks (e.g. auth's unordered
// transaction pruning, and -- first in order -- x/upgrade's own PreBlocker,
// which applies any due, scheduled upgrade). This method is defined
// directly on ExampleApp so it shadows the embedded *baseapp.BaseApp's own
// PreBlocker() getter; passing app.PreBlocker (no call) as a method value to
// SetPreBlocker wires this implementation in. Previously this line called
// app.PreBlocker() (with parens), which invokes BaseApp's getter and
// returns whatever preBlocker was already set — nil, since this is the
// very call that was supposed to set it. That compiled cleanly but
// silently disabled all PreBlock logic.
//
// It also carries a one-time bootstrap step for AddDexStoreUpgradeName.
// x/upgrade's own PreBlocker (called below via ModuleManager.PreBlock)
// only invokes a registered UpgradeHandler when it finds a matching Plan
// already scheduled ON-CHAIN, in x/upgrade's own store -- normally put
// there ahead of time by a passed MsgSoftwareUpgrade governance proposal,
// while the OLD binary (which already has x/upgrade) is still running.
// That path is impossible here: x/upgrade's own store does not exist
// before this exact block (it is created by the very same
// LoadLatestVersionAndUpgrade call, driven by upgrade-info.json, that also
// creates `dex`'s store -- see NewExampleApp above), so there was never an
// earlier point at which any binary could have scheduled a Plan in it.
//
// Keeper.ScheduleUpgrade explicitly supports scheduling a Plan for the
// CURRENT height from within PreBlock/BeginBlock ("allow for the
// possibility of chains to schedule upgrades in begin block of the same
// block as a strategy for emergency hard fork recoveries" -- see its
// implementation). Doing that here, immediately before
// ModuleManager.PreBlock runs x/upgrade's own PreBlocker, means this one
// bootstrap event flows through x/upgrade's completely standard,
// unmodified ApplyUpgrade -> RunMigrations pipeline -- the exact same code
// path every subsequent, normal, governance-scheduled upgrade on this
// chain will use. This can only fire once: it is gated on the exact block
// height recorded in upgrade-info.json (read once at boot into
// app.addDexStoreUpgradeInfo), and ScheduleUpgrade itself refuses to
// reschedule a plan name that has already been applied and marked done.
func (app *ExampleApp) PreBlocker(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
	if app.addDexStoreUpgradeInfo.Name == AddDexStoreUpgradeName &&
		req.Height == app.addDexStoreUpgradeInfo.Height {
		if err := app.UpgradeKeeper.ScheduleUpgrade(ctx, upgradetypes.Plan{
			Name:   AddDexStoreUpgradeName,
			Height: req.Height,
		}); err != nil {
			return nil, fmt.Errorf("bootstrapping %s upgrade plan at height %d: %w", AddDexStoreUpgradeName, req.Height, err)
		}
	}

	return app.ModuleManager.PreBlock(ctx)
}

func (app *ExampleApp) SimulationManager() *module.SimulationManager {
	return app.sm
}

func (app *ExampleApp) InterfaceRegistry() codectypes.InterfaceRegistry {
	return app.interfaceRegistry
}

func (app *ExampleApp) AppCodec() codec.Codec {
	return app.appCodec
}

func (app *ExampleApp) TxConfig() client.TxConfig {
	return app.txConfig
}

// BlockedAddresses returns all the app's blocked account addresses.
func BlockedAddresses() map[string]bool {
	modAccAddrs := make(map[string]bool)
	for acc := range maccPerms {
		modAccAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}

	// allow the following addresses to receive funds
	delete(modAccAddrs, authtypes.NewModuleAddress(govtypes.ModuleName).String())

	return modAccAddrs
}

// GetKey returns the KVStoreKey for the provided store key.
//
// NOTE: This is solely to be used for testing purposes.
func (app *ExampleApp) GetKey(storeKey string) *storetypes.KVStoreKey {
	return app.keys[storeKey]
}

func (app *ExampleApp) LoadHeight(height int64) error {
	return app.LoadVersion(height)
}

// DefaultGenesis returns a default genesis from the registered modules.
func (app *ExampleApp) DefaultGenesis() GenesisState {
	return app.BasicModuleManager.DefaultGenesis(app.appCodec)
}
