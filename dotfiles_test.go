package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandPath_Tilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home dir")
	}

	got := expandPath("~/foo/bar")
	want := filepath.Join(home, "foo/bar")
	assert.Equal(t, want, got)
}

func TestExpandPath_NoTilde(t *testing.T) {
	got := expandPath("/absolute/path")
	assert.Equal(t, "/absolute/path", got)
}

func TestExpandPath_TildeHomeError(t *testing.T) {
	// Unset HOME to trigger UserHomeDir error
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Unsetenv("HOME"))
	t.Cleanup(func() { require.NoError(t, os.Setenv("HOME", oldHome)) })

	got := expandPath("~/foo")
	// When HOME is unset, UserHomeDir may fail, returning path unchanged
	// OR it may still work via /etc/passwd. Either way, no panic.
	if got != "~/foo" {
		// If it resolved, that's fine too (passwd lookup)
		t.Logf("expandPath(~/foo) with no HOME = %s", got)
	}
}

func TestExpandPath_TildeOnly(t *testing.T) {
	// "~" without "/" should not be expanded
	got := expandPath("~")
	assert.Equal(t, "~", got)
}

func TestExpandPath_RelativePath(t *testing.T) {
	got := expandPath("relative/path")
	assert.Equal(t, "relative/path", got)
}

func TestFilesEqual_Same(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(f1, []byte("hello world"), 0o644))
	require.NoError(t, os.WriteFile(f2, []byte("hello world"), 0o644))

	equal, err := filesEqual(f1, f2)
	require.NoError(t, err)
	assert.Equal(t, true, equal)
}

func TestFilesEqual_Different(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(f1, []byte("hello"), 0o644))
	require.NoError(t, os.WriteFile(f2, []byte("world"), 0o644))

	equal, err := filesEqual(f1, f2)
	require.NoError(t, err)
	assert.Equal(t, false, equal)
}

func TestFilesEqual_MissingFile(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(f1, []byte("hello"), 0o644))

	_, err := filesEqual(f1, filepath.Join(dir, "nonexistent"))
	require.Error(t, err)
}

func TestFilesEqual_MissingSrc(t *testing.T) {
	dir := t.TempDir()
	f2 := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(f2, []byte("hello"), 0o644))

	_, err := filesEqual(filepath.Join(dir, "nonexistent"), f2)
	require.Error(t, err)
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dest := filepath.Join(dir, "dest.txt")

	require.NoError(t, os.WriteFile(src, []byte("file contents"), 0o755))

	err := copyFile(src, dest)
	require.NoError(t, err)

	// Check content
	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "file contents", string(data))

	// Check permissions
	srcInfo, _ := os.Stat(src)
	destInfo, _ := os.Stat(dest)
	assert.Equal(t, srcInfo.Mode(), destInfo.Mode())
}

func TestCopyFile_SrcNotExist(t *testing.T) {
	dir := t.TempDir()
	err := copyFile(filepath.Join(dir, "nonexistent"), filepath.Join(dir, "dest"))
	require.Error(t, err)
}

func TestCheckLink_Missing(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	// Create source file
	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: filepath.Join(dir, "nonexistent")})
	require.NoError(t, err)
	assert.Equal(t, LinkMissing, status)
}

func TestCheckLink_CorrectSymlink(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.Symlink(srcFile, destFile))

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile})
	require.NoError(t, err)
	assert.Equal(t, LinkCorrect, status)
}

func TestCheckLink_IncorrectSymlink(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.Symlink("/wrong/target", destFile))

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile})
	require.NoError(t, err)
	assert.Equal(t, LinkIncorrect, status)
}

func TestCheckLink_RegularFile(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.WriteFile(destFile, []byte("existing file"), 0o644))

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile})
	require.NoError(t, err)
	assert.Equal(t, LinkIsFile, status)
}

func TestCheckLink_Directory(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destDir := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destDir})
	require.NoError(t, err)
	assert.Equal(t, LinkIsDir, status)
}

