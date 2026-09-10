#!/usr/bin/env sh
set -eu

VERSION="${VERSION:-v1.0.0}"
OUT_DIR="${OUT_DIR:-dist}"
MODULE=zola/internal/buildinfo
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

case "$VERSION" in
	v*) ;;
	*) VERSION="v$VERSION" ;;
esac

if command -v go >/dev/null 2>&1; then
	GO_BIN="$(command -v go)"
elif [ -n "${GOROOT:-}" ] && [ -x "$GOROOT/bin/go" ]; then
	GO_BIN="$GOROOT/bin/go"
else
	echo "error: go binary not found on PATH" >&2
	exit 1
fi

GOOS_VALUE="$("$GO_BIN" env GOOS)"
GOARCH_VALUE="$("$GO_BIN" env GOARCH)"
if [ "$GOOS_VALUE" != "ios" ] || [ "$GOARCH_VALUE" != "arm64" ]; then
	echo "error: this script must run on an iPhone with a native ios/arm64 Go toolchain" >&2
	echo "current Go target: $GOOS_VALUE/$GOARCH_VALUE" >&2
	exit 1
fi

PLATFORM=ios-arm64
BIN_NAME="zola-$VERSION-$PLATFORM"
BUILD_DIR="$OUT_DIR/$PLATFORM"
BIN_PATH="$BUILD_DIR/$BIN_NAME"

LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X $MODULE.Version=$VERSION"
LDFLAGS="$LDFLAGS -X $MODULE.Commit=$COMMIT"
LDFLAGS="$LDFLAGS -X $MODULE.BuildTime=$BUILD_TIME"
LDFLAGS="$LDFLAGS -r ${IOS_RPATH:-/var/jb/usr/lib}"

rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

env CGO_ENABLED=1 GOOS=ios GOARCH=arm64 \
	"$GO_BIN" build \
	-trimpath \
	-buildvcs=false \
	-ldflags "$LDFLAGS" \
	-o "$BIN_PATH" \
	./cmd/zola

if command -v ldid >/dev/null 2>&1; then
	ldid -S "$BIN_PATH"
	echo "Signed with ldid: $BIN_PATH"
fi

if [ "${SKIP_ARCHIVE:-0}" = "1" ]; then
	echo "Built $BIN_PATH"
	exit 0
fi

[ -f README.md ] && cp README.md "$BUILD_DIR/README.md"
[ -f LICENSE ] && cp LICENSE "$BUILD_DIR/LICENSE"

set -- "$BIN_NAME"
[ -f "$BUILD_DIR/README.md" ] && set -- "$@" README.md
[ -f "$BUILD_DIR/LICENSE" ] && set -- "$@" LICENSE
if command -v gzip >/dev/null 2>&1; then
	ARCHIVE="$OUT_DIR/zola-$VERSION-$PLATFORM.tar.gz"
	tar -czf "$ARCHIVE" -C "$BUILD_DIR" "$@"
else
	ARCHIVE="$OUT_DIR/zola-$VERSION-$PLATFORM.tar"
	tar -cf "$ARCHIVE" -C "$BUILD_DIR" "$@"
fi

if command -v sha256sum >/dev/null 2>&1; then
	sha256sum "$ARCHIVE" > "$ARCHIVE.sha256"
elif command -v shasum >/dev/null 2>&1; then
	shasum -a 256 "$ARCHIVE" > "$ARCHIVE.sha256"
fi

echo "Built $BIN_PATH"
echo "Built $ARCHIVE"
