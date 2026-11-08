#!/bin/sh
# Run from the repository root after `make preview-build`.
set -eu
bundle='bin/Dym Bridge Preview.app'
mkdir -p "$bundle/Contents/MacOS" "$bundle/Contents/Resources"
cp bin/dym-bridge-gpu "$bundle/Contents/MacOS/dym-bridge-gpu"
cp dym-bridge.icns "$bundle/Contents/Resources/dym-bridge.icns"
cat > "$bundle/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>dym-bridge-gpu</string>
<key>CFBundleIdentifier</key><string>com.dymmer.bridge.preview</string>
<key>CFBundleName</key><string>Dym Bridge Preview</string>
<key>CFBundleIconFile</key><string>dym-bridge.icns</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
PLIST
printf 'Built %s\n' "$bundle"
