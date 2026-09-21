package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"

	querytypes "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/tutu787878/xpex-chain/x/dex/types"
)

type queryServer struct {
	*Keeper
}

func NewQueryServer(k *Keeper) types.QueryServer {
	return &queryServer{k}
}

func (q queryServer) Pool(ctx context.Context, req *types.QueryPoolRequest) (*types.QueryPoolResponse, error) {
	pool, err := q.GetPool(ctx, req.PoolId)
	if err != nil {
		return nil, err
	}

	return &types.QueryPoolResponse{
		Pool: &pool,
	}, nil
}

func (q queryServer) LPBalance(ctx context.Context, req *types.QueryLPBalanceRequest) (*types.QueryLPBalanceResponse, error) {
	lp, err := q.GetLP(ctx, req.PoolId, req.Address)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return &types.QueryLPBalanceResponse{
				LpBalance: "0",
			}, nil
		}
		return nil, err
	}

	return &types.QueryLPBalanceResponse{
		LpBalance: lp,
	}, nil
}

func (q queryServer) LPBalances(ctx context.Context, req *types.QueryLPBalancesRequest) (*types.QueryLPBalancesResponse, error) {
	iterator, err := q.lpBalances.Iterate(
		ctx,
		collections.NewPrefixedPairRange[uint64, string](req.PoolId),
	)
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	balances := make([]*types.LPBalanceEntry, 0)

	for ; iterator.Valid(); iterator.Next() {
		kv, err := iterator.KeyValue()
		if err != nil {
			return nil, err
		}

		balances = append(balances, &types.LPBalanceEntry{
			Address:   kv.Key.K2(),
			LpBalance: kv.Value,
		})
	}

	return &types.QueryLPBalancesResponse{
		Balances: balances,
	}, nil
}

func (q queryServer) Pools(ctx context.Context, req *types.QueryPoolsRequest) (*types.QueryPoolsResponse, error) {
	var allPools []*types.Pool

	err := q.pools.Walk(ctx, nil, func(id uint64, pool types.Pool) (bool, error) {
		p := pool
		allPools = append(allPools, &p)
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	total := uint64(len(allPools))
	page := req.GetPagination()

	if page == nil || page.Limit == 0 {
		return &types.QueryPoolsResponse{
			Pools: allPools,
			Pagination: &querytypes.PageResponse{
				Total: total,
			},
		}, nil
	}

	start := page.Offset
	if start > total {
		start = total
	}

	end := start + page.Limit
	if end > total {
		end = total
	}

	result := allPools[start:end]

	var nextKey []byte
	if end < total {
		nextKey = []byte("more")
	}

	return &types.QueryPoolsResponse{
		Pools: result,
		Pagination: &querytypes.PageResponse{
			NextKey: nextKey,
			Total:   total,
		},
	}, nil
}
