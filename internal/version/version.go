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

// RepoURL is the HTTPS base URL of the project's source repository. It turns the displayed
// Version into a link to the matching GitHub release. The default is the canonical upstream
// and is correct for every build of this repo (local, CI and the published images); forks
// that re-publish can override it at link time, e.g.
//
//	-X docs-sign/internal/version.RepoURL=https://github.com/you/your-fork
var RepoURL = "https://github.com/zanna-37/docs-sign"
