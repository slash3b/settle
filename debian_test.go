package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsInstalled_True(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}

	d := NewDebianManager(false)
	installed, err := d.IsInstalled("vim")
	require.NoError(t, err)
	assert.True(t, installed)
}

func TestIsInstalled_False(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("false")
	}

	d := NewDebianManager(false)
	installed, err := d.IsInstalled("nonexistent")
	require.NoError(t, err)
	assert.False(t, installed)
}

func TestIsInstalled_WrongStatus(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "deinstall ok config-files")
	}

	d := NewDebianManager(false)
	installed, err := d.IsInstalled("removed-pkg")
	require.NoError(t, err)
	assert.False(t, installed)
}

func TestCheckInstalled(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		if len(arg) >= 3 && arg[2] == "vim" {
			return exec.Command("echo", "-n", "install ok installed")
		}
		return exec.Command("false")
	}

	d := NewDebianManager(false)
	missing, err := d.CheckInstalled([]string{"vim", "curl"})
	require.NoError(t, err)
	assert.Equal(t, 1, len(missing))
	assert.Equal(t, "curl", missing[0])
}

func TestCheckInstalled_Empty(t *testing.T) {
	d := NewDebianManager(false)
	missing, err := d.CheckInstalled(nil)
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestCheckInstalled_AllInstalled(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "-n", "install ok installed")
	}

	d := NewDebianManager(false)
	missing, err := d.CheckInstalled([]string{"vim", "curl"})
	require.NoError(t, err)
	assert.Equal(t, 0, len(missing))
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
		err := d.Install([]string{"vim", "curl"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Installing 2 packages")
	assert.Contains(t, out, "done")
	require.Equal(t, 1, len(recorded))
	assert.Equal(t, "sudo", recorded[0].Name)
	args := joinArgs(recorded[0].Args)
	assert.Contains(t, args, "apt-get")
	assert.Contains(t, args, "install")
	assert.Contains(t, args, "vim")
	assert.Contains(t, args, "curl")
}

func TestInstall_Empty(t *testing.T) {
	d := NewDebianManager(false)
	err := d.Install(nil)
	require.NoError(t, err)
}

func TestInstall_Failure_ShowsStderr(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("bash", "-c", "echo 'apt error' >&2; exit 1")
	}

	d := NewDebianManager(false)
	out := captureOutput(t, func() {
		err := d.Install([]string{"badpkg"})
		require.Error(t, err)
	})

	assert.Contains(t, out, "failed")
}

func TestInstall_Verbose(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}

	d := NewDebianManager(true)
	out := captureOutput(t, func() {
		err := d.Install([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Installing 1 packages...")
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
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Upgrading 1 packages")
	assert.Contains(t, out, "done")
	args := joinArgs(recorded[0].Args)
	assert.Contains(t, args, "--only-upgrade")
}

func TestUpgrade_Empty(t *testing.T) {
	d := NewDebianManager(false)
	err := d.Upgrade(nil)
	require.NoError(t, err)
}

func TestUpgrade_Failure_ShowsStderr(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("bash", "-c", "echo 'upgrade error' >&2; exit 1")
	}

	d := NewDebianManager(false)
	out := captureOutput(t, func() {
		err := d.Upgrade([]string{"vim"})
		require.Error(t, err)
	})

	assert.Contains(t, out, "failed")
}

func TestUpgrade_Verbose(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}

	d := NewDebianManager(true)
	out := captureOutput(t, func() {
		err := d.Upgrade([]string{"vim"})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Upgrading 1 packages...")
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
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Updating package lists")
	assert.Equal(t, "sudo", recorded[0].Name)
	args := joinArgs(recorded[0].Args)
	assert.Contains(t, args, "apt-get")
	assert.Contains(t, args, "update")
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
		err := d.RunPostInstall("pipewire", "systemctl --user enable wireplumber", false)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Running post-install for pipewire")
	assert.Contains(t, out, "done")
	assert.Equal(t, "bash", recorded[0].Name)
	assert.Equal(t, "-c", recorded[0].Args[0])
	assert.Equal(t, "systemctl --user enable wireplumber", recorded[0].Args[1])
}

func TestRunPostInstall_Empty(t *testing.T) {
	d := NewDebianManager(false)
	err := d.RunPostInstall("vim", "", false)
	require.NoError(t, err)
}

func TestRunPostInstall_Failure(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("false")
	}

	d := NewDebianManager(false)
	out := captureOutput(t, func() {
		err := d.RunPostInstall("pipewire", "bad-command", false)
		require.Error(t, err)
	})

	assert.Contains(t, out, "failed")
}

func TestRunPostInstall_Verbose(t *testing.T) {
	saveMocks(t)

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}

	d := NewDebianManager(true)
	out := captureOutput(t, func() {
		err := d.RunPostInstall("pipewire", "echo hi", false)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Running post-install script for pipewire...")
}

func TestRunPostInstall_Sudo(t *testing.T) {
	saveMocks(t)

	var recorded []cmdCall
	execCommand = func(name string, arg ...string) *exec.Cmd {
		recorded = append(recorded, cmdCall{Name: name, Args: arg})
		return exec.Command("true")
	}

	d := NewDebianManager(false)
	captureOutput(t, func() {
		err := d.RunPostInstall("mypkg", "some-system-command", true)
		require.NoError(t, err)
	})

	require.Len(t, recorded, 1)
	assert.Equal(t, "sudo", recorded[0].Name)
	assert.Equal(t, []string{"bash", "-c", "some-system-command"}, recorded[0].Args)
}

// helper to join args into a single string for assertion
func joinArgs(args []string) string {
	var result strings.Builder
	for i, a := range args {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(a)
	}
	return result.String()
}
