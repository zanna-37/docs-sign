package api

import (
	"net/http"

	"docs-sign/internal/version"
)

// handleVersion reports the build version. It is public (no auth) so the SPA can show it
// on pre-login screens too; it exposes nothing beyond the publicly visible release tag.
func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version": version.Version,
		"commit":  version.Commit,
		"repoURL": version.RepoURL,
	})
}
