#!/bin/sh
set -eu

echo "======================================"
echo "        XPEX CHAIN — RAILWAY"
echo "======================================"

HOME_DIR="/data/.exampleapp"
IMAGE_GENESIS="/opt/xpex/genesis.json"

mkdir -p "$HOME_DIR/config"
mkdir -p "$HOME_DIR/data"

echo "PORT=${PORT:-not-set}"
echo "RAILWAY_TCP_PROXY_DOMAIN=${RAILWAY_TCP_PROXY_DOMAIN:-not-set}"
echo "RAILWAY_TCP_PROXY_PORT=${RAILWAY_TCP_PROXY_PORT:-not-set}"

if [ ! -f "$HOME_DIR/config/genesis.json" ]; then
    echo "Genesis not found in persistent volume."

    if [ ! -f "$IMAGE_GENESIS" ]; then
        echo "ERROR: genesis template not found at $IMAGE_GENESIS"
        exit 1
    fi

    echo "Installing genesis into persistent volume..."
    cp "$IMAGE_GENESIS" "$HOME_DIR/config/genesis.json"
else
    echo "Genesis already exists in persistent volume."
fi

echo "Genesis found."

echo "Genesis found."

# --------------------------------------------------
# Initialize config
# --------------------------------------------------

if [ ! -f "$HOME_DIR/config/config.toml" ]; then
    echo "Initializing XPEX configuration..."

    /usr/local/bin/xpexd init XPEX \
        --home "$HOME_DIR" \
        --chain-id xpex-1
fi

# --------------------------------------------------
# Validator key
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
# CometBFT RPC
# --------------------------------------------------

echo "Configuring CometBFT RPC..."

python3 - "$HOME_DIR/config/config.toml" <<'PY'
from pathlib import Path
import sys

p = Path(sys.argv[1])
s = p.read_text()

start = s.index("[rpc]")
end = s.find("\n[", start + 1)

if end == -1:
    end = len(s)

section = s[start:end]

lines = section.splitlines()

for i, line in enumerate(lines):
    if line.startswith("laddr = "):
        lines[i] = 'laddr = "tcp://0.0.0.0:26657"'

s = s[:start] + "\n".join(lines) + s[end:]

p.write_text(s)
PY

# --------------------------------------------------
# Cosmos REST API
# --------------------------------------------------

echo "Configuring Cosmos REST API..."

python3 - "$HOME_DIR/config/app.toml" "${PORT:-1317}" <<'PY'
from pathlib import Path
import sys

p = Path(sys.argv[1])
port = sys.argv[2]

s = p.read_text()

start = s.index("[api]")
end = s.find("\n[", start + 1)

if end == -1:
    end = len(s)

section = s[start:end]

lines = section.splitlines()

for i, line in enumerate(lines):
    if line.startswith("enable = "):
        lines[i] = "enable = true"

    if line.startswith("address = "):
        lines[i] = f'address = "tcp://0.0.0.0:{port}"'

s = s[:start] + "\n".join(lines) + s[end:]

p.write_text(s)
PY

# --------------------------------------------------
# gRPC
# --------------------------------------------------

echo "Configuring gRPC..."

python3 - "$HOME_DIR/config/app.toml" <<'PY'
from pathlib import Path
import sys

p = Path(sys.argv[1])
s = p.read_text()

start = s.index("[grpc]")
end = s.find("\n[", start + 1)

if end == -1:
    end = len(s)

section = s[start:end]

lines = section.splitlines()

for i, line in enumerate(lines):
    if line.startswith("enable = "):
        lines[i] = "enable = true"

    if line.startswith("address = "):
        lines[i] = 'address = "0.0.0.0:9090"'

s = s[:start] + "\n".join(lines) + s[end:]

p.write_text(s)
PY

# --------------------------------------------------
# Gas prices
# --------------------------------------------------

if [ -z "${MINIMUM_GAS_PRICES:-}" ]; then
    MINIMUM_GAS_PRICES="0uxpx"
fi

echo "MINIMUM_GAS_PRICES=$MINIMUM_GAS_PRICES"

# --------------------------------------------------
# Verify configuration
# --------------------------------------------------

echo "======================================"
echo "FINAL CONFIGURATION"
echo "======================================"

echo "--- RPC ---"
sed -n '/^\[rpc\]/,/^\[/p' "$HOME_DIR/config/config.toml"

echo "--- API ---"
sed -n '/^\[api\]/,/^\[/p' "$HOME_DIR/config/app.toml"

echo "--- gRPC ---"
sed -n '/^\[grpc\]/,/^\[/p' "$HOME_DIR/config/app.toml"

echo "======================================"
echo "Starting XPEX..."
echo "======================================"

exec /usr/local/bin/xpexd start \
    --home "$HOME_DIR" \
    --minimum-gas-prices "$MINIMUM_GAS_PRICES"
