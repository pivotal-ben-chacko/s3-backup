package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds the full backup configuration.
type Config struct {
	Source      S3Config   `json:"source"`
	Destination DestConfig `json:"destination"`
	Options     Options    `json:"options"`
}

// S3Config holds connection details for an S3-compatible source or destination.
type S3Config struct {
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Region    string `json:"region"`
	UseSSL    bool   `json:"use_ssl"`
}

// DestConfig wraps the destination, which can be local or S3.
type DestConfig struct {
	Type  string   `json:"type"` // "local" or "s3"
	Local *LocalConfig `json:"local,omitempty"`
	S3    *S3Config    `json:"s3,omitempty"`
}

// LocalConfig holds configuration for a local filesystem destination.
type LocalConfig struct {
	Path string `json:"path"`
}

// Options holds backup behavior settings.
type Options struct {
	Workers       int    `json:"workers"`
	DryRun        bool   `json:"dry_run"`
	SkipExisting  bool   `json:"skip_existing"`
	LogFile       string `json:"log_file"`
	RetryAttempts int    `json:"retry_attempts"`
}

// LoadConfig reads and parses a JSON configuration file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	// Apply defaults
	if cfg.Options.Workers <= 0 {
		cfg.Options.Workers = 4
	}
	if cfg.Options.RetryAttempts <= 0 {
		cfg.Options.RetryAttempts = 3
	}
	return &cfg, nil
}
