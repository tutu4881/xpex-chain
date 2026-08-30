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

if [ -z "${MINIMUM_GAS_PRICES:-}" ]; then
    export MINIMUM_GAS_PRICES="0uxpx"
fi

exec /usr/bin/exampled start \
    --home "$HOME_DIR" \
    --minimum-gas-prices "$MINIMUM_GAS_PRICES" \
    --p2p.laddr "tcp://0.0.0.0:26656" \
    --rpc.laddr "tcp://0.0.0.0:26657" \
    --api.enable \
    --api.address "tcp://0.0.0.0:1317" \
    --grpc.address "0.0.0.0:9090"
