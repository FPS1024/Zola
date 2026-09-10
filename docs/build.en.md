# Zola Build Guide

This document covers source builds, cross-compilation, release artifacts, and
Debian packaging. For installation and runtime usage, see
[Usage Guide](usage.en.md).

## Requirements

- Go 1.22 or newer
- Linux or macOS for desktop/server builds
- A jailbroken iPhone with a native `GOOS=ios GOARCH=arm64` Go toolchain
- Linux and `dpkg-deb` for Debian packages

## Native Build

```sh
make build
```

The Makefile detects:

```text
GOOS
GOARCH
GOARM
```

Output names include version, OS, and architecture:

```text
bin/zola-v1.0.0-linux-amd64
bin/zola-v1.0.0-linux-arm64
bin/zola-v1.0.0-darwin-arm64
bin/zola-v1.0.0-windows-amd64.exe
bin/zola-v1.0.0-ios-arm64
```

### Single Platform Targets

```sh
make build-linux
make build-darwin
make build-windows
make build-ios
```

These targets require the native Go toolchain to match the target. A mismatch
fails immediately instead of performing an implicit cross-build.

## Standard Cross-Build

```sh
make release
```

Targets:

```text
linux-386
linux-amd64
linux-armv7
linux-arm64
darwin-amd64
darwin-arm64
windows-386
windows-amd64
windows-arm64
```

Output:

```text
dist/zola-v1.0.0-linux-amd64.tar.gz
dist/zola-v1.0.0-linux-amd64/zola-v1.0.0-linux-amd64
dist/zola-v1.0.0-windows-amd64.zip
dist/zola-v1.0.0-windows-amd64/zola-v1.0.0-windows-amd64.exe
dist/SHA256SUMS
```

Archives include `README.md` and `LICENSE`.

## iOS arm64

iOS has a dedicated build script that only runs on a jailbroken iPhone:

```sh
make build-ios
```

The script requires:

```text
go env GOOS   = ios
go env GOARCH = arm64
```

It fails immediately on any other host instead of attempting a cross-build. If
`ldid` is installed on the device, the binary is signed automatically.

Standalone builds produce `.tar.gz` by default. If `gzip` is unavailable on
the iPhone, the script falls back to an uncompressed `.tar`; `deb-ios` skips
archives entirely.

Build selected platforms:

```sh
./scripts/build-all.sh linux-amd64
./scripts/build-all.sh darwin-arm64
```

`build-all.sh` intentionally rejects `ios-arm64` so Linux and macOS cannot
produce an unusable iOS artifact by mistake.

### iOS deb

Build a deb containing the binary and launchd service on the iPhone:

```sh
make deb-ios
```

Output:

```text
dist/zola_1.0.0_iphoneos-arm64.deb
dist/zola_1.0.0_iphoneos-arm64e.deb
```

Both packages are generated in one run with the same binary:

- `iphoneos-arm64`: rootless, installed under `/var/jb`
- `iphoneos-arm64e`: rootful, installed under `/usr` and `/Library`

The default service user is `mobile`. If both Codex and Zola run as root:

```sh
ZOLA_SERVICE_USER=root make deb-ios
```

The iOS deb depends on Procursus `libiosexec1`. If `dpkg` does not install it
automatically:

```sh
apt update
apt install libiosexec1
```

The binary embeds this runtime path:

```text
/var/jb/usr/lib
```

This lets rootless iOS locate `libiosexec.1.dylib`.

Install:

```sh
dpkg -i dist/zola_1.0.0_iphoneos-arm64.deb
# or
dpkg -i dist/zola_1.0.0_iphoneos-arm64e.deb
```

Configure the provider once as the service user:

```sh
zola proxy use deepseek
```

`RunAtLoad` and `KeepAlive` make the launchd service start after boot.

Check:

```sh
launchctl print system/com.fps1024.zola.proxy
tail -f /var/mobile/Library/Logs/zola-proxy.log
```

## Debian Package

```sh
make deb
```

Output:

```text
dist/zola_1.0.0_amd64.deb
dist/zola_1.0.0_amd64.deb.sha256
```

Package contents:

```text
/usr/bin/zola
/etc/default/zola
/usr/share/zola/zola-proxy.service.in
/usr/share/doc/zola/README.md
/usr/share/doc/zola/LICENSE
```

The build does not embed the build machine username. Installation creates:

```text
/etc/systemd/system/zola-proxy.service
```

Choose the service user explicitly:

```sh
sudo env ZOLA_SERVICE_USER=admin dpkg -i dist/zola_1.0.0_amd64.deb
```

## Version Injection

Build metadata is injected with `-ldflags`:

```text
Version
Commit
BuildTime
```

Override the version:

```sh
make build VERSION=v1.2.0
make release VERSION=v1.2.0
make deb VERSION=v1.2.0
```

Inspect build metadata:

```sh
zola version
```

## Verification

```sh
make test
make vet
```
