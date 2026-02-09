package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNewStateManager(t *testing.T) {
	sm := NewStateManager("/home/user/dotfiles/config.toml")
	assertEqualStr(t, sm.Path(), "/home/user/dotfiles/lockfile.json")
}

func TestStateManager_LoadNoFile(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(filepath.Join(dir, "config.toml"))

	err := sm.Load()
	assertNoError(t, err)

	// Should have empty packages
	pkgs := sm.GetAllPackages()
	assertEqualInt(t, len(pkgs), 0)
}

func TestStateManager_LoadExisting(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	// Write a lockfile
	state := State{
		Packages: map[string]PackageState{
			"vim": {Version: "9.0.1"},
			"git": {Version: "2.40.0"},
		},
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(filepath.Join(dir, "lockfile.json"), data, 0o644)

	sm := NewStateManager(configPath)
	err := sm.Load()
	assertNoError(t, err)

	v, ok := sm.GetPackageVersion("vim")
	assertEqualBool(t, ok, true)
	assertEqualStr(t, v, "9.0.1")

	v, ok = sm.GetPackageVersion("git")
	assertEqualBool(t, ok, true)
	assertEqualStr(t, v, "2.40.0")
}

func TestStateManager_LoadCorrupt(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	os.WriteFile(filepath.Join(dir, "lockfile.json"), []byte("not json{{{"), 0o644)

	sm := NewStateManager(configPath)
	err := sm.Load()
	assertError(t, err)
	assertContains(t, err.Error(), "failed to parse state file")
}

func TestStateManager_LoadReadError(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	lockPath := filepath.Join(dir, "lockfile.json")
	os.WriteFile(lockPath, []byte("{}"), 0o644)
	os.Chmod(lockPath, 0o000)
	t.Cleanup(func() { os.Chmod(lockPath, 0o644) })

	sm := NewStateManager(configPath)
	err := sm.Load()
	assertError(t, err)
	assertContains(t, err.Error(), "failed to read state file")
}

func TestStateManager_SaveNotDirty(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(filepath.Join(dir, "config.toml"))
	sm.Load()

	// Save without any changes — should be a no-op
	err := sm.Save()
	assertNoError(t, err)

	// Lockfile should NOT exist since nothing was dirty
	_, err = os.Stat(filepath.Join(dir, "lockfile.json"))
	if !os.IsNotExist(err) {
		t.Error("expected lockfile to not exist when nothing is dirty")
	}
}

func TestStateManager_SaveDirty(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	sm := NewStateManager(configPath)
	sm.Load()

	sm.SetPackageVersion("vim", "9.0.1")

	err := sm.Save()
	assertNoError(t, err)

	// Read back and verify
	data, err := os.ReadFile(filepath.Join(dir, "lockfile.json"))
	assertNoError(t, err)

	var state State
	json.Unmarshal(data, &state)
	assertEqualStr(t, state.Packages["vim"].Version, "9.0.1")
}

func TestStateManager_SaveWriteError(t *testing.T) {
	// Use a directory that doesn't exist
	sm := NewStateManager("/nonexistent/dir/config.toml")
	sm.SetPackageVersion("vim", "1.0")

	err := sm.Save()
	assertError(t, err)
	assertContains(t, err.Error(), "failed to write state file")
}

func TestStateManager_GetSetVersion(t *testing.T) {
	sm := NewStateManager("/tmp/config.toml")

	// Get non-existent
	_, ok := sm.GetPackageVersion("vim")
	assertEqualBool(t, ok, false)

	// Set and get
	sm.SetPackageVersion("vim", "9.0.1")
	v, ok := sm.GetPackageVersion("vim")
	assertEqualBool(t, ok, true)
	assertEqualStr(t, v, "9.0.1")
}

func TestStateManager_SetVersionPreservesInstalledAt(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	sm := NewStateManager(configPath)

	sm.SetPackageVersion("vim", "9.0.0")
	installedAt := sm.state.Packages["vim"].InstalledAt

	// Update version — InstalledAt should be preserved
	sm.SetPackageVersion("vim", "9.0.1")
	if sm.state.Packages["vim"].InstalledAt != installedAt {
		t.Error("InstalledAt should be preserved on version update")
	}
	assertEqualStr(t, sm.state.Packages["vim"].Version, "9.0.1")
}

func TestStateManager_SetVersionSameNoDirty(t *testing.T) {
	sm := NewStateManager("/tmp/config.toml")

	sm.SetPackageVersion("vim", "9.0.1")
	// Save to clear dirty flag
	sm.dirty = false

	// Set same version — should not mark dirty
	sm.SetPackageVersion("vim", "9.0.1")
	assertEqualBool(t, sm.dirty, false)
}

func TestStateManager_RemovePackage(t *testing.T) {
	sm := NewStateManager("/tmp/config.toml")
	sm.SetPackageVersion("vim", "9.0.1")
	sm.dirty = false

	sm.RemovePackage("vim")
	assertEqualBool(t, sm.dirty, true)

	_, ok := sm.GetPackageVersion("vim")
	assertEqualBool(t, ok, false)
}

func TestStateManager_RemoveNonexistent(t *testing.T) {
	sm := NewStateManager("/tmp/config.toml")

	sm.RemovePackage("nonexistent")
	assertEqualBool(t, sm.dirty, false)
}

func TestStateManager_GetAllPackages(t *testing.T) {
	sm := NewStateManager("/tmp/config.toml")
	sm.SetPackageVersion("vim", "9.0.1")
	sm.SetPackageVersion("git", "2.40.0")

	pkgs := sm.GetAllPackages()
	assertEqualInt(t, len(pkgs), 2)

	// Check both packages are present (order not guaranteed)
	found := make(map[string]bool)
	for _, p := range pkgs {
		found[p] = true
	}
	assertEqualBool(t, found["vim"], true)
	assertEqualBool(t, found["git"], true)
}

func TestStateManager_Path(t *testing.T) {
	sm := NewStateManager("/path/to/config.toml")
	assertEqualStr(t, sm.Path(), "/path/to/lockfile.json")
}

// Tests for the real GetInstalledVersion implementation (uses execCommand)
func TestGetInstalledVersion_Real_Success(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "2.40.0-1")
	}

	v, err := GetInstalledVersion("git")
	assertNoError(t, err)
	assertEqualStr(t, v, "2.40.0-1")
}

