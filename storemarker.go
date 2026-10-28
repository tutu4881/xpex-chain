package example

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/feegrant"

	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
)

// A store that has never held a single key cannot be loaded at a past or current version with the
// IAVL version used here: its "empty root" record reads back as missing, so loading it fails with
// "version does not exist". Every gRPC/REST query branches ALL stores at the query height, so one
// such store breaks every REST call on the node (seen on xpex-2 with authz, feegrant and evidence,
// which stay empty until someone grants, allows or double-signs).
//
// The fix is to make sure these stores are never empty: one marker key under the 0xFF prefix, which
// none of these modules use (their data lives under 0x00..0x02), so their own iterators, queries and
// genesis export never see it.
var nonEmptyMarkerKey = []byte("\xffxpex/non-empty")

// storesThatCanStayEmpty lists the stores that hold nothing until a user acts.
var storesThatCanStayEmpty = []string{
	authzkeeper.StoreKey,
	feegrant.StoreKey,
	evidencetypes.StoreKey,
}

// ensureStoresNotEmpty writes the marker into each listed store when it is missing. It runs in InitChainer,
// so the stores are never empty at any committed version, and again in EndBlocker as a cheap safety net
// (three reads per block). A data directory that already committed versions with an empty store cannot
// be repaired by this: the node fails to load it ("failed to load store: version does not exist") and
// the chain has to be restarted from genesis.
func (app *ExampleApp) ensureStoresNotEmpty(ctx sdk.Context) {
	for _, name := range storesThatCanStayEmpty {
		key := app.GetKey(name)
		if key == nil {
			continue
		}
		store := ctx.KVStore(key)
		if !store.Has(nonEmptyMarkerKey) {
			store.Set(nonEmptyMarkerKey, []byte{1})
		}
	}
}
