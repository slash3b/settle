package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoPackageBinaryName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"github.com/golangci/golangci-lint/v2/cmd/golangci-lint", "golangci-lint"},
		{"golang.org/x/tools/cmd/goimports", "goimports"},
		{"github.com/user/tool", "tool"},
		{"github.com/user/repo/v3", "v3"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, GoPackageBinaryName(tt.path))
		})
	}
}

func TestIsGoPackageInstalled_Exists(t *testing.T) {
	dir := t.TempDir()

	// Create a fake binary
	binPath := filepath.Join(dir, "mytool")

	err := os.WriteFile(binPath, []byte("fake"), 0o755) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, IsGoPackageInstalled(dir, "mytool"))
}

func TestIsGoPackageInstalled_Missing(t *testing.T) {
	dir := t.TempDir()
	assert.False(t, IsGoPackageInstalled(dir, "nonexistent"))
}

func TestIsGoPackageInstalled_IsDirectory(t *testing.T) {
	dir := t.TempDir()

	// Create a directory with the binary name
	dirPath := filepath.Join(dir, "mytool")

	err := os.Mkdir(dirPath, 0o755) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}

	assert.False(t, IsGoPackageInstalled(dir, "mytool"))
}
