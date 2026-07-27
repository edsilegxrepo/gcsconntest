# Changelog

All notable changes to the `gcsconntest` project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [v1.0.0] - 2026-07-27

### Added
* **Reusable Go Package**: Refactored core functionality into an importable root Go package (`criticalsys/gcsconntest`) for seamless integration into health checkers and backend microservices.
* **GCP Application Default Credentials (ADC)**: Added `-adc` flag and `AllowADC` option to support credential-less execution in GKE Workload Identity, Cloud Run, GCE VMs, and Workload Identity Federation.
* **Granular Diagnostic Exit Codes**: Introduced process exit codes (0 to 5) to improve orchestrator and probe diagnostics:
  * `0`: Operational success (`ExitSuccess`)
  * `1`: Invalid configuration / CLI parameters (`ExitUsageError`)
  * `2`: Credential file/key read error or authentication failure (`ExitAuthError`)
  * `3`: Network failure, DNS issue, or timeout (`ExitNetworkError`)
  * `4`: HTTP 403 Forbidden or HTTP 404 Bucket Not Found (`ExitPermissionError`)
  * `5`: General GCS API / iterator error (`ExitApiError`)
* **Operation Timeouts & Cancellation**: Added configurable `-timeout` flag (default `30s`) with context deadline enforcement and graceful OS signal handling (`SIGINT`/`SIGTERM`).
* **Structured JSON Output**: Added `-json` flag and `ToJSON()` method on `Result` for automated health check parsing.
* **Test Coverage Optimization (87.5%+ Total Coverage)**: Built 100% offline unit mocking framework using `StorageClient` interface abstraction and `mockStorageClient` stub.
* **Live Integration Test Suite (`integration_test.go`)**: Added live GCS test suite gated under `//go:build integration` covering SA key file auth, raw SA JSON bytes, ADC, dynamic prefix discovery, 403/404 non-existent bucket handling, 1ns timeout cancellation, malformed credentials, and compiled CLI binary subprocess execution.
* **Architecture & Testing Documentation**:
  * Created [ARCHITECTURE.md](ARCHITECTURE.md) with Mermaid component blueprints, operational sequence diagrams, security/IAM models, ADC vs. JSON key comparisons, and external Go microservice integration guides.
  * Created [TESTING.md](TESTING.md) with test architecture flowcharts, 34-test inventory table, coverage reports, and platform execution commands for PowerShell, CMD, and Bash.
  * Updated [README.md](README.md) with security assessment, code quality assessment, CLI flag tables, exit codes matrix, and usage examples with output samples.
* **Build Manifest Tracking**: Tracked `main.txt` (`./cmd/gcsconntest`) for automated build pipeline integration.
* **Inline Codebase Documentation**: Systematically documented 100% of code functions and methods inline with objectives, data flows, and test strategy headers.

### Changed
* **CLI Relocation**: Moved executable entry point to `cmd/gcsconntest/main.go` to conform to standard Go project layout conventions.
* **Network Query Optimization**: Applied `ProjectionNoACL` and attribute filtering (`name` field selection) on object queries, reducing network data transfer payload size by up to 80%.
* **Memory Efficiency**: Pre-allocated object slice capacities based on requested limits to prevent dynamic slice reallocations.
* **Path Security**: Sanitized credential file paths using `filepath.Clean` to protect against path traversal risks.
* **Security & Quality Compliance**: Resolved static analysis and linter findings across `GolangCI Meta-Linter` (`errcheck`), `Semgrep` (Dependabot cooldown policy), `Gosec`, `TruffleHog`, and `GoVulnCheck`.

---

## [v0.5.1] - 2025-12-29

### Added
* **Initial Release**: Basic command-line tool to verify Google Cloud Storage connectivity.
* **Service Account Authentication**: Support for authenticating with GCS using a service account JSON credential key file.
* **Bucket Object Listing**: Primary verification mechanism listing bucket objects with optional prefix filtering.
* **CLI Parameter Support**: Command-line flag parsing for `-credentials`, `-bucket`, `-project`, `-prefix`, `-max`, and `-version`.
