# settle

A declarative CLI tool for managing development environments on Linux. 
Define your packages and dotfiles in a single TOML configuration file, and let settle make your machine match.

## Features

- **Package management** — Install and track apt packages declaratively
- **Dotfile management** — Symlink or copy configuration files from a central source
- **Idempotent** — Run settle multiple times safely; only changes what's needed
- **Version pinning** — Lockfile tracks installed versions for reproducible setups
- **Single config** — One `config.toml` defines your entire environment

## Installation (Debian/Ubuntu)

```bash
# Download latest release, make executable, and install to ~/.local/bin
curl -fL https://github.com/slash3b/settle/releases/latest/download/settle-linux-amd64 -o /tmp/settle \
  && chmod +x /tmp/settle \
  && mv -f /tmp/settle ~/.local/bin/settle
```

Make sure `~/.local/bin` is in your `PATH`.
