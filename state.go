package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const stateFileName = "lockfile.json"

// PackageState tracks the installed version of a package
type PackageState struct {
	Version     string    `json:"version"`
	InstalledAt time.Time `json:"installed_at"`
}

// State represents the settle state file
type State struct {
	Packages  map[string]PackageState `json:"packages"`
	UpdatedAt time.Time               `json:"updated_at"`
}

// StateManager handles reading and writing state
type StateManager struct {
	path  string
	state *State
	dirty bool
}

// NewStateManager creates a new state manager
func NewStateManager(configPath string) *StateManager {
	// State file lives next to config file
	dir := filepath.Dir(configPath)
	statePath := filepath.Join(dir, stateFileName)

	return &StateManager{
		path: statePath,
		state: &State{
			Packages: make(map[string]PackageState),
		},
	}
}

// Load reads the state file if it exists
func (s *StateManager) Load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		// No state file yet, start fresh
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	if err := json.Unmarshal(data, s.state); err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}

	return nil
}

// Save writes the state file (only if changes were made)
func (s *StateManager) Save() error {
	if !s.dirty {
		return nil
	}

	s.state.UpdatedAt = time.Now().Truncate(time.Second)

	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	s.dirty = false
	return nil
}

// GetPackageVersion returns the stored version for a package
func (s *StateManager) GetPackageVersion(name string) (string, bool) {
	pkg, ok := s.state.Packages[name]
	if !ok {
		return "", false
	}
	return pkg.Version, true
}

// SetPackageVersion updates the version for a package
// Preserves existing InstalledAt if package already exists
// Only marks state as dirty if there's an actual change
func (s *StateManager) SetPackageVersion(name, version string) {
	if existing, ok := s.state.Packages[name]; ok {
		// Package exists - only update if version changed
		if existing.Version != version {
			existing.Version = version
			s.state.Packages[name] = existing
			s.dirty = true
		}
	} else {
		// New package - set install time
		s.state.Packages[name] = PackageState{
			Version:     version,
			InstalledAt: time.Now().Truncate(time.Second),
		}
		s.dirty = true
	}
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

	// Parse output to find "Candidate:" line
	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
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

// CompareVersions compares two Debian package versions using dpkg.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareVersions(a, b string) int {
	if a == b {
		return 0
	}
	cmd := execCommand("dpkg", "--compare-versions", a, "lt", b)
	if cmd.Run() == nil {
		return -1 // a < b
	}
	return 1 // a >= b, and we already checked a != b
}

type versionResult struct {
	name    string
	version string
}

// SyncPackageVersions updates state with current installed versions (concurrent)
func (s *StateManager) SyncPackageVersions(packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	const maxWorkers = 20
	workers := min(len(packages), maxWorkers)

	jobs := make(chan string, len(packages))
	results := make(chan versionResult, len(packages))

	// Start workers
	for i := 0; i < workers; i++ {
		go func() {
			for pkg := range jobs {
				version, err := GetInstalledVersion(pkg)
				if err != nil {
					results <- versionResult{name: pkg, version: ""}
				} else {
					results <- versionResult{name: pkg, version: version}
				}
			}
		}()
	}

	// Send jobs
	for _, pkg := range packages {
		jobs <- pkg
	}
	close(jobs)

	// Collect results — only update lockfile if installed version is higher
	for range packages {
		result := <-results
		if result.version == "" {
			continue
		}
		existing, ok := s.GetPackageVersion(result.name)
		if !ok || CompareVersions(result.version, existing) > 0 {
			s.SetPackageVersion(result.name, result.version)
		}
	}

	return nil
}

// Path returns the state file path
func (s *StateManager) Path() string {
	return s.path
}

// GetAllPackages returns all package names in the state
func (s *StateManager) GetAllPackages() []string {
	packages := make([]string, 0, len(s.state.Packages))
	for name := range s.state.Packages {
		packages = append(packages, name)
	}
	return packages
}

// RemovePackage removes a package from the state
func (s *StateManager) RemovePackage(name string) {
	if _, ok := s.state.Packages[name]; ok {
		delete(s.state.Packages, name)
		s.dirty = true
	}
}
