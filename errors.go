// OBJECTIVES:
// - Define domain error variables and granular exit codes (0 to 5) for process orchestration.
// - Implement error classification logic to map standard Go errors, net.Error, and googleapi.Error to diagnostic exit codes.
//
// CORE COMPONENTS & DATA FLOW:
// - Exit code constants (ExitSuccess, ExitUsageError, ExitAuthError, ExitNetworkError, ExitPermissionError, ExitApiError).
// - Sentinel error definitions (ErrInvalidConfig, ErrAuthFailed, ErrBucketAccess, ErrNetworkFailed, ErrApiError).
// - ClassifyError(err error) int: Inspects error tree using errors.Is and errors.As to return precise exit code.
package gcsconntest

import (
	"context"
	"errors"
	"net"
	"os"

	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

// Exit codes for CLI diagnostics and process orchestration.
const (
	ExitSuccess         = 0 // Operational success
	ExitUsageError      = 1 // Missing or invalid configuration / parameters
	ExitAuthError       = 2 // Credentials file unreadable, malformed, or authentication failed
	ExitNetworkError    = 3 // Network failure, DNS issue, or request context timeout
	ExitPermissionError = 4 // GCS 403 Forbidden or 404 Bucket Not Found
	ExitApiError        = 5 // General GCS API or iterator failure
)

// Custom error types for programmatic classification.
var (
	ErrInvalidConfig = errors.New("invalid configuration")
	ErrAuthFailed    = errors.New("authentication failed")
	ErrBucketAccess  = errors.New("bucket access denied or not found")
	ErrNetworkFailed = errors.New("network connectivity error")
	ErrApiError      = errors.New("API operation error")
)

// ClassifyError inspects an error and returns the corresponding diagnostic exit code.
// DATA FLOW: Input error -> Context check -> Sentinel error check -> OS file error check -> net.Error check -> googleapi.Error status code check -> Exit Code Integer.
func ClassifyError(err error) int {
	if err == nil {
		return ExitSuccess
	}

	// Context cancellation / timeout errors
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ExitNetworkError
	}

	// Configuration errors
	if errors.Is(err, ErrInvalidConfig) {
		return ExitUsageError
	}

	// File read / OS missing file errors for credentials
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
		return ExitAuthError
	}

	// Network / Socket errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return ExitNetworkError
	}

	// Google API HTTP status code errors
	var gErr *googleapi.Error
	if errors.As(err, &gErr) {
		switch gErr.Code {
		case 401:
			return ExitAuthError
		case 403, 404:
			return ExitPermissionError
		case 408, 502, 503, 504:
			return ExitNetworkError
		default:
			return ExitApiError
		}
	}

	// OAuth2 token exchange errors
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) {
		return ExitAuthError
	}

	if errors.Is(err, ErrAuthFailed) {
		return ExitAuthError
	}
	if errors.Is(err, ErrBucketAccess) {
		return ExitPermissionError
	}
	if errors.Is(err, ErrNetworkFailed) {
		return ExitNetworkError
	}
	if errors.Is(err, ErrApiError) {
		return ExitApiError
	}

	return ExitApiError
}
