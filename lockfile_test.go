package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStateManager(t *testing.T) {
	sm := NewStateManager("/home/user/dotfiles/config.toml")
	assert.Equal(t, "/home/user/dotfiles/lockfile.json", sm.Path())
}

func TestStateManager_LoadNoFile(t *testing.T) {
	dir := t.TempDir()
	sm := NewStateManager(filepath.Join(dir, "config.toml"))

	err := sm.Load()
	require.NoError(t, err)

	// Should have empty packages
	pkgs := sm.GetAllPackages()
	assert.Equal(t, 0, len(pkgs))
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
	require.NoError(t, err)

	v, ok := sm.GetPackageVersion("vim")
	assert.True(t, ok)
	assert.Equal(t, "9.0.1", v)

	v, ok = sm.GetPackageVersion("git")
	assert.True(t, ok)
	assert.Equal(t, "2.40.0", v)
}

func TestStateManager_LoadCorrupt(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	os.WriteFile(filepath.Join(dir, "lockfile.json"), []byte("not json{{{"), 0o644)

	sm := NewStateManager(configPath)
	err := sm.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse state file")
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
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read state file")
}

func TestStateManager_Save(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	sm := NewStateManager(configPath)
	sm.Load()

	sm.SetPackageVersion("vim", "9.0.1", "apt")

	err := sm.Save()
	require.NoError(t, err)

	// Read back and verify
	data, err := os.ReadFile(filepath.Join(dir, "lockfile.json"))
	require.NoError(t, err)

	var state State
	json.Unmarshal(data, &state)
	assert.Equal(t, "9.0.1", state.Packages["vim"].Version)
}

func TestStateManager_SaveWriteError(t *testing.T) {
	// Use a directory that doesn't exist
	sm := NewStateManager("/nonexistent/dir/config.toml")
	sm.SetPackageVersion("vim", "1.0", "apt")

	err := sm.Save()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write state file")
}

func TestStateManager_GetSetVersion(t *testing.T) {
	sm := NewStateManager("/tmp/config.toml")

	// Get non-existent
	_, ok := sm.GetPackageVersion("vim")
	assert.False(t, ok)

	// Set and get
	sm.SetPackageVersion("vim", "9.0.1", "apt")
	v, ok := sm.GetPackageVersion("vim")
	assert.True(t, ok)
	assert.Equal(t, "9.0.1", v)
}

func TestStateManager_SetVersionPreservesInstalledAt(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	sm := NewStateManager(configPath)

	sm.SetPackageVersion("vim", "9.0.0", "apt")
	installedAt := sm.state.Packages["vim"].InstalledAt

	// Update version — InstalledAt should be preserved
	sm.SetPackageVersion("vim", "9.0.1", "apt")
	assert.Equal(t, installedAt, sm.state.Packages["vim"].InstalledAt)
	assert.Equal(t, "9.0.1", sm.state.Packages["vim"].Version)
}

func TestStateManager_RemovePackage(t *testing.T) {
	sm := NewStateManager("/tmp/config.toml")
	sm.SetPackageVersion("vim", "9.0.1", "apt")

	sm.RemovePackage("vim")

	_, ok := sm.GetPackageVersion("vim")
	assert.False(t, ok)
}

func TestStateManager_RemoveNonexistent(t *testing.T) {
	sm := NewStateManager("/tmp/config.toml")
	// Should not panic
	sm.RemovePackage("nonexistent")
}

func TestStateManager_GetAllPackages(t *testing.T) {
	sm := NewStateManager("/tmp/config.toml")
	sm.SetPackageVersion("vim", "9.0.1", "apt")
	sm.SetPackageVersion("git", "2.40.0", "apt")

	pkgs := sm.GetAllPackages()
	assert.Equal(t, 2, len(pkgs))
	assert.ElementsMatch(t, []string{"vim", "git"}, pkgs)
}

func TestStateManager_GetPackagesByManager(t *testing.T) {
	sm := NewStateManager("/tmp/config.toml")
	sm.SetPackageVersion("vim", "9.0.1", "apt")
	sm.SetPackageVersion("git", "2.40.0", "apt")
	sm.SetPackageVersion("golangci-lint", "v2.9.0", "go")

	aptPkgs := sm.GetPackagesByManager("apt")
	assert.Equal(t, 2, len(aptPkgs))
	assert.ElementsMatch(t, []string{"vim", "git"}, aptPkgs)

	goPkgs := sm.GetPackagesByManager("go")
	assert.Equal(t, 1, len(goPkgs))
	assert.Equal(t, "golangci-lint", goPkgs[0])

	cargoPkgs := sm.GetPackagesByManager("cargo")
	assert.Equal(t, 0, len(cargoPkgs))
}

func TestStateManager_BackfillsEmptyManager(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	// Write a lockfile without manager field (old format)
	os.WriteFile(filepath.Join(dir, "lockfile.json"), []byte(`{
		"packages": {
			"vim": {"version": "9.0.1", "installed_at": "2026-01-01T00:00:00Z"},
			"git": {"version": "2.40.0", "installed_at": "2026-01-01T00:00:00Z"}
		}
	}`), 0o644)

	sm := NewStateManager(configPath)
	require.NoError(t, sm.Load())

	// Should be backfilled to "apt"
	aptPkgs := sm.GetPackagesByManager("apt")
	assert.Equal(t, 2, len(aptPkgs))
	assert.ElementsMatch(t, []string{"vim", "git"}, aptPkgs)
}

func TestStateManager_Path(t *testing.T) {
	sm := NewStateManager("/path/to/config.toml")
	assert.Equal(t, "/path/to/lockfile.json", sm.Path())
}

