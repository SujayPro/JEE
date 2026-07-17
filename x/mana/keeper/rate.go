package keeper

import (
	"cosmossdk.io/math"

	"github.com/jee-chain/jee/x/mana/types"
)

// computeManaRate calculates (balance * pool / supply) * multiplier using safe integer math.
// Avoids uint64 overflow when genesis-scale balances are used (H-E fix).
func computeManaRate(balanceAmt, supplyAmt math.Int, pool, multiplier uint64) (uint64, error) {
	if supplyAmt.IsZero() || balanceAmt.IsZero() || pool == 0 {
		return 0, nil
	}

	rate := balanceAmt.Mul(math.NewIntFromUint64(pool)).Quo(supplyAmt)
	if multiplier > 0 {
		rate = rate.Mul(math.NewIntFromUint64(multiplier))
	}

	if !rate.IsUint64() {
		return 0, types.ErrInsufficientBandwidth
	}
	return rate.Uint64(), nil
}
