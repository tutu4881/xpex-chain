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

if [ ! -f "$HOME_DIR/config/config.toml" ]; then
    echo "Creating CometBFT config..."

    /usr/local/bin/xpexd init XPEX \
        --home "$HOME_DIR" \
        --chain-id xpex-1 >/dev/null 2>&1 || true
fi

if [ ! -f "$HOME_DIR/config/app.toml" ]; then
    echo "ERROR: app.toml not found"
    exit 1
fi

if [ ! -f "$HOME_DIR/config/config.toml" ]; then
    echo "ERROR: config.toml not found"
    exit 1
fi

echo "Configuring public network listeners..."

sed -i 's|^laddr = "tcp://127.0.0.1:26657"|laddr = "tcp://0.0.0.0:26657"|' \
    "$HOME_DIR/config/config.toml"

sed -i 's|^laddr = "tcp://localhost:26657"|laddr = "tcp://0.0.0.0:26657"|' \
    "$HOME_DIR/config/config.toml"

sed -i 's|^address = "tcp://localhost:1317"|address = "tcp://0.0.0.0:1317"|' \
    "$HOME_DIR/config/app.toml"

sed -i 's|^address = "localhost:9090"|address = "0.0.0.0:9090"|' \
    "$HOME_DIR/config/app.toml"

echo "RPC configuration:"
grep -E '^laddr = ' "$HOME_DIR/config/config.toml" | head -1

echo "API configuration:"
grep -E '^address = ' "$HOME_DIR/config/app.toml" | head -1

if [ -z "${MINIMUM_GAS_PRICES:-}" ]; then
    export MINIMUM_GAS_PRICES="0uxpx"
fi

exec /usr/local/bin/xpexd start \
    --home "$HOME_DIR" \
    --minimum-gas-prices "$MINIMUM_GAS_PRICES"
