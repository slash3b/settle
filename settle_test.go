package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Apply tests ---

func TestApply_NoConfig(t *testing.T) {
	saveMocks(t)

	cfg := &Config{}

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no packages, dotfiles, git repos, or go packages configured")
}

func TestApply_AllInstalled(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(_ string, _ ...string) *exec.Cmd {
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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "All packages already installed")
	assert.Contains(t, out, "Done!")
}

func TestApply_InstallsMissing(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// vim installed, curl not
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" { //nolint:goconst
			if len(arg) >= 3 && arg[2] == "vim" {
				return exec.Command("echo", "-n", "install ok installed")
			}

			return exec.Command("false")
		}

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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "Need to install: 1")
	assert.Contains(t, out, "Done!")
}

func TestApply_DryRun(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// curl not installed
	execCommand = func(name string, _ ...string) *exec.Cmd {
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

	var w strings.Builder

	s := NewSettle(cfg, false, true, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "[dry-run")
	assert.Contains(t, out, "Done!")
}

func TestApply_SkipsUnknownPackages(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, _ ...string) *exec.Cmd {
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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported distribution")
}

func TestApply_Verbose(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(_ string, _ ...string) *exec.Cmd {
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

	var w strings.Builder

	s := NewSettle(cfg, true, false, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "No apt packages configured")
}

func TestApply_PostInstallHooks(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// pipewire not installed
	execCommand = func(name string, _ ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}

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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "Running post-install for pipewire")
	assert.Contains(t, out, "Done!")
}

func TestApply_DryRunWithPostInstall(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, _ ...string) *exec.Cmd {
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

	var w strings.Builder

	s := NewSettle(cfg, false, true, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "[dry-run] Would install")
	assert.Contains(t, out, "[dry-run] Would run post-install for pipewire")
}

// --- Dotfiles Apply tests ---

func TestApply_Dotfiles(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "vimrc"
dest = "`+destFile+`"
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "No dotfiles configured")
}

func TestApply_DotfilesDryRun(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "vimrc"
dest = "`+destFile+`"
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, true, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "[dry-run] Would link")
	assert.Contains(t, out, "[dry-run] Would create 1 links")
}

func TestApply_DotfilesVerbose(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "vimrc"
dest = "`+destFile+`"
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, true, false, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "Linked:")
}

func TestApply_DotfilesWithErrors(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "nonexistent"
dest = "`+filepath.Join(dir, ".vimrc")+`"
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

// --- Apply error paths ---

func TestApply_ApplyDotfilesError(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	// Source exists, but dest is a directory — will error in link mode
	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	destDir := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "vimrc"
dest = "`+destDir+`"
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory exists at destination")
}

func TestApply_CheckInstalledError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	// All packages missing, apt-get install fails
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}

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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error installing packages")
}

func TestApply_PostInstallError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(name string, _ ...string) *exec.Cmd {
		if name == "dpkg-query" {
			return exec.Command("false")
		}

		if name == "bash" {
			return exec.Command("false")
		}

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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "post-install for pipewire")
}

func TestApply_BothAptAndDotfiles(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(_ string, _ ...string) *exec.Cmd {
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
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "vimrc"), []byte("content"), 0o644))
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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "Done!")
	assert.Contains(t, out, "Created 1 links")
}

func TestApply_DotfilesAlreadyCorrect(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	// Destination already has correct symlink
	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.Symlink(srcFile, destFile))

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "vimrc"
dest = "`+destFile+`"
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "Created 0 links, 1 already correct")
}

// --- Update tests ---

func TestUpdate_Success(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(_ string, _ ...string) *exec.Cmd {
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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Update()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "Updating 1 managed packages")
}

func TestUpdate_NoPackages(t *testing.T) {
	cfg := &Config{}

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Update()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "nothing to update")
}

func TestUpdate_EmptyPackages(t *testing.T) {
	cfg := &Config{Apt: &AptConfig{}}

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Update()
	require.NoError(t, err)

	assert.Empty(t, w.String())
}

func TestUpdate_DryRun(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, true, &w)

	err := s.Update()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "[dry-run] Would run: apt-get update")
	assert.Contains(t, out, "[dry-run] Would upgrade")
}

func TestUpdate_UnsupportedDistro(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=arch\n")

	cfg := &Config{Apt: &AptConfig{Packages: []string{"vim"}}}

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Update()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported distribution")
}

