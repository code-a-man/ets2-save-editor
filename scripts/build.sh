#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."
if [[ "$(uname -s)" != Darwin ]]; then
    echo "Build on macOS with Xcode Command Line Tools and Go 1.24+." >&2
    exit 1
fi

target="${1:-$(uname -m)}"
case "$target" in
    arm64|x86_64) architectures=("$target") ;;
    universal) architectures=(arm64 x86_64) ;;
    *) echo "Usage: ./scripts/build.sh [arm64|x86_64|universal]" >&2; exit 2 ;;
esac

version="${ETS2_VERSION:-1.1.0}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "ETS2_VERSION must be a version like 1.1.0." >&2
    exit 2
fi

temp="$(mktemp -d "${TMPDIR:-/tmp}/ets2-save-editor.XXXXXXXX")"
trap 'rm -rf "$temp"' EXIT
app="$temp/ETS2 Save Editor.app"
mkdir -p "$app/Contents/"{MacOS,Helpers,Resources} dist

for arch in "${architectures[@]}"; do
    goarch="$arch"
    if [[ "$arch" == x86_64 ]]; then goarch=amd64; fi
    CGO_ENABLED=0 GOOS=darwin GOARCH="$goarch" \
        go build -p 2 -trimpath -ldflags='-s -w' -o "$temp/core-$arch" .
    xcrun swiftc -parse-as-library -O -target "$arch-apple-macos13.0" \
        Editor.swift -o "$temp/ui-$arch"
done

core="$app/Contents/Helpers/ets2-save-core"
ui="$app/Contents/MacOS/ETS2 Save Editor"
if [[ "$target" == universal ]]; then
    xcrun lipo -create "$temp/core-arm64" "$temp/core-x86_64" -output "$core"
    xcrun lipo -create "$temp/ui-arm64" "$temp/ui-x86_64" -output "$ui"
else
    cp "$temp/core-$target" "$core"
    cp "$temp/ui-$target" "$ui"
fi

cp scripts/Info.plist "$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $version" "$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $version" "$app/Contents/Info.plist"
cp LICENSE "$app/Contents/Resources/LICENSE.txt"
codesign --force --sign - "$core"
codesign --force --sign - "$app"
codesign --verify --strict --deep "$app"
ditto -c -k --sequesterRsrc --keepParent "$app" "$temp/ETS2-Save-Editor-macOS-$target.zip"

# Replace only the known build outputs after the new package has been verified.
rm -rf "dist/ETS2 Save Editor.app"
ditto "$app" "dist/ETS2 Save Editor.app"
mv -f "$temp/ETS2-Save-Editor-macOS-$target.zip" "dist/ETS2-Save-Editor-macOS-$target.zip"
printf 'dist/ETS2 Save Editor.app\ndist/ETS2-Save-Editor-macOS-%s.zip\n' "$target"
