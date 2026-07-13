package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-shortlink/internal/config"
	"ai-shortlink/internal/dbutil"
	"ai-shortlink/internal/store"
)

func TestMagicRequestDeduplicatesActiveEmailToken(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	sqlitePath := filepath.Join(dataDir, "shortlink.db")
	db, err := dbutil.Open(ctx, "embedded", "", sqlitePath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := dbutil.Migrate(ctx, db, "embedded"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db, "embedded")
	if err := st.SetSettings(ctx, map[string]string{
		"installed":            "1",
		"app_name":             "AI短链平台",
		"app_name_zh":          "AI短链平台",
		"app_name_en":          "AI Shortlink",
		"default_locale":       "zh-CN",
		"login_mode":           "hybrid",
		"smtp_enabled":         "1",
		"smtp_host":            "smtp.example.com",
		"smtp_port":            "465",
		"smtp_security":        "tls",
		"smtp_from":            "no-reply@example.com",
		"smtp_password_cipher": "configured",
	}); err != nil {
		t.Fatalf("set settings: %v", err)
	}
	acct, err := st.CreateAdminAccount(ctx, "admin@example.com", "Admin", "recovery-hash", "recovery-cipher")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if _, err := st.CreateMagicLoginToken(ctx, acct.ID, acct.Email, "existing-token", time.Now().Add(15*time.Minute), "127.0.0.1"); err != nil {
		t.Fatalf("create token: %v", err)
	}
	srv, err := New(config.Config{AppName: "AI短链平台", DataDir: dataDir, DatabaseMode: "embedded", SQLitePath: sqlitePath, AppSecret: "test-secret", SessionTTL: time.Hour}, st)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/magic/request", strings.NewReader(`{"email":"ADMIN@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.magicRequest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["ok"] != true || body["duplicate"] != true {
		t.Fatalf("response = %#v, want duplicate ok", body)
	}
	var tokenCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM magic_login_tokens`).Scan(&tokenCount); err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if tokenCount != 1 {
		t.Fatalf("token count = %d, want 1", tokenCount)
	}
}

func TestMagicConsumeRequiresExplicitPostBeforeUsingToken(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	sqlitePath := filepath.Join(dataDir, "shortlink.db")
	db, err := dbutil.Open(ctx, "embedded", "", sqlitePath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := dbutil.Migrate(ctx, db, "embedded"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db, "embedded")
	if err := st.SetSettings(ctx, map[string]string{
		"installed":      "1",
		"app_name":       "AI短链平台",
		"app_name_zh":    "AI短链平台",
		"app_name_en":    "AI Shortlink",
		"default_locale": "zh-CN",
		"login_mode":     "hybrid",
	}); err != nil {
		t.Fatalf("set settings: %v", err)
	}
	acct, err := st.CreateAdminAccount(ctx, "admin@example.com", "Admin", "recovery-hash", "recovery-cipher")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	srv, err := New(config.Config{AppName: "AI短链平台", DataDir: dataDir, DatabaseMode: "embedded", SQLitePath: sqlitePath, AppSecret: "test-secret", SessionTTL: time.Hour}, st)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	const rawToken = "magic-token-from-email"
	created, err := st.CreateMagicLoginToken(ctx, acct.ID, acct.Email, srv.auth.Hash("magic:"+rawToken), time.Now().Add(15*time.Minute), "127.0.0.1")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/auth/magic/consume?token="+url.QueryEscape(rawToken), nil)
	getRR := httptest.NewRecorder()
	srv.magicConsume(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", getRR.Code, getRR.Body.String())
	}
	if !strings.Contains(getRR.Body.String(), "确认邮箱登录") {
		t.Fatalf("GET body does not contain confirmation page: %s", getRR.Body.String())
	}
	if got := getRR.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Fatalf("GET Cache-Control = %q, want no-store", got)
	}
	if len(getRR.Result().Cookies()) != 0 {
		t.Fatalf("GET should not bind a browser or set cookies")
	}
	afterGet, err := st.GetMagicLoginToken(ctx, created.ID)
	if err != nil {
		t.Fatalf("get token after GET: %v", err)
	}
	if afterGet.UsedAt != nil {
		t.Fatalf("GET consumed token at %v", afterGet.UsedAt)
	}

	form := url.Values{"token": {rawToken}}
	postReq := httptest.NewRequest(http.MethodPost, "/auth/magic/consume", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.RemoteAddr = "127.0.0.1:1234"
	postRR := httptest.NewRecorder()
	srv.magicConsume(postRR, postReq)
	if postRR.Code != http.StatusFound {
		t.Fatalf("POST status = %d, body = %s", postRR.Code, postRR.Body.String())
	}
	if location := postRR.Header().Get("Location"); location != "/admin" {
		t.Fatalf("POST redirect = %q, want /admin", location)
	}
	afterPost, err := st.GetMagicLoginToken(ctx, created.ID)
	if err != nil {
		t.Fatalf("get token after POST: %v", err)
	}
	if afterPost.UsedAt == nil {
		t.Fatalf("POST did not consume token")
	}
}
