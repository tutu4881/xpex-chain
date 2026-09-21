#!/usr/bin/env python3
"""
Build the XPEX genesis that includes the x/dex module and the chain-native
uusdt asset, starting from the validator/gentx/accounts in the current genesis.

Usage:
  python3 scripts/genesis/make_genesis.py \
      --base railway-config/config/genesis.json \
      --out  railway-config/config/genesis.json \
      --usdt-supply 1000000 \
      --genesis-time 2026-09-22T12:00:00Z

Notes
- The validator gentx is signed for chain-id "xpex-1" and is reused unchanged,
  so the chain-id MUST stay "xpex-1".
- No pool is seeded here. The initial XPX/USDT price is a business decision:
  the treasury account creates the pool with MsgCreatePool (see the DEX page
  or docs/DEX_LAUNCH.md).
- uusdt is a chain-native asset. It is NOT Tether-issued USDT.
"""
import argparse, json, sys

TREASURY = "cosmos187vl5nunwfs4rqzdnwyce8q3u25wdanejvej4n"  # validator operator / treasury


def add_coin(coins, denom, amount):
    for c in coins:
        if c["denom"] == denom:
            c["amount"] = str(int(c["amount"]) + amount)
            break
    else:
        coins.append({"denom": denom, "amount": str(amount)})
    coins.sort(key=lambda c: c["denom"])  # bank requires sorted coins


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--usdt-supply", type=int, required=True, help="whole USDT (6 decimals are applied)")
    ap.add_argument("--genesis-time", required=True)
    ap.add_argument("--treasury", default=TREASURY)
    a = ap.parse_args()

    g = json.load(open(a.base))
    if g["chain_id"] != "xpex-1":
        sys.exit("chain_id must stay xpex-1 (gentx is signed for it)")
    st = g["app_state"]
    g["genesis_time"] = a.genesis_time

    micro = a.usdt_supply * 1_000_000
    bank = st["bank"]

    # already applied? make the script idempotent
    if any(m["base"] == "uusdt" for m in bank.get("denom_metadata", [])):
        sys.exit("base genesis already contains uusdt; run against the original genesis")

    treasury = next((b for b in bank["balances"] if b["address"] == a.treasury), None)
    if treasury is None:
        sys.exit("treasury address not found in bank balances")
    add_coin(treasury["coins"], "uusdt", micro)
    add_coin(bank["supply"], "uusdt", micro)

    bank.setdefault("denom_metadata", []).append({
        "description": "USDT-denominated asset on XPEX Chain (chain-native; not issued by Tether)",
        "denom_units": [
            {"denom": "uusdt", "exponent": 0, "aliases": ["microUSDT"]},
            {"denom": "USDT", "exponent": 6, "aliases": []},
        ],
        "base": "uusdt",
        "display": "USDT",
        "name": "USDT (XPEX Chain)",
        "symbol": "USDT",
        "uri": "",
        "uri_hash": "",
    })

    st["dex"] = {"next_pool_id": "1", "pools": []}
    st.setdefault("upgrade", {})

    json.dump(g, open(a.out, "w"), indent=2)
    open(a.out, "a").write("\n")
    print(f"ok: uusdt supply {a.usdt_supply} USDT -> {a.treasury}; genesis_time {a.genesis_time}")


if __name__ == "__main__":
    main()
