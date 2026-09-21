package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/tutu787878/xpex-chain/x/dex/types"
)

type Keeper struct {
	Schema     collections.Schema
	pools      collections.Map[uint64, types.Pool]
	nextPoolID collections.Item[uint64]
	lpBalances collections.Map[collections.Pair[uint64, string], string]
	bankKeeper types.BankKeeper
}

func NewKeeper(
	storeService store.KVStoreService,
	cdc codec.Codec,
	bankKeeper types.BankKeeper,
) *Keeper {
	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		pools: collections.NewMap(
			sb,
			collections.NewPrefix(0),
			"pools",
			collections.Uint64Key,
			codec.CollValue[types.Pool](cdc),
		),
		nextPoolID: collections.NewItem(
			sb,
			collections.NewPrefix(1),
			"next_pool_id",
			collections.Uint64Value,
		),
		lpBalances: collections.NewMap(
			sb,
			collections.NewPrefix(2),
			"lp_balances",
			collections.PairKeyCodec(collections.Uint64Key, collections.StringKey),
			collections.StringValue,
		),
		bankKeeper: bankKeeper,
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}

	k.Schema = schema

	return &k
}

func (k Keeper) GetPool(ctx context.Context, poolID uint64) (types.Pool, error) {
	pool, err := k.pools.Get(ctx, poolID)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return types.Pool{}, collections.ErrNotFound
		}
		return types.Pool{}, err
	}

	return pool, nil
}

func (k Keeper) SetPool(ctx context.Context, pool types.Pool) error {
	return k.pools.Set(ctx, pool.Id, pool)
}

func (k Keeper) FindPoolByPair(ctx context.Context, tokenA, tokenB string) (types.Pool, bool, error) {
	var found types.Pool

	err := k.pools.Walk(ctx, nil, func(id uint64, pool types.Pool) (bool, error) {
		if (pool.TokenA == tokenA && pool.TokenB == tokenB) ||
			(pool.TokenA == tokenB && pool.TokenB == tokenA) {
			found = pool
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return types.Pool{}, false, err
	}

	if found.Id == 0 {
		return types.Pool{}, false, nil
	}

	return found, true, nil
}

func (k Keeper) SendFromUser(
	ctx context.Context,
	sender string,
	coins sdk.Coins,
) error {
	addr, err := sdk.AccAddressFromBech32(sender)
	if err != nil {
		return err
	}

	return k.bankKeeper.SendCoinsFromAccountToModule(
		ctx,
		addr,
		types.ModuleName,
		coins,
	)
}

func (k Keeper) SendToUser(
	ctx context.Context,
	recipient string,
	coins sdk.Coins,
) error {
	addr, err := sdk.AccAddressFromBech32(recipient)
	if err != nil {
		return err
	}

	return k.bankKeeper.SendCoinsFromModuleToAccount(
		ctx,
		types.ModuleName,
		addr,
		coins,
	)
}

func (k Keeper) GetLP(ctx context.Context, poolID uint64, owner string) (string, error) {
	return k.lpBalances.Get(ctx, collections.Join[uint64, string](poolID, owner))
}

func (k Keeper) SetLP(ctx context.Context, poolID uint64, owner string, amount string) error {
	return k.lpBalances.Set(ctx, collections.Join[uint64, string](poolID, owner), amount)
}

func (k Keeper) NextPoolID(ctx context.Context) (uint64, error) {
	id, err := k.nextPoolID.Get(ctx)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return 1, nil
		}
		return 0, err
	}
	return id, nil
}

func (k Keeper) AllocatePoolID(ctx context.Context) (uint64, error) {
	id, err := k.NextPoolID(ctx)
	if err != nil {
		return 0, err
	}

	if err := k.nextPoolID.Set(ctx, id+1); err != nil {
		return 0, err
	}

	return id, nil
}

func (k *Keeper) InitGenesis(ctx context.Context, genesis *types.GenesisState) error {
	if genesis.NextPoolId == 0 {
		genesis.NextPoolId = 1
	}

	if err := k.nextPoolID.Set(ctx, genesis.NextPoolId); err != nil {
		return err
	}

	for _, pool := range genesis.Pools {
		if pool == nil {
			continue
		}

		if err := k.pools.Set(ctx, pool.Id, *pool); err != nil {
			return err
		}
	}

	return nil
}

func (k *Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	nextPoolID, err := k.NextPoolID(ctx)
	if err != nil {
		return nil, err
	}

	pools := make([]*types.Pool, 0)

	err = k.pools.Walk(ctx, nil, func(id uint64, pool types.Pool) (bool, error) {
		p := pool
		pools = append(pools, &p)
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return &types.GenesisState{
		NextPoolId: nextPoolID,
		Pools:      pools,
	}, nil
}
