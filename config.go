package main

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

const defaultConfigPath = "config.toml"

type PostHook struct {
	Name        string `toml:"name"`
	PostInstall string `toml:"post_install"`
	Sudo        bool   `toml:"sudo"` // run the post-install script with sudo
}

type AptConfig struct {
	Packages []string `toml:"packages"`
	// PostHooks is basically anything you want to run after package installation.
	PostHooks []PostHook `toml:"post_hook"`
}

type DotfilesConfig struct {
	SourceDir string    `toml:"source_dir"`
	Files     []Dotfile `toml:"file"`
}

type Dotfile struct {
	Src        string `toml:"src"`
	Dest       string `toml:"dest"`
	Mode       string `toml:"mode"`       // "link" (default) or "copy"
	Executable bool   `toml:"executable"` // chmod +x after deploy
	Sudo       bool   `toml:"sudo"`       // use sudo for all fs operations
}

type GitRepo struct {
	URL  string `toml:"url"`
	Dest string `toml:"dest"`
}

type GoPackage struct {
	Path    string `toml:"path"`
	Version string `toml:"version"`
}

// Config represents an entire user's config document.
type Config struct {
	Apt      *AptConfig      `toml:"apt"`
	Dotfiles *DotfilesConfig `toml:"dotfiles"`
	Git      []GitRepo       `toml:"git"`
	Go       []GoPackage     `toml:"go"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("error parsing TOML: %w", err)
	}

	return &cfg, nil
}
