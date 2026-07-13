package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ai-shortlink/internal/auth"
	"ai-shortlink/internal/store"
	"ai-shortlink/internal/util"
)

type magicRequestPayload struct {
	Email string `json:"email"`
}

func (s *Server) magicRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "仅支持 POST"))
		return
	}
	if !s.isInstalled(r.Context()) {
		writeJSON(w, http.StatusPreconditionRequired, apiErr("setup_required", "系统尚未完成安装"))
		return
	}
	var p magicRequestPayload
	if !decodeBody(w, r, &p) {
		return
	}
	email := strings.TrimSpace(p.Email)
	if !validEmail(email) {
		writeJSON(w, 400, apiErr("bad_request", "邮箱格式不正确"))
		return
	}
	st := s.settings(r.Context())
	if !st.SMTPEnabled || st.SMTPHost == "" || st.SMTPFrom == "" || !st.SMTPPasswordSet {
		writeJSON(w, 400, apiErr("smtp_required", "管理员尚未配置 SMTP，暂不能发送 Magic Link"))
		return
	}
	acct, err := s.store().FindAdminAccountByEmail(r.Context(), email)
	if errors.Is(err, store.ErrNotFound) {
		acct, _, err = s.createUserAccount(r.Context(), email, emailName(email))
		if err != nil {
			writeJSON(w, 500, apiErr("db", friendlyDBErr(err)))
			return
		}
		_ = s.store().Audit(r.Context(), nil, "user_account.signup", "admin_account", &acct.ID, email, util.ClientIP(r, s.cfg.TrustProxy))
	}
	if err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	if acct.Status != "active" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "如果邮箱账户可用，登录链接会发送到该邮箱。"})
		return
	}
	if existing, err := s.store().FindActiveMagicLoginTokenByEmail(r.Context(), acct.Email, time.Now()); err == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "duplicate": true, "retry_after_seconds": retryAfterSeconds(existing.ExpiresAt), "message": "登录链接已发送，请检查邮箱；15 分钟内无需重复发送。"})
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	token, err := auth.RandomToken(32)
	if err != nil {
		writeJSON(w, 500, apiErr("crypto", err.Error()))
		return
	}
	expiresAt := time.Now().Add(15 * time.Minute)
	mt, err := s.store().CreateMagicLoginToken(r.Context(), acct.ID, acct.Email, s.auth.Hash("magic:"+token), expiresAt, util.ClientIP(r, s.cfg.TrustProxy))
	if err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	link := s.publicBaseURL(r) + "/auth/magic/consume?token=" + url.QueryEscape(token)
	if err := s.sendMagicLink(r.Context(), acct.Email, link, expiresAt); err != nil {
		_ = s.store().DeleteMagicLoginToken(r.Context(), mt.ID)
		writeJSON(w, 500, apiErr("smtp", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "retry_after_seconds": retryAfterSeconds(expiresAt), "message": "登录链接已发送，请在 15 分钟内打开。"})
}

