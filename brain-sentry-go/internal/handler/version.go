package handler

import (
	"net/http"
)

// VersionInfo is what /version returns. Populated at process start from
// main.go's package-level vars (set via -ldflags at build time).
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
}

// VersionHandler returns a closure that always responds with the given
// build info. Public endpoint (no auth) — useful for confirming which
// image is running in homologation/production and for healthcheck
// pipelines that need to assert a specific version.
func VersionHandler(info VersionInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, info)
	}
}
