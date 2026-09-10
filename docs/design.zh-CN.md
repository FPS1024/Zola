# Zola 方案与架构

## 目标

Zola 的定位不是简单的 API 转换脚本，而是一个跨平台的 Codex Provider
Manager：

- 管理 Provider、模型、API Key 和 Codex 配置
- 支持 Codex 直接连接 Provider
- 支持本地透明 Proxy
- 在需要时转换 Responses API 与 Chat Completions
- 通过 TUI 和 CLI 提供可操作的产品入口
- 支持 Linux、macOS、Windows 和可选的 iOS arm64 构建

## 分层

```text
CLI / TUI
   |
   +-- Config Manager
   +-- Codex Config Manager
   +-- Provider Manager
   +-- Secret Manager
   +-- Proxy Server
```

代码边界：

```text
internal/cli        Cobra 命令
internal/tui        Bubble Tea TUI
internal/config     Provider、状态、路径和 JSON 存储
internal/codex      读取和写入 Codex config.toml、启动 Codex
internal/secret     Keychain 抽象与 Provider Key 解析
internal/api        Responses API 探测
internal/proxy      透明代理、模型映射和协议转换
```

## Direct Mode

```text
Codex -> Provider API
```

Zola 写入：

```toml
model = "gpt-5.4"
model_provider = "deepseek"

[model_providers.deepseek]
base_url = "https://api.deepseek.com"
env_key = "DEEPSEEK_API_KEY"
wire_api = "responses"
```

Direct Mode 不负责改写模型名。模型伪装必须使用 Proxy Mode。

## Proxy Mode

```text
Codex -> 127.0.0.1:8317 -> Zola Proxy -> Provider API
```

Proxy 路由：

```text
GET  /health
GET  /v1/models
POST /v1/responses
POST /v1/chat/completions
```

Proxy 会：

- 注入真实 Provider API Key
- 隐藏 Codex 侧 key
- 记录不含请求体和认证信息的访问日志
- 透明转发 Responses API
- 对 Chat Completions Provider 做协议转换
- 按需把 Codex 模型名改写为上游模型名
- 对 SSE 响应及时 flush

## 模型伪装

Provider 同时保存：

```text
CodexModel:  Codex 看到的模型名
Model:       上游真实模型名
```

例如：

```text
CodexModel = gpt-5.4
Model      = deepseek-v4-pro
```

Codex 使用已知模型元数据，Proxy 收到 `gpt-5.4` 后改写成
`deepseek-v4-pro`。

## 协议转换

Responses API 与 Chat Completions 的转换位于
`internal/proxy/converter.go`。

支持：

- instructions 和 message 转换
- function call 与 function call output 转换
- tools 与 tool_choice 转换
- Chat JSON 到 Responses output
- Chat SSE 到 Responses SSE
- 文本 delta 和工具参数 delta
- `response.completed`

涉及协议转换时会解析 JSON 和 SSE；纯 Responses Provider 则保持透明转发。

## 配置与密钥

Zola 配置：

```text
~/.config/zola/providers.json
~/.config/zola/state.json
```

Codex 配置：

```text
~/.codex/config.toml
```

API Key 优先使用：

```text
环境变量 -> Provider 文件 -> OS Keychain
```

Keychain 不可用时，可以回退到 0600 权限的 `providers.json`。

`zola save` 可以把 Bearer Token 持久化到 Codex 配置的
`experimental_bearer_token`，用于无需环境变量的 Direct Mode。

## Debian 与 systemd

deb 包包含：

```text
/usr/bin/zola
/usr/share/zola/zola-proxy.service.in
/etc/default/zola
```

安装时由 `postinst` 根据 `SUDO_USER`、`PKEXEC_UID` 或显式环境变量生成：

```text
/etc/systemd/system/zola-proxy.service
```

构建产物中不写入构建机器用户名。

service 以配置用户身份运行，并读取该用户的：

```text
~/.config/zola
~/.codex
```

## 安全边界

- 日志不包含 API Key
- 日志不包含请求体和响应体
- Codex 配置权限为 0600
- Provider 文件权限为 0600
- Proxy 默认监听 127.0.0.1
- 模型伪装依赖本地 Proxy，不把 GPT 模型名直接发给不兼容的上游

## 已知限制

- Provider 当前是一个 Provider 对应一个模型别名
- 1M context 是 Codex 能力提示，不代表上游一定支持
- 重写 TOML 时会规范化格式，人工注释不保证保留
- 暂未实现自动 429 重试和复杂模型映射

