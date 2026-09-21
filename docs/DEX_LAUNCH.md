# XPX/USDT pool launch

Genesis contains the `dex` module and the chain-native asset `uusdt`
(6 decimals, 1,000 USDT minted to the treasury/validator-operator account
`cosmos187vl5nunwfs4rqzdnwyce8q3u25wdanejvej4n`). **No pool exists at genesis** –
the initial price is set by whoever creates it.

Create the pool from the site (DEX → Liquidity → Create pool) while the treasury
wallet is unlocked: 100 USDT + the XPX amount you choose. Price = USDT / XPX.
The creator receives all initial LP shares and can withdraw them later
(withdrawing everything deletes the pool so it can be re-created at another
price).

`uusdt` is an XPEX-Chain asset. It is not issued or backed by Tether.
