package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"ai-shortlink/internal/model"
	"ai-shortlink/internal/util"
)

func (s *Server) settings(ctx context.Context) model.SystemSettings {
	m, _ := s.store().GetSettings(ctx)
	return s.settingsFromMap(m)
}

func (s *Server) settingsFromMap(m map[string]string) model.SystemSettings {
	if m == nil {
		m = map[string]string{}
	}
	st := model.SystemSettings{
		Installed:     settingBool(m, "installed"),
		AppName:       firstNonEmpty(m["app_name"], s.cfg.AppName, "AI短链平台"),
		BaseURL:       strings.TrimRight(firstNonEmpty(m["base_url"], s.cfg.BaseURL), "/"),
		DefaultLocale: firstNonEmpty(m["default_locale"], "zh-CN"),
		LoginMode:     firstNonEmpty(m["login_mode"], "hybrid"),
		AdminEmail:    strings.TrimSpace(m["admin_email"]),
		SMTPEnabled:   settingBool(m, "smtp_enabled"),
		SMTPHost:      strings.TrimSpace(m["smtp_host"]),
		SMTPPort:      settingInt(m, "smtp_port", 465),
		SMTPSecurity:  firstNonEmpty(m["smtp_security"], "tls"),
		SMTPUsername:  strings.TrimSpace(m["smtp_username"]),
		SMTPFrom:      strings.TrimSpace(m["smtp_from"]),
	}
	st.SMTPPasswordSet = strings.TrimSpace(m["smtp_password_cipher"]) != ""
	if st.LoginMode != "hybrid" && st.LoginMode != "magic" && st.LoginMode != "one_click" {
		st.LoginMode = "hybrid"
	}
	if st.DefaultLocale == "" {
		st.DefaultLocale = "zh-CN"
	}
	return st
}

func (s *Server) isInstalled(ctx context.Context) bool { return s.settings(ctx).Installed }

func (s *Server) appName(ctx context.Context) string { return s.settings(ctx).AppName }

func (s *Server) publicBaseURL(r *http.Request) string {
	st := s.settings(r.Context())
	if strings.TrimSpace(st.BaseURL) != "" {
		return strings.TrimRight(st.BaseURL, "/")
	}
	return util.PublicBaseURL(r, s.cfg.BaseURL, s.cfg.TrustProxy)
}

func (s *Server) smtpPassword(ctx context.Context) string {
	m, err := s.store().GetSettings(ctx)
	if err != nil {
		return ""
	}
	cipher := strings.TrimSpace(m["smtp_password_cipher"])
	if cipher == "" {
		return ""
	}
	plain, err := s.auth.Decrypt(cipher)
	if err != nil {
		return ""
	}
	return plain
}

func settingsToMap(st model.SystemSettings) map[string]string {
	return map[string]string{
		"installed":      boolSetting(st.Installed),
		"app_name":       strings.TrimSpace(st.AppName),
		"base_url":       strings.TrimRight(strings.TrimSpace(st.BaseURL), "/"),
		"default_locale": strings.TrimSpace(st.DefaultLocale),
		"login_mode":     strings.TrimSpace(st.LoginMode),
		"admin_email":    strings.TrimSpace(st.AdminEmail),
		"smtp_enabled":   boolSetting(st.SMTPEnabled),
		"smtp_host":      strings.TrimSpace(st.SMTPHost),
		"smtp_port":      strconv.Itoa(st.SMTPPort),
		"smtp_security":  strings.TrimSpace(st.SMTPSecurity),
		"smtp_username":  strings.TrimSpace(st.SMTPUsername),
		"smtp_from":      strings.TrimSpace(st.SMTPFrom),
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
func settingBool(m map[string]string, key string) bool {
	v := strings.ToLower(strings.TrimSpace(m[key]))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
func settingInt(m map[string]string, key string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(m[key]))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
func boolSetting(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func accountResponse(s *Server, acct *model.AdminAccount) map[string]any {
	recoveryKey := s.recoveryKeyForAccount(acct)
	m := map[string]any{"id": acct.ID, "email": acct.Email, "name": acct.Name, "role": acct.Role, "status": acct.Status, "is_admin": acct.Role == "admin", "recovery_key_available": recoveryKey != ""}
	if recoveryKey != "" {
		m["recovery_key"] = recoveryKey
	}
	return m
}

func validEmail(email string) bool {
	email = strings.TrimSpace(email)
	if len(email) < 5 || len(email) > 320 {
		return false
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	domain := email[at+1:]
	return strings.Contains(domain, ".") && !strings.ContainsAny(email, " \t\r\n")
}
