#!/bin/sh
# Gera FichaB3Completa.app (Apple Silicon + Intel) e o zip de distribuição em ./dist
set -e
cd "$(dirname "$0")/.."
GO=${GO:-go}
command -v "$GO" >/dev/null || GO=/usr/local/go/bin/go
rm -rf dist && mkdir -p dist/FichaB3Completa.app/Contents/MacOS dist/FichaB3Completa.app/Contents/Resources
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 "$GO" build -trimpath -ldflags "-s -w" -o dist/arm64 .
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 "$GO" build -trimpath -ldflags "-s -w" -o dist/amd64 .
lipo -create dist/arm64 dist/amd64 -output dist/FichaB3Completa.app/Contents/MacOS/FichaB3Completa
rm dist/arm64 dist/amd64
cp empacotar/Info.plist empacotar/PkgInfo dist/FichaB3Completa.app/Contents/
cp empacotar/FichaB3Completa.icns dist/FichaB3Completa.app/Contents/Resources/
codesign --force --deep -s - dist/FichaB3Completa.app
cp empacotar/Leia-me.txt dist/
VER=$(plutil -extract CFBundleShortVersionString raw empacotar/Info.plist)
(cd dist && zip -qry "FichaB3Completa-$VER.zip" FichaB3Completa.app Leia-me.txt)
echo "pronto: dist/FichaB3Completa.app e dist/FichaB3Completa-$VER.zip"
