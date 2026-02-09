package main

import (
	"os"
	"strings"
)

// Distro represents a Linux distribution
type Distro string

const (
	DistroDebian  Distro = "debian"
	DistroUbuntu  Distro = "ubuntu"
	DistroUnknown Distro = "unknown"
)

// osReleasePath is the path to the os-release file, swappable in tests.
var osReleasePath = "/etc/os-release"

// DetectDistro detects the current Linux distribution
func DetectDistro() Distro {
	// Read /etc/os-release which is standard on modern Linux
	data, err := os.ReadFile(osReleasePath)
	if err != nil {
		return DistroUnknown
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	var id, idLike string
	for _, line := range lines {
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		}
		if strings.HasPrefix(line, "ID_LIKE=") {
			idLike = strings.Trim(strings.TrimPrefix(line, "ID_LIKE="), "\"")
		}
	}

	// Check ID first
	switch id {
	case "debian":
		return DistroDebian
	case "ubuntu":
		return DistroUbuntu
	}

	// Check ID_LIKE for derivatives
	if strings.Contains(idLike, "debian") || strings.Contains(idLike, "ubuntu") {
		return DistroDebian // Use Debian package manager for all Debian-based distros
	}

	return DistroUnknown
}

// IsDebianBased returns true if the distro uses apt/dpkg
func (d Distro) IsDebianBased() bool {
	return d == DistroDebian || d == DistroUbuntu
}

// String returns the distro name
func (d Distro) String() string {
	return string(d)
}
