package keeper

import (
	"context"
	"fmt"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/tutu787878/xpex-chain/x/dex/types"
)

type msgServer struct {
	*Keeper
}

var _ types.MsgServer = msgServer{}

// emit publishes a typed event so off-chain indexers (see xpex-chain-dex) can
// follow DEX activity from block results without re-parsing transactions.
func emit(ctx context.Context, eventType string, attrs ...sdk.Attribute) {
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent(eventType, attrs...))
}

func poolIDString(id uint64) string { return strconv.FormatUint(id, 10) }

func NewMsgServerImpl(k *Keeper) types.MsgServer {
	return msgServer{Keeper: k}
}

func (k msgServer) CreatePool(
	ctx context.Context,
	msg *types.MsgCreatePool,
) (*types.MsgCreatePoolResponse, error) {
	if msg.Sender == "" {
		return nil, fmt.Errorf("sender cannot be empty")
	}

	if msg.TokenA == "" || msg.TokenB == "" {
		return nil, fmt.Errorf("token denoms cannot be empty")
	}

	if msg.TokenA == msg.TokenB {
		return nil, fmt.Errorf("token denoms must be different")
	}

	_, exists, err := k.FindPoolByPair(ctx, msg.TokenA, msg.TokenB)
	if err != nil {
		return nil, fmt.Errorf("check existing pool: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("pool for this token pair already exists")
	}

	coinA, err := parseCoins(msg.TokenA, msg.AmountA)
	if err != nil {
		return nil, fmt.Errorf("invalid token A: %w", err)
	}

	coinB, err := parseCoins(msg.TokenB, msg.AmountB)
	if err != nil {
		return nil, fmt.Errorf("invalid token B: %w", err)
	}

	amountA := coinA.Amount
	amountB := coinB.Amount

	product := amountA.Mul(amountB)
	totalLP := integerSqrt(product)

	if !totalLP.IsPositive() {
		return nil, fmt.Errorf("initial liquidity is too small")
	}

	feeBps := msg.FeeBps
	if feeBps == "" {
		feeBps = "30"
	}

	fee, ok := sdkmath.NewIntFromString(feeBps)
	if !ok || fee.IsNegative() || fee.GTE(sdkmath.NewInt(10000)) {
		return nil, fmt.Errorf("invalid fee_bps")
	}

	poolID, err := k.AllocatePoolID(ctx)
	if err != nil {
		return nil, fmt.Errorf("allocate pool id: %w", err)
	}

	if err := k.SendFromUser(
		ctx,
		msg.Sender,
		sdk.NewCoins(coinA, coinB),
	); err != nil {
		return nil, fmt.Errorf("transfer liquidity to pool: %w", err)
	}

	pool := types.Pool{
		Id:       poolID,
		TokenA:   msg.TokenA,
		TokenB:   msg.TokenB,
		ReserveA: amountA.String(),
		ReserveB: amountB.String(),
		TotalLp:  totalLP.String(),
		FeeBps:   fee.String(),
	}

	if err := k.SetPool(ctx, pool); err != nil {
		return nil, fmt.Errorf("save pool: %w", err)
	}

	if err := k.SetLP(ctx, poolID, msg.Sender, totalLP.String()); err != nil {
		return nil, fmt.Errorf("save LP balance: %w", err)
	}

	emit(ctx, "xpex.dex.pool_created",
		sdk.NewAttribute("pool_id", poolIDString(poolID)),
		sdk.NewAttribute("sender", msg.Sender),
		sdk.NewAttribute("token_a", msg.TokenA),
		sdk.NewAttribute("token_b", msg.TokenB),
		sdk.NewAttribute("amount_a", amountA.String()),
		sdk.NewAttribute("amount_b", amountB.String()),
		sdk.NewAttribute("fee_bps", fee.String()),
		sdk.NewAttribute("lp_minted", totalLP.String()),
	)

	return &types.MsgCreatePoolResponse{
		PoolId: poolID,
	}, nil
}

func (k msgServer) AddLiquidity(
	ctx context.Context,
	msg *types.MsgAddLiquidity,
) (*types.MsgAddLiquidityResponse, error) {
	if msg.Sender == "" {
		return nil, fmt.Errorf("sender cannot be empty")
	}

	pool, err := k.GetPool(ctx, msg.PoolId)
	if err != nil {
		return nil, fmt.Errorf("get pool: %w", err)
	}

	amountA, ok := sdkmath.NewIntFromString(msg.AmountA)
	if !ok || !amountA.IsPositive() {
		return nil, fmt.Errorf("invalid amount_a")
	}

	amountB, ok := sdkmath.NewIntFromString(msg.AmountB)
	if !ok || !amountB.IsPositive() {
		return nil, fmt.Errorf("invalid amount_b")
	}

	reserveA, ok := sdkmath.NewIntFromString(pool.ReserveA)
	if !ok || !reserveA.IsPositive() {
		return nil, fmt.Errorf("invalid pool reserve_a")
	}

	reserveB, ok := sdkmath.NewIntFromString(pool.ReserveB)
	if !ok || !reserveB.IsPositive() {
		return nil, fmt.Errorf("invalid pool reserve_b")
	}

	totalLP, ok := sdkmath.NewIntFromString(pool.TotalLp)
	if !ok || !totalLP.IsPositive() {
		return nil, fmt.Errorf("invalid pool total_lp")
	}

	lpFromA := amountA.Mul(totalLP).Quo(reserveA)
	lpFromB := amountB.Mul(totalLP).Quo(reserveB)

	lpReceived := lpFromA
	if lpFromB.LT(lpReceived) {
		lpReceived = lpFromB
	}

	if !lpReceived.IsPositive() {
		return nil, fmt.Errorf("liquidity contribution is too small")
	}

	if msg.MinLp != "" {
		minLP, ok := sdkmath.NewIntFromString(msg.MinLp)
		if !ok || minLP.IsNegative() {
			return nil, fmt.Errorf("invalid min_lp")
		}
		if lpReceived.LT(minLP) {
			return nil, fmt.Errorf("LP received is below min_lp")
		}
	}

	usedA := lpReceived.Mul(reserveA).Quo(totalLP)
	usedB := lpReceived.Mul(reserveB).Quo(totalLP)

	if !usedA.IsPositive() || !usedB.IsPositive() {
		return nil, fmt.Errorf("liquidity contribution is too small")
	}

	coins := sdk.NewCoins(
		sdk.NewCoin(pool.TokenA, usedA),
		sdk.NewCoin(pool.TokenB, usedB),
	)

	if err := k.SendFromUser(ctx, msg.Sender, coins); err != nil {
		return nil, fmt.Errorf("transfer liquidity to pool: %w", err)
	}

	pool.ReserveA = reserveA.Add(usedA).String()
	pool.ReserveB = reserveB.Add(usedB).String()
	pool.TotalLp = totalLP.Add(lpReceived).String()

	if err := k.SetPool(ctx, pool); err != nil {
		return nil, fmt.Errorf("save pool: %w", err)
	}

	userLPString, err := k.GetLP(ctx, msg.PoolId, msg.Sender)
	if err != nil {
		userLPString = "0"
	}

	userLP, ok := sdkmath.NewIntFromString(userLPString)
	if !ok || userLP.IsNegative() {
		return nil, fmt.Errorf("invalid user LP balance")
	}

	if err := k.SetLP(
		ctx,
		msg.PoolId,
		msg.Sender,
		userLP.Add(lpReceived).String(),
	); err != nil {
		return nil, fmt.Errorf("save LP balance: %w", err)
	}

	emit(ctx, "xpex.dex.liquidity_added",
		sdk.NewAttribute("pool_id", poolIDString(msg.PoolId)),
		sdk.NewAttribute("sender", msg.Sender),
		sdk.NewAttribute("amount_a", usedA.String()),
		sdk.NewAttribute("amount_b", usedB.String()),
		sdk.NewAttribute("lp_received", lpReceived.String()),
		sdk.NewAttribute("reserve_a", pool.ReserveA),
		sdk.NewAttribute("reserve_b", pool.ReserveB),
	)

	return &types.MsgAddLiquidityResponse{
		LpReceived: lpReceived.String(),
	}, nil
}

func (k msgServer) RemoveLiquidity(
	ctx context.Context,
	msg *types.MsgRemoveLiquidity,
) (*types.MsgRemoveLiquidityResponse, error) {
	if msg.Sender == "" {
		return nil, fmt.Errorf("sender cannot be empty")
	}

	pool, err := k.GetPool(ctx, msg.PoolId)
	if err != nil {
		return nil, fmt.Errorf("get pool: %w", err)
	}

	lpAmount, ok := sdkmath.NewIntFromString(msg.LpAmount)
	if !ok || !lpAmount.IsPositive() {
		return nil, fmt.Errorf("invalid lp_amount")
	}

	totalLP, ok := sdkmath.NewIntFromString(pool.TotalLp)
	if !ok || !totalLP.IsPositive() {
		return nil, fmt.Errorf("invalid pool total_lp")
	}

	reserveA, ok := sdkmath.NewIntFromString(pool.ReserveA)
	if !ok || !reserveA.IsPositive() {
		return nil, fmt.Errorf("invalid pool reserve_a")
	}

	reserveB, ok := sdkmath.NewIntFromString(pool.ReserveB)
	if !ok || !reserveB.IsPositive() {
		return nil, fmt.Errorf("invalid pool reserve_b")
	}

	if lpAmount.GT(totalLP) {
		return nil, fmt.Errorf("insufficient total LP supply")
	}

	userLPString, err := k.GetLP(ctx, msg.PoolId, msg.Sender)
	if err != nil {
		return nil, fmt.Errorf("get LP balance: %w", err)
	}

	userLP, ok := sdkmath.NewIntFromString(userLPString)
	if !ok || lpAmount.GT(userLP) {
		return nil, fmt.Errorf("insufficient LP balance")
	}

	var amountA sdkmath.Int
	var amountB sdkmath.Int

	// When withdrawing all LP, return the exact current reserves.
	// This prevents integer-division dust from being left in the pool.
	withdrawingAll := lpAmount.Equal(totalLP)
	if withdrawingAll {
		amountA = reserveA
		amountB = reserveB
	} else {
		amountA = lpAmount.Mul(reserveA).Quo(totalLP)
		amountB = lpAmount.Mul(reserveB).Quo(totalLP)
	}

	if !amountA.IsPositive() || !amountB.IsPositive() {
		return nil, fmt.Errorf("liquidity withdrawal is too small")
	}

	if msg.MinA != "" {
		minA, ok := sdkmath.NewIntFromString(msg.MinA)
		if !ok || minA.IsNegative() {
			return nil, fmt.Errorf("invalid min_a")
		}
		if amountA.LT(minA) {
			return nil, fmt.Errorf("amount A is below min_a")
		}
	}

	if msg.MinB != "" {
		minB, ok := sdkmath.NewIntFromString(msg.MinB)
		if !ok || minB.IsNegative() {
			return nil, fmt.Errorf("invalid min_b")
		}
		if amountB.LT(minB) {
			return nil, fmt.Errorf("amount B is below min_b")
		}
	}

	recipient, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, fmt.Errorf("invalid sender address: %w", err)
	}

	coins := sdk.NewCoins(
		sdk.NewCoin(pool.TokenA, amountA),
		sdk.NewCoin(pool.TokenB, amountB),
	)

	if err := k.bankKeeper.SendCoinsFromModuleToAccount(
		ctx,
		types.ModuleName,
		recipient,
		coins,
	); err != nil {
		return nil, fmt.Errorf("transfer liquidity to user: %w", err)
	}

	newUserLP := userLP.Sub(lpAmount)

	if withdrawingAll {
		// The pool is completely emptied. Remove it instead of leaving
		// an unusable 0/0 pool behind.
		if err := k.pools.Remove(ctx, msg.PoolId); err != nil {
			return nil, fmt.Errorf("remove empty pool: %w", err)
		}
	} else {
		pool.ReserveA = reserveA.Sub(amountA).String()
		pool.ReserveB = reserveB.Sub(amountB).String()
		pool.TotalLp = totalLP.Sub(lpAmount).String()

		if err := k.SetPool(ctx, pool); err != nil {
			return nil, fmt.Errorf("save pool: %w", err)
		}
	}
	if err := k.SetLP(ctx, msg.PoolId, msg.Sender, newUserLP.String()); err != nil {
		return nil, fmt.Errorf("save LP balance: %w", err)
	}

	emit(ctx, "xpex.dex.liquidity_removed",
		sdk.NewAttribute("pool_id", poolIDString(msg.PoolId)),
		sdk.NewAttribute("sender", msg.Sender),
		sdk.NewAttribute("lp_amount", lpAmount.String()),
		sdk.NewAttribute("amount_a", amountA.String()),
		sdk.NewAttribute("amount_b", amountB.String()),
	)

	return &types.MsgRemoveLiquidityResponse{
		AmountA: amountA.String(),
		AmountB: amountB.String(),
	}, nil
}

