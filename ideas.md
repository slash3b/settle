# Settle CLI Design

The philosophy of `settle` is: **"Read the TOML, make the machine match."**

## Implemented

- [x] `settle` (no args) - Default apply: installs packages, creates symlinks/copies
- [x] `settle list` - Shows status of all packages and dotfiles (with colors, tables)
- [x] `--dry-run` / `-n` - Preview changes without acting
- [x] `--verbose` / `-v` - Show detailed output
- [x] `--config <path>` - Specify config file location
- [x] Package removal - Packages removed from config are uninstalled (via lockfile tracking)
- [x] Version pinning - `lockfile.json` tracks versions for reproducible installs
- [x] Distro detection - Detects Debian-based distros automatically
- [x] Dotfile symlinking - Default mode for config files
- [x] Dotfile copying - `mode = "copy"` for files that can't be symlinked
- [x] Post-install hooks - Run commands after package installation
- [x] Git clone manager - `[[git]]` blocks declare repos to clone on apply, pull on update

## Not Yet Implemented

### Package Management CLI
- [x] `settle install <package>` - Install package, remind user to add to config
- [x] `settle remove <package>` - Uninstall package, remind user to remove from config
- [x] `settle update` - Upgrade all managed packages to latest versions

### Maintenance
- [ ] `settle clean` / `prune` - Remove broken symlinks, clean up orphaned files

### Dotfile Management
- [ ] `settle track <file>` - Adopt a file into managed dotfiles (move to repo, add to config, symlink back)

## Future Providers
- [ ] Cargo package manager
- [x] Go package manager - `[[go]]` blocks with `path` and `version`, installed via `go install`

## Example User Session
```bash
# First run on a new machine
settle init github.com/user/my-setup

# Day to day: I just edited my vimrc in the repo
settle dots

# I need a new tool
settle install jq

# I want to upgrade everything
settle update

# Check status
settle list
```
