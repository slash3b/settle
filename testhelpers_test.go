package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// withTempConfig writes TOML content to a temporary file and returns its path.
func withTempConfig(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	return path
}

// cmdCall records a command invocation for test assertions.
type cmdCall struct {
	Name string
	Args []string
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
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write os-release: %v", err)
	}
	osReleasePath = path
}

// mockInstalledVersion installs a mock GetInstalledVersion that returns
// versions from the given map. Missing keys return an error.
func mockInstalledVersion(versions map[string]string) {
	GetInstalledVersion = func(name string) (string, error) {
		v, ok := versions[name]
		if !ok {
			return "", fmt.Errorf("package %s not found", name)
		}
		return v, nil
	}
}

// mockAvailableVersion installs a mock GetAvailableVersion that returns
// versions from the given map. Missing keys return an error.
func mockAvailableVersion(versions map[string]string) {
	GetAvailableVersion = func(name string) (string, error) {
		v, ok := versions[name]
		if !ok {
			return "", fmt.Errorf("no candidate version")
		}
		return v, nil
	}
}
