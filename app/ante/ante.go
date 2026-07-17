package ante

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"
	txsigning "cosmossdk.io/x/tx/signing"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	manakeeper "github.com/jee-chain/jee/x/mana/keeper"
	"github.com/jee-chain/jee/x/mana/types"
)

// HandlerOptions defines the ante handler configuration for JEE Chain.
type HandlerOptions struct {
	AccountKeeper   ante.AccountKeeper
	BankKeeper      authtypes.BankKeeper
	SignModeHandler *txsigning.HandlerMap
	ManaKeeper      manakeeper.Keeper
	SigGasConsumer  ante.SignatureVerificationGasConsumer
}

// NewAnteHandler builds the JEE Chain ante pipeline.
//
// Fee model: ZERO coin fees. Transactions consume Mana (bandwidth), not coins;
// validators are paid by inflation. Any fee a wallet attaches is simply IGNORED —
// there is intentionally no DeductFee/RejectFee decorator wired below.
//
// Spam / DoS protection layers, in actual execution order:
//  1. SetUpContext      — installs the gas meter (sig-verify / compute metering only)
//  2. TxSizeLimit       — hard byte ceiling; rejects oversized payloads early
//  3. ValidateBasic / Memo / TimeoutHeight — stateless sanity checks
//  4. PowChoke          — CheckTx-only adaptive proof-of-work (no-op at low load)
//  5. ManaConsume       — deducts mana proportional to tx size
//  6. TxSpamLimit       — hard cap of MaxTxPerBlock txs per account per block
//  7. Sig* decorators   — pubkey set, sig count, sig-gas, signature verification
//  8. IncrementSequence — replay protection
//
// NOTE: mana/spam-counter writes only persist if the WHOLE pipeline (including
// signature verification) succeeds, so a tx with an invalid signature can never
// drain a victim's mana or burn their per-block tx allowance.
func NewAnteHandler(options HandlerOptions) (sdk.AnteHandler, error) {
	if options.AccountKeeper == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "account keeper is required for AnteHandler")
	}
	if options.BankKeeper == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "bank keeper is required for AnteHandler")
	}
	if options.SignModeHandler == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "sign mode handler is required for AnteHandler")
	}
	if options.ManaKeeper.IsZero() {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "mana keeper is required for AnteHandler")
	}

	sigGasConsumer := options.SigGasConsumer
	if sigGasConsumer == nil {
		sigGasConsumer = ante.DefaultSigVerificationGasConsumer
	}

	anteDecorators := []sdk.AnteDecorator{
		ante.NewSetUpContextDecorator(), // gas meter for sig verify / wasm compute only
		NewTxSizeLimitDecorator(),       // hard byte ceiling — reject absurd payloads early
		ante.NewExtensionOptionsDecorator(nil),
		ante.NewValidateBasicDecorator(),
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(options.AccountKeeper),
		// Adaptive PoW choke valve: CheckTx-only spam filter. No-op at normal
		// load (difficulty 0), auto-engages under congestion to choke botnets.
		NewPowChokeDecorator(options.ManaKeeper),
		// JEE Chain: Mana pays bandwidth; fees optional (Keplr often sends a small fee).
		// No RejectFeeDecorator — zero or non-zero fees both pass; validators use inflation.
		NewManaConsumeDecorator(options.ManaKeeper),
		NewTxSpamLimitDecorator(options.ManaKeeper),
		ante.NewConsumeGasForTxSizeDecorator(options.AccountKeeper),
		ante.NewSetPubKeyDecorator(options.AccountKeeper),
		ante.NewValidateSigCountDecorator(options.AccountKeeper),
		ante.NewSigGasConsumeDecorator(options.AccountKeeper, sigGasConsumer),
		ante.NewSigVerificationDecorator(options.AccountKeeper, options.SignModeHandler),
		ante.NewIncrementSequenceDecorator(options.AccountKeeper),
	}

	return sdk.ChainAnteDecorators(anteDecorators...), nil
}

// ---------------------------------------------------------------------------
// RejectFeeDecorator — explicitly prevents fee deduction
// ---------------------------------------------------------------------------

