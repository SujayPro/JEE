package app

import (
	"os"
	"path/filepath"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	AccountAddressPrefix   = Bech32MainPrefix
	AccountPubKeyPrefix    = Bech32MainPrefix + "pub"
	ValidatorAddressPrefix = Bech32MainPrefix + "valoper"
	ValidatorPubKeyPrefix  = Bech32MainPrefix + "valoperpub"
	ConsAddressPrefix      = Bech32MainPrefix + "valcons"
	ConsPubKeyPrefix       = Bech32MainPrefix + "valconspub"
)

// DefaultNodeHome is the default directory for node data (~/.jeechain).
var DefaultNodeHome string

func init() {
	SetBech32Prefixes()

	home, err := os.UserHomeDir()
	if err != nil {
		DefaultNodeHome = ".jeechain"
		return
	}
	DefaultNodeHome = filepath.Join(home, ".jeechain")
}

// SetBech32Prefixes configures JEE Chain address prefixes (jee, jeevaloper, jeevalcons).
func SetBech32Prefixes() {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(AccountAddressPrefix, AccountPubKeyPrefix)
	cfg.SetBech32PrefixForValidator(ValidatorAddressPrefix, ValidatorPubKeyPrefix)
	cfg.SetBech32PrefixForConsensusNode(ConsAddressPrefix, ConsPubKeyPrefix)
}
