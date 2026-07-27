// OBJECTIVES:
// - Define the StorageClient interface to decouple storage operations from concrete GCP client implementations.
// - Implement defaultStorageClient to execute optimized GCS bucket attribute fetching and object listing.
//
// CORE COMPONENTS & DATA FLOW:
// - StorageClient Interface: Defines BucketAttrs and ListObjects contracts.
// - defaultStorageClient Struct: Wraps concrete *storage.Client.
// - BucketAttrs(): Calls storage.Client.Bucket(name).Attrs(ctx).
// - ListObjects(): Applies ProjectionNoACL and SetAttrSelection([]string{"name"}), pre-allocates object slice, iterates GCS objects up to maxObjects.
package gcsconntest

import (
	"context"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// StorageClient abstracts GCS bucket attribute and object listing operations
// to enable unit testing and mocking without live network requests.
type StorageClient interface {
	BucketAttrs(ctx context.Context, bucketName string) (*storage.BucketAttrs, error)
	ListObjects(ctx context.Context, bucketName string, prefix string, maxObjects int) ([]string, error)
}

// defaultStorageClient wraps concrete *storage.Client to satisfy StorageClient.
type defaultStorageClient struct {
	client *storage.Client
}

// NewStorageClient creates a StorageClient interface wrapper around *storage.Client.
// DATA FLOW: *storage.Client -> StorageClient Interface Wrapper.
func NewStorageClient(client *storage.Client) StorageClient {
	return &defaultStorageClient{client: client}
}

// BucketAttrs fetches attributes for the specified GCS bucket.
// DATA FLOW: Context + Bucket Name -> GCS BucketHandle.Attrs -> *storage.BucketAttrs / Error.
func (c *defaultStorageClient) BucketAttrs(ctx context.Context, bucketName string) (*storage.BucketAttrs, error) {
	return c.client.Bucket(bucketName).Attrs(ctx)
}

// ListObjects lists object names in the specified GCS bucket up to maxObjects.
// DATA FLOW: Context + Bucket Name + Prefix + Limit -> GCS BucketHandle.Objects (ProjectionNoACL) -> Iteration -> []string Object Names.
func (c *defaultStorageClient) ListObjects(ctx context.Context, bucketName string, prefix string, maxObjects int) ([]string, error) {
	bucket := c.client.Bucket(bucketName)
	query := &storage.Query{
		Prefix:     prefix,
		Projection: storage.ProjectionNoACL,
	}
	_ = query.SetAttrSelection([]string{"name"})

	it := bucket.Objects(ctx, query)
	objectNames := make([]string, 0, maxObjects)
	count := 0

	for {
		objAttrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		objectNames = append(objectNames, objAttrs.Name)
		count++
		if count >= maxObjects {
			break
		}
	}

	return objectNames, nil
}
