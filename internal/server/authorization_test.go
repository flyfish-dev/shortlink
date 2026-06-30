package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-shortlink/internal/auth"
	"ai-shortlink/internal/config"
	"ai-shortlink/internal/dbutil"
	"ai-shortlink/internal/model"
	"ai-shortlink/internal/store"
)

type authorizationFixture struct {
	handler http.Handler
	srv     *Server
	st      *store.Store

	admin *model.AdminAccount
	owner *model.AdminAccount
	other *model.AdminAccount

	adminCookies []*http.Cookie
	ownerCookies []*http.Cookie

	ownerShort *model.ShortLink
	otherShort *model.ShortLink
	ownerLive  *model.LiveQR
	otherLive  *model.LiveQR
	ownerItem  *model.LiveQRItem
	otherItem  *model.LiveQRItem
}

func newAuthorizationFixture(t *testing.T) authorizationFixture {
	t.Helper()
	ctx := context.Background()
	dataDir := t.TempDir()
	sqlitePath := filepath.Join(dataDir, "shortlink.db")
	db, err := dbutil.Open(ctx, "embedded", "", sqlitePath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := dbutil.Migrate(ctx, db, "embedded"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db, "embedded")
	if err := st.SetSettings(ctx, map[string]string{
		"installed":      "1",
		"app_name":       "AI短链平台",
		"app_name_zh":    "AI短链平台",
		"app_name_en":    "AI Shortlink",
		"admin_email":    "admin@example.com",
		"default_locale": "zh-CN",
		"login_mode":     "hybrid",
	}); err != nil {
		t.Fatalf("set settings: %v", err)
	}
	srv, err := New(config.Config{AppName: "AI短链平台", DataDir: dataDir, DatabaseMode: "embedded", SQLitePath: sqlitePath, AppSecret: "authz-test-secret", SessionTTL: time.Hour}, st)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	admin := mustCreateAccount(t, st, "admin@example.com", "Admin", "admin")
	owner := mustCreateAccount(t, st, "owner@example.com", "Owner", "user")
	other := mustCreateAccount(t, st, "other@example.com", "Other", "user")

	ownerShort := mustCreateShort(t, st, owner.ID, "owner-short", "Owner short")
	otherShort := mustCreateShort(t, st, other.ID, "other-short", "Other short")
	ownerLive := mustCreateLive(t, st, owner.ID, "owner-live", "Owner live")
	otherLive := mustCreateLive(t, st, other.ID, "other-live", "Other live")
	ownerItem := mustCreateLiveItem(t, st, ownerLive.ID, "Owner item")
	otherItem := mustCreateLiveItem(t, st, otherLive.ID, "Other item")

	return authorizationFixture{
		handler:      srv.Routes(),
		srv:          srv,
		st:           st,
		admin:        admin,
		owner:        owner,
		other:        other,
		adminCookies: authzCookies(t, srv, st, admin),
		ownerCookies: authzCookies(t, srv, st, owner),
		ownerShort:   ownerShort,
		otherShort:   otherShort,
		ownerLive:    ownerLive,
		otherLive:    otherLive,
		ownerItem:    ownerItem,
		otherItem:    otherItem,
	}
}

func mustCreateAccount(t *testing.T, st *store.Store, email, name, role string) *model.AdminAccount {
	t.Helper()
	acct, err := st.CreateAccount(context.Background(), email, name, role, "active", "hash-"+email, "cipher-"+email)
	if err != nil {
		t.Fatalf("create account %s: %v", email, err)
	}
	return acct
}

func mustCreateShort(t *testing.T, st *store.Store, ownerID int64, code, title string) *model.ShortLink {
	t.Helper()
	item, err := st.CreateShortLink(context.Background(), &model.ShortLink{
		OwnerAccountID: ownerID,
		Code:           code,
		Title:          title,
		TargetURL:      "https://example.com/" + code,
		Status:         "active",
		RedirectType:   302,
	})
	if err != nil {
		t.Fatalf("create short %s: %v", code, err)
	}
	return item
}

func mustCreateLive(t *testing.T, st *store.Store, ownerID int64, code, title string) *model.LiveQR {
	t.Helper()
	item, err := st.CreateLiveQR(context.Background(), &model.LiveQR{
		OwnerAccountID:   ownerID,
		Code:             code,
		Title:            title,
		Status:           "active",
		RotationStrategy: "round_robin",
		GuideTitle:       "Guide",
		GuideText:        "Scan the code",
	})
	if err != nil {
		t.Fatalf("create live %s: %v", code, err)
	}
	return item
}

func mustCreateLiveItem(t *testing.T, st *store.Store, liveID int64, title string) *model.LiveQRItem {
	t.Helper()
	item, err := st.CreateLiveQRItem(context.Background(), &model.LiveQRItem{
		LiveQRID:   liveID,
		Title:      title,
		QRImageURL: "https://example.com/qr.png",
		TargetURL:  "https://example.com/target",
		Status:     "active",
		SortOrder:  100,
		Weight:     1,
	})
	if err != nil {
		t.Fatalf("create live item %s: %v", title, err)
	}
	return item
}

func authzCookies(t *testing.T, srv *Server, st *store.Store, acct *model.AdminAccount) []*http.Cookie {
	t.Helper()
	browserID := fmt.Sprintf("browser-%d-abcdefghijklmnopqrstuvwxyz", acct.ID)
	dev, err := st.CreateAdminDevice(context.Background(), acct.ID, "test browser", srv.auth.Hash(browserID), srv.auth.Hash("127.0.0.1"), "127.0.0.1", "authorization-test")
	if err != nil {
		t.Fatalf("create device: %v", err)
	}
	rr := httptest.NewRecorder()
	srv.auth.SetSession(rr, dev.ID, browserID)
	cookies := rr.Result().Cookies()
	cookies = append(cookies, &http.Cookie{Name: auth.BrowserCookie, Value: browserID})
	return cookies
}

func authzRequest(t *testing.T, handler http.Handler, method, path, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func requireStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rr.Code != want {
		t.Fatalf("status = %d, want %d, body = %s", rr.Code, want, rr.Body.String())
	}
}

func listedIDs(t *testing.T, rr *httptest.ResponseRecorder) []int64 {
	t.Helper()
	var body struct {
		OK   bool `json:"ok"`
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rr.Body.String())
	}
	if !body.OK {
		t.Fatalf("response ok=false: %s", rr.Body.String())
	}
	ids := make([]int64, 0, len(body.Data))
	for _, item := range body.Data {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestUserSeesOnlyOwnedContentAndAdminSeesAll(t *testing.T) {
	fx := newAuthorizationFixture(t)

	rr := authzRequest(t, fx.handler, http.MethodGet, "/api/admin/short-links", "", fx.ownerCookies)
	requireStatus(t, rr, http.StatusOK)
	if got := listedIDs(t, rr); len(got) != 1 || got[0] != fx.ownerShort.ID {
		t.Fatalf("user short list ids = %v, want only %d", got, fx.ownerShort.ID)
	}

	rr = authzRequest(t, fx.handler, http.MethodGet, "/api/admin/live-qrs", "", fx.ownerCookies)
	requireStatus(t, rr, http.StatusOK)
	if got := listedIDs(t, rr); len(got) != 1 || got[0] != fx.ownerLive.ID {
		t.Fatalf("user live list ids = %v, want only %d", got, fx.ownerLive.ID)
	}

	rr = authzRequest(t, fx.handler, http.MethodGet, "/api/admin/short-links", "", fx.adminCookies)
	requireStatus(t, rr, http.StatusOK)
	if got := listedIDs(t, rr); len(got) != 2 {
		t.Fatalf("admin short list ids = %v, want both records", got)
	}

	rr = authzRequest(t, fx.handler, http.MethodGet, "/api/admin/live-qrs", "", fx.adminCookies)
	requireStatus(t, rr, http.StatusOK)
	if got := listedIDs(t, rr); len(got) != 2 {
		t.Fatalf("admin live list ids = %v, want both records", got)
	}
}

func TestUserCannotAccessOrMutateOtherUsersShortLinks(t *testing.T) {
	fx := newAuthorizationFixture(t)
	other := fmt.Sprintf("/api/admin/short-links/%d", fx.otherShort.ID)
	update := `{"code":"other-short-hijack","title":"Hijack","target_url":"https://example.com/hijack","status":"active","redirect_type":302}`

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, other, ""},
		{http.MethodPut, other, update},
		{http.MethodDelete, other, ""},
		{http.MethodGet, other + "/stats", ""},
		{http.MethodPost, other + "/review", `{"status":"approved"}`},
	} {
		rr := authzRequest(t, fx.handler, tc.method, tc.path, tc.body, fx.ownerCookies)
		requireStatus(t, rr, http.StatusForbidden)
	}

	own := fmt.Sprintf("/api/admin/short-links/%d", fx.ownerShort.ID)
	rr := authzRequest(t, fx.handler, http.MethodPut, own, `{"code":"owner-short-updated","title":"Updated","target_url":"https://example.com/updated","status":"active","redirect_type":302}`, fx.ownerCookies)
	requireStatus(t, rr, http.StatusOK)

	rr = authzRequest(t, fx.handler, http.MethodPut, other, update, fx.adminCookies)
	requireStatus(t, rr, http.StatusOK)
}