func TestCheckLink_SourceMissing(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.CheckLink(Dotfile{Src: "nonexistent", Dest: filepath.Join(dir, ".vimrc")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source file does not exist")
}

func TestCheckLink_CopyCorrect(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	content := "same content"
	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte(content), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.WriteFile(destFile, []byte(content), 0o644))

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"})
	require.NoError(t, err)
	assert.Equal(t, CopyCorrect, status)
}

func TestCheckLink_CopyOutdated(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("new content"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.WriteFile(destFile, []byte("old content"), 0o644))

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"})
	require.NoError(t, err)
	assert.Equal(t, CopyOutdated, status)
}

func TestCheckLink_CopyDirAtDest(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	destDir := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destDir, Mode: "copy"})
	require.NoError(t, err)
	assert.Equal(t, LinkIsDir, status)
}

func TestCheckLink_CopySymlinkAtDest(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	// Create a symlink at dest in copy mode
	require.NoError(t, os.Symlink(srcFile, destFile))

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"})
	require.NoError(t, err)
	// Symlink is not a regular file, so should be CopyOutdated
	assert.Equal(t, CopyOutdated, status)
}

func TestApply_LinkMissing(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destFile := filepath.Join(dir, "config", ".vimrc")

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	require.NoError(t, err)
	assert.Equal(t, true, created)

	// Verify symlink was created
	target, err := os.Readlink(destFile)
	require.NoError(t, err)
	assert.Equal(t, srcFile, target)
}

func TestApply_LinkCorrect(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.Symlink(srcFile, destFile))

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	require.NoError(t, err)
	assert.Equal(t, false, created)
}

func TestApply_LinkIncorrect(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.Symlink("/wrong/target", destFile))

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	require.NoError(t, err)
	assert.Equal(t, true, created)

	// Verify symlink was updated
	target, err := os.Readlink(destFile)
	require.NoError(t, err)
	assert.Equal(t, srcFile, target)
}

func TestApply_LinkIsFile(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.WriteFile(destFile, []byte("old content"), 0o644))

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	require.NoError(t, err)
	assert.Equal(t, true, created)

	// Verify backup was created
	_, err = os.Stat(destFile + ".backup")
	require.NoError(t, err)

	// Verify symlink points to source
	target, err := os.Readlink(destFile)
	require.NoError(t, err)
	assert.Equal(t, srcFile, target)
}

func TestApply_LinkIsFile_Verbose(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.WriteFile(destFile, []byte("old content"), 0o644))

	dm := NewDotfilesManager(srcDir, true)
	out := captureOutput(t, func() {
		created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
		require.NoError(t, err)
		assert.Equal(t, true, created)
	})

	assert.Contains(t, out, "backed up")
}

func TestApply_LinkIsDir(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destDir := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destDir}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory exists")
}

func TestApply_CopyMissing(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("set nocompatible"), 0o644))

	destFile := filepath.Join(dir, "config", ".vimrc")

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, false)
	require.NoError(t, err)
	assert.Equal(t, true, created)

	// Verify file was copied (not symlinked)
	info, _ := os.Lstat(destFile)
	assert.Zero(t, info.Mode()&os.ModeSymlink, "expected regular file, got symlink")
	data, _ := os.ReadFile(destFile)
	assert.Equal(t, "set nocompatible", string(data))
}

func TestApply_CopyCorrect(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	content := "same content"
	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte(content), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.WriteFile(destFile, []byte(content), 0o644))

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, false)
	require.NoError(t, err)
	assert.Equal(t, false, created)
}

func TestApply_CopyOutdated(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("new content"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.WriteFile(destFile, []byte("old content"), 0o644))

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, false)
	require.NoError(t, err)
	assert.Equal(t, true, created)

	data, _ := os.ReadFile(destFile)
	assert.Equal(t, "new content", string(data))
}

func TestApply_CopyDirAtDest(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	destDir := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destDir, Mode: "copy"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory exists")
}

func TestApply_DryRun_Link(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, true)
	require.NoError(t, err)
	assert.Equal(t, true, created)

	// Verify no symlink was created
	_, err = os.Lstat(destFile)
	assert.True(t, os.IsNotExist(err), "expected no file to be created in dry-run mode")
}

func TestApply_DryRun_Copy(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, true)
	require.NoError(t, err)
	assert.Equal(t, true, created)

	// Verify no file was created
	_, err = os.Lstat(destFile)
	assert.True(t, os.IsNotExist(err), "expected no file to be created in dry-run mode")
}

