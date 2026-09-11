# Zola 中文编译指南

本文只讲源码构建、交叉编译、发布产物和 Debian 打包。安装后的使用方式见
[中文使用说明](usage.zh-CN.md)。

## 环境要求

- Go 1.22 或更高版本
- Linux/macOS 上构建桌面平台产物
- iOS 原生编译需要越狱 iPhone 上的 `GOOS=ios GOARCH=arm64` Go 工具链
- iOS 打包需要 Procursus 的 `libiosexec1`
- 构建 deb 需要 Linux 和 `dpkg-deb`

## 本机构建

```sh
make build
```

Makefile 会自动读取当前 Go 环境：

```text
GOOS
GOARCH
GOARM
```

输出文件名会包含版本、系统和架构：

```text
bin/zola-v1.0.0-linux-amd64
bin/zola-v1.0.0-linux-arm64
bin/zola-v1.0.0-darwin-arm64
bin/zola-v1.0.0-windows-amd64.exe
bin/zola-v1.0.0-ios-arm64
```

### 单平台目标

```sh
make build-linux
make build-darwin
make build-windows
make build-ios
```

这些目标要求当前 Go 工具链与目标平台一致。环境不匹配时会直接报错，不会隐式
交叉编译。

## 标准交叉编译

```sh
make release
```

标准 release 会构建：

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

输出：

```text
dist/zola-v1.0.0-linux-amd64.tar.gz
dist/zola-v1.0.0-linux-amd64/zola-v1.0.0-linux-amd64
dist/zola-v1.0.0-windows-amd64.zip
dist/zola-v1.0.0-windows-amd64/zola-v1.0.0-windows-amd64.exe
dist/SHA256SUMS
```

归档内同时包含 `README.md` 和 `LICENSE`。

## iOS arm64

iOS 有独立编译脚本，只能在越狱 iPhone 上运行：

```sh
make build-ios
```

脚本会检查：

```text
go env GOOS   = ios
go env GOARCH = arm64
```

如果当前设备不是 iPhone 的 `ios/arm64` Go 环境，脚本会立即失败，不会尝试
交叉编译。如果设备安装了 `ldid`，脚本会自动签名。

独立构建默认生成 `.tar.gz`。如果 iPhone 没有 `gzip`，会自动退回未压缩的
`.tar`；`deb-ios` 不生成归档，不受该问题影响。

单独选择平台：

```sh
./scripts/build-all.sh linux-amd64
./scripts/build-all.sh darwin-arm64
```

`build-all.sh` 不接受 `ios-arm64`，避免在 Linux/macOS 上误生成不可执行的
iOS 产物。

### iOS deb

在 iPhone 上生成包含二进制和 launchd 服务的 deb：

```sh
make deb-ios
```

默认只生成当前 rootless 设备的安装包：

```text
dist/zola_1.0.0_iphoneos-arm64.deb
```

脚本优先读取 `dpkg --print-architecture`，再退回 `uname -m`，以选择正确的
Procursus 架构。`arm64` 设备生成 `iphoneos-arm64`，`arm64e` 设备生成
`iphoneos-arm64e`。对应关系是：

- `iphoneos-arm64`：rootless，安装到 `/var/jb`
- `iphoneos-arm64e`：arm64e rootless，安装到 `/var/jb`

如果设备架构识别错误，可以显式覆盖：

```sh
ZOLA_IOS_DEVICE_ARCH=arm64 make deb-ios
# 或单独指定 rootless 包架构
ZOLA_IOS_ROOTLESS_ARCH=iphoneos-arm64 make deb-ios
```

需要同时生成 rootful 包时：

```sh
IOS_LAYOUT=all make deb-ios
```

默认 service 用户是 `mobile`，这是 rootless Codex 的推荐配置。建议固定
service 使用的 Provider，避免空 state 导致服务启动失败：

```sh
ZOLA_SERVICE_USER=mobile ZOLA_PROVIDER=deepseek make deb-ios
```

建议在构建前安装 Procursus 的 `libiosexec1`。构建脚本会在
`/var/jb/usr/lib/libiosexec.1.dylib` 存在时显式链接 shim；如果不存在会警告
并生成不带 shim 的二进制。安装 deb 时也需要这个依赖：

```sh
apt update
apt install libiosexec1
```

rootless 二进制默认写入 runtime path：

```text
/var/jb/usr/lib
```

rootless deb 内包含：

```text
/var/jb/usr/bin/zola
/var/jb/Library/LaunchDaemons/com.fps1024.zola.proxy.plist
```

安装 rootless 包：

```sh
dpkg -i dist/zola_1.0.0_iphoneos-arm64.deb
```

安装后，以 `mobile` service 用户配置一次当前 Provider。当前 shell 是
`mobile` 时：

```sh
zola proxy use deepseek
```

如果当前是 root，需要把配置写到 mobile 用户的目录：

```sh
HOME=/var/mobile \
ZOLA_CONFIG_DIR=/var/mobile/.config/zola \
zola proxy use deepseek
```

让后台 proxy 重新读取这份配置：

```sh
sudo zola service restart
```

如果 rootless 环境中的实际配置目录不同，可以在打包时显式指定：

```sh
ZOLA_SERVICE_USER=mobile \
ZOLA_PROVIDER=deepseek \
ZOLA_CONFIG_DIR_OVERRIDE=/actual/path/.config/zola \
make deb-ios
```

launchd 会通过 `RunAtLoad` 和 `KeepAlive` 在开机后自动启动代理。

安装后检查后台任务和代理健康状态：

```sh
zola service status
```

只有同时看到 `Launchd: loaded` 和 `Proxy health: ok`，才表示 Codex 可以通过
后台 proxy 正常通信。如果未运行：

```sh
sudo zola service restart
zola service status
```

继续检查 launchd 和日志：

```sh
launchctl print system/com.fps1024.zola.proxy
tail -f /var/mobile/Library/Logs/zola-proxy.log
```

## Debian 包

```sh
make deb
```

输出：

```text
dist/zola_1.0.0_amd64.deb
dist/zola_1.0.0_amd64.deb.sha256
```

deb 内容：

```text
/usr/bin/zola
/etc/default/zola
/usr/share/zola/zola-proxy.service.in
/usr/share/doc/zola/README.md
/usr/share/doc/zola/LICENSE
```

构建时不会写入构建机器用户名。安装时由 `postinst` 生成：

```text
/etc/systemd/system/zola-proxy.service
```

安装时可以指定 service 用户：

```sh
sudo env ZOLA_SERVICE_USER=admin dpkg -i dist/zola_1.0.0_amd64.deb
```

## 版本注入

构建信息通过 `-ldflags` 写入：

```text
Version
Commit
BuildTime
```

覆盖版本：

```sh
make build VERSION=v1.2.0
make release VERSION=v1.2.0
make deb VERSION=v1.2.0
```

查看构建信息：

```sh
zola version
```

## 验证

```sh
make test
make vet
```
