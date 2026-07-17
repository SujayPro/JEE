package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"math/bits"

	"cosmossdk.io/log"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/jee-chain/jee/x/mana/types"
)

// maxUint64 is the largest uint64 value, used for saturating arithmetic.
const maxUint64 = ^uint64(0)

// ViewBankKeeper is the minimal, read-only view of x/bank that the mana module
// requires. Restricting the dependency to these two methods makes the mana
// module structurally incapable of minting, burning, or transferring JEE — even
// if future code (or an external contributor) tried to add such a call. Token
// creation remains exclusively the responsibility of x/mint.
type ViewBankKeeper interface {
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	GetSupply(ctx context.Context, denom string) sdk.Coin
}

// Keeper manages Mana (bandwidth) state.
type Keeper struct {
	cdc        codec.BinaryCodec
	storeKey   storetypes.StoreKey
	bankKeeper ViewBankKeeper
	authority  string
	logger     log.Logger
}

// NewKeeper creates a new mana Keeper.
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	bankKeeper ViewBankKeeper,
	authority string,
) Keeper {
	return Keeper{
		cdc:        cdc,
		storeKey:   storeKey,
		bankKeeper: bankKeeper,
		authority:  authority,
	}
}

// IsZero reports whether the keeper was never initialized.
func (k Keeper) IsZero() bool {
	return k.storeKey == nil
}

// SetLogger attaches a logger (called from app wiring).
func (k *Keeper) SetLogger(l log.Logger) {
	k.logger = l
}

// GetParams returns module params.
func (k Keeper) GetParams(ctx context.Context) types.Params {
	store := prefix.NewStore(k.storeService(ctx), []byte("params"))
	bz := store.Get([]byte("params"))
	if bz == nil {
		return types.DefaultParams()
	}
	var params types.Params
	if err := json.Unmarshal(bz, &params); err != nil {
		return types.DefaultParams()
	}
	return params
}

// SetParams sets module params (governance only).
func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}
	store := prefix.NewStore(k.storeService(ctx), []byte("params"))
	bz, err := json.Marshal(params)
	if err != nil {
		return err
	}
	store.Set([]byte("params"), bz)
	return nil
}

// storeService returns the SDK context store adapter.
func (k Keeper) storeService(ctx context.Context) storetypes.KVStore {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.KVStore(k.storeKey)
}

// GetAccountMana returns the current (regenerated) mana state for an address.
//
// This is a pure read: it computes regeneration on the fly but never writes to
// the store. Persistence happens only in the explicit write paths (ConsumeMana,
// IncrementTxCount). Keeping reads side-effect free makes query handlers safe
// and avoids mutating consensus state from a read path.
func (k Keeper) GetAccountMana(ctx context.Context, addr sdk.AccAddress) (types.AccountMana, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	acct, err := k.loadAccountMana(ctx, addr)
	if err != nil {
		return acct, err
	}

	return k.regenerateMana(ctx, acct, params, sdkCtx.BlockHeight())
}

// loadAccountMana reads the raw stored mana record for an address without
// applying regeneration. First-time accounts start at height 0 so newly funded
// wallets accrue regeneration since genesis and can transact immediately.
//
// NOTE: there is intentionally no "reset LastUpdateHeight to 0 when mana hits 0"
// recovery path here. That shortcut allowed any funded account to instantly
// refill its mana to the cap simply by draining to exactly 0, defeating rate
// limiting. Drained accounts now recover gradually via the per-block regen floor.
func (k Keeper) loadAccountMana(ctx context.Context, addr sdk.AccAddress) (types.AccountMana, error) {
	store := prefix.NewStore(k.storeService(ctx), []byte("account/"))
	bz := store.Get(addr.Bytes())
	if bz == nil {
		return types.AccountMana{
			Address:          addr.String(),
			Mana:             0,
			LastUpdateHeight: 0,
		}, nil
	}

	var acct types.AccountMana
	if err := json.Unmarshal(bz, &acct); err != nil {
		return types.AccountMana{}, err
	}
	return acct, nil
}

