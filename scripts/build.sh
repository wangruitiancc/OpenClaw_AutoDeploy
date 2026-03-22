#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BIN_DIR="$PROJECT_DIR/bin"

echo "=== OpenClaw AutoDeploy Build ==="

if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed."
    echo "Install from: https://go.dev/dl/ or 'brew install go'"
    exit 1
fi

echo "Go version: $(go version | awk '{print $3}')"

mkdir -p "$BIN_DIR"

cd "$PROJECT_DIR"
echo "Building control-plane-api ..."
go build -o "$BIN_DIR/control-plane-api" ./cmd/control-plane-api/
echo "  -> $BIN_DIR/control-plane-api"

echo "Building ultraworker ..."
go build -o "$BIN_DIR/ultraworker" ./cmd/ultraworker/
echo "  -> $BIN_DIR/ultraworker"

echo "Building openclawctl ..."
go build -o "$BIN_DIR/openclawctl" ./cmd/openclawctl/
echo "  -> $BIN_DIR/openclawctl"

chmod +x "$BIN_DIR/openclawctl"

echo ""
echo "=== Build Complete ==="
ls -lh "$BIN_DIR/"
