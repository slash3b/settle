package main

import (
	"testing"
)

func TestDetectDistro_Debian(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=debian\nVERSION_ID=\"12\"\n")

	d := DetectDistro()
	if d != DistroDebian {
		t.Errorf("expected DistroDebian, got %s", d)
	}
}

func TestDetectDistro_Ubuntu(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=ubuntu\nVERSION_ID=\"22.04\"\n")

	d := DetectDistro()
	if d != DistroUbuntu {
		t.Errorf("expected DistroUbuntu, got %s", d)
	}
}

func TestDetectDistro_DebianDerivative(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=linuxmint\nID_LIKE=debian\n")

	d := DetectDistro()
	if d != DistroDebian {
		t.Errorf("expected DistroDebian for derivative, got %s", d)
	}
}

func TestDetectDistro_UbuntuDerivative(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=pop\nID_LIKE=ubuntu debian\n")

	d := DetectDistro()
	if d != DistroDebian {
		t.Errorf("expected DistroDebian for ubuntu derivative, got %s", d)
	}
}

func TestDetectDistro_Unknown(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=arch\n")

	d := DetectDistro()
	if d != DistroUnknown {
		t.Errorf("expected DistroUnknown, got %s", d)
	}
}

func TestDetectDistro_MissingFile(t *testing.T) {
	saveMocks(t)
	osReleasePath = "/nonexistent/os-release"

	d := DetectDistro()
	if d != DistroUnknown {
		t.Errorf("expected DistroUnknown for missing file, got %s", d)
	}
}

func TestDetectDistro_QuotedID(t *testing.T) {
	saveMocks(t)
	writeOsRelease(t, "ID=\"debian\"\n")

	d := DetectDistro()
	if d != DistroDebian {
		t.Errorf("expected DistroDebian for quoted ID, got %s", d)
	}
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
			got := tt.distro.IsDebianBased()
			assertEqualBool(t, got, tt.want)
		})
	}
}

func TestDistroString(t *testing.T) {
	assertEqualStr(t, DistroDebian.String(), "debian")
	assertEqualStr(t, DistroUbuntu.String(), "ubuntu")
	assertEqualStr(t, DistroUnknown.String(), "unknown")
}
