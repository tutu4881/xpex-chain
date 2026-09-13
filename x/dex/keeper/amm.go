package keeper

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// CalculateSwap calculates the amount of output tokens for a constant-product AMM.
//
// Formula:
//
//	amountInWithFee = amountIn * (10000 - feeBps)
//	amountOut = reserveOut * amountInWithFee /
//	            (reserveIn*10000 + amountInWithFee)
//
// All calculations use integer arithmetic.
func CalculateSwap(
	reserveIn sdkmath.Int,
	reserveOut sdkmath.Int,
	amountIn sdkmath.Int,
	feeBps sdkmath.Int,
) (sdkmath.Int, error) {
	if !reserveIn.IsPositive() || !reserveOut.IsPositive() {
		return sdkmath.ZeroInt(), fmt.Errorf("reserves must be positive")
	}

	if !amountIn.IsPositive() {
		return sdkmath.ZeroInt(), fmt.Errorf("amount in must be positive")
	}

	if feeBps.IsNegative() || feeBps.GTE(sdkmath.NewInt(10000)) {
		return sdkmath.ZeroInt(), fmt.Errorf("invalid fee")
	}

	feeMultiplier := sdkmath.NewInt(10000).Sub(feeBps)
	amountInWithFee := amountIn.Mul(feeMultiplier)

	denominator := reserveIn.MulRaw(10000).Add(amountInWithFee)

	amountOut := reserveOut.
		Mul(amountInWithFee).
		Quo(denominator)

	if !amountOut.IsPositive() {
		return sdkmath.ZeroInt(), fmt.Errorf("output amount is zero")
	}

	if amountOut.GTE(reserveOut) {
		return sdkmath.ZeroInt(), fmt.Errorf("insufficient liquidity")
	}

	return amountOut, nil
}
