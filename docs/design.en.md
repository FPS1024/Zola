# Zola Design and Architecture

## Goals

Zola is a cross-platform Codex Provider Manager, not only an API conversion
script. It manages:

- providers, models, API keys, and Codex configuration
- direct and local-proxy connection modes
- transparent Responses API forwarding
- Chat Completions to Responses conversion when required
- CLI and TUI workflows
- Linux, macOS, Windows, and optional jailbroken iOS arm64 builds

## Layers

```text
CLI / TUI
   |
   +-- Config Manager
   +-- Codex Config Manager
   +-- Provider Manager
   +-- Secret Manager
   +-- Proxy Server
```

Code boundaries:

```text
internal/cli        Cobra commands
internal/tui        Bubble Tea terminal UI
internal/config     provider, state, paths, and JSON storage
internal/codex      Codex config.toml and process launch
internal/secret     keychain abstraction and key resolution
internal/api        Responses API probe
internal/proxy      transparent proxy, aliases, and protocol conversion
```

## Direct Mode

```text
Codex -> provider API
```

Zola writes provider settings into `~/.codex/config.toml`.

Direct Mode does not rewrite model names. Model aliases require Proxy Mode.

## Proxy Mode

```text
Codex -> 127.0.0.1:8317 -> Zola Proxy -> provider API
```

Routes:

```text
GET  /health
GET  /v1/models
POST /v1/responses
POST /v1/chat/completions
```

The proxy:

- injects the real provider API key
- keeps that key out of Codex configuration
- writes logs without request bodies or authorization headers
- forwards Responses API traffic transparently
- converts Chat Completions providers
- rewrites Codex model aliases to upstream model names
- flushes SSE chunks promptly

## Model Aliases

A provider stores:

```text
CodexModel: model name visible to Codex
Model:      real upstream model name
```

Example:

```text
CodexModel = gpt-5.4
Model      = deepseek-v4-pro
```

Codex uses known metadata for `gpt-5.4`; the proxy rewrites the request model
to `deepseek-v4-pro`.

## Protocol Conversion

Conversion lives in `internal/proxy/converter.go`.

It handles:

- instructions and message conversion
- function calls and function-call outputs
- tools and tool choices
- Chat JSON to Responses output
- Chat SSE to Responses SSE
- text and function-argument deltas
- final `response.completed`

Responses-native providers remain transparent and do not pass through the
converter.

## Configuration and Secrets

Zola state:

```text
~/.config/zola/providers.json
~/.config/zola/state.json
```

Codex state:

```text
~/.codex/config.toml
```

API key resolution order:

```text
environment -> provider file -> OS keychain
```

The OS keychain is preferred. A 0600 provider file is used as a fallback.

`zola save` can persist a bearer token into Codex's
`experimental_bearer_token` field for Direct Mode.

## Debian and systemd

The deb contains:

```text
/usr/bin/zola
/usr/share/zola/zola-proxy.service.in
/etc/default/zola
```

The post-install script generates:

```text
/etc/systemd/system/zola-proxy.service
```

The build artifact never contains the build machine username. The service user
is resolved on the target machine.

## Security

- API keys are never written to logs
- request and response bodies are not logged
- Codex and provider files use 0600 permissions
- the proxy listens on 127.0.0.1 by default
- model aliases are rewritten only inside the local proxy

## Known Limits

- one model alias per provider
- the 1M context setting is only a Codex hint
- TOML rewrites normalize formatting and do not preserve comments
- automatic 429 retry and complex model maps are not implemented yet

