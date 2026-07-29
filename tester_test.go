// TEST STRATEGY EXPLANATION:
// - Objective: Comprehensive unit test coverage for gcsconntest library functionality (Config validation, cleaning, JSON output, client initialization, and storage operations).
// - Isolation: Uses mockStorageClient stub implementing the StorageClient interface to mock GCS responses (attributes, object lists, errors) completely offline.
// - Scope: Tests happy paths, nil client handling, bucket attribute errors, object iteration errors, file reading errors, and config validation logic.
package gcsconntest

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"criticalsys.net/secretprotector/pkg/libsecsecrets"
)

type mockStorageClient struct {
	attrsFunc   func(ctx context.Context, bucketName string) (*storage.BucketAttrs, error)
	listObjects func(ctx context.Context, bucketName, prefix string, maxObjects int) ([]string, error)
}

func (m *mockStorageClient) BucketAttrs(ctx context.Context, bucketName string) (*storage.BucketAttrs, error) {
	if m.attrsFunc != nil {
		return m.attrsFunc(ctx, bucketName)
	}
	return &storage.BucketAttrs{Name: bucketName, Location: "US"}, nil
}

func (m *mockStorageClient) ListObjects(ctx context.Context, bucketName, prefix string, maxObjects int) ([]string, error) {
	if m.listObjects != nil {
		return m.listObjects(ctx, bucketName, prefix, maxObjects)
	}
	return []string{"file1.txt", "file2.txt"}, nil
}

func TestNewStorageClient(t *testing.T) {
	sc := NewStorageClient(nil)
	if sc == nil {
		t.Error("expected non-nil StorageClient from NewStorageClient(nil)")
	}
}

func TestTestConnection_ValidationFail(t *testing.T) {
	ctx := context.Background()
	_, err := TestConnection(ctx, Config{})
	if err == nil {
		t.Error("expected validation error for empty Config in TestConnection, got nil")
	}
}

func TestTestConnection_NewClientFail(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		CredFile:   filepath.Join(t.TempDir(), "nonexistent.json"),
		BucketName: "my-bucket",
		ProjectID:  "my-project",
	}

	_, err := TestConnection(ctx, cfg)
	if err == nil {
		t.Error("expected file read error in TestConnection, got nil")
	}
}

func TestConnectionWithStorageClient_Success(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		BucketName: "test-bucket",
		ProjectID:  "test-project",
		AllowADC:   true,
		MaxObjects: 5,
	}

	mock := &mockStorageClient{}
	res, err := TestConnectionWithStorageClient(ctx, mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.BucketName != "test-bucket" {
		t.Errorf("expected BucketName 'test-bucket', got '%s'", res.BucketName)
	}
	if res.TotalListed != 2 {
		t.Errorf("expected TotalListed = 2, got %d", res.TotalListed)
	}
	if len(res.ObjectNames) != 2 || res.ObjectNames[0] != "file1.txt" {
		t.Errorf("unexpected ObjectNames: %v", res.ObjectNames)
	}
}

func TestConnectionWithStorageClient_NilClient(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		BucketName: "test-bucket",
		ProjectID:  "test-project",
		AllowADC:   true,
	}

	_, err := TestConnectionWithStorageClient(ctx, nil, cfg)
	if err == nil {
		t.Error("expected error for nil StorageClient, got nil")
	}
}

func TestConnectionWithStorageClient_InvalidConfig(t *testing.T) {
	ctx := context.Background()
	mock := &mockStorageClient{}
	_, err := TestConnectionWithStorageClient(ctx, mock, Config{})
	if err == nil {
		t.Error("expected validation error for empty config, got nil")
	}
}

func TestConnectionWithStorageClient_NilClientConcrete(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		BucketName: "test-bucket",
		ProjectID:  "test-project",
		AllowADC:   true,
	}

	var client *storage.Client
	_, err := TestConnectionWithClient(ctx, client, cfg)
	if err == nil {
		t.Error("expected error for nil concrete *storage.Client, got nil")
	}
}

func TestConnectionWithStorageClient_AttrsError(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		BucketName: "test-bucket",
		ProjectID:  "test-project",
		AllowADC:   true,
	}

	mock := &mockStorageClient{
		attrsFunc: func(ctx context.Context, bucketName string) (*storage.BucketAttrs, error) {
			return nil, errors.New("bucket not found")
		},
	}

	_, err := TestConnectionWithStorageClient(ctx, mock, cfg)
	if err == nil {
		t.Error("expected error when BucketAttrs fails, got nil")
	}
}

