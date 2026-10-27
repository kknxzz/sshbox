Troubleshooting
===============

Docker is not running
---------------------

Run ``docker info`` on the sshbox host. If it cannot connect, start Docker Desktop or the Docker service and check the active Docker context. For Podman, use ``--runtime podman`` and check ``podman info``.

Host key changed
----------------

sshbox creates a host key on first run and saves it at ``host_key`` by default. Keep this file across restarts. If the key genuinely changed, verify the new fingerprint with the server operator before removing the old entry:

::

   ssh-keygen -R '[localhost]:2222'


Only remove a saved key after confirming the change is expected. Connect again and check the new fingerprint before accepting it.

Packages cannot be downloaded
-----------------------------

Containers have no network access by default. Choose a network mode such as ``--network bridge`` if the session needs it. Packages installed inside a session disappear when the container is removed.
