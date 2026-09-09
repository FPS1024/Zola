#!/usr/bin/env sh
set -eu

VERSION="${VERSION:-v1.0.0}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
OUT_DIR="${OUT_DIR:-dist}"
MODULE=zola/internal/buildinfo

case "$VERSION" in
	v*) ;;
	*) VERSION="v$VERSION" ;;
esac

LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X $MODULE.Version=$VERSION"
LDFLAGS="$LDFLAGS -X $MODULE.Commit=$COMMIT"
LDFLAGS="$LDFLAGS -X $MODULE.BuildTime=$BUILD_TIME"

if command -v go >/dev/null 2>&1; then
	GO_BIN="$(command -v go)"
elif [ -n "${GOROOT:-}" ] && [ -x "$GOROOT/bin/go" ]; then
	GO_BIN="$GOROOT/bin/go"
else
	echo "error: go binary not found on PATH" >&2
	exit 1
fi

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

package_unix() {
	platform="$1"
	archive="$OUT_DIR/zola-$VERSION-$platform.tar.gz"
	tar -czf "$archive" -C "$OUT_DIR/$platform" zola
	echo "$archive"
}

package_windows() {
	platform="$1"
	archive="$OUT_DIR/zola-$VERSION-$platform.zip"
	archive_abs="$PWD/$archive"
	workdir="$PWD/$OUT_DIR/$platform"
	if command -v zip >/dev/null 2>&1; then
		(cd "$workdir" && zip -q -r "$archive_abs" zola.exe)
	elif command -v python3 >/dev/null 2>&1; then
		(cd "$workdir" && python3 -m zipfile -c "$archive_abs" zola.exe)
	else
		echo "error: zip or python3 is required for windows archives" >&2
		exit 1
	fi
	echo "$archive"
}

build_target() {
	goos="$1"
	goarch="$2"
	goarm="$3"
	platform="$4"

	build_dir="$OUT_DIR/$platform"
	mkdir -p "$build_dir"
	bin_name="zola"
	if [ "$goos" = "windows" ]; then
		bin_name="zola.exe"
	fi

	env \
		CGO_ENABLED=0 \
		GOOS="$goos" \
		GOARCH="$goarch" \
		${goarm:+GOARM="$goarm"} \
		"$GO_BIN" build \
		-trimpath \
		-buildvcs=false \
		-ldflags "$LDFLAGS" \
		-o "$build_dir/$bin_name" \
		./cmd/zola

	if [ "$goos" = "windows" ]; then
		package_windows "$platform"
	else
		package_unix "$platform"
	fi
}

build_all() {
	build_target linux 386 "" linux-x86
	build_target linux amd64 "" linux-x86_64
	build_target linux arm 7 linux-armv7
	build_target linux arm64 "" linux-arm64
	build_target darwin amd64 "" darwin-intel
	build_target darwin arm64 "" darwin-arm64
	build_target windows 386 "" windows-x86
	build_target windows amd64 "" windows-x64
	build_target windows arm64 "" windows-arm64
}

build_all

if command -v sha256sum >/dev/null 2>&1; then
	sha256sum "$OUT_DIR"/zola-"$VERSION"-*.tar.gz "$OUT_DIR"/zola-"$VERSION"-*.zip > "$OUT_DIR/SHA256SUMS"
elif command -v shasum >/dev/null 2>&1; then
	shasum -a 256 "$OUT_DIR"/zola-"$VERSION"-*.tar.gz "$OUT_DIR"/zola-"$VERSION"-*.zip > "$OUT_DIR/SHA256SUMS"
else
	echo "warning: sha256sum/shasum not found, skipping SHA256SUMS" >&2
fi

echo "Build complete: $OUT_DIR"
