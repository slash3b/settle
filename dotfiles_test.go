package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath_Tilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home dir")
	}

	got := expandPath("~/foo/bar")
	want := filepath.Join(home, "foo/bar")
	assertEqualStr(t, got, want)
}

func TestExpandPath_NoTilde(t *testing.T) {
	got := expandPath("/absolute/path")
	assertEqualStr(t, got, "/absolute/path")
}

func TestExpandPath_TildeHomeError(t *testing.T) {
	// Unset HOME to trigger UserHomeDir error
	oldHome := os.Getenv("HOME")
	os.Unsetenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

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
	assertEqualStr(t, got, "~")
}

func TestExpandPath_RelativePath(t *testing.T) {
	got := expandPath("relative/path")
	assertEqualStr(t, got, "relative/path")
}

func TestFilesEqual_Same(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	os.WriteFile(f1, []byte("hello world"), 0o644)
	os.WriteFile(f2, []byte("hello world"), 0o644)

	equal, err := filesEqual(f1, f2)
	assertNoError(t, err)
	assertEqualBool(t, equal, true)
}

func TestFilesEqual_Different(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	os.WriteFile(f1, []byte("hello"), 0o644)
	os.WriteFile(f2, []byte("world"), 0o644)

	equal, err := filesEqual(f1, f2)
	assertNoError(t, err)
	assertEqualBool(t, equal, false)
}

func TestFilesEqual_MissingFile(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	os.WriteFile(f1, []byte("hello"), 0o644)

	_, err := filesEqual(f1, filepath.Join(dir, "nonexistent"))
	assertError(t, err)
}

func TestFilesEqual_MissingSrc(t *testing.T) {
	dir := t.TempDir()
	f2 := filepath.Join(dir, "b.txt")
	os.WriteFile(f2, []byte("hello"), 0o644)

	_, err := filesEqual(filepath.Join(dir, "nonexistent"), f2)
	assertError(t, err)
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dest := filepath.Join(dir, "dest.txt")

	os.WriteFile(src, []byte("file contents"), 0o755)

	err := copyFile(src, dest)
	assertNoError(t, err)

	// Check content
	data, err := os.ReadFile(dest)
	assertNoError(t, err)
	assertEqualStr(t, string(data), "file contents")

	// Check permissions
	srcInfo, _ := os.Stat(src)
	destInfo, _ := os.Stat(dest)
	if srcInfo.Mode() != destInfo.Mode() {
		t.Errorf("permissions differ: src=%v dest=%v", srcInfo.Mode(), destInfo.Mode())
	}
}

func TestCopyFile_SrcNotExist(t *testing.T) {
	dir := t.TempDir()
	err := copyFile(filepath.Join(dir, "nonexistent"), filepath.Join(dir, "dest"))
	assertError(t, err)
}

func TestCheckLink_Missing(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	// Create source file
	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: filepath.Join(dir, "nonexistent")})
	assertNoError(t, err)
	if status != LinkMissing {
		t.Errorf("expected LinkMissing, got %d", status)
	}
}

func TestCheckLink_CorrectSymlink(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.Symlink(srcFile, destFile)

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile})
	assertNoError(t, err)
	if status != LinkCorrect {
		t.Errorf("expected LinkCorrect, got %d", status)
	}
}

func TestCheckLink_IncorrectSymlink(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.Symlink("/wrong/target", destFile)

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile})
	assertNoError(t, err)
	if status != LinkIncorrect {
		t.Errorf("expected LinkIncorrect, got %d", status)
	}
}

func TestCheckLink_RegularFile(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.WriteFile(destFile, []byte("existing file"), 0o644)

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile})
	assertNoError(t, err)
	if status != LinkIsFile {
		t.Errorf("expected LinkIsFile, got %d", status)
	}
}

func TestCheckLink_Directory(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destDir := filepath.Join(dir, ".vimrc")
	os.MkdirAll(destDir, 0o755)

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destDir})
	assertNoError(t, err)
	if status != LinkIsDir {
		t.Errorf("expected LinkIsDir, got %d", status)
	}
}

