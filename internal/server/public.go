package server

import (
	"errors"
	"fmt"
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
	switch {
	case sl.ApprovalStatus != "approved":
		status = "not_approved"
		blockedMsg = "该短链尚未通过管理员审核。"
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
	if status != "ok" && status != "not_approved" && sl.FallbackURL != "" {
		target = sl.FallbackURL
		status = status + ":fallback"
	}
	s.recordVisit(r, &model.VisitLog{ResourceType: "short_link", ResourceID: sl.ID, Code: sl.Code, EventType: "redirect", Status: status, TargetURL: target})
	if !strings.HasPrefix(status, "ok") && !strings.Contains(status, "fallback") {
		s.renderPublicError(w, r, http.StatusGone, "无法访问", blockedMsg)
		return
	}
	if err := s.store().IncrementShortVisit(r.Context(), sl.ID); err != nil {
		// Do not block redirect for non-critical stats errors.
		fmt.Printf("increment visit failed: %v\n", err)
	}
	redirectType := sl.RedirectType
	if redirectType == 0 {
		redirectType = http.StatusFound
	}
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
	if live.ApprovalStatus != "approved" {
		s.recordVisit(r, &model.VisitLog{ResourceType: "live_qr", ResourceID: live.ID, Code: live.Code, EventType: "visit", Status: "not_approved"})
		s.renderPublicError(w, r, http.StatusGone, "活码未审核", "该活码尚未通过管理员审核，暂不能使用。")
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
			s.recordVisit(r, &model.VisitLog{ResourceType: "live_qr", ResourceID: live.ID, Code: live.Code, EventType: "visit", Status: "fallback", TargetURL: live.FallbackURL})
			http.Redirect(w, r, live.FallbackURL, http.StatusFound)
			return
		}
	} else {
		itemID = &item.ID
		target = item.TargetURL
	}
	s.recordVisit(r, &model.VisitLog{ResourceType: "live_qr", ResourceID: live.ID, ItemID: itemID, Code: live.Code, EventType: "visit", Status: status, TargetURL: target})
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
	opt := localqr.Options{Scale: 10, Border: 4, Shape: style, Foreground: foreground, Background: background, LogoURL: logoURL}
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
		pngBytes, err := localqr.StyledPNG(content, opt, s.loadQRLogoImage(logoURL))
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

func (s *Server) loadQRLogoImage(logoURL string) image.Image {
	logoURL = strings.TrimSpace(logoURL)
	if logoURL == "" || !strings.HasPrefix(logoURL, "/uploads/") || strings.Contains(logoURL, "..") {
		return nil
	}
	path := filepath.Join(s.cfg.DataDir, strings.TrimPrefix(logoURL, "/"))
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	return img
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
