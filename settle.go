package main

import (
	"fmt"
	"sort"

	"github.com/fatih/color"
)

// Settle is the main orchestrator for the settle application
type Settle struct {
	config     *Config
	configPath string
	verbose    bool
	dryRun     bool
}

// NewSettle creates a new Settle orchestrator
func NewSettle(config *Config, configPath string, verbose, dryRun bool) *Settle {
	return &Settle{
		config:     config,
		configPath: configPath,
		verbose:    verbose,
		dryRun:     dryRun,
	}
}

// Apply applies the configuration by installing missing packages across all configured managers
func (s *Settle) Apply() error {
	if s.dryRun {
		fmt.Println("[dry-run mode - no changes will be made]")
		fmt.Println()
	}

	managersFound := 0

	// Handle Linux packages
	if s.config.Linux != nil {
		managersFound++
		if err := s.applyLinux(); err != nil {
			return fmt.Errorf("error handling Linux packages: %w", err)
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

	// Sync state file (skip in dry-run mode)
	if !s.dryRun {
		if err := s.syncState(); err != nil {
			fmt.Printf("Warning: failed to sync state: %v\n", err)
		}
	}

	fmt.Println("Done!")
	return nil
}

// syncState updates the state file with current package versions
func (s *Settle) syncState() error {
	stateMgr := NewStateManager(s.configPath)

	if err := stateMgr.Load(); err != nil {
		return err
	}

	// Collect all packages
	var allPackages []string
	if s.config.Linux != nil {
		allPackages = append(allPackages, s.config.Linux.Packages...)
		for _, pkg := range s.config.Linux.Package {
			allPackages = append(allPackages, pkg.Name)
		}
	}

	if err := stateMgr.SyncPackageVersions(allPackages); err != nil {
		return err
	}

	if err := stateMgr.Save(); err != nil {
		return err
	}

	if s.verbose {
		fmt.Printf("State saved to %s\n", stateMgr.Path())
	}

	return nil
}

// applyLinux handles Linux package installation
func (s *Settle) applyLinux() error {
	distro := DetectDistro()
	if !distro.IsDebianBased() {
		return fmt.Errorf("unsupported distribution: %s (only Debian-based distros are supported)", distro)
	}

	if s.verbose {
		fmt.Printf("Detected distribution: %s\n", distro)
	}

	manager := NewDebianManager(s.verbose)
	linuxCfg := s.config.Linux

	// Load lockfile for version pinning
	lockfile := NewStateManager(s.configPath)
	if err := lockfile.Load(); err != nil && s.verbose {
		fmt.Printf("Note: no lockfile found, will install latest versions\n")
	}

	// Collect all packages
	allPackages := make([]string, 0, len(linuxCfg.Packages)+len(linuxCfg.Package))
	allPackages = append(allPackages, linuxCfg.Packages...)

	// Add packages with post-install hooks
	for _, pkg := range linuxCfg.Package {
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

	// Build versions map from lockfile
	versions := make(map[string]string)
	for _, pkg := range missingPackages {
		if version, ok := lockfile.GetPackageVersion(pkg); ok {
			versions[pkg] = version
		}
	}

	// Install missing packages
	if len(missingPackages) > 0 {
		if s.dryRun {
			fmt.Println("\n[dry-run] Would install:")
			for _, pkg := range missingPackages {
				if version, ok := versions[pkg]; ok {
					fmt.Printf("  - %s=%s (pinned)\n", pkg, version)
				} else {
					fmt.Printf("  - %s (latest)\n", pkg)
				}
			}
			// Show post-install hooks that would run
			for _, pkg := range linuxCfg.Package {
				if pkg.PostInstall != "" && missingSet[pkg.Name] {
					fmt.Printf("\n[dry-run] Would run post-install for %s\n", pkg.Name)
				}
			}
		} else {
			if err := manager.Install(missingPackages, versions); err != nil {
				return fmt.Errorf("error installing packages: %w", err)
			}

			// Run post-install scripts ONLY for packages that were just installed
			for _, pkg := range linuxCfg.Package {
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

	// Check for packages to remove (in lockfile but not in config)
	configSet := make(map[string]bool)
	for _, pkg := range allPackages {
		configSet[pkg] = true
	}

	var toRemove []string
	for _, pkg := range lockfile.GetAllPackages() {
		if !configSet[pkg] {
			toRemove = append(toRemove, pkg)
		}
	}

	if len(toRemove) > 0 {
		if s.dryRun {
			fmt.Println("\n[dry-run] Would remove (not in config):")
			for _, pkg := range toRemove {
				fmt.Printf("  - %s\n", pkg)
			}
		} else {
			fmt.Printf("\nRemoving %d packages not in config...\n", len(toRemove))
			if err := manager.Remove(toRemove); err != nil {
				return fmt.Errorf("error removing packages: %w", err)
			}
			// Remove from lockfile
			for _, pkg := range toRemove {
				lockfile.RemovePackage(pkg)
			}
			if err := lockfile.Save(); err != nil {
				fmt.Printf("Warning: failed to update lockfile: %v\n", err)
			}
		}
	}

	return nil
}

// List shows the status of all packages and dotfiles
func (s *Settle) List() error {
	// List Linux packages
	if s.config.Linux != nil {
		if err := s.listLinux(); err != nil {
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

// packageInfo holds version information for a package
type packageInfo struct {
	name         string
	installed    string
	available    string
	isMissing    bool
	notFound     bool
}

// listLinux lists all Linux packages and their status
func (s *Settle) listLinux() error {
	manager := NewDebianManager(s.verbose)
	cfg := s.config.Linux

	// Load state file for version comparison
	stateMgr := NewStateManager(s.configPath)
	if err := stateMgr.Load(); err != nil {
		if s.verbose {
			fmt.Printf("Warning: could not load state: %v\n", err)
		}
	}

	// Collect all packages
	allPackages := make([]string, 0, len(cfg.Packages)+len(cfg.Package))
	allPackages = append(allPackages, cfg.Packages...)
	for _, pkg := range cfg.Package {
		allPackages = append(allPackages, pkg.Name)
	}

	if len(allPackages) == 0 {
		return nil
	}

	// Check which packages are installed (already concurrent)
	missing, err := manager.CheckInstalled(allPackages)
	if err != nil {
		return err
	}

	missingSet := make(map[string]bool)
	for _, pkg := range missing {
		missingSet[pkg] = true
	}

	// Fetch version info concurrently
	const maxWorkers = 20
	workers := maxWorkers
	if len(allPackages) < workers {
		workers = len(allPackages)
	}

	jobs := make(chan string, len(allPackages))
	results := make(chan packageInfo, len(allPackages))

	// Start workers
	for i := 0; i < workers; i++ {
		go func() {
			for pkg := range jobs {
				info := packageInfo{name: pkg, isMissing: missingSet[pkg]}

				if info.isMissing {
					_, err := GetAvailableVersion(pkg)
					info.notFound = (err != nil)
				} else {
					info.installed, _ = GetInstalledVersion(pkg)
					info.available, _ = GetAvailableVersion(pkg)
				}

				results <- info
			}
		}()
	}

	// Send jobs
	for _, pkg := range allPackages {
		jobs <- pkg
	}
	close(jobs)

	// Collect results into a map
	infoMap := make(map[string]packageInfo)
	for i := 0; i < len(allPackages); i++ {
		info := <-results
		infoMap[info.name] = info
	}

	// Sort and print
	sort.Strings(allPackages)

	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	var items []ListItem
	for _, pkg := range allPackages {
		info := infoMap[pkg]
		var item ListItem
		item.Name = pkg

		if info.isMissing {
			if info.notFound {
				item.Status = "unknown"
				item.Color = red
			} else {
				item.Status = "missing"
			}
			items = append(items, item)
			continue
		}

		if info.installed == "" {
			item.Status = "installed (version unknown)"
			items = append(items, item)
			continue
		}

		// Build status string
		if info.available != "" && info.available != info.installed {
			item.Status = fmt.Sprintf("%s (upgrade: %s)", info.installed, info.available)
			item.Color = yellow
		} else {
			item.Status = info.installed
		}

		// Check if upgraded since last state sync
		stateVersion, hasState := stateMgr.GetPackageVersion(pkg)
		if hasState && stateVersion != info.installed {
			item.Status = fmt.Sprintf("%s (was %s)", item.Status, stateVersion)
		}

		items = append(items, item)
	}

	PrintListTable("Packages", items)
	return nil
}

// listDotfiles lists all dotfiles and their status
func (s *Settle) listDotfiles() error {
	cfg := s.config.Dotfiles

	if len(cfg.Files) == 0 {
		return nil
	}

	manager := NewDotfilesManager(cfg.SourceDir, s.verbose)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	var items []ListItem
	for _, link := range cfg.Files {
		status, err := manager.CheckLink(link)
		var item ListItem
		item.Name = link.Dest

		if err != nil {
			item.Status = fmt.Sprintf("error: %v", err)
			item.Color = red
		} else {
			switch status {
			case LinkCorrect:
				item.Status = "linked"
				item.Color = green
			case LinkMissing:
				item.Status = "missing"
				item.Color = red
			case LinkIncorrect:
				item.Status = "wrong target"
				item.Color = yellow
			case LinkIsFile:
				item.Status = "file exists"
				item.Color = yellow
			case LinkIsDir:
				item.Status = "dir exists"
				item.Color = yellow
			case CopyCorrect:
				item.Status = "copied"
				item.Color = green
			case CopyOutdated:
				item.Status = "outdated"
				item.Color = yellow
			}
		}
		items = append(items, item)
	}

	PrintListTable("Dotfiles", items)
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
