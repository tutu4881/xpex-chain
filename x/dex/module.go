package dex

import (
	"context"
	"encoding/json"

	"cosmossdk.io/core/appmodule"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	"github.com/tutu787878/xpex-chain/x/dex/keeper"
	dextypes "github.com/tutu787878/xpex-chain/x/dex/types"
)

var (
	_ module.AppModuleBasic      = AppModuleBasic{}
	_ module.HasGenesisBasics    = AppModuleBasic{}
	_ appmodule.AppModule        = AppModule{}
	_ module.HasConsensusVersion = AppModule{}
	_ module.HasGenesis          = AppModule{}
	_ module.HasServices         = AppModule{}
	_ appmodule.HasEndBlocker    = AppModule{}
	_ appmodule.HasBeginBlocker  = AppModule{}
)

type AppModuleBasic struct {
	cdc codec.Codec
}

func (a AppModuleBasic) DefaultGenesis(jsonCodec codec.JSONCodec) json.RawMessage {
	gs := dextypes.GenesisState{
		NextPoolId: 1,
		Pools:      []*dextypes.Pool{},
	}
	return jsonCodec.MustMarshalJSON(&gs)
}

func (a AppModuleBasic) ValidateGenesis(
	jsonCodec codec.JSONCodec,
	_ client.TxEncodingConfig,
	message json.RawMessage,
) error {
	gs := dextypes.GenesisState{}
	return jsonCodec.UnmarshalJSON(message, &gs)
}

func (a AppModuleBasic) Name() string {
	return dextypes.ModuleName
}

func (a AppModuleBasic) RegisterInterfaces(registry types.InterfaceRegistry) {
	dextypes.RegisterInterfaces(registry)
}

func (a AppModuleBasic) RegisterLegacyAminoCodec(amino *codec.LegacyAmino) {}

func (a AppModuleBasic) RegisterGRPCGatewayRoutes(
	clientCtx client.Context,
	mux *runtime.ServeMux,
) {
	if err := dextypes.RegisterQueryHandlerClient(
		context.Background(),
		mux,
		dextypes.NewQueryClient(clientCtx),
	); err != nil {
		panic(err)
	}
}

type AppModule struct {
	keeper *keeper.Keeper
	AppModuleBasic
}

func NewAppModule(cdc codec.Codec, keeper *keeper.Keeper) AppModule {
	return AppModule{
		keeper:         keeper,
		AppModuleBasic: AppModuleBasic{cdc: cdc},
	}
}

func (a AppModule) BeginBlock(ctx context.Context) error {
	return nil
}

func (a AppModule) EndBlock(ctx context.Context) error {
	return nil
}

func (a AppModule) RegisterServices(configurator module.Configurator) {
	dextypes.RegisterMsgServer(
		configurator.MsgServer(),
		keeper.NewMsgServerImpl(a.keeper),
	)
	dextypes.RegisterQueryServer(
		configurator.QueryServer(),
		keeper.NewQueryServer(a.keeper),
	)
}

func (a AppModule) InitGenesis(
	ctx sdk.Context,
	jsonCodec codec.JSONCodec,
	message json.RawMessage,
) {
	gs := &dextypes.GenesisState{}
	jsonCodec.MustUnmarshalJSON(message, gs)

	if err := a.keeper.InitGenesis(ctx, gs); err != nil {
		panic(err)
	}
}

func (a AppModule) ExportGenesis(
	ctx sdk.Context,
	jsonCodec codec.JSONCodec,
) json.RawMessage {
	gs, err := a.keeper.ExportGenesis(ctx)
	if err != nil {
		panic(err)
	}

	return jsonCodec.MustMarshalJSON(gs)
}

func (a AppModule) ConsensusVersion() uint64 {
	return 1
}

func (a AppModule) IsOnePerModuleType() {}
func (a AppModule) IsAppModule()        {}