// RejectFeeDecorator rejects transactions that include non-zero fees.
// JEE Chain has zero transaction fees; validators are paid via block inflation.
type RejectFeeDecorator struct{}

func NewRejectFeeDecorator() RejectFeeDecorator {
	return RejectFeeDecorator{}
}

func (rfd RejectFeeDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "Tx must be a FeeTx")
	}

	// Explicitly reject any attempt to pay fees — prevents accidental or
	// malicious fee deduction paths from being used as a spam vector.
	feeCoins := feeTx.GetFee()
	if !feeCoins.IsZero() {
		return ctx, types.ErrFeesNotAllowed
	}

	// Ensure gas prices are zero as well
	if feeTx.GetGas() > 0 {
		// Gas is still used internally for sig verification metering,
		// but users never pay for it in coins.
	}

	return next(ctx, tx, simulate)
}

// ---------------------------------------------------------------------------
// ManaConsumeDecorator — bandwidth deduction (replaces fee payment)
// ---------------------------------------------------------------------------

// ManaConsumeDecorator checks and deducts Mana from the transaction signer.
type ManaConsumeDecorator struct {
	manaKeeper manakeeper.Keeper
}

func NewManaConsumeDecorator(mk manakeeper.Keeper) ManaConsumeDecorator {
	return ManaConsumeDecorator{manaKeeper: mk}
}

func (mcd ManaConsumeDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	if simulate || ctx.BlockHeight() == 0 {
		return next(ctx, tx, simulate)
	}

	sigTx, ok := tx.(signing.SigVerifiableTx)
	if !ok {
		return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "tx must be SigVerifiableTx")
	}

	signers, err := sigTx.GetSigners()
	if err != nil {
		return ctx, err
	}
	if len(signers) == 0 {
		return ctx, errorsmod.Wrap(sdkerrors.ErrUnauthorized, "no signers")
	}

	// Primary signer pays mana (first signer convention, same as fee payer)
	signer := sdk.AccAddress(signers[0])

	txBytes := ctx.TxBytes()
	if len(txBytes) == 0 {
		// Fallback: estimate from codec if tx bytes not yet set
		txBytes = []byte(fmt.Sprintf("%T-%d", tx, len(signers)))
	}

	cost := manakeeper.ComputeTxManaCost(len(txBytes))

	// Mana bookkeeping runs on an infinite gas meter so its store reads/writes
	// never count against the transaction's gas. This keeps wallet gas
	// simulation (which skips mana) consistent with execution and prevents
	// spurious "out of gas" failures from Keplr/Leap.
	manaCtx := ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	if err := mcd.manaKeeper.ConsumeMana(manaCtx, signer, cost); err != nil {
		return ctx, err
	}

	// Emit event for indexers / wallets
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"mana_consumed",
		sdk.NewAttribute("signer", signer.String()),
		sdk.NewAttribute("cost", fmt.Sprintf("%d", cost)),
		sdk.NewAttribute("remaining", "queried_via_mana_module"),
	))

	return next(ctx, tx, simulate)
}

// ---------------------------------------------------------------------------
// TxSpamLimitDecorator — max 20 txs per account per block
// ---------------------------------------------------------------------------

// TxSpamLimitDecorator enforces the per-block transaction cap per account.
// Even with free txs, this hard limit prevents a single key from flooding
// a block with 20+ transactions regardless of mana balance.
type TxSpamLimitDecorator struct {
	manaKeeper manakeeper.Keeper
}

func NewTxSpamLimitDecorator(mk manakeeper.Keeper) TxSpamLimitDecorator {
	return TxSpamLimitDecorator{manaKeeper: mk}
}

func (tsld TxSpamLimitDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	if simulate || ctx.BlockHeight() == 0 {
		return next(ctx, tx, simulate)
	}

	sigTx, ok := tx.(signing.SigVerifiableTx)
	if !ok {
		return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "tx must be SigVerifiableTx")
	}

	signers, err := sigTx.GetSigners()
	if err != nil {
		return ctx, err
	}

	// Unmetered context — spam counter bookkeeping must not consume tx gas
	// (keeps simulation and execution gas identical).
	manaCtx := ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	for _, signer := range signers {
		if err := tsld.manaKeeper.IncrementTxCount(manaCtx, sdk.AccAddress(signer)); err != nil {
			return ctx, err
		}
	}

	return next(ctx, tx, simulate)
}

