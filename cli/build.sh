#!/usr/bin/env bash
set -euo pipefail

echo "Building TensorReaper CLI..."

# Clean previous builds
cargo clean

# Build release binary
cargo build --release

# Get version
VERSION=$(cargo metadata --no-deps --format-version 1 | jq -r '.packages[0].version')

echo ""
echo "✓ Build complete!"
echo "  Version: $VERSION"
echo "  Binary: target/release/tensorreaper"
echo ""
echo "To install:"
echo "  sudo cp target/release/tensorreaper /usr/local/bin/"
echo ""
echo "To test:"
echo "  ./target/release/tensorreaper --help"
