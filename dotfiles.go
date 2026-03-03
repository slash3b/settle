package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DotfileMode represents how a dotfile should be installed
type DotfileMode string

const (
	ModeLink DotfileMode = "link" // Default: create symlink
	ModeCopy DotfileMode = "copy" // Copy file instead of symlink
)

// DotfilesManager handles dotfile operations
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

// LinkStatus represents the result of checking a dotfile
type LinkStatus int

const (
	LinkMissing   LinkStatus = iota // Destination doesn't exist
	LinkCorrect                     // Symlink exists and points to correct target
	LinkIncorrect                   // Symlink exists but points elsewhere
	LinkIsFile                      // A regular file exists at destination (for link mode)
	LinkIsDir                       // A directory exists at destination
	CopyCorrect                     // Copy exists and matches source
	CopyOutdated                    // Copy exists but differs from source
)

// CheckLink checks the status of a single dotfile (symlink or copy)
func (d *DotfilesManager) CheckLink(link Dotfile) (LinkStatus, error) {
	src := filepath.Join(d.sourceDir, link.Src)
	dest := expandPath(link.Dest)

	// Check if source exists
	_, err := os.Stat(src)
	if os.IsNotExist(err) {
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

	// Handle copy mode
	if DotfileMode(link.Mode) == ModeCopy {
		if info.IsDir() {
			return LinkIsDir, nil
		}
		// Check if it's a regular file and compare contents
		if info.Mode().IsRegular() {
			equal, err := filesEqual(src, dest)
			if err != nil {
				return CopyOutdated, err
			}

			if equal {
				return CopyCorrect, nil
			}

			return CopyOutdated, nil
		}

		return CopyOutdated, nil
	}

	// Default: link mode
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

// Apply creates a symlink or copies the file depending on mode
// Returns true if changes were made, false if already correct
func (d *DotfilesManager) Apply(link Dotfile, dryRun bool) (bool, error) {
	src := filepath.Join(d.sourceDir, link.Src)
	dest := expandPath(link.Dest)

	status, err := d.CheckLink(link)
	if err != nil {
		return false, err
	}

	var changed bool
	if DotfileMode(link.Mode) == ModeCopy {
		changed, err = d.applyCopy(src, dest, status, dryRun, link.Sudo)
	} else {
		changed, err = d.applyLink(src, dest, status, dryRun, link.Sudo)
	}

	if err != nil {
		return false, err
	}

	if link.Executable && !dryRun {
		if link.Sudo {
			err = runSudo("chmod", "755", dest)
			if err != nil {
				return false, fmt.Errorf("failed to set executable: %w", err)
			}
		} else {
			// os.Chmod follows symlinks, so this correctly chmods the source
			// in link mode and the copied file in copy mode.
			err = os.Chmod(dest, 0o755) //nolint:gosec // executable bit is intentional
			if err != nil {
				return false, fmt.Errorf("failed to set executable: %w", err)
			}
		}
	}

	return changed, nil
}

// runSudo executes a command with sudo, returning a descriptive error on failure.
func runSudo(args ...string) error {
	var stderr bytes.Buffer

	cmd := execCommand("sudo", args...)
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}

		return err
	}

	return nil
}

// applyLink handles symlink mode
func (d *DotfilesManager) applyLink(src, dest string, status LinkStatus, dryRun, sudo bool) (bool, error) {
	switch status {
	case LinkCorrect:
		return false, nil

	case LinkMissing:
		if dryRun {
			return true, nil
		}

		if sudo {
			err := runSudo("mkdir", "-p", filepath.Dir(dest))
			if err != nil {
				return false, fmt.Errorf("failed to create directory: %w", err)
			}

			err = runSudo("ln", "-sf", src, dest)
			if err != nil {
				return false, fmt.Errorf("failed to create symlink: %w", err)
			}
		} else {
			err := os.MkdirAll(filepath.Dir(dest), 0o755) //nolint:gosec // 0755 is standard for directories
			if err != nil {
				return false, fmt.Errorf("failed to create directory: %w", err)
			}

			err = os.Symlink(src, dest)
			if err != nil {
				return false, fmt.Errorf("failed to create symlink: %w", err)
			}
		}

		return true, nil

	case LinkIncorrect:
		if dryRun {
			return true, nil
		}

		if sudo {
			// ln -sf atomically replaces the existing symlink
			err := runSudo("ln", "-sf", src, dest)
			if err != nil {
				return false, fmt.Errorf("failed to create symlink: %w", err)
			}
		} else {
			err := os.Remove(dest)
			if err != nil {
				return false, fmt.Errorf("failed to remove old symlink: %w", err)
			}

			err = os.Symlink(src, dest)
			if err != nil {
				return false, fmt.Errorf("failed to create symlink: %w", err)
			}
		}

		return true, nil

	case LinkIsFile:
		if dryRun {
			return true, nil
		}

		backupPath := dest + ".backup"
		if sudo {
			err := runSudo("mv", dest, backupPath)
			if err != nil {
				return false, fmt.Errorf("failed to backup existing file: %w", err)
			}

			if d.verbose {
				fmt.Printf("  backed up %s -> %s\n", dest, backupPath)
			}

			err = runSudo("ln", "-sf", src, dest)
			if err != nil {
				return false, fmt.Errorf("failed to create symlink: %w", err)
			}
		} else {
			err := os.Rename(dest, backupPath)
			if err != nil {
				return false, fmt.Errorf("failed to backup existing file: %w", err)
			}

			if d.verbose {
				fmt.Printf("  backed up %s -> %s\n", dest, backupPath)
			}

			err = os.Symlink(src, dest)
			if err != nil {
				return false, fmt.Errorf("failed to create symlink: %w", err)
			}
		}

		return true, nil

	case LinkIsDir:
		return false, fmt.Errorf("directory exists at destination: %s", dest)

	case CopyCorrect, CopyOutdated:
		// not reachable in link mode
	}

	return false, nil
}

