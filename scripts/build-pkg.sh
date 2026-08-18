#!/bin/sh
# Builds a macOS installer package (dist/viterm-<version>.pkg) containing a
# universal binary that installs to /usr/local/bin. Run on macOS.
set -eu

VERSION="${1:-dev}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist"
STAGE="$DIST/pkg-stage"

rm -rf "$STAGE"
mkdir -p "$STAGE/usr/local/bin" "$DIST"

echo "building universal binary..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -C "$ROOT" \
    -ldflags "-s -w -X main.version=$VERSION" \
    -o "$DIST/viterm-arm64" ./cmd/viterm
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -C "$ROOT" \
    -ldflags "-s -w -X main.version=$VERSION" \
    -o "$DIST/viterm-amd64" ./cmd/viterm
lipo -create -output "$STAGE/usr/local/bin/viterm" \
    "$DIST/viterm-arm64" "$DIST/viterm-amd64"
chmod 755 "$STAGE/usr/local/bin/viterm"
rm -f "$DIST/viterm-arm64" "$DIST/viterm-amd64"
xattr -cr "$STAGE" 2>/dev/null || true

PKG="$DIST/viterm-$VERSION.pkg"
pkgbuild \
    --root "$STAGE" \
    --identifier com.vivekviswam.viterm \
    --version "$VERSION" \
    --install-location / \
    "$PKG"

rm -rf "$STAGE"
echo "built $PKG"
