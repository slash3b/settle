package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/fatih/color"
)

const statusMissing = "missing"

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

// Apply applies the configuration: installs missing packages, links dotfiles, clones repos.
func (s *Settle) Apply() error {
	if s.dryRun {
		fmt.Println("[dry-run mode - no changes will be made]")
		fmt.Println()
	}

	managersFound := 0

	var errs []error

	if s.config.Apt != nil {
		managersFound++

		err := s.applyApt()
		if err != nil {
			errs = append(errs, fmt.Errorf("apt: %w", err))
		}
	}

	if s.config.Dotfiles != nil {
		managersFound++

		err := s.applyDotfiles()
		if err != nil {
			errs = append(errs, fmt.Errorf("dotfiles: %w", err))
		}
	}

	if len(s.config.Git) > 0 {
		managersFound++

		err := s.applyGit()
		if err != nil {
			errs = append(errs, fmt.Errorf("git: %w", err))
		}
	}

	if len(s.config.Go) > 0 {
		managersFound++

		err := s.applyGo()
		if err != nil {
			errs = append(errs, fmt.Errorf("go: %w", err))
		}
	}

	if managersFound == 0 {
		return fmt.Errorf("no packages, dotfiles, git repos, or go packages configured in config.toml")
	}

	if len(errs) == 0 {
		fmt.Println("Done!")
	}

	return errors.Join(errs...)
}

// Update upgrades all managed packages to their latest versions.
func (s *Settle) Update() error {
	var (
		hasApt = s.config.Apt != nil
		hasGit = len(s.config.Git) > 0
		hasGo  = len(s.config.Go) > 0
		errs   []error
	)

	if hasApt {
		distro := DetectDistro()
		if !distro.IsDebianBased() {
			errs = append(errs, fmt.Errorf("unsupported distribution: %s", distro))
		} else {
			allPackages := make([]string, 0, len(s.config.Apt.Packages)+len(s.config.Apt.PostHooks))

			allPackages = append(allPackages, s.config.Apt.Packages...)
			for _, pkg := range s.config.Apt.PostHooks {
				allPackages = append(allPackages, pkg.Name)
			}

			manager := NewDebianManager(s.verbose)

			if len(allPackages) > 0 {
				fmt.Printf("Updating %d managed packages...\n", len(allPackages))

				if s.dryRun {
					fmt.Println("[dry-run] Would run: apt-get update")
					fmt.Printf("[dry-run] Would upgrade: %v\n", allPackages)
				} else {
					err := manager.RefreshPackageLists()
					if err != nil {
						errs = append(errs, fmt.Errorf("failed to update package lists: %w", err))
					}

					err = manager.Upgrade(allPackages)
					if err != nil {
						errs = append(errs, fmt.Errorf("failed to upgrade packages: %w", err))
					}
				}
			}
		}
	}

	if hasGit {
		err := s.updateGit()
		if err != nil {
			errs = append(errs, fmt.Errorf("git: %w", err))
		}
	}

	if hasGo {
		err := s.updateGo()
		if err != nil {
			errs = append(errs, fmt.Errorf("go: %w", err))
		}
	}

	if !hasApt && !hasGit && !hasGo {
		fmt.Println("nothing to update")

		return nil
	}

	return errors.Join(errs...)
}

