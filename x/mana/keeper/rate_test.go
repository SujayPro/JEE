package keeper

import (
	"testing"

	"cosmossdk.io/math"
)

// TestComputeManaRateNoOverflow verifies genesis-scale balances don't overflow (H-E fix).
func TestComputeManaRateNoOverflow(t *testing.T) {
	// Genesis validator balance: 100_000_000_000_000 jeff
	balance := math.NewInt(100_000_000_000_000)
	supply := math.NewInt(1_000_000_000_000_000)
	pool := uint64(1_000_000_000)

	rate, err := computeManaRate(balance, supply, pool, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// (1e14 * 1e9) / 1e15 = 1e8 = 100_000_000
	if rate != 100_000_000 {
		t.Fatalf("expected rate 100000000, got %d", rate)
	}
}

// TestComputeManaRateMultiplier applies multiplier consistently (H-D guard).
func TestComputeManaRateMultiplier(t *testing.T) {
	balance := math.NewInt(100)
	supply := math.NewInt(1000)
	pool := uint64(1_000_000_000)

	rate, err := computeManaRate(balance, supply, pool, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	base, _ := computeManaRate(balance, supply, pool, 1)
	if rate != base*2 {
		t.Fatalf("expected %d, got %d", base*2, rate)
	}
}

// TestAddManaSaturating verifies regen arithmetic never overflows uint64 and is
// always capped at maxMana — protecting long-dormant accounts from wrap-around.
func TestAddManaSaturating(t *testing.T) {
	const cap = uint64(1_000_000)

	// Normal accumulation below the cap.
	if got := addManaSaturating(0, 1_000, 5, cap); got != 5_000 {
		t.Fatalf("normal: expected 5000, got %d", got)
	}

	// Accumulation that exceeds the cap is clamped.
	if got := addManaSaturating(0, 1_000, 10_000, cap); got != cap {
		t.Fatalf("clamp: expected %d, got %d", cap, got)
	}

	// Already at/above the cap stays at the cap.
	if got := addManaSaturating(2_000_000, 1_000, 10, cap); got != cap {
		t.Fatalf("above-cap: expected %d, got %d", cap, got)
	}

	// perBlock*blocksElapsed overflows uint64 — must saturate to cap, not wrap.
	maxU := ^uint64(0)
	if got := addManaSaturating(0, maxU, maxU, cap); got != cap {
		t.Fatalf("overflow: expected %d, got %d", cap, got)
	}

	// Exact headroom fill lands precisely on the cap.
	if got := addManaSaturating(900_000, 1_000, 100, cap); got != cap {
		t.Fatalf("exact-fill: expected %d, got %d", cap, got)
	}
}
