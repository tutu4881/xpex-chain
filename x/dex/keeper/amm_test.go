package keeper

import (
	"testing"

	sdkmath "cosmossdk.io/math"
)

func TestCalculateSwap(t *testing.T) {
	reserveIn := sdkmath.NewInt(10_000_000)
	reserveOut := sdkmath.NewInt(10_000_000)
	amountIn := sdkmath.NewInt(1_000_000)
	feeBps := sdkmath.NewInt(30)

	out, err := CalculateSwap(reserveIn, reserveOut, amountIn, feeBps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !out.Equal(sdkmath.NewInt(906_610)) {
		t.Fatalf("unexpected output: got %s, want 906610", out)
	}
}

func TestCalculateSwapRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		reserveIn  sdkmath.Int
		reserveOut sdkmath.Int
		amountIn   sdkmath.Int
		feeBps     sdkmath.Int
	}{
		{
			name:       "zero reserve in",
			reserveIn:  sdkmath.ZeroInt(),
			reserveOut: sdkmath.NewInt(10_000_000),
			amountIn:   sdkmath.NewInt(1_000),
			feeBps:     sdkmath.NewInt(30),
		},
		{
			name:       "zero amount in",
			reserveIn:  sdkmath.NewInt(10_000_000),
			reserveOut: sdkmath.NewInt(10_000_000),
			amountIn:   sdkmath.ZeroInt(),
			feeBps:     sdkmath.NewInt(30),
		},
		{
			name:       "fee 100 percent",
			reserveIn:  sdkmath.NewInt(10_000_000),
			reserveOut: sdkmath.NewInt(10_000_000),
			amountIn:   sdkmath.NewInt(1_000),
			feeBps:     sdkmath.NewInt(10_000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := CalculateSwap(
				tt.reserveIn,
				tt.reserveOut,
				tt.amountIn,
				tt.feeBps,
			); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestCalculateSwapNeverExceedsReserveOut(t *testing.T) {
	reserveIn := sdkmath.NewInt(10_000_000)
	reserveOut := sdkmath.NewInt(10_000_000)

	out, err := CalculateSwap(
		reserveIn,
		reserveOut,
		sdkmath.NewInt(1_000_000_000_000),
		sdkmath.NewInt(30),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !out.IsPositive() {
		t.Fatal("expected positive output")
	}

	if !out.LT(reserveOut) {
		t.Fatalf("output must remain below reserve: got %s, reserve %s", out, reserveOut)
	}
}

func TestCalculateSwapPreservesInvariant(t *testing.T) {
	reserveIn := sdkmath.NewInt(10_000_000)
	reserveOut := sdkmath.NewInt(10_000_000)
	amountIn := sdkmath.NewInt(1_000_000)
	feeBps := sdkmath.NewInt(30)

	amountOut, err := CalculateSwap(
		reserveIn,
		reserveOut,
		amountIn,
		feeBps,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kBefore := reserveIn.Mul(reserveOut)
	newReserveIn := reserveIn.Add(amountIn)
	newReserveOut := reserveOut.Sub(amountOut)
	kAfter := newReserveIn.Mul(newReserveOut)

	if kAfter.LT(kBefore) {
		t.Fatalf(
			"constant-product invariant decreased: before=%s after=%s",
			kBefore,
			kAfter,
		)
	}
}
