package server

import (
	"net/http"
	"strings"

	"ai-shortlink/internal/auth"
	"ai-shortlink/internal/model"
	"ai-shortlink/internal/util"
)

type userPayload struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

func (s *Server) requireActor(w http.ResponseWriter, r *http.Request) (*actorInfo, bool) {
	actor, err := s.currentActor(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, apiErr("unauthorized", "账户无效或已停用"))
		return nil, false
	}
	return actor, true
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (*actorInfo, bool) {
	actor, ok := s.requireActor(w, r)
	if !ok {
		return nil, false
	}
	if !actor.IsAdmin() {
		writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前账户没有管理权限"))
		return nil, false
	}
	return actor, true
}

func canAccessShort(actor *actorInfo, item *model.ShortLink) bool {
	return actor != nil && item != nil && (actor.IsAdmin() || item.OwnerAccountID == actor.Account.ID)
}
func canAccessLive(actor *actorInfo, item *model.LiveQR) bool {
	return actor != nil && item != nil && (actor.IsAdmin() || item.OwnerAccountID == actor.Account.ID)
}

func (s *Server) apiListUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	limit, offset := pagination(r)
	items, err := s.store().ListAdminAccounts(r.Context(), r.URL.Query().Get("q"), limit, offset)
	if err != nil {
		writeJSON(w, 500, apiErr("db", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": accountsResponse(items)})
}

func (s *Server) apiCreateUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var p userPayload
	if !decodeBody(w, r, &p) {
		return
	}
	p.Email = strings.TrimSpace(p.Email)
	p.Name = strings.TrimSpace(p.Name)
	if !validEmail(p.Email) {
		writeJSON(w, 400, apiErr("bad_request", "请填写有效邮箱"))
		return
	}
	role := normalizeAPIRole(p.Role)
	status := normalizeAPIAccountStatus(p.Status)
	recoveryKey, err := auth.NewRecoveryKey()
	if err != nil {
		writeJSON(w, 500, apiErr("crypto", err.Error()))
		return
	}
	cipher, err := s.auth.Encrypt(recoveryKey)
	if err != nil {
		writeJSON(w, 500, apiErr("crypto", err.Error()))
		return
	}
	acct, err := s.store().CreateAccount(r.Context(), p.Email, p.Name, role, status, s.auth.RecoveryHash(recoveryKey), cipher)
	if err != nil {
		writeJSON(w, 500, apiErr("db", friendlyDBErr(err)))
		return
	}
	_ = s.store().Audit(r.Context(), &actor.Device.ID, "account.create", "admin_account", &acct.ID, p.Email, util.ClientIP(r, s.cfg.TrustProxy))
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": publicAccountResponse(acct), "recovery_key": recoveryKey})
}

func (s *Server) apiUserDetail(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id, tail, err := pathID(r.URL.Path, "/api/admin/users/")
	if err != nil || tail != "" {
		writeJSON(w, 400, apiErr("bad_id", "ID 不正确"))
		return
	}
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "method not allowed"))
		return
	}
	var p userPayload
	if !decodeBody(w, r, &p) {
		return
	}
	current, err := s.store().GetAdminAccount(r.Context(), id)
	if handleStoreErr(w, err) {
		return
	}
	role := normalizeAPIRole(p.Role)
	status := normalizeAPIAccountStatus(p.Status)
	if current.Role == "admin" && (role != "admin" || status != "active") {
		admins, err := s.store().CountActiveAdmins(r.Context())
		if err != nil {
			writeJSON(w, 500, apiErr("db", err.Error()))
			return
		}
		if admins <= 1 {
			writeJSON(w, 400, apiErr("last_admin", "至少需要保留一个启用的管理员"))
			return
		}
	}
	acct, err := s.store().UpdateAdminAccountRoleStatus(r.Context(), id, p.Name, role, status)
	if handleStoreErr(w, err) {
		return
	}
	_ = s.store().Audit(r.Context(), &actor.Device.ID, "account.update", "admin_account", &id, role+":"+status, util.ClientIP(r, s.cfg.TrustProxy))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": publicAccountResponse(acct)})
}

func normalizeAPIRole(v string) string {
	if strings.ToLower(strings.TrimSpace(v)) == "admin" {
		return "admin"
	}
	return "user"
}
func normalizeAPIAccountStatus(v string) string {
	if strings.ToLower(strings.TrimSpace(v)) == "disabled" {
		return "disabled"
	}
	return "active"
}
func publicAccountResponse(a *model.AdminAccount) map[string]any {
	return map[string]any{"id": a.ID, "email": a.Email, "name": a.Name, "role": a.Role, "status": a.Status, "created_at": a.CreatedAt, "updated_at": a.UpdatedAt}
}
func accountsResponse(items []model.AdminAccount) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		out = append(out, publicAccountResponse(&items[i]))
	}
	return out
}
