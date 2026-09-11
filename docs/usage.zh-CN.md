# Zola 中文使用说明

Zola 是一个 Codex Provider Manager，用来管理 Codex 的模型 Provider、API Key、
`~/.codex/config.toml`，并可选运行一个本地 Proxy。

本文以 Debian 系 Linux 和 systemd 为主，覆盖安装、配置和运行。源码编译、
交叉编译与 deb 构建见 [中文编译指南](build.zh-CN.md)。

## 1. 核心链路

普通 Direct Mode：

```text
codex -> Provider API
```

Proxy Mode：

```text
codex -> 127.0.0.1:8317 -> Zola Proxy -> Provider API
```

模型伪装也依赖 Proxy Mode：

```text
Codex 看到 gpt-5.4
Zola Proxy 改写为 deepseek-v4-pro
上游收到 deepseek-v4-pro
```

## 2. 安装 Debian 包

使用平时登录的管理员用户安装：

```sh
sudo dpkg -i dist/zola_1.0.0_amd64.deb
```

如果安装环境无法自动识别用户，可以明确指定：

```sh
sudo env ZOLA_SERVICE_USER=admin dpkg -i dist/zola_1.0.0_amd64.deb
```

安装内容：

```text
/usr/bin/zola
/etc/default/zola
/usr/share/zola/zola-proxy.service.in
/etc/systemd/system/zola-proxy.service
/usr/share/doc/zola/README.md
/usr/share/doc/zola/LICENSE
```

验证：

```sh
zola version
zola doctor
```

## 3. 添加 DeepSeek Provider

普通模式：

```sh
zola add deepseek \
  --name DeepSeek \
  --base-url https://api.deepseek.com \
  --model deepseek-v4-flash \
  --wire-api responses \
  --api-key '你的 DeepSeek API Key'
```

如果使用 DeepSeek Pro：

```sh
zola edit deepseek \
  --model deepseek-v4-pro
```

查看 Provider：

```sh
zola list
zola current
```

## 4. 模型伪装

模型伪装用于避免 Codex 报告自定义模型没有 metadata：

```text
Model metadata for `deepseek-v4-pro` not found
```

示例配置：

```sh
zola edit deepseek \
  --model deepseek-v4-pro \
  --codex-model gpt-5.4
```

含义：

```text
Codex model: gpt-5.4
Upstream model: deepseek-v4-pro
```

注意模型名必须准确。`gpt-5.4` 是正确名称，`gpt5.4` 会继续触发 metadata
警告。

模型伪装必须使用 Proxy Mode，因为模型名改写发生在 Zola Proxy。

## 5. 开启 1M Context

CLI：

```sh
zola edit deepseek --context-window 1000000
```

TUI：

```sh
zola tui
```

在 TUI 中按 `C` 切换当前 Provider 的 1M context。

该设置会写入 Codex 配置：

```toml
model_context_window = 1000000
```

这是写给 Codex 的能力提示。实际上游是否支持 1M context，仍取决于 Provider
模型本身。

## 6. 切换到 Proxy Mode

```sh
zola proxy use deepseek
```

这会修改：

```text
~/.codex/config.toml
```

让 Codex 使用：

```text
http://127.0.0.1:8317/v1
```

启动 systemd service：

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now zola-proxy.service
sudo systemctl status zola-proxy.service
```

查看日志：

```sh
sudo journalctl -u zola-proxy.service -f
```

检查代理：

```sh
curl http://127.0.0.1:8317/health
zola proxy status
```

在 iOS 越狱设备上，同时检查 launchd 后台任务和代理健康状态：

```sh
zola service status
sudo zola service restart
```

## 7. 直接运行 Codex

Service 正常运行后，直接执行：

```sh
codex
```

不需要每次运行：

```sh
zola run
```

## 8. Service 配置

编辑：

```sh
sudo nano /etc/default/zola
```

示例：

```sh
ZOLA_SERVICE_USER="admin"
ZOLA_PROXY_ARGS="--provider deepseek"
```

重新生成 service 并重启：

```sh
sudo env ZOLA_SERVICE_USER=admin dpkg-reconfigure zola
sudo systemctl restart zola-proxy.service
```

## 9. Direct Mode

如果不需要模型伪装，也不使用 Proxy，可以让 Codex 直接连接 Provider。

切回 Direct Mode：

```sh
zola proxy direct
```

如果希望直接运行 `codex`，可以持久化 key：

```sh
zola save deepseek
codex
```

`zola save` 会把 API Key 写入 Codex 的
`experimental_bearer_token` 字段。模型伪装 Provider 不支持 Direct Mode。

## 10. TUI

运行：

```sh
zola tui
```

按键：

```text
Enter  使用当前 Provider
A      添加 Provider
E      编辑 Provider
D      删除 Provider
T      测试 Provider
R      启动 Codex
P      切换 Direct/Proxy，并启动/停止本地 Proxy
C      切换 1M Context
Q      退出
```

## 11. 常见问题

### Missing environment variable

说明你直接运行了 `codex`，但 Codex 配置使用 `env_key`。

解决：

```sh
zola run
```

或者：

```sh
zola save deepseek
codex
```

模型伪装 Provider 必须使用 Proxy Mode。

### gpt5.4 metadata not found

模型名写错了。改成：

```text
gpt-5.4
```

### Proxy 没有启动

```sh
sudo systemctl status zola-proxy.service
sudo journalctl -u zola-proxy.service -n 100
```

检查端口：

```sh
ss -lntp | grep 8317
```

### Codex 提示 bubblewrap

这是 Codex 的 Linux sandbox 提示，与 Zola 无关。

```sh
sudo apt install bubblewrap
```

## 12. 升级

```sh
git pull
make deb
sudo dpkg -i dist/zola_1.0.0_amd64.deb
sudo systemctl restart zola-proxy.service
```

## 13. 卸载

```sh
sudo systemctl disable --now zola-proxy.service
sudo dpkg -r zola
```
