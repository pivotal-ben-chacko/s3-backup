package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to config file")
	dryRun := flag.Bool("dry-run", false, "List what would be copied without transferring")
	workers := flag.Int("workers", 0, "Override number of concurrent workers")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// CLI overrides
	if *dryRun {
		cfg.Options.DryRun = true
	}
	if *workers > 0 {
		cfg.Options.Workers = *workers
	}

	// Setup logger
	var logWriter io.Writer = os.Stdout
	if cfg.Options.LogFile != "" {
		f, err := os.OpenFile(cfg.Options.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		logWriter = io.MultiWriter(os.Stdout, f)
	}
	logger := log.New(logWriter, "[s3backup] ", log.LstdFlags)

	// Build destination
	var dest Destination
	switch cfg.Destination.Type {
	case "local":
		if cfg.Destination.Local == nil {
			logger.Fatal("destination type is 'local' but no local config provided")
		}
		dest, err = NewLocalDestination(cfg.Destination.Local.Path)
	case "s3":
		if cfg.Destination.S3 == nil {
			logger.Fatal("destination type is 's3' but no s3 config provided")
		}
		dest, err = NewS3Destination(cfg.Destination.S3)
	default:
		logger.Fatalf("unknown destination type: %s (use 'local' or 's3')", cfg.Destination.Type)
	}
	if err != nil {
		logger.Fatalf("creating destination: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		logger.Println("Received shutdown signal, finishing in-progress transfers...")
		cancel()
	}()

	// Run backup
	engine, err := NewBackupEngine(cfg, dest, logger)
	if err != nil {
		logger.Fatalf("initializing backup engine: %v", err)
	}
	if err := engine.Run(ctx); err != nil {
		logger.Fatalf("Backup failed: %v", err)
	}
}
