#!/usr/bin/env python3
"""
Build the xpex-2 genesis TEMPLATE from the xpex-1 genesis.

What changes
  - chain_id            xpex-1 -> xpex-2
  - address prefix      cosmos... -> xpex...   (accounts, validator and consensus addresses, module accounts).
                        The underlying keys/bytes are identical, only the bech32 text changes.
  - chain-native stable uusdt/"USDT" -> --stable-denom (default uxusd / XUSD). Same amounts.
  - gen_txs             removed. A gentx is signed over the chain-id and the bech32 addresses, so the old one is
                        invalid. The operator signs a new one with their own key (docs/RELAUNCH.md).

Amounts, vesting, params and the (empty) dex state are copied unchanged. Nothing here needs a private key.

Usage
  python3 scripts/genesis/make_genesis_v2.py --base railway-config/config/genesis.json \
      --out railway-config/config/genesis.xpex-2.template.json --genesis-time 2026-11-01T12:00:00Z
"""
import argparse, json, re, sys

CHARSET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
GEN = [0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3]


def polymod(values):
    chk = 1
    for v in values:
        b = chk >> 25
        chk = (chk & 0x1ffffff) << 5 ^ v
        for i in range(5):
            chk ^= GEN[i] if ((b >> i) & 1) else 0
    return chk


def hrp_expand(hrp):
    return [ord(x) >> 5 for x in hrp] + [0] + [ord(x) & 31 for x in hrp]


def decode(addr):
    """Return (hrp, 5-bit data) or None when addr is not a valid bech32 string."""
    if addr != addr.lower() or "1" not in addr:
        return None
    hrp, _, data = addr.rpartition("1")
    if not hrp or len(data) < 6 or any(c not in CHARSET for c in data):
        return None
    vals = [CHARSET.find(c) for c in data]
    if polymod(hrp_expand(hrp) + vals) != 1:
        return None
    return hrp, vals[:-6]


def encode(hrp, data):
    values = hrp_expand(hrp) + data
    mod = polymod(values + [0] * 6) ^ 1
    checksum = [(mod >> 5 * (5 - i)) & 31 for i in range(6)]
    return hrp + "1" + "".join(CHARSET[d] for d in data + checksum)


PREFIX_MAP = {"cosmos": "xpex", "cosmosvaloper": "xpexvaloper", "cosmosvalcons": "xpexvalcons"}
ADDR_RE = re.compile(r"\b(cosmos(?:valoper|valcons)?)1[02-9ac-hj-np-z]{20,}\b")


def convert_text(text):
    count = 0

    def sub(m):
        nonlocal count
        d = decode(m.group(0))
        if d is None or d[0] not in PREFIX_MAP:
            return m.group(0)
        count += 1
        return encode(PREFIX_MAP[d[0]], d[1])

    return ADDR_RE.sub(sub, text), count


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--genesis-time", required=True)
    ap.add_argument("--chain-id", default="xpex-2")
    ap.add_argument("--stable-denom", default="uxusd")
    ap.add_argument("--stable-symbol", default="XUSD")
    ap.add_argument("--stable-name", default="XPEX USD (chain-native, unbacked)")
    a = ap.parse_args()

    raw = open(a.base).read()
    g = json.loads(raw)
    if g["chain_id"] != "xpex-1":
        sys.exit("expected the xpex-1 genesis as --base")

    text, n_addr = convert_text(raw)
    g = json.loads(text)
    g["chain_id"] = a.chain_id
    g["genesis_time"] = a.genesis_time
    st = g["app_state"]

    # stable asset rename (base denom, display denom, metadata)
    old = "uusdt"
    text = json.dumps(g)
    text = text.replace(f'"{old}"', f'"{a.stable_denom}"')
    g = json.loads(text)
    st = g["app_state"]
    for m in st["bank"]["denom_metadata"]:
        if m["base"] == a.stable_denom:
            m["description"] = "Chain-native unit of account on XPEX Chain. Not backed by any asset and not a stablecoin; not issued by Tether."
            m["denom_units"] = [
                {"denom": a.stable_denom, "exponent": 0, "aliases": []},
                {"denom": a.stable_symbol, "exponent": 6, "aliases": []},
            ]
            m["display"] = a.stable_symbol
            m["name"] = a.stable_name
            m["symbol"] = a.stable_symbol
    # coins inside bank balances/supply must stay sorted by denom
    for b in st["bank"]["balances"]:
        b["coins"].sort(key=lambda c: c["denom"])
    st["bank"]["supply"].sort(key=lambda c: c["denom"])

    st["genutil"]["gen_txs"] = []

    out = json.dumps(g, indent=2) + "\n"
    leftover = len(re.findall(r"cosmos1|cosmosvaloper1|cosmosvalcons1|uusdt|USDT", out))
    open(a.out, "w").write(out)
    print(f"converted {n_addr} addresses, chain_id={g['chain_id']}, stable={a.stable_denom}, gen_txs=0, leftovers={leftover}")
    if leftover:
        print("WARNING: leftover old identifiers remain, review the output", file=sys.stderr)


if __name__ == "__main__":
    main()
