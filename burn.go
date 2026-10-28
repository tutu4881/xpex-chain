package example

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BurnModuleName is a module account with the Burner permission. It is deliberately NOT on the blocked-address
// list (see BlockedAddresses in app.go), so anyone can send coins to it with an ordinary MsgSend. At the end of
// every block, whatever it holds is burned for real: it leaves the bank module's total supply, so
// `total supply` goes down (unlike sending to an unspendable address, which only parks the coins).
//
// The address is derived from the name alone (authtypes.NewModuleAddress(BurnModuleName)); nobody holds a key
// for it. Use it for the share of AI payments (or anything else) that should be destroyed.
const BurnModuleName = "xpexburn"

// burnPending burns the whole balance of the burn module account. It is a no-op when the balance is empty.
func (app *ExampleApp) burnPending(ctx sdk.Context) error {
	addr := app.AccountKeeper.GetModuleAddress(BurnModuleName)
	coins := app.BankKeeper.GetAllBalances(ctx, addr)
	if coins.IsZero() {
		return nil
	}
	return app.BankKeeper.BurnCoins(ctx, BurnModuleName, coins)
}
