# xpex-2 relaunch runbook

Why: xpex-1 uses the Cosmos Hub address prefix (`cosmos1...`) and a chain-native token called "USDT".
xpex-2 fixes both: addresses start with `xpex1...` and the quote asset is `uxusd` / XUSD (unbacked, not a stablecoin).
Amounts and the XPX supply are unchanged. **Nothing below needs more validators.**

Code changes on this branch: `bech32.go` (prefix), `app.go` (uses it), `railway-start.sh` (`CHAIN_ID`, default xpex-2,
and a guard that refuses a validator-less genesis), `scripts/genesis/make_genesis_v2.py` + the generated
`railway-config/config/genesis.xpex-2.template.json`.

**The Go code on this branch has not been compiled or tested by its author** (no Go toolchain was available).
Step 1 below is the first compile.

## What only the key owner can do (about 15 minutes, on your own computer)

The old gentx is signed for chain-id `xpex-1` and `cosmos...` addresses, so it is invalid. The validator operator
(the treasury account, now `xpex187vl5nunwfs4rqzdnwyce8q3u25wdaneuzwsn8`) must sign a new one.
Never paste the mnemonic anywhere except your own terminal.

1. Build the binary (Go 1.25+):
   `git clone https://github.com/tutu4881/xpex-chain && cd xpex-chain && git checkout release/xpex-2 && go build -o xpexd ./exampled`
   If this fails, send me the error text.
2. Fresh home and genesis:
   `./xpexd init XPEX --chain-id xpex-2 --home ./h --overwrite`
   `cp railway-config/config/genesis.xpex-2.template.json ./h/config/genesis.json`
3. Use the SAME validator consensus key as now: put the content of the Railway variable `VALIDATOR_KEY_JSON_B64`
   (base64-decoded) into `./h/config/priv_validator_key.json`.
4. Import the treasury key from its mnemonic:
   `./xpexd keys add treasury --recover --keyring-backend test --home ./h`
   Check that `./xpexd keys show treasury -a --keyring-backend test --home ./h` prints `xpex187vl5nu...wsn8`.
5. Create the gentx (same parameters as xpex-1):
   `./xpexd genesis gentx treasury 100000000uxpx --chain-id xpex-2 --moniker XPEX --commission-rate 0.10 --commission-max-rate 0.20 --commission-max-change-rate 0.01 --min-self-delegation 1 --keyring-backend test --home ./h`
6. `./xpexd genesis collect-gentxs --home ./h` then `./xpexd genesis validate --home ./h`
7. Send/commit `./h/config/genesis.json` as `railway-config/config/genesis.json` on branch `release/xpex-2`
   (it contains only public data; the mnemonic and the key never leave your computer).

## Switch-over (done together with the site and DEX updates)

1. xpex-chain1: set branch `release/xpex-2`, set `CHAIN_RESET_ID=xpex-2` (wipes xpex-1 data, keeps node and validator keys),
   `CHAIN_ID=xpex-2`.
2. xpex-chain-dex: reset its Postgres tables, set the new chain id, address prefix `xpex`, denom `uxusd`.
3. xpex site: chain config (`xpex-2`, prefix `xpex`, `uxusd`/XUSD) and the labels that say USDT for the chain asset.
4. Re-create the XPX/XUSD pool with MsgCreatePool (docs/DEX_LAUNCH.md).

Rollback: set the service branch back to `feat/timeout-commit-env`. The xpex-1 volume is wiped by the reset, so only reset
when you are sure.
