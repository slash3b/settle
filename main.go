package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

// Version and BuildTime are set via ldflags at build time
var (
	Version   = "development"
	BuildTime = ""
)

var (
	verbose     bool
	configPath  string
	dryRun      bool
	showVersion bool
)

func printVersion() {
	exe, err := os.Executable()
	if err != nil {
		exe = "unknown"
	}
	fmt.Printf("settle %s\n", Version)
	if BuildTime != "" {
		fmt.Printf("built: %s\n", BuildTime)
	}
	fmt.Printf("binary: %s\n", exe)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: settle [flags] [command] [args]

Commands:
  apply              Apply configuration (default when no command given)
  install <pkg>...   Install packages and track in lockfile
  remove <pkg>...    Remove packages and update lockfile
  update             Upgrade all managed packages to latest versions
  list               Show status of all packages and dotfiles
  version            Show version information

Flags:
`)
	flag.PrintDefaults()
}

func main() {
	flag.BoolVar(&verbose, "v", false, "Enable verbose output")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose output")
	flag.StringVar(&configPath, "config", defaultConfigPath, "Path to config file")
	flag.BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	flag.BoolVar(&dryRun, "n", false, "Show what would be done without making changes")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.Usage = printUsage
	flag.Parse()

	// Handle --version flag early
	if showVersion {
		printVersion()
		return
	}

	// Handle version subcommand early (before config loading)
	if len(flag.Args()) > 0 && flag.Arg(0) == "version" {
		printVersion()
		return
	}

	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "%s is not supported\n", runtime.GOOS)

		os.Exit(1)
	}

	// Set up signal handling for graceful shutdown on Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\nInterrupted! Cleaning up...")

		os.Exit(130) // Standard exit code for SIGINT
	}()

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create the orchestrator
	settle := NewSettle(cfg, configPath, verbose, dryRun)

	// Handle subcommands
	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "list":
			if err := settle.List(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "install":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "Usage: settle install <package> [package...]\n")
				os.Exit(1)
			}
			if err := settle.Install(args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "remove":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "Usage: settle remove <package> [package...]\n")
				os.Exit(1)
			}
			if err := settle.Remove(args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "update":
			if err := settle.Update(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", args[0])
			printUsage()
			os.Exit(1)
		}
	}

	// Default: run apply
	if err := settle.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
