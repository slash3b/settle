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

// --- GitClone tests ---

func TestGitClone_Success(t *testing.T) {
	// Create a bare repo as the "remote"
	remote := t.TempDir()
	run(t, remote, "git", "init", "--bare")

	// Create a temp clone, add a commit, push — so the bare repo has content
	scratch := t.TempDir()
	run(t, scratch, "git", "clone", remote, "work")
	work := filepath.Join(scratch, "work")
	run(t, work, "git", "config", "user.email", "test@test.com")
	run(t, work, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(work, "README.md"), []byte("hello"), 0o644)
	run(t, work, "git", "add", ".")
	run(t, work, "git", "commit", "-m", "init")
	run(t, work, "git", "push")

	// Now clone from our bare remote using GitClone
	dest := filepath.Join(t.TempDir(), "cloned")
	err := GitClone(remote, dest, false)
	require.NoError(t, err)

	// Verify .git exists
	_, err = os.Stat(filepath.Join(dest, ".git"))
	assert.NoError(t, err)

	// Verify the file arrived
	data, err := os.ReadFile(filepath.Join(dest, "README.md"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestGitClone_BadURL(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "cloned")
	err := GitClone("/nonexistent/path/repo.git", dest, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git clone failed")
}

func TestGitClone_DestExists(t *testing.T) {
	// Create a bare repo
	remote := t.TempDir()
	run(t, remote, "git", "init", "--bare")

	scratch := t.TempDir()
	run(t, scratch, "git", "clone", remote, "work")
	work := filepath.Join(scratch, "work")
	run(t, work, "git", "config", "user.email", "test@test.com")
	run(t, work, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(work, "README.md"), []byte("hello"), 0o644)
	run(t, work, "git", "add", ".")
	run(t, work, "git", "commit", "-m", "init")
	run(t, work, "git", "push")

	// Create destination directory that already exists with a file in it
	dest := filepath.Join(t.TempDir(), "cloned")
	os.MkdirAll(dest, 0o755)
	os.WriteFile(filepath.Join(dest, "blocker.txt"), []byte("occupied"), 0o644)

	// git clone should fail because dest is non-empty
	err := GitClone(remote, dest, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git clone failed")
}

// --- GitPullRepo tests ---

func TestGitPullRepo_Success(t *testing.T) {
	// Create "remote" repo
	remote := t.TempDir()
	run(t, remote, "git", "init", "--bare")

	// Clone to local
	parent := t.TempDir()
	local := filepath.Join(parent, "repo")
	run(t, parent, "git", "clone", remote, local)
	run(t, local, "git", "config", "user.email", "test@test.com")
	run(t, local, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(local, "file.txt"), []byte("initial"), 0o644)
	run(t, local, "git", "add", ".")
	run(t, local, "git", "commit", "-m", "initial")
	run(t, local, "git", "push", "-u", "origin", "HEAD")

	// Push a new commit from another clone
	other := filepath.Join(parent, "other")
	run(t, parent, "git", "clone", remote, other)
	run(t, other, "git", "config", "user.email", "test@test.com")
	run(t, other, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(other, "new.txt"), []byte("new content"), 0o644)
	run(t, other, "git", "add", ".")
	run(t, other, "git", "commit", "-m", "add new file")
	run(t, other, "git", "push")

	// Pull from local using GitPullRepo
	err := GitPullRepo(local, false)
	require.NoError(t, err)

	// Verify the new file arrived
	data, err := os.ReadFile(filepath.Join(local, "new.txt"))
	require.NoError(t, err)
	assert.Equal(t, "new content", string(data))
}

func TestGitPullRepo_NoRemote(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644)
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")

	err := GitPullRepo(dir, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git pull failed")
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
