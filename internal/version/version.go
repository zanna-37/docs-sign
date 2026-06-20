// Package version exposes the build version of the binary. The values are stamped at
// link time from the git tag/commit (see the Makefile, Dockerfile and CI workflow):
//
//	go build -ldflags "-X docs-sign/internal/version.Version=v1.2.3 -X docs-sign/internal/version.Commit=abc1234"
//
// Unstamped builds (e.g. `go run`) report the defaults below.
package version

// Version is the human-readable build version, normally `git describe --tags`
// (e.g. "v1.2.3" on a tagged release, or "v1.2.3-4-gabc1234" between tags).
var Version = "dev"

// Commit is the short git commit hash the binary was built from.
var Commit = "none"
