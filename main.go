package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

var (
	// version and buildTime are set via ldflags at build time
	version   = "development"
	buildTime = ""

	verbose     bool
	configPath  string
	dryRun      bool
	showVersion bool
)

func main() {
	flag.StringVar(&configPath, "c", defaultConfigPath, "Path to config file")
	flag.BoolVar(&verbose, "v", false, "Enable verbose output")
	flag.BoolVar(&dryRun, "n", false, "Show what would be done without making changes")
	flag.BoolVar(&showVersion, "version", false, "Show version information")

	usage := printUsage(os.Stdout)

	flag.Usage = usage

	flag.Parse()

	// so we handle --version, -version and version variants.
	if showVersion || (len(flag.Args()) > 0 && flag.Arg(0) == "version") {
		printVersion(os.Stdout)

		return
	}

	if runtime.GOOS != "linux" {
		_, _ = fmt.Fprintf(os.Stderr, "%s is not supported\n", runtime.GOOS)

		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan

		_, _ = fmt.Println("\n\ninterrupted! cleaning up...")

		os.Exit(2)
	}()

	cfg, err := loadConfig(configPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to load config %s, err %v\n", configPath, err)

		os.Exit(1)
	}

	var (
		settle = NewSettle(cfg, verbose, dryRun, os.Stdout)
		args   = flag.Args()
	)

	if len(args) == 0 {
		err := settle.Apply()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

			os.Exit(1)
		}

		return
	}

	switch args[0] {
	case "list":
		settle.List()
	case "update":
		err := settle.Update()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

			os.Exit(1)
		}
	default:
		_, _ = fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", args[0])

		usage()

		os.Exit(1)
	}
}

func printVersion(w io.Writer) {
	exe, err := os.Executable()
	if err != nil {
		exe = "unknown"
	}

	fmt.Fprintf(w, "%-9s %s\n", "version", version) //nolint:errcheck

	if buildTime != "" {
		fmt.Fprintf(w, "%-9s %s\n", "built", buildTime) //nolint:errcheck
	}

	fmt.Fprintf(w, "%-9s %s\n", "binary", exe) //nolint:errcheck
}

// an extension on top of flag default help message.
func printUsage(w io.Writer) func() {
	flag.CommandLine.SetOutput(w)

	return func() {
		_, _ = fmt.Fprintf(w, `Usage: settle [flags] [command]

Running settle with no command syncs your system to match config.toml.

Commands:
    update   Upgrade all managed packages to latest versions
    list     Show status of all packages and dotfiles
    version  Show version information
Flags:
`)

		flag.PrintDefaults()
	}
}
