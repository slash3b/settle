package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_ValidApt(t *testing.T) {
	path := withTempConfig(t, `
[apt]
packages = ["vim", "curl", "git"]
`)
	cfg, err := loadConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Apt)
	assert.Equal(t, 3, len(cfg.Apt.Packages))
	assert.Equal(t, "vim", cfg.Apt.Packages[0])
	assert.Equal(t, "curl", cfg.Apt.Packages[1])
	assert.Equal(t, "git", cfg.Apt.Packages[2])
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
	require.NoError(t, err)
	require.NotNil(t, cfg.Dotfiles)
	assert.Equal(t, "~/dotfiles/sources", cfg.Dotfiles.SourceDir)
	assert.Equal(t, 2, len(cfg.Dotfiles.Files))
	assert.Equal(t, "alacritty.toml", cfg.Dotfiles.Files[0].Src)
	assert.Equal(t, "~/.config/alacritty/alacritty.toml", cfg.Dotfiles.Files[0].Dest)
	assert.Equal(t, "", cfg.Dotfiles.Files[0].Mode)
	assert.Equal(t, "copy", cfg.Dotfiles.Files[1].Mode)
}

func TestLoadConfig_PackageWithPostInstall(t *testing.T) {
	path := withTempConfig(t, `
[apt]
packages = ["curl"]

[[apt.post_hook]]
name = "pipewire"
post_install = "systemctl --user --now enable wireplumber.service"
`)
	cfg, err := loadConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Apt)
	assert.Equal(t, 1, len(cfg.Apt.Packages))
	assert.Equal(t, 1, len(cfg.Apt.PostHooks))
	assert.Equal(t, "pipewire", cfg.Apt.PostHooks[0].Name)
	assert.Equal(t, "systemctl --user --now enable wireplumber.service", cfg.Apt.PostHooks[0].PostInstall)
}

func TestLoadConfig_Empty(t *testing.T) {
	path := withTempConfig(t, "")
	cfg, err := loadConfig(path)
	require.NoError(t, err)
	assert.Nil(t, cfg.Apt)
	assert.Nil(t, cfg.Dotfiles)
}

func TestLoadConfig_NotFound(t *testing.T) {
	_, err := loadConfig("/nonexistent/path/config.toml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error reading config file")
}

func TestLoadConfig_InvalidTOML(t *testing.T) {
	path := withTempConfig(t, `this is not valid toml [[[`)
	_, err := loadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error parsing TOML")
}

func TestLoadConfig_FullConfig(t *testing.T) {
	path := withTempConfig(t, `
[apt]
packages = ["vim", "curl"]

[[apt.post_hook]]
name = "pipewire"
post_install = "systemctl --user enable wireplumber"

[dotfiles]
source_dir = "~/dotfiles"

[[dotfiles.file]]
src = "vimrc"
dest = "~/.vimrc"
`)
	cfg, err := loadConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Apt)
	require.NotNil(t, cfg.Dotfiles)
	assert.Equal(t, 2, len(cfg.Apt.Packages))
	assert.Equal(t, 1, len(cfg.Apt.PostHooks))
	assert.Equal(t, 1, len(cfg.Dotfiles.Files))
}

func TestLoadConfig_ReadError(t *testing.T) {
	path := withTempConfig(t, "hello")
	os.Chmod(path, 0o000)
	t.Cleanup(func() { os.Chmod(path, 0o644) })

	_, err := loadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error reading config file")
}