func TestUpdate_WithPackageSection(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(_ string, _ ...string) *exec.Cmd {
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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Update()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "Updating 2 managed packages")
}

func TestUpdate_Verbose(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(_ string, _ ...string) *exec.Cmd {
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

	var w strings.Builder

	s := NewSettle(cfg, true, false, &w)

	err := s.Update()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "Updating 1 managed packages")
}

func TestUpdate_RefreshError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(_ string, _ ...string) *exec.Cmd {
		return exec.Command("bash", "-c", "exit 1")
	}

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Update()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update package lists")
}

func TestUpdate_UpgradeError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	callCount := 0
	execCommand = func(_ string, _ ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			return exec.Command("true")
		}

		return exec.Command("bash", "-c", "exit 1")
	}

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Update()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to upgrade packages")
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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.List()

	out := w.String()

	assert.Contains(t, out, "Packages:")
	assert.Contains(t, out, "vim")
	assert.Contains(t, out, "curl")
}

func TestList_Empty(t *testing.T) {
	cfg := &Config{}

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.List()

	assert.Empty(t, w.String())
}

func TestList_Dotfiles(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.Symlink(srcFile, destFile))

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "vimrc"
dest = "`+destFile+`"
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.List()

	out := w.String()

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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.List()

	out := w.String()

	assert.NotContains(t, out, "Dotfiles:")
}

