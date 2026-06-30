package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ai-shortlink/internal/auth"
	"ai-shortlink/internal/model"
	"ai-shortlink/internal/store"
	"ai-shortlink/internal/util"
)

const githubOAuthStateCookie = "ais_github_oauth_state"

var (
	githubOAuthTokenURL = "https://github.com/login/oauth/access_token"
	githubAPIUserURL    = "https://api.github.com/user"
	githubAPIEmailsURL  = "https://api.github.com/user/emails"
	githubOAuthClient   = &http.Client{Timeout: 10 * time.Second}
)

type githubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

type githubUserResponse struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type githubEmailResponse struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (s *Server) githubOAuthConfigured() bool {
	return strings.TrimSpace(s.cfg.GitHubClientID) != "" && strings.TrimSpace(s.cfg.GitHubClientSecret) != ""
}

func (s *Server) githubOAuthStart(w http.ResponseWriter, r *http.Request) {
	if !s.isInstalled(r.Context()) {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	if !s.githubOAuthConfigured() {
		s.renderPublicError(w, r, http.StatusServiceUnavailable, "GitHub 登录不可用", "管理员尚未配置 GitHub OAuth。")
		return
	}
	state, err := auth.RandomToken(32)
	if err != nil {
		http.Error(w, "state error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     githubOAuthStateCookie,
		Value:    state,
		Path:     "/auth/github",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
		Expires:  time.Now().Add(10 * time.Minute),
	})
	v := url.Values{}
	v.Set("client_id", s.cfg.GitHubClientID)
	v.Set("redirect_uri", s.publicBaseURL(r)+"/auth/github/callback")
	v.Set("scope", "read:user user:email")
	v.Set("state", state)
	http.Redirect(w, r, "https://github.com/login/oauth/authorize?"+v.Encode(), http.StatusFound)
}

func (s *Server) githubOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if !s.isInstalled(r.Context()) {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	if !s.githubOAuthConfigured() {
		s.renderPublicError(w, r, http.StatusServiceUnavailable, "GitHub 登录不可用", "管理员尚未配置 GitHub OAuth。")
		return
	}
	stateCookie, err := r.Cookie(githubOAuthStateCookie)
	if err != nil || strings.TrimSpace(stateCookie.Value) == "" || r.URL.Query().Get("state") != stateCookie.Value {
		s.renderPublicError(w, r, http.StatusBadRequest, "GitHub 登录已失效", "登录状态校验失败，请重新发起 GitHub 登录。")
		return
	}
	clearGitHubOAuthState(w, s.cfg.CookieSecure)
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		s.renderPublicError(w, r, http.StatusBadRequest, "GitHub 登录失败", "GitHub 未返回授权码，请重新登录。")
		return
	}
	token, err := s.exchangeGitHubCode(r.Context(), code, s.publicBaseURL(r)+"/auth/github/callback")
	if err != nil {
		s.renderPublicError(w, r, http.StatusBadGateway, "GitHub 登录失败", err.Error())
		return
	}
	ghUser, emails, err := fetchGitHubIdentity(r.Context(), token)
	if err != nil {
		s.renderPublicError(w, r, http.StatusBadGateway, "GitHub 登录失败", err.Error())
		return
	}
	email := selectGitHubEmail(ghUser.Email, emails)
	if !validEmail(email) {
		s.renderPublicError(w, r, http.StatusBadRequest, "GitHub 邮箱不可用", "请在 GitHub 账户中配置并验证邮箱后再登录。")
		return
	}
	acct, err := s.store().FindAdminAccountByEmail(r.Context(), email)
	if errors.Is(err, store.ErrNotFound) {
		name := firstNonEmpty(ghUser.Name, ghUser.Login, emailName(email))
		acct, _, err = s.createUserAccount(r.Context(), email, name)
		if err == nil {
			_ = s.store().Audit(r.Context(), nil, "user_account.github_signup", "admin_account", &acct.ID, email, util.ClientIP(r, s.cfg.TrustProxy))
		}
	}
	if err != nil {
		s.renderPublicError(w, r, http.StatusInternalServerError, "GitHub 登录失败", friendlyDBErr(err))
		return
	}
	if acct.Status != "active" {
		s.renderPublicError(w, r, http.StatusUnauthorized, "账户不可用", "该邮箱对应账户不存在或已停用。")
		return
	}
	if err := s.loginAccountWithBrowser(w, r, acct, "GitHub OAuth 绑定设备", "account.github_login", "github oauth"); err != nil {
		s.renderPublicError(w, r, http.StatusInternalServerError, "GitHub 登录失败", err.Error())
		return
	}
	http.Redirect(w, r, "/admin", http.StatusFound)
}

