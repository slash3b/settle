package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitPull attempts to pull latest changes in the config directory.
// Returns nil if no .git directory exists (not a repo, nothing to do).
// Returns an error if the working tree is dirty or pull fails.
func GitPull(configPath string, verbose bool) error {
	dir := filepath.Dir(configPath)
	gitDir := filepath.Join(dir, ".git")

	// Not a git repo — skip silently
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil
	}

	// Check for uncommitted changes
	status := exec.Command("git", "-C", dir, "status", "--porcelain")
	output, err := status.Output()
	if err != nil {
		return fmt.Errorf("failed to check git status: %w", err)
	}
	if len(strings.TrimSpace(string(output))) > 0 {
		return fmt.Errorf("config repo has uncommitted changes — commit or stash before running settle")
	}

	// Pull latest
	pull := exec.Command("git", "-C", dir, "pull", "--ff-only")
	if verbose {
		pull.Stdout = os.Stdout
		pull.Stderr = os.Stderr
	}

	if err := pull.Run(); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	return nil
}
