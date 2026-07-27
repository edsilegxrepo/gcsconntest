// OBJECTIVES:
// - Encapsulate execution results from GCS connection testing.
// - Provide JSON serialization support for structured logging and machine consumption.
//
// CORE COMPONENTS & DATA FLOW:
// - Result struct: Holds target bucket name, bucket attributes (*storage.BucketAttrs), object names slice, and object count.
// - Result.ToJSON(): Converts Result into a formatted JSON string using json.MarshalIndent.
package gcsconntest

import (
	"encoding/json"

	"cloud.google.com/go/storage"
)

// Result contains details returned from a GCS connection test.
type Result struct {
	// BucketName is the target bucket tested.
	BucketName string `json:"bucket_name"`
	// BucketAttrs contains metadata retrieved for the bucket.
	BucketAttrs *storage.BucketAttrs `json:"bucket_attrs,omitempty"`
	// ObjectNames contains names of objects found (up to MaxObjects).
	ObjectNames []string `json:"object_names"`
	// TotalListed is the count of objects listed in this execution.
	TotalListed int `json:"total_listed"`
}

// ToJSON formats the Result as a pretty-printed JSON string.
// DATA FLOW: Result Receiver -> json.MarshalIndent -> Formatted JSON String / Error.
func (r *Result) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
