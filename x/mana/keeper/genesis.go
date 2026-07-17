package keeper

import (
	"context"
	"encoding/json"

	"cosmossdk.io/store/prefix"

	"github.com/jee-chain/jee/x/mana/types"
)

// InitGenesis initializes the mana module state from genesis.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	if err := gs.Validate(); err != nil {
		return err
	}

	if err := k.SetParams(ctx, gs.Params); err != nil {
		return err
	}

	store := prefix.NewStore(k.storeService(ctx), []byte("account/"))
	for _, acct := range gs.AccountManas {
		addr := mustAccAddr(acct.Address)
		k.setAccountMana(store, addr.Bytes(), acct)
	}
	return nil
}

// ExportGenesis exports the mana module state.
func (k Keeper) ExportGenesis(ctx context.Context) types.GenesisState {
	params := k.GetParams(ctx)

	store := prefix.NewStore(k.storeService(ctx), []byte("account/"))
	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	var accounts []types.AccountMana
	for ; iterator.Valid(); iterator.Next() {
		var acct types.AccountMana
		if err := json.Unmarshal(iterator.Value(), &acct); err != nil {
			panic(err)
		}
		accounts = append(accounts, acct)
	}

	return types.NewGenesisState(params, accounts)
}
