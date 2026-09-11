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

SERVICE_USER="${ZOLA_SERVICE_USER:-mobile}"
ZOLA_PROVIDER="${ZOLA_PROVIDER:-}"
case "$SERVICE_USER" in
	mobile) SERVICE_HOME=/var/mobile ;;
	root) SERVICE_HOME=/var/root ;;
	*)
		if [ -z "${ZOLA_SERVICE_HOME:-}" ]; then
			echo "error: set ZOLA_SERVICE_HOME for non-standard service user $SERVICE_USER" >&2
			exit 1
		fi
		SERVICE_HOME="$ZOLA_SERVICE_HOME"
		;;
esac

DEVICE_ARCH="${ZOLA_IOS_DEVICE_ARCH:-}"
if [ -z "$DEVICE_ARCH" ] && command -v dpkg >/dev/null 2>&1; then
	DEVICE_ARCH="$(dpkg --print-architecture 2>/dev/null || true)"
fi
if [ -z "$DEVICE_ARCH" ]; then
	DEVICE_ARCH="$(uname -m)"
fi
# Procursus uses the legacy iphoneos-arm architecture for rootful arm64 packages.
case "$DEVICE_ARCH" in
	arm64|iphoneos-arm|iphoneos-arm64)
		DEFAULT_ROOTLESS_ARCH=iphoneos-arm64
		DEFAULT_ROOTFUL_ARCH=iphoneos-arm
		;;
	arm64e|iphoneos-arm64e)
		DEFAULT_ROOTLESS_ARCH=iphoneos-arm64e
		DEFAULT_ROOTFUL_ARCH=iphoneos-arm64e
		;;
	*)
		echo "error: unsupported iOS device architecture: $DEVICE_ARCH" >&2
		echo "set ZOLA_IOS_DEVICE_ARCH to arm64, arm64e, iphoneos-arm, iphoneos-arm64, or iphoneos-arm64e" >&2
		exit 1
		;;
esac
ROOTLESS_ARCH="${ZOLA_IOS_ROOTLESS_ARCH:-$DEFAULT_ROOTLESS_ARCH}"
ROOTFUL_ARCH="${ZOLA_IOS_ROOTFUL_ARCH:-$DEFAULT_ROOTFUL_ARCH}"

build_package() {
	layout="$1"

	case "$layout" in
	rootless)
		ARCH="$ROOTLESS_ARCH"
		IOS_LIB_DIR=/var/jb/usr/lib
		IOS_RPATH=/var/jb/usr/lib
		BIN_DIR=/var/jb/usr/bin
		PLIST_DIR=/var/jb/Library/LaunchDaemons
		DOC_DIR=/var/jb/usr/share/doc/zola
		LAUNCHCTL=/var/jb/usr/bin/launchctl
		;;
	rootful)
		ARCH="$ROOTFUL_ARCH"
		IOS_LIB_DIR=/usr/lib
		IOS_RPATH=/usr/lib
		BIN_DIR=/usr/bin
		PLIST_DIR=/Library/LaunchDaemons
		DOC_DIR=/usr/share/doc/zola
		LAUNCHCTL=/bin/launchctl
		;;
	*)
		echo "error: unsupported iOS layout: $layout" >&2
		exit 1
		;;
	esac

	SKIP_ARCHIVE=1 \
		IOS_LIB_DIR="$IOS_LIB_DIR" \
		IOS_RPATH="$IOS_RPATH" \
		VERSION="$VERSION" \
		OUT_DIR="$OUT_DIR" \
		./scripts/build-ios.sh
	SOURCE_BIN="$OUT_DIR/ios-arm64/zola-$VERSION-ios-arm64"

	ZOLA_CONFIG_DIR="${ZOLA_CONFIG_DIR_OVERRIDE:-$SERVICE_HOME/.config/zola}"
	LOG_DIR="$SERVICE_HOME/Library/Logs"
	LOG_PATH="$LOG_DIR/zola-proxy.log"
	ZOLA_BIN="$BIN_DIR/zola"
	PLIST_PATH="$PLIST_DIR/com.fps1024.zola.proxy.plist"

	PKG_NAME="zola_${DEB_VERSION}_${ARCH}"
	if [ "$layout" = "rootful" ]; then
		PKG_NAME="zola-rootful_${DEB_VERSION}_${ARCH}"
	fi
	PKG_ROOT="$OUT_DIR/$PKG_NAME"
	DEB_FILE="$OUT_DIR/$PKG_NAME.deb"

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
		-e "s|@SERVICE_USER@|$SERVICE_USER|g" \
		-e "s|@SERVICE_HOME@|$SERVICE_HOME|g" \
		-e "s|@ZOLA_CONFIG_DIR@|$ZOLA_CONFIG_DIR|g" \
		-e "s|@ZOLA_PROVIDER@|$ZOLA_PROVIDER|g" \
		-e "s|@BIN_DIR@|$BIN_DIR|g" \
		-e "s|@LOG_PATH@|$LOG_PATH|g" \
		packaging/ios/launchd.plist.in > "$PKG_ROOT$PLIST_PATH"

	sed \
		-e "s|@PLIST_PATH@|$PLIST_PATH|g" \
		-e "s|@LAUNCHCTL@|$LAUNCHCTL|g" \
		-e "s|@SERVICE_USER@|$SERVICE_USER|g" \
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
}

IOS_LAYOUT="${IOS_LAYOUT:-rootless}"
case "$IOS_LAYOUT" in
	rootless)
		build_package rootless
		;;
	rootful)
		build_package rootful
		;;
	all)
		build_package rootless
		build_package rootful
		;;
	*)
		echo "error: IOS_LAYOUT must be rootless, rootful, or all" >&2
		exit 1
		;;
esac

echo "Service user: $SERVICE_USER"
echo "iOS layout: $IOS_LAYOUT"
case "$IOS_LAYOUT" in
	rootless) echo "Rootless architecture: $ROOTLESS_ARCH" ;;
	rootful) echo "Rootful architecture: $ROOTFUL_ARCH" ;;
	all)
		echo "Rootless architecture: $ROOTLESS_ARCH"
		echo "Rootful architecture: $ROOTFUL_ARCH"
		;;
esac
