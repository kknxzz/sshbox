# Configuration

sshbox uses built-in defaults when no configuration file exists. It looks for `config.toml` in the current directory, then the operating system's standard configuration directory. Use `--config path/to/config.toml` to choose a file.

Settings follow this order, from lowest to highest priority: defaults, TOML file, environment variables, and command-line flags. Environment variables use the flag name with an `SSHBOX_` prefix, uppercase letters, and underscores instead of hyphens. For example, `--idle-timeout` corresponds to `SSHBOX_IDLE_TIMEOUT`.

## Container settings

- `--image`: container image, default `alpine:latest`.
- `--shell`: command inside the container, default `/bin/sh`.
- `--runtime`: container runtime, default `docker`; use `podman` to select Podman.
- `--network`: runtime network mode, default `none`; use `bridge` to allow normal Docker networking.
- `--memory`: memory limit, default `256m`.
- `--cpus`: CPU limit, default `0.5`.

## Server settings

- `--listen`: listening address, default `:2222`.
- `--idle-timeout`: idle limit, default `10m`.
- `--host-key`: persistent SSH host key path, default `host_key`.
- `--max-sessions`: concurrent session limit, default `0` for unlimited.
- `--log-format`: `text` or `json`, default `text`.
- `--audit-log`: optional path for connection logs.
- `--session-log`: optional directory for raw session recordings, disabled by default.

The TOML names are in [config.toml](https://github.com/kknxzz/sshbox/blob/main/config.toml). For example, the `--listen` flag sets `listen_addr`, and `--host-key` sets `host_key_path`.