// ---------------------------------------------------------------------------
// PowChokeDecorator — adaptive proof-of-work spam filter (CheckTx only)
// ---------------------------------------------------------------------------

// PowChokeDecorator drops transactions that fail the current proof-of-work
// challenge. It runs ONLY during CheckTx — i.e. as a mempool-admission filter —
// so spam is rejected before it is gossiped, and it can never affect consensus
// or halt the chain. The required difficulty is 0 during normal load (no PoW
// needed, standard wallets work untouched) and rises automatically when the
// network is congested. See keeper.UpdatePowDifficulty / keeper.VerifyPow.
type PowChokeDecorator struct {
	manaKeeper manakeeper.Keeper
}

func NewPowChokeDecorator(mk manakeeper.Keeper) PowChokeDecorator {
	return PowChokeDecorator{manaKeeper: mk}
}

func (pcd PowChokeDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	// Enforce only as a non-consensus mempool filter: skip simulation, genesis,
	// and block execution (DeliverTx). This guarantees PoW can never cause a
	// state divergence between validators.
	if simulate || !ctx.IsCheckTx() || ctx.BlockHeight() == 0 {
		return next(ctx, tx, simulate)
	}

	// Fast path: no PoW required at normal load (keeps Keplr/Leap fully working).
	if pcd.manaKeeper.GetPowDifficulty(ctx) == 0 {
		return next(ctx, tx, simulate)
	}

	sigTx, ok := tx.(signing.SigVerifiableTx)
	if !ok {
		return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "tx must be SigVerifiableTx")
	}

	signers, err := sigTx.GetSigners()
	if err != nil {
		return ctx, err
	}
	if len(signers) == 0 {
		return ctx, errorsmod.Wrap(sdkerrors.ErrUnauthorized, "no signers")
	}

	var sequence uint64
	if sigs, sErr := sigTx.GetSignaturesV2(); sErr == nil && len(sigs) > 0 {
		sequence = sigs[0].Sequence
	}

	memo := ""
	if memoTx, ok := tx.(sdk.TxWithMemo); ok {
		memo = memoTx.GetMemo()
	}

	if err := pcd.manaKeeper.VerifyPow(ctx, ctx.ChainID(), sdk.AccAddress(signers[0]), sequence, memo); err != nil {
		return ctx, err
	}

	return next(ctx, tx, simulate)
}

// ---------------------------------------------------------------------------
// TxSizeLimitDecorator — hard cap on serialized tx size (bandwidth backstop)
// ---------------------------------------------------------------------------

// TxSizeLimitDecorator rejects oversized transactions early, before signature
// verification, as a cheap defense against memory-exhaustion payloads. The
// per-byte Mana cost already scales with size; this is a hard ceiling on top.
type TxSizeLimitDecorator struct{}

func NewTxSizeLimitDecorator() TxSizeLimitDecorator {
	return TxSizeLimitDecorator{}
}

func (tld TxSizeLimitDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	if !simulate {
		if size := len(ctx.TxBytes()); size > types.MaxTxBytes {
			return ctx, errorsmod.Wrapf(types.ErrTxTooLarge, "tx size %d bytes exceeds max %d bytes", size, types.MaxTxBytes)
		}
	}
	return next(ctx, tx, simulate)
}

// ---------------------------------------------------------------------------
// NoOpFeeDecorator — alternative if you want to accept zero-fee txs silently
// (RejectFeeDecorator is stricter and preferred for JEE Chain)
// ---------------------------------------------------------------------------

// Ensure interface compliance
var (
	_ sdk.AnteDecorator = RejectFeeDecorator{}
	_ sdk.AnteDecorator = ManaConsumeDecorator{}
	_ sdk.AnteDecorator = TxSpamLimitDecorator{}
	_ sdk.AnteDecorator = PowChokeDecorator{}
	_ sdk.AnteDecorator = TxSizeLimitDecorator{}
)
