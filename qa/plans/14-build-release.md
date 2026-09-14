# Build & Release

Tests for cross-compilation, packaging, version tagging, and release artifacts.

---

## Build

### BUILD-001: Default Build

**Priority**: P0
**Automated**: `.github/workflows/test.yml` job "test" (5-platform matrix). Runs in CI on every push; a human need not repeat this case.
**Preconditions**: Go toolchain installed (1.15+)

**Steps**:
1. `go build`
2. `./websocketd --version`

**Expected Result**: Binary compiles without errors. Version string is displayed.

---

### BUILD-002: Build with Version Flags

**Priority**: P1

**Steps**:
1. `go build -ldflags "-X main.version=1.0.0-test"`
2. `./websocketd --version`

**Expected Result**: Binary shows "1.0.0-test" as version. Build metadata can be injected via ldflags.

---

### BUILD-003: Run Tests

**Priority**: P0
**Automated**: `.github/workflows/test.yml` job "test" (5-platform matrix). Runs in CI on every push; a human need not repeat this case.

**Steps**:
1. `go test ./...`

**Expected Result**: All tests pass. Zero failures.

---

### BUILD-004: Race Detector

**Priority**: P1
**Automated**: `.github/workflows/test.yml` job "test" runs go test -race. Runs in CI on every push; a human need not repeat this case.

**Steps**:
1. `go test -race ./...`

**Expected Result**: No data races detected. All tests still pass.

---

### BUILD-005: Go Vet

**Priority**: P1
**Automated**: `.github/workflows/test.yml` job "lint" runs go vet. Runs in CI on every push; a human need not repeat this case.

**Steps**:
1. `go vet ./...`

**Expected Result**: No issues reported.

---

### BUILD-006: Static Analysis

**Priority**: P2
**Automated**: `.github/workflows/test.yml` job "lint" runs staticcheck and gosec. Runs in CI on every push; a human need not repeat this case.

**Steps**:
1. Install staticcheck: `go install honnef.co/go/tools/cmd/staticcheck@latest`
2. `staticcheck ./...`

**Expected Result**: No issues or only documented exceptions.

**Notes**: Commit 9032271 fixed issues found by staticcheck.

---

## Cross-Compilation

### BUILD-010: Linux amd64

**Priority**: P0

**Steps**:
1. `GOOS=linux GOARCH=amd64 go build -o websocketd_linux_amd64`
2. Copy to Linux amd64 machine
3. Run `./websocketd_linux_amd64 --version`

**Expected Result**: Binary runs correctly on Linux amd64.

---

### BUILD-011: Linux 386

**Priority**: P2

**Steps**:
1. `GOOS=linux GOARCH=386 go build -o websocketd_linux_386`

**Expected Result**: Compiles successfully.

---

### BUILD-012: Linux ARM

**Priority**: P1

**Steps**:
1. `GOOS=linux GOARCH=arm go build -o websocketd_linux_arm`

**Expected Result**: Compiles successfully. Runs on ARM devices (Raspberry Pi).

---

### BUILD-013: Linux ARM64

**Priority**: P1

**Steps**:
1. `GOOS=linux GOARCH=arm64 go build -o websocketd_linux_arm64`

**Expected Result**: Compiles successfully. Runs on ARM64 devices.

---

### BUILD-014: macOS amd64

**Priority**: P0

**Steps**:
1. `GOOS=darwin GOARCH=amd64 go build -o websocketd_darwin_amd64`

**Expected Result**: Compiles and runs on macOS Intel.

---

### BUILD-015: macOS ARM64

**Priority**: P0

**Steps**:
1. `GOOS=darwin GOARCH=arm64 go build -o websocketd_darwin_arm64`

**Expected Result**: Compiles and runs on Apple Silicon.

---

### BUILD-016: Windows amd64

**Priority**: P0

**Steps**:
1. `GOOS=windows GOARCH=amd64 go build -o websocketd_windows_amd64.exe`

