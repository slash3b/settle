package main

import (
    "fmt"
    "log"
    "os"
    "os/exec"
    "runtime"

    "github.com/pelletier/go-toml/v2"
)

type Config struct {
	Packages struct {
		List []string `toml:"list"`
	} `toml:"packages"`
}

func main() {
    if runtime.GOOS != "linux" {
        fmt.Fprintf(os.Stderr, "%s is not supported", runtime.GOOS)
        os.Exit(1);
    }

	data, err := os.ReadFile("config.toml")
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	var cfg Config
	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		log.Fatalf("Error parsing TOML: %v", err)
	}

	fmt.Println("Successfully loaded packages:")
	for i, pkg := range cfg.Packages.List {
		fmt.Printf("%d: %s\n", i+1, pkg)
	}
	
	fmt.Printf("\nTotal packages: %d\n", len(cfg.Packages.List))


    if err := installPackages(cfg.Packages.List); err != nil {
        fmt.Printf("Error installing packages: %v\n", err)
        os.Exit(1)
    }
}

func installPackages(packages []string) error {
    if len(packages) == 0 {
        return nil
    }

    args := append([]string{"install", "-y"}, packages...)

    cmd := exec.Command("sudo", append([]string{"apt-get"}, args...)...)

    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin

    cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")

    fmt.Printf("Installing %d packages...\n", len(packages))

    return cmd.Run()
}