// Tests for the real GetInstalledVersion implementation (uses execCommand)
func TestGetInstalledVersion_Real_Success(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "2.40.0-1")
	}

	v, err := GetInstalledVersion("git")
	require.NoError(t, err)
	assert.Equal(t, "2.40.0-1", v)
}

func TestGetInstalledVersion_Real_Error(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("false")
	}

	_, err := GetInstalledVersion("nonexistent")
	require.Error(t, err)
}

func TestGetInstalledVersion_Real_Whitespace(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "  2.40.0-1  ")
	}

	v, err := GetInstalledVersion("git")
	require.NoError(t, err)
	assert.Equal(t, "2.40.0-1", v)
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
	require.NoError(t, err)
	assert.Equal(t, "1:2.41.0-1", v)
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
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no candidate version")
}

func TestGetAvailableVersion_Real_NotFound(t *testing.T) {
	saveMocks(t)

	// Output with no Candidate line
	aptOutput := `N: Unable to locate package badpkg`

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", aptOutput)
	}

	_, err := GetAvailableVersion("badpkg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "candidate version not found")
}

func TestGetAvailableVersion_Real_CmdError(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("false")
	}

	_, err := GetAvailableVersion("badpkg")
	require.Error(t, err)
}

func TestSyncPackageVersions(t *testing.T) {
	saveMocks(t)

	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
		"git": "2.40.0",
	})

	sm := NewStateManager("/tmp/config.toml")

	err := sm.SyncPackageVersions([]string{"vim", "git"})
	require.NoError(t, err)

	v, ok := sm.GetPackageVersion("vim")
	assert.True(t, ok)
	assert.Equal(t, "9.0.1", v)

	v, ok = sm.GetPackageVersion("git")
	assert.True(t, ok)
	assert.Equal(t, "2.40.0", v)
}

func TestSyncPackageVersions_Empty(t *testing.T) {
	sm := NewStateManager("/tmp/config.toml")
	err := sm.SyncPackageVersions(nil)
	require.NoError(t, err)
}

func TestSyncPackageVersions_ErrorSkipped(t *testing.T) {
	saveMocks(t)

	// Only vim has a version; git returns error
	mockInstalledVersion(map[string]string{
		"vim": "9.0.1",
	})

	sm := NewStateManager("/tmp/config.toml")
	err := sm.SyncPackageVersions([]string{"vim", "git"})
	require.NoError(t, err)

	// vim should be set
	v, ok := sm.GetPackageVersion("vim")
	assert.True(t, ok)
	assert.Equal(t, "9.0.1", v)

	// git should not be set (error was skipped)
	_, ok = sm.GetPackageVersion("git")
	assert.False(t, ok)
}

func TestCompareVersions(t *testing.T) {
	// Equal versions
	assert.Equal(t, 0, CompareVersions("9.0.1", "9.0.1"))

	// Less than
	assert.Equal(t, -1, CompareVersions("9.0.1", "9.0.2"))

	// Greater than
	assert.Equal(t, 1, CompareVersions("9.0.2", "9.0.1"))

	// Real Debian version strings
	assert.Equal(t, -1, CompareVersions("1.3.2-0.1", "1.4.6-01"))
	assert.Equal(t, -1, CompareVersions("0.8.1-2+b9", "0.9.1"))
	assert.Equal(t, -1, CompareVersions("6.12.63-1", "6.12.69-1"))

	// Epoch versions
	assert.Equal(t, 1, CompareVersions("1:2.0", "2.0"))
}

func TestSyncPackageVersions_NoDowngrade(t *testing.T) {
	saveMocks(t)

	// Lockfile has higher version (from another machine)
	// Installed version is lower
	mockInstalledVersion(map[string]string{
		"btop": "1.3.2-0.1",
		"duf":  "0.8.1-2+b9",
	})

	sm := NewStateManager("/tmp/config.toml")
	// Pre-populate with higher versions (as if pulled from another machine)
	sm.SetPackageVersion("btop", "1.4.6-01", "apt")
	sm.SetPackageVersion("duf", "0.9.1", "apt")

	err := sm.SyncPackageVersions([]string{"btop", "duf"})
	require.NoError(t, err)

	// Versions should NOT be downgraded
	v, _ := sm.GetPackageVersion("btop")
	assert.Equal(t, "1.4.6-01", v)

	v, _ = sm.GetPackageVersion("duf")
	assert.Equal(t, "0.9.1", v)
}

func TestSyncPackageVersions_UpgradesHigher(t *testing.T) {
	saveMocks(t)

	// Installed version is higher than lockfile (upgraded externally)
	mockInstalledVersion(map[string]string{
		"vim": "9.0.2",
	})

	sm := NewStateManager("/tmp/config.toml")
	sm.SetPackageVersion("vim", "9.0.1", "apt")

	err := sm.SyncPackageVersions([]string{"vim"})
	require.NoError(t, err)

	// Should be updated to higher version
	v, _ := sm.GetPackageVersion("vim")
	assert.Equal(t, "9.0.2", v)
}

func TestSyncPackageVersions_NewPackage(t *testing.T) {
	saveMocks(t)

	mockInstalledVersion(map[string]string{
		"curl": "7.88.1",
	})

	sm := NewStateManager("/tmp/config.toml")

	err := sm.SyncPackageVersions([]string{"curl"})
	require.NoError(t, err)

	// New package should be added
	v, ok := sm.GetPackageVersion("curl")
	assert.True(t, ok)
	assert.Equal(t, "7.88.1", v)
}
