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
	Src  string `toml:"src"`
	Dest string `toml:"dest"`
	Mode string `toml:"mode"` // "link" (default) or "copy"
}

// Config represents an entire user's config document.
type Config struct {
	Apt      *AptConfig      `toml:"apt"`
	Dotfiles *DotfilesConfig `toml:"dotfiles"`
	// Future managers:
	// Cargo  *CargoConfig  `toml:"cargo"`
	// Go     *GoConfig     `toml:"go"`
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