func TestApply_DryRun_LinkIncorrect(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.Symlink("/wrong/target", destFile))

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, true)
	require.NoError(t, err)
	assert.Equal(t, true, created)

	// Verify symlink was NOT changed
	target, _ := os.Readlink(destFile)
	assert.Equal(t, "/wrong/target", target)
}

func TestApply_DryRun_LinkIsFile(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.WriteFile(destFile, []byte("existing"), 0o644))

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, true)
	require.NoError(t, err)
	assert.Equal(t, true, created)

	// Verify file was NOT changed
	data, _ := os.ReadFile(destFile)
	assert.Equal(t, "existing", string(data))
}

func TestApply_DryRun_CopyOutdated(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("new"), 0o644))

	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.WriteFile(destFile, []byte("old"), 0o644))

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, true)
	require.NoError(t, err)
	assert.Equal(t, true, created)

	// Verify file was NOT changed
	data, _ := os.ReadFile(destFile)
	assert.Equal(t, "old", string(data))
}

// --- Error paths ---

func TestCheckLink_LstatError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	// Create a parent dir that we'll make unreadable
	parentDir := filepath.Join(dir, "noperm")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))
	destFile := filepath.Join(parentDir, ".vimrc")
	require.NoError(t, os.WriteFile(destFile, []byte("x"), 0o644))
	// Remove execute permission from parent so Lstat fails
	require.NoError(t, os.Chmod(parentDir, 0o000))
	t.Cleanup(func() { require.NoError(t, os.Chmod(parentDir, 0o755)) })

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile})
	require.Error(t, err)
}

func TestCheckLink_FilesEqualError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	// Create dest file that is unreadable
	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.WriteFile(destFile, []byte("other"), 0o000))
	t.Cleanup(func() { require.NoError(t, os.Chmod(destFile, 0o644)) })

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"})
	// filesEqual should fail reading unreadable dest file
	assert.Equal(t, CopyOutdated, status)
	require.Error(t, err)
}

func TestApply_LinkMissing_MkdirAllError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	// Dest in an unwritable directory
	parentDir := filepath.Join(dir, "noperm")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))
	require.NoError(t, os.Chmod(parentDir, 0o555))
	t.Cleanup(func() { require.NoError(t, os.Chmod(parentDir, 0o755)) })

	destFile := filepath.Join(parentDir, "subdir", ".vimrc")

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create directory")
}

func TestApply_LinkMissing_SymlinkError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	// Dest dir exists but is read-only
	parentDir := filepath.Join(dir, "readonly")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))

	destFile := filepath.Join(parentDir, ".vimrc")

	// Make parent read-only after creating it
	require.NoError(t, os.Chmod(parentDir, 0o555))
	t.Cleanup(func() { require.NoError(t, os.Chmod(parentDir, 0o755)) })

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create symlink")
}

func TestApply_LinkIncorrect_RemoveError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	// Create incorrect symlink in a dir that we'll make read-only
	parentDir := filepath.Join(dir, "readonly")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))
	destFile := filepath.Join(parentDir, ".vimrc")
	require.NoError(t, os.Symlink("/wrong/target", destFile))

	// Make parent dir read-only so Remove fails
	require.NoError(t, os.Chmod(parentDir, 0o555))
	t.Cleanup(func() { require.NoError(t, os.Chmod(parentDir, 0o755)) })

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove old symlink")
}

func TestApply_LinkIsFile_RenameError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	// File at dest in a dir we'll make read-only
	parentDir := filepath.Join(dir, "readonly")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))
	destFile := filepath.Join(parentDir, ".vimrc")
	require.NoError(t, os.WriteFile(destFile, []byte("existing"), 0o644))

	// Make parent read-only so Rename fails
	require.NoError(t, os.Chmod(parentDir, 0o555))
	t.Cleanup(func() { require.NoError(t, os.Chmod(parentDir, 0o755)) })

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to backup existing file")
}

func TestApply_CopyMissing_MkdirAllError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	// Read-only parent
	parentDir := filepath.Join(dir, "noperm")
	require.NoError(t, os.MkdirAll(parentDir, 0o555))
	t.Cleanup(func() { require.NoError(t, os.Chmod(parentDir, 0o755)) })

	destFile := filepath.Join(parentDir, "subdir", ".vimrc")

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create directory")
}

