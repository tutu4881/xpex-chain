#!/usr/bin/env bash
# Starts Hermes between XPEX Chain and a counterparty chain (Osmosis testnet by default; osmosis-1 for mainnet).
#
# Variables (Railway service variables):
#   RELAYER_MNEMONIC        required. 24 words of the relayer's own hot wallet (NOT the treasury).
#                           Same key on both chains: xpex1... and osmo1... addresses.
#   XPEX_CHAIN_ID           default xpex-2
#   XPEX_RPC / XPEX_GRPC    default the node on Railway's private network
#   CP_CHAIN_ID             default osmo-test-5
#   CP_RPC / CP_GRPC        default the public Osmosis testnet endpoints
#   CP_PREFIX / CP_DENOM / CP_GAS_PRICE   default osmo / uosmo / 0.025
#   RELAYER_CREATE_CHANNEL  1 = create a transfer channel (client + connection + channel) once,
#                           if none exists yet between the two chains. Default 1.
#   XPEX_REST               default https://api.xpexpex.com; used to find the channel to the counterparty
#
# Hermes only relays the one transfer channel between the two chains (packet filter on both sides):
# a big counterparty like Osmosis mainnet has thousands of channels, and scanning them all would
# take ages and more memory than this service has.
set -uo pipefail

XPEX_CHAIN_ID="${XPEX_CHAIN_ID:-xpex-2}"
XPEX_RPC="${XPEX_RPC:-http://xpex-chain.railway.internal:26657}"
XPEX_GRPC="${XPEX_GRPC:-http://xpex-chain.railway.internal:9090}"
CP_CHAIN_ID="${CP_CHAIN_ID:-osmo-test-5}"
CP_RPC="${CP_RPC:-https://rpc.testnet.osmosis.zone}"
CP_GRPC="${CP_GRPC:-https://grpc.testnet.osmosis.zone:443}"
CP_PREFIX="${CP_PREFIX:-osmo}"
CP_DENOM="${CP_DENOM:-uosmo}"
CP_GAS_PRICE="${CP_GAS_PRICE:-0.025}"
# Public Osmosis nodes do not keep block results, so events come over the websocket (push), not by polling.
CP_WS="${CP_WS:-wss://rpc.testnet.osmosis.zone/websocket}"
# Hermes logs a line per client it scans on a busy chain like Osmosis (thousands); warn keeps the log usable.
LOG_LEVEL="${LOG_LEVEL:-warn}"
RELAYER_CREATE_CHANNEL="${RELAYER_CREATE_CHANNEL:-1}"
XPEX_REST="${XPEX_REST:-https://api.xpexpex.com}"

if [ -z "${RELAYER_MNEMONIC:-}" ]; then
  echo "RELAYER_MNEMONIC is not set. Create a dedicated relayer key and put its 24 words in the"
  echo "RELAYER_MNEMONIC variable of this service. Waiting instead of restarting in a loop."
  exec sleep infinity
fi

mkdir -p "$HOME/.hermes"
CONFIG="$HOME/.hermes/config.toml"

# write_config [xpex-channel] [counterparty-channel]: with both, each chain relays only that channel.
write_config() {
XPEX_FILTER=""
CP_FILTER=""
if [ -n "${1:-}" ] && [ -n "${2:-}" ]; then
  XPEX_FILTER="packet_filter = { policy = 'allow', list = [['transfer', '$1']] }"
  CP_FILTER="packet_filter = { policy = 'allow', list = [['transfer', '$2']] }"
fi
cat > "$CONFIG" <<TOML
[global]
log_level = '${LOG_LEVEL}'

[mode.clients]
enabled = true
refresh = true
misbehaviour = false

[mode.connections]
enabled = false

[mode.channels]
enabled = false

[mode.packets]
enabled = true
clear_interval = 100
clear_on_start = true
tx_confirmation = false

[rest]
enabled = false
host = '127.0.0.1'
port = 3000

[telemetry]
enabled = false
host = '127.0.0.1'
port = 3001

[[chains]]
id = '${XPEX_CHAIN_ID}'
type = 'CosmosSdk'
rpc_addr = '${XPEX_RPC}'
grpc_addr = '${XPEX_GRPC}'
event_source = { mode = 'pull', interval = '2s' }
rpc_timeout = '15s'
trusted_node = true
account_prefix = 'xpex'
key_name = 'relayer'
address_type = { derivation = 'cosmos' }
store_prefix = 'ibc'
default_gas = 200000
max_gas = 3000000
gas_price = { price = 0, denom = 'uxpx' }
gas_multiplier = 1.3
max_msg_num = 30
max_tx_size = 180000
clock_drift = '10s'
max_block_time = '30s'
trust_threshold = '2/3'
compat_mode = '0.38'
memo_prefix = 'xpex-relayer'
${XPEX_FILTER}

[[chains]]
id = '${CP_CHAIN_ID}'
type = 'CosmosSdk'
rpc_addr = '${CP_RPC}'
grpc_addr = '${CP_GRPC}'
event_source = { mode = 'push', url = '${CP_WS}', batch_delay = '500ms' }
rpc_timeout = '20s'
trusted_node = false
account_prefix = '${CP_PREFIX}'
key_name = 'relayer'
address_type = { derivation = 'cosmos' }
store_prefix = 'ibc'
default_gas = 400000
max_gas = 4000000
gas_price = { price = ${CP_GAS_PRICE}, denom = '${CP_DENOM}' }
dynamic_gas_price = { enabled = true, multiplier = 1.2, max = 0.6 }
gas_multiplier = 1.3
max_msg_num = 30
max_tx_size = 180000
clock_drift = '10s'
max_block_time = '30s'
trust_threshold = '2/3'
memo_prefix = 'xpex-relayer'
${CP_FILTER}
TOML
}
write_config

