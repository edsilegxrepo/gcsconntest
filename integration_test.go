//go:build integration

// TEST STRATEGY EXPLANATION:
//   - Objective: End-to-end integration testing against live Google Cloud Storage buckets and endpoints.
//   - Isolation: Gated with '//go:build integration' so standard unit test runs skip live integration automatically.
//   - Scope: Tests Service Account file auth, Service Account JSON bytes auth, Application Default Credentials (ADC),
//     dynamic prefix matching, non-existent bucket 403/404 handling (Exit 4), 1ns timeout context cancellation (Exit 3),
//     malformed SA key JSON handling (Exit 2), and live CLI binary subprocess execution.
package gcsconntest

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/edsilegxrepo/secretprotector/pkg/libsecsecrets"
)

// getIntegrationEnv retrieves live testing parameters from environment variables.
func getIntegrationEnv(t *testing.T) (bucket, project, credPath string) {
	t.Helper()
	bucket = os.Getenv("TEST_GCS_BUCKET")
	project = os.Getenv("TEST_GCS_PROJECT")
	credPath = os.Getenv("TEST_GCS_CREDENTIALS")
	if credPath == "" {
		credPath = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	}

	if bucket == "" || project == "" {
		t.Skip("Skipping live integration test: TEST_GCS_BUCKET and TEST_GCS_PROJECT env vars must be set")
	}

	return bucket, project, credPath
}

func TestIntegration_LiveConnection_ServiceAccountFile(t *testing.T) {
	bucket, project, credPath := getIntegrationEnv(t)
	if credPath == "" {
		t.Skip("Skipping SA file integration test: TEST_GCS_CREDENTIALS or GOOGLE_APPLICATION_CREDENTIALS not set")
	}

	ctx := context.Background()
	cfg := Config{
		CredFile:   credPath,
		BucketName: bucket,
		ProjectID:  project,
		MaxObjects: 5,
		Timeout:    30 * time.Second,
	}

	res, err := TestConnection(ctx, cfg)
	if err != nil {
		t.Fatalf("Live connection failed with SA file: %v", err)
	}

	if res.BucketName != bucket {
		t.Errorf("expected bucket %s, got %s", bucket, res.BucketName)
	}
	if res.BucketAttrs == nil {
		t.Errorf("expected non-nil BucketAttrs for bucket %s", bucket)
	}
	t.Logf("Live SA File Success: Listed %d objects in bucket '%s'", res.TotalListed, res.BucketName)
}

func TestIntegration_LiveConnection_ServiceAccountJSONBytes(t *testing.T) {
	bucket, project, credPath := getIntegrationEnv(t)
	if credPath == "" {
		t.Skip("Skipping SA JSON bytes integration test: TEST_GCS_CREDENTIALS or GOOGLE_APPLICATION_CREDENTIALS not set")
	}

	credBytes, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatalf("failed to read test credentials file '%s': %v", credPath, err)
	}

	ctx := context.Background()
	cfg := Config{
		CredJSON:   credBytes,
		BucketName: bucket,
		ProjectID:  project,
		MaxObjects: 3,
		Timeout:    30 * time.Second,
	}

	res, err := TestConnection(ctx, cfg)
	if err != nil {
		t.Fatalf("Live connection failed with SA JSON bytes: %v", err)
	}

	if res.BucketAttrs == nil {
		t.Errorf("expected non-nil BucketAttrs for bucket %s", bucket)
	}
	t.Logf("Live SA JSON Bytes Success: Listed %d objects in bucket '%s'", res.TotalListed, res.BucketName)
}

func TestIntegration_LiveConnection_ApplicationDefaultCredentials(t *testing.T) {
	bucket, project, _ := getIntegrationEnv(t)

	ctx := context.Background()
	cfg := Config{
		AllowADC:   true,
		BucketName: bucket,
		ProjectID:  project,
		MaxObjects: 3,
		Timeout:    30 * time.Second,
	}

	res, err := TestConnection(ctx, cfg)
	if err != nil {
		t.Skipf("Live ADC connection skipped/failed (ADC credentials may not be configured in local env): %v", err)
	}

	t.Logf("Live ADC Success: Listed %d objects in bucket '%s'", res.TotalListed, res.BucketName)
}

