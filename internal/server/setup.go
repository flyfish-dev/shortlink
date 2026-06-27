package server

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"ai-shortlink/internal/auth"
	"ai-shortlink/internal/config"
	"ai-shortlink/internal/dbutil"
	"ai-shortlink/internal/model"
	"ai-shortlink/internal/store"
	"ai-shortlink/internal/util"
)

type setupPayload struct {
	DatabaseMode  string `json:"database_mode"`
	DSN           string `json:"dsn"`
	SQLitePath    string `json:"sqlite_path"`
	AppName       string `json:"app_name"`
	AppNameZH     string `json:"app_name_zh"`
	AppNameEN     string `json:"app_name_en"`
	BaseURL       string `json:"base_url"`
	DefaultLocale string `json:"default_locale"`
	AdminEmail    string `json:"admin_email"`
	AdminName     string `json:"admin_name"`
	SMTPEnabled   bool   `json:"smtp_enabled"`
	SMTPHost      string `json:"smtp_host"`
	SMTPPort      int    `json:"smtp_port"`
	SMTPSecurity  string `json:"smtp_security"`
	SMTPUsername  string `json:"smtp_username"`
	SMTPPassword  string `json:"smtp_password"`
	SMTPFrom      string `json:"smtp_from"`
}

func (s *Server) setupPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/setup" {
		http.NotFound(w, r)
		return
	}
	if s.isInstalled(r.Context()) {
		if s.isLoggedIn(r) {
			http.Redirect(w, r, "/admin", http.StatusFound)
		} else {
			http.Redirect(w, r, "/login", http.StatusFound)
		}
		return
	}
	s.render(w, r, "setup.html", map[string]any{"AppName": s.cfg.AppName, "BaseURL": util.PublicBaseURL(r, s.cfg.BaseURL, s.cfg.TrustProxy), "SQLitePath": s.cfg.SQLitePath})
}