func TestUserCannotAccessOrMutateOtherUsersLiveQRs(t *testing.T) {
	fx := newAuthorizationFixture(t)
	other := fmt.Sprintf("/api/admin/live-qrs/%d", fx.otherLive.ID)
	update := `{"code":"other-live-hijack","title":"Hijack","status":"active","rotation_strategy":"round_robin","guide_title":"Guide","guide_text":"Text"}`

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, other, ""},
		{http.MethodPut, other, update},
		{http.MethodDelete, other, ""},
		{http.MethodGet, other + "/items", ""},
		{http.MethodPost, other + "/items", `{"title":"Bad","qr_image_url":"https://example.com/qr.png","target_url":"https://example.com/bad","status":"active"}`},
		{http.MethodGet, other + "/stats", ""},
		{http.MethodPost, other + "/review", `{"status":"approved","include_items":true}`},
	} {
		rr := authzRequest(t, fx.handler, tc.method, tc.path, tc.body, fx.ownerCookies)
		requireStatus(t, rr, http.StatusForbidden)
	}

	own := fmt.Sprintf("/api/admin/live-qrs/%d", fx.ownerLive.ID)
	rr := authzRequest(t, fx.handler, http.MethodPut, own, `{"code":"owner-live-updated","title":"Updated","status":"active","rotation_strategy":"round_robin","guide_title":"Guide","guide_text":"Text"}`, fx.ownerCookies)
	requireStatus(t, rr, http.StatusOK)

	rr = authzRequest(t, fx.handler, http.MethodPut, other, update, fx.adminCookies)
	requireStatus(t, rr, http.StatusOK)
}

