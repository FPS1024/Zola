# Zola Build Guide

This document covers source builds, cross-compilation, release artifacts, and
Debian packaging. For installation and runtime usage, see
[Usage Guide](usage.en.md).

## Requirements

- Go 1.22 or newer
- Linux or macOS for desktop/server builds
- A native `GOOS=ios GOARCH=arm64` Go toolchain for native iOS builds
- Xcode and an iOS SDK for macOS-to-iOS builds
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

The iOS target requires external linking:

```sh
make build-ios
```

This works directly on a jailbroken iPhone with a native `ios/arm64` Go
toolchain.

From macOS with Xcode and the iOS SDK:

```sh
make release-ios
```

Build standard targets plus iOS:

```sh
./scripts/build-all.sh all-with-ios
```

Build selected platforms:

```sh
./scripts/build-all.sh linux-amd64
./scripts/build-all.sh darwin-arm64
./scripts/build-all.sh ios-arm64
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

