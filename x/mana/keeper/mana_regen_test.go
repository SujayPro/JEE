package keeper

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/jee-chain/jee/x/mana/types"
)

// TestManaRegenFormula verifies:
//
//	Mana_Regen_Rate = (Balance * Total_Mana_Pool) / Total_Supply
//	Mana_Per_Block  = Mana_Regen_Rate / Blocks_Per_Year
func TestManaRegenFormula(t *testing.T) {
	params := types.DefaultParams()

	// 10% of supply → 10% of mana pool
	balance := uint64(100_000_000)
	supply := uint64(1_000_000_000)
	expectedRate := (balance * params.TotalManaPool) / supply // 100_000_000

	perBlock := expectedRate / params.BlocksPerYear
	if perBlock == 0 {
		t.Fatal("expected non-zero per-block regen for 10% holder")
	}

	// After 1 year of blocks, account should approach cap = expectedRate.
	// Integer division truncates per-block regen, so gained may be slightly below rate.
	blocksPerYear := params.BlocksPerYear
	gained := perBlock * blocksPerYear
	if gained > expectedRate {
		t.Fatalf("gained mana %d exceeds cap %d", gained, expectedRate)
	}
	// Allow up to 10% truncation loss from per-block integer division
	minGained := expectedRate - expectedRate/10
	if gained < minGained {
		t.Fatalf("expected at least %d mana after 1 year, got %d (rate=%d perBlock=%d)", minGained, gained, expectedRate, perBlock)
	}

	// Tx cost sanity check
	cost := ComputeTxManaCost(500) // 100 + 500 = 600
	if cost != 600 {
		t.Fatalf("unexpected tx cost: %d", cost)
	}

	_ = sdk.NewCoin(types.BondDenom, math.NewInt(1))
}
