package main

import (
	"fmt"
	"os"
	"os/exec"
)

// GitClone clones a git repository to the given destination path.
func GitClone(url, dest string, verbose bool) error {
	cmd := exec.Command("git", "clone", url, dest)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	err := cmd.Run()
	if err != nil {
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

	err := pull.Run()
	if err != nil {
		return fmt.Errorf("git pull failed in %s: %w", dir, err)
	}

	return nil
}
