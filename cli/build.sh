#!/usr/bin/env bash
set -euo pipefail

echo "Building KubeFabric CLI..."

# Clean previous builds
cargo clean

# Build release binary
cargo build --release

# Get version
VERSION=$(cargo metadata --no-deps --format-version 1 | jq -r '.packages[0].version')

echo ""
echo "✓ Build complete!"
echo "  Version: $VERSION"
echo "  Binary: target/release/kubefabric"
echo ""
echo "To install:"
echo "  sudo cp target/release/kubefabric /usr/local/bin/"
echo ""
echo "To test:"
echo "  ./target/release/kubefabric --help"