func TestGetInstalledVersion_Real_Error(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("false")
	}

	_, err := GetInstalledVersion("nonexistent")
	assertError(t, err)
}

func TestGetInstalledVersion_Real_Whitespace(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "  2.40.0-1  ")
	}

	v, err := GetInstalledVersion("git")
	assertNoError(t, err)
	assertEqualStr(t, v, "2.40.0-1")
}

// Tests for the real GetAvailableVersion implementation
func TestGetAvailableVersion_Real_Success(t *testing.T) {
	saveMocks(t)

	aptOutput := `git:
  Installed: 1:2.39.2-1.1
  Candidate: 1:2.41.0-1
  Version table:
     1:2.41.0-1 500`

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", aptOutput)
	}

	v, err := GetAvailableVersion("git")
	assertNoError(t, err)
	assertEqualStr(t, v, "1:2.41.0-1")
}

func TestGetAvailableVersion_Real_None(t *testing.T) {
	saveMocks(t)

	aptOutput := `virtual-pkg:
  Installed: (none)
  Candidate: (none)
  Version table:`

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", aptOutput)
	}

	_, err := GetAvailableVersion("virtual-pkg")
	assertError(t, err)
	assertContains(t, err.Error(), "no candidate version")
}

func TestGetAvailableVersion_Real_NotFound(t *testing.T) {
	saveMocks(t)

	// Output with no Candidate line
	aptOutput := `N: Unable to locate package badpkg`

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", aptOutput)
	}

	_, err := GetAvailableVersion("badpkg")
	assertError(t, err)
	assertContains(t, err.Error(), "candidate version not found")
}

func TestGetAvailableVersion_Real_CmdError(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("false")
	}

	_, err := GetAvailableVersion("badpkg")
	assertError(t, err)
}

func TestSyncPackageVersions(t *testing.T) {
	saveMocks(t)

	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
		"git": "2.40.0",
	})

	sm := NewStateManager("/tmp/config.toml")

	err := sm.SyncPackageVersions([]string{"vim", "git"})
	assertNoError(t, err)

	v, ok := sm.GetPackageVersion("vim")
	assertEqualBool(t, ok, true)
	assertEqualStr(t, v, "9.0.1")

	v, ok = sm.GetPackageVersion("git")
	assertEqualBool(t, ok, true)
	assertEqualStr(t, v, "2.40.0")
}

func TestSyncPackageVersions_Empty(t *testing.T) {
	sm := NewStateManager("/tmp/config.toml")
	err := sm.SyncPackageVersions(nil)
	assertNoError(t, err)
}

func TestSyncPackageVersions_ErrorSkipped(t *testing.T) {
	saveMocks(t)

	// Only vim has a version; git returns error
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})

	sm := NewStateManager("/tmp/config.toml")
	err := sm.SyncPackageVersions([]string{"vim", "git"})
	assertNoError(t, err)

	// vim should be set
	v, ok := sm.GetPackageVersion("vim")
	assertEqualBool(t, ok, true)
	assertEqualStr(t, v, "9.0.1")

	// git should not be set (error was skipped)
	_, ok = sm.GetPackageVersion("git")
	assertEqualBool(t, ok, false)
}
