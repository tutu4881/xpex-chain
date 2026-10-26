# chain-registry draft (do not submit before xpex-2 is live)

`xpex/chain.json` and `xpex/assetlist.json` follow the cosmos/chain-registry layout (folder `xpex/`).
Fill `genesis_url` with the raw GitHub URL of the final xpex-2 `genesis.json`, check the explorer URLs, then open a PR to
https://github.com/cosmos/chain-registry. Keplr and Leap read the registry; until it is merged, wallets need a manual
"suggest chain" call.
Reviewers expect a working public RPC/REST (rpc.xpexpex.com and api.xpexpex.com exist) and a launched network.
Be accurate in the descriptions: XUSD is unbacked, XPEX runs one validator operated by the team.
