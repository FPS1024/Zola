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
if [ "$GOOS_VALUE" != "linux" ]; then
	echo "error: .deb packages must be built on linux, current GOOS is $GOOS_VALUE" >&2
	exit 1
fi

case "$GOARCH_VALUE" in
	amd64) DEB_ARCH=amd64 ;;
	arm64) DEB_ARCH=arm64 ;;
	386) DEB_ARCH=i386 ;;
	arm) DEB_ARCH=armhf ;;
	*)
		echo "error: unsupported Debian architecture: $GOARCH_VALUE" >&2
		exit 1
		;;
esac

if [ -n "${ZOLA_SERVICE_USER:-}" ]; then
	SERVICE_USER="$ZOLA_SERVICE_USER"
elif [ -n "${SUDO_USER:-}" ] && [ "$SUDO_USER" != "root" ]; then
	SERVICE_USER="$SUDO_USER"
else
	SERVICE_USER="$(id -un)"
fi

SERVICE_HOME="$(getent passwd "$SERVICE_USER" | cut -d: -f6)"
if [ -z "$SERVICE_HOME" ]; then
	echo "error: could not resolve home directory for user $SERVICE_USER" >&2
	exit 1
fi
ZOLA_CONFIG_DIR="$SERVICE_HOME/.config/zola"

PKG_NAME="zola_${DEB_VERSION}_${DEB_ARCH}"
PKG_ROOT="$OUT_DIR/$PKG_NAME"
DEB_FILE="$OUT_DIR/$PKG_NAME.deb"

rm -rf "$PKG_ROOT"
mkdir -p \
	"$PKG_ROOT/DEBIAN" \
	"$PKG_ROOT/usr/bin" \
	"$PKG_ROOT/usr/share/doc/zola" \
	"$PKG_ROOT/etc/default" \
	"$PKG_ROOT/lib/systemd/system"

LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X $MODULE.Version=$VERSION"
LDFLAGS="$LDFLAGS -X $MODULE.Commit=$COMMIT"
LDFLAGS="$LDFLAGS -X $MODULE.BuildTime=$BUILD_TIME"

env CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH_VALUE" \
	"$GO_BIN" build \
	-trimpath \
	-buildvcs=false \
	-ldflags "$LDFLAGS" \
	-o "$PKG_ROOT/usr/bin/zola" \
	./cmd/zola

install -m 0755 packaging/debian/postinst "$PKG_ROOT/DEBIAN/postinst"
install -m 0755 packaging/debian/prerm "$PKG_ROOT/DEBIAN/prerm"
install -m 0755 packaging/debian/postrm "$PKG_ROOT/DEBIAN/postrm"
install -m 0644 packaging/default/zola "$PKG_ROOT/etc/default/zola"
install -m 0644 README.md "$PKG_ROOT/usr/share/doc/zola/README.md"
install -m 0644 LICENSE "$PKG_ROOT/usr/share/doc/zola/LICENSE"

sed \
	-e "s|@ARCH@|$DEB_ARCH|g" \
	-e "s|@VERSION@|$DEB_VERSION|g" \
	packaging/debian/control.in > "$PKG_ROOT/DEBIAN/control"

sed \
	-e "s|@SERVICE_USER@|$SERVICE_USER|g" \
	-e "s|@SERVICE_HOME@|$SERVICE_HOME|g" \
	-e "s|@ZOLA_CONFIG_DIR@|$ZOLA_CONFIG_DIR|g" \
	packaging/systemd/zola-proxy.service.in > "$PKG_ROOT/lib/systemd/system/zola-proxy.service"

dpkg-deb --root-owner-group --build "$PKG_ROOT" "$DEB_FILE"

if command -v sha256sum >/dev/null 2>&1; then
	sha256sum "$DEB_FILE" > "$DEB_FILE.sha256"
elif command -v shasum >/dev/null 2>&1; then
	shasum -a 256 "$DEB_FILE" > "$DEB_FILE.sha256"
fi

echo "Built $DEB_FILE"
echo "Service user: $SERVICE_USER"
echo "Service config: $ZOLA_CONFIG_DIR"
