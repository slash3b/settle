// Package main is the entry point for the settle CLI.
package main

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

const defaultConfigPath = "config.toml"

// Config represents an entire user's config document.
type Config struct {
	Apt      *AptConfig      `toml:"apt"`
	Dotfiles *DotfilesConfig `toml:"dotfiles"`
	Git      []GitRepo       `toml:"git"`
	Go       []GoPackage     `toml:"go"`
	Scripts  []Script        `toml:"scripts"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is user-supplied config file
	if err != nil {
		return nil, fmt.Errorf("error reading config file %s: %w", path, err)
	}

	var cfg Config

	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("error parsing TOML: %w", err)
	}

	for i := range cfg.Go {
		if cfg.Go[i].Version == "" {
			cfg.Go[i].Version = "latest"
		}
	}

	return &cfg, nil
}

type AptConfig struct {
	Packages []string `toml:"packages"`
	// PostHooks is basically anything you want to run after package installation.
	PostHooks []PostHook `toml:"post_hook"`
}

type PostHook struct {
	Name        string `toml:"name"`
	PostInstall string `toml:"post_install"`
	Sudo        bool   `toml:"sudo"` // run the post-install script with sudo
}

type DotfilesConfig struct {
	SourceDir string       `toml:"source_dir"`
	Files     []Dotfile    `toml:"file"`
	Dirs      []DotfileDir `toml:"dir"`
}

type Dotfile struct {
	Src        string `toml:"src"`
	Dest       string `toml:"dest"`
	Mode       string `toml:"mode"`       // "link" (default) or "copy"
	Executable bool   `toml:"executable"` // chmod +x after deploy
	Sudo       bool   `toml:"sudo"`       // use sudo for all fs operations
}

type DotfileDir struct {
	Src  string `toml:"src"`
	Dest string `toml:"dest"`
	Sudo bool   `toml:"sudo"`
}

type GitRepo struct {
	URL  string `toml:"url"`
	Dest string `toml:"dest"`
}

type GoPackage struct {
	Path    string `toml:"path"`
	Version string `toml:"version"`
}

type Script struct {
	Bin     string   `toml:"bin"`
	Run     string   `toml:"run"`
	Plugins []string `toml:"plugins"`
}
