# Zola

Zola is a cross-platform provider manager for OpenAI Codex.

It manages Codex providers, API keys, `~/.codex/config.toml`, and an optional
local proxy. The proxy can transparently forward Responses API traffic,
convert Chat Completions to Responses, stream SSE, and rewrite model aliases.

## Features

- Provider presets and custom OpenAI-compatible providers
- Direct Mode and persistent Proxy Mode
- Responses API and Chat Completions routing
- Model aliases such as `gpt-5.4 -> deepseek-v4-pro`
- 1M context-window hint
- OS keychain support with file fallback
- Bubble Tea TUI and Cobra CLI
- Debian package with a systemd proxy service
- Linux, macOS, Windows, and optional jailbroken iOS arm64 builds

## Documentation

- [English usage](docs/usage.en.md)  [Chinese usage](docs/usage.zh-CN.md)
- [English build guide](docs/build.en.md)  [Chiese build guide](docs/build.zh-CN.md)
- [English design and architecture](docs/design.en.md)  [Chinese design and architecture](docs/design.zh-CN.md)

## Quick Start

Build the current platform:

```sh
make build
```

Build a Debian package:

```sh
make deb
```

Open the TUI:

```sh
zola tui
```

## Repository Layout

```text
cmd/zola             main entry point
internal/cli         Cobra commands
internal/tui         Bubble Tea terminal UI
internal/config      providers and state
internal/codex       Codex configuration and launch
internal/secret      OS keychain abstraction
internal/api         Responses API probe
internal/proxy       proxy, SSE, and protocol conversion
packaging/           Debian and systemd packaging
scripts/             release and package build scripts
docs/                usage, build, and design documentation
```

## Development

```sh
make test
make vet
```

Current release baseline: `v1.0.1`.

## License

MIT. See [LICENSE](LICENSE).

