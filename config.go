package main

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

const defaultConfigPath = "config.toml"

type Package struct {
	Name        string `toml:"name"`
	PostInstall string `toml:"post_install"`
}

type LinuxConfig struct {
	Packages []string  `toml:"packages"`
	Package  []Package `toml:"package"`
}

type Dotfile struct {
	Src  string `toml:"src"`
	Dest string `toml:"dest"`
	Mode string `toml:"mode"` // "link" (default) or "copy"
}

type DotfilesConfig struct {
	SourceDir string    `toml:"source_dir"`
	Files     []Dotfile `toml:"file"`
}

type Config struct {
	Linux    *LinuxConfig    `toml:"linux"`
	Dotfiles *DotfilesConfig `toml:"dotfiles"`
	// Future managers:
	// Cargo  *CargoConfig  `toml:"cargo"`
	// Go     *GoConfig     `toml:"go"`
}

func loadConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s\n\nCreate a config.toml file or specify a path with --config", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var cfg Config
	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("error parsing TOML: %w", err)
	}

	return &cfg, nil
}
