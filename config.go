// Package gcsconntest provides a production-grade, reusable Go engine and CLI to verify
// Google Cloud Storage (GCS) connectivity, authentication, and object accessibility.
//
// OBJECTIVES:
// - Encapsulate configuration parameters for GCS connection testing.
// - Perform input sanitization (path cleaning) and parameter defaulting.
// - Enforce mandatory validation rules before initiating network connections.
//
// CORE COMPONENTS & DATA FLOW:
// - Config struct: Holds credential paths, raw credential bytes, ADC flags, project/bucket metadata, limits, and timeouts.
// - Config.Clean(): Sanitizes input paths via filepath.Clean and populates default limits/timeouts.
// - Config.Validate(): Evaluates mandatory parameter presence, returning ErrInvalidConfig on failure.
package gcsconntest

import (
	"fmt"
	"path/filepath"
	"time"
)

// DefaultMaxObjects is the default limit for listed objects when not specified.
const DefaultMaxObjects = 10

// DefaultTimeout is the default duration limit for GCS operations.
const DefaultTimeout = 30 * time.Second

// Config holds the parameters for GCS connection testing.
type Config struct {
	// CredFile is the path to the Google Cloud service account JSON key file.
	CredFile string
	// CredJSON is raw service account JSON key content. If provided, CredFile is ignored.
	CredJSON []byte
	// AllowADC allows falling back to GCP Application Default Credentials if CredFile/CredJSON are omitted.
	AllowADC bool
	// BucketName is the target GCS bucket name (required).
	BucketName string
	// BucketPrefix is an optional prefix filter for object listing.
	BucketPrefix string
	// ProjectID is the Google Cloud Project ID (required).
	ProjectID string
	// MaxObjects limits the number of objects to list. Defaults to 10 if <= 0.
	MaxObjects int
	// Timeout specifies the maximum time allowed for the connection test. Defaults to 30s if <= 0.
	Timeout time.Duration
	// MasterKey is an optional raw/hex SecretProtector master key used to decrypt CredFile or CredJSON.
	MasterKey string
	// MasterKeyEnv is the name of an environment variable containing the SecretProtector master key.
	MasterKeyEnv string
	// MasterKeyFile is the file path containing the SecretProtector master key.
	MasterKeyFile string
}

// Clean returns a copy of Config with sanitized file paths and defaulted parameters.
// DATA FLOW: Input Config -> Path Cleaning (filepath.Clean) -> Default Fallbacks -> Cleaned Config Copy.
func (c Config) Clean() Config {
	cfg := c
	if cfg.CredFile != "" {
		cfg.CredFile = filepath.Clean(cfg.CredFile)
	}
	if cfg.MasterKeyFile != "" {
		cfg.MasterKeyFile = filepath.Clean(cfg.MasterKeyFile)
	}
	if cfg.MaxObjects <= 0 {
		cfg.MaxObjects = DefaultMaxObjects
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	return cfg
}

// Validate checks whether mandatory parameters are supplied.
// DATA FLOW: Evaluates BucketName, ProjectID, and authentication options (CredFile, CredJSON, AllowADC).
// Returns a wrapped ErrInvalidConfig on validation error, or nil on success.
func (c Config) Validate() error {
	if c.BucketName == "" {
		return fmt.Errorf("%w: bucket name is required", ErrInvalidConfig)
	}
	if c.ProjectID == "" {
		return fmt.Errorf("%w: project ID is required", ErrInvalidConfig)
	}
	if c.CredFile == "" && len(c.CredJSON) == 0 && !c.AllowADC {
		return fmt.Errorf("%w: credentials file, credential JSON, or -adc flag is required", ErrInvalidConfig)
	}
	return nil
}
