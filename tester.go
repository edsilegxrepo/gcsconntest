// OBJECTIVES:
// - Implement core GCS connection verification logic for library consumers and CLI entry points.
// - Manage client lifecycle, authentication resolution (File, JSON Bytes, ADC), context timeouts, and error handling.
// - Expose interface-based TestConnectionWithStorageClient to support mock-driven testing.
//
// CORE COMPONENTS & DATA FLOW:
// - NewClient(ctx, cfg): Resolves authentication options -> creates *storage.Client.
// - TestConnection(ctx, cfg): Validates config -> cleans parameters -> applies timeout context -> instantiates client -> executes test -> closes client.
// - TestConnectionWithClient(ctx, client, cfg): Wraps concrete *storage.Client into StorageClient interface.
// - TestConnectionWithStorageClient(ctx, client, cfg): Executes BucketAttrs and ListObjects operations -> aggregates into *Result.
package gcsconntest

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/edsilegxrepo/secretprotector/pkg/libsecsecrets"
	"google.golang.org/api/option"
)

// NewClient initializes a storage.Client using credentials specified in Config.
// DATA FLOW: Config -> SecretProtector Key Resolution (optional) -> Credential Mode Resolution (CredJSON / CredFile / AllowADC) -> Decryption (optional) -> Memory Zeroing -> storage.NewClient -> *storage.Client / Error.
func NewClient(ctx context.Context, cfg Config) (*storage.Client, error) {
	cfg = cfg.Clean()

	var opts []option.ClientOption

	hasMasterKeyOpts := cfg.MasterKey != "" || cfg.MasterKeyEnv != "" || cfg.MasterKeyFile != ""
	var masterKey []byte
	if hasMasterKeyOpts {
		var err error
		masterKey, err = libsecsecrets.ResolveKey(ctx, cfg.MasterKey, cfg.MasterKeyEnv, cfg.MasterKeyFile)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to resolve master key: %v", ErrAuthFailed, err)
		}
		defer libsecsecrets.ZeroBuffer(masterKey)
	}

	if len(cfg.CredJSON) > 0 {
		credBytes := cfg.CredJSON
		if len(masterKey) > 0 {
			decryptedStr, err := libsecsecrets.Decrypt(ctx, strings.TrimSpace(string(cfg.CredJSON)), masterKey)
			if err != nil {
				return nil, fmt.Errorf("%w: failed to decrypt CredJSON: %v", ErrAuthFailed, err)
			}
			credBytes = []byte(decryptedStr)
			defer libsecsecrets.ZeroBuffer(credBytes)
		}
		opts = append(opts, option.WithAuthCredentialsJSON(option.ServiceAccount, credBytes))
	} else if cfg.CredFile != "" {
		rawBytes, err := os.ReadFile(cfg.CredFile)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to read credentials file: %v", ErrAuthFailed, err)
		}
		defer libsecsecrets.ZeroBuffer(rawBytes)
		credBytes := rawBytes
		if len(masterKey) > 0 {
			decryptedStr, err := libsecsecrets.Decrypt(ctx, strings.TrimSpace(string(rawBytes)), masterKey)
			if err != nil {
				return nil, fmt.Errorf("%w: failed to decrypt CredFile content: %v", ErrAuthFailed, err)
			}
			credBytes = []byte(decryptedStr)
			defer libsecsecrets.ZeroBuffer(credBytes)
		}
		opts = append(opts, option.WithAuthCredentialsJSON(option.ServiceAccount, credBytes))
	} else if !cfg.AllowADC {
		return nil, fmt.Errorf("%w: no credentials file, JSON, or ADC enabled", ErrInvalidConfig)
	}

	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAuthFailed, err)
	}

	return client, nil
}

// TestConnection tests connectivity to GCS by validating configuration,
// applying timeouts, instantiating a client, fetching bucket attributes, and listing objects.
// DATA FLOW: Context + Config -> Validation & Cleaning -> Timeout Context Creation -> Client Initialization -> Test Execution -> Resource Cleanup -> *Result.
func TestConnection(ctx context.Context, cfg Config) (*Result, error) {
	cfg = cfg.Clean()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	client, err := NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = client.Close()
	}()

	return TestConnectionWithStorageClient(ctx, NewStorageClient(client), cfg)
}

// TestConnectionWithClient runs the connectivity test using a caller-provided storage.Client.
// DATA FLOW: Context + *storage.Client + Config -> StorageClient Interface Wrapper -> TestConnectionWithStorageClient.
func TestConnectionWithClient(ctx context.Context, client *storage.Client, cfg Config) (*Result, error) {
	if client == nil {
		return nil, fmt.Errorf("%w: storage client is nil", ErrInvalidConfig)
	}
	return TestConnectionWithStorageClient(ctx, NewStorageClient(client), cfg)
}

// TestConnectionWithStorageClient runs the connectivity test using a StorageClient interface (enables full offline mocking).
// DATA FLOW: Context + StorageClient + Config -> BucketAttrs() -> ListObjects() -> Aggregated *Result / Error.
func TestConnectionWithStorageClient(ctx context.Context, client StorageClient, cfg Config) (*Result, error) {
	cfg = cfg.Clean()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("%w: storage client is nil", ErrInvalidConfig)
	}

	attrs, err := client.BucketAttrs(ctx, cfg.BucketName)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to fetch bucket attributes for '%s': %v", ErrBucketAccess, cfg.BucketName, err)
	}

	objectNames, err := client.ListObjects(ctx, cfg.BucketName, cfg.BucketPrefix, cfg.MaxObjects)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to list objects in bucket '%s': %v", ErrApiError, cfg.BucketName, err)
	}

	return &Result{
		BucketName:  cfg.BucketName,
		BucketAttrs: attrs,
		ObjectNames: objectNames,
		TotalListed: len(objectNames),
	}, nil
}
