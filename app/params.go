package app

const (
	// AppName is the application name.
	AppName = "jeechain"

	// Bech32MainPrefix is the Bech32 prefix for account addresses.
	Bech32MainPrefix = "jee"

	// BondDenom is the smallest on-chain unit (1 JEE = 1_000_000 jeff).
	BondDenom = "jeff"

	// DisplayDenom is the human-readable token symbol.
	DisplayDenom = "JEE"

	// ManaCostPerByte is the base mana cost per byte of serialized tx.
	ManaCostPerByte = 1

	// ManaBaseCost is the flat mana cost applied to every transaction.
	ManaBaseCost = 100

	// TargetBlockTimeMs is the target block time in milliseconds (1 second).
	TargetBlockTimeMs = 1000

	// LogoURL is the publicly hosted JEE Money / JEE Chain logo (GitHub Gist).
	// Source: https://gist.github.com/SujayPro/b866414969ef061973af469ebc01b38e
	LogoURL = "https://gist.githubusercontent.com/SujayPro/b866414969ef061973af469ebc01b38e/raw/5a9f0104eadab169db5ea3b8f959f6d9c8218a/gistfile1.svg"
)
