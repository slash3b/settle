package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// mockExecCommand returns an execCommand replacement that records calls and
// returns preconfigured output.  The returned recorder accumulates every
// (name, args...) invocation so tests can assert on it.
type cmdCall struct {
	Name string
	Args []string
}

type cmdRecorder struct {
	Calls    []cmdCall
	output   []byte // stdout data to return
	err      error  // error to return from cmd.Run/Output
	exitCode int    // for CombinedOutput / Run
}

// mockExecSuccess returns a function that replaces execCommand.
// Every call records the invocation and produces a *exec.Cmd that will
// succeed with the given stdout output.
func mockExecSuccess(output string) (func(name string, arg ...string) *exec.Cmd, *cmdRecorder) {
	rec := &cmdRecorder{output: []byte(output)}

	fn := func(name string, arg ...string) *exec.Cmd {
		rec.Calls = append(rec.Calls, cmdCall{Name: name, Args: arg})
		// Use "echo" to produce the output
		cmd := exec.Command("echo", "-n", output)
		return cmd
	}

	return fn, rec
}

// mockExecFailure returns a function that replaces execCommand.
// Every call records the invocation and produces a *exec.Cmd that will
// fail with exit code 1.
func mockExecFailure() (func(name string, arg ...string) *exec.Cmd, *cmdRecorder) {
	rec := &cmdRecorder{}

	fn := func(name string, arg ...string) *exec.Cmd {
		rec.Calls = append(rec.Calls, cmdCall{Name: name, Args: arg})
		cmd := exec.Command("false")
		return cmd
	}

	return fn, rec
}

// mockExecFunc returns a function that replaces execCommand, invoking
// a callback to decide what command to produce per call.
func mockExecFunc(cb func(name string, arg ...string) *exec.Cmd) func(name string, arg ...string) *exec.Cmd {
	return cb
}

// withTempConfig writes TOML content to a temporary file and returns its path.
func withTempConfig(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	return path
}

// captureOutput redirects os.Stdout for the duration of fn, then returns
// whatever was written.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// captureStderr redirects os.Stderr for the duration of fn.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	fn()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// saveMocks saves the current values of all swappable function variables
// and registers a t.Cleanup to restore them.
func saveMocks(t *testing.T) {
	t.Helper()
	origExecCommand := execCommand
	origGetInstalledVersion := GetInstalledVersion
	origGetAvailableVersion := GetAvailableVersion
	origOsReleasePath := osReleasePath

	t.Cleanup(func() {
		execCommand = origExecCommand
		GetInstalledVersion = origGetInstalledVersion
		GetAvailableVersion = origGetAvailableVersion
		osReleasePath = origOsReleasePath
	})
}

// writeOsRelease writes a fake os-release file and sets osReleasePath to it.
func writeOsRelease(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "os-release")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write os-release: %v", err)
	}
	osReleasePath = path
}

// writeLockfile writes a lockfile.json next to a config file.
func writeLockfile(t *testing.T, configPath string, content string) {
	t.Helper()
	dir := filepath.Dir(configPath)
	path := filepath.Join(dir, "lockfile.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write lockfile: %v", err)
	}
}

// assertContains checks that s contains substr.
func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !containsStr(s, substr) {
		t.Errorf("expected output to contain %q, got:\n%s", substr, s)
	}
}

// assertNotContains checks that s does not contain substr.
func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if containsStr(s, substr) {
		t.Errorf("expected output to NOT contain %q, got:\n%s", substr, s)
	}
}

func containsStr(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && bytes.Contains([]byte(s), []byte(substr))
}

// assertNoError fails if err is non-nil.
func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// assertError fails if err is nil.
func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error but got nil")
	}
}

// assertEqualStr fails if a != b.
func assertEqualStr(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// assertEqualInt fails if a != b.
func assertEqualInt(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

// assertEqualBool fails if a != b.
func assertEqualBool(t *testing.T, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// mockInstalledVersion installs a mock GetInstalledVersion that returns
// versions from the given map. Missing keys return an error.
func mockInstalledVersion(versions map[string]string) {
	GetInstalledVersion = func(name string) (string, error) {
		v, ok := versions[name]
		if !ok {
			return "", fmt.Errorf("package %s not found", name)
		}
		return v, nil
	}
}

// mockAvailableVersion installs a mock GetAvailableVersion that returns
// versions from the given map. Missing keys return an error.
func mockAvailableVersion(versions map[string]string) {
	GetAvailableVersion = func(name string) (string, error) {
		v, ok := versions[name]
		if !ok {
			return "", fmt.Errorf("no candidate version")
		}
		return v, nil
	}
}
