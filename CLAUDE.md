# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`settle` is a CLI utility for managing development environment setup on Linux systems. It combines package management (via apt-get) and dotfile management into a single declarative configuration file (`config.toml`).

**Current Status:** Early development - basic package installation via apt-get is implemented. Dotfile symlinking, multi-provider support (cargo, go), and most CLI commands from ideas.md are not yet implemented.

## Migration from Bash Scripts

### Current State: ~/dotfiles Repository

The existing `~/dotfiles` directory contains bash scripts that need to be replaced by settle's declarative config approach:

#### Existing Scripts (to be migrated):

1. **soft.sh** (~216 lines) - Package installation script
   - Installs ~80+ packages via apt-get across multiple categories:
     - Desktop environment: i3, i3blocks, lightdm, picom, rofi, feh
     - Terminal tools: alacritty, tmux, fish, neovim
     - Network tools: nm-tray, blueman, syncthing
     - Audio: pipewire, wireplumber, pulseaudio
     - Development: git, make, gcc, gdb, nasm, docker
     - System utilities: htop, btop, ncdu, duf, ripgrep, fzf, jq
     - Media: vlc, transmission, evince, peek
     - Special setup for pipewire/wireplumber with systemctl
   - Has TODOs, commented sections, and some duplication
   - Needs to be extracted into structured config.toml sections

2. **config.sh** (~197 lines) - Dotfile deployment script
   - Copies config files from `~/dotfiles/sources/` to `~/.config/` or `~/`
   - Uses `chattr +i` to make deployed configs immutable (prevents accidental edits)
   - Uses `chattr -i` before copying (to allow updates)
   - Manages configurations for:
     - alacritty (terminal emulator + theme)
     - tmux (terminal multiplexer)
     - nvim & vim (editor configs)
     - i3 (window manager + status bar)
     - picom (compositor)
     - fish (shell)
     - autorandr (display profiles: docked/mobile)
     - SSH config
     - git config (personal + work conditional includes)
     - redshift (screen color temperature)
   - Creates necessary directories with proper ownership
   - Must be run as root

3. **config.toml** - The future declarative config (currently minimal)
   - Currently only has 3 packages: ripgrep, tmux, neofetch
   - This is the template for how all configs should be structured

