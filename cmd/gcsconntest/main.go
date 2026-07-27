// OBJECTIVES:
// - Standalone CLI application entry point for Google Cloud Storage connectivity testing.
// - Parse command-line flags (-credentials, -bucket, -project, -prefix, -max, -timeout, -adc, -json, -version).
// - Handle cross-platform OS signals (SIGINT/SIGTERM) for graceful cancellation.
// - Execute connection verification and return granular diagnostic exit codes (0 to 5).
//
// CORE COMPONENTS & DATA FLOW:
// - main(): Initializes signal context -> calls runApp -> calls os.Exit(exitCode).
// - runApp(ctx, args, stdout, stderr, testFn): Parses flags into Config struct -> invokes testFn -> formats human or JSON output -> classifies errors to exit codes.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"criticalsys/gcsconntest"
)

// Compile-time version variable (can be overridden via -ldflags)
var version = "dev"

// TestFunc defines the function signature for running a GCS connection test.
type TestFunc func(ctx context.Context, cfg gcsconntest.Config) (*gcsconntest.Result, error)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	exitCode := runApp(ctx, os.Args[1:], os.Stdout, os.Stderr, gcsconntest.TestConnection)
	os.Exit(exitCode)
}

// runApp encapsulates flag parsing, execution, and output formatting for unit testing.
// DATA FLOW: Context + []string Args + Stdout/Stderr Writers + TestFunc -> Flag Parsing -> Config Struct -> TestFunc Execution -> Format Output (JSON/Human) or Classify Error -> Exit Code Integer.
func runApp(ctx context.Context, args []string, stdout, stderr io.Writer, testFn TestFunc) int {
	var (
		credFile     string
		bucketName   string
		bucketPrefix string
		projectID    string
		maxObjects   int
		timeout      time.Duration
		allowADC     bool
		jsonOutput   bool
		showVersion  bool
		masterKey    string
		masterKeyEnv string
		masterKeyFile string
	)

	fs := flag.NewFlagSet("gcsconntest", flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.StringVar(&credFile, "credentials", "", "Path to JSON credential file")
	fs.StringVar(&bucketName, "bucket", "", "GCS bucket name")
	fs.StringVar(&bucketPrefix, "prefix", "", "GCS bucket prefix filter")
	fs.StringVar(&projectID, "project", "", "GCP project ID")
	fs.IntVar(&maxObjects, "max", gcsconntest.DefaultMaxObjects, "Maximum number of objects to list")
	fs.DurationVar(&timeout, "timeout", gcsconntest.DefaultTimeout, "Operation timeout (e.g., 30s, 1m)")
	fs.BoolVar(&allowADC, "adc", false, "Allow GCP Application Default Credentials if key file is omitted")
	fs.BoolVar(&jsonOutput, "json", false, "Format output as structured JSON")
	fs.BoolVar(&showVersion, "version", false, "Display application version")
	fs.StringVar(&masterKey, "key", "", "SecretProtector direct master key (hex or raw)")
	fs.StringVar(&masterKeyEnv, "key-env", "", "Environment variable name containing SecretProtector master key")
	fs.StringVar(&masterKeyFile, "key-file", "", "File path containing SecretProtector master key")

	if err := fs.Parse(args); err != nil {
		return gcsconntest.ExitUsageError
	}

	if showVersion {
		_, _ = fmt.Fprintln(stdout, "GCS Connection Tester - Version:", version)
		return gcsconntest.ExitSuccess
	}

	cfg := gcsconntest.Config{
		CredFile:      credFile,
		AllowADC:      allowADC,
		BucketName:    bucketName,
		BucketPrefix:  bucketPrefix,
		ProjectID:     projectID,
		MaxObjects:    maxObjects,
		Timeout:       timeout,
		MasterKey:     masterKey,
		MasterKeyEnv:  masterKeyEnv,
		MasterKeyFile: masterKeyFile,
	}

	if testFn == nil {
		testFn = gcsconntest.TestConnection
	}

	res, err := testFn(ctx, cfg)
	if err != nil {
		exitCode := gcsconntest.ClassifyError(err)
		_, _ = fmt.Fprintf(stderr, "Error [Exit %d]: %v\n", exitCode, err)
		return exitCode
	}

	if jsonOutput {
		jsonStr, err := res.ToJSON()
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "Error [Exit %d]: failed to format JSON output: %v\n", gcsconntest.ExitApiError, err)
			return gcsconntest.ExitApiError
		}
		_, _ = fmt.Fprintln(stdout, jsonStr)
		return gcsconntest.ExitSuccess
	}

	if cfg.BucketPrefix == "" {
		_, _ = fmt.Fprintf(stdout, "Listing up to %d objects in bucket %s:\n", cfg.MaxObjects, cfg.BucketName)
	} else {
		_, _ = fmt.Fprintf(stdout, "Listing up to %d objects in bucket %s with prefix %s:\n", cfg.MaxObjects, cfg.BucketName, cfg.BucketPrefix)
	}

	for _, name := range res.ObjectNames {
		_, _ = fmt.Fprintln(stdout, " -", name)
	}

	_, _ = fmt.Fprintln(stdout, "GCS connectivity test successful.")
	return gcsconntest.ExitSuccess
}
