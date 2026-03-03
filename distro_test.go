package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectDistro_Debian(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\nVERSION_ID=\"12\"\n")

	assert.Equal(t, DistroDebian, DetectDistro())
}

func TestDetectDistro_Ubuntu(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=ubuntu\nVERSION_ID=\"22.04\"\n")

	assert.Equal(t, DistroUbuntu, DetectDistro())
}

func TestDetectDistro_DebianDerivative(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=linuxmint\nID_LIKE=debian\n")

	assert.Equal(t, DistroDebian, DetectDistro())
}

func TestDetectDistro_UbuntuDerivative(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=pop\nID_LIKE=ubuntu debian\n")

	assert.Equal(t, DistroDebian, DetectDistro())
}

func TestDetectDistro_Unknown(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=arch\n")

	assert.Equal(t, DistroUnknown, DetectDistro())
}

func TestDetectDistro_MissingFile(t *testing.T) {
	saveMocks(t)

	osReleasePath = "/nonexistent/os-release"

	assert.Equal(t, DistroUnknown, DetectDistro())
}

func TestDetectDistro_QuotedID(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=\"debian\"\n")

	assert.Equal(t, DistroDebian, DetectDistro())
}

func TestIsDebianBased(t *testing.T) {
	tests := []struct {
		distro Distro
		want   bool
	}{
		{DistroDebian, true},
		{DistroUbuntu, true},
		{DistroUnknown, false},
		{Distro("arch"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.distro), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.distro.IsDebianBased())
		})
	}
}

func TestDistroString(t *testing.T) {
	assert.Equal(t, "debian", DistroDebian.String())
	assert.Equal(t, "ubuntu", DistroUbuntu.String())
	assert.Equal(t, "unknown", DistroUnknown.String())
}
