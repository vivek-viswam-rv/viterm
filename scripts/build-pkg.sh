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

echo "building launcher app..."
APP="$STAGE/Applications/viterm.app"
mkdir -p "$APP/Contents/MacOS"

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
	<string>viterm-launcher</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>BUNDLE_VERSION</string>
	<key>LSMinimumSystemVersion</key>
	<string>11.0</string>
</dict>
</plist>
PLIST
sed -i '' "s/BUNDLE_VERSION/$VERSION/" "$APP/Contents/Info.plist"

cat > "$APP/Contents/MacOS/viterm-launcher" <<'LAUNCHER'
#!/bin/sh
# Opens viterm in the user's terminal. The launcher app exists so viterm can
# be started from /Applications even though it is a terminal program.
exec open -a Terminal /usr/local/bin/viterm
LAUNCHER
chmod 755 "$APP/Contents/MacOS/viterm-launcher"
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
