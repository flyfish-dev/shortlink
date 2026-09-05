package server

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-shortlink/internal/model"
	localqr "ai-shortlink/internal/qrcode"
	"ai-shortlink/internal/store"
	"ai-shortlink/internal/util"
	_ "golang.org/x/image/webp"
)

func (s *Server) shortRedirect(w http.ResponseWriter, r *http.Request) {
	code := strings.Trim(strings.TrimPrefix(r.URL.Path, "/s/"), "/")
	if code == "" || strings.Contains(code, "/") {
		http.NotFound(w, r)
		return
	}
	s.redirectCode(w, r, code)
}

func (s *Server) redirectCode(w http.ResponseWriter, r *http.Request, code string) {
	sl, err := s.store().GetShortLinkByCode(r.Context(), code)
	if errors.Is(err, store.ErrNotFound) {
		s.renderPublicError(w, r, http.StatusNotFound, "短链不存在", "请检查链接是否完整，或联系管理员确认短链是否已删除。")
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	status := "ok"
	target := sl.TargetURL
	now := time.Now()
	blockedMsg := ""
	tenantID, _ := s.store().ResourceTenantID(r.Context(), "short_link", sl.ID)
	var targetID *int64
	strategy := "single"
	switch {
	case sl.ApprovalStatus != "approved":
		status = "not_approved"
		blockedMsg = "该短链尚未完成租户初审和平台终审。"
	case sl.Status != "active":
		status = "disabled"
		blockedMsg = "该短链已停用。"
	case sl.StartsAt != nil && now.Before(*sl.StartsAt):
		status = "not_started"
		blockedMsg = "该短链尚未到开始时间。"
	case sl.ExpiresAt != nil && now.After(*sl.ExpiresAt):
		status = "expired"
		blockedMsg = "该短链已过期。"
	case sl.MaxVisits > 0 && sl.VisitCount >= sl.MaxVisits:
		status = "limit_reached"
		blockedMsg = "该短链访问次数已达到上限。"
	}

	if status == "ok" {
		clientKey := s.auth.Hash(util.ClientIP(r, s.cfg.TrustProxy))
		decision, routeErr := s.store().SelectShortTargetForVisit(r.Context(), sl.ID, clientKey)
		if routeErr == nil {
			tenantID = decision.TenantID
			targetID = decision.TargetID
			target = decision.TargetURL
			strategy = decision.Strategy
		} else {
			switch {
			case errors.Is(routeErr, store.ErrNotPublished):
				status = "not_approved"
				blockedMsg = "该短链当前内容版本尚未完成两级审批。"
			case errors.Is(routeErr, store.ErrVisitLimitReached):
				status = "limit_reached"
				blockedMsg = "该短链访问次数已达到上限。"
			case errors.Is(routeErr, store.ErrSubscriptionInactive):
				status = "subscription_inactive"
				blockedMsg = "该工作空间的订阅当前不可用。"
			case errors.Is(routeErr, store.ErrQuotaExceeded):
				status = "tenant_quota_reached"
				blockedMsg = "该工作空间本月访问额度已用完。"
			default:
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}
	}
	if status != "ok" && status != "not_approved" && sl.FallbackURL != "" {
		target = sl.FallbackURL
		status += ":fallback"
	}
	s.recordVisitSaaS(r, &model.VisitLog{ResourceType: "short_link", ResourceID: sl.ID, Code: sl.Code, EventType: "redirect", Status: status, TargetURL: target}, tenantID, targetID)
	if status != "ok" && !strings.Contains(status, ":fallback") {
		s.renderPublicError(w, r, http.StatusGone, "无法访问", blockedMsg)
		return
	}
	redirectType := sl.RedirectType
	if redirectType == 0 {
		redirectType = http.StatusFound
	}
	// A load-balanced link must not emit a permanent redirect: browsers and
	// intermediary caches would pin one selected backend and bypass routing.
	if strategy != "single" && (redirectType == http.StatusMovedPermanently || redirectType == http.StatusPermanentRedirect) {
		redirectType = http.StatusFound
	}
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("Pragma", "no-cache")
	http.Redirect(w, r, target, redirectType)
}

func (s *Server) liveQRPublic(w http.ResponseWriter, r *http.Request) {
	code := strings.Trim(strings.TrimPrefix(r.URL.Path, "/q/"), "/")
	if code == "" || strings.Contains(code, "/") {
		http.NotFound(w, r)
		return
	}
	live, err := s.store().GetLiveQRByCode(r.Context(), code)
	if errors.Is(err, store.ErrNotFound) {
		s.renderPublicError(w, r, http.StatusNotFound, "活码不存在", "请检查链接是否完整，或联系管理员确认活码是否已删除。")
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	tenantID, _ := s.store().ResourceTenantID(r.Context(), "live_qr", live.ID)
	if live.ApprovalStatus != "approved" {
		s.recordVisitSaaS(r, &model.VisitLog{ResourceType: "live_qr", ResourceID: live.ID, Code: live.Code, EventType: "visit", Status: "not_approved"}, tenantID, nil)
		s.renderPublicError(w, r, http.StatusGone, "活码未审核", "该活码尚未完成租户初审和平台终审，暂不能使用。")
		return
	}
	if err := s.store().IncrementTenantVisitForLive(r.Context(), tenantID); err != nil {
		status := "subscription_inactive"
		message := "该工作空间的订阅当前不可用。"
		if errors.Is(err, store.ErrQuotaExceeded) {
			status = "tenant_quota_reached"
			message = "该工作空间本月访问额度已用完。"
		}
		s.recordVisitSaaS(r, &model.VisitLog{ResourceType: "live_qr", ResourceID: live.ID, Code: live.Code, EventType: "visit", Status: status}, tenantID, nil)
		if live.FallbackURL != "" {
			http.Redirect(w, r, live.FallbackURL, http.StatusFound)
			return
		}
		s.renderPublicError(w, r, http.StatusGone, "无法访问", message)
		return
	}

	live, item, err := s.store().SelectLiveQRItemForVisit(r.Context(), live.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	status := "ok"
	if live.Status != "active" {
		status = "disabled"
	}
	var itemID *int64
	target := ""
	if item == nil {
		status = "no_active_item"
		if live.FallbackURL != "" {
			s.recordVisitSaaS(r, &model.VisitLog{ResourceType: "live_qr", ResourceID: live.ID, Code: live.Code, EventType: "visit", Status: "fallback", TargetURL: live.FallbackURL}, tenantID, nil)
			http.Redirect(w, r, live.FallbackURL, http.StatusFound)
			return
		}
	} else {
		itemID = &item.ID
		target = item.TargetURL
	}
	s.recordVisitSaaS(r, &model.VisitLog{ResourceType: "live_qr", ResourceID: live.ID, ItemID: itemID, Code: live.Code, EventType: "visit", Status: status, TargetURL: target}, tenantID, nil)
	if item == nil {
		s.renderPublicError(w, r, http.StatusGone, "暂无可用二维码", "当前活码下的二维码均未开始、已过期、已停用或达到展示上限。")
		return
	}
	base := s.publicBaseURL(r)
	s.render(w, r, "live.html", map[string]any{
		"AppName":  s.appName(r.Context()),
		"Live":     live,
		"Item":     item,
		"BaseURL":  base,
		"TrackURL": "/api/public/live-longpress/" + live.Code,
	})
}

func (s *Server) recordVisitSaaS(r *http.Request, v *model.VisitLog, tenantID int64, targetID *int64) {
	ip := util.ClientIP(r, s.cfg.TrustProxy)
	ua := r.UserAgent()
	device, browser, osName := util.DetectClient(ua)
	v.IP = ip
	v.IPHash = s.auth.Hash(ip)
	v.UserAgent = util.Truncate(ua, 1024)
	v.Referer = util.Truncate(r.Referer(), 1000)
	v.DeviceType = device
	v.Browser = browser
	v.OS = osName
	if v.EventType == "" {
		v.EventType = "visit"
	}
	if v.Status == "" {
		v.Status = "ok"
	}
	_ = s.store().RecordVisitSaaS(r.Context(), v, tenantID, targetID)
}

func (s *Server) recordVisit(r *http.Request, v *model.VisitLog) {
	ip := util.ClientIP(r, s.cfg.TrustProxy)
	ua := r.UserAgent()
	device, browser, osName := util.DetectClient(ua)
	v.IP = ip
	v.IPHash = s.auth.Hash(ip)
	v.UserAgent = util.Truncate(ua, 1024)
	v.Referer = util.Truncate(r.Referer(), 1000)
	v.DeviceType = device
	v.Browser = browser
	v.OS = osName
	if v.EventType == "" {
		v.EventType = "visit"
	}
	if v.Status == "" {
		v.Status = "ok"
	}
	_ = s.store().RecordVisit(r.Context(), v)
}

func (s *Server) shortQRCode(w http.ResponseWriter, r *http.Request) {
	code, format := parseQRPath(r.URL.Path, "/qr/short/")
	if code == "" {
		http.NotFound(w, r)
		return
	}
	link, err := s.store().GetShortLinkByCode(r.Context(), code)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.writeStyledQRCode(w, publicShortURL(s.publicBaseURL(r), code), format, link.QRStyle, link.QRForeground, link.QRBackground, link.QRLogoURL)
}

func (s *Server) liveQRCode(w http.ResponseWriter, r *http.Request) {
	code, format := parseQRPath(r.URL.Path, "/qr/live/")
	if code == "" {
		http.NotFound(w, r)
		return
	}
	live, err := s.store().GetLiveQRByCode(r.Context(), code)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.writeStyledQRCode(w, publicLiveURL(s.publicBaseURL(r), code), format, live.QRStyle, live.QRForeground, live.QRBackground, live.QRLogoURL)
}

func (s *Server) writeQRCode(w http.ResponseWriter, content string) {
	s.writeStyledQRCode(w, content, "svg", "classic", "#000000", "#ffffff", "")
}

func (s *Server) writeStyledQRCode(w http.ResponseWriter, content, format, style, foreground, background, logoURL string) {
	style, foreground, background = normalizeQRPayload(style, foreground, background)
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "svg"
	}
	logo := s.loadQRLogoAsset(logoURL)
	opt := localqr.Options{Scale: 10, Border: 4, Shape: style, Foreground: foreground, Background: background, LogoURL: logoURL, LogoDataURI: logo.DataURI}
	switch format {
	case "svg":
		svg, err := localqr.StyledSVG(content, opt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write([]byte(svg))
	case "png":
		pngBytes, err := localqr.StyledPNG(content, opt, logo.Image)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(pngBytes)
	default:
		http.Error(w, "unsupported QR format", http.StatusBadRequest)
	}
}

func parseQRPath(path, prefix string) (string, string) {
	raw := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	format := "svg"
	if ext := strings.ToLower(filepath.Ext(raw)); ext != "" {
		format = strings.TrimPrefix(ext, ".")
		raw = strings.TrimSuffix(raw, ext)
	}
	return strings.TrimSpace(raw), format
}

type qrLogoAsset struct {
	Image   image.Image
	DataURI string
}

func (s *Server) loadQRLogoAsset(logoURL string) qrLogoAsset {
	logoURL = strings.TrimSpace(logoURL)
	if logoURL == "" || !strings.HasPrefix(logoURL, "/uploads/") || strings.Contains(logoURL, "..") {
		return qrLogoAsset{}
	}
	path := filepath.Join(s.cfg.DataDir, strings.TrimPrefix(logoURL, "/"))
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > 2<<20 {
		return qrLogoAsset{}
	}
	asset := qrLogoAsset{}
	if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
		asset.Image = img
	}
	if contentType := http.DetectContentType(data); strings.HasPrefix(contentType, "image/") {
		asset.DataURI = "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	return asset
}

func (s *Server) renderPublicError(w http.ResponseWriter, r *http.Request, status int, title, message string) {
	w.WriteHeader(status)
	s.render(w, r, "public_error.html", map[string]any{"AppName": s.appName(r.Context()), "Title": title, "Message": message})
}

func (s *Server) publicLiveLongpress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "仅支持 POST"))
		return
	}
	code := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/public/live-longpress/"), "/")
	if code == "" || strings.Contains(code, "/") {
		writeJSON(w, http.StatusBadRequest, apiErr("bad_code", "code 不正确"))
		return
	}
	live, err := s.store().GetLiveQRByCode(r.Context(), code)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	s.recordVisit(r, &model.VisitLog{ResourceType: "live_qr", ResourceID: live.ID, Code: live.Code, EventType: "long_press", Status: "ok"})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
