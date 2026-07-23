# TODO

- Podman as an alternative to docker -- the binary name is hardcoded in three places in main.go (checkDocker, the run call, the kill call)
- optional persistent scratch volume, off by default, so state can survive across connections if someone wants that
- per-username image mapping in config.toml, so different SSH users can land in different images
- SFTP/SCP support, if it turns out people actually want files in and out rather than just a shell
