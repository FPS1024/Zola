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
bin/zola-v1.0.1-linux-amd64
bin/zola-v1.0.1-linux-arm64
bin/zola-v1.0.1-darwin-arm64
bin/zola-v1.0.1-windows-amd64.exe
bin/zola-v1.0.1-ios-arm64
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
dist/zola-v1.0.1-linux-amd64.tar.gz
dist/zola-v1.0.1-linux-amd64/zola-v1.0.1-linux-amd64
dist/zola-v1.0.1-windows-amd64.zip
dist/zola-v1.0.1-windows-amd64/zola-v1.0.1-windows-amd64.exe
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

只生成 rootless 包：

```text
dist/zola_1.0.1_iphoneos-arm64.deb
```

安装路径固定为：

```text
/var/jb/usr/bin/zola
/var/jb/Library/LaunchDaemons/com.fps1024.zola.proxy.plist
```

iOS service 固定使用 root 用户，并固定使用 rootless HOME：

```text
UserName = root
HOME = /var/jb/var/root
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
dpkg -i dist/zola_1.0.1_iphoneos-arm64.deb
```

安装后，以与 plist 一致的 HOME 配置一次当前 Provider：

```sh
HOME=/var/jb/var/root \
zola proxy use deepseek
```

让后台 proxy 重新读取配置：

```sh
zola service restart
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
dist/zola_1.0.1_amd64.deb
dist/zola_1.0.1_amd64.deb.sha256
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
sudo env ZOLA_SERVICE_USER=admin dpkg -i dist/zola_1.0.1_amd64.deb
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