func TestList_PackagesUpgradeAvailable(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(_ string, _ ...string) *exec.Cmd {
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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.List()

	out := w.String()

	assert.Contains(t, out, "upgrade: 9.0.2")
}

func TestList_UnknownPackage(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(_ string, _ ...string) *exec.Cmd {
		return exec.Command("false")
	}

	mockInstalledVersion(map[string]string{})
	mockAvailableVersion(map[string]string{})

	configPath := withTempConfig(t, `
[apt]
packages = ["badpkg"]
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.List()

	out := w.String()

	assert.Contains(t, out, "unknown")
}

func TestList_InstalledVersionUnknown(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(_ string, _ ...string) *exec.Cmd {
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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.List()

	out := w.String()

	assert.Contains(t, out, "installed (version unknown)")
}

func TestList_PackagesWithPackageSection(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	execCommand = func(_ string, _ ...string) *exec.Cmd {
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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.List()

	out := w.String()

	assert.Contains(t, out, "pipewire")
	assert.Contains(t, out, "vim")
}

func TestList_DotfileStatuses(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "vimrc"), []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "tmux.conf"), []byte("content"), 0o644))

	vimDest := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.Symlink(filepath.Join(srcDir, "vimrc"), vimDest))

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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.List()

	out := w.String()

	assert.Contains(t, out, "linked")
	assert.Contains(t, out, "missing")
}

func TestList_ListAptError(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=arch\n") // unsupported distro

	configPath := withTempConfig(t, `
[apt]
packages = ["vim"]
`)

	_ = configPath
}

// --- listApt ---

func TestListApt_EmptyPackages(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\n")

	configPath := withTempConfig(t, `
[apt]
packages = []
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.List()

	out := w.String()

	assert.NotContains(t, out, "Packages:")
}

// --- listDotfiles all status branches ---

func TestListDotfiles_AllStatuses(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "linked"), []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "wronglink"), []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "fileatdest"), []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "diratdest"), []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "copygood"), []byte("same"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "copyold"), []byte("new content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "missing"), []byte("content"), 0o644))

	linkedDest := filepath.Join(dir, "linked")
	require.NoError(t, os.Symlink(filepath.Join(srcDir, "linked"), linkedDest))

	wrongDest := filepath.Join(dir, "wronglink")
	require.NoError(t, os.Symlink("/wrong/target", wrongDest))

	fileDest := filepath.Join(dir, "fileatdest")
	require.NoError(t, os.WriteFile(fileDest, []byte("existing file"), 0o644))

	dirDest := filepath.Join(dir, "diratdest")
	require.NoError(t, os.MkdirAll(dirDest, 0o755))

	copyGoodDest := filepath.Join(dir, "copygood")
	require.NoError(t, os.WriteFile(copyGoodDest, []byte("same"), 0o644))

	copyOldDest := filepath.Join(dir, "copyold")
	require.NoError(t, os.WriteFile(copyOldDest, []byte("old content"), 0o644))

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

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.List()

	out := w.String()

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
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	configPath := withTempConfig(t, `
[dotfiles]
source_dir = "`+srcDir+`"

[[dotfiles.file]]
src = "nonexistent"
dest = "`+filepath.Join(dir, ".vimrc")+`"
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.List()

	out := w.String()

	assert.Contains(t, out, "error:")
}

// --- Git Apply tests ---

func TestApplyGit_ClonesMissing(t *testing.T) {
	saveMocks(t)

	remote := t.TempDir()
	run(t, remote, "init", "--bare")

	scratch := t.TempDir()
	run(t, scratch, "clone", remote, "work")
	work := filepath.Join(scratch, "work")
	run(t, work, "config", "user.email", "test@test.com")
	run(t, work, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(work, "README.md"), []byte("hello"), 0o644))
	run(t, work, "add", ".")
	run(t, work, "commit", "-m", "init")
	run(t, work, "push")

	dest := filepath.Join(t.TempDir(), "cloned")

	configPath := withTempConfig(t, fmt.Sprintf(`
[[git]]
url = "%s"
dest = "%s"
`, remote, dest))

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "Checking 1 git repos")
	assert.Contains(t, out, "Done!")

	_, err = os.Stat(filepath.Join(dest, ".git"))
	require.NoError(t, err)

	data, _ := os.ReadFile(filepath.Join(dest, "README.md"))
	assert.Equal(t, "hello", string(data))
}

func TestApplyGit_SkipsExisting(t *testing.T) {
	saveMocks(t)

	remote := t.TempDir()
	run(t, remote, "init", "--bare")

	scratch := t.TempDir()
	run(t, scratch, "clone", remote, "work")
	work := filepath.Join(scratch, "work")
	run(t, work, "config", "user.email", "test@test.com")
	run(t, work, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(work, "README.md"), []byte("hello"), 0o644))
	run(t, work, "add", ".")
	run(t, work, "commit", "-m", "init")
	run(t, work, "push")

	dest := filepath.Join(t.TempDir(), "cloned")
	run(t, filepath.Dir(dest), "clone", remote, dest)

	configPath := withTempConfig(t, fmt.Sprintf(`
[[git]]
url = "%s"
dest = "%s"
`, remote, dest))

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "Done!")
	_ = out
}

func TestApplyGit_DryRun(t *testing.T) {
	saveMocks(t)

	remote := t.TempDir()
	run(t, remote, "init", "--bare")

	dest := filepath.Join(t.TempDir(), "cloned")

	configPath := withTempConfig(t, fmt.Sprintf(`
[[git]]
url = "%s"
dest = "%s"
`, remote, dest))

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, true, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "[dry-run] Would clone")

	_, err = os.Stat(dest)
	assert.True(t, os.IsNotExist(err))
}

func TestApplyGit_DestNotRepo(t *testing.T) {
	saveMocks(t)

	dest := t.TempDir()

	configPath := withTempConfig(t, fmt.Sprintf(`
[[git]]
url = "https://example.com/repo.git"
dest = "%s"
`, dest))

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestApplyGit_DestIsFile(t *testing.T) {
	saveMocks(t)

	dest := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(dest, []byte("not a dir"), 0o644))

	configPath := withTempConfig(t, fmt.Sprintf(`
[[git]]
url = "https://example.com/repo.git"
dest = "%s"
`, dest))

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

// --- Git Update tests ---

func TestUpdateGit_PullsExisting(t *testing.T) {
	saveMocks(t)

	remote := t.TempDir()
	run(t, remote, "init", "--bare")

	parent := t.TempDir()
	dest := filepath.Join(parent, "repo")
	run(t, parent, "clone", remote, dest)
	run(t, dest, "config", "user.email", "test@test.com")
	run(t, dest, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(dest, "file.txt"), []byte("initial"), 0o644))
	run(t, dest, "add", ".")
	run(t, dest, "commit", "-m", "initial")
	run(t, dest, "push", "-u", "origin", "HEAD")

	other := filepath.Join(parent, "other")
	run(t, parent, "clone", remote, other)
	run(t, other, "config", "user.email", "test@test.com")
	run(t, other, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(other, "new.txt"), []byte("new"), 0o644))
	run(t, other, "add", ".")
	run(t, other, "commit", "-m", "add new file")
	run(t, other, "push")

	configPath := withTempConfig(t, fmt.Sprintf(`
[[git]]
url = "%s"
dest = "%s"
`, remote, dest))

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Update()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "Updating 1 git repos")

	data, err := os.ReadFile(filepath.Join(dest, "new.txt"))
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))
}

func TestUpdateGit_NotCloned(t *testing.T) {
	saveMocks(t)

	dest := filepath.Join(t.TempDir(), "missing")

	configPath := withTempConfig(t, fmt.Sprintf(`
[[git]]
url = "https://example.com/repo.git"
dest = "%s"
`, dest))

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Update()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "not cloned, run settle first")
}

func TestUpdateGit_DryRun(t *testing.T) {
	saveMocks(t)

	remote := t.TempDir()
	run(t, remote, "init", "--bare")

	parent := t.TempDir()
	dest := filepath.Join(parent, "repo")
	run(t, parent, "clone", remote, dest)
	run(t, dest, "config", "user.email", "test@test.com")
	run(t, dest, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(dest, "file.txt"), []byte("data"), 0o644))
	run(t, dest, "add", ".")
	run(t, dest, "commit", "-m", "init")
	run(t, dest, "push", "-u", "origin", "HEAD")

	configPath := withTempConfig(t, fmt.Sprintf(`
[[git]]
url = "%s"
dest = "%s"
`, remote, dest))

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, true, &w)

	err := s.Update()
	require.NoError(t, err)

	out := w.String()

	assert.Contains(t, out, "[dry-run] Would pull")
}

// --- Git List tests ---

func TestListGit(t *testing.T) {
	saveMocks(t)

	remote := t.TempDir()
	run(t, remote, "init", "--bare")

	parent := t.TempDir()
	clonedDest := filepath.Join(parent, "cloned")
	run(t, parent, "clone", remote, clonedDest)

	missingDest := filepath.Join(parent, "missing")

	notRepoDest := filepath.Join(parent, "notagitrepo")
	require.NoError(t, os.MkdirAll(notRepoDest, 0o755))

	configPath := withTempConfig(t, fmt.Sprintf(`
[[git]]
url = "%s"
dest = "%s"

[[git]]
url = "https://example.com/repo.git"
dest = "%s"

[[git]]
url = "https://example.com/repo2.git"
dest = "%s"
`, remote, clonedDest, missingDest, notRepoDest))

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.List()

	out := w.String()

	assert.Contains(t, out, "Git Repos:")
	assert.Contains(t, out, "cloned")
	assert.Contains(t, out, "missing")
	assert.Contains(t, out, "not a git repo")
}

// --- Go package tests ---

func TestApplyGo_SkipsExisting(t *testing.T) {
	saveMocks(t)

	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "golangci-lint"), []byte("fake"), 0o755))

	GoBinPath = func() (string, error) { return binDir, nil }
	installCalled := false
	GoInstall = func(_, _ string, _ bool) error {
		installCalled = true
		return nil
	}

	configPath := withTempConfig(t, `
[[go]]
path = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
version = "v2.9.0"
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.applyGo()
	require.NoError(t, err)

	out := w.String()

	assert.False(t, installCalled)
	assert.Contains(t, out, "1 packages already installed")
}

func TestApplyGo_InstallsMissing(t *testing.T) {
	saveMocks(t)

	binDir := t.TempDir()
	GoBinPath = func() (string, error) { return binDir, nil }

	var installedPkg string

	GoInstall = func(path, _ string, _ bool) error {
		installedPkg = path
		return nil
	}

	configPath := withTempConfig(t, `
[[go]]
path = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
version = "v2.9.0"
`)
	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.applyGo()
	require.NoError(t, err)

	out := w.String()

	assert.Equal(t, "github.com/golangci/golangci-lint/v2/cmd/golangci-lint", installedPkg)
	assert.Contains(t, out, "golangci-lint")
	assert.Contains(t, out, "installed")
}

func TestApplyGo_DryRun(t *testing.T) {
	saveMocks(t)

	binDir := t.TempDir()
	GoBinPath = func() (string, error) { return binDir, nil }

	installCalled := false
	GoInstall = func(_, _ string, _ bool) error {
		installCalled = true
		return nil
	}

	configPath := withTempConfig(t, `
[[go]]
path = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
version = "v2.9.0"
`)
	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, true, &w)

	err := s.applyGo()
	require.NoError(t, err)

	out := w.String()

	assert.False(t, installCalled)
	assert.Contains(t, out, "[dry-run]")
	assert.Contains(t, out, "golangci-lint")
}