func TestConnectionWithStorageClient_ListError(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		BucketName: "test-bucket",
		ProjectID:  "test-project",
		AllowADC:   true,
	}

	mock := &mockStorageClient{
		listObjects: func(ctx context.Context, bucketName, prefix string, maxObjects int) ([]string, error) {
			return nil, errors.New("iteration failed")
		},
	}

	_, err := TestConnectionWithStorageClient(ctx, mock, cfg)
	if err == nil {
		t.Error("expected error when ListObjects fails, got nil")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "empty config",
			cfg:     Config{},
			wantErr: true,
		},
		{
			name: "missing credentials without ADC",
			cfg: Config{
				BucketName: "my-bucket",
				ProjectID:  "my-project",
			},
			wantErr: true,
		},
		{
			name: "valid with ADC",
			cfg: Config{
				BucketName: "my-bucket",
				ProjectID:  "my-project",
				AllowADC:   true,
			},
			wantErr: false,
		},
		{
			name: "missing bucket",
			cfg: Config{
				CredFile:  "creds.json",
				ProjectID: "my-project",
			},
			wantErr: true,
		},
		{
			name: "missing project id",
			cfg: Config{
				CredFile:   "creds.json",
				BucketName: "my-bucket",
			},
			wantErr: true,
		},
		{
			name: "valid config with cred file",
			cfg: Config{
				CredFile:   "creds.json",
				BucketName: "my-bucket",
				ProjectID:  "my-project",
			},
			wantErr: false,
		},
		{
			name: "valid config with cred JSON",
			cfg: Config{
				CredJSON:   []byte(`{"type": "service_account"}`),
				BucketName: "my-bucket",
				ProjectID:  "my-project",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigClean(t *testing.T) {
	cfg := Config{
		CredFile: "some/path/../creds.json",
	}.Clean()

	if cfg.MaxObjects != DefaultMaxObjects {
		t.Errorf("expected MaxObjects = %d, got %d", DefaultMaxObjects, cfg.MaxObjects)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("expected Timeout = %v, got %v", DefaultTimeout, cfg.Timeout)
	}
	expectedPath := filepath.Clean("some/path/../creds.json")
	if cfg.CredFile != expectedPath {
		t.Errorf("expected CredFile = %s, got %s", expectedPath, cfg.CredFile)
	}
}

func TestResultToJSON(t *testing.T) {
	res := &Result{
		BucketName:  "test-bucket",
		ObjectNames: []string{"file1.txt", "file2.txt"},
		TotalListed: 2,
	}

	jsonStr, err := res.ToJSON()
	if err != nil {
		t.Fatalf("unexpected error from ToJSON(): %v", err)
	}

	if jsonStr == "" {
		t.Error("expected non-empty JSON string")
	}
}

func TestNewClientMissingFile(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		CredFile:   filepath.Join(t.TempDir(), "nonexistent.json"),
		BucketName: "my-bucket",
		ProjectID:  "my-project",
		Timeout:    1 * time.Second,
	}

	_, err := NewClient(ctx, cfg)
	if err == nil {
		t.Error("expected error for nonexistent credentials file, got nil")
	}
}

func TestNewClientInvalidJSON(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	credPath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(credPath, []byte("invalid json content"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	cfg := Config{
		CredFile:   credPath,
		BucketName: "my-bucket",
		ProjectID:  "my-project",
		Timeout:    1 * time.Second,
	}

	_, err := NewClient(ctx, cfg)
	if err == nil {
		t.Error("expected error for invalid credentials json, got nil")
	}
}

func TestNewClientNoCredentials(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		BucketName: "my-bucket",
		ProjectID:  "my-project",
		AllowADC:   false,
	}

	_, err := NewClient(ctx, cfg)
	if err == nil {
		t.Error("expected error when no credentials and ADC false, got nil")
	}
}

func TestNewClient_SecretProtector_EncryptedCredJSON_DirectKey(t *testing.T) {
	ctx := context.Background()
	hexKey, err := libsecsecrets.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	keyBytes, _ := hex.DecodeString(hexKey)

	plainJSON := `{"type": "service_account", "project_id": "test"}`
	encryptedJSON, err := libsecsecrets.Encrypt(ctx, plainJSON, keyBytes)
	if err != nil {
		t.Fatalf("failed to encrypt json: %v", err)
	}

	cfg := Config{
		CredJSON:   []byte(encryptedJSON),
		MasterKey:  hexKey,
		BucketName: "my-bucket",
		ProjectID:  "my-project",
		Timeout:    1 * time.Second,
	}

	_, err = NewClient(ctx, cfg)
	if err == nil {
		t.Error("expected auth error for invalid SA key format, got nil")
	}
	if errors.Is(err, ErrInvalidConfig) {
		t.Errorf("unexpected ErrInvalidConfig: %v", err)
	}
}

func TestNewClient_SecretProtector_EncryptedCredFile_KeyEnv(t *testing.T) {
	ctx := context.Background()
	hexKey, err := libsecsecrets.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	keyBytes, _ := hex.DecodeString(hexKey)

	envVar := "TEST_GCSCONNTEST_MASTER_KEY"
	t.Setenv(envVar, hexKey)

	plainJSON := `{"type": "service_account", "project_id": "test"}`
	encryptedJSON, err := libsecsecrets.Encrypt(ctx, plainJSON, keyBytes)
	if err != nil {
		t.Fatalf("failed to encrypt json: %v", err)
	}

	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "encrypted_cred.json")
	if err := os.WriteFile(credFile, []byte(encryptedJSON), 0o644); err != nil {
		t.Fatalf("failed to write cred file: %v", err)
	}

	cfg := Config{
		CredFile:     credFile,
		MasterKeyEnv: envVar,
		BucketName:   "my-bucket",
		ProjectID:    "my-project",
		Timeout:      1 * time.Second,
	}

	_, err = NewClient(ctx, cfg)
	if err == nil {
		t.Error("expected auth error for invalid SA key format, got nil")
	}
}

func TestNewClient_SecretProtector_EncryptedCredFile_KeyFile(t *testing.T) {
	ctx := context.Background()
	hexKey, err := libsecsecrets.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	keyBytes, _ := hex.DecodeString(hexKey)

	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "master.key")
	if err := os.WriteFile(keyFile, []byte(hexKey), 0o600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	plainJSON := `{"type": "service_account", "project_id": "test"}`
	encryptedJSON, err := libsecsecrets.Encrypt(ctx, plainJSON, keyBytes)
	if err != nil {
		t.Fatalf("failed to encrypt json: %v", err)
	}

	credFile := filepath.Join(tmpDir, "encrypted_cred.json")
	if err := os.WriteFile(credFile, []byte(encryptedJSON), 0o644); err != nil {
		t.Fatalf("failed to write cred file: %v", err)
	}

	cfg := Config{
		CredFile:      credFile,
		MasterKeyFile: keyFile,
		BucketName:    "my-bucket",
		ProjectID:     "my-project",
		Timeout:       1 * time.Second,
	}

	_, err = NewClient(ctx, cfg)
	if err == nil {
		t.Error("expected auth error for invalid SA key format, got nil")
	}
}

func TestNewClient_SecretProtector_InvalidMasterKey(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		CredJSON:   []byte("encrypted_string_here"),
		MasterKey:  "invalidkey",
		BucketName: "my-bucket",
		ProjectID:  "my-project",
	}

	_, err := NewClient(ctx, cfg)
	if err == nil {
		t.Error("expected error for invalid master key, got nil")
	}
	if !errors.Is(err, ErrAuthFailed) {
		t.Errorf("expected ErrAuthFailed, got: %v", err)
	}
}

func TestNewClient_SecretProtector_DecryptionFailure(t *testing.T) {
	ctx := context.Background()
	hexKey, _ := libsecsecrets.GenerateKey()
	cfg := Config{
		CredJSON:   []byte("invalidbase64data!!"),
		MasterKey:  hexKey,
		BucketName: "my-bucket",
		ProjectID:  "my-project",
	}

	_, err := NewClient(ctx, cfg)
	if err == nil {
		t.Error("expected error for failed decryption, got nil")
	}
	if !errors.Is(err, ErrAuthFailed) {
		t.Errorf("expected ErrAuthFailed, got: %v", err)
	}
}
