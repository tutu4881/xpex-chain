# Extra chain modules (xpex-2)

Added on top of the base chain. **Not compiled or tested by their author** before the first Railway build.

| Module | What it gives you |
|---|---|
| `x/authz` | An account can grant another address limited rights, e.g. a capped `SendAuthorization` ("this service may spend up to N XPX from my wallet"), revocable at any time. This is how a "connect my wallet" payment can work without signing every request. |
| `x/feegrant` | One account pays the transaction fees of another (gasless UX). |
| `x/evidence` | Double-signing is punished (validator tombstoned). Without it only downtime is slashed. |
| `x/protocolpool` | The chain's community pool / treasury. Spend and continuous funds are controlled by governance. It replaces distribution's built-in community pool. |
| burn account (`burn.go`) | Module account `xpexburn` with the Burner permission. Anything sent to it with a normal MsgSend is destroyed at the end of the block and leaves `total supply`. Address: `xpex16zc492wmxzyu9ccsaplwfueayv3jc06uch8fk6`. |

Deliberately NOT added: `x/mint` (it creates new coins; the supply is meant to stay fixed or shrink) and IBC
(`ibc-go` has no release for this SDK pre-release).

Genesis: `railway-config/config/genesis.json` is the previous xpex-2 genesis plus default sections for authz, feegrant,
evidence and protocolpool. The validator gentx and chain-id are unchanged, so the existing gentx stays valid.
