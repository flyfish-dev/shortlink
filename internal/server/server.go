package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"ai-shortlink/internal/auth"
	"ai-shortlink/internal/config"
	"ai-shortlink/internal/model"
	"ai-shortlink/internal/store"
	"ai-shortlink/internal/util"
	webassets "ai-shortlink/web"
)

type ctxKey string

const deviceIDKey ctxKey = "deviceID"

type actorInfo struct {
	Device  *model.AdminDevice
	Account *model.AdminAccount
}

func (a *actorInfo) IsAdmin() bool { return a != nil && a.Account != nil && a.Account.Role == "admin" }

type Server struct {
	cfg        config.Config
	storeValue atomic.Value // stores *store.Store
	auth       *auth.Manager
	tpl        *template.Template
}

func New(cfg config.Config, st *store.Store) (*Server, error) {
	t, err := template.ParseFS(webassets.FS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "uploads"), 0755); err != nil {
		return nil, err
	}
	srv := &Server{cfg: cfg, auth: auth.NewManager(cfg.AppSecret, cfg.SessionTTL, cfg.CookieSecure), tpl: t}
	srv.setStore(st)
	return srv, nil
}

func (s *Server) store() *store.Store {
	v := s.storeValue.Load()
	if v == nil {
		return nil
	}
	return v.(*store.Store)
}

func (s *Server) setStore(st *store.Store) {
	s.storeValue.Store(st)
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, map[string]any{"ok": true}) })
	mux.HandleFunc("/favicon.svg", s.favicon)
	mux.HandleFunc("/api/public/bootstrap", s.publicBootstrap)
	mux.HandleFunc("/setup", s.setupPage)
	mux.HandleFunc("/api/setup/test-db", s.setupTestDB)
	mux.HandleFunc("/api/setup/install", s.setupInstall)
	mux.HandleFunc("/login", s.loginPage)
	mux.HandleFunc("/auth/one-click", s.oneClickLogin)
	mux.HandleFunc("/auth/magic/request", s.magicRequest)
	mux.HandleFunc("/auth/magic/consume", s.magicConsume)
	mux.HandleFunc("/auth/recover", s.recoverLogin)
	mux.HandleFunc("/auth/logout", s.logout)

	staticFS, _ := fs.Sub(webassets.FS, "static")
	mux.Handle("/assets/", http.StripPrefix("/assets/", cacheStatic(http.FileServer(http.FS(staticFS)))))
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", cacheStatic(http.FileServer(http.Dir(filepath.Join(s.cfg.DataDir, "uploads"))))))

	mux.Handle("/admin", s.requireAuthPage(http.HandlerFunc(s.adminPage)))
	mux.Handle("/admin/", s.requireAuthPage(http.HandlerFunc(s.adminPage)))
	for _, prefix := range []string{"/api/admin/users", "/api/admin/users/", "/api/admin/settings", "/api/admin/overview", "/api/admin/short-links", "/api/admin/short-links/", "/api/admin/live-qrs", "/api/admin/live-qrs/", "/api/admin/live-qr-items", "/api/admin/live-qr-items/"} {
		mux.Handle(prefix, s.requireAuthAPI(http.HandlerFunc(s.adminAPIExt)))
	}
	mux.Handle("/api/admin/", s.requireAuthAPI(http.HandlerFunc(s.adminAPI)))
	mux.HandleFunc("/api/public/live-longpress/", s.publicLiveLongpress)

	mux.HandleFunc("/qr/short/", s.shortQRCode)
	mux.HandleFunc("/qr/live/", s.liveQRCode)
	mux.HandleFunc("/s/", s.shortRedirect)
	mux.HandleFunc("/q/", s.liveQRPublic)
	mux.HandleFunc("/", s.root)

	return recoverer(logger(securityHeaders(mux)))
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/login" {
		http.NotFound(w, r)
		return
	}
	if !s.isInstalled(r.Context()) {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	if s.isLoggedIn(r) {
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}
	st := s.settings(r.Context())
	s.render(w, r, "login.html", map[string]any{"AppName": st.AppName, "Settings": st})
}