// regenerateMana applies the core formula:
//
//	Mana_Regen_Rate = (Balance * Total_Mana_Pool) / Total_Supply
//
// Mana gained per block = Mana_Regen_Rate / BlocksPerYear * blocks_elapsed
//
// Holding more JEE Money yields proportionally more bandwidth over time.
// This replaces gas fees: users "pay" with time-regenerated mana, not coins.
func (k Keeper) regenerateMana(
	ctx context.Context,
	acct types.AccountMana,
	params types.Params,
	currentHeight int64,
) (types.AccountMana, error) {
	if currentHeight <= acct.LastUpdateHeight {
		return acct, nil
	}

	blocksElapsed := uint64(currentHeight - acct.LastUpdateHeight)
	if blocksElapsed == 0 {
		return acct, nil
	}

	balance := k.bankKeeper.GetBalance(sdk.UnwrapSDKContext(ctx), mustAccAddr(acct.Address), types.BondDenom)
	totalSupply := k.bankKeeper.GetSupply(sdk.UnwrapSDKContext(ctx), types.BondDenom)

	// Balance-proportional rate (heavy holders earn more bandwidth).
	// rate = (balance * totalManaPool) / totalSupply — safe math, no uint64 overflow.
	var rate uint64
	if !totalSupply.Amount.IsZero() && !balance.Amount.IsZero() {
		r, err := computeManaRate(balance.Amount, totalSupply.Amount, params.TotalManaPool, params.ManaRegenMultiplier)
		if err != nil {
			return acct, err
		}
		rate = r
	}

	// Every account gets a minimum free bandwidth allowance so small/new
	// holders can always transact. Heavy holders exceed the floor via rate.
	maxMana := rate
	if maxMana < types.MinManaFloor {
		maxMana = types.MinManaFloor
	}

	// Per-block regen: balance-based, but at least enough to refill the free
	// allowance within ManaRegenWindowBlocks. Guard against a zero
	// BlocksPerYear (defensive — params are validated, but division by zero
	// would panic the whole chain).
	blocksPerYear := params.BlocksPerYear
	if blocksPerYear == 0 {
		blocksPerYear = types.DefaultBlocksPerYear
	}

	perBlock := rate / blocksPerYear
	minPerBlock := types.MinManaFloor / types.ManaRegenWindowBlocks
	if perBlock < minPerBlock {
		perBlock = minPerBlock
	}

	acct.Mana = addManaSaturating(acct.Mana, perBlock, blocksElapsed, maxMana)
	acct.LastUpdateHeight = currentHeight
	return acct, nil
}

// addManaSaturating returns min(current + perBlock*blocksElapsed, maxMana),
// computed without uint64 overflow. A long-dormant account can have a very
// large blocksElapsed, so the naive perBlock*blocksElapsed (and the subsequent
// addition) could overflow uint64 and wrap to a bogus small value. Since mana is
// always capped at maxMana, any overflow simply saturates to the cap instead.
func addManaSaturating(current, perBlock, blocksElapsed, maxMana uint64) uint64 {
	if current >= maxMana {
		return maxMana
	}

	var gained uint64
	if perBlock != 0 && blocksElapsed > maxUint64/perBlock {
		gained = maxUint64
	} else {
		gained = perBlock * blocksElapsed
	}

	// Headroom (maxMana-current) is safe here because current < maxMana.
	if gained > maxMana-current {
		return maxMana
	}
	return current + gained
}

// ConsumeMana deducts mana for a transaction. Returns ErrInsufficientBandwidth if short.
func (k Keeper) ConsumeMana(ctx context.Context, addr sdk.AccAddress, cost uint64) error {
	acct, err := k.GetAccountMana(ctx, addr)
	if err != nil {
		return err
	}

	if acct.Mana < cost {
		return types.ErrInsufficientBandwidth
	}

	acct.Mana -= cost
	store := prefix.NewStore(k.storeService(ctx), []byte("account/"))
	k.setAccountMana(store, addr.Bytes(), acct)
	return nil
}

// IncrementTxCount enforces max 20 transactions per account per block (spam protection).
func (k Keeper) IncrementTxCount(ctx context.Context, addr sdk.AccAddress) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(ctx)

	acct, err := k.GetAccountMana(ctx, addr)
	if err != nil {
		return err
	}

	if acct.LastTxBlock != sdkCtx.BlockHeight() {
		acct.TxCountThisBlock = 0
		acct.LastTxBlock = sdkCtx.BlockHeight()
	}

	if uint64(acct.TxCountThisBlock) >= params.MaxTxPerBlock {
		return types.ErrTxSpamLimitExceeded
	}

	acct.TxCountThisBlock++
	store := prefix.NewStore(k.storeService(ctx), []byte("account/"))
	k.setAccountMana(store, addr.Bytes(), acct)
	return nil
}

// ComputeTxManaCost calculates mana cost from serialized tx size.
func ComputeTxManaCost(txSize int) uint64 {
	if txSize <= 0 {
		return types.DefaultManaBaseCost
	}
	return types.DefaultManaBaseCost + uint64(txSize)*types.DefaultManaCostPerByte
}

func (k Keeper) setAccountMana(store storetypes.KVStore, key []byte, acct types.AccountMana) {
	bz, err := json.Marshal(acct)
	if err != nil {
		panic(err)
	}
	store.Set(key, bz)
}

