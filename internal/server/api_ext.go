package server

import (
	"net/http"
	"strings"

	"ai-shortlink/internal/model"
)

func (s *Server) adminAPIExt(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin")
	switch {
	case path == "/overview" && r.Method == http.MethodGet:
		s.apiExtOverview(w, r)
	case path == "/settings" && r.Method == http.MethodGet:
		s.apiExtGetSettings(w, r)
	case path == "/settings" && r.Method == http.MethodPut:
		s.apiExtUpdateSettings(w, r)
	case path == "/users" || path == "/users/":
		if r.Method == http.MethodGet {
			s.apiListUsers(w, r)
			return
		}
		if r.Method == http.MethodPost {
			s.apiCreateUser(w, r)
			return
		}
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "method not allowed"))
	case strings.HasPrefix(path, "/users/"):
		s.apiUserDetail(w, r)
	case path == "/short-links" || path == "/short-links/":
		if r.Method == http.MethodGet {
			s.apiExtListShortLinks(w, r)
			return
		}
		if r.Method == http.MethodPost {
			s.apiExtCreateShortLink(w, r)
			return
		}
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "method not allowed"))
	case strings.HasPrefix(path, "/short-links/"):
		s.apiExtShortLinkDetail(w, r)
	case path == "/live-qrs" || path == "/live-qrs/":
		if r.Method == http.MethodGet {
			s.apiExtListLiveQRs(w, r)
			return
		}
		if r.Method == http.MethodPost {
			s.apiExtCreateLiveQR(w, r)
			return
		}
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "method not allowed"))
	case path == "/live-qrs/bundle" && r.Method == http.MethodPost:
		s.apiExtCreateLiveQRBundle(w, r)
	case strings.HasPrefix(path, "/live-qrs/"):
		s.apiExtLiveQRDetail(w, r)
	case strings.HasPrefix(path, "/live-qr-items/"):
		s.apiExtLiveQRItemDetail(w, r)
	default:
		writeJSON(w, http.StatusNotFound, apiErr("not_found", "接口不存在"))
	}
}

func (s *Server) apiExtOverview(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireActor(w, r)
	if !ok {
		return
	}
	o, err := s.store().OverviewForAccount(r.Context(), actor.Account.ID, actor.IsAdmin())
	if err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": o})
}

func (s *Server) apiExtGetSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	st := s.settings(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": st, "database_mode": s.cfg.DatabaseMode})
}

func (s *Server) apiExtUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var p settingsPayload
	if !decodeBody(w, r, &p) {
		return
	}
	st := model.SystemSettings{Installed: true, AppName: strings.TrimSpace(p.AppName), BaseURL: strings.TrimRight(strings.TrimSpace(p.BaseURL), "/"), DefaultLocale: strings.TrimSpace(p.DefaultLocale), LoginMode: strings.TrimSpace(p.LoginMode), SMTPEnabled: p.SMTPEnabled, SMTPHost: strings.TrimSpace(p.SMTPHost), SMTPPort: p.SMTPPort, SMTPSecurity: strings.TrimSpace(p.SMTPSecurity), SMTPUsername: strings.TrimSpace(p.SMTPUsername), SMTPFrom: strings.TrimSpace(p.SMTPFrom)}
	if st.AppName == "" {
		st.AppName = "AI短链平台"
	}
	if st.DefaultLocale == "" {
		st.DefaultLocale = "zh-CN"
	}
	if st.LoginMode == "" {
		st.LoginMode = "hybrid"
	}
	if st.LoginMode != "hybrid" && st.LoginMode != "magic" && st.LoginMode != "one_click" {
		writeJSON(w, 400, apiErr("bad_request", "登录模式无效"))
		return
	}
	if st.BaseURL != "" {
		if err := validateHTTPURL(st.BaseURL); err != nil {
			writeJSON(w, 400, apiErr("bad_request", "站点域名无效："+err.Error()))
			return
		}
	}
	if st.SMTPEnabled {
		if st.SMTPHost == "" || st.SMTPFrom == "" || st.SMTPPort <= 0 {
			writeJSON(w, 400, apiErr("bad_request", "启用 SMTP 时必须填写主机、端口和发信邮箱"))
			return
		}
		if !validEmail(st.SMTPFrom) {
			writeJSON(w, 400, apiErr("bad_request", "发信邮箱格式不正确"))
			return
		}
	}
	current, _ := s.store().GetSettings(r.Context())
	vals := settingsToMap(st)
	if strings.TrimSpace(p.SMTPPassword) != "" {
		cipher, err := s.auth.Encrypt(p.SMTPPassword)
		if err != nil {
			writeJSON(w, 500, apiErr("crypto", err.Error()))
			return
		}
		vals["smtp_password_cipher"] = cipher
	} else if current != nil && strings.TrimSpace(current["smtp_password_cipher"]) != "" {
		vals["smtp_password_cipher"] = current["smtp_password_cipher"]
	} else {
		vals["smtp_password_cipher"] = ""
	}
	if err := s.store().SetSettings(r.Context(), vals); err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": s.settings(r.Context())})
}
