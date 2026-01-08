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

var verbose bool

func main() {
	// Parse command line flags
	flag.BoolVar(&verbose, "v", false, "Enable verbose output")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose output")
	flag.Parse()

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

	// Load config from default location
	// TODO: Support custom paths via --config flag
	cfg, err := loadConfig(defaultConfigPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create and run the orchestrator
	settle := NewSettle(cfg, verbose)
	if err := settle.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}


