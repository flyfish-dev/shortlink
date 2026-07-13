package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"ai-shortlink/internal/model"
	"ai-shortlink/internal/store"
	"ai-shortlink/internal/util"
)

type extShortLinkPayload struct {
	Code         string `json:"code"`
	Title        string `json:"title"`
	TargetURL    string `json:"target_url"`
	Status       string `json:"status"`
	RedirectType int    `json:"redirect_type"`
	StartsAt     string `json:"starts_at"`
	ExpiresAt    string `json:"expires_at"`
	MaxVisits    int64  `json:"max_visits"`
	FallbackURL  string `json:"fallback_url"`
	Remark       string `json:"remark"`
	QRStyle      string `json:"qr_style"`
	QRForeground string `json:"qr_foreground"`
	QRBackground string `json:"qr_background"`
	QRLogoURL    string `json:"qr_logo_url"`
}

func (s *Server) apiExtListShortLinks(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireActor(w, r)
	if !ok {
		return
	}
	limit, offset := pagination(r)
	items, err := s.store().ListShortLinksForAccount(r.Context(), r.URL.Query().Get("q"), limit, offset, actor.Account.ID, actor.IsAdmin())
	if err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items, "links": publicLinksForShorts(s.publicBaseURL(r), items)})
}

func (s *Server) apiExtCreateShortLink(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireActor(w, r)
	if !ok {
		return
	}
	var p extShortLinkPayload
	if !decodeBody(w, r, &p) {
		return
	}
	in, err := s.extShortPayloadToModel(p, true)
	if err != nil {
		writeJSON(w, 400, apiErr("bad_request", err.Error()))
		return
	}
	in.OwnerAccountID = actor.Account.ID
	created, err := s.store().CreateShortLink(r.Context(), in)
	if err != nil {
		writeJSON(w, 500, apiErr("db", friendlyDBErr(err)))
		return
	}
	_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "short_link.create", "short_link", &created.ID, created.Code, util.ClientIP(r, s.cfg.TrustProxy))
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": created, "public_url": publicShortURL(s.publicBaseURL(r), created.Code)})
}

func (s *Server) apiExtShortLinkDetail(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireActor(w, r)
	if !ok {
		return
	}
	id, tail, err := pathID(r.URL.Path, "/api/admin/short-links/")
	if err != nil {
		writeJSON(w, 400, apiErr("bad_id", "ID 不正确"))
		return
	}
	if tail == "review" {
		if !actor.IsAdmin() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "只有管理员可以审核"))
			return
		}
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "method not allowed"))
			return
		}
		var p reviewPayload
		if !decodeBody(w, r, &p) {
			return
		}
		before, err := s.store().GetShortLinkByID(r.Context(), id)
		if handleStoreErr(w, err) {
			return
		}
		updated, err := s.store().ReviewShortLink(r.Context(), id, p.Status, p.Note)
		if handleStoreErr(w, err) {
			return
		}
		_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "short_link.review", "short_link", &id, p.Status, util.ClientIP(r, s.cfg.TrustProxy))
		s.notifyShortLinkReviewed(before, updated, publicShortURL(s.publicBaseURL(r), updated.Code))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated})
		return
	}
	current, err := s.store().GetShortLinkByID(r.Context(), id)
	if handleStoreErr(w, err) {
		return
	}
	if !canAccessShort(actor, current) {
		writeJSON(w, http.StatusForbidden, apiErr("forbidden", "无权访问该短链"))
		return
	}
	switch {
	case tail == "" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": current, "public_url": publicShortURL(s.publicBaseURL(r), current.Code)})
	case tail == "" && r.Method == http.MethodPut:
		var p extShortLinkPayload
		if !decodeBody(w, r, &p) {
			return
		}
		in, err := s.extShortPayloadToModel(p, false)
		if err != nil {
			writeJSON(w, 400, apiErr("bad_request", err.Error()))
			return
		}
		updated, err := s.store().UpdateShortLink(r.Context(), id, in)
		if handleStoreErr(w, err) {
			return
		}
		_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "short_link.update", "short_link", &id, updated.Code, util.ClientIP(r, s.cfg.TrustProxy))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated, "public_url": publicShortURL(s.publicBaseURL(r), updated.Code)})
	case tail == "" && r.Method == http.MethodDelete:
		err = s.store().DeleteShortLink(r.Context(), id)
		if handleStoreErr(w, err) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case tail == "stats" && r.Method == http.MethodGet:
		days, _ := strconv.Atoi(r.URL.Query().Get("days"))
		st, err := s.store().Stats(r.Context(), "short_link", id, days)
		if err != nil {
			writeJSON(w, 500, apiErr("db", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": st})
	default:
		writeJSON(w, http.StatusNotFound, apiErr("not_found", "接口不存在"))
	}
}

func (s *Server) extShortPayloadToModel(p extShortLinkPayload, allowGenerate bool) (*model.ShortLink, error) {
	code := strings.TrimSpace(p.Code)
	var err error
	if code == "" && allowGenerate {
		code, err = s.generateUniqueCode("short")
		if err != nil {
			return nil, err
		}
	}
	if err := util.ValidateCode(code); err != nil {
		return nil, err
	}
	target := util.CleanURL(p.TargetURL)
	if err := validateHTTPURL(target); err != nil {
		return nil, fmt.Errorf("目标链接无效：%w", err)
	}
	if p.FallbackURL != "" {
		p.FallbackURL = util.CleanURL(p.FallbackURL)
		if err := validateHTTPURL(p.FallbackURL); err != nil {
			return nil, fmt.Errorf("备用链接无效：%w", err)
		}
	}
	starts, err := util.ParseAPITime(p.StartsAt)
	if err != nil {
		return nil, err
	}
	expires, err := util.ParseAPITime(p.ExpiresAt)
	if err != nil {
		return nil, err
	}
	if starts != nil && expires != nil && !expires.After(*starts) {
		return nil, fmt.Errorf("过期时间必须晚于开始时间")
	}
	if p.MaxVisits < 0 {
		return nil, fmt.Errorf("访问上限不能为负数")
	}
	status := p.Status
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "disabled" {
		return nil, fmt.Errorf("状态只支持 active/disabled")
	}
	if p.RedirectType == 0 {
		p.RedirectType = 302
	}
	if p.RedirectType != 301 && p.RedirectType != 302 && p.RedirectType != 307 && p.RedirectType != 308 {
		return nil, fmt.Errorf("跳转类型只支持 301/302/307/308")
	}
	qrStyle, qrFg, qrBg := normalizeQRPayload(p.QRStyle, p.QRForeground, p.QRBackground)
	qrLogoURL, err := normalizeImageURL(p.QRLogoURL, "二维码中心贴图")
	if err != nil {
		return nil, err
	}
	return &model.ShortLink{Code: code, Title: strings.TrimSpace(p.Title), TargetURL: target, Status: status, RedirectType: p.RedirectType, StartsAt: starts, ExpiresAt: expires, MaxVisits: p.MaxVisits, FallbackURL: p.FallbackURL, Remark: p.Remark, QRStyle: qrStyle, QRForeground: qrFg, QRBackground: qrBg, QRLogoURL: qrLogoURL}, nil
}

var _ = store.ErrNotFound
