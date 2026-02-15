package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GoInstall runs `go install <path>@<version>`.
// This is a function variable so it can be swapped in tests.
var GoInstall = func(path, version string, verbose bool) error {
	target := path + "@" + version
	cmd := exec.Command("go", "install", target)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("Running: go install %s\n", target)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go install %s failed: %w", target, err)
	}

	return nil
}

// GoBinPath returns the Go binary directory (<GOPATH>/bin).
// This is a function variable so it can be swapped in tests.
var GoBinPath = func() (string, error) {
	cmd := exec.Command("go", "env", "GOPATH")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get GOPATH: %w", err)
	}

	gopath := strings.TrimSpace(string(output))
	if gopath == "" {
		return "", fmt.Errorf("GOPATH is empty")
	}

	return filepath.Join(gopath, "bin"), nil
}

// GoPackageBinaryName returns the binary name from a Go package path.
// e.g. "github.com/golangci/golangci-lint/v2/cmd/golangci-lint" -> "golangci-lint"
func GoPackageBinaryName(path string) string {
	return filepath.Base(path)
}

// IsGoPackageInstalled checks if a binary exists in the given directory.
func IsGoPackageInstalled(binDir, binaryName string) bool {
	path := filepath.Join(binDir, binaryName)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