func TestCheckLink_SourceMissing(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.CheckLink(Dotfile{Src: "nonexistent", Dest: filepath.Join(dir, ".vimrc")})
	assertError(t, err)
	assertContains(t, err.Error(), "source file does not exist")
}

func TestCheckLink_CopyCorrect(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	content := "same content"
	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte(content), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.WriteFile(destFile, []byte(content), 0o644)

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"})
	assertNoError(t, err)
	if status != CopyCorrect {
		t.Errorf("expected CopyCorrect, got %d", status)
	}
}

func TestCheckLink_CopyOutdated(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("new content"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.WriteFile(destFile, []byte("old content"), 0o644)

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"})
	assertNoError(t, err)
	if status != CopyOutdated {
		t.Errorf("expected CopyOutdated, got %d", status)
	}
}

func TestCheckLink_CopyDirAtDest(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	destDir := filepath.Join(dir, ".vimrc")
	os.MkdirAll(destDir, 0o755)

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destDir, Mode: "copy"})
	assertNoError(t, err)
	if status != LinkIsDir {
		t.Errorf("expected LinkIsDir for copy mode with dir at dest, got %d", status)
	}
}

func TestCheckLink_CopySymlinkAtDest(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	// Create a symlink at dest in copy mode
	os.Symlink(srcFile, destFile)

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"})
	assertNoError(t, err)
	// Symlink is not a regular file, so should be CopyOutdated
	if status != CopyOutdated {
		t.Errorf("expected CopyOutdated for symlink in copy mode, got %d", status)
	}
}

func TestApply_LinkMissing(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destFile := filepath.Join(dir, "config", ".vimrc")

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	assertNoError(t, err)
	assertEqualBool(t, created, true)

	// Verify symlink was created
	target, err := os.Readlink(destFile)
	assertNoError(t, err)
	assertEqualStr(t, target, srcFile)
}

func TestApply_LinkCorrect(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.Symlink(srcFile, destFile)

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	assertNoError(t, err)
	assertEqualBool(t, created, false)
}

func TestApply_LinkIncorrect(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.Symlink("/wrong/target", destFile)

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	assertNoError(t, err)
	assertEqualBool(t, created, true)

	// Verify symlink was updated
	target, err := os.Readlink(destFile)
	assertNoError(t, err)
	assertEqualStr(t, target, srcFile)
}

func TestApply_LinkIsFile(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.WriteFile(destFile, []byte("old content"), 0o644)

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	assertNoError(t, err)
	assertEqualBool(t, created, true)

	// Verify backup was created
	_, err = os.Stat(destFile + ".backup")
	assertNoError(t, err)

	// Verify symlink points to source
	target, err := os.Readlink(destFile)
	assertNoError(t, err)
	assertEqualStr(t, target, srcFile)
}

func TestApply_LinkIsFile_Verbose(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.WriteFile(destFile, []byte("old content"), 0o644)

	dm := NewDotfilesManager(srcDir, true)
	out := captureOutput(t, func() {
		created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
		assertNoError(t, err)
		assertEqualBool(t, created, true)
	})

	assertContains(t, out, "backed up")
}

func TestApply_LinkIsDir(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destDir := filepath.Join(dir, ".vimrc")
	os.MkdirAll(destDir, 0o755)

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destDir}, false)
	assertError(t, err)
	assertContains(t, err.Error(), "directory exists")
}

func TestApply_CopyMissing(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("set nocompatible"), 0o644)

	destFile := filepath.Join(dir, "config", ".vimrc")

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, false)
	assertNoError(t, err)
	assertEqualBool(t, created, true)

	// Verify file was copied (not symlinked)
	info, _ := os.Lstat(destFile)
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("expected regular file, got symlink")
	}
	data, _ := os.ReadFile(destFile)
	assertEqualStr(t, string(data), "set nocompatible")
}

func TestApply_CopyCorrect(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	content := "same content"
	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte(content), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.WriteFile(destFile, []byte(content), 0o644)

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, false)
	assertNoError(t, err)
	assertEqualBool(t, created, false)
}

