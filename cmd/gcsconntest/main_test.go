// TEST STRATEGY EXPLANATION:
//   - Objective: Comprehensive unit test coverage for the CLI application entry point (runApp).
//   - Isolation: Uses bytes.Buffer for stdout/stderr capturing and injectable mock TestFunc handlers.
//   - Scope: Tests version output (-version), unknown flag errors, validation errors, human output formatting,
//     structured JSON output formatting (-json), prefix string formatting, error exit code classification (Exit 1-5),
//     and nil TestFunc fallback behavior.
package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/edsilegxrepo/gcsconntest"
)

func TestRunApp_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runApp(context.Background(), []string{"-version"}, &stdout, &stderr, nil)

	if code != gcsconntest.ExitSuccess {
		t.Errorf("expected exit code %d, got %d", gcsconntest.ExitSuccess, code)
	}
	if !strings.Contains(stdout.String(), "GCS Connection Tester - Version:") {
		t.Errorf("expected version output in stdout, got: %s", stdout.String())
	}
}

func TestRunApp_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runApp(context.Background(), []string{"-help"}, &stdout, &stderr, nil)

	if code != gcsconntest.ExitSuccess {
		t.Errorf("expected exit code %d for -help, got %d", gcsconntest.ExitSuccess, code)
	}
}

func TestRunApp_InvalidFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runApp(context.Background(), []string{"-unknownflag"}, &stdout, &stderr, nil)

	if code != gcsconntest.ExitUsageError {
		t.Errorf("expected exit code %d, got %d", gcsconntest.ExitUsageError, code)
	}
}

func TestRunApp_ValidationError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runApp(context.Background(), []string{"-bucket", "mybucket"}, &stdout, &stderr, nil)

	if code != gcsconntest.ExitUsageError {
		t.Errorf("expected exit code %d, got %d", gcsconntest.ExitUsageError, code)
	}
	if !strings.Contains(stderr.String(), "project ID is required") {
		t.Errorf("expected project ID error in stderr, got: %s", stderr.String())
	}
}

func TestRunApp_SuccessHumanOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mockTest := func(ctx context.Context, cfg gcsconntest.Config) (*gcsconntest.Result, error) {
		return &gcsconntest.Result{
			BucketName:  cfg.BucketName,
			ObjectNames: []string{"a.txt", "b.txt"},
			TotalListed: 2,
		}, nil
	}

	args := []string{"-bucket", "mybucket", "-project", "myproj", "-adc"}
	code := runApp(context.Background(), args, &stdout, &stderr, mockTest)

	if code != gcsconntest.ExitSuccess {
		t.Errorf("expected exit code %d, got %d", gcsconntest.ExitSuccess, code)
	}
	if !strings.Contains(stdout.String(), "GCS connectivity test successful.") {
		t.Errorf("expected success message in stdout, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "a.txt") {
		t.Errorf("expected object listing in stdout, got: %s", stdout.String())
	}
}

func TestRunApp_SuccessJSONOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mockTest := func(ctx context.Context, cfg gcsconntest.Config) (*gcsconntest.Result, error) {
		return &gcsconntest.Result{
			BucketName:  cfg.BucketName,
			ObjectNames: []string{"data.csv"},
			TotalListed: 1,
		}, nil
	}

	args := []string{"-bucket", "mybucket", "-project", "myproj", "-adc", "-json"}
	code := runApp(context.Background(), args, &stdout, &stderr, mockTest)

	if code != gcsconntest.ExitSuccess {
		t.Errorf("expected exit code %d, got %d", gcsconntest.ExitSuccess, code)
	}
	if !strings.Contains(stdout.String(), `"bucket_name": "mybucket"`) {
		t.Errorf("expected JSON output containing bucket_name, got: %s", stdout.String())
	}
}

