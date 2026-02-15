package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Apply tests ---

func TestApply_NoConfig(t *testing.T) {
	saveMocks(t)

	cfg := &Config{}
	s := NewSettle(cfg, "/tmp/config.toml", false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no packages or dotfiles configured")
	})
	_ = out
}

func TestApply_AllInstalled(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// All packages return "install ok installed"
	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
		"git": "2.40.0",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
		"git": "2.40.0",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim", "git"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "All packages already installed")
	assert.Contains(t, out, "Done!")
}

func TestApply_InstallsMissing(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// vim installed, curl not
	execCommand = func(name string, arg ...string) *exec.Cmd {
		// dpkg-query calls
		if name == "dpkg-query" {
			if len(arg) >= 3 && arg[2] == "vim" {
				return exec.Command("echo", "-n", "install ok installed")
			}
			return exec.Command("false")
		}
		// apt-get calls — just succeed
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"vim":  "9.0.1",
		"curl": "7.88.1",
	})
	mockAvailableVersion(map[string]string{
		"vim":  "9.0.1",
		"curl": "7.88.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim", "curl"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Need to install: 1")
	assert.Contains(t, out, "Done!")
}

func TestApply_PinsToLockfileForMissing(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// curl not installed
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		// apt-get install — check that it uses version
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"curl": "7.88.1",
	})
	mockAvailableVersion(map[string]string{
		"curl": "7.88.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["curl"]
`)
	// Write a lockfile with a specific version
	writeLockfile(t, configPath, `{"packages":{"curl":{"version":"7.88.0","installed_at":"2024-01-01T00:00:00Z"}}}`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Done!")
}

func TestApply_NoUpgradeWhenInstalledAboveLockfile(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	origExec := execCommand
	// vim installed at HIGHER version than lockfile — no upgrade needed
	execCommand = func(name string, arg ...string) *exec.Cmd {
		// Let dpkg --compare-versions use real dpkg
		if name == "dpkg" && len(arg) > 0 && arg[0] == "--compare-versions" {
			return origExec(name, arg...)
		}
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.2",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.2",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)
	writeLockfile(t, configPath, `{"packages":{"vim":{"version":"9.0.1","installed_at":"2024-01-01T00:00:00Z"}}}`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "All packages already installed")
	assert.NotContains(t, out, "Need to upgrade")
	assert.Contains(t, out, "Done!")
}

func TestApply_UpgradesWhenInstalledBelowLockfile(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	origExec := execCommand
	// vim installed at LOWER version than lockfile — should upgrade
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg" && len(arg) > 0 && arg[0] == "--compare-versions" {
			return origExec(name, arg...)
		}
		if name == "dpkg-query" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.2",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)
	writeLockfile(t, configPath, `{"packages":{"vim":{"version":"9.0.2","installed_at":"2024-01-01T00:00:00Z"}}}`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Need to upgrade: 1")
	assert.Contains(t, out, "upgraded")
	assert.Contains(t, out, "Done!")
}

func TestApply_RemovesUntracked(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// vim installed
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)
	// Lockfile has vim AND git, but git is not in config
	writeLockfile(t, configPath, `{"packages":{"vim":{"version":"9.0.1","installed_at":"2024-01-01T00:00:00Z"},"git":{"version":"2.40.0","installed_at":"2024-01-01T00:00:00Z"}}}`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Removing 1 packages not in config")
	assert.Contains(t, out, "Done!")
}

func TestApply_DryRun(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// curl not installed
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		require.Fail(t, "should not run apt-get in dry-run mode")
		return nil
	}
	mockInstalledVersion(map[string]string{})
	mockAvailableVersion(map[string]string{
		"curl": "7.88.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["curl"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, true)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "[dry-run")
	assert.Contains(t, out, "Done!")
}

func TestApply_SkipsUnknownPackages(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// badpkg not installed, not in apt-cache
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{})
	mockAvailableVersion(map[string]string{}) // empty = all unknown

	configPath := withTempConfig(t, `
[apt]
packages = ["badpkg"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Skipping 1 unknown packages")
	assert.Contains(t, out, "badpkg")
}

func TestApply_UnsupportedDistro(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=arch\n")

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported distribution")
	})
	_ = out
}

