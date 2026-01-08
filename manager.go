package main

// PackageManager defines the interface that all package managers must implement
type PackageManager interface {
	// CheckInstalled checks which packages from the list are not installed
	// Returns a list of package names that need to be installed
	CheckInstalled(packages []string) ([]string, error)

	// Install installs the given list of packages
	Install(packages []string) error

	// RunPostInstall executes a post-install script for a package
	RunPostInstall(packageName, script string) error
}
