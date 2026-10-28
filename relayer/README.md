# XPEX IBC relayer (Hermes)

Railway service built from this folder. It relays ICS-20 transfers between XPEX Chain and a counterparty
(default: Osmosis testnet `osmo-test-5`) and, on first start, creates the transfer channel.

Setup:
1. Create a dedicated relayer key (never the treasury): `xpexd keys add relayer --keyring-backend test`.
2. Put its 24 words in the service variable `RELAYER_MNEMONIC`.
3. Fund it once: a little XPX on `xpex1...` (fees on XPEX are 0, the account only has to exist) and testnet
   OSMO on `osmo1...` from the Osmosis testnet faucet. The log prints both addresses at start.
4. Redeploy: the channel is created, then Hermes keeps relaying.

For Osmosis mainnet set `CP_CHAIN_ID=osmosis-1`, `CP_RPC`, `CP_GRPC` and fund the osmo address with real OSMO.
