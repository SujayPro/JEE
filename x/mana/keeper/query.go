package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/jee-chain/jee/x/mana/types"
)

// QueryMana returns current mana for an address (for REST/gRPC wiring).
func (k Keeper) QueryMana(ctx context.Context, addr sdk.AccAddress) (uint64, uint64, error) {
	acct, err := k.GetAccountMana(ctx, addr)
	if err != nil {
		return 0, 0, err
	}

	params := k.GetParams(ctx)
	balance := k.bankKeeper.GetBalance(sdk.UnwrapSDKContext(ctx), addr, types.BondDenom)
	supply := k.bankKeeper.GetSupply(sdk.UnwrapSDKContext(ctx), types.BondDenom)

	var maxMana uint64
	if !supply.Amount.IsZero() && !balance.Amount.IsZero() {
		maxMana, err = computeManaRate(balance.Amount, supply.Amount, params.TotalManaPool, params.ManaRegenMultiplier)
		if err != nil {
			return 0, 0, err
		}
	}

	// Apply the universal free-bandwidth floor.
	if maxMana < types.MinManaFloor {
		maxMana = types.MinManaFloor
	}

	return acct.Mana, maxMana, nil
}
