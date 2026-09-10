# Zola 中文编译指南

本文只讲源码构建、交叉编译、发布产物和 Debian 打包。安装后的使用方式见
[中文使用说明](usage.zh-CN.md)。

## 环境要求

- Go 1.22 或更高版本
- Linux/macOS 上构建桌面平台产物
- iOS 原生编译需要越狱 iPhone 上的 `GOOS=ios GOARCH=arm64` Go 工具链
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

单独选择平台：

```sh
./scripts/build-all.sh linux-amd64
./scripts/build-all.sh darwin-arm64
```

`build-all.sh` 不接受 `ios-arm64`，避免在 Linux/macOS 上误生成不可执行的
iOS 产物。

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