func (k msgServer) Swap(
	ctx context.Context,
	msg *types.MsgSwap,
) (*types.MsgSwapResponse, error) {
	if msg.Sender == "" {
		return nil, fmt.Errorf("sender cannot be empty")
	}

	pool, err := k.GetPool(ctx, msg.PoolId)
	if err != nil {
		return nil, fmt.Errorf("get pool: %w", err)
	}

	amountIn, ok := sdkmath.NewIntFromString(msg.OfferAmount)
	if !ok || !amountIn.IsPositive() {
		return nil, fmt.Errorf("invalid offer_amount")
	}

	reserveA, ok := sdkmath.NewIntFromString(pool.ReserveA)
	if !ok || !reserveA.IsPositive() {
		return nil, fmt.Errorf("invalid pool reserve_a")
	}

	reserveB, ok := sdkmath.NewIntFromString(pool.ReserveB)
	if !ok || !reserveB.IsPositive() {
		return nil, fmt.Errorf("invalid pool reserve_b")
	}

	feeBps, ok := sdkmath.NewIntFromString(pool.FeeBps)
	if !ok || feeBps.IsNegative() || feeBps.GTE(sdkmath.NewInt(10000)) {
		return nil, fmt.Errorf("invalid pool fee")
	}

	var reserveIn sdkmath.Int
	var reserveOut sdkmath.Int
	var outputDenom string

	switch msg.OfferDenom {
	case pool.TokenA:
		reserveIn = reserveA
		reserveOut = reserveB
		outputDenom = pool.TokenB
	case pool.TokenB:
		reserveIn = reserveB
		reserveOut = reserveA
		outputDenom = pool.TokenA
	default:
		return nil, fmt.Errorf("offer denom is not part of pool")
	}

	amountOut, err := CalculateSwap(
		reserveIn,
		reserveOut,
		amountIn,
		feeBps,
	)
	if err != nil {
		return nil, fmt.Errorf("calculate swap: %w", err)
	}

	if msg.MinReceive != "" {
		minReceive, ok := sdkmath.NewIntFromString(msg.MinReceive)
		if !ok || minReceive.IsNegative() {
			return nil, fmt.Errorf("invalid min_receive")
		}

		if amountOut.LT(minReceive) {
			return nil, fmt.Errorf("output amount is below min_receive")
		}
	}

	sender, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, fmt.Errorf("invalid sender address: %w", err)
	}

	inputCoin := sdk.NewCoin(msg.OfferDenom, amountIn)

	if err := k.bankKeeper.SendCoinsFromAccountToModule(
		ctx,
		sender,
		types.ModuleName,
		sdk.NewCoins(inputCoin),
	); err != nil {
		return nil, fmt.Errorf("transfer input tokens to pool: %w", err)
	}

	outputCoin := sdk.NewCoin(outputDenom, amountOut)

	if err := k.bankKeeper.SendCoinsFromModuleToAccount(
		ctx,
		types.ModuleName,
		sender,
		sdk.NewCoins(outputCoin),
	); err != nil {
		return nil, fmt.Errorf("transfer output tokens to user: %w", err)
	}

	if msg.OfferDenom == pool.TokenA {
		pool.ReserveA = reserveA.Add(amountIn).String()
		pool.ReserveB = reserveB.Sub(amountOut).String()
	} else {
		pool.ReserveB = reserveB.Add(amountIn).String()
		pool.ReserveA = reserveA.Sub(amountOut).String()
	}

	if err := k.SetPool(ctx, pool); err != nil {
		return nil, fmt.Errorf("save pool: %w", err)
	}

	emit(ctx, "xpex.dex.swap",
		sdk.NewAttribute("pool_id", poolIDString(msg.PoolId)),
		sdk.NewAttribute("sender", msg.Sender),
		sdk.NewAttribute("offer_denom", msg.OfferDenom),
		sdk.NewAttribute("offer_amount", amountIn.String()),
		sdk.NewAttribute("receive_denom", outputDenom),
		sdk.NewAttribute("receive_amount", amountOut.String()),
		sdk.NewAttribute("token_a", pool.TokenA),
		sdk.NewAttribute("token_b", pool.TokenB),
		sdk.NewAttribute("reserve_a", pool.ReserveA),
		sdk.NewAttribute("reserve_b", pool.ReserveB),
	)

	return &types.MsgSwapResponse{
		ReceiveAmount: amountOut.String(),
	}, nil
}

func integerSqrt(n sdkmath.Int) sdkmath.Int {
	if !n.IsPositive() {
		return sdkmath.ZeroInt()
	}

	x := n
	y := x.AddRaw(1).QuoRaw(2)

	for y.LT(x) {
		x = y
		y = x.Add(n.Quo(x)).QuoRaw(2)
	}

	return x
}

func parseCoins(denom string, amount string) (sdk.Coin, error) {
	coin, ok := sdk.NewInt64Coin(denom, 0), true
	if amount == "" {
		return coin, fmt.Errorf("amount cannot be empty")
	}

	amt, ok := sdkmath.NewIntFromString(amount)
	if !ok || !amt.IsPositive() {
		return coin, fmt.Errorf("invalid amount")
	}

	return sdk.NewCoin(denom, amt), nil
}
