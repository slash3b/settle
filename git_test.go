package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- GitClone tests ---

func TestGitClone_Success(t *testing.T) {
	// Create a bare repo as the "remote"
	remote := t.TempDir()
	run(t, remote, "init", "--bare")

	// Create a temp clone, add a commit, push — so the bare repo has content
	scratch := t.TempDir()
	run(t, scratch, "clone", remote, "work")
	work := filepath.Join(scratch, "work")
	run(t, work, "config", "user.email", "test@test.com")
	run(t, work, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(work, "README.md"), []byte("hello"), 0o644)) //nolint:gosec
	run(t, work, "add", ".")
	run(t, work, "commit", "-m", "init")
	run(t, work, "push")

	// Now clone from our bare remote using GitClone
	dest := filepath.Join(t.TempDir(), "cloned")
	err := GitClone(remote, dest, false)
	require.NoError(t, err)

	// Verify .git exists
	_, err = os.Stat(filepath.Join(dest, ".git"))
	require.NoError(t, err)

	// Verify the file arrived
	data, err := os.ReadFile(filepath.Join(dest, "README.md")) //nolint:gosec
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
	run(t, remote, "init", "--bare")

	scratch := t.TempDir()
	run(t, scratch, "clone", remote, "work")
	work := filepath.Join(scratch, "work")
	run(t, work, "config", "user.email", "test@test.com")
	run(t, work, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(work, "README.md"), []byte("hello"), 0o644)) //nolint:gosec
	run(t, work, "add", ".")
	run(t, work, "commit", "-m", "init")
	run(t, work, "push")

	// Create destination directory that already exists with a file in it
	dest := filepath.Join(t.TempDir(), "cloned")
	require.NoError(t, os.MkdirAll(dest, 0o755))                                                    //nolint:gosec
	require.NoError(t, os.WriteFile(filepath.Join(dest, "blocker.txt"), []byte("occupied"), 0o644)) //nolint:gosec

	// git clone should fail because dest is non-empty
	err := GitClone(remote, dest, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git clone failed")
}

// --- GitPullRepo tests ---

func TestGitPullRepo_Success(t *testing.T) {
	// Create "remote" repo
	remote := t.TempDir()
	run(t, remote, "init", "--bare")

	// Clone to local
	parent := t.TempDir()
	local := filepath.Join(parent, "repo")
	run(t, parent, "clone", remote, local)
	run(t, local, "config", "user.email", "test@test.com")
	run(t, local, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(local, "file.txt"), []byte("initial"), 0o644)) //nolint:gosec
	run(t, local, "add", ".")
	run(t, local, "commit", "-m", "initial")
	run(t, local, "push", "-u", "origin", "HEAD")

	// Push a new commit from another clone
	other := filepath.Join(parent, "other")
	run(t, parent, "clone", remote, other)
	run(t, other, "config", "user.email", "test@test.com")
	run(t, other, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(other, "new.txt"), []byte("new content"), 0o644)) //nolint:gosec
	run(t, other, "add", ".")
	run(t, other, "commit", "-m", "add new file")
	run(t, other, "push")

	// Pull from local using GitPullRepo
	err := GitPullRepo(local, false)
	require.NoError(t, err)

	// Verify the new file arrived
	data, err := os.ReadFile(filepath.Join(local, "new.txt")) //nolint:gosec
	require.NoError(t, err)
	assert.Equal(t, "new content", string(data))
}

func TestGitPullRepo_NoRemote(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init")
	run(t, dir, "config", "user.email", "test@test.com")
	run(t, dir, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644)) //nolint:gosec
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-m", "init")

	err := GitPullRepo(dir, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git pull failed")
}

// run is a test helper that executes a git command in a directory.
func run(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