func TestIntegration_LiveConnection_DynamicPrefixAndMaxLimit(t *testing.T) {
	bucket, project, credPath := getIntegrationEnv(t)
	if credPath == "" {
		t.Skip("Skipping test: credentials not set")
	}

	ctx := context.Background()
	// Step 1: List without prefix to discover live object names
	baseCfg := Config{
		CredFile:   credPath,
		BucketName: bucket,
		ProjectID:  project,
		MaxObjects: 5,
		Timeout:    30 * time.Second,
	}

	baseRes, err := TestConnection(ctx, baseCfg)
	if err != nil {
		t.Fatalf("Live connection failed: %v", err)
	}

	prefix := ""
	if len(baseRes.ObjectNames) > 0 {
		firstName := baseRes.ObjectNames[0]
		if len(firstName) > 2 {
			prefix = firstName[:2] // Use first 2 chars of actual object name as prefix
		}
	}

	// Step 2: Test with prefix and max object limit 2
	prefixCfg := Config{
		CredFile:     credPath,
		BucketName:   bucket,
		ProjectID:    project,
		BucketPrefix: prefix,
		MaxObjects:   2,
		Timeout:      30 * time.Second,
	}

	res, err := TestConnection(ctx, prefixCfg)
	if err != nil {
		t.Fatalf("Live connection with prefix '%s' failed: %v", prefix, err)
	}

	if len(res.ObjectNames) > 2 {
		t.Errorf("expected max 2 objects, got %d", len(res.ObjectNames))
	}
	t.Logf("Live Dynamic Prefix Success: Listed %d objects under prefix '%s'", res.TotalListed, prefix)
}

func TestIntegration_LiveConnection_NonExistentBucketError(t *testing.T) {
	_, project, credPath := getIntegrationEnv(t)
	if credPath == "" {
		t.Skip("Skipping test: credentials not set")
	}

	// Dynamic unique bucket name to avoid global GCP collision
	nonExistentBucket := fmt.Sprintf("nonexistent-bucket-%d-gcsconntest", time.Now().UnixNano())
	ctx := context.Background()
	cfg := Config{
		CredFile:   credPath,
		BucketName: nonExistentBucket,
		ProjectID:  project,
		Timeout:    10 * time.Second,
	}

	_, err := TestConnection(ctx, cfg)
	if err == nil {
		t.Fatalf("expected error for non-existent bucket '%s', got nil", nonExistentBucket)
	}

	exitCode := ClassifyError(err)
	if exitCode != ExitPermissionError && exitCode != ExitAuthError {
		t.Errorf("expected exit code %d (ExitPermissionError) or %d (ExitAuthError), got %d (err: %v)", ExitPermissionError, ExitAuthError, exitCode, err)
	}
	t.Logf("Live Non-Existent Bucket Correctly Returned Error Exit Code %d: %v", exitCode, err)
}

func TestIntegration_LiveConnection_TimeoutCancellation(t *testing.T) {
	bucket, project, credPath := getIntegrationEnv(t)

	ctx := context.Background()
	cfg := Config{
		CredFile:   credPath,
		AllowADC:   credPath == "",
		BucketName: bucket,
		ProjectID:  project,
		Timeout:    1 * time.Nanosecond, // Immediate deadline forces context cancellation
	}

	_, err := TestConnection(ctx, cfg)
	if err == nil {
		t.Fatal("expected timeout error for 1ns timeout, got nil")
	}

	exitCode := ClassifyError(err)
	if exitCode != ExitNetworkError {
		t.Errorf("expected ExitNetworkError (%d), got exit code %d (err: %v)", ExitNetworkError, exitCode, err)
	}
	t.Logf("Live Timeout Correctly Returned Exit Code %d: %v", exitCode, err)
}

func TestIntegration_LiveConnection_MalformedCredFile(t *testing.T) {
	bucket, project, _ := getIntegrationEnv(t)
	tmpDir := t.TempDir()
	invalidCredPath := filepath.Join(tmpDir, "invalid-sa-key.json")
	if err := os.WriteFile(invalidCredPath, []byte(`{"type": "service_account", "private_key": "invalid"}`), 0o644); err != nil {
		t.Fatalf("failed to write invalid key file: %v", err)
	}

	ctx := context.Background()
	cfg := Config{
		CredFile:   invalidCredPath,
		BucketName: bucket,
		ProjectID:  project,
		Timeout:    10 * time.Second,
	}

	_, err := TestConnection(ctx, cfg)
	if err == nil {
		t.Fatal("expected auth error for malformed key file, got nil")
	}

	exitCode := ClassifyError(err)
	if exitCode != ExitAuthError {
		t.Errorf("expected ExitAuthError (%d), got exit code %d (err: %v)", ExitAuthError, exitCode, err)
	}
	t.Logf("Live Malformed Credential Correctly Returned Exit Code %d: %v", exitCode, err)
}