// List shows the status of all packages and dotfiles.
func (s *Settle) List() {
	if s.config.Apt != nil {
		s.listApt()
	}

	if s.config.Dotfiles != nil {
		s.listDotfiles()
	}

	if len(s.config.Git) > 0 {
		s.listGit()
	}

	if len(s.config.Go) > 0 {
		s.listGo()
	}
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

	missingPackages := manager.CheckInstalled(allPackages)

	missingSet := make(map[string]bool)
	for _, pkg := range missingPackages {
		missingSet[pkg] = true
	}

	installedCount := len(allPackages) - len(missingPackages)
	fmt.Printf("Already installed: %d\n", installedCount)
	fmt.Printf("Need to install: %d\n", len(missingPackages))

	// Filter out unknown packages
	var (
		installable []string
		unknown     []string
	)

	for _, pkg := range missingPackages {
		_, err := GetAvailableVersion(pkg)
		if err != nil {
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
			err := manager.RefreshPackageLists()
			if err != nil {
				return fmt.Errorf("failed to update package lists: %w", err)
			}

			err = manager.Install(installable)
			if err != nil {
				return fmt.Errorf("error installing packages: %w", err)
			}

			var hooksToRun []PostHook

			for _, pkg := range aptCfg.PostHooks {
				if pkg.PostInstall != "" && missingSet[pkg.Name] {
					hooksToRun = append(hooksToRun, pkg)
				}
			}

			if len(hooksToRun) > 0 {
				err = ValidateSudo()
				if err != nil {
					return fmt.Errorf("sudo authentication failed: %w", err)
				}

				var hookErrs []error

				for _, pkg := range hooksToRun {
					err = manager.RunPostInstall(pkg.Name, pkg.PostInstall, pkg.Sudo)
					if err != nil {
						hookErrs = append(hookErrs, fmt.Errorf("post-install for %s: %w", pkg.Name, err))
					}
				}

				err = errors.Join(hookErrs...)
				if err != nil {
					return err
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

	var errs []error

	for _, link := range cfg.Files {
		created, err := manager.Apply(link, s.dryRun)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", link.Dest, err))
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
			errs = append(errs, fmt.Errorf("%s: %w", dir.Dest, err))
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

	return errors.Join(errs...)
}

// applyGit clones missing git repos.
func (s *Settle) applyGit() error {
	fmt.Printf("\nChecking %d git repos...\n", len(s.config.Git))

	var (
		statuses []PackageStatus
		errs     []error
	)

	for _, repo := range s.config.Git {
		dest := expandPath(repo.Dest)
		gitDir := filepath.Join(dest, ".git")

		info, err := os.Stat(dest)
		if err == nil {
			if info.IsDir() {
				_, err = os.Stat(gitDir)
				if err == nil {
					statuses = append(statuses, PackageStatus{Name: repo.Dest, Status: StatusSkipped})

					continue
				}

				errs = append(errs, fmt.Errorf("destination %s exists but is not a git repository", dest))

				continue
			}

			errs = append(errs, fmt.Errorf("destination %s exists but is not a directory", dest))

			continue
		}

		if s.dryRun {
			fmt.Printf("[dry-run] Would clone: %s -> %s\n", repo.URL, repo.Dest)
			statuses = append(statuses, PackageStatus{Name: repo.Dest, Status: StatusInstalled})

			continue
		}

		err = GitClone(repo.URL, dest, s.verbose)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to clone %s: %w", repo.URL, err))

			continue
		}

		statuses = append(statuses, PackageStatus{Name: repo.Dest, Status: StatusInstalled})
	}

	PrintPackageTable(statuses)

	return errors.Join(errs...)
}

// applyGo installs missing Go packages via `go install`.
func (s *Settle) applyGo() error {
	binDir, err := GoBinPath()
	if err != nil {
		return fmt.Errorf("cannot determine Go bin path: %w", err)
	}

	fmt.Printf("\nChecking %d go packages...\n", len(s.config.Go))

	var (
		statuses []PackageStatus
		errs     []error
	)

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

		err = GoInstall(pkg.Path, pkg.Version, s.verbose)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to install %s: %w", pkg.Path, err))
			continue
		}

		statuses = append(statuses, PackageStatus{Name: binName, Status: StatusInstalled})
	}

	PrintPackageTable(statuses)

	return errors.Join(errs...)
}

// updateGit pulls latest changes for all cloned git repos.
func (s *Settle) updateGit() error {
	fmt.Printf("\nUpdating %d git repos...\n", len(s.config.Git))

	var errs []error

	for _, repo := range s.config.Git {
		dest := expandPath(repo.Dest)
		gitDir := filepath.Join(dest, ".git")

		_, err := os.Stat(gitDir)
		if os.IsNotExist(err) {
			fmt.Printf("Warning: %s not cloned, run settle first\n", repo.Dest)

			continue
		}

		if s.dryRun {
			fmt.Printf("[dry-run] Would pull: %s\n", repo.Dest)

			continue
		}

		err = GitPullRepo(dest, s.verbose)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to pull %s: %w", repo.Dest, err))
		}
	}

	return errors.Join(errs...)
}

// updateGo re-installs all Go packages at their configured versions.
func (s *Settle) updateGo() error {
	fmt.Printf("\nUpdating %d go packages...\n", len(s.config.Go))

	var errs []error

	for _, pkg := range s.config.Go {
		if s.dryRun {
			fmt.Printf("[dry-run] Would install: go install %s@%s\n", pkg.Path, pkg.Version)

			continue
		}

		err := GoInstall(pkg.Path, pkg.Version, s.verbose)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to update %s: %w", pkg.Path, err))
		}
	}

	return errors.Join(errs...)
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
func (s *Settle) listApt() {
	manager := NewDebianManager(s.verbose)
	cfg := s.config.Apt

	allPackages := make([]string, 0, len(cfg.Packages)+len(cfg.PostHooks))

	allPackages = append(allPackages, cfg.Packages...)
	for _, pkg := range cfg.PostHooks {
		allPackages = append(allPackages, pkg.Name)
	}

	if len(allPackages) == 0 {
		return
	}

	missing := manager.CheckInstalled(allPackages)

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

	for range allPackages {
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
				item.Status = statusMissing
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
}

// listDotfiles lists all dotfiles and their status.
func (s *Settle) listDotfiles() {
	cfg := s.config.Dotfiles

	if len(cfg.Files) == 0 && len(cfg.Dirs) == 0 {
		return
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
			item.Status = statusMissing
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
}

// listGit lists all git repos and their status.
func (s *Settle) listGit() {
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)

	items := make([]ListItem, 0, len(s.config.Git))
	for _, repo := range s.config.Git {
		dest := expandPath(repo.Dest)
		gitDir := filepath.Join(dest, ".git")

		var item ListItem

		item.Name = repo.Dest

		info, err := os.Stat(dest)

		_, gitErr := os.Stat(gitDir)
		switch {
		case os.IsNotExist(err):
			item.Status = statusMissing
			item.Color = red
		case err != nil:
			item.Status = fmt.Sprintf("error: %v", err)
			item.Color = red
		case !info.IsDir():
			item.Status = "not a directory"
			item.Color = red
		case os.IsNotExist(gitErr):
			item.Status = "not a git repo"
			item.Color = red
		default:
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
			item.Status = statusMissing
			item.Color = red
		}

		items = append(items, item)
	}

	PrintListTable("Go Packages", items)
}
