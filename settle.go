package main

import (
	"fmt"
	"os"
	"path/filepath"
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

// Apply applies the configuration: installs missing packages, links dotfiles, clones repos.
func (s *Settle) Apply() error {
	if s.dryRun {
		fmt.Println("[dry-run mode - no changes will be made]")
		fmt.Println()
	}

	// Pull latest config from git if applicable (best-effort)
	if !s.dryRun {
		if err := GitPull(s.configPath, s.verbose); err != nil {
			fmt.Printf("Warning: %v\n", err)
		}
	}

	managersFound := 0

	if s.config.Apt != nil {
		managersFound++
		if err := s.applyApt(); err != nil {
			return fmt.Errorf("error handling apt packages: %w", err)
		}
	}

	if s.config.Dotfiles != nil {
		managersFound++
		if err := s.applyDotfiles(); err != nil {
			return fmt.Errorf("error handling dotfiles: %w", err)
		}
	}

	if len(s.config.Git) > 0 {
		managersFound++
		if err := s.applyGit(); err != nil {
			return fmt.Errorf("error handling git repos: %w", err)
		}
	}

	if len(s.config.Go) > 0 {
		managersFound++
		if err := s.applyGo(); err != nil {
			return fmt.Errorf("error handling go packages: %w", err)
		}
	}

	if managersFound == 0 {
		return fmt.Errorf("no packages, dotfiles, git repos, or go packages configured in config.toml")
	}

	fmt.Println("Done!")
	return nil
}

// Update upgrades all managed packages to their latest versions.
func (s *Settle) Update() error {
	hasApt := s.config.Apt != nil
	hasGit := len(s.config.Git) > 0
	hasGo := len(s.config.Go) > 0

	if hasApt {
		distro := DetectDistro()
		if !distro.IsDebianBased() {
			return fmt.Errorf("unsupported distribution: %s", distro)
		}

		manager := NewDebianManager(s.verbose)

		var allPackages []string
		allPackages = append(allPackages, s.config.Apt.Packages...)
		for _, pkg := range s.config.Apt.PostHooks {
			allPackages = append(allPackages, pkg.Name)
		}

		if len(allPackages) > 0 {
			fmt.Printf("Updating %d managed packages...\n", len(allPackages))

			if s.dryRun {
				fmt.Println("[dry-run] Would run: apt-get update")
				fmt.Printf("[dry-run] Would upgrade: %v\n", allPackages)
			} else {
				if err := manager.RefreshPackageLists(); err != nil {
					return fmt.Errorf("failed to update package lists: %w", err)
				}
				if err := manager.Upgrade(allPackages); err != nil {
					return fmt.Errorf("failed to upgrade packages: %w", err)
				}
			}
		}
	}

	if hasGit {
		if err := s.updateGit(); err != nil {
			return fmt.Errorf("error updating git repos: %w", err)
		}
	}

	if hasGo {
		if err := s.updateGo(); err != nil {
			return fmt.Errorf("error updating go packages: %w", err)
		}
	}

	if !hasApt && !hasGit && !hasGo {
		fmt.Println("No packages or git repos configured")
		return nil
	}

	fmt.Println("Done!")
	return nil
}

// List shows the status of all packages and dotfiles.
func (s *Settle) List() error {
	if s.config.Apt != nil {
		if err := s.listApt(); err != nil {
			return err
		}
	}

	if s.config.Dotfiles != nil {
		if err := s.listDotfiles(); err != nil {
			return err
		}
	}

	if len(s.config.Git) > 0 {
		s.listGit()
	}

	if len(s.config.Go) > 0 {
		s.listGo()
	}

	return nil
}

// applyApt handles apt package installation.
func (s *Settle) applyApt() error {
	distro := DetectDistro()
	if !distro.IsDebianBased() {
		return fmt.Errorf("unsupported distribution: %s (only Debian-based distros are supported)", distro)
	}

	if s.verbose {
		fmt.Printf("Detected distribution: %s\n", distro)
	}

	manager := NewDebianManager(s.verbose)
	aptCfg := s.config.Apt

	allPackages := make([]string, 0, len(aptCfg.Packages)+len(aptCfg.PostHooks))
	allPackages = append(allPackages, aptCfg.Packages...)
	for _, pkg := range aptCfg.PostHooks {
		allPackages = append(allPackages, pkg.Name)
	}

	if len(allPackages) == 0 {
		fmt.Println("No apt packages configured")
		return nil
	}

	fmt.Printf("Checking %d apt packages...\n", len(allPackages))

	missingPackages, err := manager.CheckInstalled(allPackages)
	if err != nil {
		return fmt.Errorf("error checking packages: %w", err)
	}

	missingSet := make(map[string]bool)
	for _, pkg := range missingPackages {
		missingSet[pkg] = true
	}

	installedCount := len(allPackages) - len(missingPackages)
	fmt.Printf("Already installed: %d\n", installedCount)
	fmt.Printf("Need to install: %d\n", len(missingPackages))

	// Filter out unknown packages
	var installable []string
	var unknown []string
	for _, pkg := range missingPackages {
		if _, err := GetAvailableVersion(pkg); err != nil {
			unknown = append(unknown, pkg)
			continue
		}
		installable = append(installable, pkg)
	}

	if len(unknown) > 0 {
		fmt.Printf("\nSkipping %d unknown packages:\n", len(unknown))
		for _, pkg := range unknown {
			fmt.Printf("  - %s\n", pkg)
		}
	}

	if len(installable) > 0 {
		if s.dryRun {
			fmt.Println("\n[dry-run] Would install:")
			for _, pkg := range installable {
				fmt.Printf("  - %s\n", pkg)
			}
			for _, pkg := range aptCfg.PostHooks {
				if pkg.PostInstall != "" && missingSet[pkg.Name] {
					fmt.Printf("\n[dry-run] Would run post-install for %s\n", pkg.Name)
				}
			}
		} else {
			if err := manager.RefreshPackageLists(); err != nil {
				return fmt.Errorf("failed to update package lists: %w", err)
			}
			if err := manager.Install(installable); err != nil {
				return fmt.Errorf("error installing packages: %w", err)
			}
			var hooksToRun []PostHook
			for _, pkg := range aptCfg.PostHooks {
				if pkg.PostInstall != "" && missingSet[pkg.Name] {
					hooksToRun = append(hooksToRun, pkg)
				}
			}
			if len(hooksToRun) > 0 {
				if err := ValidateSudo(); err != nil {
					return fmt.Errorf("sudo authentication failed: %w", err)
				}
				for _, pkg := range hooksToRun {
					if err := manager.RunPostInstall(pkg.Name, pkg.PostInstall, pkg.Sudo); err != nil {
						return fmt.Errorf("error running post-install for %s: %w", pkg.Name, err)
					}
				}
			}
		}
	} else if len(missingPackages) == 0 {
		fmt.Println("All packages already installed!")
	}

	// Print results table
	statuses := make([]PackageStatus, 0, len(allPackages))
	unknownSet := make(map[string]bool)
	for _, pkg := range unknown {
		unknownSet[pkg] = true
	}
	for _, pkg := range allPackages {
		if unknownSet[pkg] {
			continue
		}
		status := StatusSkipped
		if missingSet[pkg] {
			status = StatusInstalled
		}
		statuses = append(statuses, PackageStatus{Name: pkg, Status: status})
	}
	PrintPackageTable(statuses)

	return nil
}

// applyDotfiles handles dotfile symlinking.
func (s *Settle) applyDotfiles() error {
	cfg := s.config.Dotfiles

	if len(cfg.Files) == 0 && len(cfg.Dirs) == 0 {
		fmt.Println("No dotfiles configured")
		return nil
	}

	manager := NewDotfilesManager(cfg.SourceDir, s.verbose)

	fmt.Printf("\nChecking %d dotfile links...\n", len(cfg.Files)+len(cfg.Dirs))

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

	for _, dir := range cfg.Dirs {
		created, err := manager.ApplyDir(dir, s.dryRun)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", dir.Dest, err))
			continue
		}

		if created {
			linked++
			if s.dryRun {
				fmt.Printf("[dry-run] Would link dir: %s -> %s\n", dir.Dest, dir.Src)
			} else if s.verbose {
				fmt.Printf("Linked dir: %s -> %s\n", dir.Dest, dir.Src)
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

// applyGit clones missing git repos.
func (s *Settle) applyGit() error {
	fmt.Printf("\nChecking %d git repos...\n", len(s.config.Git))

	var statuses []PackageStatus
	for _, repo := range s.config.Git {
		dest := expandPath(repo.Dest)
		gitDir := filepath.Join(dest, ".git")

		info, err := os.Stat(dest)
		if err == nil {
			if info.IsDir() {
				if _, err := os.Stat(gitDir); err == nil {
					statuses = append(statuses, PackageStatus{Name: repo.Dest, Status: StatusSkipped})
					continue
				}
				return fmt.Errorf("destination %s exists but is not a git repository", dest)
			}
			return fmt.Errorf("destination %s exists but is not a directory", dest)
		}

		if s.dryRun {
			fmt.Printf("[dry-run] Would clone: %s -> %s\n", repo.URL, repo.Dest)
			statuses = append(statuses, PackageStatus{Name: repo.Dest, Status: StatusInstalled})
			continue
		}

		if err := GitClone(repo.URL, dest, s.verbose); err != nil {
			return fmt.Errorf("failed to clone %s: %w", repo.URL, err)
		}
		statuses = append(statuses, PackageStatus{Name: repo.Dest, Status: StatusInstalled})
	}

	PrintPackageTable(statuses)
	return nil
}

// applyGo installs missing Go packages via `go install`.
func (s *Settle) applyGo() error {
	binDir, err := GoBinPath()
	if err != nil {
		return fmt.Errorf("cannot determine Go bin path: %w", err)
	}

	fmt.Printf("\nChecking %d go packages...\n", len(s.config.Go))

	var statuses []PackageStatus
	for _, pkg := range s.config.Go {
		binName := GoPackageBinaryName(pkg.Path)

		if IsGoPackageInstalled(binDir, binName) {
			statuses = append(statuses, PackageStatus{Name: binName, Status: StatusSkipped})
			continue
		}

		if s.dryRun {
			fmt.Printf("[dry-run] Would install: go install %s@%s\n", pkg.Path, pkg.Version)
			statuses = append(statuses, PackageStatus{Name: binName, Status: StatusInstalled})
			continue
		}

		if err := GoInstall(pkg.Path, pkg.Version, s.verbose); err != nil {
			return fmt.Errorf("failed to install %s: %w", pkg.Path, err)
		}
		statuses = append(statuses, PackageStatus{Name: binName, Status: StatusInstalled})
	}

	PrintPackageTable(statuses)
	return nil
}

// updateGit pulls latest changes for all cloned git repos.
func (s *Settle) updateGit() error {
	fmt.Printf("\nUpdating %d git repos...\n", len(s.config.Git))

	for _, repo := range s.config.Git {
		dest := expandPath(repo.Dest)
		gitDir := filepath.Join(dest, ".git")

		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			fmt.Printf("Warning: %s not cloned, run settle apply first\n", repo.Dest)
			continue
		}

		if s.dryRun {
			fmt.Printf("[dry-run] Would pull: %s\n", repo.Dest)
			continue
		}

		if err := GitPullRepo(dest, s.verbose); err != nil {
			return fmt.Errorf("failed to pull %s: %w", repo.Dest, err)
		}
	}

	return nil
}

// updateGo reinstalls all Go packages at their configured versions.
func (s *Settle) updateGo() error {
	fmt.Printf("\nUpdating %d go packages...\n", len(s.config.Go))

	for _, pkg := range s.config.Go {
		if s.dryRun {
			fmt.Printf("[dry-run] Would install: go install %s@%s\n", pkg.Path, pkg.Version)
			continue
		}

		if err := GoInstall(pkg.Path, pkg.Version, s.verbose); err != nil {
			return fmt.Errorf("failed to update %s: %w", pkg.Path, err)
		}
	}

	return nil
}

// packageInfo holds version information for a package
type packageInfo struct {
	name      string
	installed string
	available string
	isMissing bool
	notFound  bool
}

// listApt lists all apt packages and their status.
func (s *Settle) listApt() error {
	manager := NewDebianManager(s.verbose)
	cfg := s.config.Apt

	allPackages := make([]string, 0, len(cfg.Packages)+len(cfg.PostHooks))
	allPackages = append(allPackages, cfg.Packages...)
	for _, pkg := range cfg.PostHooks {
		allPackages = append(allPackages, pkg.Name)
	}

	if len(allPackages) == 0 {
		return nil
	}

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
	workers := min(len(allPackages), maxWorkers)

	jobs := make(chan string, len(allPackages))
	results := make(chan packageInfo, len(allPackages))

	for range workers {
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

	for _, pkg := range allPackages {
		jobs <- pkg
	}
	close(jobs)

	infoMap := make(map[string]packageInfo)
	for i := 0; i < len(allPackages); i++ {
		info := <-results
		infoMap[info.name] = info
	}

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

		if info.available != "" && info.available != info.installed {
			item.Status = fmt.Sprintf("%s (upgrade: %s)", info.installed, info.available)
			item.Color = yellow
		} else {
			item.Status = info.installed
		}

		items = append(items, item)
	}

	PrintListTable("Packages", items)
	return nil
}

// listDotfiles lists all dotfiles and their status.
func (s *Settle) listDotfiles() error {
	cfg := s.config.Dotfiles

	if len(cfg.Files) == 0 && len(cfg.Dirs) == 0 {
		return nil
	}

	manager := NewDotfilesManager(cfg.SourceDir, s.verbose)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)

	linkStatusItem := func(name string, status LinkStatus, err error) ListItem {
		var item ListItem
		item.Name = name
		if err != nil {
			item.Status = fmt.Sprintf("error: %v", err)
			item.Color = red
			return item
		}
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
		return item
	}

	var items []ListItem
	for _, link := range cfg.Files {
		status, err := manager.CheckLink(link)
		items = append(items, linkStatusItem(link.Dest, status, err))
	}
	for _, dir := range cfg.Dirs {
		status, err := manager.CheckDir(dir)
		items = append(items, linkStatusItem(dir.Dest, status, err))
	}

	PrintListTable("Dotfiles", items)
	return nil
}

// listGit lists all git repos and their status.
func (s *Settle) listGit() {
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)

	var items []ListItem
	for _, repo := range s.config.Git {
		dest := expandPath(repo.Dest)
		gitDir := filepath.Join(dest, ".git")

		var item ListItem
		item.Name = repo.Dest

		info, err := os.Stat(dest)
		if os.IsNotExist(err) {
			item.Status = "missing"
			item.Color = red
		} else if err != nil {
			item.Status = fmt.Sprintf("error: %v", err)
			item.Color = red
		} else if !info.IsDir() {
			item.Status = "not a directory"
			item.Color = red
		} else if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			item.Status = "not a git repo"
			item.Color = red
		} else {
			item.Status = "cloned"
			item.Color = green
		}

		items = append(items, item)
	}

	PrintListTable("Git Repos", items)
}

// listGo lists all Go packages and their installed status.
func (s *Settle) listGo() {
	binDir, err := GoBinPath()
	if err != nil {
		fmt.Printf("Warning: cannot determine Go bin path: %v\n", err)
		return
	}

	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)

	var items []ListItem
	for _, pkg := range s.config.Go {
		binName := GoPackageBinaryName(pkg.Path)
		var item ListItem
		item.Name = binName

		if IsGoPackageInstalled(binDir, binName) {
			item.Status = pkg.Version
			item.Color = green
		} else {
			item.Status = "missing"
			item.Color = red
		}

		items = append(items, item)
	}

	PrintListTable("Go Packages", items)
}