func mustAccAddr(addr string) sdk.AccAddress {
	a, err := sdk.AccAddressFromBech32(addr)
	if err != nil {
		panic(err)
	}
	return a
}

// BlockTxCountKey returns a per-block global counter key (optional observability).
func BlockTxCountKey(height int64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(height))
	return append([]byte("blocktx/"), bz...)
}

// ---------------------------------------------------------------------------
// Adaptive Proof-of-Work choke valve (anti-spam)
//
// The required PoW difficulty (leading zero bits) is a single value stored in
// module state. It is recomputed once per block in UpdatePowDifficulty from the
// number of transactions in that block — a value every validator agrees on — so
// the difficulty is fully deterministic. Verification (VerifyPow) is used by the
// ante handler purely as a CheckTx mempool-admission filter; it never runs as a
// consensus rule, so it can never cause a chain halt.
// ---------------------------------------------------------------------------

var powDifficultyKey = []byte("difficulty")

func powStore(store storetypes.KVStore) prefix.Store {
	return prefix.NewStore(store, []byte("pow/"))
}

// GetPowDifficulty returns the current required PoW difficulty in leading zero
// bits. Zero means no PoW is required (normal, calm-network operation).
func (k Keeper) GetPowDifficulty(ctx context.Context) uint32 {
	store := powStore(k.storeService(ctx))
	bz := store.Get(powDifficultyKey)
	if len(bz) < 4 {
		return 0
	}
	return binary.BigEndian.Uint32(bz)
}

func (k Keeper) setPowDifficulty(ctx context.Context, difficulty uint32) {
	store := powStore(k.storeService(ctx))
	bz := make([]byte, 4)
	binary.BigEndian.PutUint32(bz, difficulty)
	store.Set(powDifficultyKey, bz)
}

// UpdatePowDifficulty adjusts the stored PoW difficulty based on how busy the
// just-proposed block is. It is called from the app PreBlocker with
// len(req.Txs), which is identical on every node, keeping the result
// deterministic. Difficulty rises one bit per congested block and decays one
// bit per calm block, clamped to [0, PowMaxDifficulty].
func (k Keeper) UpdatePowDifficulty(ctx context.Context, blockTxCount uint64) {
	difficulty := k.GetPowDifficulty(ctx)

	switch {
	case blockTxCount > types.PowHighWatermarkTxPerBlock && difficulty < types.PowMaxDifficulty:
		difficulty++
	case blockTxCount < types.PowLowWatermarkTxPerBlock && difficulty > 0:
		difficulty--
	default:
		return // no change; avoid a redundant store write
	}

	k.setPowDifficulty(ctx, difficulty)
}

// VerifyPow checks that the transaction satisfies the current PoW difficulty.
// At difficulty 0 it is a no-op (so standard wallets work untouched). The puzzle
// binds the work to (chainID, signer, sequence) so a solved nonce cannot be
// replayed across chains, accounts, or transactions.
func (k Keeper) VerifyPow(ctx context.Context, chainID string, signer sdk.AccAddress, sequence uint64, memo string) error {
	difficulty := k.GetPowDifficulty(ctx)
	if difficulty == 0 {
		return nil
	}

	challenge := powChallenge(chainID, signer.Bytes(), sequence)
	work := powWork(challenge, memo)
	if leadingZeroBits(work) < difficulty {
		return types.ErrInsufficientPow
	}
	return nil
}

// powChallenge derives the per-(account,sequence) puzzle seed. The wallet must
// find a memo whose work hash has enough leading zero bits.
func powChallenge(chainID string, signer []byte, sequence uint64) []byte {
	buf := make([]byte, 0, len(chainID)+len(signer)+8)
	buf = append(buf, []byte(chainID)...)
	buf = append(buf, signer...)
	var seq [8]byte
	binary.BigEndian.PutUint64(seq[:], sequence)
	buf = append(buf, seq[:]...)
	sum := sha256.Sum256(buf)
	return sum[:]
}

// powWork hashes the challenge with the candidate nonce (the tx memo).
func powWork(challenge []byte, memo string) []byte {
	buf := make([]byte, 0, len(challenge)+len(memo))
	buf = append(buf, challenge...)
	buf = append(buf, []byte(memo)...)
	sum := sha256.Sum256(buf)
	return sum[:]
}

// leadingZeroBits counts the number of leading zero bits in b.
func leadingZeroBits(b []byte) uint32 {
	var count uint32
	for _, by := range b {
		if by == 0 {
			count += 8
			continue
		}
		count += uint32(bits.LeadingZeros8(by))
		break
	}
	return count
}
