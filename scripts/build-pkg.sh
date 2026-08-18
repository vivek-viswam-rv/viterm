#!/bin/sh
# Builds a macOS installer package (dist/viterm-<version>.pkg). The package
# installs a universal binary to /usr/local/bin and a launcher app to
# /Applications that opens viterm in Terminal. Run on macOS.
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

echo "building window host..."
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -C "$ROOT" \
    -ldflags "-s -w" -o "$DIST/viterm-app-arm64" ./cmd/viterm-app
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -C "$ROOT" \
    -ldflags "-s -w" -o "$DIST/viterm-app-amd64" ./cmd/viterm-app

APP="$STAGE/Applications/viterm.app"
mkdir -p "$APP/Contents/MacOS"
lipo -create -output "$APP/Contents/MacOS/viterm-app" \
    "$DIST/viterm-app-arm64" "$DIST/viterm-app-amd64"
chmod 755 "$APP/Contents/MacOS/viterm-app"
rm -f "$DIST/viterm-app-arm64" "$DIST/viterm-app-amd64"
cp "$STAGE/usr/local/bin/viterm" "$APP/Contents/MacOS/viterm"

cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>viterm</string>
	<key>CFBundleDisplayName</key>
	<string>viterm</string>
	<key>CFBundleIdentifier</key>
	<string>com.vivekviswam.viterm.launcher</string>
	<key>CFBundleExecutable</key>
	<string>viterm-app</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>BUNDLE_VERSION</string>
	<key>LSMinimumSystemVersion</key>
	<string>11.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
</dict>
</plist>
PLIST
sed -i '' "s/BUNDLE_VERSION/$VERSION/" "$APP/Contents/Info.plist"
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