func TestApply_CopyOutdated(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("new content"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.WriteFile(destFile, []byte("old content"), 0o644)

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, false)
	assertNoError(t, err)
	assertEqualBool(t, created, true)

	data, _ := os.ReadFile(destFile)
	assertEqualStr(t, string(data), "new content")
}

func TestApply_CopyDirAtDest(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	destDir := filepath.Join(dir, ".vimrc")
	os.MkdirAll(destDir, 0o755)

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destDir, Mode: "copy"}, false)
	assertError(t, err)
	assertContains(t, err.Error(), "directory exists")
}

func TestApply_DryRun_Link(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, true)
	assertNoError(t, err)
	assertEqualBool(t, created, true)

	// Verify no symlink was created
	_, err = os.Lstat(destFile)
	if !os.IsNotExist(err) {
		t.Error("expected no file to be created in dry-run mode")
	}
}

func TestApply_DryRun_Copy(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, true)
	assertNoError(t, err)
	assertEqualBool(t, created, true)

	// Verify no file was created
	_, err = os.Lstat(destFile)
	if !os.IsNotExist(err) {
		t.Error("expected no file to be created in dry-run mode")
	}
}

func TestApply_DryRun_LinkIncorrect(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.Symlink("/wrong/target", destFile)

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, true)
	assertNoError(t, err)
	assertEqualBool(t, created, true)

	// Verify symlink was NOT changed
	target, _ := os.Readlink(destFile)
	assertEqualStr(t, target, "/wrong/target")
}

func TestApply_DryRun_LinkIsFile(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.WriteFile(destFile, []byte("existing"), 0o644)

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, true)
	assertNoError(t, err)
	assertEqualBool(t, created, true)

	// Verify file was NOT changed
	data, _ := os.ReadFile(destFile)
	assertEqualStr(t, string(data), "existing")
}

func TestApply_DryRun_CopyOutdated(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("new"), 0o644)

	destFile := filepath.Join(dir, ".vimrc")
	os.WriteFile(destFile, []byte("old"), 0o644)

	dm := NewDotfilesManager(srcDir, false)
	created, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, true)
	assertNoError(t, err)
	assertEqualBool(t, created, true)

	// Verify file was NOT changed
	data, _ := os.ReadFile(destFile)
	assertEqualStr(t, string(data), "old")
}

// --- Error paths ---

func TestCheckLink_LstatError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	// Create a parent dir that we'll make unreadable
	parentDir := filepath.Join(dir, "noperm")
	os.MkdirAll(parentDir, 0o755)
	destFile := filepath.Join(parentDir, ".vimrc")
	os.WriteFile(destFile, []byte("x"), 0o644)
	// Remove execute permission from parent so Lstat fails
	os.Chmod(parentDir, 0o000)
	t.Cleanup(func() { os.Chmod(parentDir, 0o755) })

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile})
	assertError(t, err)
}

func TestCheckLink_FilesEqualError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	// Create dest file that is unreadable
	destFile := filepath.Join(dir, ".vimrc")
	os.WriteFile(destFile, []byte("other"), 0o000)
	t.Cleanup(func() { os.Chmod(destFile, 0o644) })

	dm := NewDotfilesManager(srcDir, false)
	status, err := dm.CheckLink(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"})
	// filesEqual should fail reading unreadable dest file
	if status != CopyOutdated {
		t.Errorf("expected CopyOutdated, got %d", status)
	}
	assertError(t, err)
}

func TestApply_LinkMissing_MkdirAllError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	// Dest in an unwritable directory
	parentDir := filepath.Join(dir, "noperm")
	os.MkdirAll(parentDir, 0o755)
	os.Chmod(parentDir, 0o555)
	t.Cleanup(func() { os.Chmod(parentDir, 0o755) })

	destFile := filepath.Join(parentDir, "subdir", ".vimrc")

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	assertError(t, err)
	assertContains(t, err.Error(), "failed to create directory")
}

