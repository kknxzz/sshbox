# TODO

- Podman as an alternative to docker -- the binary name is hardcoded in three places in main.go (checkDocker, the run call, the kill call)
- optional persistent scratch volume, off by default, so state can survive across connections if someone wants that
- per-username image mapping in config.toml, so different SSH users can land in different images
- SFTP/SCP support, if it turns out people actually want files in and out rather than just a shell
- some way to install packages at session start (curl, sudo, git, whatever) without building a custom image for it -- worth checking the startup-time cost against just telling people to bring their own image first
- some sort of hardened mode that you can choose at start and makes it harder for something to escape the container
