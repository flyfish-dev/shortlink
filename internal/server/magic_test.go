package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
