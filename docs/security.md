# Security

sshbox accepts any username and password. Put it behind a trusted access layer such as Tailscale, a VPN, or an SSH bastion. Do not expose it directly to the internet.

Each session gets a new container. No host filesystem is mounted by default, networking defaults to `none`, and each container is limited to 256 MiB of memory and half a CPU. These defaults can be changed in the configuration.

Containers are not a hardened security boundary. The host runs the container runtime, so keep it updated and give shell access only to people you trust.

## Session recordings

Session recording is off by default. If you enable `--session-log`, the raw data sent and received by each session is stored on the host. Tell users before recording their sessions, and protect those files.