func clearGitHubOAuthState(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: githubOAuthStateCookie, Value: "", Path: "/auth/github", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
}

func (s *Server) exchangeGitHubCode(ctx context.Context, code, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("client_id", s.cfg.GitHubClientID)
	form.Set("client_secret", s.cfg.GitHubClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := githubOAuthClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("GitHub token exchange returned %s", res.Status)
	}
	var out githubTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", fmt.Errorf("GitHub token exchange failed: %s", firstNonEmpty(out.Description, out.Error))
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		return "", fmt.Errorf("GitHub did not return an access token")
	}
	return out.AccessToken, nil
}

func fetchGitHubIdentity(ctx context.Context, token string) (githubUserResponse, []githubEmailResponse, error) {
	var user githubUserResponse
	if err := getGitHubJSON(ctx, githubAPIUserURL, token, &user); err != nil {
		return user, nil, err
	}
	var emails []githubEmailResponse
	if err := getGitHubJSON(ctx, githubAPIEmailsURL, token, &emails); err != nil {
		return user, nil, err
	}
	return user, emails, nil
}

func getGitHubJSON(ctx context.Context, endpoint, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := githubOAuthClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("GitHub API returned %s", res.Status)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func selectGitHubEmail(publicEmail string, emails []githubEmailResponse) string {
	for _, item := range emails {
		if item.Primary && item.Verified && validEmail(item.Email) {
			return strings.TrimSpace(item.Email)
		}
	}
	for _, item := range emails {
		if item.Verified && validEmail(item.Email) {
			return strings.TrimSpace(item.Email)
		}
	}
	if validEmail(publicEmail) {
		return strings.TrimSpace(publicEmail)
	}
	return ""
}

func (s *Server) loginAccountWithBrowser(w http.ResponseWriter, r *http.Request, acct *model.AdminAccount, label, action, detail string) error {
	ip := util.ClientIP(r, s.cfg.TrustProxy)
	browserID, err := s.auth.EnsureBrowserID(w, r)
	if err != nil {
		return fmt.Errorf("browser error: %w", err)
	}
	browserHash := s.auth.Hash(browserID)
	ipHash := s.auth.Hash(util.NormalizeIPForLogin(ip))
	dev, err := s.store().FindAdminDeviceByBrowserHash(r.Context(), browserHash)
	if errors.Is(err, store.ErrNotFound) {
		dev, err = s.store().CreateAdminDevice(r.Context(), acct.ID, label, browserHash, ipHash, ip, util.Truncate(r.UserAgent(), 512))
	} else if err == nil {
		if dev.AccountID != acct.ID {
			if err := s.store().UpdateAdminDeviceAccount(r.Context(), dev.ID, acct.ID); err != nil {
				return fmt.Errorf("device error: %w", err)
			}
			dev.AccountID = acct.ID
		}
		_ = s.store().TouchAdminDevice(r.Context(), dev.ID, ip, util.Truncate(r.UserAgent(), 512))
	}
	if err != nil {
		return fmt.Errorf("device error: %w", err)
	}
	s.auth.SetSession(w, dev.ID, browserID)
	_ = s.store().Audit(r.Context(), &dev.ID, action, "admin_account", &acct.ID, detail, ip)
	return nil
}
