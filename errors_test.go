// TEST STRATEGY EXPLANATION:
//   - Objective: Verify that ClassifyError correctly maps various Go error types, net.Error,
//     context timeout/cancellation errors, and googleapi.Error status codes to diagnostic exit codes (0 to 5).
//   - Isolation: Pure unit testing using error stubs and standard Go testing library without external network calls.
//   - Scope: Covers nil errors, context.DeadlineExceeded, os.ErrNotExist, os.ErrPermission, net.OpError,
//     HTTP status codes (401, 403, 404, 500, 503), wrapped domain errors, and unknown error fallbacks.
package gcsconntest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "nil error",
			err:      nil,
			wantCode: ExitSuccess,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			wantCode: ExitNetworkError,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			wantCode: ExitNetworkError,
		},
		{
			name:     "invalid config error",
			err:      ErrInvalidConfig,
			wantCode: ExitUsageError,
		},
		{
			name:     "file not found error",
			err:      os.ErrNotExist,
			wantCode: ExitAuthError,
		},
		{
			name:     "permission error",
			err:      os.ErrPermission,
			wantCode: ExitAuthError,
		},
		{
			name:     "network error",
			err:      &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			wantCode: ExitNetworkError,
		},
		{
			name:     "google api 401 unauthorized",
			err:      &googleapi.Error{Code: 401, Message: "Unauthorized"},
			wantCode: ExitAuthError,
		},
		{
			name:     "google api 403 forbidden",
			err:      &googleapi.Error{Code: 403, Message: "Forbidden"},
			wantCode: ExitPermissionError,
		},
		{
			name:     "google api 404 not found",
			err:      &googleapi.Error{Code: 404, Message: "Not Found"},
			wantCode: ExitPermissionError,
		},
		{
			name:     "google api 503 service unavailable",
			err:      &googleapi.Error{Code: 503, Message: "Service Unavailable"},
			wantCode: ExitNetworkError,
		},
		{
			name:     "google api 500 internal server error",
			err:      &googleapi.Error{Code: 500, Message: "Internal Error"},
			wantCode: ExitApiError,
		},
		{
			name:     "wrapped ErrAuthFailed",
			err:      fmt.Errorf("%w: invalid SA key", ErrAuthFailed),
			wantCode: ExitAuthError,
		},
		{
			name:     "wrapped ErrBucketAccess",
			err:      fmt.Errorf("%w: missing bucket", ErrBucketAccess),
			wantCode: ExitPermissionError,
		},
		{
			name:     "wrapped ErrNetworkFailed",
			err:      fmt.Errorf("%w: timeout connecting", ErrNetworkFailed),
			wantCode: ExitNetworkError,
		},
		{
			name:     "wrapped ErrApiError",
			err:      fmt.Errorf("%w: list failed", ErrApiError),
			wantCode: ExitApiError,
		},
		{
			name:     "oauth2 retrieve error",
			err:      &oauth2.RetrieveError{Response: nil, Body: []byte("invalid_grant")},
			wantCode: ExitAuthError,
		},
		{
			name:     "generic unknown error",
			err:      errors.New("some unexpected error"),
			wantCode: ExitApiError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCode := ClassifyError(tt.err)
			if gotCode != tt.wantCode {
				t.Errorf("ClassifyError(%v) = %d, want %d", tt.err, gotCode, tt.wantCode)
			}
		})
	}
}
