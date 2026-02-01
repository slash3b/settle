package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const defaultConfigPath = "config.toml"

type Package struct {
	Name        string `toml:"name"`
	PostInstall string `toml:"post_install"`
}

type LinuxConfig struct {
	Packages []string  `toml:"packages"`
	Package  []Package `toml:"package"`
}

type Dotfile struct {
	Src  string `toml:"src"`
	Dest string `toml:"dest"`
	Mode string `toml:"mode"` // "link" (default) or "copy"
}

type DotfilesConfig struct {
	SourceDir string    `toml:"source_dir"`
	Files     []Dotfile `toml:"file"`
}

type Config struct {
	Linux    *LinuxConfig    `toml:"linux"`
	Dotfiles *DotfilesConfig `toml:"dotfiles"`
	// Future managers:
	// Cargo  *CargoConfig  `toml:"cargo"`
	// Go     *GoConfig     `toml:"go"`
}

func loadConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s\n\nCreate a config.toml file or specify a path with --config", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var cfg Config
	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("error parsing TOML: %w", err)
	}

	return &cfg, nil
}

// addPackagesToConfig adds packages to the [linux] packages array while preserving formatting
func addPackagesToConfig(path string, packages []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Find the packages array in [linux] section
	inLinuxSection := false
	packagesLineIdx := -1
	closingBracketIdx := -1
	indent := "    " // default indent

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "[linux]" {
			inLinuxSection = true
			continue
		}

		// Check if we've left the linux section
		if inLinuxSection && strings.HasPrefix(trimmed, "[") && trimmed != "[linux]" {
			break
		}

		if inLinuxSection && strings.HasPrefix(trimmed, "packages") && strings.Contains(line, "=") {
			packagesLineIdx = i

			// Check if it's a single-line array
			if strings.Contains(line, "]") {
				// Single line array like: packages = ["a", "b"]
				// Insert before the closing bracket
				closingBracketIdx = i
				break
			}
			continue
		}

		// If we're after the packages line, look for the closing bracket
		if packagesLineIdx != -1 && closingBracketIdx == -1 {
			// Detect indent from existing entries
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				if match := regexp.MustCompile(`^(\s+)`).FindString(line); match != "" {
					indent = match
				}
			}
			if trimmed == "]" {
				closingBracketIdx = i
				break
			}
		}
	}

	if packagesLineIdx == -1 {
		return fmt.Errorf("could not find packages array in [linux] section")
	}

	// Build the new package entries
	var newEntries []string
	for _, pkg := range packages {
		newEntries = append(newEntries, fmt.Sprintf("%s\"%s\",", indent, pkg))
	}

	// Handle single-line vs multi-line array
	if packagesLineIdx == closingBracketIdx {
		// Single-line array - convert to multi-line or append inline
		line := lines[packagesLineIdx]
		bracketIdx := strings.LastIndex(line, "]")
		if bracketIdx != -1 {
			before := strings.TrimRight(line[:bracketIdx], " \t")
			// Check if array is empty
			if strings.HasSuffix(strings.TrimSpace(before), "[") {
				// Empty array, add entries
				lines[packagesLineIdx] = before
				newLines := append([]string{lines[packagesLineIdx]}, newEntries...)
				newLines = append(newLines, "]")
				lines = append(lines[:packagesLineIdx], append(newLines, lines[packagesLineIdx+1:]...)...)
			} else {
				// Non-empty single line - add comma if needed and append
				if !strings.HasSuffix(before, ",") && !strings.HasSuffix(strings.TrimSpace(before), "[") {
					before += ","
				}
				for _, pkg := range packages {
					before += fmt.Sprintf(" \"%s\",", pkg)
				}
				lines[packagesLineIdx] = before + "]"
			}
		}
	} else {
		// Multi-line array - insert before closing bracket
		// First, find the last package entry and ensure it has a trailing comma
		for i := closingBracketIdx - 1; i > packagesLineIdx; i-- {
			trimmed := strings.TrimSpace(lines[i])
			// Skip empty lines and comments
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			// Found the last entry - add comma if missing
			if strings.Contains(trimmed, "\"") && !strings.HasSuffix(trimmed, ",") {
				lines[i] = lines[i] + ","
			}
			break
		}

		newLines := make([]string, 0, len(lines)+len(newEntries))
		newLines = append(newLines, lines[:closingBracketIdx]...)
		newLines = append(newLines, newEntries...)
		newLines = append(newLines, lines[closingBracketIdx:]...)
		lines = newLines
	}

	result := strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(result), 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}

// removePackagesFromConfig removes packages from the [linux] packages array while preserving formatting
func removePackagesFromConfig(path string, packages []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	content := string(data)

	// Build a set of packages to remove
	toRemove := make(map[string]bool)
	for _, pkg := range packages {
		toRemove[pkg] = true
	}

	// Process line by line
	var result strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))
	inLinuxSection := false
	inPackagesArray := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "[linux]" {
			inLinuxSection = true
			result.WriteString(line + "\n")
			continue
		}

		if inLinuxSection && strings.HasPrefix(trimmed, "[") && trimmed != "[linux]" {
			inLinuxSection = false
		}

		if inLinuxSection && strings.HasPrefix(trimmed, "packages") && strings.Contains(line, "=") {
			inPackagesArray = true
		}

		if inPackagesArray {
			// Check if this line contains a package to remove
			shouldRemove := false
			for pkg := range toRemove {
				// Match "package" or 'package' with optional trailing comma
				pattern := fmt.Sprintf(`["']%s["'],?`, regexp.QuoteMeta(pkg))
				if matched, _ := regexp.MatchString(pattern, line); matched {
					shouldRemove = true
					break
				}
			}

			if trimmed == "]" {
				inPackagesArray = false
			}

			if shouldRemove {
				continue // Skip this line
			}
		}

		result.WriteString(line + "\n")
	}

	// Remove trailing newline if original didn't have one
	output := result.String()
	if !strings.HasSuffix(content, "\n") {
		output = strings.TrimSuffix(output, "\n")
	}

	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}
