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

type DebianConfig struct {
	Packages []string  `toml:"packages"`
	Package  []Package `toml:"package"`
}

type Config struct {
	Debian *DebianConfig `toml:"debian"`
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
	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("error parsing TOML: %w", err)
	}

	return &cfg, nil
}
