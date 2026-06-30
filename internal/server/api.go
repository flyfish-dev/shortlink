package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ai-shortlink/internal/model"
	"ai-shortlink/internal/store"
	"ai-shortlink/internal/util"
)

type shortLinkPayload struct {
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
}

type liveQRPayload struct {
	Code             string `json:"code"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Status           string `json:"status"`
	RotationStrategy string `json:"rotation_strategy"`
	GuideTitle       string `json:"guide_title"`
	GuideText        string `json:"guide_text"`
	FallbackURL      string `json:"fallback_url"`
}

type liveQRItemPayload struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	QRImageURL string `json:"qr_image_url"`
	TargetURL  string `json:"target_url"`
	Status     string `json:"status"`
	StartsAt   string `json:"starts_at"`
	ExpiresAt  string `json:"expires_at"`
	MaxViews   int64  `json:"max_views"`
	SortOrder  int    `json:"sort_order"`
	Weight     int    `json:"weight"`
}

type liveQRBundlePayload struct {
	Live          liveQRPayload       `json:"live"`
	Items         []liveQRItemPayload `json:"items"`
	DeleteItemIDs []int64             `json:"delete_item_ids"`
}

type reviewPayload struct {
	Status       string `json:"status"`
	Note         string `json:"note"`
	IncludeItems bool   `json:"include_items"`
}

type settingsPayload struct {
	AppName       string `json:"app_name"`
	AppNameZH     string `json:"app_name_zh"`
	AppNameEN     string `json:"app_name_en"`
	BaseURL       string `json:"base_url"`
	DefaultLocale string `json:"default_locale"`
	LoginMode     string `json:"login_mode"`
	SMTPEnabled   bool   `json:"smtp_enabled"`
	SMTPHost      string `json:"smtp_host"`
	SMTPPort      int    `json:"smtp_port"`
	SMTPSecurity  string `json:"smtp_security"`
	SMTPUsername  string `json:"smtp_username"`
	SMTPPassword  string `json:"smtp_password"`
	SMTPFrom      string `json:"smtp_from"`
}

type accountPayload struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (s *Server) adminAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin")
	switch {
	case path == "/me" && r.Method == http.MethodGet:
		s.apiMe(w, r)
	case path == "/account/recovery-key/rotate" && r.Method == http.MethodPost:
		s.apiRotateRecoveryKey(w, r)
	case path == "/account" && r.Method == http.MethodPut:
		s.apiUpdateAccount(w, r)
	case path == "/qr-preview" && r.Method == http.MethodGet:
		s.apiQRPreview(w, r)
	case path == "/settings" && r.Method == http.MethodGet:
		s.apiGetSettings(w, r)
	case path == "/settings" && r.Method == http.MethodPut:
		s.apiUpdateSettings(w, r)
	case path == "/overview" && r.Method == http.MethodGet:
		s.apiOverview(w, r)
	case path == "/short-links" || path == "/short-links/":
		if r.Method == http.MethodGet {
			s.apiListShortLinks(w, r)
			return
		}
		if r.Method == http.MethodPost {
			s.apiCreateShortLink(w, r)
			return
		}
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "method not allowed"))
	case strings.HasPrefix(path, "/short-links/"):
		s.apiShortLinkDetail(w, r)
	case path == "/live-qrs" || path == "/live-qrs/":
		if r.Method == http.MethodGet {
			s.apiListLiveQRs(w, r)
			return
		}
		if r.Method == http.MethodPost {
			s.apiCreateLiveQR(w, r)
			return
		}
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "method not allowed"))
	case path == "/live-qrs/bundle" && r.Method == http.MethodPost:
		s.apiCreateLiveQRBundle(w, r)
	case strings.HasPrefix(path, "/live-qrs/"):
		s.apiLiveQRDetail(w, r)
	case strings.HasPrefix(path, "/live-qr-items/"):
		s.apiLiveQRItemDetail(w, r)
	case path == "/uploads/images" && r.Method == http.MethodPost:
		s.apiUploadImage(w, r)
	default:
		writeJSON(w, http.StatusNotFound, apiErr("not_found", "接口不存在"))
	}
}

func (s *Server) apiQRPreview(w http.ResponseWriter, r *http.Request) {
	content := strings.TrimSpace(r.URL.Query().Get("content"))
	if content == "" {
		content = s.publicBaseURL(r) + "/q/preview"
	}
	logoURL, err := normalizeImageURL(r.URL.Query().Get("logo_url"), "二维码中心贴图")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeStyledQRCode(w, content, "svg", r.URL.Query().Get("style"), r.URL.Query().Get("foreground"), r.URL.Query().Get("background"), logoURL)
}

func (s *Server) apiMe(w http.ResponseWriter, r *http.Request) {
	id := deviceIDFromContext(r.Context())
	if id == nil {
		writeJSON(w, 401, apiErr("unauthorized", "未登录"))
		return
	}
	dev, err := s.store().GetAdminDevice(r.Context(), *id)
	if err != nil {
		writeJSON(w, 401, apiErr("unauthorized", "设备无效"))
		return
	}
	acct, freshKey, err := s.ensureDeviceAccount(r.Context(), dev)
	if err != nil {
		writeJSON(w, 500, apiErr("account", err.Error()))
		return
	}
	account := accountResponse(s, acct)
	if freshKey != "" {
		account["recovery_key"] = freshKey
		account["recovery_key_available"] = true
	}
	st := s.settings(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"device":   dev,
		"account":  account,
		"settings": st,
		"app_name": st.AppName,
		"base_url": s.publicBaseURL(r),
	})
}

func (s *Server) apiRotateRecoveryKey(w http.ResponseWriter, r *http.Request) {
	id := deviceIDFromContext(r.Context())
	if id == nil {
		writeJSON(w, 401, apiErr("unauthorized", "未登录"))
		return
	}
	dev, err := s.store().GetAdminDevice(r.Context(), *id)
	if err != nil {
		writeJSON(w, 401, apiErr("unauthorized", "设备无效"))
		return
	}
	acct, _, err := s.ensureDeviceAccount(r.Context(), dev)
	if err != nil {
		writeJSON(w, 500, apiErr("account", err.Error()))
		return
	}
	acct, recoveryKey, err := s.rotateAccountRecoveryKey(r.Context(), acct.ID)
	if err != nil {
		writeJSON(w, 500, apiErr("account", err.Error()))
		return
	}
	_ = s.store().Audit(r.Context(), id, "admin_account.rotate_recovery_key", "admin_account", &acct.ID, "rotate recovery key", util.ClientIP(r, s.cfg.TrustProxy))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "account": map[string]any{"id": acct.ID, "recovery_key": recoveryKey, "recovery_key_available": true}})
}

func (s *Server) apiOverview(w http.ResponseWriter, r *http.Request) {
	o, err := s.store().Overview(r.Context())
	if err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": o})
}

func (s *Server) apiListShortLinks(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	items, err := s.store().ListShortLinks(r.Context(), r.URL.Query().Get("q"), limit, offset)
	if err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	base := s.publicBaseURL(r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items, "links": publicLinksForShorts(base, items)})
}

func (s *Server) apiCreateShortLink(w http.ResponseWriter, r *http.Request) {
	var p shortLinkPayload
	if !decodeBody(w, r, &p) {
		return
	}
	in, err := s.shortPayloadToModel(p, true)
	if err != nil {
		writeJSON(w, 400, apiErr("bad_request", err.Error()))
		return
	}
	created, err := s.store().CreateShortLink(r.Context(), in)
	if err != nil {
		writeJSON(w, 500, apiErr("db", friendlyDBErr(err)))
		return
	}
	_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "short_link.create", "short_link", &created.ID, created.Code, util.ClientIP(r, s.cfg.TrustProxy))
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": created, "public_url": publicShortURL(s.publicBaseURL(r), created.Code)})
}

func (s *Server) apiShortLinkDetail(w http.ResponseWriter, r *http.Request) {
	id, tail, err := pathID(r.URL.Path, "/api/admin/short-links/")
	if err != nil {
		writeJSON(w, 400, apiErr("bad_id", "ID 不正确"))
		return
	}
	switch {
	case tail == "" && r.Method == http.MethodGet:
		item, err := s.store().GetShortLinkByID(r.Context(), id)
		if handleStoreErr(w, err) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": item, "public_url": publicShortURL(s.publicBaseURL(r), item.Code)})
	case tail == "" && r.Method == http.MethodPut:
		var p shortLinkPayload
		if !decodeBody(w, r, &p) {
			return
		}
		in, err := s.shortPayloadToModel(p, false)
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
		err := s.store().DeleteShortLink(r.Context(), id)
		if handleStoreErr(w, err) {
			return
		}
		_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "short_link.delete", "short_link", &id, "", util.ClientIP(r, s.cfg.TrustProxy))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case tail == "review" && r.Method == http.MethodPost:
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
		s.notifyShortLinkApproved(before, updated, publicShortURL(s.publicBaseURL(r), updated.Code))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated})
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

func (s *Server) shortPayloadToModel(p shortLinkPayload, allowGenerate bool) (*model.ShortLink, error) {
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
	return &model.ShortLink{Code: code, Title: strings.TrimSpace(p.Title), TargetURL: target, Status: status, RedirectType: p.RedirectType, StartsAt: starts, ExpiresAt: expires, MaxVisits: p.MaxVisits, FallbackURL: p.FallbackURL, Remark: p.Remark}, nil
}

func (s *Server) apiListLiveQRs(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	items, err := s.store().ListLiveQRs(r.Context(), r.URL.Query().Get("q"), limit, offset)
	if err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	base := s.publicBaseURL(r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items, "links": publicLinksForLives(base, items)})
}

func (s *Server) apiCreateLiveQR(w http.ResponseWriter, r *http.Request) {
	var p liveQRPayload
	if !decodeBody(w, r, &p) {
		return
	}
	in, err := s.livePayloadToModel(p, true)
	if err != nil {
		writeJSON(w, 400, apiErr("bad_request", err.Error()))
		return
	}
	created, err := s.store().CreateLiveQR(r.Context(), in)
	if err != nil {
		writeJSON(w, 500, apiErr("db", friendlyDBErr(err)))
		return
	}
	_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "live_qr.create", "live_qr", &created.ID, created.Code, util.ClientIP(r, s.cfg.TrustProxy))
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": created, "public_url": publicLiveURL(s.publicBaseURL(r), created.Code)})
}

func (s *Server) apiCreateLiveQRBundle(w http.ResponseWriter, r *http.Request) {
	var p liveQRBundlePayload
	if !decodeBody(w, r, &p) {
		return
	}
	in, items, err := s.bundlePayloadToModels(p, true)
	if err != nil {
		writeJSON(w, 400, apiErr("bad_request", err.Error()))
		return
	}
	created, err := s.store().SaveLiveQRBundle(r.Context(), 0, in, items, nil)
	if err != nil {
		writeJSON(w, 500, apiErr("db", friendlyDBErr(err)))
		return
	}
	_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "live_qr.bundle_create", "live_qr", &created.ID, created.Code, util.ClientIP(r, s.cfg.TrustProxy))
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": created, "public_url": publicLiveURL(s.publicBaseURL(r), created.Code)})
}

func (s *Server) apiLiveQRDetail(w http.ResponseWriter, r *http.Request) {
	id, tail, err := pathID(r.URL.Path, "/api/admin/live-qrs/")
	if err != nil {
		writeJSON(w, 400, apiErr("bad_id", "ID 不正确"))
		return
	}
	switch {
	case tail == "" && r.Method == http.MethodGet:
		item, err := s.store().GetLiveQRByID(r.Context(), id)
		if handleStoreErr(w, err) {
			return
		}
		children, _ := s.store().ListLiveQRItems(r.Context(), id)
		item.Items = children
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": item, "public_url": publicLiveURL(s.publicBaseURL(r), item.Code)})
	case tail == "" && r.Method == http.MethodPut:
		var p liveQRPayload
		if !decodeBody(w, r, &p) {
			return
		}
		in, err := s.livePayloadToModel(p, false)
		if err != nil {
			writeJSON(w, 400, apiErr("bad_request", err.Error()))
			return
		}
		updated, err := s.store().UpdateLiveQR(r.Context(), id, in)
		if handleStoreErr(w, err) {
			return
		}
		_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "live_qr.update", "live_qr", &id, updated.Code, util.ClientIP(r, s.cfg.TrustProxy))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated, "public_url": publicLiveURL(s.publicBaseURL(r), updated.Code)})
	case tail == "bundle" && r.Method == http.MethodPut:
		var p liveQRBundlePayload
		if !decodeBody(w, r, &p) {
			return
		}
		in, items, err := s.bundlePayloadToModels(p, false)
		if err != nil {
			writeJSON(w, 400, apiErr("bad_request", err.Error()))
			return
		}
		updated, err := s.store().SaveLiveQRBundle(r.Context(), id, in, items, p.DeleteItemIDs)
		if handleStoreErr(w, err) {
			return
		}
		_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "live_qr.bundle_update", "live_qr", &id, updated.Code, util.ClientIP(r, s.cfg.TrustProxy))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated, "public_url": publicLiveURL(s.publicBaseURL(r), updated.Code)})
	case tail == "" && r.Method == http.MethodDelete:
		err := s.store().DeleteLiveQR(r.Context(), id)
		if handleStoreErr(w, err) {
			return
		}
		_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "live_qr.delete", "live_qr", &id, "", util.ClientIP(r, s.cfg.TrustProxy))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case tail == "review" && r.Method == http.MethodPost:
		var p reviewPayload
		if !decodeBody(w, r, &p) {
			return
		}
		before, err := s.store().GetLiveQRByID(r.Context(), id)
		if handleStoreErr(w, err) {
			return
		}
		updated, err := s.store().ReviewLiveQR(r.Context(), id, p.Status, p.Note, p.IncludeItems)
		if handleStoreErr(w, err) {
			return
		}
		_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "live_qr.review", "live_qr", &id, p.Status, util.ClientIP(r, s.cfg.TrustProxy))
		s.notifyLiveQRApproved(before, updated, publicLiveURL(s.publicBaseURL(r), updated.Code))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated})
	case tail == "items" && r.Method == http.MethodGet:
		items, err := s.store().ListLiveQRItems(r.Context(), id)
		if err != nil {
			writeJSON(w, 500, apiErr("db", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items})
	case tail == "items" && r.Method == http.MethodPost:
		var p liveQRItemPayload
		if !decodeBody(w, r, &p) {
			return
		}
		in, err := s.itemPayloadToModel(p)
		if err != nil {
			writeJSON(w, 400, apiErr("bad_request", err.Error()))
			return
		}
		in.LiveQRID = id
		created, err := s.store().CreateLiveQRItem(r.Context(), in)
		if err != nil {
			writeJSON(w, 500, apiErr("db", friendlyDBErr(err)))
			return
		}
		_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "live_qr_item.create", "live_qr", &id, created.Title, util.ClientIP(r, s.cfg.TrustProxy))
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": created})
	case tail == "stats" && r.Method == http.MethodGet:
		days, _ := strconv.Atoi(r.URL.Query().Get("days"))
		st, err := s.store().Stats(r.Context(), "live_qr", id, days)
		if err != nil {
			writeJSON(w, 500, apiErr("db", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": st})
	default:
		writeJSON(w, http.StatusNotFound, apiErr("not_found", "接口不存在"))
	}
}

func (s *Server) livePayloadToModel(p liveQRPayload, allowGenerate bool) (*model.LiveQR, error) {
	code := strings.TrimSpace(p.Code)
	var err error
	if code == "" && allowGenerate {
		code, err = s.generateUniqueCode("live")
		if err != nil {
			return nil, err
		}
	}
	if err := util.ValidateCode(code); err != nil {
		return nil, err
	}
	status := p.Status
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "disabled" {
		return nil, fmt.Errorf("状态只支持 active/disabled")
	}
	strategy := p.RotationStrategy
	if strategy == "" {
		strategy = "round_robin"
	}
	if strategy != "round_robin" && strategy != "random" && strategy != "least_used" {
		return nil, fmt.Errorf("轮替规则只支持 round_robin/random/least_used")
	}
	if p.FallbackURL != "" {
		p.FallbackURL = util.CleanURL(p.FallbackURL)
		if err := validateHTTPURL(p.FallbackURL); err != nil {
			return nil, fmt.Errorf("备用链接无效：%w", err)
		}
	}
	return &model.LiveQR{Code: code, Title: strings.TrimSpace(p.Title), Description: p.Description, Status: status, RotationStrategy: strategy, GuideTitle: p.GuideTitle, GuideText: p.GuideText, FallbackURL: p.FallbackURL}, nil
}

func (s *Server) bundlePayloadToModels(p liveQRBundlePayload, allowGenerate bool) (*model.LiveQR, []model.LiveQRItem, error) {
	live, err := s.livePayloadToModel(p.Live, allowGenerate)
	if err != nil {
		return nil, nil, err
	}
	items := make([]model.LiveQRItem, 0, len(p.Items))
	seen := map[int64]bool{}
	for _, raw := range p.Items {
		item, err := s.itemPayloadToModel(raw)
		if err != nil {
			name := strings.TrimSpace(raw.Title)
			if name == "" {
				name = "未命名"
			}
			return nil, nil, fmt.Errorf("二维码「%s」配置错误：%w", name, err)
		}
		item.ID = raw.ID
		if item.ID > 0 {
			if seen[item.ID] {
				return nil, nil, fmt.Errorf("二维码 ID %d 重复", item.ID)
			}
			seen[item.ID] = true
		}
		items = append(items, *item)
	}
	return live, items, nil
}

func (s *Server) apiLiveQRItemDetail(w http.ResponseWriter, r *http.Request) {
	id, tail, err := pathID(r.URL.Path, "/api/admin/live-qr-items/")
	if err != nil || tail != "" {
		writeJSON(w, 400, apiErr("bad_id", "ID 不正确"))
		return
	}
	switch r.Method {
	case http.MethodPut:
		var p liveQRItemPayload
		if !decodeBody(w, r, &p) {
			return
		}
		in, err := s.itemPayloadToModel(p)
		if err != nil {
			writeJSON(w, 400, apiErr("bad_request", err.Error()))
			return
		}
		updated, err := s.store().UpdateLiveQRItem(r.Context(), id, in)
		if handleStoreErr(w, err) {
			return
		}
		_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "live_qr_item.update", "live_qr_item", &id, updated.Title, util.ClientIP(r, s.cfg.TrustProxy))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated})
	case http.MethodPost:
		var p reviewPayload
		if !decodeBody(w, r, &p) {
			return
		}
		item, err := s.store().GetLiveQRItemByID(r.Context(), id)
		if handleStoreErr(w, err) {
			return
		}
		live, err := s.store().GetLiveQRByID(r.Context(), item.LiveQRID)
		if handleStoreErr(w, err) {
			return
		}
		before := item
		updated, err := s.store().ReviewLiveQRItem(r.Context(), id, p.Status, p.Note)
		if handleStoreErr(w, err) {
			return
		}
		_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "live_qr_item.review", "live_qr_item", &id, p.Status, util.ClientIP(r, s.cfg.TrustProxy))
		s.notifyLiveQRItemApproved(before, updated, live, publicLiveURL(s.publicBaseURL(r), live.Code))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated})
	case http.MethodDelete:
		err := s.store().DeleteLiveQRItem(r.Context(), id)
		if handleStoreErr(w, err) {
			return
		}
		_ = s.store().Audit(r.Context(), deviceIDFromContext(r.Context()), "live_qr_item.delete", "live_qr_item", &id, "", util.ClientIP(r, s.cfg.TrustProxy))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "method not allowed"))
	}
}

func (s *Server) itemPayloadToModel(p liveQRItemPayload) (*model.LiveQRItem, error) {
	img, err := normalizeImageURL(p.QRImageURL, "二维码图片")
	if err != nil {
		return nil, err
	}
	if img == "" {
		return nil, fmt.Errorf("请上传或填写二维码图片地址")
	}
	target := strings.TrimSpace(p.TargetURL)
	if target != "" {
		target = util.CleanURL(target)
		if err := validateHTTPURL(target); err != nil {
			return nil, fmt.Errorf("二维码目标链接无效：%w", err)
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
		return nil, fmt.Errorf("二维码过期时间必须晚于开始时间")
	}
	if p.MaxViews < 0 {
		return nil, fmt.Errorf("展示上限不能为负数")
	}
	if p.Weight < 1 {
		return nil, fmt.Errorf("权重不能小于 1")
	}
	status := p.Status
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "disabled" {
		return nil, fmt.Errorf("状态只支持 active/disabled")
	}
	return &model.LiveQRItem{Title: strings.TrimSpace(p.Title), QRImageURL: img, TargetURL: target, Status: status, StartsAt: starts, ExpiresAt: expires, MaxViews: p.MaxViews, SortOrder: p.SortOrder, Weight: p.Weight}, nil
}

func normalizeImageURL(raw, label string) (string, error) {
	img := strings.TrimSpace(raw)
	if img == "" {
		return "", nil
	}
	if strings.HasPrefix(img, "http://") || strings.HasPrefix(img, "https://") {
		if err := validateHTTPURL(img); err != nil {
			return "", fmt.Errorf("%s地址无效：%w", label, err)
		}
		return img, nil
	}
	if strings.HasPrefix(img, "/uploads/") {
		return img, nil
	}
	return "", fmt.Errorf("%s地址只允许 /uploads/... 或 http(s) 图片地址", label)
}

func (s *Server) apiUploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.UploadMaxBytes+1024*1024)
	if err := r.ParseMultipartForm(s.cfg.UploadMaxBytes); err != nil {
		writeJSON(w, 400, apiErr("upload", "图片过大或表单错误"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, 400, apiErr("upload", "缺少 file 字段"))
		return
	}
	defer file.Close()
	if header.Size > s.cfg.UploadMaxBytes {
		writeJSON(w, 400, apiErr("upload", "图片超过大小限制"))
		return
	}

	sniff := make([]byte, 512)
	n, _ := file.Read(sniff)
	_, _ = file.Seek(0, io.SeekStart)
	ct := http.DetectContentType(sniff[:n])
	allowed := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp"}
	ext, ok := allowed[ct]
	if !ok {
		// Some browsers may send webp as application/octet-stream; fall back to extension only after MIME parse.
		if guessed := strings.ToLower(filepath.Ext(header.Filename)); guessed != "" {
			if mt := mime.TypeByExtension(guessed); strings.HasPrefix(mt, "image/") {
				ext = guessed
				ok = true
			}
		}
	}
	if !ok {
		writeJSON(w, 400, apiErr("upload", "仅支持 png/jpg/gif/webp 图片"))
		return
	}
	token, err := util.RandomCode(18)
	if err != nil {
		writeJSON(w, 500, apiErr("random", err.Error()))
		return
	}
	subdir := time.Now().Format("200601")
	dir := filepath.Join(s.cfg.DataDir, "uploads", subdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeJSON(w, 500, apiErr("fs", err.Error()))
		return
	}
	name := token + ext
	dst, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		writeJSON(w, 500, apiErr("fs", err.Error()))
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		writeJSON(w, 500, apiErr("fs", err.Error()))
		return
	}
	publicPath := "/uploads/" + subdir + "/" + name
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "url": publicPath})
}

func (s *Server) generateUniqueCode(kind string) (string, error) {
	for i := 0; i < 8; i++ {
		c, err := util.RandomCode(6 + i/3)
		if err != nil {
			return "", err
		}
		if util.ValidateCode(c) != nil {
			continue
		}
		var errGet error
		if kind == "live" {
			_, errGet = s.store().GetLiveQRByCode(contextBG(), c)
		} else {
			_, errGet = s.store().GetShortLinkByCode(contextBG(), c)
		}
		if errors.Is(errGet, store.ErrNotFound) {
			return c, nil
		}
		if errGet != nil {
			return "", errGet
		}
	}
	return "", fmt.Errorf("无法生成唯一短码，请手动填写")
}

func contextBG() context.Context { return context.Background() }

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSON(w, 400, apiErr("json", "JSON 格式错误: "+err.Error()))
		return false
	}
	return true
}

func pagination(r *http.Request) (limit, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if page <= 0 {
		page = 1
	}
	return limit, (page - 1) * limit
}

func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("只支持 http/https")
	}
	if u.Host == "" {
		return fmt.Errorf("缺少域名")
	}
	return nil
}

func handleStoreErr(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, apiErr("not_found", "数据不存在"))
		return true
	}
	writeJSON(w, http.StatusInternalServerError, apiErr("db", friendlyDBErr(err)))
	return true
}

func friendlyDBErr(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "Duplicate entry") {
		return "短码已存在，请换一个自定义短码"
	}
	return msg
}

func publicShortURL(base, code string) string { return strings.TrimRight(base, "/") + "/s/" + code }
func publicLiveURL(base, code string) string  { return strings.TrimRight(base, "/") + "/q/" + code }

func publicLinksForShorts(base string, items []model.ShortLink) map[int64]string {
	out := map[int64]string{}
	for _, it := range items {
		out[it.ID] = publicShortURL(base, it.Code)
	}
	return out
}

func publicLinksForLives(base string, items []model.LiveQR) map[int64]string {
	out := map[int64]string{}
	for _, it := range items {
		out[it.ID] = publicLiveURL(base, it.Code)
	}
	return out
}

func (s *Server) apiGetSettings(w http.ResponseWriter, r *http.Request) {
	st := s.settings(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": st, "database_mode": s.cfg.DatabaseMode})
}

func (s *Server) apiUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var p settingsPayload
	if !decodeBody(w, r, &p) {
		return
	}
	st := model.SystemSettings{
		Installed:     true,
		AppName:       strings.TrimSpace(p.AppName),
		AppNameZH:     strings.TrimSpace(p.AppNameZH),
		AppNameEN:     strings.TrimSpace(p.AppNameEN),
		BaseURL:       strings.TrimRight(strings.TrimSpace(p.BaseURL), "/"),
		DefaultLocale: strings.TrimSpace(p.DefaultLocale),
		LoginMode:     strings.TrimSpace(p.LoginMode),
		SMTPEnabled:   p.SMTPEnabled,
		SMTPHost:      strings.TrimSpace(p.SMTPHost),
		SMTPPort:      p.SMTPPort,
		SMTPSecurity:  strings.TrimSpace(p.SMTPSecurity),
		SMTPUsername:  strings.TrimSpace(p.SMTPUsername),
		SMTPFrom:      strings.TrimSpace(p.SMTPFrom),
	}
	if st.AppName == "" {
		st.AppName = st.AppNameZH
	}
	if st.AppNameZH == "" {
		st.AppNameZH = firstNonEmpty(st.AppName, "AI短链平台")
	}
	if st.AppNameEN == "" {
		st.AppNameEN = firstNonEmpty(st.AppName, "AI Shortlink")
	}
	st.AppName = st.AppNameZH
	if st.DefaultLocale == "" {
		st.DefaultLocale = "zh-CN"
	}
	if st.LoginMode == "" {
		st.LoginMode = "hybrid"
	}
	if st.LoginMode != "hybrid" && st.LoginMode != "magic" && st.LoginMode != "one_click" {
		writeJSON(w, 400, apiErr("bad_request", "登录模式无效"))
		return
	}
	current, _ := s.store().GetSettings(r.Context())
	smtpPasswordConfigured := strings.TrimSpace(p.SMTPPassword) != "" || (current != nil && strings.TrimSpace(current["smtp_password_cipher"]) != "")
	if st.BaseURL != "" {
		if err := validateHTTPURL(st.BaseURL); err != nil {
			writeJSON(w, 400, apiErr("bad_request", "站点域名无效："+err.Error()))
			return
		}
	}
	if st.SMTPEnabled {
		if st.SMTPHost == "" || st.SMTPFrom == "" || st.SMTPPort <= 0 {
			writeJSON(w, 400, apiErr("bad_request", "启用 SMTP 时必须填写主机、端口和发信邮箱"))
			return
		}
		if !validEmail(st.SMTPFrom) {
			writeJSON(w, 400, apiErr("bad_request", "发信邮箱格式不正确"))
			return
		}
	}
	if st.LoginMode == "magic" && (!st.SMTPEnabled || st.SMTPHost == "" || st.SMTPFrom == "" || st.SMTPPort <= 0 || !smtpPasswordConfigured) {
		writeJSON(w, 400, apiErr("bad_request", "仅 Magic Link 登录需要先完整配置 SMTP 主机、端口、发信邮箱和密码"))
		return
	}
	vals := settingsToMap(st)
	if strings.TrimSpace(p.SMTPPassword) != "" {
		cipher, err := s.auth.Encrypt(p.SMTPPassword)
		if err != nil {
			writeJSON(w, 500, apiErr("crypto", err.Error()))
			return
		}
		vals["smtp_password_cipher"] = cipher
	} else if current != nil && strings.TrimSpace(current["smtp_password_cipher"]) != "" {
		vals["smtp_password_cipher"] = current["smtp_password_cipher"]
	} else {
		vals["smtp_password_cipher"] = ""
	}
	if err := s.store().SetSettings(r.Context(), vals); err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": s.settings(r.Context())})
}

func (s *Server) apiUpdateAccount(w http.ResponseWriter, r *http.Request) {
	var p accountPayload
	if !decodeBody(w, r, &p) {
		return
	}
	p.Email = strings.TrimSpace(p.Email)
	p.Name = strings.TrimSpace(p.Name)
	if p.Email != "" && !validEmail(p.Email) {
		writeJSON(w, 400, apiErr("bad_request", "邮箱格式不正确"))
		return
	}
	id := deviceIDFromContext(r.Context())
	if id == nil {
		writeJSON(w, 401, apiErr("unauthorized", "未登录"))
		return
	}
	dev, err := s.store().GetAdminDevice(r.Context(), *id)
	if err != nil {
		writeJSON(w, 401, apiErr("unauthorized", "设备无效"))
		return
	}
	acct, _, err := s.ensureDeviceAccount(r.Context(), dev)
	if err != nil {
		writeJSON(w, 500, apiErr("account", err.Error()))
		return
	}
	acct, err = s.store().UpdateAdminAccountEmailName(r.Context(), acct.ID, p.Email, p.Name)
	if err != nil {
		writeJSON(w, 500, apiErr("db", friendlyDBErr(err)))
		return
	}
	settings := map[string]string{"admin_email": p.Email}
	_ = s.store().SetSettings(r.Context(), settings)
	_ = s.store().Audit(r.Context(), id, "admin_account.update", "admin_account", &acct.ID, p.Email, util.ClientIP(r, s.cfg.TrustProxy))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "account": accountResponse(s, acct)})
}