func TestUserCannotAccessOrReviewOtherUsersLiveQRItems(t *testing.T) {
	fx := newAuthorizationFixture(t)
	otherItem := fmt.Sprintf("/api/admin/live-qr-items/%d", fx.otherItem.ID)
	ownItem := fmt.Sprintf("/api/admin/live-qr-items/%d", fx.ownerItem.ID)
	update := `{"title":"Updated item","qr_image_url":"https://example.com/updated.png","target_url":"https://example.com/updated","status":"active","sort_order":101,"weight":2}`

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPut, otherItem, update},
		{http.MethodDelete, otherItem, ""},
		{http.MethodPost, otherItem, `{"status":"approved"}`},
	} {
		rr := authzRequest(t, fx.handler, tc.method, tc.path, tc.body, fx.ownerCookies)
		requireStatus(t, rr, http.StatusForbidden)
	}

	rr := authzRequest(t, fx.handler, http.MethodPut, ownItem, update, fx.ownerCookies)
	requireStatus(t, rr, http.StatusOK)

	rr = authzRequest(t, fx.handler, http.MethodPost, ownItem, `{"status":"approved"}`, fx.ownerCookies)
	requireStatus(t, rr, http.StatusForbidden)

	rr = authzRequest(t, fx.handler, http.MethodPost, otherItem, `{"status":"approved"}`, fx.adminCookies)
	requireStatus(t, rr, http.StatusOK)
}

func TestUserCannotUseAdminOnlyAPIsOrMutateGlobalAdminEmail(t *testing.T) {
	fx := newAuthorizationFixture(t)

	for _, path := range []string{"/api/admin/settings", "/api/admin/users"} {
		rr := authzRequest(t, fx.handler, http.MethodGet, path, "", fx.ownerCookies)
		requireStatus(t, rr, http.StatusForbidden)
	}

	rr := authzRequest(t, fx.handler, http.MethodPut, "/api/admin/account", `{"email":"owner-new@example.com","name":"Owner New"}`, fx.ownerCookies)
	requireStatus(t, rr, http.StatusOK)
	settings, err := fx.st.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if got := settings["admin_email"]; got != "admin@example.com" {
		t.Fatalf("admin_email = %q after user account update, want unchanged", got)
	}

	rr = authzRequest(t, fx.handler, http.MethodPut, "/api/admin/account", `{"email":"admin-new@example.com","name":"Admin New"}`, fx.adminCookies)
	requireStatus(t, rr, http.StatusOK)
	settings, err = fx.st.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if got := settings["admin_email"]; got != "admin-new@example.com" {
		t.Fatalf("admin_email = %q after admin account update, want updated", got)
	}
}
