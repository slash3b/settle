package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GitClone clones a git repository to the given destination path.
func GitClone(url, dest string, verbose bool) error {
	cmd := exec.Command("git", "clone", url, dest)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	return nil
}

// GitPullRepo pulls latest changes in the given directory using --ff-only.
// If the working tree is dirty or pull fails, the error is returned as-is.
func GitPullRepo(dir string, verbose bool) error {
	pull := exec.Command("git", "-C", dir, "pull", "--ff-only")
	if verbose {
		pull.Stdout = os.Stdout
		pull.Stderr = os.Stderr
	}

	if err := pull.Run(); err != nil {
		return fmt.Errorf("git pull failed in %s: %w", dir, err)
	}

	return nil
}

// GitPull attempts to pull latest changes in the config directory.
// Returns nil if no .git directory exists (not a repo, nothing to do).
// If the working tree is dirty but there's nothing to pull, succeeds silently.
// If a pull conflicts with dirty files, git itself will return a descriptive error.
func GitPull(configPath string, verbose bool) error {
	dir := filepath.Dir(configPath)
	gitDir := filepath.Join(dir, ".git")

	// Not a git repo — skip silently
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil
	}

	// Pull latest — git handles dirty working tree conflicts naturally
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
