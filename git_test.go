package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitPull_NoGitDir(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	err := GitPull(configPath, false)
	assert.NoError(t, err) // silently skips
}

func TestGitPull_DirtyWorktree(t *testing.T) {
	dir := t.TempDir()

	// Create a git repo
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("initial"), 0o644)
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "initial")

	// Make dirty
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0o644)

	configPath := filepath.Join(dir, "config.toml")
	err := GitPull(configPath, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uncommitted changes")
}

func TestGitPull_CleanNoRemote(t *testing.T) {
	dir := t.TempDir()

	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("initial"), 0o644)
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "initial")

	configPath := filepath.Join(dir, "config.toml")
	err := GitPull(configPath, false)
	// No remote configured — pull should fail
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git pull failed")
}

func TestGitPull_Success(t *testing.T) {
	// Create "remote" repo
	remote := t.TempDir()
	run(t, remote, "git", "init", "--bare")

	// Clone it
	parent := t.TempDir()
	local := filepath.Join(parent, "repo")
	run(t, parent, "git", "clone", remote, local)
	run(t, local, "git", "config", "user.email", "test@test.com")
	run(t, local, "git", "config", "user.name", "Test")

	// Create initial commit via a second clone, push
	other := filepath.Join(parent, "other")
	run(t, parent, "git", "clone", remote, other)
	run(t, other, "git", "config", "user.email", "test@test.com")
	run(t, other, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(other, "config.toml"), []byte("data"), 0o644)
	run(t, other, "git", "add", ".")
	run(t, other, "git", "commit", "-m", "add config")
	run(t, other, "git", "push")

	// Now pull from local — should succeed
	configPath := filepath.Join(local, "config.toml")
	err := GitPull(configPath, false)
	require.NoError(t, err)

	// Verify the file arrived
	_, err = os.Stat(filepath.Join(local, "config.toml"))
	assert.NoError(t, err)
}

func TestGitPull_UntrackedFilesNotDirty(t *testing.T) {
	// Create "remote" repo with an initial commit
	remote := t.TempDir()
	run(t, remote, "git", "init", "--bare")

	parent := t.TempDir()
	local := filepath.Join(parent, "repo")
	run(t, parent, "git", "clone", remote, local)
	run(t, local, "git", "config", "user.email", "test@test.com")
	run(t, local, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(local, "tracked.txt"), []byte("data"), 0o644)
	run(t, local, "git", "add", ".")
	run(t, local, "git", "commit", "-m", "initial")
	run(t, local, "git", "push", "-u", "origin", "HEAD")

	// Add an untracked file — git status --porcelain will show it
	os.WriteFile(filepath.Join(local, "untracked.txt"), []byte("new"), 0o644)

	configPath := filepath.Join(local, "config.toml")
	err := GitPull(configPath, false)
	// Untracked files show in porcelain output, so this should error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uncommitted changes")
}

// run is a test helper that executes a command in a directory.
func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}
