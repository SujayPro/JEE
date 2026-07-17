package types

import (
	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	ModuleName = "mana"
	StoreKey   = ModuleName

	// BondDenom is the native token base denomination used for mana regen lookups.
	BondDenom = "jeff"

	// Default parameter values.
	DefaultTotalManaPool       uint64 = 1_000_000_000
	DefaultMaxTxPerBlock       uint64 = 20
	DefaultBlocksPerYear       uint64 = 31_557_600
	DefaultInflationRate       string = "0.05"
	DefaultManaRegenMultiplier uint64 = 1
	DefaultManaBaseCost        uint64 = 100
	DefaultManaCostPerByte     uint64 = 1

	// MinManaFloor is the free bandwidth allowance every account receives,
	// independent of token balance. Ensures small/new holders can always
	// perform basic transactions. Heavy holders get proportionally more.
	MinManaFloor uint64 = 1_000_000

	// ManaRegenWindowBlocks is how many blocks it takes to refill the free
	// allowance from empty (~ a few minutes at 1s blocks).
	ManaRegenWindowBlocks uint64 = 1_000

	// MaxTxBytes is a hard upper bound on the serialized size of a single
	// transaction, enforced in the ante handler as a cheap bandwidth backstop
	// (rejected before signature verification). The per-byte Mana cost already
	// scales with size; this simply caps absurd payloads early.
	MaxTxBytes int = 1 << 20 // 1 MiB

	// --- Adaptive Proof-of-Work (anti-spam choke valve) ---
	//
	// PoW is a non-consensus, CheckTx-only mempool-admission filter. The
	// required difficulty (leading zero bits) is stored deterministically in
	// module state and recomputed every block from that block's transaction
	// count. At normal load the difficulty is 0, so NO PoW is required and
	// standard wallets (Keplr/Leap) are unaffected. When traffic spikes, the
	// difficulty automatically rises to choke out automated spam.

	// PowMaxDifficulty caps the required leading zero bits so legitimate clients
	// can always solve the puzzle in well under a second.
	PowMaxDifficulty uint32 = 24

	// PowHighWatermarkTxPerBlock: if a block carried more txs than this, the
	// network is considered congested and difficulty is raised by one bit.
	PowHighWatermarkTxPerBlock uint64 = 1_000

	// PowLowWatermarkTxPerBlock: if a block carried fewer txs than this, the
	// network is considered calm and difficulty is lowered by one bit.
	PowLowWatermarkTxPerBlock uint64 = 200
)

var (
	// ErrInsufficientBandwidth is returned when an account lacks mana for a tx.
	ErrInsufficientBandwidth = errors.Register(ModuleName, 1, "Insufficient Bandwidth")

	// ErrTxSpamLimitExceeded is returned when an account exceeds per-block tx limit.
	ErrTxSpamLimitExceeded = errors.Register(ModuleName, 2, "transaction spam limit exceeded: max 20 per block per account")

	// ErrFeesNotAllowed is returned when a tx attempts to pay gas fees.
	ErrFeesNotAllowed = errors.Register(ModuleName, 3, "JEE Chain does not accept transaction fees; use Mana instead")

	// ErrInsufficientPow is returned when a tx fails the adaptive proof-of-work
	// challenge during network congestion (mempool admission only).
	ErrInsufficientPow = errors.Register(ModuleName, 4, "insufficient proof-of-work: network congested, wallet must attach a valid PoW nonce")

	// ErrTxTooLarge is returned when a tx exceeds the hard MaxTxBytes ceiling.
	ErrTxTooLarge = errors.Register(ModuleName, 5, "transaction exceeds maximum allowed size")
)

// AccountMana tracks per-account mana state.
type AccountMana struct {
	Address          string `json:"address"`
	Mana             uint64 `json:"mana"`
	LastUpdateHeight int64  `json:"last_update_height"`
	TxCountThisBlock uint32 `json:"tx_count_this_block"`
	LastTxBlock      int64  `json:"last_tx_block"`
}

// Params defines the mana module parameters.
type Params struct {
	TotalManaPool       uint64 `json:"total_mana_pool"`
	MaxTxPerBlock       uint64 `json:"max_tx_per_block"`
	BlocksPerYear       uint64 `json:"blocks_per_year"`
	ManaRegenMultiplier uint64 `json:"mana_regen_multiplier"`
}

// DefaultParams returns sane defaults for mainnet-style deployment.
func DefaultParams() Params {
	return Params{
		TotalManaPool:       DefaultTotalManaPool,
		MaxTxPerBlock:       DefaultMaxTxPerBlock,
		BlocksPerYear:       DefaultBlocksPerYear,
		ManaRegenMultiplier: DefaultManaRegenMultiplier,
	}
}

// Validate validates params.
func (p Params) Validate() error {
	if p.TotalManaPool == 0 {
		return errors.Wrap(ErrInsufficientBandwidth, "total_mana_pool must be > 0")
	}
	if p.MaxTxPerBlock == 0 {
		return errors.Wrap(ErrTxSpamLimitExceeded, "max_tx_per_block must be > 0")
	}
	if p.BlocksPerYear == 0 {
		return errors.Wrap(ErrInsufficientBandwidth, "blocks_per_year must be > 0")
	}
	return nil
}

// GenesisState defines the mana module genesis.
type GenesisState struct {
	Params       Params        `json:"params"`
	AccountManas []AccountMana `json:"account_manas"`
}

// NewGenesisState creates a genesis state.
func NewGenesisState(params Params, accounts []AccountMana) GenesisState {
	return GenesisState{
		Params:       params,
		AccountManas: accounts,
	}
}

// DefaultGenesisState returns default genesis.
func DefaultGenesisState() GenesisState {
	return NewGenesisState(DefaultParams(), nil)
}

// Validate validates genesis.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	for _, acct := range gs.AccountManas {
		if _, err := sdk.AccAddressFromBech32(acct.Address); err != nil {
			return errors.Wrapf(err, "invalid mana account address %q", acct.Address)
		}
	}
	return nil
}

// ModuleAccountPermissions returns nil — mana is not a mintable module account.
func ModuleAccountPermissions() []string {
	return nil
}

// Ensure sdk.AccAddress compatibility at compile time.
var _ sdk.Address = sdk.AccAddress(nil)
