package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DotfilesManager handles dotfile symlinking operations
type DotfilesManager struct {
	sourceDir string
	verbose   bool
}

// NewDotfilesManager creates a new dotfiles manager
func NewDotfilesManager(sourceDir string, verbose bool) *DotfilesManager {
	return &DotfilesManager{
		sourceDir: expandPath(sourceDir),
		verbose:   verbose,
	}
}

// LinkStatus represents the result of checking a symlink
type LinkStatus int

const (
	LinkMissing    LinkStatus = iota // Symlink doesn't exist
	LinkCorrect                      // Symlink exists and points to correct target
	LinkIncorrect                    // Symlink exists but points elsewhere
	LinkIsFile                       // A regular file exists at destination
	LinkIsDir                        // A directory exists at destination
)

// CheckLink checks the status of a single symlink
func (d *DotfilesManager) CheckLink(link DotfileLink) (LinkStatus, error) {
	src := filepath.Join(d.sourceDir, link.Src)
	dest := expandPath(link.Dest)

	// Check if source exists
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return LinkMissing, fmt.Errorf("source file does not exist: %s", src)
	}

	// Check destination
	info, err := os.Lstat(dest)
	if os.IsNotExist(err) {
		return LinkMissing, nil
	}
	if err != nil {
		return LinkMissing, err
	}

	// Check if it's a symlink
	if info.Mode()&os.ModeSymlink != 0 {
		// It's a symlink - check where it points
		target, err := os.Readlink(dest)
		if err != nil {
			return LinkIncorrect, err
		}
		if target == src {
			return LinkCorrect, nil
		}
		return LinkIncorrect, nil
	}

	// It's a regular file or directory
	if info.IsDir() {
		return LinkIsDir, nil
	}
	return LinkIsFile, nil
}

// CreateLink creates a symlink from dest -> src
// Returns true if a new link was created, false if already correct
func (d *DotfilesManager) CreateLink(link DotfileLink, dryRun bool) (bool, error) {
	src := filepath.Join(d.sourceDir, link.Src)
	dest := expandPath(link.Dest)

	status, err := d.CheckLink(link)
	if err != nil {
		return false, err
	}

	switch status {
	case LinkCorrect:
		return false, nil

	case LinkMissing:
		if dryRun {
			return true, nil
		}
		// Create parent directory if needed
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return false, fmt.Errorf("failed to create directory: %w", err)
		}
		// Create symlink
		if err := os.Symlink(src, dest); err != nil {
			return false, fmt.Errorf("failed to create symlink: %w", err)
		}
		return true, nil

	case LinkIncorrect:
		if dryRun {
			return true, nil
		}
		// Remove old symlink and create new one
		if err := os.Remove(dest); err != nil {
			return false, fmt.Errorf("failed to remove old symlink: %w", err)
		}
		if err := os.Symlink(src, dest); err != nil {
			return false, fmt.Errorf("failed to create symlink: %w", err)
		}
		return true, nil

	case LinkIsFile:
		if dryRun {
			return true, nil
		}
		// Backup existing file and replace with symlink
		backupPath := dest + ".backup"
		if err := os.Rename(dest, backupPath); err != nil {
			return false, fmt.Errorf("failed to backup existing file: %w", err)
		}
		if d.verbose {
			fmt.Printf("  backed up %s -> %s\n", dest, backupPath)
		}
		if err := os.Symlink(src, dest); err != nil {
			return false, fmt.Errorf("failed to create symlink: %w", err)
		}
		return true, nil

	case LinkIsDir:
		return false, fmt.Errorf("directory exists at destination: %s", dest)
	}

	return false, nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