4. **sources/** - Directory containing actual dotfiles
   - Contains the source-of-truth config files that config.sh copies out
   - Structure mirrors ~/.config/ layout

### Migration Goals

The end goal is to consolidate soft.sh and config.sh into a single `config.toml` that settle will understand and apply:

1. **Package Management**: Extract all apt-get packages from soft.sh into structured sections
   - Group by purpose (desktop, dev-tools, network, media, etc.)
   - Support special post-install steps (systemctl commands, etc.)
   - Eventually support cargo, go, and other package managers

2. **Dotfile Management**: Replace config.sh's copy+chattr approach with:
   - Declarative mappings: source file → destination path
   - Symlinking strategy instead of copying
   - No need for chattr immutability (symlinks enforce single source)
   - Proper ownership and permission handling

3. **Single Source of Truth**: `~/dotfiles/config.toml` becomes the complete machine state definition
   - What packages should be installed
   - What configs should be deployed
   - How the system should be configured

4. **Idempotent Execution**: Running `settle` multiple times should be safe
   - Only install missing packages
   - Only update changed configs
   - Skip already-correct state

### Migration Strategy

When working on migration tasks:
- Start with simple packages from soft.sh (don't tackle special cases like pipewire first)
- Design TOML schema to accommodate the complexity in soft.sh
- Consider how to handle post-install commands (systemctl, etc.)
- Think about how to represent the config.sh file mappings declaratively
- Preserve the directory creation and ownership logic from config.sh

**Reference Files:**
- Current bash scripts: `~/dotfiles/soft.sh` and `~/dotfiles/config.sh`
- Target config: `~/dotfiles/config.toml` (currently minimal)
- Dotfile sources: `~/dotfiles/sources/` directory structure

## Architecture

### Configuration-Driven Design
The core philosophy is: **"Read the TOML, make the machine match."**

- **Config file:** `config.toml` (TOML format) - declares desired system state
- **Current structure:**
  ```toml
  [packages]
  list = ["package1", "package2", ...]
  ```
- **Planned structure:** Will expand to support multiple package managers (apt, cargo, go) and dotfile paths

### Current Implementation (main.go)

The application is a single-file Go program with two main components:

1. **Config parsing:** Uses `github.com/pelletier/go-toml/v2` to unmarshal TOML into `Config` struct
2. **Package installation:** `installPackages()` function wraps `apt-get install` with sudo

**Platform:** Linux-only (enforced via runtime.GOOS check)

### Planned Architecture (from ideas.md)

The tool will grow to support:
- Multiple package managers (apt, cargo, go, etc.) - each as a "provider"
- Dotfile management via symlinking from a tracked directory
- State tracking to detect removed packages/configs
- Idempotent "apply" operation as the default command
- Subcommands for targeted operations (dots-only, packages-only)

## Development Commands

### Build & Run
```bash
# Build the binary
go build -o settle main.go

# Run directly
go run main.go

# Run with a specific config
go run main.go  # Expects config.toml in current directory
```

### Testing
No test suite exists yet. When adding tests:
```bash
# Run all tests
go test ./...

# Run specific test
go test -run TestFunctionName ./...
```

### Dependencies
```bash
# Download/update dependencies
go mod tidy

# Add a new dependency
go get <package>
```

## Key Design Constraints

1. **Linux-only:** The application explicitly checks `runtime.GOOS` and exits on non-Linux systems
2. **Requires sudo:** Package installation commands are wrapped with sudo and expect passwordless sudo or user interaction
3. **Config location:** Currently hardcoded to `config.toml` in the working directory
4. **Non-interactive apt:** Uses `DEBIAN_FRONTEND=noninteractive` to prevent prompts during package installation

## Design Philosophy & Priorities

### What Makes Settle Different

Settle combines **package management** and **dotfile management** in a single declarative config. Most tools focus on one or the other (chezmoi/dotbot for dotfiles, Ansible/NixOS for system config). The value proposition is simplicity - one TOML file, one command, consistent environment.

### Core Strengths

1. **Declarative Philosophy**: "Read the TOML, make the machine match" - the config file is the source of truth, not a script that runs commands
2. **Idempotent by Design**: Running `settle` multiple times should be safe and fast (skip what's already correct)
3. **Version Control Native**: `~/dotfiles/config.toml` lives in git, making environment changes reviewable and portable
4. **Ergonomic UX**: `settle install <pkg>` modifies the TOML (not just your system), encouraging declarative thinking

### Implementation Priorities

When adding features, prioritize in this order:

1. **Dotfile symlinking** - Self-contained, immediately valuable, simpler than package management
2. **Robust apt package management** - Get the basics solid before adding cargo/go providers
3. **Idempotent apply logic** - The killer feature; running settle should be fast when nothing changed
4. **Simple post-install hooks** - Start with basic shell command execution, expand later
5. **Multi-provider support** - Only after apt is solid

### Key Architectural Challenges

**State Tracking**: Do you auto-remove packages removed from config.toml? This is philosophically appealing but risky (what about dependencies? system packages?). Consider making removal explicit or opt-in.

**Root vs User Context**: Some operations need root (apt install, chattr), others need user context (systemctl --user). Design privilege separation carefully - perhaps separate "system" and "user" phases.

**Multi-Provider Complexity**: apt, cargo, go, npm all have different:
- Install/update/remove semantics
- Ways to check if packages exist
- Version pinning strategies
- Post-install requirements

Start simple (apt only), design the abstraction carefully before adding more.

**Post-Install Hooks**: Some packages need special setup (pipewire → systemctl, NetworkManager → config edits). Design considerations:
- Should hooks run every time or only on first install?
- How to handle failures in hooks vs failures in package install?
- User-level vs system-level command execution?

### Scope Management

The temptation will be to add more providers, templating, secrets management, etc. Resist until the core is solid. NixOS exists because declarative system management is genuinely hard. Keep settle focused on "developer environment setup" not "full system management."

## Implementation Notes

### Current Dotfile Pattern (config.sh approach)
The existing config.sh uses a copy + immutability pattern:
- Files are copied from `~/dotfiles/sources/` to their destinations
- `chattr +i` makes files immutable (prevents accidental edits)
- `chattr -i` before copying allows updates
- Forces users to edit in the dotfiles repo only

### Planned Dotfile Pattern (settle approach)
The new approach should use symlinks instead:
- Symlink from destination to `~/dotfiles/sources/` files
- No need for immutability - symlinks naturally enforce single source
- Simpler and more transparent (users can see where files come from)
- Easier to manage - editing the "deployed" file edits the source directly

### Special Considerations
Some packages in soft.sh require post-install steps:
- **pipewire/wireplumber**: Requires `systemctl --user --now enable wireplumber.service`
- **NetworkManager + openresolv**: Manual /etc/NetworkManager/NetworkManager.conf edit for tailscale compatibility
- Future TOML schema should support hooks or post-install commands

## Future CLI Design (Reference ideas.md)

When implementing new commands, follow this planned interface:

- `settle` - Idempotent apply (default behavior)
- `settle install <package>` - Add to TOML and install
- `settle update` - Upgrade all managed packages
- `settle track <file>` - Adopt a file into managed dotfiles

See ideas.md for complete command design and examples.
