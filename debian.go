package main

import (
	"fmt"
	"os"
	"os/exec"
)

// DebianManager handles Debian package operations
type DebianManager struct{}

// NewDebianManager creates a new Debian package manager
func NewDebianManager() *DebianManager {
	return &DebianManager{}
}

// IsInstalled checks if a package is installed using dpkg
func (d *DebianManager) IsInstalled(packageName string) (bool, error) {
	cmd := exec.Command("dpkg", "-s", packageName)
	err := cmd.Run()
	if err != nil {
		// dpkg returns non-zero if package is not installed
		return false, nil
	}
	return true, nil
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
func (d *DebianManager) Install(packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	args := append([]string{"install", "-y"}, packages...)
	cmd := exec.Command("sudo", append([]string{"apt-get"}, args...)...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")

	fmt.Printf("Installing %d packages...\n", len(packages))

	return cmd.Run()
}

// Update runs apt-get update
func (d *DebianManager) Update() error {
	cmd := exec.Command("sudo", "apt-get", "update", "-y")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	fmt.Println("Updating package lists...")
	return cmd.Run()
}

// RunPostInstall executes a post-install script for a package
func (d *DebianManager) RunPostInstall(packageName, script string) error {
	if script == "" {
		return nil
	}

	fmt.Printf("Running post-install script for %s...\n", packageName)

	cmd := exec.Command("bash", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
