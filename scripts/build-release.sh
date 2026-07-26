#!/bin/bash
# Builds release archives for all supported platforms plus a source-only
# bundle and checksums.txt. Run from the repo root:
#
#   scripts/build-release.sh 1.2.0
#
# Output lands in ./dist (gitignored). Upload its contents as release
# assets -- nothing here gets committed.
set -euo pipefail

VERSION="${1:?usage: build-release.sh <version, e.g. 1.2.0>}"
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="$REPO/dist"

rm -rf "$DIST"
mkdir -p "$DIST"

build_target() {
  local goos="$1" goarch="$2" ext="$3"
  local name="sshbox_${VERSION}_${goos}_${goarch}"
  local stage="$DIST/$name"

  mkdir -p "$stage/bin"
  echo "building $goos/$goarch..."
  ( cd "$REPO" && CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -ldflags "-X main.version=v${VERSION}" -o "$stage/bin/sshbox${ext}" . )
  cp "$REPO/LICENSE" "$stage/LICENSE"
  cp "$REPO/scripts/release-readme.txt" "$stage/README.txt"

  if [ "$goos" = "windows" ]; then
    ( cd "$DIST" && zip -qr "${name}.zip" "$name" )
  else
    ( cd "$DIST" && tar -czf "${name}.tar.gz" "$name" )
  fi
  rm -rf "$stage"
}

build_target linux   amd64 ""
build_target linux   arm64 ""
build_target darwin  arm64 ""
build_target darwin  amd64 ""
build_target windows amd64 ".exe"

# minimal source bundle for people who want to compile it themselves
# without cloning the whole repo (docs, media, tape scripts, etc)
SRC_NAME="sshbox_${VERSION}_source"
SRC_STAGE="$DIST/$SRC_NAME"
mkdir -p "$SRC_STAGE"
cp -R "$REPO/internal" "$SRC_STAGE/internal"
cp "$REPO/main.go" "$SRC_STAGE/main.go"
cp "$REPO/go.mod" "$SRC_STAGE/go.mod"
cp "$REPO/go.sum" "$SRC_STAGE/go.sum"
cp "$REPO/config.toml" "$SRC_STAGE/config.toml"
cp "$REPO/LICENSE" "$SRC_STAGE/LICENSE"
cat > "$SRC_STAGE/README.txt" <<EOF
sshbox -- source only
======================

Everything needed to build sshbox yourself, nothing else -- no docs,
no media, no history. For the full picture see the repo.

Build:

    go build -ldflags "-X main.version=v${VERSION}" .

config.toml ships with the defaults already; edit it or use flags,
run \`go run . -h\` to see them all.

Full documentation, source history, and issue tracker:
https://github.com/kknxzz/sshbox

License: see LICENSE in this archive.
EOF
( cd "$DIST" && tar -czf "${SRC_NAME}.tar.gz" "$SRC_NAME" )
rm -rf "$SRC_STAGE"

echo ""
echo "generating checksums..."
( cd "$DIST" && shasum -a 256 *.tar.gz *.zip > checksums.txt )

echo ""
echo "done. artifacts:"
ls -la "$DIST"
