# sshbox

SSH into a fresh Docker container. Every connection gets a new Alpine shell; disconnect and the container is gone.

<p align="center">

  <img src="demo.gif" alt="sshbox demo" width="900">

</p>
^ No persistance in-between sessions.


It's a single static binary (one TOML file). Point it at a Docker socket and it works.

## Why

Give it to a friend, a co-worker, or just test something out in a disposable environment -- whoever's on the other end gets a real Alpine box to mess around in, and when they're done it just goes away, nothing left running that you forgot about. It's also handy for workshops, demos, or letting someone quickly reproduce an issue.

The use-cases expand even more if you have some sort of home-server. Point people at it and they get instant, disposable Linux access whenever they need it, without you handing out a real account or keeping track of which container to kill afterward. A Dockerfile with sshd baked in gets you most of the way there, but you're still the one remembering to tear the container down every time. sshbox does that part automatically, per connection.

## Security model

sshbox accepts any username and password, no exceptions and no key checking. Authentication is deliberately left to whatever sits in front of it -- run this behind Tailscale, a VPN, or a bastion host and let that layer decide who's even allowed to reach port 2222.

Every connection spins up its own container with nothing from the host mounted into it, so a session only ever touches its own throwaway filesystem. It's killed and removed the moment you disconnect, so nothing carries over between sessions. Containers also get no network access by default and are capped at 256MB of memory and half a CPU, so a session that misbehaves is limited in what it can actually do.

This keeps one session from touching your files or your other containers, but it won't hold up against someone actively trying to break out of the container -- that's a harder problem than what sshbox is solving here. Don't expose port 2222 straight to the internet and assume the container boundary alone will protect you.

## Getting started

**1. Install Go and Docker.**

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

**2. Clone this repo and make sure Docker is actually running.**

```
git clone https://github.com/kknxzz/sshbox
cd sshbox
```

`docker info` should succeed. If it doesn't, start Docker Desktop (or the docker daemon) first -- sshbox checks this at startup and refuses to run with no daemon.

**3. Start sshbox.**

```
go run .
```

or build a binary and run that instead:

```
go build
./sshbox
```

You should see a log line like:

```
time=... level=INFO msg=listening addr=:2222 runtime=docker image=alpine:latest network=none memory=256m cpus=0.5 idle_timeout=10m0s
```

That means it's listening on `:2222` and ready for connections.

**4. Connect.**

```
ssh -p 2222 anyone@localhost
```

Any username and password gets in. You land in `/bin/sh` inside a fresh `alpine:latest` container. Exit or disconnect and the container is destroyed -- `docker ps -a` won't show it.

Under the hood: sshbox accepts the connection, runs `docker run --rm -it <image> <shell>` and wires your terminal to it, then kills and removes the container as soon as you disconnect.

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
| `host_key_path` | `--host-key` | `host_key` | where the ssh host key is stored, generated on first run |
| `runtime` | `--runtime` | `docker` | container runtime binary -- `docker` or `podman` |

Point at a different file with `--config path/to/file.toml`.

## Limitations

- **No authentication** (see Security model above).
- **Single node only.** Containers run on whatever machine sshbox is running on. No clustering, no remote Docker hosts.
- **No persistent storage.** Everything in the container disappears with the session. No volume mounting yet, so nothing carries between connections.
- **No SFTP, SCP, or port forwarding.** It's an interactive shell, not a general SSH server.
- **No Docker Compose.** One image, one container, per session.
- **Linux containers only.**
- **`alpine:latest` is barebones of barebones.** No curl, no sudo, no bash, nothing beyond a busybox userland and `apk`. Point `image` at something fuller (`ubuntu:latest`, a custom image with what you need already installed) if that's not enough. There's no way yet to install packages at session start -- see [TODO.md](TODO.md).
- **PTY support** covers normal interactive shells, arrow keys, colors, resize. Full-screen apps that do unusual things with escape sequences haven't been tested exhaustively.

## Contributing

Consider Starring this project if you liked it or found it useful. It helps out a ton.

PRs welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for style notes. Open issues labeled `good-first-issue` are a reasonable place to start.

## License

MIT
