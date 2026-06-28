package server

import (
	"fmt"
	"net/http"
	"strings"

	"ai-shortlink/internal/model"
	"ai-shortlink/internal/util"
)

type extLiveQRPayload struct {
	Code             string `json:"code"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Status           string `json:"status"`
	RotationStrategy string `json:"rotation_strategy"`
	GuideTitle       string `json:"guide_title"`
	GuideText        string `json:"guide_text"`
	FallbackURL      string `json:"fallback_url"`
	QRStyle          string `json:"qr_style"`
	QRForeground     string `json:"qr_foreground"`
	QRBackground     string `json:"qr_background"`
	QRLogoURL        string `json:"qr_logo_url"`
}

type extLiveQRBundlePayload struct {
	Live          extLiveQRPayload    `json:"live"`
	Items         []liveQRItemPayload `json:"items"`
	DeleteItemIDs []int64             `json:"delete_item_ids"`
}

func (s *Server) apiExtListLiveQRs(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireActor(w, r)
	if !ok {
		return
	}
	limit, offset := pagination(r)
	items, err := s.store().ListLiveQRsForAccount(r.Context(), r.URL.Query().Get("q"), limit, offset, actor.Account.ID, actor.IsAdmin())
	if err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items, "links": publicLinksForLives(s.publicBaseURL(r), items)})
}

func (s *Server) apiExtCreateLiveQR(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireActor(w, r)
	if !ok {
		return
	}
	var p extLiveQRPayload
	if !decodeBody(w, r, &p) {
		return
	}
	in, err := s.extLivePayloadToModel(p, true)
	if err != nil {
		writeJSON(w, 400, apiErr("bad_request", err.Error()))
		return
	}
	in.OwnerAccountID = actor.Account.ID
	created, err := s.store().CreateLiveQR(r.Context(), in)
	if err != nil {
		writeJSON(w, 500, apiErr("db", friendlyDBErr(err)))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": created, "public_url": publicLiveURL(s.publicBaseURL(r), created.Code)})
}

func (s *Server) apiExtCreateLiveQRBundle(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireActor(w, r)
	if !ok {
		return
	}
	var p extLiveQRBundlePayload
	if !decodeBody(w, r, &p) {
		return
	}
	in, items, err := s.extBundlePayloadToModels(p, true)
	if err != nil {
		writeJSON(w, 400, apiErr("bad_request", err.Error()))
		return
	}
	in.OwnerAccountID = actor.Account.ID
	created, err := s.store().SaveLiveQRBundle(r.Context(), 0, in, items, nil)
	if err != nil {
		writeJSON(w, 500, apiErr("db", friendlyDBErr(err)))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": created, "public_url": publicLiveURL(s.publicBaseURL(r), created.Code)})
}

func (s *Server) extLivePayloadToModel(p extLiveQRPayload, allowGenerate bool) (*model.LiveQR, error) {
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
		return nil, fmt.Errorf("status must be active or disabled")
	}
	strategy := p.RotationStrategy
	if strategy == "" {
		strategy = "round_robin"
	}
	if strategy != "round_robin" && strategy != "random" && strategy != "least_used" {
		return nil, fmt.Errorf("bad rotation strategy")
	}
	if p.FallbackURL != "" {
		p.FallbackURL = util.CleanURL(p.FallbackURL)
		if err := validateHTTPURL(p.FallbackURL); err != nil {
			return nil, err
		}
	}
	qrStyle, qrFg, qrBg := normalizeQRPayload(p.QRStyle, p.QRForeground, p.QRBackground)
	qrLogoURL, err := normalizeImageURL(p.QRLogoURL, "二维码中心贴图")
	if err != nil {
		return nil, err
	}
	return &model.LiveQR{Code: code, Title: strings.TrimSpace(p.Title), Description: p.Description, Status: status, RotationStrategy: strategy, GuideTitle: p.GuideTitle, GuideText: p.GuideText, FallbackURL: p.FallbackURL, QRStyle: qrStyle, QRForeground: qrFg, QRBackground: qrBg, QRLogoURL: qrLogoURL}, nil
}

func (s *Server) extBundlePayloadToModels(p extLiveQRBundlePayload, allowGenerate bool) (*model.LiveQR, []model.LiveQRItem, error) {
	live, err := s.extLivePayloadToModel(p.Live, allowGenerate)
	if err != nil {
		return nil, nil, err
	}
	items := make([]model.LiveQRItem, 0, len(p.Items))
	seen := map[int64]bool{}
	for _, raw := range p.Items {
		item, err := s.itemPayloadToModel(raw)
		if err != nil {
			return nil, nil, err
		}
		item.ID = raw.ID
		if item.ID > 0 {
			if seen[item.ID] {
				return nil, nil, fmt.Errorf("duplicate item id")
			}
			seen[item.ID] = true
		}
		items = append(items, *item)
	}
	return live, items, nil
}
