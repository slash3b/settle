package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
)

func main() {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "%s is not supported\n", runtime.GOOS)
		os.Exit(1)
	}

	// Load config from default location
	// TODO: Support custom paths via --config flag
	cfg, err := loadConfig(defaultConfigPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Detect which managers are configured
	managersFound := 0

	if cfg.Debian != nil {
		managersFound++
	}

	if managersFound == 0 {
		fmt.Fprintf(os.Stderr, "No package managers configured in config.toml\n")
		os.Exit(1)
	}

	// Process each configured manager
	if cfg.Debian != nil {
		if err := handleDebian(cfg.Debian); err != nil {
			fmt.Fprintf(os.Stderr, "Error handling Debian packages: %v\n", err)
			os.Exit(1)
		}
	}

	// Future managers would be handled here:
	// if cfg.Cargo != nil {
	//     if err := handleCargo(cfg.Cargo); err != nil {
	//         os.Exit(1)
	//     }
	// }

	fmt.Println("Done!")
}

func handleDebian(debianCfg *DebianConfig) error {
	debian := NewDebianManager()

	// Collect all packages
	allPackages := make([]string, 0, len(debianCfg.Packages)+len(debianCfg.Package))
	allPackages = append(allPackages, debianCfg.Packages...)

	// Add packages with post-install hooks
	for _, pkg := range debianCfg.Package {
		allPackages = append(allPackages, pkg.Name)
	}

	if len(allPackages) == 0 {
		fmt.Println("No Debian packages configured")
		return nil
	}

	fmt.Printf("Checking %d Debian packages...\n", len(allPackages))

	// Check which packages are not installed
	missingPackages, err := debian.CheckInstalled(allPackages)
	if err != nil {
		return fmt.Errorf("error checking packages: %w", err)
	}

	installedCount := len(allPackages) - len(missingPackages)
	fmt.Printf("Already installed: %d\n", installedCount)
	fmt.Printf("Need to install: %d\n", len(missingPackages))

	// Install missing packages
	if len(missingPackages) > 0 {
		if err := debian.Install(missingPackages); err != nil {
			return fmt.Errorf("error installing packages: %w", err)
		}
	} else {
		fmt.Println("All Debian packages already installed!")
	}

	// Run post-install scripts for packages that have them
	for _, pkg := range debianCfg.Package {
		if pkg.PostInstall != "" {
			if err := debian.RunPostInstall(pkg.Name, pkg.PostInstall); err != nil {
				return fmt.Errorf("error running post-install for %s: %w", pkg.Name, err)
			}
		}
	}

	return nil
}

