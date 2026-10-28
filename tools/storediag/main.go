// Command storediag answers the Step-2 forensic questions directly against
// a real data directory, without needing a full node or CometBFT:
//
//  1. which root version rootmulti's top-level CommitInfo says is latest
//  2. what CommitInfo (StoreInfos / per-store CommitID) is recorded at that
//     version
//  3. for each mounted store, whether that store's own IAVL nodedb actually
//     has that version on disk
//
// STOP THE NODE before running it — goleveldb takes an exclusive lock on the
// data dir and will fail to open if xpexd is still running against it.
//
// IMPORTANT: LoadStore may perform an IAVL storage upgrade/migration when it
// detects an upgradeable store. Run this only against a disposable test DB or
// a backup when doing forensic diagnostics.
//
// The rootmulti/IAVL APIs used here (GetCommitInfo, GetAllVersions,
// VersionExists) are taken from the exact SDK/IAVL versions installed in this
// repository.
//
// Usage:
//
//	go run ./tools/storediag --home /path/to/node/home
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"cosmossdk.io/log/v2"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/store/v2/iavl"
	"github.com/cosmos/cosmos-sdk/store/v2/rootmulti"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
)

// Hardcoded to match the exact list in app.go's storetypes.NewKVStoreKeys(...)
// call. These are the literal StoreKey string values (module name, not
// package name -- e.g. auth's is "acc", not "auth"), not something looked
// up dynamically, to keep this tool's own dependency graph minimal and
// independent of the rest of the app compiling. Update this list if you
// add/remove/rename a store in app.go.
var storeKeyNames = []string{
	"acc",          // authtypes.StoreKey
	"bank",         // banktypes.StoreKey
	"staking",      // stakingtypes.StoreKey
	"distribution", // distrtypes.StoreKey
	"slashing",     // slashingtypes.StoreKey
	"gov",          // govtypes.StoreKey
	"consensus",    // consensusparamtypes.StoreKey
	"counter",      // countertypes.StoreKey
	"dex",          // dextypes.StoreKey
}

func main() {
	home := flag.String("home", "", "node home directory (the one containing data/application.db)")
	flag.Parse()

	if *home == "" {
		fmt.Fprintln(os.Stderr, "usage: storediag --home /path/to/node/home")
		os.Exit(2)
	}

	dataDir := filepath.Join(*home, "data")

	db, err := dbm.NewDB("application", dbm.GoLevelDBBackend, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open application.db (is the node still running against this home?): %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	logger := log.NewLogger(os.Stdout)

	// Mount exactly the same store keys the real app mounts, in the same
	// way, WITHOUT calling LoadLatestVersion — so a broken store doesn't
	// panic before we get to inspect it.
	keys := storetypes.NewKVStoreKeys(storeKeyNames...)

	rs := rootmulti.NewStore(db, logger)
	for _, key := range keys {
		rs.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	}

	latest := rs.LatestVersion()
	fmt.Printf("== rootmulti.LatestVersion() (db.LastVersion / \"s/latest\") == %d\n\n", latest)

	info, err := rs.GetCommitInfo(latest)
	if err != nil {
		fmt.Printf("GetCommitInfo(%d) FAILED: %v\n", latest, err)
		fmt.Println("This alone is notable: the top-level \"s/<version>\" CommitInfo record for the version LastVersion() points to is missing or unreadable.")
		os.Exit(1)
	}

	fmt.Printf("CommitInfo.Version = %d\n", info.Version)
	fmt.Println("StoreInfos:")

	names := make([]string, 0, len(keys))
	for name := range keys {
		names = append(names, name)
	}
	sort.Strings(names)

	storeVersion := map[string]int64{}
	for _, si := range info.StoreInfos {
		storeVersion[si.Name] = si.CommitId.Version
		fmt.Printf("  - %-14s CommitID.Version=%-6d Hash=%x\n", si.Name, si.CommitId.Version, si.CommitId.Hash)
	}

	fmt.Println()

	fmt.Println("== Raw IAVL key inspection ==")
	fmt.Println("Inspecting keys beginning with ASCII 's'.")

	iter, err := db.Iterator([]byte("s"), []byte("t"))
	if err != nil {
		panic(fmt.Sprintf("opening IAVL key iterator: %v", err))
	}
	defer iter.Close()

	count := 0
	for ; iter.Valid() && count < 120; iter.Next() {
		key := append([]byte(nil), iter.Key()...)
		value := iter.Value()

		fmt.Printf("  key_len=%-3d key=%x value_len=%d\n", len(key), key, len(value))
		count++
	}

	if err := iter.Error(); err != nil {
		fmt.Printf("  iterator error: %v\n", err)
	}

	fmt.Printf("  displayed=%d\n\n", count)

	fmt.Println("== Per-store on-disk IAVL check ==")
	fmt.Println("For each store, does its own nodedb actually have the version recorded above?")
	fmt.Println()

	anyMismatch := false
	for _, name := range names {
		recordedVersion, ok := storeVersion[name]
		if !ok {
			fmt.Printf("  %-14s NOT PRESENT in CommitInfo.StoreInfos at all (added after version %d and not yet upgraded?)\n", name, latest)
			continue
		}

		idx := -1
		for i, si := range info.StoreInfos {
			if si.Name == name {
				idx = i
				break
			}
		}
		if idx == -1 {
			fmt.Printf("  %-14s missing StoreInfo -> FAILED\\n", name)
			anyMismatch = true
			continue
		}

		commitID := info.StoreInfos[idx].CommitId

		storeDB := dbm.NewPrefixDB(db, []byte("s/k:"+name+"/"))

		iavlStore, err := iavl.LoadStore(
			storeDB,
			logger,
			keys[name],
			commitID,
			100,
			false,
		)
		if err != nil {
			fmt.Printf("  %-14s LoadStore(version=%d) FAILED: %v\\n", name, recordedVersion, err)
			anyMismatch = true
			continue
		}

		concreteStore, ok := iavlStore.(*iavl.Store)
		if !ok {
			fmt.Printf("  %-14s unexpected store type %T -> FAILED\\n", name, iavlStore)
			anyMismatch = true
			continue
		}

		versions := concreteStore.GetAllVersions()
		hasRecorded := concreteStore.VersionExists(recordedVersion)

		status := "OK"
		if !hasRecorded {
			status = "*** MISSING ON DISK ***"
			anyMismatch = true
		}
		maxAvail := int64(-1)
		firstAvail := int64(-1)
		if len(versions) > 0 {
			firstAvail = int64(versions[0])
			maxAvail = int64(versions[len(versions)-1])
		}

		fmt.Printf("  %-14s recorded=%-6d first=%-6d last=%-6d count=%-6d contains_recorded=%t -> %s\n",
			name, recordedVersion, firstAvail, maxAvail, len(versions), hasRecorded, status)
	}

	fmt.Println()
	if anyMismatch {
		fmt.Println("=> Found at least one store whose CommitInfo-recorded version has no matching IAVL nodes on disk.")
		fmt.Println("   That store is your root cause. Check whether it lines up with async pruning (SetIAVLSyncPruning)")
		fmt.Println("   or with how the process was actually stopped (graceful vs. killed) before this snapshot was taken.")
	} else {
		fmt.Println("=> All mounted stores have their recorded version present on disk. If LoadLatestVersion still panics,")
		fmt.Println("   the mismatch is elsewhere (e.g. the top-level CommitInfo vs. what LastVersion() returns) -- rerun")
		fmt.Println("   this after reproducing the panic once more and compare `latest` here against the panic's version.")
	}
}
