package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// execCommand is a package-level variable for exec.Command, swappable in tests.
var execCommand = exec.Command

// DebianManager handles Debian package operations
type DebianManager struct {
	verbose bool
}

// NewDebianManager creates a new Debian package manager
func NewDebianManager(verbose bool) *DebianManager {
	return &DebianManager{verbose: verbose}
}

// IsInstalled checks if a package is installed using dpkg-query
// This also handles virtual/transitional packages correctly
func (d *DebianManager) IsInstalled(packageName string) (bool, error) {
	// Use dpkg-query with format to check install status
	// This returns "install ok installed" for installed packages
	cmd := execCommand("dpkg-query", "-W", "-f=${Status}", packageName)
	output, err := cmd.Output()
	if err != nil {
		// Package not found or error
		return false, nil
	}

	// Check if status contains "install ok installed"
	status := string(output)
	return status == "install ok installed", nil
}

type packageCheckResult struct {
	name        string
	isInstalled bool
}

// CheckInstalled concurrently checks which packages are not installed
// Returns a list of package names that need to be installed
func (d *DebianManager) CheckInstalled(packages []string) ([]string, error) {
	if len(packages) == 0 {
		return nil, nil
	}

	// Use a worker pool to limit concurrent checks
	const maxWorkers = 20
	workers := maxWorkers
	if len(packages) < workers {
		workers = len(packages)
	}

	// Channels for communication
	jobs := make(chan string, len(packages))
	results := make(chan packageCheckResult, len(packages))

	// Start workers
	for i := 0; i < workers; i++ {
		go func() {
			for pkg := range jobs {
				installed, _ := d.IsInstalled(pkg)
				results <- packageCheckResult{
					name:        pkg,
					isInstalled: installed,
				}
			}
		}()
	}

	// Send jobs
	for _, pkg := range packages {
		jobs <- pkg
	}
	close(jobs)

	// Collect results
	missing := make([]string, 0)
	for i := 0; i < len(packages); i++ {
		result := <-results
		if !result.isInstalled {
			missing = append(missing, result.name)
		}
	}

	return missing, nil
}

// Install installs a list of packages using apt-get
// If versions map is provided, installs specific versions (package=version)
func (d *DebianManager) Install(packages []string, versions map[string]string) error {
	if len(packages) == 0 {
		return nil
	}

	// Build package specs with versions where available
	packageSpecs := make([]string, 0, len(packages))
	for _, pkg := range packages {
		if version, ok := versions[pkg]; ok && version != "" {
			packageSpecs = append(packageSpecs, fmt.Sprintf("%s=%s", pkg, version))
		} else {
			packageSpecs = append(packageSpecs, pkg)
		}
	}

	args := append([]string{"install", "-y"}, packageSpecs...)
	cmd := execCommand("sudo", append([]string{"apt-get"}, args...)...)

	var stderr bytes.Buffer
	if d.verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("Installing %d packages...\n", len(packages))
	} else {
		cmd.Stderr = &stderr
		fmt.Printf("Installing %d packages... ", len(packages))
	}

	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")

	err := cmd.Run()

	if !d.verbose {
		if err != nil {
			fmt.Println("failed")
			if stderr.Len() > 0 {
				fmt.Print(stderr.String())
			}
		} else {
			fmt.Println("done")
		}
	}

	return err
}

// RefreshPackageLists runs apt-get update
func (d *DebianManager) RefreshPackageLists() error {
	cmd := execCommand("sudo", "apt-get", "update", "-y")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	fmt.Println("Updating package lists...")
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
		fmt.Printf("Upgrading %d packages...\n", len(packages))
	} else {
		cmd.Stderr = &stderr
		fmt.Printf("Upgrading %d packages... ", len(packages))
	}

	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")

	err := cmd.Run()

	if !d.verbose {
		if err != nil {
			fmt.Println("failed")
			if stderr.Len() > 0 {
				fmt.Print(stderr.String())
			}
		} else {
			fmt.Println("done")
		}
	}

	return err
}

// Remove removes a list of packages using apt-get
func (d *DebianManager) Remove(packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	args := append([]string{"remove", "-y"}, packages...)
	cmd := execCommand("sudo", append([]string{"apt-get"}, args...)...)

	var stderr bytes.Buffer
	if d.verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("Removing %d packages...\n", len(packages))
	} else {
		cmd.Stderr = &stderr
		fmt.Printf("Removing %d packages... ", len(packages))
	}

	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")

	err := cmd.Run()

	if !d.verbose {
		if err != nil {
			fmt.Println("failed")
			if stderr.Len() > 0 {
				fmt.Print(stderr.String())
			}
		} else {
			fmt.Println("done")
		}
	}

	return err
}

// RunPostInstall executes a post-install script for a package
// Scripts run as the current user and should include 'sudo' where needed
func (d *DebianManager) RunPostInstall(packageName, script string) error {
	if script == "" {
		return nil
	}

	if d.verbose {
		fmt.Printf("Running post-install script for %s...\n", packageName)
	} else {
		fmt.Printf("Running post-install for %s... ", packageName)
	}

	cmd := execCommand("bash", "-c", script)

	if d.verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	cmd.Stdin = os.Stdin

	err := cmd.Run()

	if !d.verbose {
		if err != nil {
			fmt.Println("failed")
		} else {
			fmt.Println("done")
		}
	}

	return err
}