func TestIntegration_LiveCLI_ExecuteBinary(t *testing.T) {
	bucket, project, credPath := getIntegrationEnv(t)
	if credPath == "" {
		t.Skip("Skipping live CLI binary integration test: credentials file not set")
	}

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "gcsconntest_test_bin")
	if os.Getenv("GOOS") == "windows" || filepath.Separator == '\\' {
		binaryPath += ".exe"
	}

	// Build CLI executable
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/gcsconntest")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build test binary: %v (output: %s)", err, out)
	}

	// Execute binary with live flags and -json mode
	cmd := exec.Command(binaryPath, "-credentials", credPath, "-bucket", bucket, "-project", project, "-json", "-max", "2")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Live CLI binary execution failed: %v (stderr: %s)", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), fmt.Sprintf(`"bucket_name": "%s"`, bucket)) {
		t.Errorf("expected CLI stdout to contain JSON bucket_name '%s', got: %s", bucket, stdout.String())
	}

	t.Logf("Live CLI Binary Success: Outputted JSON result for bucket '%s'", bucket)
}

func TestIntegration_LiveConnection_SecretProtector_EncryptedFile(t *testing.T) {
	bucket, project, credPath := getIntegrationEnv(t)
	if credPath == "" {
		t.Skip("Skipping live SecretProtector test: credentials file not set")
	}

	rawJSON, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatalf("failed to read live SA credentials file: %v", err)
	}

	ctx := context.Background()
	hexKey, err := libsecsecrets.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate SecretProtector key: %v", err)
	}
	keyBytes, _ := hex.DecodeString(hexKey)

	encryptedStr, err := libsecsecrets.Encrypt(ctx, string(rawJSON), keyBytes)
	if err != nil {
		t.Fatalf("failed to encrypt SA credentials: %v", err)
	}

	tmpDir := t.TempDir()
	encCredPath := filepath.Join(tmpDir, "encrypted-sa.json")
	if err := os.WriteFile(encCredPath, []byte(encryptedStr), 0o644); err != nil {
		t.Fatalf("failed to write encrypted cred file: %v", err)
	}

	cfg := Config{
		CredFile:   encCredPath,
		MasterKey:  hexKey,
		BucketName: bucket,
		ProjectID:  project,
		MaxObjects: 3,
		Timeout:    30 * time.Second,
	}

	res, err := TestConnection(ctx, cfg)
	if err != nil {
		t.Fatalf("Live connection failed with SecretProtector encrypted file: %v", err)
	}

	if res.BucketName != bucket {
		t.Errorf("expected bucket %s, got %s", bucket, res.BucketName)
	}
	t.Logf("Live SecretProtector Integration Success: Listed %d objects from encrypted SA file in bucket '%s'", res.TotalListed, res.BucketName)
}

func TestIntegration_LiveCLI_SecretProtector_EncryptedFile_KeyEnv(t *testing.T) {
	bucket, project, credPath := getIntegrationEnv(t)
	if credPath == "" {
		t.Skip("Skipping live CLI SecretProtector test: credentials file not set")
	}

	rawJSON, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatalf("failed to read live SA credentials file: %v", err)
	}

	ctx := context.Background()
	hexKey, err := libsecsecrets.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate SecretProtector key: %v", err)
	}
	keyBytes, _ := hex.DecodeString(hexKey)

	encryptedStr, err := libsecsecrets.Encrypt(ctx, string(rawJSON), keyBytes)
	if err != nil {
		t.Fatalf("failed to encrypt SA credentials: %v", err)
	}

	tmpDir := t.TempDir()
	encCredPath := filepath.Join(tmpDir, "encrypted-sa.json")
	if err := os.WriteFile(encCredPath, []byte(encryptedStr), 0o644); err != nil {
		t.Fatalf("failed to write encrypted cred file: %v", err)
	}

	binaryPath := filepath.Join(tmpDir, "gcsconntest_test_bin")
	if os.Getenv("GOOS") == "windows" || filepath.Separator == '\\' {
		binaryPath += ".exe"
	}

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/gcsconntest")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build test binary: %v (output: %s)", err, out)
	}

	envVarName := "LIVE_TEST_SECRETPROTECTOR_KEY"
	cmd := exec.Command(binaryPath, "-credentials", encCredPath, "-key-env", envVarName, "-bucket", bucket, "-project", project, "-json", "-max", "2")
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", envVarName, hexKey))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Live CLI binary execution with SecretProtector failed: %v (stderr: %s)", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), fmt.Sprintf(`"bucket_name": "%s"`, bucket)) {
		t.Errorf("expected CLI stdout to contain JSON bucket_name '%s', got: %s", bucket, stdout.String())
	}

	t.Logf("Live CLI SecretProtector Success: Outputted JSON result for bucket '%s'", bucket)
}

