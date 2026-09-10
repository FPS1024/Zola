# Zola

Zola is a cross-platform terminal provider manager for OpenAI Codex. This
repository is the Phase 1 foundation: provider storage, Codex config
management, a small CLI, and tests.

Current release baseline: v1.0.0.

## Current Status

Implemented in the current version:

- Provider data model and `providers.json` storage
- DeepSeek, OpenAI, and OpenRouter presets
- `zola add`, `zola edit`, `zola remove`, `zola list`
- `zola use` writes Codex `~/.codex/config.toml`
- `zola test` sends a minimal Responses API probe
- Local transparent proxy with Responses API, Chat Completions, models,
  health, and SSE streaming
- OS keychain storage through Secret Service, macOS Keychain, or Windows
  Credential Manager
- `zola current`, `zola run`, `zola doctor`, `zola version`
- `zola save` persists the provider key into Codex config for direct `codex`
  launches
- Bubble Tea terminal UI as the default no-argument entry point
- API keys are not written into Codex config or printed by normal commands

Not yet implemented:

- Automatic 429 retry and advanced model mapping
- GitHub Actions release matrix

## Build

```sh
make build
./bin/zola --help
```

`make build` detects the native `GOOS`, `GOARCH`, and `GOARM` and names the
output with the full target and version:

```text
bin/zola-v1.0.0-linux-amd64
bin/zola-v1.0.0-darwin-arm64
bin/zola-v1.0.0-ios-arm64
```

Native single-platform targets fail fast when they do not match the installed
Go toolchain:

```sh
make build-ios
make build-darwin
make build-linux
make build-windows
```

Use the release script for cross-compilation:

```sh
make release
```

`make release` builds the standard desktop/server targets. iOS is intentionally
separate because Go requires CGO/external linking for `ios/arm64`:

```sh
# Native ios/arm64 Go toolchain, such as a jailbroken iPhone
make build-ios

# Explicit iOS release target from macOS with Xcode/iOS SDK
make release-ios

# All standard targets plus iOS
./scripts/build-all.sh all-with-ios
```

The release output contains detailed binary names inside both the archive and
the staging directory:

```text
dist/zola-v1.0.0-linux-amd64.tar.gz
dist/zola-v1.0.0-linux-amd64/zola-v1.0.0-linux-amd64
dist/zola-v1.0.0-windows-amd64.zip
dist/zola-v1.0.0-windows-amd64/zola-v1.0.0-windows-amd64.exe
dist/zola-v1.0.0-ios-arm64.tar.gz
dist/SHA256SUMS
```

Run the test suite:

```sh
make test
```

## Quick Start

```sh
zola add deepseek --api-key "$DEEPSEEK_API_KEY"
zola list
zola use deepseek
zola test deepseek
zola run
```

Running `zola` with no command opens the TUI. The TUI supports provider
selection, add/edit/delete forms, testing through `/responses`, Direct/Proxy
mode switching with an in-TUI proxy server, launching Codex, and quitting
with `q`.

TUI keys:

```text
Enter Use
A     Add provider
E     Edit provider
D     Delete provider
T     Test Responses API
R     Run Codex
P     Toggle Direct/Proxy mode and start/stop the local proxy
Q     Quit
```

`zola add` accepts an id and all relevant fields as flags. When required
fields are missing, it prompts for them.

```sh
zola add my-api \
  --name "My API" \
  --base-url "https://api.example.com/v1" \
  --model "my-model" \
  --wire-api responses \
  --api-key "$MY_API_KEY"
```

## Storage

Zola uses standard user configuration locations:

- Providers: `$ZOLA_CONFIG_DIR/providers.json` or
  `$XDG_CONFIG_HOME/zola/providers.json`, falling back to
  `~/.config/zola/providers.json`
- Current selection: same directory, `state.json`
- Codex config: `$CODEX_HOME/config.toml` or `~/.codex/config.toml`

`zola run` sets the provider's API key environment variable for the child
Codex process only. It does not export the key into the shell or log it.

If you prefer launching `codex` directly instead of `zola run`, persist the
selected provider first:

```sh
zola save deepseek
codex
```

`zola save` writes Codex's documented `experimental_bearer_token` field into
`~/.codex/config.toml`, so no environment variable is required. The token is
only stored in the 0600 Codex config file. Switching back to env-based direct
mode with `zola use deepseek` removes that persisted token.

`zola test` sends a small `POST {base_url}/responses` request with the
provider's default model. A successful test may consume a small number of
tokens depending on the provider.

## Local Proxy

Point Codex at the proxy, start it, then launch Codex:

```sh
zola proxy use deepseek
zola proxy start
codex
```

Return to direct provider access with:

```sh
zola proxy direct
```

The proxy listens on `127.0.0.1:8317` by default, always presents the
Responses API surface to Codex, and exposes:

- `GET /health`
- `GET /v1/models`
- `POST /v1/responses`
- `POST /v1/chat/completions`

Responses and SSE streams are forwarded transparently. The proxy injects the
resolved provider API key upstream, but it never logs request bodies,
responses, or `Authorization` headers.

For providers whose real API is `wire_api = "chat"`, the proxy also performs
protocol conversion:

- Responses API instructions/messages/tool calls are converted to Chat
  Completions request format
- Chat Completions JSON responses are converted back to Responses API output
- Chat Completions SSE streams are converted into Responses API SSE events with
  text deltas, tool-call argument deltas, and a final `response.completed`

That means an older Chat-only DeepSeek-style endpoint can be exposed to Codex
as if it spoke Responses API.

`zola run` remembers whether the current mode is direct or proxy. In direct
mode it writes the provider key into the Codex child environment. In proxy
mode it launches Codex against the local proxy instead.

## Keychain

`zola add` stores API keys in the OS keychain when available:

- Linux: Secret Service through DBus
- macOS: Keychain
- Windows: Credential Manager

If no keychain backend is available, Zola falls back to storing the key in
`providers.json` with `0600` permissions and prints a warning. To force a
keychain failure instead of fallback, set:

```sh
export ZOLA_SECRET_BACKEND=keyring
```

Existing plaintext keys in `providers.json` are still read for backward
compatibility. Editing a legacy provider without passing a new `--api-key`
attempts to migrate the existing key into the keychain.

## Architecture Notes

The public command line is thin. Domain code lives under `internal/`:

- `internal/config`: provider model, JSON store, state, and presets
- `internal/codex`: TOML-aware Codex config updates and process launch
- `internal/api`: Responses API probe client
- `internal/proxy`: local transparent proxy and SSE streaming
- `internal/secret`: OS keychain abstraction and provider key resolution
- `internal/tui`: Bubble Tea provider list and quick actions
- `internal/cli`: Cobra commands

The intended later architecture keeps provider management, Codex config
management, and an optional local proxy as separate layers.
