#!/bin/sh
set -eu

HOME_DIR="/root/.exampleapp"

mkdir -p "$HOME_DIR/config"
mkdir -p "$HOME_DIR/data"

echo "===== XPEX RAILWAY START ====="
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

echo "===== INITIALIZING CONFIG ====="

if [ ! -f "$HOME_DIR/config/config.toml" ]; then
    /usr/local/bin/xpexd init XPEX \
        --home "$HOME_DIR" \
        --chain-id xpex-1
fi

echo "===== FIXING RPC ====="

sed -i \
's|^laddr = .*|laddr = "tcp://0.0.0.0:26657"|' \
"$HOME_DIR/config/config.toml"

echo "===== FIXING API ====="

sed -i \
's|^address = .*|address = "0.0.0.0:1317"|' \
"$HOME_DIR/config/app.toml"

echo "===== FIXING GRPC ====="

sed -i \
's|^address = .*|address = "0.0.0.0:9090"|' \
"$HOME_DIR/config/app.toml"

echo "===== FINAL CONFIG ====="

echo "--- RPC ---"
grep 'laddr =' "$HOME_DIR/config/config.toml" || true

echo "--- API ---"
grep -n 'address =' "$HOME_DIR/config/app.toml" | head -5 || true

echo "--- GRPC ---"
grep -n 'address =' "$HOME_DIR/config/app.toml" | head -20 || true

echo "=========================="

if [ -z "${MINIMUM_GAS_PRICES:-}" ]; then
    export MINIMUM_GAS_PRICES="0uxpx"
fi

echo "===== STARTING XPEX NODE ====="

exec /usr/local/bin/xpexd start \
    --home "$HOME_DIR" \
    --minimum-gas-prices "$MINIMUM_GAS_PRICES"
