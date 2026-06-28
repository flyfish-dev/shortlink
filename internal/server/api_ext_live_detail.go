package server

import (
	"net/http"
	"strconv"

	"ai-shortlink/internal/util"
)

func (s *Server) apiExtLiveQRDetail(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireActor(w, r)
	if !ok { return }
	id, tail, err := pathID(r.URL.Path, "/api/admin/live-qrs/")
	if err != nil { writeJSON(w, 400, apiErr("bad_id", "bad id")); return }
	if tail == "review" {
		if !actor.IsAdmin() { writeJSON(w, http.StatusForbidden, apiErr("forbidden", "admin only")); return }
		if r.Method != http.MethodPost { writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "method not allowed")); return }
		var p reviewPayload
		if !decodeBody(w, r, &p) { return }
		updated, err := s.store().ReviewLiveQR(r.Context(), id, p.Status, p.Note, p.IncludeItems)
		if handleStoreErr(w, err) { return }
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated})
		return
	}
	current, err := s.store().GetLiveQRByID(r.Context(), id)
	if handleStoreErr(w, err) { return }
	if !canAccessLive(actor, current) { writeJSON(w, http.StatusForbidden, apiErr("forbidden", "no permission")); return }
	switch {
	case tail == "" && r.Method == http.MethodGet:
		children, _ := s.store().ListLiveQRItems(r.Context(), id)
		current.Items = children
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": current, "public_url": publicLiveURL(s.publicBaseURL(r), current.Code)})
	case tail == "" && r.Method == http.MethodPut:
		var p extLiveQRPayload
		if !decodeBody(w, r, &p) { return }
		in, err := s.extLivePayloadToModel(p, false)
		if err != nil { writeJSON(w, 400, apiErr("bad_request", err.Error())); return }
		updated, err := s.store().UpdateLiveQR(r.Context(), id, in)
		if handleStoreErr(w, err) { return }
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated, "public_url": publicLiveURL(s.publicBaseURL(r), updated.Code)})
	case tail == "bundle" && r.Method == http.MethodPut:
		var p extLiveQRBundlePayload
		if !decodeBody(w, r, &p) { return }
		in, items, err := s.extBundlePayloadToModels(p, false)
		if err != nil { writeJSON(w, 400, apiErr("bad_request", err.Error())); return }
		in.OwnerAccountID = current.OwnerAccountID
		updated, err := s.store().SaveLiveQRBundle(r.Context(), id, in, items, p.DeleteItemIDs)
		if handleStoreErr(w, err) { return }
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated, "public_url": publicLiveURL(s.publicBaseURL(r), updated.Code)})
	case tail == "" && r.Method == http.MethodDelete:
		err = s.store().DeleteLiveQR(r.Context(), id)
		if handleStoreErr(w, err) { return }
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case tail == "items" && r.Method == http.MethodGet:
		items, err := s.store().ListLiveQRItems(r.Context(), id)
		if err != nil { writeJSON(w, 500, apiErr("db", err.Error())); return }
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items})
	case tail == "items" && r.Method == http.MethodPost:
		var p liveQRItemPayload
		if !decodeBody(w, r, &p) { return }
		in, err := s.itemPayloadToModel(p)
		if err != nil { writeJSON(w, 400, apiErr("bad_request", err.Error())); return }
		in.LiveQRID = id
		created, err := s.store().CreateLiveQRItem(r.Context(), in)
		if err != nil { writeJSON(w, 500, apiErr("db", friendlyDBErr(err))); return }
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": created})
	case tail == "stats" && r.Method == http.MethodGet:
		days, _ := strconv.Atoi(r.URL.Query().Get("days"))
		st, err := s.store().Stats(r.Context(), "live_qr", id, days)
		if err != nil { writeJSON(w, 500, apiErr("db", err.Error())); return }
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": st})
	default:
		writeJSON(w, http.StatusNotFound, apiErr("not_found", "not found"))
	}
}

func (s *Server) apiExtLiveQRItemDetail(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireActor(w, r)
	if !ok { return }
	id, tail, err := pathID(r.URL.Path, "/api/admin/live-qr-items/")
	if err != nil || tail != "" { writeJSON(w, 400, apiErr("bad_id", "bad id")); return }
	item, err := s.store().GetLiveQRItemByID(r.Context(), id)
	if handleStoreErr(w, err) { return }
	live, err := s.store().GetLiveQRByID(r.Context(), item.LiveQRID)
	if handleStoreErr(w, err) { return }
	if !canAccessLive(actor, live) { writeJSON(w, http.StatusForbidden, apiErr("forbidden", "no permission")); return }
	switch r.Method {
	case http.MethodPut:
		var p liveQRItemPayload
		if !decodeBody(w, r, &p) { return }
		in, err := s.itemPayloadToModel(p)
		if err != nil { writeJSON(w, 400, apiErr("bad_request", err.Error())); return }
		updated, err := s.store().UpdateLiveQRItem(r.Context(), id, in)
		if handleStoreErr(w, err) { return }
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated})
	case http.MethodPost:
		if !actor.IsAdmin() { writeJSON(w, http.StatusForbidden, apiErr("forbidden", "admin only")); return }
		var p reviewPayload
		if !decodeBody(w, r, &p) { return }
		updated, err := s.store().ReviewLiveQRItem(r.Context(), id, p.Status, p.Note)
		if handleStoreErr(w, err) { return }
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated})
	case http.MethodDelete:
		err := s.store().DeleteLiveQRItem(r.Context(), id)
		if handleStoreErr(w, err) { return }
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "method not allowed"))
	}
}

var _ = util.ClientIP