func TestApplyGo_InstallError(t *testing.T) {
	saveMocks(t)

	binDir := t.TempDir()
	GoBinPath = func() (string, error) { return binDir, nil }
	GoInstall = func(_, _ string, _ bool) error {
		return fmt.Errorf("network error")
	}

	configPath := withTempConfig(t, `
[[go]]
path = "github.com/user/tool"
version = "v1.0.0"
`)
	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.applyGo()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to install")
}

func TestApplyGo_BinPathError(t *testing.T) {
	saveMocks(t)

	GoBinPath = func() (string, error) { return "", fmt.Errorf("go not found") }

	configPath := withTempConfig(t, `
[[go]]
path = "github.com/user/tool"
version = "v1.0.0"
`)
	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.applyGo()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot determine Go bin path")
}

func TestUpdateGo(t *testing.T) {
	saveMocks(t)

	var installed []string

	GoInstall = func(path, version string, _ bool) error {
		installed = append(installed, path+"@"+version)
		return nil
	}

	configPath := withTempConfig(t, `
[[go]]
path = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
version = "v2.9.0"

[[go]]
path = "golang.org/x/tools/cmd/goimports"
version = "v0.25.0"
`)
	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.updateGo()
	require.NoError(t, err)

	out := w.String()

	assert.Len(t, installed, 2)
	assert.Contains(t, out, "Updating 2 go packages")
}

