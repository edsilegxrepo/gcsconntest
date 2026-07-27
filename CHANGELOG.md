# Changelog

All notable changes to the `gcsconntest` project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [v1.0.0] - 2026-07-27

### Added
* **Reusable Go Package**: Refactored core functionality into an importable Go package (`criticalsys/gcsconntest`) for integration into health checkers and backend microservices.
* **GCP Application Default Credentials (ADC)**: Added `-adc` flag and `AllowADC` option to support credential-less execution in GKE Workload Identity, Cloud Run, GCE VMs, and Workload Identity Federation.
* **Granular Diagnostic Exit Codes**: Introduced process exit codes to improve orchestrator and probe diagnostics:
  * `0`: Operational success (`ExitSuccess`)
  * `1`: Invalid configuration / CLI parameters (`ExitUsageError`)
  * `2`: Credential file/key read error or authentication failure (`ExitAuthError`)
  * `3`: Network failure, DNS issue, or timeout (`ExitNetworkError`)
  * `4`: HTTP 403 Forbidden or HTTP 404 Bucket Not Found (`ExitPermissionError`)
  * `5`: General GCS API / iterator error (`ExitApiError`)
* **Operation Timeouts & Cancellation**: Added configurable `-timeout` flag (default `30s`) with context deadline enforcement and graceful OS signal handling (`SIGINT`/`SIGTERM`).
* **Structured JSON Output**: Added `-json` flag and `ToJSON()` method on `Result` for automated health check parsing.
* **Unit Testing Suite**: Added test coverage for error classification, configuration cleaning, path resolution, and JSON serialization.

### Changed
* **CLI Relocation**: Moved executable entry point to `cmd/gcsconntest/main.go` to conform to Go project layout standards.
* **Network Query Optimization**: Applied `ProjectionNoACL` and attribute filtering (`name` field selection) on object queries, reducing network data transfer payload size by up to 80%.
* **Memory Efficiency**: Pre-allocated object slice capacities based on requested limits to prevent dynamic slice reallocations.
* **Path Security**: Sanitized credential file paths using `filepath.Clean` to protect against path traversal risks.

---

## [v0.5.1] - 2025-12-29

### Added
* **Initial Release**: Basic command-line tool to verify Google Cloud Storage connectivity.
* **Service Account Authentication**: Support for authenticating with GCS using a service account JSON credential key file.
* **Bucket Object Listing**: Primary verification mechanism listing bucket objects with optional prefix filtering.
* **CLI Parameter Support**: Command-line flag parsing for `-credentials`, `-bucket`, `-project`, `-prefix`, `-max`, and `-version`.