func retryAfterSeconds(expiresAt time.Time) int {
	seconds := int(time.Until(expiresAt).Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}

func emailName(email string) string {
	if at := strings.Index(email, "@"); at > 0 {
		return email[:at]
	}
	return email
}

func (s *Server) magicConsume(w http.ResponseWriter, r *http.Request) {
	if !s.isInstalled(r.Context()) {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		s.renderPublicError(w, r, http.StatusMethodNotAllowed, "请求方式不支持", "请从邮件中重新打开登录链接。")
		return
	}

	// GET is intentionally validation-only. Mail providers and corporate security
	// gateways commonly prefetch links; consuming the token on GET would let a
	// scanner invalidate the Magic Link before the user can open it.
	token := ""
	if r.Method == http.MethodGet {
		token = strings.TrimSpace(r.URL.Query().Get("token"))
	} else {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		if err := r.ParseForm(); err == nil {
			token = strings.TrimSpace(r.PostForm.Get("token"))
		}
	}
	if token == "" {
		s.renderPublicError(w, r, http.StatusBadRequest, "登录链接无效", "Magic Link 缺少登录令牌。")
		return
	}
	mt, err := s.store().FindMagicLoginTokenByHash(r.Context(), s.auth.Hash("magic:"+token))
	if errors.Is(err, store.ErrNotFound) {
		s.renderPublicError(w, r, http.StatusUnauthorized, "登录链接无效", "该 Magic Link 不存在或已经失效。请重新发送登录链接。")
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if mt.UsedAt != nil {
		s.renderPublicError(w, r, http.StatusUnauthorized, "登录链接已使用", "为了安全，每个 Magic Link 只能使用一次。请重新发送登录链接。")
		return
	}
	if time.Now().After(mt.ExpiresAt) {
		s.renderPublicError(w, r, http.StatusUnauthorized, "登录链接已过期", "Magic Link 有效期为 15 分钟，请重新发送。 ")
		return
	}
	acct, err := s.store().GetAdminAccount(r.Context(), mt.AccountID)
	if err != nil || acct.Status != "active" {
		s.renderPublicError(w, r, http.StatusUnauthorized, "账户不可用", "该登录链接对应的账户不存在或已停用。")
		return
	}
	if r.Method == http.MethodGet {
		st := s.settings(r.Context())
		view := map[string]any{
			"AppName":    s.appName(r.Context()),
			"Lang":       "zh-CN",
			"Title":      "确认邮箱登录",
			"Message":    "登录链接有效。点击下方按钮后，系统才会使用这次一次性登录凭证。",
			"ButtonText": "确认登录",
			"Footnote":   "Magic Link 仅可成功使用一次，且会在邮件所示时间过期。",
			"Token":      token,
		}
		if strings.HasPrefix(strings.ToLower(st.DefaultLocale), "en") {
			view["Lang"] = "en"
			view["Title"] = "Confirm email sign-in"
			view["Message"] = "This login link is valid. The one-time credential is used only after you confirm below."
			view["ButtonText"] = "Confirm sign-in"
			view["Footnote"] = "A Magic Link can be used successfully only once and expires at the time shown in the email."
		}
		s.render(w, r, "magic_confirm.html", view)
		return
	}

	ip := util.ClientIP(r, s.cfg.TrustProxy)
	browserID, err := s.auth.EnsureBrowserID(w, r)
	if err != nil {
		http.Error(w, "browser error", http.StatusInternalServerError)
		return
	}
	browserHash := s.auth.Hash(browserID)
	ipHash := s.auth.Hash(util.NormalizeIPForLogin(ip))
	dev, err := s.store().FindAdminDeviceByBrowserHash(r.Context(), browserHash)
	if errors.Is(err, store.ErrNotFound) {
		dev, err = s.store().CreateAdminDevice(r.Context(), acct.ID, "Magic Link 绑定设备", browserHash, ipHash, ip, util.Truncate(r.UserAgent(), 512))
	} else if err == nil {
		if dev.AccountID != acct.ID {
			_ = s.store().UpdateAdminDeviceAccount(r.Context(), dev.ID, acct.ID)
		}
		_ = s.store().TouchAdminDevice(r.Context(), dev.ID, ip, util.Truncate(r.UserAgent(), 512))
	}
	if err != nil {
		http.Error(w, "device error", http.StatusInternalServerError)
		return
	}
	if err := s.store().MarkMagicLoginTokenUsed(r.Context(), mt.ID); err != nil {
		http.Error(w, "token error", http.StatusInternalServerError)
		return
	}
	s.auth.SetSession(w, dev.ID, browserID)
	_ = s.store().Audit(r.Context(), &dev.ID, "account.magic_login", "admin_account", &acct.ID, "magic link", ip)
	http.Redirect(w, r, "/admin", http.StatusFound)
}
