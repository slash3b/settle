package main

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

const (
	StatusInstalled = "installed"
	StatusSkipped   = "ok"
	StatusPinned    = "pinned"
)

type PackageStatus struct {
	Name   string
	Status string
}

// PrintPackageTable prints an ASCII table of packages and their statuses
// Only shows packages that were installed, with a summary for already-installed packages
func PrintPackageTable(packages []PackageStatus) {
	if len(packages) == 0 {
		return
	}

	// Separate packages by status
	var installed []PackageStatus
	skippedCount := 0

	for _, pkg := range packages {
		if pkg.Status == StatusInstalled || pkg.Status == StatusPinned {
			installed = append(installed, pkg)
		} else if pkg.Status == StatusSkipped {
			skippedCount++
		}
	}

	fmt.Println()

	// Print summary for already-installed packages
	if skippedCount > 0 {
		fmt.Printf("%d packages already installed (out of %d total)\n", skippedCount, len(packages))
	}

	// Print table only for newly installed packages
	if len(installed) > 0 {
		// Find the longest package name for column width
		maxNameLen := len("Package")
		for _, pkg := range installed {
			if len(pkg.Name) > maxNameLen {
				maxNameLen = len(pkg.Name)
			}
		}

		// Column widths
		nameWidth := maxNameLen + 2
		statusWidth := len("installed") + 2

		// Print header
		fmt.Println()
		fmt.Printf("%-*s | %s\n", nameWidth, "Package", "Status")
		fmt.Printf("%s-+-%s\n", strings.Repeat("-", nameWidth), strings.Repeat("-", statusWidth))

		// Print rows
		for _, pkg := range installed {
			fmt.Printf("%-*s | %s\n", nameWidth, pkg.Name, pkg.Status)
		}
	}

	fmt.Println()
}

// ListItem represents an item in the list table
type ListItem struct {
	Name   string
	Status string
	Color  *color.Color // nil for default color
}

// PrintListTable prints a formatted table for list command
func PrintListTable(title string, items []ListItem) {
	if len(items) == 0 {
		return
	}

	// Find max widths
	maxNameLen := len("Name")
	maxStatusLen := len("Status")
	for _, item := range items {
		if len(item.Name) > maxNameLen {
			maxNameLen = len(item.Name)
		}
		if len(item.Status) > maxStatusLen {
			maxStatusLen = len(item.Status)
		}
	}

	nameWidth := maxNameLen + 2
	statusWidth := maxStatusLen + 2

	// Print title
	fmt.Printf("\n%s:\n", title)
	fmt.Printf("%-*s | %s\n", nameWidth, "Name", "Status")
	fmt.Printf("%s-+-%s\n", strings.Repeat("-", nameWidth), strings.Repeat("-", statusWidth))

	// Print rows
	for _, item := range items {
		if item.Color != nil {
			fmt.Printf("%-*s | %s\n", nameWidth, item.Name, item.Color.Sprint(item.Status))
		} else {
			fmt.Printf("%-*s | %s\n", nameWidth, item.Name, item.Status)
		}
	}
}