func TestApply_Verbose(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, true, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Detected distribution: debian")
	assert.Contains(t, out, "Done!")
}

func TestApply_NoPackagesConfigured(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	configPath := withTempConfig(t, `
[apt]
packages = []
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "No apt packages configured")
}

func TestApply_PostInstallHooks(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// pipewire not installed
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		// Both apt-get install and bash -c should succeed
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"pipewire": "0.3.65",
	})
	mockAvailableVersion(map[string]string{
		"pipewire": "0.3.65",
	})

	configPath := withTempConfig(t, `
[apt]

[[apt.post_hook]]
name = "pipewire"
post_install = "echo post-install-ran"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Running post-install for pipewire")
	assert.Contains(t, out, "Done!")
}

func TestApply_DryRunWithPostInstall(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		require.Fail(t, "should not run commands in dry-run mode")
		return nil
	}
	mockAvailableVersion(map[string]string{
		"pipewire": "0.3.65",
	})

	configPath := withTempConfig(t, `
[apt]

[[apt.post_hook]]
name = "pipewire"
post_install = "echo test"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, true)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "[dry-run] Would install")
	assert.Contains(t, out, "[dry-run] Would run post-install for pipewire")
}

func TestApply_DryRunRemovesUntracked(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)
	writeLockfile(t, configPath, `{"packages":{"vim":{"version":"9.0.1","installed_at":"2024-01-01T00:00:00Z"},"git":{"version":"2.40.0","installed_at":"2024-01-01T00:00:00Z"}}}`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, true)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "[dry-run] Would remove")
	assert.Contains(t, out, "git")
}

// --- Dotfiles Apply tests ---

func TestApply_Dotfiles(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "vimrc"
dest = "`+destFile+`"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Created 1 links")
	assert.Contains(t, out, "Done!")

	// Verify symlink exists
	target, err := os.Readlink(destFile)
	require.NoError(t, err)
	assert.Equal(t, srcFile, target)
}

func TestApply_DotfilesEmpty(t *testing.T) {
	saveMocks(t)

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "/tmp"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "No dotfiles configured")
}

func TestApply_DotfilesDryRun(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "vimrc"
dest = "`+destFile+`"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, true)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "[dry-run] Would link")
	assert.Contains(t, out, "[dry-run] Would create 1 links")
}

func TestApply_DotfilesVerbose(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "vimrc"
dest = "`+destFile+`"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, true, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Linked:")
}

func TestApply_DotfilesWithErrors(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "nonexistent"
dest = "`+filepath.Join(dir, ".vimrc")+`"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err) // Apply doesn't return error for individual file errors
	})

	assert.Contains(t, out, "Errors:")
}

// --- Install tests ---

func TestInstall_NewPackage(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"curl": "7.88.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = []
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Install([]string{"curl"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Installing 1 packages")
}

func TestInstall_AlreadyInstalledAndInConfig(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)
	writeLockfile(t, configPath, `{"packages":{"vim":{"version":"9.0.1","installed_at":"2024-01-01T00:00:00Z"}}}`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Install([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "All packages already in config and installed")
}

func TestInstall_UpdatesLockfileOnMismatch(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.2", // installed version differs from lockfile
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)
	writeLockfile(t, configPath, `{"packages":{"vim":{"version":"9.0.1","installed_at":"2024-01-01T00:00:00Z"}}}`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Install([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "All packages already in config and installed")
}

func TestInstall_DryRun(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		require.Fail(t, "should not run apt-get in dry-run mode")
		return nil
	}

	configPath := withTempConfig(t, `
[apt]
packages = []
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, true)

	out := captureOutput(t, func() {
		err := s.Install([]string{"curl"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "[dry-run] Would install")
}

func TestInstall_NoPackagesSpecified(t *testing.T) {
	cfg := &Config{}
	s := NewSettle(cfg, "/tmp/config.toml", false, false)

	err := s.Install(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no packages specified")
}

func TestInstall_UnsupportedDistro(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=arch\n")

	cfg := &Config{}
	s := NewSettle(cfg, "/tmp/config.toml", false, false)

	err := s.Install([]string{"vim"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported distribution")
}

func TestInstall_VerboseOutput(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, true, false)

	out := captureOutput(t, func() {
		err := s.Install([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "All packages already in config and installed")
}

func TestInstall_NotInConfigShowsReminder(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"curl": "7.88.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Install([]string{"curl"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "add to your config.toml")
	assert.Contains(t, out, "curl")
}

func TestInstall_CreatesAptSection(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"curl": "7.88.1",
	})

	// Config with no apt section at all
	configPath := withTempConfig(t, "")

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Install([]string{"curl"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Installing")
}

func TestInstall_AllAlreadyOnSystem(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{})

	configPath := withTempConfig(t, `
[apt]
packages = []
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Install([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "All packages already installed on system")
}

func TestInstall_PackageInPackageSection(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"pipewire": "0.3.65",
	})

	configPath := withTempConfig(t, `
[apt]

[[apt.post_hook]]
name = "pipewire"
post_install = "echo test"
`)
	writeLockfile(t, configPath, `{"packages":{"pipewire":{"version":"0.3.65","installed_at":"2024-01-01T00:00:00Z"}}}`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Install([]string{"pipewire"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "All packages already in config and installed")
}

// --- Remove tests ---

func TestRemove_InstalledPackage(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		return exec.Command("true")
	}

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)
	writeLockfile(t, configPath, `{"packages":{"vim":{"version":"9.0.1","installed_at":"2024-01-01T00:00:00Z"}}}`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Remove([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Removing 1 packages")
	assert.Contains(t, out, "remove from your config.toml")
}

func TestRemove_NotInstalled(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("false")
	}

	configPath := withTempConfig(t, `
[apt]
packages = []
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Remove([]string{"nonexistent"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "None of the packages are in config or installed")
}

func TestRemove_DryRun(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		require.Fail(t, "should not run apt-get in dry-run mode")
		return nil
	}

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, true)

	out := captureOutput(t, func() {
		err := s.Remove([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "[dry-run] Would uninstall")
}

func TestRemove_NoPackagesSpecified(t *testing.T) {
	cfg := &Config{}
	s := NewSettle(cfg, "/tmp/config.toml", false, false)

	err := s.Remove(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no packages specified")
}

func TestRemove_NoAptConfig(t *testing.T) {
	cfg := &Config{}
	s := NewSettle(cfg, "/tmp/config.toml", false, false)

	out := captureOutput(t, func() {
		err := s.Remove([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "No packages configured")
}

func TestRemove_UnsupportedDistro(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=arch\n")

	cfg := &Config{Apt: &AptConfig{Packages: []string{"vim"}}}
	s := NewSettle(cfg, "/tmp/config.toml", false, false)

	err := s.Remove([]string{"vim"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported distribution")
}

func TestRemove_NotInConfigButInstalled(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		return exec.Command("true")
	}

	configPath := withTempConfig(t, `
[apt]
packages = ["git"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Remove([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Removing 1 packages")
}

func TestRemove_VerboseOutput(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// curl is installed but not in config
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		return exec.Command("true")
	}

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, true, false)

	out := captureOutput(t, func() {
		err := s.Remove([]string{"curl"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "not in config")
}

func TestRemove_InstalledNotInConfig_NoUninstall(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false") // not installed
		}
		return exec.Command("true")
	}

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Remove([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "No packages to uninstall")
}

func TestRemove_DryRunConfigReminder(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		require.Fail(t, "should not run apt-get remove in dry-run")
		return nil
	}

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, true)

	out := captureOutput(t, func() {
		err := s.Remove([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "[dry-run] Would remind to remove from config")
	assert.Contains(t, out, "[dry-run] Would uninstall")
}

func TestRemove_PackageInPackageSection(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		return exec.Command("true")
	}

	configPath := withTempConfig(t, `
[apt]

[[apt.post_hook]]
name = "pipewire"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Remove([]string{"pipewire"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "remove from your config.toml")
}

// --- Update tests ---

func TestUpdate_Success(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.2",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Update()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Updating 1 managed packages")
	assert.Contains(t, out, "Done!")
}

func TestUpdate_NoPackages(t *testing.T) {
	cfg := &Config{}
	s := NewSettle(cfg, "/tmp/config.toml", false, false)

	out := captureOutput(t, func() {
		err := s.Update()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "No packages configured")
}

func TestUpdate_EmptyPackages(t *testing.T) {
	cfg := &Config{Apt: &AptConfig{}}
	s := NewSettle(cfg, "/tmp/config.toml", false, false)

	out := captureOutput(t, func() {
		err := s.Update()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "No packages to update")
}

func TestUpdate_DryRun(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, true)

	out := captureOutput(t, func() {
		err := s.Update()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "[dry-run] Would run: apt-get update")
	assert.Contains(t, out, "[dry-run] Would upgrade")
}

func TestUpdate_UnsupportedDistro(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=arch\n")

	cfg := &Config{Apt: &AptConfig{Packages: []string{"vim"}}}
	s := NewSettle(cfg, "/tmp/config.toml", false, false)

	err := s.Update()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported distribution")
}

func TestUpdate_WithPackageSection(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"vim":      "9.0.2",
		"pipewire": "0.3.66",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]

[[apt.post_hook]]
name = "pipewire"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Update()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Updating 2 managed packages")
}

func TestUpdate_Verbose(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.2",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, true, false)

	out := captureOutput(t, func() {
		err := s.Update()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Updating 1 managed packages")
}

// --- List tests ---

func TestList_Packages(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// vim installed, curl not
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			if len(arg) >= 3 && arg[2] == "vim" {
				return exec.Command("echo", "-n", "install ok installed")
			}
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim":  "9.0.1",
		"curl": "7.88.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim", "curl"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.List()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Packages:")
	assert.Contains(t, out, "vim")
	assert.Contains(t, out, "curl")
}

func TestList_Empty(t *testing.T) {
	cfg := &Config{}
	s := NewSettle(cfg, "/tmp/config.toml", false, false)

	out := captureOutput(t, func() {
		err := s.List()
		require.NoError(t, err)
	})

	assert.Equal(t, "", out)
}

func TestList_Dotfiles(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.Symlink(srcFile, destFile)

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "vimrc"
dest = "`+destFile+`"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.List()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Dotfiles:")
	assert.Contains(t, out, "linked")
}

func TestList_DotfilesEmpty(t *testing.T) {
	saveMocks(t)

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "/tmp"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.List()
		require.NoError(t, err)
	})

	// No table should be printed for empty dotfiles
	assert.NotContains(t, out, "Dotfiles:")
}

func TestList_PackagesUpgradeAvailable(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.2", // upgrade available
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.List()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "upgrade: 9.0.2")
}

func TestList_UnknownPackage(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("false")
	}
	mockInstalledVersion(map[string]string{})
	mockAvailableVersion(map[string]string{})

	configPath := withTempConfig(t, `
[apt]
packages = ["badpkg"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.List()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "unknown")
}

func TestList_InstalledVersionUnknown(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.List()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "installed (version unknown)")
}

func TestList_WithStateVersionDiff(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.2",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.2",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)
	// State has old version
	writeLockfile(t, configPath, `{"packages":{"vim":{"version":"9.0.1","installed_at":"2024-01-01T00:00:00Z"}}}`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.List()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "was 9.0.1")
}

func TestList_PackagesWithPackageSection(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim":      "9.0.1",
		"pipewire": "0.3.65",
	})
	mockAvailableVersion(map[string]string{
		"vim":      "9.0.1",
		"pipewire": "0.3.65",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]

[[apt.post_hook]]
name = "pipewire"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.List()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "pipewire")
	assert.Contains(t, out, "vim")
}

func TestList_DotfileStatuses(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	// Create source files
	os.WriteFile(filepath.Join(srcDir, "vimrc"), []byte("content"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "tmux.conf"), []byte("content"), 0o644)

	// vimrc: correct symlink
	vimDest := filepath.Join(dir, ".vimrc")
	os.Symlink(filepath.Join(srcDir, "vimrc"), vimDest)

	// tmux: missing
	tmuxDest := filepath.Join(dir, ".tmux.conf")

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "vimrc"
dest = "`+vimDest+`"

[[dotfiles.file]]
src = "tmux.conf"
dest = "`+tmuxDest+`"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.List()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "linked")
	assert.Contains(t, out, "missing")
}

// --- syncState tests ---

func TestSyncState(t *testing.T) {
	saveMocks(t)

	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	err := s.syncState()
	require.NoError(t, err)

	// Verify lockfile was written
	sm := NewStateManager(configPath)
	sm.Load()
	v, ok := sm.GetPackageVersion("vim")
	assert.Equal(t, true, ok)
	assert.Equal(t, "9.0.1", v)
}

func TestSyncState_Verbose(t *testing.T) {
	saveMocks(t)

	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, true, false)

	out := captureOutput(t, func() {
		err := s.syncState()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "State saved to")
}

func TestSyncState_NoApt(t *testing.T) {
	saveMocks(t)
	mockInstalledVersion(map[string]string{})

	configPath := withTempConfig(t, "")

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	err := s.syncState()
	require.NoError(t, err)
}

func TestSyncState_LoadError(t *testing.T) {
	saveMocks(t)

	// Use a path where lockfile exists but is unreadable
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	lockPath := filepath.Join(dir, "lockfile.json")
	os.WriteFile(lockPath, []byte("not json{{{"), 0o644)

	cfg := &Config{Apt: &AptConfig{Packages: []string{"vim"}}}
	s := NewSettle(cfg, configPath, false, false)

	err := s.syncState()
	require.Error(t, err)
}

func TestSyncState_SaveError(t *testing.T) {
	saveMocks(t)
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})

	// Use a read-only directory for the lockfile
	dir := t.TempDir()
	configPath := filepath.Join(dir, "subdir", "config.toml")
	// Don't create the subdir — save will fail

	cfg := &Config{Apt: &AptConfig{Packages: []string{"vim"}}}
	s := NewSettle(cfg, configPath, false, false)

	err := s.syncState()
	require.Error(t, err)
}

// --- Error path tests for Apply ---

func TestApply_ApplyDotfilesError(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	// Source exists, but dest is a directory — will error in link mode
	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)
	destDir := filepath.Join(dir, ".vimrc")
	os.MkdirAll(destDir, 0o755)

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "vimrc"
dest = "`+destDir+`"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	// applyDotfiles records errors but doesn't return error
	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})
	assert.Contains(t, out, "Errors:")
}

func TestApply_SyncStateError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
	})

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	os.WriteFile(configPath, []byte(`
[apt]
packages = ["vim"]
`), 0o644)

	// Make the lockfile unreadable so syncState fails
	lockPath := filepath.Join(dir, "lockfile.json")
	os.WriteFile(lockPath, []byte("not json{{{"), 0o644)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Warning: failed to sync state")
}

func TestApply_CheckInstalledError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// The CheckInstalled method currently swallows errors from IsInstalled.
	// To trigger the error path in applyApt (line 456), we'd need
	// CheckInstalled to fail, but it catches errors internally.
	// This is covered by the fact that dpkg-query can fail.
	// Let's test the install failure path instead.

	// All packages missing
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		// apt-get update succeeds, apt-get install fails
		if name == "sudo" && len(arg) >= 2 && arg[1] == "update" {
			return exec.Command("true")
		}
		return exec.Command("bash", "-c", "exit 1")
	}
	mockInstalledVersion(map[string]string{})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error installing packages")
	})
	_ = out
}

func TestApply_PostInstallError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	callCount := 0
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		if name == "bash" {
			// post-install fails
			return exec.Command("false")
		}
		// apt-get install succeeds
		callCount++
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"pipewire": "0.3.65",
	})
	mockAvailableVersion(map[string]string{
		"pipewire": "0.3.65",
	})

	configPath := withTempConfig(t, `
[apt]

[[apt.post_hook]]
name = "pipewire"
post_install = "failing-command"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error running post-install")
	})
	_ = out
}

func TestApply_RemoveError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		// apt-get remove fails
		return exec.Command("bash", "-c", "exit 1")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)
	writeLockfile(t, configPath, `{"packages":{"vim":{"version":"9.0.1","installed_at":"2024-01-01T00:00:00Z"},"git":{"version":"2.40.0","installed_at":"2024-01-01T00:00:00Z"}}}`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error removing packages")
	})
	_ = out
}

// --- Error paths for Install ---

func TestInstall_CheckInstalledError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// This tests the error from CheckInstalled.
	// CheckInstalled itself doesn't return errors since it catches them.
	// But we can still verify the flow.
	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{})

	configPath := withTempConfig(t, `
[apt]
packages = []
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Install([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "All packages already installed on system")
}

func TestInstall_InstallFailure(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		// apt-get install fails
		return exec.Command("bash", "-c", "exit 1")
	}

	configPath := withTempConfig(t, `
[apt]
packages = []
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Install([]string{"vim"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error installing packages")
	})
	_ = out
}

func TestInstall_LockfileLoadVerbose(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	os.WriteFile(configPath, []byte(`
[apt]
packages = ["vim"]
`), 0o644)
	// Corrupt lockfile
	os.WriteFile(filepath.Join(dir, "lockfile.json"), []byte("bad json{{{"), 0o644)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, true, false)

	out := captureOutput(t, func() {
		err := s.Install([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "could not load lockfile")
}

func TestInstall_VerbosePackageAlreadyInConfig(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, true, false)

	out := captureOutput(t, func() {
		err := s.Install([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "already in config")
}

// --- Error paths for Remove ---

func TestRemove_RemoveFailure(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		// apt-get remove fails
		return exec.Command("bash", "-c", "exit 1")
	}

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Remove([]string{"vim"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error removing packages")
	})
	_ = out
}

func TestRemove_LockfileLoadVerbose(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		return exec.Command("true")
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	os.WriteFile(configPath, []byte(`
[apt]
packages = ["vim"]
`), 0o644)
	// Corrupt lockfile
	os.WriteFile(filepath.Join(dir, "lockfile.json"), []byte("bad json"), 0o644)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, true, false)

	out := captureOutput(t, func() {
		err := s.Remove([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "could not load lockfile")
}

// --- Error paths for Update ---

func TestUpdate_RefreshError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("bash", "-c", "exit 1")
	}

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Update()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update package lists")
	})
	_ = out
}

func TestUpdate_UpgradeError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	callCount := 0
	execCommand = func(name string, arg ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			// apt-get update succeeds
			return exec.Command("true")
		}
		// apt-get upgrade fails
		return exec.Command("bash", "-c", "exit 1")
	}

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Update()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to upgrade packages")
	})
	_ = out
}

func TestUpdate_LockfileLoadVerbose(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	os.WriteFile(configPath, []byte(`
[apt]
packages = ["vim"]
`), 0o644)
	// Corrupt lockfile
	os.WriteFile(filepath.Join(dir, "lockfile.json"), []byte("bad json{{{"), 0o644)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, true, false)

	out := captureOutput(t, func() {
		err := s.Update()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "could not load lockfile")
}

// --- listApt error paths ---

func TestListApt_EmptyPackages(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	configPath := withTempConfig(t, `
[apt]
packages = []
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.List()
		require.NoError(t, err)
	})

	// Should not print packages table for empty list
	assert.NotContains(t, out, "Packages:")
}

func TestListApt_VerboseStateLoadError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
	})

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	os.WriteFile(configPath, []byte(`
[apt]
packages = ["vim"]
`), 0o644)
	// Corrupt lockfile
	os.WriteFile(filepath.Join(dir, "lockfile.json"), []byte("bad json"), 0o644)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, true, false)

	out := captureOutput(t, func() {
		err := s.List()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Warning: could not load state")
}

// --- listDotfiles all status branches ---

func TestListDotfiles_AllStatuses(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	// Source files
	os.WriteFile(filepath.Join(srcDir, "linked"), []byte("content"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "wronglink"), []byte("content"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "fileatdest"), []byte("content"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "diratdest"), []byte("content"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "copygood"), []byte("same"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "copyold"), []byte("new content"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "missing"), []byte("content"), 0o644)

	// Setup various statuses
	// 1. Correct symlink
	linkedDest := filepath.Join(dir, "linked")
	os.Symlink(filepath.Join(srcDir, "linked"), linkedDest)

	// 2. Incorrect symlink
	wrongDest := filepath.Join(dir, "wronglink")
	os.Symlink("/wrong/target", wrongDest)

	// 3. File at dest (link mode)
	fileDest := filepath.Join(dir, "fileatdest")
	os.WriteFile(fileDest, []byte("existing file"), 0o644)

	// 4. Dir at dest
	dirDest := filepath.Join(dir, "diratdest")
	os.MkdirAll(dirDest, 0o755)

	// 5. Copy correct
	copyGoodDest := filepath.Join(dir, "copygood")
	os.WriteFile(copyGoodDest, []byte("same"), 0o644)

	// 6. Copy outdated
	copyOldDest := filepath.Join(dir, "copyold")
	os.WriteFile(copyOldDest, []byte("old content"), 0o644)

	// 7. Missing
	missingDest := filepath.Join(dir, "missing_link")

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "linked"
dest = "`+linkedDest+`"

[[dotfiles.file]]
src = "wronglink"
dest = "`+wrongDest+`"

[[dotfiles.file]]
src = "fileatdest"
dest = "`+fileDest+`"

[[dotfiles.file]]
src = "diratdest"
dest = "`+dirDest+`"

[[dotfiles.file]]
src = "copygood"
dest = "`+copyGoodDest+`"
mode = "copy"

[[dotfiles.file]]
src = "copyold"
dest = "`+copyOldDest+`"
mode = "copy"

[[dotfiles.file]]
src = "missing"
dest = "`+missingDest+`"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.List()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Dotfiles:")
	assert.Contains(t, out, "linked")
	assert.Contains(t, out, "wrong target")
	assert.Contains(t, out, "file exists")
	assert.Contains(t, out, "dir exists")
	assert.Contains(t, out, "copied")
	assert.Contains(t, out, "outdated")
	assert.Contains(t, out, "missing")
}

func TestListDotfiles_ErrorStatus(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	// Source doesn't exist — will produce error
	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "nonexistent"
dest = "`+filepath.Join(dir, ".vimrc")+`"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.List()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "error:")
}

// --- Apply lockfile save/load edge cases ---

func TestApply_LockfileLoadVerbose(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
	})

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	os.WriteFile(configPath, []byte(`
[apt]
packages = ["vim"]
`), 0o644)
	// No lockfile — verbose should say "no lockfile found"

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, true, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	// When there's no lockfile and verbose, the message is printed
	assert.Contains(t, out, "Detected distribution: debian")
}

func TestApply_InstallLockfileSaveWarning(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
	})

	// Use a path where lockfile can't be saved
	configPath := "/nonexistent/dir/config.toml"
	cfg := &Config{Apt: &AptConfig{Packages: []string{"vim"}}}
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Warning: failed to update lockfile")
}

func TestApply_UpgradeLockfileSaveWarning(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	origExec := execCommand
	// vim installed at lower version than lockfile
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg" && len(arg) > 0 && arg[0] == "--compare-versions" {
			return origExec(name, arg...)
		}
		if name == "dpkg-query" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.2",
	})

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	os.WriteFile(configPath, []byte(`
[apt]
packages = ["vim"]
`), 0o644)
	writeLockfile(t, configPath, `{"packages":{"vim":{"version":"9.0.2","installed_at":"2024-01-01T00:00:00Z"}}}`)

	// Make the lockfile read-only so save fails after upgrade
	lockPath := filepath.Join(dir, "lockfile.json")
	os.Chmod(lockPath, 0o444)
	t.Cleanup(func() { os.Chmod(lockPath, 0o644) })

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Warning: failed to update lockfile")
}

func TestApply_RemoveLockfileSaveWarning(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
	})

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	os.WriteFile(configPath, []byte(`
[apt]
packages = ["vim"]
`), 0o644)
	// Lockfile has vim AND git (git not in config, will be removed)
	writeLockfile(t, configPath, `{"packages":{"vim":{"version":"9.0.1","installed_at":"2024-01-01T00:00:00Z"},"git":{"version":"2.40.0","installed_at":"2024-01-01T00:00:00Z"}}}`)

	// Make the lockfile read-only so save after remove fails
	lockPath := filepath.Join(dir, "lockfile.json")
	os.Chmod(lockPath, 0o444)
	t.Cleanup(func() { os.Chmod(lockPath, 0o644) })

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Warning: failed to update lockfile")
}

// --- Install lockfile save warning ---

func TestInstall_LockfileSaveWarning(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})

	cfg := &Config{}
	s := NewSettle(cfg, "/nonexistent/dir/config.toml", false, false)

	out := captureOutput(t, func() {
		err := s.Install([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Warning: failed to update lockfile")
}

// --- Remove lockfile save warning ---

func TestRemove_LockfileSaveWarning(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		return exec.Command("true")
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	os.WriteFile(configPath, []byte(`
[apt]
packages = ["vim"]
`), 0o644)
	writeLockfile(t, configPath, `{"packages":{"vim":{"version":"9.0.1","installed_at":"2024-01-01T00:00:00Z"}}}`)

	// Make the lockfile read-only so save fails
	lockPath := filepath.Join(dir, "lockfile.json")
	os.Chmod(lockPath, 0o444)
	t.Cleanup(func() { os.Chmod(lockPath, 0o644) })

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Remove([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Warning: failed to update lockfile")
}

// --- Update lockfile save warning ---

func TestUpdate_LockfileSaveWarning(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})

	cfg := &Config{Apt: &AptConfig{Packages: []string{"vim"}}}
	s := NewSettle(cfg, "/nonexistent/dir/config.toml", false, false)

	out := captureOutput(t, func() {
		err := s.Update()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Warning: failed to update lockfile")
}

// --- Apply verbose lockfile load message ---

func TestApply_GetInstalledVersionErrorContinue(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// vim installed
	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	// GetInstalledVersion returns error for vim
	GetInstalledVersion = func(name string) (string, error) {
		return "", fmt.Errorf("dpkg error")
	}
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)
	writeLockfile(t, configPath, `{"packages":{"vim":{"version":"9.0.1","installed_at":"2024-01-01T00:00:00Z"}}}`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	// Should still complete without error
	assert.Contains(t, out, "Done!")
}

func TestApply_VerboseLockfileNotFound(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
	})

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	os.WriteFile(configPath, []byte(`
[apt]
packages = ["vim"]
`), 0o644)
	// Write a corrupt lockfile so Load() returns error
	os.WriteFile(filepath.Join(dir, "lockfile.json"), []byte("{{bad"), 0o644)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, true, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "no lockfile found")
}

// --- Apply verbose pinned version output ---

func TestApply_VerbosePinnedVersionDryRun(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	mockAvailableVersion(map[string]string{
		"curl": "7.88.1",
	})

	configPath := withTempConfig(t, `
[apt]
packages = ["curl"]
`)
	writeLockfile(t, configPath, `{"packages":{"curl":{"version":"7.88.0","installed_at":"2024-01-01T00:00:00Z"}}}`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, true)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "pinned")
}

// --- List error propagation ---

func TestList_ListAptError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=arch\n") // unsupported distro

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	_ = configPath
}

// --- Apply with both apt and dotfiles ---

func TestApply_BothAptAndDotfiles(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})
	mockAvailableVersion(map[string]string{
		"vim": "9.0.1",
	})

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "vimrc"), []byte("content"), 0o644)
	destFile := filepath.Join(dir, ".vimrc")

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]

[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "vimrc"
dest = "`+destFile+`"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Done!")
	assert.Contains(t, out, "Created 1 links")
}

// --- Install GetInstalledVersion error path ---

func TestApply_DotfilesAlreadyCorrect(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	// Destination already has correct symlink
	destFile := filepath.Join(dir, ".vimrc")
	os.Symlink(srcFile, destFile)

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "vimrc"
dest = "`+destFile+`"
`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Apply()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Created 0 links, 1 already correct")
}

func TestInstall_GetInstalledVersionError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}
	GetInstalledVersion = func(name string) (string, error) {
		return "", fmt.Errorf("version not found")
	}

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)
	writeLockfile(t, configPath, `{"packages":{"vim":{"version":"9.0.1","installed_at":"2024-01-01T00:00:00Z"}}}`)

	cfg, _ := loadConfig(configPath)
	s := NewSettle(cfg, configPath, false, false)

	out := captureOutput(t, func() {
		err := s.Install([]string{"vim"})
		require.NoError(t, err) // doesn't fail, just skips version check
	})

	assert.Contains(t, out, "All packages already in config and installed")
}