func (s *Server) adminPage(w http.ResponseWriter, r *http.Request) {
	st := s.settings(r.Context())
	s.render(w, r, "admin.html", map[string]any{"AppName": st.AppName, "BaseURL": s.publicBaseURL(r), "Settings": st})
}

func (s *Server) root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		if !s.isInstalled(r.Context()) {
			http.Redirect(w, r, "/setup", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}
	code := strings.Trim(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if strings.Contains(code, "/") || code == "" {
		http.NotFound(w, r)
		return
	}
	s.redirectCode(w, r, code)
}

func (s *Server) oneClickLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method_not_allowed", "仅支持 POST"))
		return
	}
	if !s.isInstalled(r.Context()) {
		writeJSON(w, http.StatusPreconditionRequired, apiErr("setup_required", "系统尚未完成安装"))
		return
	}
	if s.settings(r.Context()).LoginMode == "magic" {
		writeJSON(w, http.StatusForbidden, apiErr("magic_required", "当前系统设置为 Magic Link 登录，请使用邮箱登录。"))
		return
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&struct{}{})

	ip := util.ClientIP(r, s.cfg.TrustProxy)
	browserID, err := s.auth.EnsureBrowserID(w, r)
	if err != nil {
		writeJSON(w, 500, apiErr("browser", err.Error()))
		return
	}
	browserHash := s.auth.Hash(browserID)
	ipHash := s.auth.Hash(util.NormalizeIPForLogin(ip))
	ctx := r.Context()

	dev, err := s.store().FindAdminDeviceByBrowserHash(ctx, browserHash)
	if err == nil {
		acct, recoveryKey, err := s.ensureDeviceAccount(ctx, dev)
		if err != nil {
			writeJSON(w, 500, apiErr("account", err.Error()))
			return
		}
		_ = acct
		_ = s.store().TouchAdminDevice(ctx, dev.ID, ip, util.Truncate(r.UserAgent(), 512))
		s.auth.SetSession(w, dev.ID, browserID)
		resp := map[string]any{"ok": true, "redirect": "/admin"}
		if recoveryKey != "" {
			resp["recovery_key"] = recoveryKey
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if !errors.Is(err, store.ErrNotFound) {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}

	accountCount, err := s.store().CountAdminAccounts(ctx)
	if err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	if accountCount > 0 {
		writeJSON(w, http.StatusUnauthorized, apiErr("recovery_required", "当前浏览器还没有登录令牌。请用后台里保存的恢复 Key 绑定此浏览器。"))
		return
	}

	acct, recoveryKey, err := s.createAdminAccount(ctx)
	if err != nil {
		writeJSON(w, 500, apiErr("account", err.Error()))
		return
	}
	dev, err = s.store().CreateAdminDevice(ctx, acct.ID, "自动登录设备", browserHash, ipHash, ip, util.Truncate(r.UserAgent(), 512))
	if err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	s.auth.SetSession(w, dev.ID, browserID)
	_ = s.store().Audit(ctx, &dev.ID, "admin_account.create", "admin_account", &acct.ID, "first one-click login", ip)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "redirect": "/admin", "new_account": true, "recovery_key": recoveryKey})
}

