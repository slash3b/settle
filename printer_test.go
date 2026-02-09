package main

import (
	"testing"

	"github.com/fatih/color"
)

func TestPrintPackageTable_Empty(t *testing.T) {
	out := captureOutput(t, func() {
		PrintPackageTable(nil)
	})
	assertEqualStr(t, out, "")
}

func TestPrintPackageTable_Installed(t *testing.T) {
	pkgs := []PackageStatus{
		{Name: "vim", Status: StatusInstalled},
		{Name: "curl", Status: StatusInstalled},
	}
	out := captureOutput(t, func() {
		PrintPackageTable(pkgs)
	})

	assertContains(t, out, "Package")
	assertContains(t, out, "Status")
	assertContains(t, out, "vim")
	assertContains(t, out, "curl")
	assertContains(t, out, "installed")
}

func TestPrintPackageTable_AllSkipped(t *testing.T) {
	pkgs := []PackageStatus{
		{Name: "vim", Status: StatusSkipped},
		{Name: "curl", Status: StatusSkipped},
	}
	out := captureOutput(t, func() {
		PrintPackageTable(pkgs)
	})

	assertContains(t, out, "2 packages already installed")
	// Should NOT have the table header since no installed packages
	assertNotContains(t, out, "Package")
}

func TestPrintPackageTable_Mixed(t *testing.T) {
	pkgs := []PackageStatus{
		{Name: "vim", Status: StatusInstalled},
		{Name: "curl", Status: StatusSkipped},
		{Name: "git", Status: StatusSkipped},
	}
	out := captureOutput(t, func() {
		PrintPackageTable(pkgs)
	})

	assertContains(t, out, "2 packages already installed")
	assertContains(t, out, "vim")
	assertContains(t, out, "installed")
}

func TestPrintPackageTable_Pinned(t *testing.T) {
	pkgs := []PackageStatus{
		{Name: "vim", Status: StatusPinned},
	}
	out := captureOutput(t, func() {
		PrintPackageTable(pkgs)
	})

	assertContains(t, out, "vim")
	assertContains(t, out, "pinned")
}

func TestPrintListTable_Empty(t *testing.T) {
	out := captureOutput(t, func() {
		PrintListTable("Packages", nil)
	})
	assertEqualStr(t, out, "")
}

func TestPrintListTable_Items(t *testing.T) {
	items := []ListItem{
		{Name: "vim", Status: "9.0.1"},
		{Name: "curl", Status: "missing"},
	}
	out := captureOutput(t, func() {
		PrintListTable("Packages", items)
	})

	assertContains(t, out, "Packages:")
	assertContains(t, out, "Name")
	assertContains(t, out, "Status")
	assertContains(t, out, "vim")
	assertContains(t, out, "9.0.1")
	assertContains(t, out, "curl")
	assertContains(t, out, "missing")
}

func TestPrintListTable_WithColor(t *testing.T) {
	// Disable color for testing
	color.NoColor = true
	defer func() { color.NoColor = false }()

	red := color.New(color.FgRed)
	items := []ListItem{
		{Name: "vim", Status: "missing", Color: red},
		{Name: "curl", Status: "9.0.1"},
	}
	out := captureOutput(t, func() {
		PrintListTable("Test", items)
	})

	assertContains(t, out, "vim")
	assertContains(t, out, "missing")
	assertContains(t, out, "curl")
}