**Expected Result**: Compiles successfully. Runs on Windows.

---

### BUILD-017: Windows 386

**Priority**: P2

**Steps**:
1. `GOOS=windows GOARCH=386 go build -o websocketd_windows_386.exe`

**Expected Result**: Compiles successfully.

---

## Release Artifacts

### BUILD-030: Release Makefile

**Priority**: P1
**Preconditions**: Release tooling available

**Steps**:
1. Review `release/Makefile`
2. Run the release build for one platform
3. Verify the output archive

**Expected Result**: Release Makefile produces versioned archives with correct naming.

---

### BUILD-031: SHA256 Checksums

**Priority**: P1

**Steps**:
1. Build release artifacts
2. Generate checksums
3. Verify checksums match the artifacts

**Expected Result**: CHECKSUMS file is generated and accurate.

---

### BUILD-032: DEB Package

**Priority**: P2
**Preconditions**: fpm tool installed

**Steps**:
1. Build DEB package via release Makefile
2. Install on Debian/Ubuntu
3. Verify binary is placed in correct location
4. Run `websocketd --version`

**Expected Result**: DEB package installs correctly.

---

### BUILD-033: RPM Package

**Priority**: P2
**Preconditions**: fpm tool installed

**Steps**:
1. Build RPM package via release Makefile
2. Install on RHEL/CentOS/Fedora
3. Verify binary location and version

**Expected Result**: RPM package installs correctly.

---

### BUILD-034: Version from Git Tag

**Priority**: P1

**Steps**:
1. Create a git tag: `git tag v99.99.99`
2. Build with the release Makefile
3. Check version output

**Expected Result**: Version string matches the git tag.

---

## Dependency Management

### BUILD-040: Go Modules

**Priority**: P1

**Steps**:
1. `go mod verify`
2. `go mod tidy`
3. Check for any changes to go.mod/go.sum

**Expected Result**: Dependencies are clean. No unexpected changes.

---

### BUILD-041: Dependency Versions

**Priority**: P1

**Steps**:
1. `go list -m all`
2. Check gorilla/websocket version

**Expected Result**: gorilla/websocket v1.4.0 (or later if updated). All dependencies accounted for.

---

### BUILD-042: Vulnerability Scan

**Priority**: P0

**Steps**:
1. `govulncheck ./...` (if available)
2. Check for known CVEs in dependencies

**Expected Result**: No unpatched vulnerabilities in dependencies.

**Notes**: Issue #441 — gorilla/websocket vulnerability.

---

## Go Version Compatibility

### BUILD-050: Build with the go.mod Minimum

**Priority**: P2
**Automated**: `.github/workflows/test.yml` job "test", the `go: '1.21'` matrix entry. Runs in CI on every push; a human need not repeat this case.

**Steps**:
1. Install the toolchain named by the `go` directive in `go.mod` (1.21 as of this writing)
2. `go build`

**Expected Result**: Compiles successfully.

**Notes**: Rewritten 2026-09-07. This case used to name Go 1.15 and call it "the
version used in v0.4.1". `go.mod` was raised to 1.21, so a 1.15 toolchain now
refuses outright — see `qa/integration/known_bugs_test.go`, "TestBUG005 — fixed:
go.mod updated from Go 1.15 to Go 1.21". Note also that only the Linux runner
uses the go.mod minimum: recent macOS dyld rejects binaries built by pre-1.22
toolchains, so the minimum is not validated on macOS.

---

### BUILD-051: Build with Latest Go

**Priority**: P0
**Automated**: `.github/workflows/test.yml` job "test" pins go-version stable. Runs in CI on every push; a human need not repeat this case.

**Steps**:
1. Install latest Go release
2. `go build`
3. `go test ./...`

**Expected Result**: Compiles and tests pass with latest Go.

---

### BUILD-052: Build with Go Tip (Development)

**Priority**: P3

**Steps**:
1. Install Go tip (development version)
2. `go build`

**Expected Result**: Document any compatibility issues with upcoming Go releases.