func TestRunApp_PrefixHumanOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mockTest := func(ctx context.Context, cfg gcsconntest.Config) (*gcsconntest.Result, error) {
		return &gcsconntest.Result{
			BucketName:  cfg.BucketName,
			ObjectNames: []string{"sub/file.txt"},
			TotalListed: 1,
		}, nil
	}

	args := []string{"-bucket", "mybucket", "-project", "myproj", "-prefix", "sub/", "-adc"}
	code := runApp(context.Background(), args, &stdout, &stderr, mockTest)

	if code != gcsconntest.ExitSuccess {
		t.Errorf("expected exit code %d, got %d", gcsconntest.ExitSuccess, code)
	}
	if !strings.Contains(stdout.String(), "with prefix sub/") {
		t.Errorf("expected prefix in stdout, got: %s", stdout.String())
	}
}

func TestRunApp_ErrorClassification(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mockTest := func(ctx context.Context, cfg gcsconntest.Config) (*gcsconntest.Result, error) {
		return nil, gcsconntest.ErrAuthFailed
	}

	args := []string{"-bucket", "mybucket", "-project", "myproj", "-adc"}
	code := runApp(context.Background(), args, &stdout, &stderr, mockTest)

	if code != gcsconntest.ExitAuthError {
		t.Errorf("expected exit code %d, got %d", gcsconntest.ExitAuthError, code)
	}
	if !strings.Contains(stderr.String(), "Error [Exit 2]") {
		t.Errorf("expected Exit 2 error in stderr, got: %s", stderr.String())
	}
}

func TestRunApp_ContextTimeoutError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mockTest := func(ctx context.Context, cfg gcsconntest.Config) (*gcsconntest.Result, error) {
		return nil, context.DeadlineExceeded
	}

	args := []string{"-bucket", "mybucket", "-project", "myproj", "-adc"}
	code := runApp(context.Background(), args, &stdout, &stderr, mockTest)

	if code != gcsconntest.ExitNetworkError {
		t.Errorf("expected exit code %d, got %d", gcsconntest.ExitNetworkError, code)
	}
	if !strings.Contains(stderr.String(), "Error [Exit 3]") {
		t.Errorf("expected Exit 3 error in stderr, got: %s", stderr.String())
	}
}

func TestRunApp_NilTestFnFallback(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Invalid config triggers quick validation error without network calls
	args := []string{"-bucket", "mybucket"}
	code := runApp(context.Background(), args, &stdout, &stderr, nil)

	if code != gcsconntest.ExitUsageError {
		t.Errorf("expected exit code %d, got %d", gcsconntest.ExitUsageError, code)
	}
}

func TestRunApp_SecretProtectorFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var capturedCfg gcsconntest.Config
	mockTest := func(ctx context.Context, cfg gcsconntest.Config) (*gcsconntest.Result, error) {
		capturedCfg = cfg
		return &gcsconntest.Result{
			BucketName:  cfg.BucketName,
			ObjectNames: []string{"test.txt"},
			TotalListed: 1,
		}, nil
	}

	args := []string{
		"-credentials", "enc_credentials.json",
		"-bucket", "mybucket",
		"-project", "myproj",
		"-key", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		"-key-env", "MY_KEY_ENV",
		"-key-file", "/path/to/key.file",
	}

	code := runApp(context.Background(), args, &stdout, &stderr, mockTest)
	if code != gcsconntest.ExitSuccess {
		t.Errorf("expected exit code %d, got %d", gcsconntest.ExitSuccess, code)
	}

	if capturedCfg.MasterKey != "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef" {
		t.Errorf("unexpected MasterKey: %s", capturedCfg.MasterKey)
	}
	if capturedCfg.MasterKeyEnv != "MY_KEY_ENV" {
		t.Errorf("unexpected MasterKeyEnv: %s", capturedCfg.MasterKeyEnv)
	}
	if capturedCfg.MasterKeyFile != "/path/to/key.file" {
		t.Errorf("unexpected MasterKeyFile: %s", capturedCfg.MasterKeyFile)
	}
}
