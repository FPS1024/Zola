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
	bin_name="$2"
	archive="$OUT_DIR/zola-$VERSION-$platform.tar.gz"
	set -- "$bin_name"
	[ -f README.md ] && set -- "$@" README.md
	[ -f LICENSE ] && set -- "$@" LICENSE
	tar -czf "$archive" -C "$OUT_DIR/$platform" "$@"
	echo "$archive"
}

package_windows() {
	platform="$1"
	bin_name="$2"
	archive="$OUT_DIR/zola-$VERSION-$platform.zip"
	archive_abs="$PWD/$archive"
	workdir="$PWD/$OUT_DIR/$platform"
	set -- "$bin_name"
	[ -f "$workdir/README.md" ] && set -- "$@" README.md
	[ -f "$workdir/LICENSE" ] && set -- "$@" LICENSE
	if command -v zip >/dev/null 2>&1; then
		(cd "$workdir" && zip -q -r "$archive_abs" "$@")
	elif command -v python3 >/dev/null 2>&1; then
		(cd "$workdir" && python3 -m zipfile -c "$archive_abs" "$@")
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
	cgo_enabled="${5:-0}"

	if [ "$goos" = "ios" ]; then
		host_os="$("$GO_BIN" env GOHOSTOS)"
		if [ "$host_os" != "ios" ] && [ "$host_os" != "darwin" ]; then
			echo "error: ios/$goarch requires a native ios Go toolchain or macOS with Xcode/iOS SDK; current Go host is $host_os" >&2
			exit 1
		fi
	fi

	build_dir="$OUT_DIR/$platform"
	mkdir -p "$build_dir"

	bin_name="zola-$VERSION-$platform"
	if [ "$goos" = "windows" ]; then
		bin_name="$bin_name.exe"
	fi

	env \
		CGO_ENABLED="$cgo_enabled" \
		GOOS="$goos" \
		GOARCH="$goarch" \
		${goarm:+GOARM="$goarm"} \
		"$GO_BIN" build \
		-trimpath \
		-buildvcs=false \
		-ldflags "$LDFLAGS" \
		-o "$build_dir/$bin_name" \
		./cmd/zola

	[ -f README.md ] && cp README.md "$build_dir/README.md"
	[ -f LICENSE ] && cp LICENSE "$build_dir/LICENSE"

	if [ "$goos" = "windows" ]; then
		package_windows "$platform" "$bin_name"
	else
		package_unix "$platform" "$bin_name"
	fi
}

build_named() {
	case "$1" in
	linux-386) build_target linux 386 "" linux-386 0 ;;
	linux-amd64) build_target linux amd64 "" linux-amd64 0 ;;
	linux-armv7) build_target linux arm 7 linux-armv7 0 ;;
	linux-arm64) build_target linux arm64 "" linux-arm64 0 ;;
	darwin-amd64) build_target darwin amd64 "" darwin-amd64 0 ;;
	darwin-arm64) build_target darwin arm64 "" darwin-arm64 0 ;;
	windows-386) build_target windows 386 "" windows-386 0 ;;
	windows-amd64) build_target windows amd64 "" windows-amd64 0 ;;
	windows-arm64) build_target windows arm64 "" windows-arm64 0 ;;
	ios-arm64) build_target ios arm64 "" ios-arm64 1 ;;
	*)
		echo "error: unknown platform '$1'" >&2
		echo "supported: linux-386 linux-amd64 linux-armv7 linux-arm64 darwin-amd64 darwin-arm64 windows-386 windows-amd64 windows-arm64 ios-arm64" >&2
		exit 1
		;;
	esac
}

build_all() {
	build_named linux-386
	build_named linux-amd64
	build_named linux-armv7
	build_named linux-arm64
	build_named darwin-amd64
	build_named darwin-arm64
	build_named windows-386
	build_named windows-amd64
	build_named windows-arm64
}

if [ "$#" -eq 0 ]; then
	build_all
else
	for platform in "$@"; do
		case "$platform" in
		all) build_all ;;
		all-with-ios)
			build_all
			build_named ios-arm64
			;;
		*) build_named "$platform" ;;
		esac
	done
fi

ARCHIVES=""
for file in "$OUT_DIR"/zola-"$VERSION"-*.tar.gz "$OUT_DIR"/zola-"$VERSION"-*.zip; do
	[ -f "$file" ] || continue
	ARCHIVES="$ARCHIVES $file"
done

if [ -n "$ARCHIVES" ]; then
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum $ARCHIVES > "$OUT_DIR/SHA256SUMS"
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 $ARCHIVES > "$OUT_DIR/SHA256SUMS"
	else
		echo "warning: sha256sum/shasum not found, skipping SHA256SUMS" >&2
	fi
fi

echo "Build complete: $OUT_DIR"
