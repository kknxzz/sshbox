sshbox
======

SSH into a fresh Docker container. Every connection gets a new Alpine
shell -- disconnect, and the container is gone.

Quick start
-----------

Docker needs to be running first (`docker info` should succeed).

Run:

    bin/sshbox              (macOS/Linux)
    bin\sshbox.exe          (Windows)

From another terminal:

    ssh -p 2222 anyone@localhost

Any username and password gets in. Exit or disconnect and the
container is destroyed -- `docker ps -a` won't show it.

Config
------

sshbox reads config.toml from the current directory if one exists,
otherwise it checks the OS-standard config location
(~/.config/sshbox/config.toml on Linux, the platform equivalent
elsewhere), otherwise built-in defaults apply. Every setting also has
a matching flag -- run `sshbox -h` to see them all. Every setting also
has an SSHBOX_-prefixed env var (SSHBOX_IMAGE, SSHBOX_IDLE_TIMEOUT,
etc). Precedence is defaults < config file < env var < flag.

  listen_addr    --listen         :2222
  image          --image          alpine:latest
  shell          --shell          /bin/sh
  network        --network        none
  memory         --memory         256m
  cpus           --cpus           0.5
  idle_timeout   --idle-timeout   10m
  host_key_path  --host-key       host_key
  runtime        --runtime        docker
  log_format     --log-format     text
  audit_log      --audit-log      (none, disabled)
  session_log    --session-log    (none, disabled)
  max_sessions   --max-sessions   0 (unlimited)

Good to know
------------

- No authentication -- any username and password works. That's on
  purpose, sshbox isn't your access control layer. Don't expose it
  to the internet; put it behind Tailscale, a VPN, or a bastion.
- alpine:latest is barebones -- no curl, no sudo, no bash, just
  busybox and apk. `apk add curl` works fine once you're in, it
  just won't persist between sessions.
- Point --image at something fuller (ubuntu:latest, your own image)
  if alpine is too bare for what you're doing.
- The host key is generated once and saved to host_key, so it
  won't change every time you restart sshbox.
- `sshbox --version` prints the build version.
- `sshbox --check` validates the config and runtime, then exits without listening -- useful for a systemd ExecStartPre or just sanity-checking a config file.
- --session-log records everything a session sends and receives, raw, per connection. It's opt-in for a reason -- enabling it means recording everything anyone using sshbox types and sees. Disclose that if you turn it on.

More info
---------

Full docs, source, and issue tracker:
https://github.com/kknxzz/sshbox

License: see LICENSE in this archive.