func (s *Server) setupTestDB(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "仅支持 POST"))
		return
	}
	var p setupPayload
	if !decodeBody(w, r, &p) {
		return
	}
	mode := config.NormalizeDatabaseMode(p.DatabaseMode)
	dsn, sqlitePath := strings.TrimSpace(p.DSN), normalizeSQLitePath(s.cfg.DataDir, p.SQLitePath)
	db, err := dbutil.Open(r.Context(), mode, dsn, sqlitePath)
	if err != nil {
		writeJSON(w, 400, apiErr("db", err.Error()))
		return
	}
	defer db.Close()
	if err := dbutil.Migrate(r.Context(), db, mode); err != nil {
		writeJSON(w, 400, apiErr("migrate", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) setupInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "仅支持 POST"))
		return
	}
	if s.isInstalled(r.Context()) {
		writeJSON(w, http.StatusConflict, apiErr("installed", "系统已完成安装"))
		return
	}
	var p setupPayload
	if !decodeBody(w, r, &p) {
		return
	}
	p.AdminEmail = strings.TrimSpace(p.AdminEmail)
	p.AdminName = strings.TrimSpace(p.AdminName)
	if !validEmail(p.AdminEmail) {
		writeJSON(w, 400, apiErr("bad_request", "请填写有效的管理员邮箱，用于 Magic Link 登录和找回控制权"))
		return
	}
	mode := config.NormalizeDatabaseMode(p.DatabaseMode)
	dsn := strings.TrimSpace(p.DSN)
	sqlitePath := normalizeSQLitePath(s.cfg.DataDir, p.SQLitePath)
	if mode == "mysql" && dsn == "" {
		writeJSON(w, 400, apiErr("bad_request", "MySQL/MariaDB 模式必须填写 DSN"))
		return
	}
	if strings.TrimSpace(p.BaseURL) != "" {
		if err := validateHTTPURL(strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")); err != nil {
			writeJSON(w, 400, apiErr("bad_request", "站点域名无效："+err.Error()))
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	targetStore := s.store()
	if mode != s.cfg.DatabaseMode || (mode == "mysql" && dsn != s.cfg.DSN) || (mode == "embedded" && sqlitePath != s.cfg.SQLitePath) {
		db, err := dbutil.Open(ctx, mode, dsn, sqlitePath)
		if err != nil {
			writeJSON(w, 400, apiErr("db", err.Error()))
			return
		}
		if err := dbutil.Migrate(ctx, db, mode); err != nil {
			_ = db.Close()
			writeJSON(w, 400, apiErr("migrate", err.Error()))
			return
		}
		targetStore = store.New(db, mode)
	}
	if err := dbutil.Migrate(ctx, targetStore.DB(), mode); err != nil {
		writeJSON(w, 400, apiErr("migrate", err.Error()))
		return
	}
	appNameZH := firstNonEmpty(p.AppNameZH, p.AppName, "AI短链平台")
	appNameEN := firstNonEmpty(p.AppNameEN, "AI Shortlink")
	settings := model.SystemSettings{Installed: true, AppName: appNameZH, AppNameZH: appNameZH, AppNameEN: appNameEN, BaseURL: strings.TrimRight(strings.TrimSpace(p.BaseURL), "/"), DefaultLocale: firstNonEmpty(p.DefaultLocale, "zh-CN"), LoginMode: "hybrid", AdminEmail: p.AdminEmail, SMTPEnabled: p.SMTPEnabled, SMTPHost: strings.TrimSpace(p.SMTPHost), SMTPPort: p.SMTPPort, SMTPSecurity: firstNonEmpty(p.SMTPSecurity, "tls"), SMTPUsername: strings.TrimSpace(p.SMTPUsername), SMTPFrom: strings.TrimSpace(p.SMTPFrom)}
	if settings.SMTPEnabled {
		if settings.SMTPHost == "" || settings.SMTPFrom == "" || settings.SMTPPort <= 0 {
			writeJSON(w, 400, apiErr("bad_request", "启用 SMTP 时必须填写主机、端口和发信邮箱"))
			return
		}
		if !validEmail(settings.SMTPFrom) {
			writeJSON(w, 400, apiErr("bad_request", "SMTP 发信邮箱格式不正确"))
			return
		}
	}
	vals := settingsToMap(settings)
	if strings.TrimSpace(p.SMTPPassword) != "" {
		cipher, err := s.auth.Encrypt(p.SMTPPassword)
		if err != nil {
			writeJSON(w, 500, apiErr("crypto", err.Error()))
			return
		}
		vals["smtp_password_cipher"] = cipher
	}
	count, err := targetStore.CountAdminAccounts(ctx)
	if err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	if count > 0 {
		writeJSON(w, http.StatusConflict, apiErr("account_exists", "目标数据库已存在管理员账户，请确认数据库是否已经安装过"))
		return
	}
	recoveryKey, err := auth.NewRecoveryKey()
	if err != nil {
		writeJSON(w, 500, apiErr("crypto", err.Error()))
		return
	}
	cipherText, err := s.auth.Encrypt(recoveryKey)
	if err != nil {
		writeJSON(w, 500, apiErr("crypto", err.Error()))
		return
	}
	acct, err := targetStore.CreateAdminAccount(ctx, p.AdminEmail, p.AdminName, s.auth.RecoveryHash(recoveryKey), cipherText)
	if err != nil {
		writeJSON(w, 500, apiErr("db", friendlyDBErr(err)))
		return
	}
	if err := targetStore.SetSettings(ctx, vals); err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	rc := config.RuntimeConfig{DatabaseMode: mode, DSN: dsn, SQLitePath: sqlitePath, AppSecret: s.cfg.AppSecret, TrustProxy: s.cfg.TrustProxy, CookieSecure: s.cfg.CookieSecure}
	if err := config.SaveRuntime(s.cfg.DataDir, rc); err != nil {
		writeJSON(w, 500, apiErr("config", err.Error()))
		return
	}
	s.cfg = s.cfg.WithRuntime(rc)
	s.setStore(targetStore)

	ip := util.ClientIP(r, s.cfg.TrustProxy)
	browserID, err := s.auth.EnsureBrowserID(w, r)
	if err != nil {
		writeJSON(w, 500, apiErr("browser", err.Error()))
		return
	}
	dev, err := targetStore.CreateAdminDevice(ctx, acct.ID, "安装浏览器", s.auth.Hash(browserID), s.auth.Hash(util.NormalizeIPForLogin(ip)), ip, util.Truncate(r.UserAgent(), 512))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, 500, apiErr("db", err.Error()))
			return
		}
		writeJSON(w, 500, apiErr("db", friendlyDBErr(err)))
		return
	}
	s.auth.SetSession(w, dev.ID, browserID)
	_ = targetStore.Audit(ctx, &dev.ID, "system.install", "system", nil, mode, ip)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "redirect": "/admin", "recovery_key": recoveryKey})
}

func normalizeSQLitePath(dataDir, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		p = filepath.Join(dataDir, "ai-shortlink.db")
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dataDir, p)
	}
	return filepath.Clean(p)
}
