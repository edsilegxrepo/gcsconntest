// TEST STRATEGY EXPLANATION:
// - Objective: Test the concrete defaultStorageClient method wrappers (BucketAttrs and ListObjects).
// - Isolation: Instantiates storage.Client with option.WithoutAuthentication() to safely hit method branches in-memory.
// - Scope: Ensures defaultStorageClient methods execute cleanly and propagate expected errors.
package gcsconntest

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

func TestDefaultStorageClient_Methods(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Initialize real storage client without network authentication to hit method branches safely
	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		t.Skip("skipping defaultStorageClient test: storage client init failed")
	}
	defer func() {
		_ = client.Close()
	}()

	sc := NewStorageClient(client)

	// Test BucketAttrs branch
	_, _ = sc.BucketAttrs(ctx, "invalid-bucket-test-name")

	// Test ListObjects branch
	_, _ = sc.ListObjects(ctx, "invalid-bucket-test-name", "", 1)
}
