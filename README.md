# sshbox

SSH into a fresh Docker container. Every connection gets a new Alpine shell; disconnect and the container is gone.

It's a single static binary (one TOML file). Point it at a Docker socket and it works.

## Install and run

macOS:

```
brew install go # install homebrew if you dont already have it
brew install --cask docker
```

Linux:

```
sudo apt install golang-go # or your distro's package manager
curl -fsSL https://get.docker.com | sh
```

Windows:

```
winget install GoLang.Go
winget install Docker.DockerDesktop
```

Then from this repo:

```
go run .
```

Or build first:

```
go build
./sshbox
```

Docker needs to be running (`docker info` should succeed).

## Usage

```
ssh -p 2222 anyone@localhost
```

Any username and password gets in. You land in `/bin/sh` inside a fresh `alpine:latest` container. Exit or disconnect and the container is destroyed. `docker ps -a` won't show it.

## Config

sshbox reads `config.toml` from the current directory by default. Every field has a matching flag that overrides the file if passed.

| Field | Flag | Default | Meaning |
|-------|------|---------|---------|
| `listen_addr` | `--listen` | `:2222` | address the SSH server binds to |
| `image` | `--image` | `alpine:latest` | image to run per session |
| `shell` | `--shell` | `/bin/sh` | command run inside the container |
| `network` | `--network` | `none` | Docker network mode |
| `memory` | `--memory` | `256m` | Docker memory limit |
| `cpus` | `--cpus` | `0.5` | Docker CPU limit |
| `idle_timeout` | `--idle-timeout` | `10m` | disconnect after this long with no activity |

Point at a different file with `--config path/to/file.toml`.

## Limitations

**No authentication.** Any username or password gets in. sshbox is not meant to be your access control boundary. Put it behind Tailscale, a VPN, or an SSH bastion and let that layer decide who reaches port 2222.

**Single node only.** Containers run on whatever machine sshbox is running on. No clustering, no remote Docker hosts.

**No persistent storage.** Everything in the container disappears with the session. No volume mounting yet, so nothing carries between connections.

**PTY support** covers normal interactive shells, arrow keys, colors, resize. Full-screen apps that do unusual things with escape sequences haven't been tested exhaustively.

## Safety

Containers get no network access (`--network none`) and are capped at 256MB of memory and half a CPU by default.

Don't expose port 2222 directly to the internet. Anyone who reaches it gets a shell. It's fine on a LAN or behind a VPN; the container isolation here is for convenience, not a hardened security boundary.

## Contributing

PRs welcome. Open issues labeled `good-first-issue` are a reasonable place to start.

## License

MIT
