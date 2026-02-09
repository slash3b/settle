package main

import (
	"os/exec"
	"testing"
)

func TestIsInstalled_True(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		// Simulate dpkg-query returning "install ok installed"
		return exec.Command("echo", "-n", "install ok installed")
	}

	d := NewDebianManager(false)
	installed, err := d.IsInstalled("vim")
	assertNoError(t, err)
	assertEqualBool(t, installed, true)
}

func TestIsInstalled_False(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		// Simulate dpkg-query failing (package not installed)
		return exec.Command("false")
	}

	d := NewDebianManager(false)
	installed, err := d.IsInstalled("nonexistent")
	assertNoError(t, err) // returns false, nil on not-installed
	assertEqualBool(t, installed, false)
}

func TestIsInstalled_WrongStatus(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "deinstall ok config-files")
	}

	d := NewDebianManager(false)
	installed, err := d.IsInstalled("removed-pkg")
	assertNoError(t, err)
	assertEqualBool(t, installed, false)
}

func TestCheckInstalled(t *testing.T) {
	saveMocks(t)

	// vim is installed, curl is not
	execCommand = func(name string, arg ...string) *exec.Cmd {
		// arg[0] is -W, arg[1] is -f=..., arg[2] is package name
		if len(arg) >= 3 && arg[2] == "vim" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		return exec.Command("false")
	}

	d := NewDebianManager(false)
	missing, err := d.CheckInstalled([]string{"vim", "curl"})
	assertNoError(t, err)

	// curl should be in missing list
	assertEqualInt(t, len(missing), 1)
	assertEqualStr(t, missing[0], "curl")
}

func TestCheckInstalled_Empty(t *testing.T) {
	d := NewDebianManager(false)
	missing, err := d.CheckInstalled(nil)
	assertNoError(t, err)
	if missing != nil {
		t.Errorf("expected nil, got %v", missing)
	}
}

func TestCheckInstalled_AllInstalled(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}

	d := NewDebianManager(false)
	missing, err := d.CheckInstalled([]string{"vim", "curl"})
	assertNoError(t, err)
	assertEqualInt(t, len(missing), 0)
}

func TestInstall_Success(t *testing.T) {
	saveMocks(t)

	var recorded []cmdCall
	execCommand = func(name string, arg ...string) *exec.Cmd {
		recorded = append(recorded, cmdCall{Name: name, Args: arg})
		return exec.Command("true")
	}

	d := NewDebianManager(false)
	out := captureOutput(t, func() {
		err := d.Install([]string{"vim", "curl"}, nil)
		assertNoError(t, err)
	})

	assertContains(t, out, "Installing 2 packages")
	assertContains(t, out, "done")

	// Verify the command was called with correct args
	if len(recorded) != 1 {
		t.Fatalf("expected 1 command call, got %d", len(recorded))
	}
	assertEqualStr(t, recorded[0].Name, "sudo")
	// Should contain apt-get install -y vim curl
	assertContains(t, joinArgs(recorded[0].Args), "apt-get")
	assertContains(t, joinArgs(recorded[0].Args), "install")
	assertContains(t, joinArgs(recorded[0].Args), "vim")
	assertContains(t, joinArgs(recorded[0].Args), "curl")
}

func TestInstall_WithVersions(t *testing.T) {
	saveMocks(t)

	var recorded []cmdCall
	execCommand = func(name string, arg ...string) *exec.Cmd {
		recorded = append(recorded, cmdCall{Name: name, Args: arg})
		return exec.Command("true")
	}

	versions := map[string]string{
		"vim": "9.0.1-1",
	}

	d := NewDebianManager(false)
	captureOutput(t, func() {
		err := d.Install([]string{"vim", "curl"}, versions)
		assertNoError(t, err)
	})

	// Should contain vim=9.0.1-1 and curl (without version)
	args := joinArgs(recorded[0].Args)
	assertContains(t, args, "vim=9.0.1-1")
	assertContains(t, args, "curl")
}

func TestInstall_Empty(t *testing.T) {
	d := NewDebianManager(false)
	err := d.Install(nil, nil)
	assertNoError(t, err)
}

func TestInstall_Failure_ShowsStderr(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		// Use bash to produce stderr and fail
		return exec.Command("bash", "-c", "echo 'apt error' >&2; exit 1")
	}

	d := NewDebianManager(false)
	out := captureOutput(t, func() {
		err := d.Install([]string{"badpkg"}, nil)
		assertError(t, err)
	})

	assertContains(t, out, "failed")
}

func TestInstall_Verbose(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}

	d := NewDebianManager(true)
	out := captureOutput(t, func() {
		err := d.Install([]string{"vim"}, nil)
		assertNoError(t, err)
	})

	assertContains(t, out, "Installing 1 packages...")
}

