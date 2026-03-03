package main

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempConfig writes TOML content to a temporary file and returns its path.
func withTempConfig(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	err := os.WriteFile(path, []byte(content), 0o644) //nolint:gosec
	if err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	return path
}

// saveMocks saves the current values of all swappable function variables
// and registers a t.Cleanup to restore them.
func saveMocks(t *testing.T) {
	t.Helper()

	origExecCommand := execCommand
	origGetInstalledVersion := GetInstalledVersion
	origGetAvailableVersion := GetAvailableVersion
	origOsReleasePath := osReleasePath
	origGoInstall := GoInstall
	origGoBinPath := GoBinPath

	t.Cleanup(func() {
		execCommand = origExecCommand
		GetInstalledVersion = origGetInstalledVersion
		GetAvailableVersion = origGetAvailableVersion
		osReleasePath = origOsReleasePath
		GoInstall = origGoInstall
		GoBinPath = origGoBinPath
	})
}

// writeOsRelease writes a fake os-release file and sets osReleasePath to it.
func writeOsRelease(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "os-release")

	err := os.WriteFile(path, []byte(content), 0o644) //nolint:gosec
	if err != nil {
		t.Fatalf("failed to write os-release: %v", err)
	}

	osReleasePath = path
}
