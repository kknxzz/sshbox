# TODO

- **Important: investigate SSH host key changes.** The default `host_key` path is relative to the working directory, so launching from a different directory can create a different key. Check persistence across restarts and working directories, releases, and existing known_hosts entries; make the default location stable and document safe key verification and migration. A changed-key warning can also mean a different service is listening, so verify the fingerprint before replacing a trusted key.

- per-username image mapping in config.toml, so different SSH users can land in different images
- SFTP/SCP support, if it turns out people actually want files in and out rather than just a shell
- some way to install packages at session start (curl, sudo, git, whatever) without building a custom image for it -- worth checking the startup-time cost against just telling people to bring their own image first
- some sort of hardened mode that you can choose at start and makes it harder for something to escape the container
- might want to slow down the demo gif a bit