func TestApply_LinkMissing_SymlinkError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	// Dest dir exists but is read-only
	parentDir := filepath.Join(dir, "readonly")
	os.MkdirAll(parentDir, 0o755)

	destFile := filepath.Join(parentDir, ".vimrc")

	// Make parent read-only after creating it
	os.Chmod(parentDir, 0o555)
	t.Cleanup(func() { os.Chmod(parentDir, 0o755) })

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	assertError(t, err)
	assertContains(t, err.Error(), "failed to create symlink")
}

func TestApply_LinkIncorrect_RemoveError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	// Create incorrect symlink in a dir that we'll make read-only
	parentDir := filepath.Join(dir, "readonly")
	os.MkdirAll(parentDir, 0o755)
	destFile := filepath.Join(parentDir, ".vimrc")
	os.Symlink("/wrong/target", destFile)

	// Make parent dir read-only so Remove fails
	os.Chmod(parentDir, 0o555)
	t.Cleanup(func() { os.Chmod(parentDir, 0o755) })

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	assertError(t, err)
	assertContains(t, err.Error(), "failed to remove old symlink")
}

func TestApply_LinkIsFile_RenameError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	// File at dest in a dir we'll make read-only
	parentDir := filepath.Join(dir, "readonly")
	os.MkdirAll(parentDir, 0o755)
	destFile := filepath.Join(parentDir, ".vimrc")
	os.WriteFile(destFile, []byte("existing"), 0o644)

	// Make parent read-only so Rename fails
	os.Chmod(parentDir, 0o555)
	t.Cleanup(func() { os.Chmod(parentDir, 0o755) })

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile}, false)
	assertError(t, err)
	assertContains(t, err.Error(), "failed to backup existing file")
}

func TestApply_CopyMissing_MkdirAllError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	// Read-only parent
	parentDir := filepath.Join(dir, "noperm")
	os.MkdirAll(parentDir, 0o555)
	t.Cleanup(func() { os.Chmod(parentDir, 0o755) })

	destFile := filepath.Join(parentDir, "subdir", ".vimrc")

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, false)
	assertError(t, err)
	assertContains(t, err.Error(), "failed to create directory")
}

func TestApply_CopyMissing_CopyFileError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("content"), 0o644)

	// Read-only dest dir (dir exists so MkdirAll succeeds, but can't write)
	parentDir := filepath.Join(dir, "readonly")
	os.MkdirAll(parentDir, 0o555)
	t.Cleanup(func() { os.Chmod(parentDir, 0o755) })

	destFile := filepath.Join(parentDir, ".vimrc")

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, false)
	assertError(t, err)
	assertContains(t, err.Error(), "failed to copy file")
}

func TestApply_CopyOutdated_CopyFileError(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	srcFile := filepath.Join(srcDir, "vimrc")
	os.WriteFile(srcFile, []byte("new content"), 0o644)

	// Existing file that we make unwritable
	destFile := filepath.Join(dir, ".vimrc")
	os.WriteFile(destFile, []byte("old content"), 0o444)
	t.Cleanup(func() { os.Chmod(destFile, 0o644) })

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "vimrc", Dest: destFile, Mode: "copy"}, false)
	assertError(t, err)
	assertContains(t, err.Error(), "failed to update copy")
}

func TestCopyFile_DestOpenError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	os.WriteFile(src, []byte("content"), 0o644)

	// Dest in unwritable dir
	parentDir := filepath.Join(dir, "readonly")
	os.MkdirAll(parentDir, 0o555)
	t.Cleanup(func() { os.Chmod(parentDir, 0o755) })

	dest := filepath.Join(parentDir, "dest.txt")
	err := copyFile(src, dest)
	assertError(t, err)
}


func TestApply_SourceMissing(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "sources")
	os.MkdirAll(srcDir, 0o755)

	dm := NewDotfilesManager(srcDir, false)
	_, err := dm.Apply(Dotfile{Src: "nonexistent", Dest: filepath.Join(dir, ".vimrc")}, false)
	assertError(t, err)
	assertContains(t, err.Error(), "source file does not exist")
}
