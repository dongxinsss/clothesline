#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
app="build/bin/拾光绳.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
if [ "${1:-}" = "--universal" ]; then
  GOARCH=arm64 CGO_ENABLED=1 go build -trimpath -tags desktop,production -ldflags '-s -w' -o build/bin/clothesline-darwin-arm64 .
  GOARCH=amd64 CGO_ENABLED=1 go build -trimpath -tags desktop,production -ldflags '-s -w' -o build/bin/clothesline-darwin-amd64 .
  lipo -create build/bin/clothesline-darwin-arm64 build/bin/clothesline-darwin-amd64 -output "$app/Contents/MacOS/clothesline"
else
  go build -trimpath -tags desktop,production -ldflags '-s -w' -o "$app/Contents/MacOS/clothesline" .
fi
cp frontend/appicon.png "$app/Contents/Resources/appicon.png"
if [ -f docs/THIRD-PARTY-NOTICES.txt ]; then cp docs/THIRD-PARTY-NOTICES.txt "$app/Contents/Resources/"; fi
cat > "$app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleName</key><string>拾光绳</string>
<key>CFBundleDisplayName</key><string>拾光绳</string>
<key>CFBundleExecutable</key><string>clothesline</string>
<key>CFBundleIdentifier</key><string>app.clothesline.desktop</string>
<key>CFBundleVersion</key><string>0.4.1</string>
<key>CFBundleShortVersionString</key><string>0.4.1</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>NSHighResolutionCapable</key><true/>
<key>LSUIElement</key><false/>
<key>LSMinimumSystemVersion</key><string>12.0</string>
<key>NSScreenCaptureUsageDescription</key><string>仅在你点击截图后，使用系统截图工具选择并收纳画面。</string>
</dict></plist>
PLIST
if [ -f frontend/appicon.icns ]; then
  cp frontend/appicon.icns "$app/Contents/Resources/ClotheslineIcon-0.4.1.icns"
  /usr/libexec/PlistBuddy -c 'Add :CFBundleIconFile string ClotheslineIcon-0.4.1.icns' "$app/Contents/Info.plist"
fi
# TCC keys the Screen Recording grant to the designated requirement. An ad-hoc
# signature defaults to the binary's cdhash, so every rebuild silently drops the
# grant while System Settings still shows it switched on. Pin it to the bundle
# identifier, or set CODESIGN_IDENTITY to a real signing certificate.
identity="${CODESIGN_IDENTITY:--}"
if [ "$identity" = "-" ]; then
  /usr/bin/codesign --force --deep --sign - --requirements '=designated => identifier "app.clothesline.desktop"' "$app"
else
  /usr/bin/codesign --force --deep --sign "$identity" "$app"
fi
printf 'Built: %s\n' "$app"
