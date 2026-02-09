package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ValidLinux(t *testing.T) {
	path := withTempConfig(t, `
[linux]
packages = ["vim", "curl", "git"]
`)
	cfg, err := loadConfig(path)
	assertNoError(t, err)

	if cfg.Linux == nil {
		t.Fatal("expected Linux config to be non-nil")
	}
	assertEqualInt(t, len(cfg.Linux.Packages), 3)
	assertEqualStr(t, cfg.Linux.Packages[0], "vim")
	assertEqualStr(t, cfg.Linux.Packages[1], "curl")
	assertEqualStr(t, cfg.Linux.Packages[2], "git")
}

func TestLoadConfig_ValidDotfiles(t *testing.T) {
	path := withTempConfig(t, `
[dotfiles]
source_dir = "~/dotfiles/sources"

[[dotfiles.file]]
src = "alacritty.toml"
dest = "~/.config/alacritty/alacritty.toml"

[[dotfiles.file]]
src = "tmux.conf"
dest = "~/.tmux.conf"
mode = "copy"
`)
	cfg, err := loadConfig(path)
	assertNoError(t, err)

	if cfg.Dotfiles == nil {
		t.Fatal("expected Dotfiles config to be non-nil")
	}
	assertEqualStr(t, cfg.Dotfiles.SourceDir, "~/dotfiles/sources")
	assertEqualInt(t, len(cfg.Dotfiles.Files), 2)
	assertEqualStr(t, cfg.Dotfiles.Files[0].Src, "alacritty.toml")
	assertEqualStr(t, cfg.Dotfiles.Files[0].Dest, "~/.config/alacritty/alacritty.toml")
	assertEqualStr(t, cfg.Dotfiles.Files[0].Mode, "")
	assertEqualStr(t, cfg.Dotfiles.Files[1].Mode, "copy")
}

func TestLoadConfig_PackageWithPostInstall(t *testing.T) {
	path := withTempConfig(t, `
[linux]
packages = ["curl"]

[[linux.package]]
name = "pipewire"
post_install = "systemctl --user --now enable wireplumber.service"
`)
	cfg, err := loadConfig(path)
	assertNoError(t, err)

	if cfg.Linux == nil {
		t.Fatal("expected Linux config to be non-nil")
	}
	assertEqualInt(t, len(cfg.Linux.Packages), 1)
	assertEqualInt(t, len(cfg.Linux.Package), 1)
	assertEqualStr(t, cfg.Linux.Package[0].Name, "pipewire")
	assertEqualStr(t, cfg.Linux.Package[0].PostInstall, "systemctl --user --now enable wireplumber.service")
}

func TestLoadConfig_Empty(t *testing.T) {
	path := withTempConfig(t, "")
	cfg, err := loadConfig(path)
	assertNoError(t, err)

	if cfg.Linux != nil {
		t.Error("expected Linux config to be nil for empty config")
	}
	if cfg.Dotfiles != nil {
		t.Error("expected Dotfiles config to be nil for empty config")
	}
}

func TestLoadConfig_NotFound(t *testing.T) {
	_, err := loadConfig("/nonexistent/path/config.toml")
	assertError(t, err)
	assertContains(t, err.Error(), "config file not found")
}

func TestLoadConfig_InvalidTOML(t *testing.T) {
	path := withTempConfig(t, `this is not valid toml [[[`)
	_, err := loadConfig(path)
	assertError(t, err)
	assertContains(t, err.Error(), "error parsing TOML")
}

func TestLoadConfig_FullConfig(t *testing.T) {
	path := withTempConfig(t, `
[linux]
packages = ["vim", "curl"]

[[linux.package]]
name = "pipewire"
post_install = "systemctl --user enable wireplumber"

[dotfiles]
source_dir = "~/dotfiles"

[[dotfiles.file]]
src = "vimrc"
dest = "~/.vimrc"
`)
	cfg, err := loadConfig(path)
	assertNoError(t, err)

	if cfg.Linux == nil {
		t.Fatal("expected Linux config")
	}
	if cfg.Dotfiles == nil {
		t.Fatal("expected Dotfiles config")
	}
	assertEqualInt(t, len(cfg.Linux.Packages), 2)
	assertEqualInt(t, len(cfg.Linux.Package), 1)
	assertEqualInt(t, len(cfg.Dotfiles.Files), 1)
}

func TestLoadConfig_ReadError(t *testing.T) {
	// Create a file that exists but cannot be read
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make it unreadable
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })

	_, err := loadConfig(path)
	assertError(t, err)
	assertContains(t, err.Error(), "error reading config file")
}