func TestIntegration_LiveCLI_SecretProtector_EncryptedFile_KeyFile(t *testing.T) {
	bucket, project, credPath := getIntegrationEnv(t)
	if credPath == "" {
		t.Skip("Skipping live CLI SecretProtector key-file test: credentials file not set")
	}

	rawJSON, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatalf("failed to read live SA credentials file: %v", err)
	}

	ctx := context.Background()
	hexKey, err := libsecsecrets.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate SecretProtector key: %v", err)
	}
	keyBytes, _ := hex.DecodeString(hexKey)

	encryptedStr, err := libsecsecrets.Encrypt(ctx, string(rawJSON), keyBytes)
	if err != nil {
		t.Fatalf("failed to encrypt SA credentials: %v", err)
	}

	// Create key file in user home dir subfolder to satisfy Windows/Linux path & permission security checks
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = t.TempDir()
	}
	keyDir := filepath.Join(homeDir, ".gcsconntest_test_keys")
	_ = os.MkdirAll(keyDir, 0o700)
	defer func() {
		_ = os.RemoveAll(keyDir)
	}()

	keyFilePath := filepath.Join(keyDir, "live_master.key")
	if err := os.WriteFile(keyFilePath, []byte(hexKey), 0o600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	tmpDir := t.TempDir()
	encCredPath := filepath.Join(tmpDir, "encrypted-sa.json")
	if err := os.WriteFile(encCredPath, []byte(encryptedStr), 0o644); err != nil {
		t.Fatalf("failed to write encrypted cred file: %v", err)
	}

	binaryPath := filepath.Join(tmpDir, "gcsconntest_test_bin")
	if os.Getenv("GOOS") == "windows" || filepath.Separator == '\\' {
		binaryPath += ".exe"
	}

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/gcsconntest")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build test binary: %v (output: %s)", err, out)
	}

	cmd := exec.Command(binaryPath, "-credentials", encCredPath, "-key-file", keyFilePath, "-bucket", bucket, "-project", project, "-json", "-max", "2")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Live CLI binary execution with SecretProtector key-file failed: %v (stderr: %s)", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), fmt.Sprintf(`"bucket_name": "%s"`, bucket)) {
		t.Errorf("expected CLI stdout to contain JSON bucket_name '%s', got: %s", bucket, stdout.String())
	}

	t.Logf("Live CLI SecretProtector Key-File Success: Outputted JSON result for bucket '%s'", bucket)
}

func TestIntegration_LiveConnection_SecretProtector_PlatformIdiomatic(t *testing.T) {
	bucket, project, credPath := getIntegrationEnv(t)
	if credPath == "" {
		t.Skip("Skipping live platform-idiomatic SecretProtector test: credentials file not set")
	}

	rawJSON, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatalf("failed to read live SA credentials file: %v", err)
	}

	ctx := context.Background()
	hexKey, err := libsecsecrets.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate SecretProtector key: %v", err)
	}
	keyBytes, _ := hex.DecodeString(hexKey)

	encryptedStr, err := libsecsecrets.Encrypt(ctx, string(rawJSON), keyBytes)
	if err != nil {
		t.Fatalf("failed to encrypt SA credentials: %v", err)
	}

	tmpDir := t.TempDir()
	encCredPath := filepath.Join(tmpDir, "encrypted-sa.json")
	if err := os.WriteFile(encCredPath, []byte(encryptedStr), 0o644); err != nil {
		t.Fatalf("failed to write encrypted cred file: %v", err)
	}

	cfg := Config{
		CredFile:   encCredPath,
		BucketName: bucket,
		ProjectID:  project,
		MaxObjects: 2,
		Timeout:    30 * time.Second,
	}

	if os.Getenv("GOOS") == "windows" || filepath.Separator == '\\' {
		// On Windows, environment variable resolution (-key-env) is idiomatic
		envVarName := "PLATFORM_IDIOMATIC_MASTER_KEY"
		t.Setenv(envVarName, hexKey)
		cfg.MasterKeyEnv = envVarName
		t.Logf("Platform Idiomatic Test (Windows): Using MasterKeyEnv (%s)", envVarName)
	} else {
		// On Linux/Unix, restricted permission key file (-key-file) is idiomatic
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = tmpDir
		}
		keyDir := filepath.Join(homeDir, ".gcsconntest_idiomatic_keys")
		_ = os.MkdirAll(keyDir, 0o700)
		defer func() { _ = os.RemoveAll(keyDir) }()

		keyFilePath := filepath.Join(keyDir, "master.key")
		if err := os.WriteFile(keyFilePath, []byte(hexKey), 0o400); err != nil {
			t.Fatalf("failed to write restricted key file: %v", err)
		}
		cfg.MasterKeyFile = keyFilePath
		t.Logf("Platform Idiomatic Test (Linux/Unix): Using MasterKeyFile (%s, mode 0400)", keyFilePath)
	}

	res, err := TestConnection(ctx, cfg)
	if err != nil {
		t.Fatalf("Live platform-idiomatic connection failed: %v", err)
	}

	if res.BucketName != bucket {
		t.Errorf("expected bucket %s, got %s", bucket, res.BucketName)
	}
	t.Logf("Live Platform Idiomatic SecretProtector Test Success: Bucket '%s'", res.BucketName)
}