func TestUpgrade_Success(t *testing.T) {
	saveMocks(t)

	var recorded []cmdCall
	execCommand = func(name string, arg ...string) *exec.Cmd {
		recorded = append(recorded, cmdCall{Name: name, Args: arg})
		return exec.Command("true")
	}

	d := NewDebianManager(false)
	out := captureOutput(t, func() {
		err := d.Upgrade([]string{"vim"})
		assertNoError(t, err)
	})

	assertContains(t, out, "Upgrading 1 packages")
	assertContains(t, out, "done")

	args := joinArgs(recorded[0].Args)
	assertContains(t, args, "--only-upgrade")
}

func TestUpgrade_Empty(t *testing.T) {
	d := NewDebianManager(false)
	err := d.Upgrade(nil)
	assertNoError(t, err)
}

func TestUpgrade_Failure_ShowsStderr(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("bash", "-c", "echo 'upgrade error' >&2; exit 1")
	}

	d := NewDebianManager(false)
	out := captureOutput(t, func() {
		err := d.Upgrade([]string{"vim"})
		assertError(t, err)
	})

	assertContains(t, out, "failed")
}

func TestUpgrade_Verbose(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}

	d := NewDebianManager(true)
	out := captureOutput(t, func() {
		err := d.Upgrade([]string{"vim"})
		assertNoError(t, err)
	})

	assertContains(t, out, "Upgrading 1 packages...")
}

func TestRemove_Success(t *testing.T) {
	saveMocks(t)

	var recorded []cmdCall
	execCommand = func(name string, arg ...string) *exec.Cmd {
		recorded = append(recorded, cmdCall{Name: name, Args: arg})
		return exec.Command("true")
	}

	d := NewDebianManager(false)
	out := captureOutput(t, func() {
		err := d.Remove([]string{"vim"})
		assertNoError(t, err)
	})

	assertContains(t, out, "Removing 1 packages")
	assertContains(t, out, "done")

	args := joinArgs(recorded[0].Args)
	assertContains(t, args, "remove")
}

func TestRemove_Empty(t *testing.T) {
	d := NewDebianManager(false)
	err := d.Remove(nil)
	assertNoError(t, err)
}

func TestRemove_Failure_ShowsStderr(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("bash", "-c", "echo 'remove error' >&2; exit 1")
	}

	d := NewDebianManager(false)
	out := captureOutput(t, func() {
		err := d.Remove([]string{"vim"})
		assertError(t, err)
	})

	assertContains(t, out, "failed")
}

func TestRemove_Verbose(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}

	d := NewDebianManager(true)
	out := captureOutput(t, func() {
		err := d.Remove([]string{"vim"})
		assertNoError(t, err)
	})

	assertContains(t, out, "Removing 1 packages...")
}

func TestRefreshPackageLists(t *testing.T) {
	saveMocks(t)

	var recorded []cmdCall
	execCommand = func(name string, arg ...string) *exec.Cmd {
		recorded = append(recorded, cmdCall{Name: name, Args: arg})
		return exec.Command("true")
	}

	d := NewDebianManager(false)
	out := captureOutput(t, func() {
		err := d.RefreshPackageLists()
		assertNoError(t, err)
	})

	assertContains(t, out, "Updating package lists")

	assertEqualStr(t, recorded[0].Name, "sudo")
	args := joinArgs(recorded[0].Args)
	assertContains(t, args, "apt-get")
	assertContains(t, args, "update")
}

func TestRunPostInstall(t *testing.T) {
	saveMocks(t)

	var recorded []cmdCall
	execCommand = func(name string, arg ...string) *exec.Cmd {
		recorded = append(recorded, cmdCall{Name: name, Args: arg})
		return exec.Command("true")
	}

	d := NewDebianManager(false)
	out := captureOutput(t, func() {
		err := d.RunPostInstall("pipewire", "systemctl --user enable wireplumber")
		assertNoError(t, err)
	})

	assertContains(t, out, "Running post-install for pipewire")
	assertContains(t, out, "done")

	assertEqualStr(t, recorded[0].Name, "bash")
	assertEqualStr(t, recorded[0].Args[0], "-c")
	assertEqualStr(t, recorded[0].Args[1], "systemctl --user enable wireplumber")
}

func TestRunPostInstall_Empty(t *testing.T) {
	d := NewDebianManager(false)
	err := d.RunPostInstall("vim", "")
	assertNoError(t, err)
}

func TestRunPostInstall_Failure(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("false")
	}

	d := NewDebianManager(false)
	out := captureOutput(t, func() {
		err := d.RunPostInstall("pipewire", "bad-command")
		assertError(t, err)
	})

	assertContains(t, out, "failed")
}

func TestRunPostInstall_Verbose(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}

	d := NewDebianManager(true)
	out := captureOutput(t, func() {
		err := d.RunPostInstall("pipewire", "echo hi")
		assertNoError(t, err)
	})

	assertContains(t, out, "Running post-install script for pipewire...")
}

// helper to join args into a single string for assertion
func joinArgs(args []string) string {
	result := ""
	for i, a := range args {
		if i > 0 {
			result += " "
		}
		result += a
	}
	return result
}