func TestApply_CopyMissing_CopyFileError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	// Read-only dest dir (dir exists so MkdirAll succeeds, but can't write)
	parentDir := filepath.Join(dir, "readonly")
	require.NoError(t, os.MkdirAll(parentDir, 0o555))
	t.Cleanup(func() { require.NoError(t, os.Chmod(parentDir, 0o755)) })

	destFile := filepath.Join(parentDir, ".vimrc")

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to copy file")
}

func TestApply_CopyOutdated_CopyFileError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "vimrc")
	require.NoError(t, os.WriteFile(srcFile, []byte("new content"), 0o644))

	// Existing file that we make unwritable
	destFile := filepath.Join(dir, ".vimrc")
	require.NoError(t, os.WriteFile(destFile, []byte("old content"), 0o444))
	t.Cleanup(func() { require.NoError(t, os.Chmod(destFile, 0o644)) })

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update copy")
}

func TestCopyFile_DestOpenError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	require.NoError(t, os.WriteFile(src, []byte("content"), 0o644))

	// Dest in unwritable dir
	parentDir := filepath.Join(dir, "readonly")
	require.NoError(t, os.MkdirAll(parentDir, 0o555))
	t.Cleanup(func() { require.NoError(t, os.Chmod(parentDir, 0o755)) })

	dest := filepath.Join(parentDir, "dest.txt")
	err := copyFile(src, dest)
	require.Error(t, err)
}

func TestApply_Executable_Link(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "postswitch")
	require.NoError(t, os.WriteFile(srcFile, []byte("#!/bin/sh\necho hello"), 0o644))

	destFile := filepath.Join(dir, "postswitch")

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "postswitch", Dest: destFile, Executable: true}, false)
	require.NoError(t, err)
	assert.True(t, created)

	// os.Chmod follows the symlink, so the source file should have the executable bit
	info, err := os.Stat(srcFile)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o111, "expected source file to be executable")
}

func TestApply_Executable_Copy(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "postswitch")
	require.NoError(t, os.WriteFile(srcFile, []byte("#!/bin/sh\necho hello"), 0o644))

	destFile := filepath.Join(dir, "postswitch")

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "postswitch", Dest: destFile, Mode: "copy", Executable: true}, false)
	require.NoError(t, err)
	assert.True(t, created)

	info, err := os.Stat(destFile)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o111, "expected dest file to be executable")
}

func TestApply_Executable_DryRun(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "postswitch")
	require.NoError(t, os.WriteFile(srcFile, []byte("#!/bin/sh\necho hello"), 0o644))

	destFile := filepath.Join(dir, "postswitch")

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "postswitch", Dest: destFile, Executable: true}, true)
	require.NoError(t, err)
	assert.True(t, created)

	// No file should be created in dry-run
	_, err = os.Lstat(destFile)
	assert.True(t, os.IsNotExist(err))

	// Source should NOT have been chmoded
	info, err := os.Stat(srcFile)
	require.NoError(t, err)
	assert.Zero(t, info.Mode()&0o111, "expected source NOT to be executable in dry-run")
}

func TestApply_SourceMissing(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "nonexistent", Dest: filepath.Join(dir, ".vimrc")}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source file does not exist")
}

func TestApply_Sudo_LinkMissing(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "20-amdgpu.conf")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	destFile := filepath.Join(dir, "etc", "20-amdgpu.conf")

	var calls []cmdCall
	execCommand = func(name string, arg ...string) *exec.Cmd {
		calls = append(calls, cmdCall{Name: name, Args: arg})
		return exec.Command("true")
	}

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "20-amdgpu.conf", Dest: destFile, Sudo: true}, false)
	require.NoError(t, err)
	assert.True(t, created)

	require.Len(t, calls, 2)
	assert.Equal(t, "sudo", calls[0].Name)
	assert.Equal(t, []string{"mkdir", "-p", filepath.Dir(destFile)}, calls[0].Args)
	assert.Equal(t, "sudo", calls[1].Name)
	assert.Equal(t, []string{"ln", "-sf", srcFile, destFile}, calls[1].Args)
}

