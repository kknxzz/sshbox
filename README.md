# sshbox

SSH into a brand new Docker container. Every connection gets a fresh `alpine` shell; when you disconnect, the container is destroyed.

## Why

[ContainerSSH](https://containerssh.io) solves this problem too, but it wants a webhook-based auth server and a Kubernetes-style deployment even for a single-user homelab box. sshbox is a single static binary with a TOML file. No auth server, no orchestrator, no config server -- point it at a Docker socket and it works.

## Install and run

On a Mac:

```
brew install go
brew install --cask docker
```

Then, from this repo:

```
go run .
```

or build a binary and run that instead:

```
go build
./sshbox
```

Docker needs to actually be running (`docker info` should succeed) -- sshbox checks this at startup and exits with a clear error if it isn't.

## Usage

```
ssh -p 2222 anyone@localhost
```

Any username and password are accepted. You land in `/bin/sh` inside a fresh `alpine:latest` container. Exit the shell or close the connection and the container is gone -- check with `docker ps -a`, it won't be there.

## Config

sshbox reads `config.toml` from the current directory by default. Every field also has a matching flag, which overrides the file if passed:

| Field | Flag | Default | Meaning |
|---|---|---|---|
| `listen_addr` | `--listen` | `:2222` | address the SSH server binds to |
| `image` | `--image` | `alpine:latest` | image to run per session |
| `shell` | `--shell` | `/bin/sh` | command run inside the container |
| `network` | `--network` | `none` | Docker `--network` mode |
| `memory` | `--memory` | `256m` | Docker `--memory` limit |
| `cpus` | `--cpus` | `0.5` | Docker `--cpus` limit |
| `idle_timeout` | `--idle-timeout` | `10m` | disconnect a session with no activity after this long |

Point at a different file with `--config path/to/file.toml`.

## Limitations

- **No authentication.** Any username or password gets in. This is deliberate -- sshbox isn't meant to be your access-control boundary. Put it behind Tailscale, a VPN, or an SSH bastion, and let that layer decide who gets to reach port 2222.
- **Single node only.** There's no scheduling, no clustering, no remote Docker hosts. It runs containers on whatever machine sshbox is running on.
- **No persistent storage.** Everything in the container disappears with the session, on purpose. There's no volume mounting yet, so you can't carry work between connections.
- **PTY support covers normal interactive shells** -- arrow keys, colors, clear screen, window resize. Full-screen terminal apps that do unusual things with escape sequences haven't been tested exhaustively.

## Safety

By default, containers get no network access (`--network none`) and are capped at 256MB of memory and half a CPU. That's the isolation you get out of the box.

What this is not safe for: exposing directly to the internet without something in front of it. There's no auth, so anyone who reaches the port gets a shell. It's fine on a LAN or behind a VPN; it is not a substitute for real access control, and container isolation here is for convenience, not a hardened boundary against a determined attacker trying to break out of Alpine.

## Contributing

PRs welcome. Check open issues labeled `good-first-issue` for a place to start.

## License

MIT