// applyCopy handles copy mode
func (d *DotfilesManager) applyCopy(src, dest string, status LinkStatus, dryRun, sudo bool) (bool, error) {
	switch status {
	case CopyCorrect:
		return false, nil

	case LinkMissing:
		if dryRun {
			return true, nil
		}

		if sudo {
			err := runSudo("mkdir", "-p", filepath.Dir(dest))
			if err != nil {
				return false, fmt.Errorf("failed to create directory: %w", err)
			}

			err = runSudo("cp", src, dest)
			if err != nil {
				return false, fmt.Errorf("failed to copy file: %w", err)
			}
		} else {
			err := os.MkdirAll(filepath.Dir(dest), 0o755) //nolint:gosec // 0755 is standard for directories
			if err != nil {
				return false, fmt.Errorf("failed to create directory: %w", err)
			}

			err = copyFile(src, dest)
			if err != nil {
				return false, fmt.Errorf("failed to copy file: %w", err)
			}
		}

		return true, nil

	case CopyOutdated:
		if dryRun {
			return true, nil
		}

		if sudo {
			err := runSudo("cp", src, dest)
			if err != nil {
				return false, fmt.Errorf("failed to update copy: %w", err)
			}
		} else {
			err := copyFile(src, dest)
			if err != nil {
				return false, fmt.Errorf("failed to update copy: %w", err)
			}
		}

		return true, nil

	case LinkIsDir:
		return false, fmt.Errorf("directory exists at destination: %s", dest)

	case LinkCorrect, LinkIncorrect, LinkIsFile:
		// not reachable in copy mode
	}

	return false, nil
}

// CheckDir checks the status of a directory symlink.
func (d *DotfilesManager) CheckDir(dir DotfileDir) (LinkStatus, error) {
	return d.CheckLink(Dotfile{Src: dir.Src, Dest: dir.Dest})
}

// ApplyDir creates a symlink for a directory entry.
// If a real directory already exists at the destination, it is backed up before symlinking.
func (d *DotfilesManager) ApplyDir(dir DotfileDir, dryRun bool) (bool, error) {
	src := filepath.Join(d.sourceDir, dir.Src)
	dest := expandPath(dir.Dest)

	status, err := d.CheckDir(dir)
	if err != nil {
		return false, err
	}

	if status == LinkIsDir {
		if dryRun {
			return true, nil
		}

		backupPath := dest + ".backup"
		if dir.Sudo {
			err = runSudo("mv", dest, backupPath)
			if err != nil {
				return false, fmt.Errorf("failed to backup existing directory: %w", err)
			}

			err = runSudo("ln", "-sf", src, dest)
			if err != nil {
				return false, fmt.Errorf("failed to create symlink: %w", err)
			}
		} else {
			err = os.Rename(dest, backupPath)
			if err != nil {
				return false, fmt.Errorf("failed to backup existing directory: %w", err)
			}

			err = os.Symlink(src, dest)
			if err != nil {
				return false, fmt.Errorf("failed to create symlink: %w", err)
			}
		}

		if d.verbose {
			fmt.Printf("  backed up %s -> %s\n", dest, backupPath)
		}

		return true, nil
	}

	return d.Apply(Dotfile{Src: dir.Src, Dest: dir.Dest, Sudo: dir.Sudo}, dryRun)
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

// filesEqual compares two files and returns true if they have identical content
func filesEqual(path1, path2 string) (bool, error) {
	f1, err := os.ReadFile(path1) //nolint:gosec // paths are internal dotfile sources
	if err != nil {
		return false, err
	}

	f2, err := os.ReadFile(path2) //nolint:gosec // paths are internal dotfile sources
	if err != nil {
		return false, err
	}

	return bytes.Equal(f1, f2), nil
}

// copyFile copies a file from src to dest
func copyFile(src, dest string) error {
	srcFile, err := os.Open(src) //nolint:gosec // paths are internal dotfile sources
	if err != nil {
		return err
	}

	defer srcFile.Close() //nolint:errcheck

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	destFile, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC, srcInfo.Mode()) //nolint:gosec // paths are internal dotfile sources
	if err != nil {
		return err
	}

	defer destFile.Close() //nolint:errcheck

	_, err = io.Copy(destFile, srcFile)

	return err
}
