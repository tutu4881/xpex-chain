#!/bin/sh
set -eu

HOME_DIR="/root/.exampleapp"

mkdir -p "$HOME_DIR/config"
mkdir -p "$HOME_DIR/data"

echo "Starting XPEX chain..."
echo "Home: $HOME_DIR"

if [ ! -f "$HOME_DIR/config/genesis.json" ]; then
    echo "ERROR: genesis.json not found"
    exit 1
fi

if [ -z "${VALIDATOR_KEY_JSON_B64:-}" ]; then
    echo "ERROR: VALIDATOR_KEY_JSON_B64 is not set"
    exit 1
fi

echo "$VALIDATOR_KEY_JSON_B64" | base64 -d > "$HOME_DIR/config/priv_validator_key.json"

chmod 600 "$HOME_DIR/config/priv_validator_key.json"

echo "Validator key installed."

if [ -z "${MINIMUM_GAS_PRICES:-}" ]; then
    export MINIMUM_GAS_PRICES="0uxpx"
fi

exec /usr/local/bin/xpexd start \
    --home "$HOME_DIR" \
    --minimum-gas-prices "$MINIMUM_GAS_PRICES" \
    --p2p.laddr "tcp://0.0.0.0:26656" \
    --rpc.laddr "tcp://0.0.0.0:26657" \
    --api.enable \
    --api.address "tcp://0.0.0.0:1317" \
    --grpc.address "0.0.0.0:9090"
