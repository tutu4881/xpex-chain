package example

import sdk "github.com/cosmos/cosmos-sdk/types"

// Address prefixes of the XPEX chain. They were "cosmos" (the Cosmos Hub prefix) on xpex-1, which makes
// wallets and explorers confuse XPEX addresses with Cosmos Hub ones; xpex-2 uses its own prefix.
const (
	Bech32Prefix         = "xpex"
	Bech32PrefixAccPub   = Bech32Prefix + "pub"
	Bech32PrefixValAddr  = Bech32Prefix + "valoper"
	Bech32PrefixValPub   = Bech32Prefix + "valoperpub"
	Bech32PrefixConsAddr = Bech32Prefix + "valcons"
	Bech32PrefixConsPub  = Bech32Prefix + "valconspub"
)

// init runs before anything else in this package and before the root command seals the SDK config,
// so every codec created from sdk.GetConfig() (see app.go) uses the XPEX prefixes.
func init() {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(Bech32Prefix, Bech32PrefixAccPub)
	cfg.SetBech32PrefixForValidator(Bech32PrefixValAddr, Bech32PrefixValPub)
	cfg.SetBech32PrefixForConsensusNode(Bech32PrefixConsAddr, Bech32PrefixConsPub)
}
