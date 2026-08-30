#!/bin/sh
set -eu

echo "======================================"
echo "        XPEX CHAIN — RAILWAY"
echo "======================================"

HOME_DIR="/root/.exampleapp"

mkdir -p "$HOME_DIR/config"
mkdir -p "$HOME_DIR/data"

echo "PORT=${PORT:-not-set}"

# --------------------------------------------------
# 1. Genesis
# --------------------------------------------------

if [ ! -f "$HOME_DIR/config/genesis.json" ]; then
    echo "ERROR: genesis.json not found"
    exit 1
fi

echo "Genesis found."

# --------------------------------------------------
# 2. Initialize configuration if needed
# --------------------------------------------------

if [ ! -f "$HOME_DIR/config/config.toml" ]; then
    echo "Initializing XPEX configuration..."

    /usr/local/bin/xpexd init XPEX \
        --home "$HOME_DIR" \
        --chain-id xpex-1
fi

# --------------------------------------------------
# 3. Install validator key
# --------------------------------------------------

if [ -z "${VALIDATOR_KEY_JSON_B64:-}" ]; then
    echo "ERROR: VALIDATOR_KEY_JSON_B64 is not set"
    exit 1
fi

echo "Installing validator key..."

echo "$VALIDATOR_KEY_JSON_B64" | base64 -d \
    > "$HOME_DIR/config/priv_validator_key.json"

chmod 600 "$HOME_DIR/config/priv_validator_key.json"

echo "Validator key installed."

# --------------------------------------------------
# 4. CometBFT RPC
# --------------------------------------------------

echo "Configuring CometBFT RPC..."

sed -i \
    's|^laddr = .*|laddr = "tcp://0.0.0.0:26657"|' \
    "$HOME_DIR/config/config.toml"

# --------------------------------------------------
# 5. P2P
# --------------------------------------------------

sed -i \
    's|^laddr = "tcp://0.0.0.0:26656"|laddr = "tcp://0.0.0.0:26656"|' \
    "$HOME_DIR/config/config.toml"

# --------------------------------------------------
# 6. Cosmos REST API
# --------------------------------------------------

echo "Configuring Cosmos REST API..."

sed -i \
    '/^\[api\]/,/^\[/ s/^enable = false$/enable = true/' \
    "$HOME_DIR/config/app.toml"

sed -i \
    's|^address = "tcp://localhost:1317"|address = "tcp://0.0.0.0:'"${PORT:-1317}"'"|' \
    "$HOME_DIR/config/app.toml"

# --------------------------------------------------
# 7. gRPC
# --------------------------------------------------

echo "Configuring gRPC..."

sed -i \
    's|^address = "localhost:9090"|address = "0.0.0.0:9090"|' \
    "$HOME_DIR/config/app.toml"

# --------------------------------------------------
# 8. Gas prices
# --------------------------------------------------

if [ -z "${MINIMUM_GAS_PRICES:-}" ]; then
    export MINIMUM_GAS_PRICES="0uxpx"
fi

echo "MINIMUM_GAS_PRICES=$MINIMUM_GAS_PRICES"

# --------------------------------------------------
# 9. Final configuration
# --------------------------------------------------

echo "======================================"
echo "FINAL CONFIGURATION"
echo "======================================"

echo "--- CometBFT RPC ---"
grep '^laddr =' "$HOME_DIR/config/config.toml" || true

echo "--- REST API ---"
grep -A12 '^\[api\]' "$HOME_DIR/config/app.toml" || true

echo "--- gRPC ---"
grep -A8 '^\[grpc\]' "$HOME_DIR/config/app.toml" || true

echo "======================================"
echo "Starting XPEX..."
echo "======================================"

exec /usr/local/bin/xpexd start \
    --home "$HOME_DIR" \
    --minimum-gas-prices "$MINIMUM_GAS_PRICES"
