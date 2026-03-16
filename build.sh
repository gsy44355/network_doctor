#!/bin/bash
VERSION=${1:-"dev"}
DIST="dist"
rm -rf $DIST && mkdir -p $DIST

platforms=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
)

for platform in "${platforms[@]}"; do
  GOOS="${platform%/*}"
  GOARCH="${platform#*/}"
  output="$DIST/network-doctor-${GOOS}-${GOARCH}"
  [ "$GOOS" = "windows" ] && output+=".exe"

  echo "Building $GOOS/$GOARCH..."
  GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o "$output" .
done

echo "Done. Binaries in $DIST/"
ls -lh $DIST/