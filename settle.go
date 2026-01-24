package main

import (
	"fmt"
)

// Settle is the main orchestrator for the settle application
type Settle struct {
	config  *Config
	verbose bool
	dryRun  bool
}

// NewSettle creates a new Settle orchestrator
func NewSettle(config *Config, verbose, dryRun bool) *Settle {
	return &Settle{
		config:  config,
		verbose: verbose,
		dryRun:  dryRun,
	}
}

// Apply applies the configuration by installing missing packages across all configured managers
func (s *Settle) Apply() error {
	if s.dryRun {
		fmt.Println("[dry-run mode - no changes will be made]")
		fmt.Println()
	}

	managersFound := 0

	// Handle Debian packages
	if s.config.Debian != nil {
		managersFound++
		if err := s.applyDebian(); err != nil {
			return fmt.Errorf("error handling Debian packages: %w", err)
		}
	}

	// Handle Dotfiles
	if s.config.Dotfiles != nil {
		managersFound++
		if err := s.applyDotfiles(); err != nil {
			return fmt.Errorf("error handling dotfiles: %w", err)
		}
	}

	// Future managers would be handled here:
	// if s.config.Cargo != nil {
	//     managersFound++
	//     if err := s.applyCargo(); err != nil {
	//         return err
	//     }
	// }

	if managersFound == 0 {
		return fmt.Errorf("no packages or dotfiles configured in config.toml")
	}

	fmt.Println("Done!")
	return nil
}

// applyDebian handles Debian package installation
func (s *Settle) applyDebian() error {
	manager := NewDebianManager(s.verbose)
	debianCfg := s.config.Debian

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
	missingPackages, err := manager.CheckInstalled(allPackages)
	if err != nil {
		return fmt.Errorf("error checking packages: %w", err)
	}

	// Create a set of missing packages for quick lookup
	missingSet := make(map[string]bool)
	for _, pkg := range missingPackages {
		missingSet[pkg] = true
	}

	// Collect package statuses for printing
	statuses := make([]PackageStatus, 0, len(allPackages))
	for _, pkg := range allPackages {
		status := StatusSkipped
		if missingSet[pkg] {
			status = StatusInstalled
		}
		statuses = append(statuses, PackageStatus{
			Name:   pkg,
			Status: status,
		})
	}

	installedCount := len(allPackages) - len(missingPackages)
	fmt.Printf("Already installed: %d\n", installedCount)
	fmt.Printf("Need to install: %d\n", len(missingPackages))

	// Install missing packages
	if len(missingPackages) > 0 {
		if s.dryRun {
			fmt.Println("\n[dry-run] Would install:")
			for _, pkg := range missingPackages {
				fmt.Printf("  - %s\n", pkg)
			}
			// Show post-install hooks that would run
			for _, pkg := range debianCfg.Package {
				if pkg.PostInstall != "" && missingSet[pkg.Name] {
					fmt.Printf("\n[dry-run] Would run post-install for %s\n", pkg.Name)
				}
			}
		} else {
			if err := manager.Install(missingPackages); err != nil {
				return fmt.Errorf("error installing packages: %w", err)
			}

			// Run post-install scripts ONLY for packages that were just installed
			for _, pkg := range debianCfg.Package {
				if pkg.PostInstall != "" && missingSet[pkg.Name] {
					if err := manager.RunPostInstall(pkg.Name, pkg.PostInstall); err != nil {
						return fmt.Errorf("error running post-install for %s: %w", pkg.Name, err)
					}
				}
			}
		}
	} else {
		fmt.Println("All Debian packages already installed!")
	}

	// Print results table
	PrintPackageTable(statuses)

	return nil
}

// List shows the status of all packages and dotfiles
func (s *Settle) List() error {
	// List Debian packages
	if s.config.Debian != nil {
		if err := s.listDebian(); err != nil {
			return err
		}
	}

	// List Dotfiles
	if s.config.Dotfiles != nil {
		if err := s.listDotfiles(); err != nil {
			return err
		}
	}

	return nil
}

// listDebian lists all Debian packages and their status
func (s *Settle) listDebian() error {
	manager := NewDebianManager(s.verbose)
	cfg := s.config.Debian

	// Collect all packages
	allPackages := make([]string, 0, len(cfg.Packages)+len(cfg.Package))
	allPackages = append(allPackages, cfg.Packages...)
	for _, pkg := range cfg.Package {
		allPackages = append(allPackages, pkg.Name)
	}

	if len(allPackages) == 0 {
		return nil
	}

	fmt.Println("Packages:")

	// Check which packages are installed
	missing, err := manager.CheckInstalled(allPackages)
	if err != nil {
		return err
	}

	missingSet := make(map[string]bool)
	for _, pkg := range missing {
		missingSet[pkg] = true
	}

	for _, pkg := range allPackages {
		status := "installed"
		if missingSet[pkg] {
			status = "missing"
		}
		fmt.Printf("  %s: %s\n", pkg, status)
	}

	fmt.Println()
	return nil
}

// listDotfiles lists all dotfiles and their status
func (s *Settle) listDotfiles() error {
	cfg := s.config.Dotfiles

	if len(cfg.Files) == 0 {
		return nil
	}

	manager := NewDotfilesManager(cfg.SourceDir, s.verbose)

	fmt.Println("Dotfiles:")

	for _, link := range cfg.Files {
		status, err := manager.CheckLink(link)
		statusStr := "unknown"

		if err != nil {
			statusStr = fmt.Sprintf("error: %v", err)
		} else {
			switch status {
			case LinkCorrect:
				statusStr = "linked"
			case LinkMissing:
				statusStr = "missing"
			case LinkIncorrect:
				statusStr = "wrong target"
			case LinkIsFile:
				statusStr = "file exists"
			case LinkIsDir:
				statusStr = "dir exists"
			case CopyCorrect:
				statusStr = "copied"
			case CopyOutdated:
				statusStr = "outdated"
			}
		}

		fmt.Printf("  %s: %s\n", link.Dest, statusStr)
	}

	fmt.Println()
	return nil
}

// applyDotfiles handles dotfile symlinking
func (s *Settle) applyDotfiles() error {
	cfg := s.config.Dotfiles

	if len(cfg.Files) == 0 {
		fmt.Println("No dotfiles configured")
		return nil
	}

	manager := NewDotfilesManager(cfg.SourceDir, s.verbose)

	fmt.Printf("\nChecking %d dotfile links...\n", len(cfg.Files))

	linked := 0
	skipped := 0
	var errors []string

	for _, link := range cfg.Files {
		created, err := manager.Apply(link, s.dryRun)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", link.Dest, err))
			continue
		}

		if created {
			linked++
			if s.dryRun {
				fmt.Printf("[dry-run] Would link: %s -> %s\n", link.Dest, link.Src)
			} else if s.verbose {
				fmt.Printf("Linked: %s -> %s\n", link.Dest, link.Src)
			}
		} else {
			skipped++
		}
	}

	if s.dryRun {
		fmt.Printf("\n[dry-run] Would create %d links, %d already correct\n", linked, skipped)
	} else {
		fmt.Printf("Created %d links, %d already correct\n", linked, skipped)
	}

	if len(errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range errors {
			fmt.Printf("  - %s\n", e)
		}
	}

	return nil
}
