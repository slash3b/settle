package main

import (
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

func TestPrintPackageTable_Empty(t *testing.T) {
	out := captureOutput(t, func() {
		PrintPackageTable(nil)
	})
	assert.Equal(t, "", out)
}

func TestPrintPackageTable_Installed(t *testing.T) {
	pkgs := []PackageStatus{
		{Name: "vim", Status: StatusInstalled},
		{Name: "curl", Status: StatusInstalled},
	}
	out := captureOutput(t, func() {
		PrintPackageTable(pkgs)
	})

	assert.Contains(t, out, "Package")
	assert.Contains(t, out, "Status")
	assert.Contains(t, out, "vim")
	assert.Contains(t, out, "curl")
	assert.Contains(t, out, "installed")
}

func TestPrintPackageTable_AllSkipped(t *testing.T) {
	pkgs := []PackageStatus{
		{Name: "vim", Status: StatusSkipped},
		{Name: "curl", Status: StatusSkipped},
	}
	out := captureOutput(t, func() {
		PrintPackageTable(pkgs)
	})

	assert.Contains(t, out, "2 packages already installed")
	// Should NOT have the table header since no installed packages
	assert.NotContains(t, out, "Package")
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

	assert.Contains(t, out, "2 packages already installed")
	assert.Contains(t, out, "vim")
	assert.Contains(t, out, "installed")
}

func TestPrintPackageTable_Pinned(t *testing.T) {
	pkgs := []PackageStatus{
		{Name: "vim", Status: StatusPinned},
	}
	out := captureOutput(t, func() {
		PrintPackageTable(pkgs)
	})

	assert.Contains(t, out, "vim")
	assert.Contains(t, out, "pinned")
}

func TestPrintListTable_Empty(t *testing.T) {
	out := captureOutput(t, func() {
		PrintListTable("Packages", nil)
	})
	assert.Equal(t, "", out)
}

func TestPrintListTable_Items(t *testing.T) {
	items := []ListItem{
		{Name: "vim", Status: "9.0.1"},
		{Name: "curl", Status: "missing"},
	}
	out := captureOutput(t, func() {
		PrintListTable("Packages", items)
	})

	assert.Contains(t, out, "Packages:")
	assert.Contains(t, out, "Name")
	assert.Contains(t, out, "Status")
	assert.Contains(t, out, "vim")
	assert.Contains(t, out, "9.0.1")
	assert.Contains(t, out, "curl")
	assert.Contains(t, out, "missing")
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

	assert.Contains(t, out, "vim")
	assert.Contains(t, out, "missing")
	assert.Contains(t, out, "curl")
}
