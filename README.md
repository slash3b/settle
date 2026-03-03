# settle

A declarative CLI tool for managing development environments on Linux.
Define your packages and dotfiles in a single TOML configuration file, and let settle make your machine match.

## Features

- **Package management** — Install apt packages declaratively, with optional post-install hooks
- **Dotfile management** — Symlink or copy files and directories from a central source
- **Go tools** — Install Go binaries via `go install`
- **Git repos** — Clone repositories to specified locations
- **Single config** — One `config.toml` defines your entire environment

## Installation (Debian/Ubuntu)

```bash
# Download latest release, make executable, and install to ~/.local/bin
curl -fL https://github.com/slash3b/settle/releases/latest/download/settle-linux-amd64 -o /tmp/settle \
  && chmod +x /tmp/settle \
  && mv -f /tmp/settle ~/.local/bin/settle
```

Make sure `~/.local/bin` is in your `PATH`.

## Usage

```
settle [flags] [command]

Commands:
  apply    Apply configuration (default when no command given)
  update   Upgrade all managed packages and pull git repos
  list     Show status of all packages and dotfiles
  version  Show version information

Flags:
  -config string   Path to config file (default "config.toml")
  -dry-run, -n     Show what would be done without making changes
  -verbose, -v     Enable verbose output
  -version         Show version information
```

## Configuration

settle reads a `config.toml` file (pass a custom path with `-config`).

### Apt packages

```toml
[apt]
packages = [
    "git",
    "neovim",
    "ripgrep",
    "tmux",
]

# Packages with post-install hooks
[[apt.post_hook]]
name = "pipewire-audio"
post_install = "systemctl --user --now enable wireplumber.service"
```

### Dotfiles

Symlink individual files or entire directories from a source directory.

```toml
[dotfiles]
source_dir = "~/dotfiles/sources"

# Symlink a file
[[dotfiles.file]]
src = "tmux/tmux.conf"
dest = "~/.config/tmux/tmux.conf"

# Copy a file instead of symlinking
[[dotfiles.file]]
src = "redshift.conf"
dest = "~/.config/redshift.conf"
mode = "copy"

# Symlink with sudo (e.g. system config files)
[[dotfiles.file]]
src = "xorg/20-amdgpu.conf"
dest = "/etc/X11/xorg.conf.d/20-amdgpu.conf"
mode = "copy"
sudo = true

# Symlink an executable script
[[dotfiles.file]]
src = "bin/myscript"
dest = "~/.local/bin/myscript"
executable = true

# Symlink an entire directory
[[dotfiles.dir]]
src = "autorandr"
dest = "~/.config/autorandr"
```

If a file already exists at the destination it is backed up as `<dest>.backup` before the symlink is created.

### Go tools

```toml
[[go]]
path = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
version = "v2.9.0"

[[go]]
path = "mvdan.cc/gofumpt"
version = "v0.9.2"
```

### Git repositories

```toml
[[git]]
url = "https://github.com/tmux-plugins/tpm"
dest = "~/.tmux/plugins/tpm"
```
