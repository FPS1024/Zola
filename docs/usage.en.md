# Zola Usage Guide

Zola is a provider manager for OpenAI Codex. It manages providers, API keys,
`~/.codex/config.toml`, optional model aliases, and a local proxy.

This guide targets Debian-based Linux systems with systemd. Source builds,
cross-compilation, and deb packaging are covered in the
[Build Guide](build.en.md).

## Architecture

Direct Mode:

```text
codex -> provider API
```

Proxy Mode:

```text
codex -> 127.0.0.1:8317 -> Zola Proxy -> provider API
```

Model aliases also require Proxy Mode:

```text
Codex sees gpt-5.4
Zola rewrites the request to deepseek-v4-pro
DeepSeek receives deepseek-v4-pro
```

## Install the Debian Package

```sh
sudo dpkg -i dist/zola_1.0.0_amd64.deb
```

To choose the service user explicitly:

```sh
sudo env ZOLA_SERVICE_USER=admin dpkg -i dist/zola_1.0.0_amd64.deb
```

Installed paths:

```text
/usr/bin/zola
/etc/default/zola
/usr/share/zola/zola-proxy.service.in
/etc/systemd/system/zola-proxy.service
/usr/share/doc/zola/README.md
/usr/share/doc/zola/LICENSE
```

Verify:

```sh
zola version
zola doctor
```

## Add a DeepSeek Provider

```sh
zola add deepseek \
  --name DeepSeek \
  --base-url https://api.deepseek.com \
  --model deepseek-v4-flash \
  --wire-api responses \
  --api-key 'YOUR_DEEPSEEK_API_KEY'
```

Change the upstream model:

```sh
zola edit deepseek --model deepseek-v4-pro
```

Inspect providers:

```sh
zola list
zola current
```

## Model Alias

Use a Codex-known model name to avoid warnings about unknown model metadata.

```sh
zola edit deepseek \
  --model deepseek-v4-pro \
  --codex-model gpt-5.4
```

Meaning:

```text
Codex model: gpt-5.4
Upstream model: deepseek-v4-pro
```

Use the exact Codex model slug. `gpt-5.4` is valid; `gpt5.4` is not.

Model aliases require Proxy Mode because Zola performs the rewrite locally.

## 1M Context

CLI:

```sh
zola edit deepseek --context-window 1000000
```

TUI:

```sh
zola tui
```

Press `C` to toggle the 1M context hint.

The resulting Codex setting is:

```toml
model_context_window = 1000000
```

This is a capability hint for Codex. The upstream model must support the
configured context size.

## Enable Proxy Mode

```sh
zola proxy use deepseek
```

This updates `~/.codex/config.toml` to use:

```text
http://127.0.0.1:8317/v1
```

Start the service:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now zola-proxy.service
sudo systemctl status zola-proxy.service
```

View logs:

```sh
sudo journalctl -u zola-proxy.service -f
```

Check the proxy:

```sh
curl http://127.0.0.1:8317/health
zola proxy status
```

On jailbroken iOS, check both the launchd service and proxy health:

```sh
zola service status
sudo zola service restart
```

## Run Codex

After the service is running:

```sh
codex
```

You do not need to keep `zola run` or a foreground `zola proxy start` process
open.

## Service Configuration

```sh
sudo nano /etc/default/zola
```

Example:

```sh
ZOLA_SERVICE_USER="admin"
ZOLA_PROXY_ARGS="--provider deepseek"
```

Regenerate and restart:

```sh
sudo env ZOLA_SERVICE_USER=admin dpkg-reconfigure zola
sudo systemctl restart zola-proxy.service
```

## Direct Mode

Switch back to direct provider access:

```sh
zola proxy direct
```

Persist the API key into Codex config, so `codex` can run directly:

```sh
zola save deepseek
codex
```

Direct Mode does not support model aliases because no local proxy is present
to rewrite the model name.

## TUI

```sh
zola tui
```

Keys:

```text
Enter  Use provider
A      Add provider
E      Edit provider
D      Delete provider
T      Test provider
R      Run Codex
P      Toggle Direct/Proxy and start/stop the local proxy
C      Toggle 1M context
Q      Quit
```

## Troubleshooting

### Missing environment variable

Codex is using an `env_key`, but the variable is not set in the shell.

Use:

```sh
zola run
```

or:

```sh
zola save deepseek
codex
```

Aliased providers must use Proxy Mode.

### gpt5.4 metadata not found

Use the correct slug:

```text
gpt-5.4
```

### Proxy is not running

```sh
sudo systemctl status zola-proxy.service
sudo journalctl -u zola-proxy.service -n 100
ss -lntp | grep 8317
```

### Codex reports missing bubblewrap

This is a Codex Linux sandbox warning, unrelated to Zola:

```sh
sudo apt install bubblewrap
```

## Upgrade

```sh
git pull
make deb
sudo dpkg -i dist/zola_1.0.0_amd64.deb
sudo systemctl restart zola-proxy.service
```

## Uninstall

```sh
sudo systemctl disable --now zola-proxy.service
sudo dpkg -r zola
```
