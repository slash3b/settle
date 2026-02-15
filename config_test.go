package main

import (
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

func TestLoadConfig_ValidGit(t *testing.T) {
	path := withTempConfig(t, `
[[git]]
url = "https://github.com/tmux-plugins/tpm"
dest = "~/.tmux/plugins/tpm"

[[git]]
url = "https://github.com/user/repo"
dest = "~/projects/repo"
`)
	cfg, err := loadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, 2, len(cfg.Git))
	assert.Equal(t, "https://github.com/tmux-plugins/tpm", cfg.Git[0].URL)
	assert.Equal(t, "~/.tmux/plugins/tpm", cfg.Git[0].Dest)
	assert.Equal(t, "https://github.com/user/repo", cfg.Git[1].URL)
	assert.Equal(t, "~/projects/repo", cfg.Git[1].Dest)
}

func TestLoadConfig_EmptyGit(t *testing.T) {
	path := withTempConfig(t, `
[apt]
packages = ["vim"]
`)
	cfg, err := loadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, 0, len(cfg.Git))
}

func TestLoadConfig_ValidGo(t *testing.T) {
	path := withTempConfig(t, `
[[go]]
path = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
version = "v2.9.0"

[[go]]
path = "golang.org/x/tools/cmd/goimports"
version = "v0.25.0"
`)
	cfg, err := loadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, 2, len(cfg.Go))
	assert.Equal(t, "github.com/golangci/golangci-lint/v2/cmd/golangci-lint", cfg.Go[0].Path)
	assert.Equal(t, "v2.9.0", cfg.Go[0].Version)
	assert.Equal(t, "golang.org/x/tools/cmd/goimports", cfg.Go[1].Path)
	assert.Equal(t, "v0.25.0", cfg.Go[1].Version)
}

func TestLoadConfig_EmptyGo(t *testing.T) {
	path := withTempConfig(t, `
[apt]
packages = ["vim"]
`)
	cfg, err := loadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, 0, len(cfg.Go))
}

