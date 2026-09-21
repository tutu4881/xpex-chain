package dex

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
)

func (a AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: "example.dex.Query",
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "Pool",
					Use:       "pool [pool-id]",
					Short:     "Query a liquidity pool",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "pool_id"},
					},
				},
				{
					RpcMethod: "Pools",
					Use:       "pools",
					Short:     "Query all liquidity pools",
				},
				{
					RpcMethod: "LPBalance",
					Use:       "lp-balance [pool-id] [address]",
					Short:     "Query LP token balance for an address",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "pool_id"},
						{ProtoField: "address"},
					},
				},
				{
					RpcMethod: "LPBalances",
					Use:       "lp-balances [pool-id]",
					Short:     "Query all LP token balances for a pool",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "pool_id"},
					},
				},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              "example.dex.Msg",
			EnhanceCustomCommand: true,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "CreatePool",
					Use:       "create-pool [token-a] [token-b] [amount-a] [amount-b] [fee-bps]",
					Short:     "Create a new liquidity pool",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "token_a"},
						{ProtoField: "token_b"},
						{ProtoField: "amount_a"},
						{ProtoField: "amount_b"},
						{ProtoField: "fee_bps"},
					},
				},
				{
					RpcMethod: "AddLiquidity",
					Use:       "add-liquidity [pool-id] [amount-a] [amount-b] [min-lp]",
					Short:     "Add liquidity to a pool",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "pool_id"},
						{ProtoField: "amount_a"},
						{ProtoField: "amount_b"},
						{ProtoField: "min_lp"},
					},
				},
				{
					RpcMethod: "RemoveLiquidity",
					Use:       "remove-liquidity [pool-id] [lp-amount] [min-a] [min-b]",
					Short:     "Remove liquidity from a pool",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "pool_id"},
						{ProtoField: "lp_amount"},
						{ProtoField: "min_a"},
						{ProtoField: "min_b"},
					},
				},
				{
					RpcMethod: "Swap",
					Use:       "swap [pool-id] [offer-denom] [offer-amount] [min-receive]",
					Short:     "Swap tokens through a liquidity pool",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "pool_id"},
						{ProtoField: "offer_denom"},
						{ProtoField: "offer_amount"},
						{ProtoField: "min_receive"},
					},
				},
			},
		},
	}
}