func TestApply_Sudo_CopyMissing(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	srcFile := filepath.Join(srcDir, "20-amdgpu.conf")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0o644))

	destFile := filepath.Join(dir, "etc", "20-amdgpu.conf")

	var calls []cmdCall
	execCommand = func(name string, arg ...string) *exec.Cmd {
		calls = append(calls, cmdCall{Name: name, Args: arg})
		return exec.Command("true")
	}

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "20-amdgpu.conf", Dest: destFile, Mode: "copy", Sudo: true}, false)
	require.NoError(t, err)
	assert.True(t, created)

	require.Len(t, calls, 2)
	assert.Equal(t, "sudo", calls[0].Name)
	assert.Equal(t, []string{"mkdir", "-p", filepath.Dir(destFile)}, calls[0].Args)
	assert.Equal(t, "sudo", calls[1].Name)
	assert.Equal(t, []string{"cp", srcFile, destFile}, calls[1].Args)
}

func TestCheckDir_Missing(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, "autorandr"), 0o755))

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckDir(DotfileDir{Src: "autorandr", Dest: filepath.Join(dir, "nonexistent")})
	require.NoError(t, err)
	assert.Equal(t, LinkMissing, status)
}

func TestCheckDir_Correct(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	srcSubDir := filepath.Join(srcDir, "autorandr")
	require.NoError(t, os.MkdirAll(srcSubDir, 0o755))

	dest := filepath.Join(dir, "autorandr-link")
	require.NoError(t, os.Symlink(srcSubDir, dest))

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckDir(DotfileDir{Src: "autorandr", Dest: dest})
	require.NoError(t, err)
	assert.Equal(t, LinkCorrect, status)
}

func TestApplyDir_CreatesMissing(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	srcSubDir := filepath.Join(srcDir, "autorandr")
	require.NoError(t, os.MkdirAll(srcSubDir, 0o755))

	dest := filepath.Join(dir, "autorandr-link")

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.ApplyDir(DotfileDir{Src: "autorandr", Dest: dest}, false)
	require.NoError(t, err)
	assert.True(t, created)

	target, err := os.Readlink(dest)
	require.NoError(t, err)
	assert.Equal(t, srcSubDir, target)
}

func TestApplyDir_AlreadyCorrect(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	srcSubDir := filepath.Join(srcDir, "autorandr")
	require.NoError(t, os.MkdirAll(srcSubDir, 0o755))

	dest := filepath.Join(dir, "autorandr-link")
	require.NoError(t, os.Symlink(srcSubDir, dest))

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.ApplyDir(DotfileDir{Src: "autorandr", Dest: dest}, false)
	require.NoError(t, err)
	assert.False(t, created)
}

func TestApplyDir_RealDirExistsBacksUpAndSymlinks(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	srcSubDir := filepath.Join(srcDir, "autorandr")
	require.NoError(t, os.MkdirAll(srcSubDir, 0o755))

	// Existing real directory at destination with a file inside
	dest := filepath.Join(dir, "autorandr")
	require.NoError(t, os.MkdirAll(dest, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dest, "config"), []byte("old"), 0o644))

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.ApplyDir(DotfileDir{Src: "autorandr", Dest: dest}, false)
	require.NoError(t, err)
	assert.True(t, created)

	// dest is now a symlink to src
	target, err := os.Readlink(dest)
	require.NoError(t, err)
	assert.Equal(t, srcSubDir, target)

	// backup was created
	_, err = os.Stat(dest + ".backup")
	require.NoError(t, err)
}

func TestApplyDir_DryRun(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, "autorandr"), 0o755))

	dest := filepath.Join(dir, "autorandr-link")

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.ApplyDir(DotfileDir{Src: "autorandr", Dest: dest}, true)
	require.NoError(t, err)
	assert.True(t, created)

	// Dry run: symlink must not exist
	_, err = os.Lstat(dest)
	assert.True(t, os.IsNotExist(err))
}

func TestApply_Sudo_DryRun(t *testing.T) {
	saveMocks(t)

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "20-amdgpu.conf"), []byte("content"), 0o644))

	destFile := filepath.Join(dir, "etc", "20-amdgpu.conf")

	var calls []cmdCall
	execCommand = func(name string, arg ...string) *exec.Cmd {
		calls = append(calls, cmdCall{Name: name, Args: arg})
		return exec.Command("true")
	}

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "20-amdgpu.conf", Dest: destFile, Sudo: true}, true)
	require.NoError(t, err)
	assert.True(t, created)

	// No sudo commands should be issued in dry-run
	assert.Empty(t, calls)
}
