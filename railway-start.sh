#!/bin/sh
set -eu

echo "### XPEX RAILWAY START SCRIPT v4 ###"
echo "### PORT=${PORT:-NOT_SET} ###"

HOME_DIR="/root/.exampleapp"

echo "======================================"
echo "Starting XPEX chain"
echo "======================================"

echo "HOME_DIR=$HOME_DIR"
echo "PORT=${PORT:-not-set}"

mkdir -p "$HOME_DIR/config"
mkdir -p "$HOME_DIR/data"

if [ ! -f "$HOME_DIR/config/genesis.json" ]; then
    echo "ERROR: genesis.json not found"
    exit 1
fi

if [ -z "${VALIDATOR_KEY_JSON_B64:-}" ]; then
    echo "ERROR: VALIDATOR_KEY_JSON_B64 is not set"
    exit 1
fi

echo "Installing validator key..."

echo "$VALIDATOR_KEY_JSON_B64" | base64 -d \
    > "$HOME_DIR/config/priv_validator_key.json"

chmod 600 "$HOME_DIR/config/priv_validator_key.json"

echo "Validator key installed."

echo "Initializing XPEX configuration..."

if [ ! -f "$HOME_DIR/config/config.toml" ]; then
    /usr/local/bin/xpexd init XPEX \
        --home "$HOME_DIR" \
        --chain-id xpex-1
fi

echo "Configuring CometBFT RPC..."

sed -i \
    's|^laddr = .*26657.*|laddr = "tcp://0.0.0.0:26657"|' \
    "$HOME_DIR/config/config.toml"

echo "Configuring Cosmos API..."

sed -i \
    's|^address = .*1317.*|address = "tcp://0.0.0.0:'"${PORT:-1317}"'"|' \
    "$HOME_DIR/config/app.toml"

echo "Configuring gRPC..."

sed -i \
    's|^address = .*9090.*|address = "0.0.0.0:9090"|' \
    "$HOME_DIR/config/app.toml"

echo "======================================"
echo "FINAL CONFIG"
echo "======================================"

echo "--- RPC ---"
grep -n 'laddr = .*26657' \
    "$HOME_DIR/config/config.toml" || true

echo "--- API ---"
grep -n 'address = .*1317' \
    "$HOME_DIR/config/app.toml" || true

echo "--- gRPC ---"
grep -n 'address = .*9090' \
    "$HOME_DIR/config/app.toml" || true

echo "======================================"

echo "===== FILE CONFIG BEFORE START ====="

echo "--- config.toml RPC ---"
grep -n "laddr" \
    "$HOME_DIR/config/config.toml" || true

echo "--- app.toml API ---"
grep -n "address" \
    "$HOME_DIR/config/app.toml" | grep 1317 || true

echo "--- app.toml gRPC ---"
grep -n "address" \
    "$HOME_DIR/config/app.toml" | grep 9090 || true

echo "====================================="

if [ -z "${MINIMUM_GAS_PRICES:-}" ]; then
    export MINIMUM_GAS_PRICES="0uxpx"
fi

echo "MINIMUM_GAS_PRICES=$MINIMUM_GAS_PRICES"

echo "Starting xpexd..."

exec /usr/local/bin/xpexd start \
    --home "$HOME_DIR" \
    --minimum-gas-prices "$MINIMUM_GAS_PRICES"
