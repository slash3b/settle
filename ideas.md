Notes:
- manage state if package is gone from the list we need to remove it?


Here is a speculative design for the **`settle`** CLI.

Since the tool combines **package management** (stateful system changes) and **dotfiles** (symlinking/templating), the arguments should clearly separate these concerns while offering a "do it all" default.

### The Core Workflow
The philosophy of `settle` is: **"Read the TOML, make the machine match."**

#### 1. The Main Command (Idempotent Apply)
This is the command you run 95% of the time. It reads `settle.toml` and enforces the state (installs missing packages, updates symlinks).

*   **`settle`** (no args)
    *   *Behavior:* Runs the default "settle" process: installs packages defined in config and symlinks dotfiles.
*   **`settle apply`**
    *   *Behavior:* Explicit alias for the default behavior.
    *   *Flags:*
        *   `--dry-run` / `-n`: Show what would be installed or linked without doing it.
        *   `--verbose` / `-v`: Show detailed output (e.g., `apt-get` logs).

#### 2. Selective Settle (Targeting specific tasks)
Sometimes you only want to fix your vim config without waiting for `apt-get` to check for updates.

*   **`--only <scope>`**
    *   `settle apply --only dotfiles` (or `settle dots`): Skips package checks; only refreshes symlinks/templates.
    *   `settle apply --only packages` (or `settle soft`): Skips dotfiles; only checks software.

#### 3. Package Management (Modifying the manifest)
Instead of editing the TOML file manually, use the CLI to add software. This makes it feel like a native package manager.

*   **`settle install <package>`** (or `settle add`)
    *   *Behavior:* Adds the package to `settle.toml` under the correct provider and installs it immediately.
    *   *Examples:*
        *   `settle install neovim` (Defaults to system pkg manager, e.g., `apt`).
        *   `settle install --go gopls` (Adds to `[go]` section).
        *   `settle install --cargo ripgrep` (Adds to `[cargo]` section).
*   **`settle remove <package>`**
    *   *Behavior:* Removes from TOML and uninstalls from the system.

#### 4. Updates & Maintenance
*   **`settle update`**
    *   *Behavior:* Runs the update command for all managed package managers (e.g., `apt-get update && apt-get upgrade`, `go install ...@latest`).
    *   *Difference from `apply`:* `apply` ensures *presence* (is it installed?). `update` ensures *freshness* (is it the latest version?).
*   **`settle clean`** (or `prune`)
    *   *Behavior:* Removes broken symlinks or packages that were removed from the TOML file (if you support state tracking).

#### 5. Dotfile Specifics
*   **`settle track <file>`**
    *   *Behavior:* Takes a file currently in your home directory (e.g., `~/.bashrc`), moves it to your dotfiles repo, adds it to `settle.toml`, and symlinks it back.
    *   *Example:* `settle track .zshrc`

### Summary Table

| Command | Argument | Description |
| :--- | :--- | :--- |
| **`settle`** | *(none)* | **Default.** Applies full state (Soft + Dots). |
| | `--dry-run` | Preview changes without acting. |
| | `--only dots` | Skip package installs, only sync config files. |
| | `--only soft` | Skip config files, only install software. |
| **`install`** | `<name>` | Adds tool to TOML and installs it. |
| | `--apt`, `--go` | Specify which provider to use (optional). |
| **`update`** | *(none)* | Upgrades all installed packages to latest versions. |
| **`track`** | `<file>` | "Adopts" a local file into your managed dotfiles. |
| **`init`** | `[url]` | Clones a repo and runs `settle` immediately (bootstrap). |

### Example User Session
```bash
# First run on a new machine
settle init github.com/user/my-setup

# Day to day: I just edited my vimrc in the repo
settle dots

# I need a new tool
settle install jq

# I want to upgrade everything (apt, go binaries, cargo tools)
settle update
```
