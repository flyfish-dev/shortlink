package server

import (
	"net/http"

	webassets "ai-shortlink/web"
)

func (s *Server) favicon(w http.ResponseWriter, r *http.Request) {
	b, err := webassets.FS.ReadFile("static/brand.svg")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	_, _ = w.Write(b)
}

func (s *Server) publicBootstrap(w http.ResponseWriter, r *http.Request) {
	st := s.settings(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                  true,
		"installed":           st.Installed,
		"settings":            st,
		"database_mode":       s.cfg.DatabaseMode,
		"sqlite_path":         s.cfg.SQLitePath,
		"base_url":            s.publicBaseURL(r),
		"github_auth_enabled": s.githubOAuthConfigured(),
	})
}
