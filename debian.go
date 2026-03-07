package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// execCommand is a package-level variable for exec.Command, swappable in tests.
var execCommand = exec.Command

// DebianManager handles Debian package operations
type DebianManager struct {
	verbose bool
	w       io.Writer
}

// NewDebianManager creates a new Debian package manager
func NewDebianManager(verbose bool, w io.Writer) *DebianManager {
	return &DebianManager{verbose: verbose, w: w}
}

// IsInstalled checks if a package is installed using dpkg-query
func (d *DebianManager) IsInstalled(packageName string) bool {
	cmd := execCommand("dpkg-query", "-W", "-f=${Status}", packageName)

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return string(output) == "install ok installed"
}

type packageCheckResult struct {
	name        string
	isInstalled bool
}

// CheckInstalled concurrently checks which packages from the list are not installed.
// Returns a list of package names that need to be installed.
func (d *DebianManager) CheckInstalled(packages []string) []string {
	if len(packages) == 0 {
		return nil
	}

	const maxWorkers = 20

	workers := min(len(packages), maxWorkers)

	jobs := make(chan string, len(packages))
	results := make(chan packageCheckResult, len(packages))

	for range workers {
		go func() {
			for pkg := range jobs {
				results <- packageCheckResult{name: pkg, isInstalled: d.IsInstalled(pkg)}
			}
		}()
	}

	for _, pkg := range packages {
		jobs <- pkg
	}

	close(jobs)

	missing := make([]string, 0)

	for range packages {
		result := <-results
		if !result.isInstalled {
			missing = append(missing, result.name)
		}
	}

	return missing
}

// Install installs a list of packages using apt-get.
func (d *DebianManager) Install(packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	args := append([]string{"install", "-y"}, packages...)
	cmd := execCommand("sudo", append([]string{"apt-get"}, args...)...)

	var stderr bytes.Buffer

	if d.verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		_, _ = fmt.Fprintf(d.w, "Installing %d packages...\n", len(packages))
	} else {
		cmd.Stderr = &stderr

		_, _ = fmt.Fprintf(d.w, "Installing %d packages... ", len(packages))
	}

	cmd.Stdin = os.Stdin

	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")

	err := cmd.Run()

	if !d.verbose {
		if err != nil {
			_, _ = fmt.Fprintln(d.w, "failed")

			if stderr.Len() > 0 {
				_, _ = fmt.Fprint(d.w, stderr.String())
			}
		} else {
			_, _ = fmt.Fprintln(d.w, "done")
		}
	}

	return err
}

// RefreshPackageLists runs apt-get update
func (d *DebianManager) RefreshPackageLists() error {
	cmd := execCommand("sudo", "apt-get", "update", "-y")
	cmd.Stdin = os.Stdin

	_, _ = fmt.Fprintf(d.w, "Updating package lists... ")

	if d.verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		_, _ = fmt.Fprintln(d.w)
	} else {
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf

		if err := cmd.Run(); err != nil {
			_, _ = fmt.Fprintln(d.w, "failed")
			_, _ = d.w.Write(buf.Bytes())
			return err
		}

		_, _ = fmt.Fprintln(d.w, "done")

		return nil
	}

	return cmd.Run()
}

// Upgrade upgrades specific packages using apt-get install --only-upgrade
func (d *DebianManager) Upgrade(packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	args := append([]string{"install", "--only-upgrade", "-y"}, packages...)
	cmd := execCommand("sudo", append([]string{"apt-get"}, args...)...)

	var stderr bytes.Buffer

	if d.verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		_, _ = fmt.Fprintf(d.w, "Upgrading %d packages...\n", len(packages))
	} else {
		cmd.Stderr = &stderr

		_, _ = fmt.Fprintf(d.w, "Upgrading %d packages... ", len(packages))
	}

	cmd.Stdin = os.Stdin

	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")

	err := cmd.Run()

	if !d.verbose {
		if err != nil {
			_, _ = fmt.Fprintln(d.w, "failed")

			if stderr.Len() > 0 {
				_, _ = fmt.Fprint(d.w, stderr.String())
			}
		} else {
			_, _ = fmt.Fprintln(d.w, "done")
		}
	}

	return err
}

// RunPostInstall executes a post-install script for a package.
// If sudo is true, the script runs under sudo.
func (d *DebianManager) RunPostInstall(packageName, script string, sudo bool) error {
	if script == "" {
		return nil
	}

	if d.verbose {
		_, _ = fmt.Fprintf(d.w, "Running post-install script for %s...\n", packageName)
	} else {
		_, _ = fmt.Fprintf(d.w, "Running post-install for %s... ", packageName)
	}

	var cmd *exec.Cmd
	if sudo {
		cmd = execCommand("sudo", "bash", "-c", script)
	} else {
		cmd = execCommand("bash", "-c", script)
	}

	if d.verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	cmd.Stdin = os.Stdin

	err := cmd.Run()

	if !d.verbose {
		if err != nil {
			_, _ = fmt.Fprintln(d.w, "failed")
		} else {
			_, _ = fmt.Fprintln(d.w, "done")
		}
	}

	return err
}

// ValidateSudo refreshes the sudo credential, prompting for a password if needed.
// Call this before running any privileged post-install hooks to ensure sudo is available.
func ValidateSudo() error {
	cmd := execCommand("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// GetInstalledVersion queries dpkg for the installed version of a package.
// This is a function variable so it can be swapped in tests.
var GetInstalledVersion = func(name string) (string, error) {
	cmd := execCommand("dpkg-query", "-W", "-f=${Version}", name)

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// GetAvailableVersion queries apt-cache for the candidate version of a package.
// This is a function variable so it can be swapped in tests.
var GetAvailableVersion = func(name string) (string, error) {
	cmd := execCommand("apt-cache", "policy", name)

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	for line := range strings.SplitSeq(string(output), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "Candidate:"); ok {
			version := strings.TrimSpace(after)
			if version == "(none)" {
				return "", fmt.Errorf("no candidate version")
			}

			return version, nil
		}
	}

	return "", fmt.Errorf("candidate version not found")
}