# Keys: same mnemonic on both chains (coin type 118 on both). Written to a temp file only for the import.
MN="$(mktemp)"
printf '%s' "$RELAYER_MNEMONIC" > "$MN"
for chain in "$XPEX_CHAIN_ID" "$CP_CHAIN_ID"; do
  hermes keys add --chain "$chain" --key-name relayer --mnemonic-file "$MN" --overwrite >/dev/null 2>&1 \
    || echo "WARN: could not import the relayer key for $chain"
done
rm -f "$MN"
unset RELAYER_MNEMONIC

echo "Relayer addresses (fund these once):"
for chain in "$XPEX_CHAIN_ID" "$CP_CHAIN_ID"; do
  hermes keys list --chain "$chain" 2>/dev/null | grep -o '[a-z]*1[0-9a-z]\{38,\}' | head -n1 | sed "s/^/  $chain: /"
done

hermes health-check || echo "WARN: health-check reported problems (see above); starting anyway"

# Our open transfer channel to the counterparty, as "xpex-channel:counterparty-channel" (empty if none).
# Read from XPEX's own REST API: the channel whose light client tracks CP_CHAIN_ID.
find_channel() {
  local list ch a cid
  list="$(curl -fsS --max-time 20 "$XPEX_REST/ibc/core/channel/v1/channels?pagination.limit=200" 2>/dev/null     | jq -r '.channels[] | select(.state == "STATE_OPEN" and .port_id == "transfer") | .channel_id + ":" + .counterparty.channel_id' 2>/dev/null)"
  for ch in $list; do
    a="${ch%%:*}"
    cid="$(curl -fsS --max-time 20 "$XPEX_REST/ibc/core/channel/v1/channels/$a/ports/transfer/client_state" 2>/dev/null       | jq -r '.identified_client_state.client_state.chain_id' 2>/dev/null)"
    [ "$cid" = "$CP_CHAIN_ID" ] && echo "$ch"
  done | tail -n1
}

PAIR="$(find_channel)"
if [ -z "$PAIR" ] && [ "$RELAYER_CREATE_CHANNEL" = "1" ]; then
  echo "No transfer channel between $XPEX_CHAIN_ID and $CP_CHAIN_ID yet: creating client, connection and channel..."
  if hermes create channel --a-chain "$XPEX_CHAIN_ID" --b-chain "$CP_CHAIN_ID"       --a-port transfer --b-port transfer --new-client-connection --yes; then
    echo "Channel created."
    sleep 10
    PAIR="$(find_channel)"
  else
    echo "WARN: channel creation failed (usually: an account is not funded yet). Redeploy to retry."
  fi
fi

if [ -z "$PAIR" ]; then
  echo "No open transfer channel to $CP_CHAIN_ID: nothing to relay. Waiting instead of restarting in a loop."
  exec sleep infinity
fi
XPEX_CH="${PAIR%%:*}"
CP_CH="${PAIR##*:}"
echo "Relaying transfer/$XPEX_CH ($XPEX_CHAIN_ID) <-> transfer/$CP_CH ($CP_CHAIN_ID) only."
write_config "$XPEX_CH" "$CP_CH"

# Optional one-off end-to-end test (RELAYER_TEST_TRANSFER=1): send 0.1 XPX from the relayer wallet to its own
# address on the counterparty, then 0.05 of the received voucher back. Remove the variable afterwards.
if [ "${RELAYER_TEST_TRANSFER:-0}" = "1" ]; then
  SRC_CH="${TEST_SRC_CHANNEL:-$XPEX_CH}"
  DST_CH="${TEST_DST_CHANNEL:-$CP_CH}"
  VOUCHER="ibc/$(printf 'transfer/%s/uxpx' "$DST_CH" | sha256sum | awk '{print toupper($1)}')"
  (
    sleep 30
    echo "TEST: sending 100000uxpx ${XPEX_CHAIN_ID}/${SRC_CH} -> ${CP_CHAIN_ID}"
    hermes tx ft-transfer --src-chain "$XPEX_CHAIN_ID" --dst-chain "$CP_CHAIN_ID" --src-port transfer --src-channel "$SRC_CH" \
      --amount 100000 --denom uxpx --timeout-seconds 900 2>&1 | grep -E "SUCCESS|ERROR|Error" | head -n 3 | sed 's/^/TEST: /'
    sleep 180
    echo "TEST: sending 50000 ${VOUCHER} back ${CP_CHAIN_ID}/${DST_CH} -> ${XPEX_CHAIN_ID}"
    hermes tx ft-transfer --src-chain "$CP_CHAIN_ID" --dst-chain "$XPEX_CHAIN_ID" --src-port transfer --src-channel "$DST_CH" \
      --amount 50000 --denom "$VOUCHER" --timeout-seconds 900 2>&1 | grep -E "SUCCESS|ERROR|Error" | head -n 3 | sed 's/^/TEST: /'
  ) &
fi

exec hermes start
