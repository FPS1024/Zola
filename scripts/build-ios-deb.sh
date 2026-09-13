#!/usr/bin/env sh
set -eu

VERSION="${VERSION:-v1.0.0}"
OUT_DIR="${OUT_DIR:-dist}"

case "$VERSION" in
	v*) ;;
	*) VERSION="v$VERSION" ;;
esac
DEB_VERSION="${VERSION#v}"

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

if ! command -v dpkg-deb >/dev/null 2>&1; then
	echo "error: dpkg-deb is required" >&2
	exit 1
fi
if ! command -v ldid >/dev/null 2>&1; then
	echo "error: ldid is required to sign the iOS binary" >&2
	exit 1
fi
if [ "$(id -u)" != "0" ]; then
	echo "error: run this script as root so package ownership is correct" >&2
	exit 1
fi

SKIP_ARCHIVE=1 VERSION="$VERSION" OUT_DIR="$OUT_DIR" ./scripts/build-ios.sh

ARCH=iphoneos-arm64
SOURCE_BIN="$OUT_DIR/ios-arm64/zola-$VERSION-ios-arm64"
PKG_NAME="zola_${DEB_VERSION}_${ARCH}"
PKG_ROOT="$OUT_DIR/$PKG_NAME"
DEB_FILE="$OUT_DIR/$PKG_NAME.deb"

BIN_DIR=/var/jb/usr/bin
PLIST_DIR=/var/jb/Library/LaunchDaemons
DOC_DIR=/var/jb/usr/share/doc/zola
ZOLA_BIN="$BIN_DIR/zola"
PLIST_PATH="$PLIST_DIR/com.fps1024.zola.proxy.plist"
LAUNCHCTL=/var/jb/usr/bin/launchctl
SERVICE_HOME=/var/jb/var/root
LOG_DIR=/var/mobile/Library/Logs
LOG_PATH="$LOG_DIR/zola-proxy.log"

rm -rf "$PKG_ROOT"
mkdir -p \
	"$PKG_ROOT/DEBIAN" \
	"$PKG_ROOT${BIN_DIR}" \
	"$PKG_ROOT${PLIST_DIR}" \
	"$PKG_ROOT${DOC_DIR}"

install -m 0755 "$SOURCE_BIN" "$PKG_ROOT$ZOLA_BIN"
install -m 0644 README.md "$PKG_ROOT$DOC_DIR/README.md"
install -m 0644 LICENSE "$PKG_ROOT$DOC_DIR/LICENSE"

sed \
	-e "s|@VERSION@|$DEB_VERSION|g" \
	-e "s|@ARCH@|$ARCH|g" \
	packaging/ios/control.in > "$PKG_ROOT/DEBIAN/control"

sed \
	-e "s|@ZOLA_BIN@|$ZOLA_BIN|g" \
	-e "s|@SERVICE_HOME@|$SERVICE_HOME|g" \
	-e "s|@BIN_DIR@|$BIN_DIR|g" \
	-e "s|@LOG_PATH@|$LOG_PATH|g" \
	packaging/ios/launchd.plist.in > "$PKG_ROOT$PLIST_PATH"

sed \
	-e "s|@PLIST_PATH@|$PLIST_PATH|g" \
	-e "s|@LAUNCHCTL@|$LAUNCHCTL|g" \
	-e "s|@LOG_DIR@|$LOG_DIR|g" \
	packaging/ios/postinst.in > "$PKG_ROOT/DEBIAN/postinst"

sed \
	-e "s|@PLIST_PATH@|$PLIST_PATH|g" \
	-e "s|@LAUNCHCTL@|$LAUNCHCTL|g" \
	packaging/ios/prerm.in > "$PKG_ROOT/DEBIAN/prerm"

sed \
	-e "s|@PLIST_PATH@|$PLIST_PATH|g" \
	-e "s|@LAUNCHCTL@|$LAUNCHCTL|g" \
	packaging/ios/postrm.in > "$PKG_ROOT/DEBIAN/postrm"

chmod 0755 "$PKG_ROOT/DEBIAN/postinst" "$PKG_ROOT/DEBIAN/prerm" "$PKG_ROOT/DEBIAN/postrm"
dpkg-deb --build "$PKG_ROOT" "$DEB_FILE"

if command -v sha256sum >/dev/null 2>&1; then
	sha256sum "$DEB_FILE" > "$DEB_FILE.sha256"
elif command -v shasum >/dev/null 2>&1; then
	shasum -a 256 "$DEB_FILE" > "$DEB_FILE.sha256"
fi

echo "Built $DEB_FILE"