func (s *Server) recoverLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method_not_allowed", "仅支持 POST"))
		return
	}
	var req struct {
		RecoveryKey string `json:"recovery_key"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req)
	recoveryKey := auth.NormalizeRecoveryKey(req.RecoveryKey)
	if recoveryKey == "" {
		writeJSON(w, http.StatusBadRequest, apiErr("bad_request", "请填写恢复 Key"))
		return
	}

	ctx := r.Context()
	acct, err := s.store().FindAdminAccountByRecoveryHash(ctx, s.auth.RecoveryHash(recoveryKey))
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusUnauthorized, apiErr("bad_key", "恢复 Key 不正确"))
		return
	}
	if err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}

	ip := util.ClientIP(r, s.cfg.TrustProxy)
	browserID, err := s.auth.EnsureBrowserID(w, r)
	if err != nil {
		writeJSON(w, 500, apiErr("browser", err.Error()))
		return
	}
	browserHash := s.auth.Hash(browserID)
	ipHash := s.auth.Hash(util.NormalizeIPForLogin(ip))
	dev, err := s.store().FindAdminDeviceByBrowserHash(ctx, browserHash)
	if errors.Is(err, store.ErrNotFound) {
		dev, err = s.store().CreateAdminDevice(ctx, acct.ID, "恢复绑定设备", browserHash, ipHash, ip, util.Truncate(r.UserAgent(), 512))
	} else if err == nil {
		if dev.AccountID != acct.ID {
			if err := s.store().UpdateAdminDeviceAccount(ctx, dev.ID, acct.ID); err != nil {
				writeJSON(w, 500, apiErr("db", err.Error()))
				return
			}
			dev.AccountID = acct.ID
		}
		_ = s.store().TouchAdminDevice(ctx, dev.ID, ip, util.Truncate(r.UserAgent(), 512))
	}
	if err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}

	s.auth.SetSession(w, dev.ID, browserID)
	_ = s.store().Audit(ctx, &dev.ID, "admin_account.recover", "admin_account", &acct.ID, "recovery key login", ip)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "redirect": "/admin"})
}

func (s *Server) createAdminAccount(ctx context.Context, emailAndName ...string) (*model.AdminAccount, string, error) {
	recoveryKey, err := auth.NewRecoveryKey()
	if err != nil {
		return nil, "", err
	}
	cipherText, err := s.auth.Encrypt(recoveryKey)
	if err != nil {
		return nil, "", err
	}
	email, name := "", ""
	if len(emailAndName) > 0 {
		email = strings.TrimSpace(emailAndName[0])
	}
	if len(emailAndName) > 1 {
		name = strings.TrimSpace(emailAndName[1])
	}
	acct, err := s.store().CreateAdminAccount(ctx, email, name, s.auth.RecoveryHash(recoveryKey), cipherText)
	if err != nil {
		return nil, "", err
	}
	return acct, recoveryKey, nil
}

func (s *Server) createUserAccount(ctx context.Context, email, name string) (*model.AdminAccount, string, error) {
	recoveryKey, err := auth.NewRecoveryKey()
	if err != nil {
		return nil, "", err
	}
	cipherText, err := s.auth.Encrypt(recoveryKey)
	if err != nil {
		return nil, "", err
	}
	acct, err := s.store().CreateUserAccount(ctx, strings.TrimSpace(email), strings.TrimSpace(name), s.auth.RecoveryHash(recoveryKey), cipherText)
	if err != nil {
		return nil, "", err
	}
	return acct, recoveryKey, nil
}

func (s *Server) ensureDeviceAccount(ctx context.Context, dev *model.AdminDevice) (*model.AdminAccount, string, error) {
	if dev.AccountID != 0 {
		acct, err := s.store().GetAdminAccount(ctx, dev.AccountID)
		return acct, "", err
	}
	acct, recoveryKey, err := s.createAdminAccount(ctx)
	if err != nil {
		return nil, "", err
	}
	if err := s.store().UpdateAdminDeviceAccount(ctx, dev.ID, acct.ID); err != nil {
		return nil, "", err
	}
	dev.AccountID = acct.ID
	return acct, recoveryKey, nil
}

func (s *Server) recoveryKeyForAccount(acct *model.AdminAccount) string {
	if acct == nil || strings.TrimSpace(acct.RecoveryKeyCipher) == "" {
		return ""
	}
	key, err := s.auth.Decrypt(acct.RecoveryKeyCipher)
	if err != nil {
		return ""
	}
	return key
}

func (s *Server) rotateAccountRecoveryKey(ctx context.Context, accountID int64) (*model.AdminAccount, string, error) {
	recoveryKey, err := auth.NewRecoveryKey()
	if err != nil {
		return nil, "", err
	}
	cipherText, err := s.auth.Encrypt(recoveryKey)
	if err != nil {
		return nil, "", err
	}
	acct, err := s.store().UpdateAdminAccountRecoveryKey(ctx, accountID, s.auth.RecoveryHash(recoveryKey), cipherText)
	if err != nil {
		return nil, "", err
	}
	return acct, recoveryKey, nil
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	s.auth.Clear(w)
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (s *Server) requireAuthPage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.isInstalled(r.Context()) {
			http.Redirect(w, r, "/setup", http.StatusFound)
			return
		}
		sess, ok := s.validateSession(r)
		if !ok || sess == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), deviceIDKey, sess.DeviceID)))
	})
}

func (s *Server) requireAuthAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.isInstalled(r.Context()) {
			writeJSON(w, http.StatusPreconditionRequired, apiErr("setup_required", "系统尚未完成安装"))
			return
		}
		sess, ok := s.validateSession(r)
		if !ok || sess == nil {
			writeJSON(w, http.StatusUnauthorized, apiErr("unauthorized", "登录已过期或当前浏览器未绑定"))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), deviceIDKey, sess.DeviceID)))
	})
}

func (s *Server) isLoggedIn(r *http.Request) bool {
	_, ok := s.validateSession(r)
	return ok
}

func (s *Server) validateSession(r *http.Request) (*auth.Session, bool) {
	sess, err := s.auth.ReadSession(r)
	if err != nil {
		return nil, false
	}
	browserID := s.auth.BrowserID(r)
	if browserID == "" || browserID != sess.BrowserID {
		return nil, false
	}
	dev, err := s.store().GetAdminDevice(r.Context(), sess.DeviceID)
	if err != nil || dev.BrowserHash != s.auth.Hash(browserID) {
		return nil, false
	}
	if dev.AccountID != 0 {
		acct, err := s.store().GetAdminAccount(r.Context(), dev.AccountID)
		if err != nil || acct.Status != "active" {
			return nil, false
		}
	}
	ip := util.ClientIP(r, s.cfg.TrustProxy)
	_ = s.store().TouchAdminDevice(r.Context(), sess.DeviceID, ip, util.Truncate(r.UserAgent(), 512))
	return sess, true
}

func (s *Server) currentActor(ctx context.Context) (*actorInfo, error) {
	id := deviceIDFromContext(ctx)
	if id == nil {
		return nil, store.ErrNotFound
	}
	dev, err := s.store().GetAdminDevice(ctx, *id)
	if err != nil {
		return nil, err
	}
	acct, _, err := s.ensureDeviceAccount(ctx, dev)
	if err != nil {
		return nil, err
	}
	if acct.Status != "active" {
		return nil, store.ErrNotFound
	}
	return &actorInfo{Device: dev, Account: acct}, nil
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template %s: %v", name, err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func apiErr(code, msg string) map[string]any {
	return map[string]any{"ok": false, "error": code, "message": msg}
}

func cacheStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start).Truncate(time.Millisecond))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Printf("panic: %v", v)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func pathID(path, prefix string) (int64, string, error) {
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return 0, "", fmt.Errorf("missing id")
	}
	parts := strings.SplitN(rest, "/", 2)
	id, err := strconv.ParseInt(parts[0], 10, 64)
	tail := ""
	if len(parts) > 1 {
		tail = parts[1]
	}
	return id, tail, err
}

func deviceIDFromContext(ctx context.Context) *int64 {
	if v, ok := ctx.Value(deviceIDKey).(int64); ok {
		return &v
	}
	return nil
}