func TestUpdateGo_DryRun(t *testing.T) {
	saveMocks(t)

	installCalled := false
	GoInstall = func(_, _ string, _ bool) error {
		installCalled = true
		return nil
	}

	configPath := withTempConfig(t, `
[[go]]
path = "github.com/user/tool"
version = "v1.0.0"
`)
	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, true, &w)

	err := s.updateGo()
	require.NoError(t, err)

	out := w.String()

	assert.False(t, installCalled)
	assert.Contains(t, out, "[dry-run]")
}

func TestUpdateGo_Error(t *testing.T) {
	saveMocks(t)

	GoInstall = func(_, _ string, _ bool) error {
		return fmt.Errorf("failed")
	}

	configPath := withTempConfig(t, `
[[go]]
path = "github.com/user/tool"
version = "v1.0.0"
`)
	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.updateGo()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update")
}

func TestListGo(t *testing.T) {
	saveMocks(t)

	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "golangci-lint"), []byte("fake"), 0o755))

	GoBinPath = func() (string, error) { return binDir, nil }

	configPath := withTempConfig(t, `
[[go]]
path = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
version = "v2.9.0"

[[go]]
path = "golang.org/x/tools/cmd/goimports"
version = "v0.25.0"
`)

	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.listGo()

	out := w.String()

	assert.Contains(t, out, "Go Packages:")
	assert.Contains(t, out, "golangci-lint")
	assert.Contains(t, out, "v2.9.0")
	assert.Contains(t, out, "goimports")
	assert.Contains(t, out, "missing")
}

func TestListGo_BinPathError(t *testing.T) {
	saveMocks(t)

	GoBinPath = func() (string, error) { return "", fmt.Errorf("go not found") }

	configPath := withTempConfig(t, `
[[go]]
path = "github.com/user/tool"
version = "v1.0.0"
`)
	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	s.listGo()

	out := w.String()

	assert.Contains(t, out, "Warning: cannot determine Go bin path")
}

func TestApply_GoPackages(t *testing.T) {
	saveMocks(t)

	binDir := t.TempDir()
	GoBinPath = func() (string, error) { return binDir, nil }

	var installed []string

	GoInstall = func(path, _ string, _ bool) error {
		installed = append(installed, path)
		return nil
	}

	configPath := withTempConfig(t, `
[[go]]
path = "github.com/user/tool"
version = "v1.0.0"
`)
	cfg, _ := loadConfig(configPath)

	var w strings.Builder

	s := NewSettle(cfg, false, false, &w)

	err := s.Apply()
	require.NoError(t, err)

	out := w.String()

	assert.Len(t, installed, 1)
	assert.Contains(t, out, "Done!")
}
