package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"ai-shortlink/internal/auth"
)

func TestGitHubOAuthBootstrapAndStart(t *testing.T) {
	fx := newAuthorizationFixture(t)

	rr := authzRequest(t, fx.handler, http.MethodGet, "/api/public/bootstrap", "", nil)
	requireStatus(t, rr, http.StatusOK)
	var boot map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &boot); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	if boot["github_auth_enabled"] != false {
		t.Fatalf("github_auth_enabled = %#v, want false", boot["github_auth_enabled"])
	}

	fx.srv.cfg.GitHubClientID = "client-id"
	fx.srv.cfg.GitHubClientSecret = "client-secret"
	fx.srv.cfg.BaseURL = "https://s.example.com"

	rr = authzRequest(t, fx.handler, http.MethodGet, "/api/public/bootstrap", "", nil)
	requireStatus(t, rr, http.StatusOK)
	boot = map[string]any{}
	if err := json.Unmarshal(rr.Body.Bytes(), &boot); err != nil {
		t.Fatalf("decode configured bootstrap: %v", err)
	}
	if boot["github_auth_enabled"] != true {
		t.Fatalf("github_auth_enabled = %#v, want true", boot["github_auth_enabled"])
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/github/start", nil)
	rr = httptest.NewRecorder()
	fx.handler.ServeHTTP(rr, req)
	requireStatus(t, rr, http.StatusFound)
	loc, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}
	if loc.Scheme != "https" || loc.Host != "github.com" || loc.Path != "/login/oauth/authorize" {
		t.Fatalf("redirect location = %s, want GitHub authorize URL", loc.String())
	}
	q := loc.Query()
	if q.Get("client_id") != "client-id" {
		t.Fatalf("client_id = %q, want configured value", q.Get("client_id"))
	}
	if q.Get("redirect_uri") != "https://s.example.com/auth/github/callback" {
		t.Fatalf("redirect_uri = %q", q.Get("redirect_uri"))
	}
	if !strings.Contains(q.Get("scope"), "user:email") {
		t.Fatalf("scope = %q, want user:email", q.Get("scope"))
	}
	if q.Get("state") == "" {
		t.Fatalf("state is empty")
	}
	if c := findCookie(rr.Result().Cookies(), githubOAuthStateCookie); c == nil || c.Value == "" || c.Path != "/auth/github" {
		t.Fatalf("state cookie = %#v, want oauth state cookie", c)
	}
}

func TestGitHubOAuthCallbackCreatesUserSession(t *testing.T) {
	fx := newAuthorizationFixture(t)
	fx.srv.cfg.GitHubClientID = "client-id"
	fx.srv.cfg.GitHubClientSecret = "client-secret"
	fx.srv.cfg.BaseURL = "https://s.example.com"

	fakeGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/oauth/access_token":
			if r.Method != http.MethodPost {
				t.Errorf("token method = %s, want POST", r.Method)
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse token form: %v", err)
				http.Error(w, "bad form", http.StatusBadRequest)
				return
			}
			if r.Form.Get("client_id") != "client-id" || r.Form.Get("code") != "oauth-code" {
				t.Errorf("token form = %#v", r.Form)
				http.Error(w, "bad token form", http.StatusBadRequest)
				return
			}
			if r.Form.Get("redirect_uri") != "https://s.example.com/auth/github/callback" {
				t.Errorf("redirect_uri = %q", r.Form.Get("redirect_uri"))
				http.Error(w, "bad redirect uri", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"github-token","token_type":"bearer","scope":"read:user,user:email"}`))
		case "/user":
			if !requireGitHubAuth(t, w, r) {
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"login":"octocat","name":"Octo User","email":""}`))
		case "/user/emails":
			if !requireGitHubAuth(t, w, r) {
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"email":"octo@example.com","primary":true,"verified":true}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer fakeGitHub.Close()

	oldTokenURL, oldUserURL, oldEmailsURL, oldClient := githubOAuthTokenURL, githubAPIUserURL, githubAPIEmailsURL, githubOAuthClient
	githubOAuthTokenURL = fakeGitHub.URL + "/login/oauth/access_token"
	githubAPIUserURL = fakeGitHub.URL + "/user"
	githubAPIEmailsURL = fakeGitHub.URL + "/user/emails"
	githubOAuthClient = fakeGitHub.Client()
	t.Cleanup(func() {
		githubOAuthTokenURL = oldTokenURL
		githubAPIUserURL = oldUserURL
		githubAPIEmailsURL = oldEmailsURL
		githubOAuthClient = oldClient
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=oauth-code&state=state-123", nil)
	req.AddCookie(&http.Cookie{Name: githubOAuthStateCookie, Value: "state-123"})
	rr := httptest.NewRecorder()
	fx.handler.ServeHTTP(rr, req)
	requireStatus(t, rr, http.StatusFound)
	if rr.Header().Get("Location") != "/admin" {
		t.Fatalf("redirect = %q, want /admin", rr.Header().Get("Location"))
	}

	acct, err := fx.st.FindAdminAccountByEmail(context.Background(), "octo@example.com")
	if err != nil {
		t.Fatalf("find github account: %v", err)
	}
	if acct.Role != "user" || acct.Status != "active" || acct.Name != "Octo User" {
		t.Fatalf("account = %#v, want active regular GitHub user", acct)
	}
	if findCookie(rr.Result().Cookies(), auth.BrowserCookie) == nil {
		t.Fatalf("missing browser cookie")
	}
	if findCookie(rr.Result().Cookies(), auth.SessionCookie) == nil {
		t.Fatalf("missing session cookie")
	}
	if cleared := findCookie(rr.Result().Cookies(), githubOAuthStateCookie); cleared == nil || cleared.MaxAge != -1 {
		t.Fatalf("state cookie was not cleared: %#v", cleared)
	}
}

func TestSelectGitHubEmailPrefersVerifiedPrimary(t *testing.T) {
	got := selectGitHubEmail("public@example.com", []githubEmailResponse{
		{Email: "first@example.com", Primary: false, Verified: true},
		{Email: "primary@example.com", Primary: true, Verified: true},
		{Email: "unverified@example.com", Primary: true, Verified: false},
	})
	if got != "primary@example.com" {
		t.Fatalf("email = %q, want verified primary", got)
	}
}

func requireGitHubAuth(t *testing.T, w http.ResponseWriter, r *http.Request) bool {
	t.Helper()
	if r.Header.Get("Authorization") != "Bearer github-token" {
		t.Errorf("authorization = %q, want bearer token", r.Header.Get("Authorization"))
		http.Error(w, "missing auth", http.StatusUnauthorized)
		return false
	}
	return true
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}
