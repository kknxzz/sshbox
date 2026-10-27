Getting started
===============

sshbox runs a fresh Linux container for each SSH session. Install Docker or Podman on the host and start its service before running sshbox.

Download and run
----------------

Download the archive for your operating system and processor from `GitHub Releases <https://github.com/kknxzz/sshbox/releases/latest>`_. Extract it and run the binary:

::

   ./bin/sshbox


On Windows, run ``bin\sshbox.exe``. Native Windows server PTY support has not been verified; WSL with the Linux build is an alternative.

In another terminal, connect:

::

   ssh -p 2222 anyone@localhost


Any username and password works. Press Enter at the password prompt. The container is removed when you disconnect.

Build from source
-----------------

Install Go 1.23 or later and clone the repository. The main branch may be ahead of the latest release.

::

   git clone https://github.com/kknxzz/sshbox
   cd sshbox
   go run .


Run ``sshbox --check`` to validate the configuration and runtime, or ``sshbox --version`` to display the version.
