package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-shortlink/internal/model"
	"ai-shortlink/internal/store"
	"ai-shortlink/internal/util"
)

type tenantRequestActor struct {
	Actor  *actorInfo
	Tenant *model.Tenant
	Member *model.TenantMember
}

func (a *tenantRequestActor) role() string {
	if a == nil || a.Member == nil {
		return ""
	}
	return a.Member.Role
}

func (a *tenantRequestActor) canWrite() bool {
	switch a.role() {
	case "owner", "admin", "member":
		return true
	default:
		return false
	}
}

func (a *tenantRequestActor) canReview() bool {
	switch a.role() {
	case "owner", "admin", "reviewer":
		return true
	default:
		return false
	}
}

func (a *tenantRequestActor) canManageMembers() bool {
	return a.role() == "owner" || a.role() == "admin"
}

func (s *Server) requireTenantActor(w http.ResponseWriter, r *http.Request) (*tenantRequestActor, bool) {
	actor, err := s.currentActor(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, apiErr("unauthorized", "登录状态无效"))
		return nil, false
	}
	raw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	var tenant *model.Tenant
	var member *model.TenantMember
	if raw == "" {
		var role string
		tenant, role, err = s.store().EnsurePersonalTenant(r.Context(), actor.Account.ID)
		if err == nil {
			_, member, err = s.store().GetTenantAccess(r.Context(), tenant.ID, actor.Account.ID)
			if member != nil && member.Role == "" {
				member.Role = role
			}
		}
	} else {
		tenantID, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || tenantID <= 0 {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_tenant", "X-Tenant-ID 不正确"))
			return nil, false
		}
		tenant, member, err = s.store().GetTenantAccess(r.Context(), tenantID, actor.Account.ID)
	}
	if err != nil {
		if errors.Is(err, store.ErrTenantForbidden) || errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusForbidden, apiErr("tenant_forbidden", "无权访问该工作空间"))
		} else {
			writeJSON(w, http.StatusInternalServerError, apiErr("tenant", err.Error()))
		}
		return nil, false
	}
	if tenant.Status != "active" {
		writeJSON(w, http.StatusForbidden, apiErr("tenant_disabled", "工作空间已停用"))
		return nil, false
	}
	return &tenantRequestActor{Actor: actor, Tenant: tenant, Member: member}, true
}

func (s *Server) requirePlatformActor(w http.ResponseWriter, r *http.Request) (*actorInfo, bool) {
	actor, err := s.currentActor(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, apiErr("unauthorized", "登录状态无效"))
		return nil, false
	}
	if !actor.IsAdmin() {
		writeJSON(w, http.StatusForbidden, apiErr("platform_admin_required", "仅平台总管理员可执行此操作"))
		return nil, false
	}
	return actor, true
}

func writeSaaSError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, apiErr("not_found", "资源不存在"))
	case errors.Is(err, store.ErrTenantForbidden):
		writeJSON(w, http.StatusForbidden, apiErr("tenant_forbidden", "无权访问该工作空间资源"))
	case errors.Is(err, store.ErrSubscriptionInactive):
		writeJSON(w, http.StatusPaymentRequired, apiErr("subscription_inactive", "当前订阅不可用，请联系平台管理员"))
	case errors.Is(err, store.ErrQuotaExceeded):
		writeJSON(w, http.StatusConflict, apiErr("quota_exceeded", err.Error()))
	case errors.Is(err, store.ErrApprovalState):
		writeJSON(w, http.StatusConflict, apiErr("approval_state", err.Error()))
	default:
		writeJSON(w, http.StatusInternalServerError, apiErr("internal", err.Error()))
	}
}

func (s *Server) adminAPISaaS(w http.ResponseWriter, r *http.Request, path string) bool {
	switch {
	case strings.HasPrefix(path, "/saas"):
		s.handleSaaSManagement(w, r, path)
		return true
	case path == "/short-links" || path == "/short-links/" || strings.HasPrefix(path, "/short-links/"):
		s.handleTenantShortLinks(w, r, path)
		return true
	case path == "/live-qrs" || path == "/live-qrs/" || strings.HasPrefix(path, "/live-qrs/"):
		s.handleTenantLiveQRs(w, r, path)
		return true
	case strings.HasPrefix(path, "/live-qr-items/"):
		s.handleTenantLiveQRItem(w, r, path)
		return true
	default:
		return false
	}
}

func (s *Server) handleSaaSManagement(w http.ResponseWriter, r *http.Request, path string) {
	if strings.HasPrefix(path, "/saas/platform") {
		s.handlePlatformManagement(w, r, path)
		return
	}
	ta, ok := s.requireTenantActor(w, r)
	if !ok {
		return
	}
	switch {
	case path == "/saas/bootstrap" && r.Method == http.MethodGet:
		access, err := s.store().ListTenantAccessForAccount(r.Context(), ta.Actor.Account.ID)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		quota, err := s.store().GetTenantQuotaSnapshot(r.Context(), ta.Tenant.ID)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "tenants": access, "current_tenant": ta.Tenant, "tenant_role": ta.role(), "quota": quota, "platform_admin": ta.Actor.IsAdmin()})
	case (path == "/saas/tenants" || path == "/saas/tenants/") && r.Method == http.MethodGet:
		access, err := s.store().ListTenantAccessForAccount(r.Context(), ta.Actor.Account.ID)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": access})
	case (path == "/saas/tenants" || path == "/saas/tenants/") && r.Method == http.MethodPost:
		var p struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
		}
		if !decodeBody(w, r, &p) {
			return
		}
		p.Name = strings.TrimSpace(p.Name)
		p.Slug = safeTenantSlug(p.Slug, p.Name)
		if p.Name == "" {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_request", "请填写工作空间名称"))
			return
		}
		tenant, err := s.store().CreateTenant(r.Context(), ta.Actor.Account.ID, p.Slug, p.Name)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		_ = s.store().AuditTenant(r.Context(), deviceIDFromContext(r.Context()), tenant.ID, "tenant.create", "tenant", &tenant.ID, tenant.Slug, util.ClientIP(r, s.cfg.TrustProxy))
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": tenant})
	case strings.HasPrefix(path, "/saas/tenants/"):
		s.handleTenantSettings(w, r, ta, strings.TrimPrefix(path, "/saas/tenants/"))
	case path == "/saas/plans" && r.Method == http.MethodGet:
		plans, err := s.store().ListPlans(r.Context(), false)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": plans})
	case path == "/saas/subscription" && r.Method == http.MethodGet:
		quota, err := s.store().GetTenantQuotaSnapshot(r.Context(), ta.Tenant.ID)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": quota})
	case path == "/saas/subscription/requests" && r.Method == http.MethodGet:
		items, err := s.store().ListSubscriptionRequests(r.Context(), ta.Tenant.ID, false)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items})
	case path == "/saas/subscription/requests" && r.Method == http.MethodPost:
		if ta.role() != "owner" && ta.role() != "admin" {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "仅工作空间所有者或管理员可以申请套餐变更"))
			return
		}
		var p struct {
			PlanID int64  `json:"plan_id"`
			Note   string `json:"note"`
		}
		if !decodeBody(w, r, &p) {
			return
		}
		item, err := s.store().RequestPlanChange(r.Context(), ta.Tenant.ID, ta.Actor.Account.ID, p.PlanID, p.Note)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		_ = s.store().AuditTenant(r.Context(), deviceIDFromContext(r.Context()), ta.Tenant.ID, "subscription.request", "subscription_change", &item.ID, item.ToPlan, util.ClientIP(r, s.cfg.TrustProxy))
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": item})
	case path == "/saas/reviews" && r.Method == http.MethodGet:
		if !ta.canReview() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色没有租户审批权限"))
			return
		}
		items, err := s.store().ListApprovalQueue(r.Context(), ta.Tenant.ID, r.URL.Query().Get("stage"), false)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items})
	case strings.HasPrefix(path, "/saas/reviews/") && r.Method == http.MethodPost:
		if !ta.canReview() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色没有租户审批权限"))
			return
		}
		parts := splitPath(strings.TrimPrefix(path, "/saas/reviews/"))
		if len(parts) != 2 {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_path", "审批资源路径不正确"))
			return
		}
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_id", "资源 ID 不正确"))
			return
		}
		var p struct {
			Action       string `json:"action"`
			Status       string `json:"status"`
			Note         string `json:"note"`
			IncludeItems bool   `json:"include_items"`
		}
		if !decodeBody(w, r, &p) {
			return
		}
		action := normalizeReviewAction(p.Action, p.Status)
		if err := s.store().ReviewResourceTenant(r.Context(), ta.Tenant.ID, ta.Actor.Account.ID, parts[0], id, action, p.Note, p.IncludeItems); err != nil {
			writeSaaSError(w, err)
			return
		}
		_ = s.store().AuditTenant(r.Context(), deviceIDFromContext(r.Context()), ta.Tenant.ID, "approval.tenant."+action, parts[0], &id, p.Note, util.ClientIP(r, s.cfg.TrustProxy))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case path == "/saas/approval-events" && r.Method == http.MethodGet:
		resourceType := strings.TrimSpace(r.URL.Query().Get("resource_type"))
		resourceID, _ := strconv.ParseInt(r.URL.Query().Get("resource_id"), 10, 64)
		items, err := s.store().ApprovalEventsByResource(r.Context(), ta.Tenant.ID, resourceType, resourceID)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items})
	default:
		writeJSON(w, http.StatusNotFound, apiErr("not_found", "SaaS 接口不存在"))
	}
}

func (s *Server) handleTenantSettings(w http.ResponseWriter, r *http.Request, ta *tenantRequestActor, rest string) {
	parts := splitPath(rest)
	if len(parts) == 0 {
		writeJSON(w, http.StatusNotFound, apiErr("not_found", "工作空间接口不存在"))
		return
	}
	tenantID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || tenantID != ta.Tenant.ID {
		writeJSON(w, http.StatusForbidden, apiErr("tenant_forbidden", "只能管理当前工作空间"))
		return
	}
	if len(parts) == 1 && r.Method == http.MethodPut {
		if ta.role() != "owner" && ta.role() != "admin" {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色不能修改工作空间"))
			return
		}
		var p struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}
		if !decodeBody(w, r, &p) {
			return
		}
		if p.Status == "" {
			p.Status = ta.Tenant.Status
		}
		item, err := s.store().UpdateTenant(r.Context(), tenantID, p.Name, p.Status)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": item})
		return
	}
	if len(parts) >= 2 && parts[1] == "members" {
		if !ta.canManageMembers() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色不能管理成员"))
			return
		}
		if len(parts) == 2 && r.Method == http.MethodGet {
			items, err := s.store().ListTenantMembers(r.Context(), tenantID)
			if err != nil {
				writeSaaSError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items})
			return
		}
		var accountID int64
		var payload struct {
			AccountID int64  `json:"account_id"`
			Email     string `json:"email"`
			Role      string `json:"role"`
			Status    string `json:"status"`
		}
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			if !decodeBody(w, r, &payload) {
				return
			}
			accountID = payload.AccountID
			if len(parts) == 3 {
				accountID, _ = strconv.ParseInt(parts[2], 10, 64)
			}
			if accountID <= 0 && strings.TrimSpace(payload.Email) != "" {
				acct, err := s.store().FindAdminAccountByEmail(r.Context(), payload.Email)
				if err != nil {
					writeSaaSError(w, err)
					return
				}
				accountID = acct.ID
			}
			if accountID <= 0 {
				writeJSON(w, http.StatusBadRequest, apiErr("bad_account", "请填写已存在账户的邮箱或 ID"))
				return
			}
			if payload.Role == "owner" && ta.role() != "owner" {
				writeJSON(w, http.StatusForbidden, apiErr("owner_required", "只有所有者可以授予 owner 角色"))
				return
			}
			item, err := s.store().UpsertTenantMember(r.Context(), tenantID, accountID, payload.Role, payload.Status)
			if err != nil {
				writeSaaSError(w, err)
				return
			}
			_ = s.store().AuditTenant(r.Context(), deviceIDFromContext(r.Context()), tenantID, "tenant_member.upsert", "tenant_member", &item.ID, item.Role, util.ClientIP(r, s.cfg.TrustProxy))
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": item})
			return
		}
		if len(parts) == 3 && r.Method == http.MethodDelete {
			accountID, _ = strconv.ParseInt(parts[2], 10, 64)
			if accountID <= 0 {
				writeJSON(w, http.StatusBadRequest, apiErr("bad_account", "账户 ID 不正确"))
				return
			}
			if err := s.store().DeleteTenantMember(r.Context(), tenantID, accountID); err != nil {
				writeSaaSError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, apiErr("not_found", "工作空间接口不存在"))
}

func (s *Server) handlePlatformManagement(w http.ResponseWriter, r *http.Request, path string) {
	actor, ok := s.requirePlatformActor(w, r)
	if !ok {
		return
	}
	switch {
	case path == "/saas/platform/reviews" && r.Method == http.MethodGet:
		items, err := s.store().ListApprovalQueue(r.Context(), 0, r.URL.Query().Get("stage"), true)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items})
	case strings.HasPrefix(path, "/saas/platform/reviews/") && r.Method == http.MethodPost:
		parts := splitPath(strings.TrimPrefix(path, "/saas/platform/reviews/"))
		if len(parts) != 2 {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_path", "终审资源路径不正确"))
			return
		}
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_id", "资源 ID 不正确"))
			return
		}
		var p struct {
			Action       string `json:"action"`
			Status       string `json:"status"`
			Note         string `json:"note"`
			IncludeItems bool   `json:"include_items"`
		}
		if !decodeBody(w, r, &p) {
			return
		}
		action := normalizeReviewAction(p.Action, p.Status)
		if err := s.store().ReviewResourcePlatform(r.Context(), actor.Account.ID, parts[0], id, action, p.Note, p.IncludeItems); err != nil {
			writeSaaSError(w, err)
			return
		}
		tenantID, _ := s.store().ResourceTenantID(r.Context(), parts[0], id)
		_ = s.store().AuditTenant(r.Context(), deviceIDFromContext(r.Context()), tenantID, "approval.platform."+action, parts[0], &id, p.Note, util.ClientIP(r, s.cfg.TrustProxy))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case path == "/saas/platform/subscription-requests" && r.Method == http.MethodGet:
		items, err := s.store().ListSubscriptionRequests(r.Context(), 0, true)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items})
	case strings.HasPrefix(path, "/saas/platform/subscription-requests/") && r.Method == http.MethodPost:
		parts := splitPath(strings.TrimPrefix(path, "/saas/platform/subscription-requests/"))
		if len(parts) != 1 {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_path", "订阅申请路径不正确"))
			return
		}
		id, _ := strconv.ParseInt(parts[0], 10, 64)
		var p struct {
			Action string `json:"action"`
			Status string `json:"status"`
			Note   string `json:"note"`
		}
		if !decodeBody(w, r, &p) {
			return
		}
		action := normalizeReviewAction(p.Action, p.Status)
		item, err := s.store().ReviewSubscriptionRequest(r.Context(), id, actor.Account.ID, action, p.Note)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": item})
	case path == "/saas/platform/tenants" && r.Method == http.MethodGet:
		limit, offset := pagination(r)
		items, err := s.store().ListAllTenants(r.Context(), r.URL.Query().Get("q"), limit, offset)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items})
	case strings.HasPrefix(path, "/saas/platform/tenants/") && r.Method == http.MethodPut:
		parts := splitPath(strings.TrimPrefix(path, "/saas/platform/tenants/"))
		if len(parts) != 1 {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_path", "租户路径不正确"))
			return
		}
		id, _ := strconv.ParseInt(parts[0], 10, 64)
		var p struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}
		if !decodeBody(w, r, &p) {
			return
		}
		item, err := s.store().UpdateTenant(r.Context(), id, p.Name, p.Status)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": item})
	case path == "/saas/platform/plans" && r.Method == http.MethodGet:
		items, err := s.store().ListPlans(r.Context(), true)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items})
	case strings.HasPrefix(path, "/saas/platform/plans/") && r.Method == http.MethodPut:
		parts := splitPath(strings.TrimPrefix(path, "/saas/platform/plans/"))
		if len(parts) != 1 {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_path", "套餐路径不正确"))
			return
		}
		id, _ := strconv.ParseInt(parts[0], 10, 64)
		var p model.Plan
		if !decodeBody(w, r, &p) {
			return
		}
		item, err := s.store().UpdatePlan(r.Context(), id, p)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": item})
	default:
		writeJSON(w, http.StatusNotFound, apiErr("not_found", "平台 SaaS 接口不存在"))
	}
}

type saasShortLinkPayload struct {
	extShortLinkPayload
	RoutingStrategy string `json:"routing_strategy"`
}

type shortTargetPayload struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	TargetURL    string `json:"target_url"`
	Status       string `json:"status"`
	Weight       int    `json:"weight"`
	SortOrder    int    `json:"sort_order"`
	StartsAt     string `json:"starts_at"`
	ExpiresAt    string `json:"expires_at"`
	MaxHits      int64  `json:"max_hits"`
	HealthStatus string `json:"health_status"`
}

type shortTargetsPayload struct {
	RoutingStrategy string               `json:"routing_strategy"`
	Targets         []shortTargetPayload `json:"targets"`
}

func (s *Server) handleTenantShortLinks(w http.ResponseWriter, r *http.Request, path string) {
	ta, ok := s.requireTenantActor(w, r)
	if !ok {
		return
	}
	if path == "/short-links" || path == "/short-links/" {
		switch r.Method {
		case http.MethodGet:
			limit, offset := pagination(r)
			items, err := s.store().ListShortLinksForTenant(r.Context(), ta.Tenant.ID, r.URL.Query().Get("q"), limit, offset)
			if err != nil {
				writeSaaSError(w, err)
				return
			}
			links := map[int64]string{}
			for _, item := range items {
				links[item.ID] = publicShortURL(s.publicBaseURL(r), item.Code)
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items, "links": links})
		case http.MethodPost:
			if !ta.canWrite() {
				writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色不能创建短链"))
				return
			}
			var p saasShortLinkPayload
			if !decodeBody(w, r, &p) {
				return
			}
			in, err := s.extShortPayloadToModel(p.extShortLinkPayload, true)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, apiErr("bad_request", err.Error()))
				return
			}
			in.OwnerAccountID = ta.Actor.Account.ID
			created, err := s.store().CreateShortLinkForTenant(r.Context(), ta.Tenant.ID, in, p.RoutingStrategy)
			if err != nil {
				writeSaaSError(w, err)
				return
			}
			_ = s.store().AuditTenant(r.Context(), deviceIDFromContext(r.Context()), ta.Tenant.ID, "short_link.create", "short_link", &created.ID, created.Code, util.ClientIP(r, s.cfg.TrustProxy))
			writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": created, "public_url": publicShortURL(s.publicBaseURL(r), created.Code)})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "method not allowed"))
		}
		return
	}
	id, tail, err := pathIDFromAPI(path, "/short-links/")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("bad_id", "短链 ID 不正确"))
		return
	}
	current, err := s.store().GetShortLinkForTenant(r.Context(), id, ta.Tenant.ID, tail == "targets")
	if err != nil {
		writeSaaSError(w, err)
		return
	}
	switch {
	case tail == "" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": current, "public_url": publicShortURL(s.publicBaseURL(r), current.Code)})
	case tail == "" && r.Method == http.MethodPut:
		if !ta.canWrite() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色不能修改短链"))
			return
		}
		var p saasShortLinkPayload
		if !decodeBody(w, r, &p) {
			return
		}
		if strings.TrimSpace(p.RoutingStrategy) == "" {
			p.RoutingStrategy = current.RoutingStrategy
		}
		in, err := s.extShortPayloadToModel(p.extShortLinkPayload, false)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_request", err.Error()))
			return
		}
		updated, err := s.store().UpdateShortLinkForTenant(r.Context(), id, ta.Tenant.ID, in, p.RoutingStrategy)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		_ = s.store().AuditTenant(r.Context(), deviceIDFromContext(r.Context()), ta.Tenant.ID, "short_link.update", "short_link", &id, updated.Code, util.ClientIP(r, s.cfg.TrustProxy))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated, "public_url": publicShortURL(s.publicBaseURL(r), updated.Code)})
	case tail == "" && r.Method == http.MethodDelete:
		if !ta.canWrite() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色不能删除短链"))
			return
		}
		if err := s.store().DeleteShortLinkForTenant(r.Context(), id, ta.Tenant.ID); err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case tail == "targets" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "routing_strategy": current.RoutingStrategy, "data": current.Targets})
	case tail == "targets" && r.Method == http.MethodPut:
		if !ta.canWrite() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色不能修改目标池"))
			return
		}
		var p shortTargetsPayload
		if !decodeBody(w, r, &p) {
			return
		}
		if strings.TrimSpace(p.RoutingStrategy) == "" {
			p.RoutingStrategy = current.RoutingStrategy
		}
		targets, err := s.targetsFromPayload(p.Targets)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_request", err.Error()))
			return
		}
		items, err := s.store().SaveShortLinkTargets(r.Context(), id, ta.Tenant.ID, p.RoutingStrategy, targets)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		_ = s.store().AuditTenant(r.Context(), deviceIDFromContext(r.Context()), ta.Tenant.ID, "short_link.targets.update", "short_link", &id, fmt.Sprintf("%d targets", len(items)), util.ClientIP(r, s.cfg.TrustProxy))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "routing_strategy": p.RoutingStrategy, "data": items})
	case tail == "review" && r.Method == http.MethodPost:
		if !ta.canReview() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色没有租户审批权限"))
			return
		}
		var p reviewPayload
		if !decodeBody(w, r, &p) {
			return
		}
		action := normalizeReviewAction("", p.Status)
		if err := s.store().ReviewResourceTenant(r.Context(), ta.Tenant.ID, ta.Actor.Account.ID, "short_link", id, action, p.Note, false); err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case tail == "stats" && r.Method == http.MethodGet:
		days, _ := strconv.Atoi(r.URL.Query().Get("days"))
		stats, err := s.store().Stats(r.Context(), "short_link", id, days)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": stats})
	case tail == "approval-events" && r.Method == http.MethodGet:
		items, err := s.store().ApprovalEventsByResource(r.Context(), ta.Tenant.ID, "short_link", id)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items})
	default:
		writeJSON(w, http.StatusNotFound, apiErr("not_found", "短链接口不存在"))
	}
}

func (s *Server) targetsFromPayload(raw []shortTargetPayload) ([]model.ShortLinkTarget, error) {
	if len(raw) > 500 {
		return nil, fmt.Errorf("目标数量不能超过 500")
	}
	out := make([]model.ShortLinkTarget, 0, len(raw))
	ids := map[int64]bool{}
	for _, p := range raw {
		if p.ID > 0 {
			if ids[p.ID] {
				return nil, fmt.Errorf("目标 ID %d 重复", p.ID)
			}
			ids[p.ID] = true
		}
		url := util.CleanURL(p.TargetURL)
		if err := validateHTTPURL(url); err != nil {
			return nil, fmt.Errorf("目标链接无效：%w", err)
		}
		status := strings.ToLower(strings.TrimSpace(p.Status))
		if status == "" {
			status = "active"
		}
		if status != "active" && status != "disabled" {
			return nil, fmt.Errorf("目标状态只支持 active/disabled")
		}
		health := strings.ToLower(strings.TrimSpace(p.HealthStatus))
		if health == "" {
			health = "unknown"
		}
		if health != "unknown" && health != "healthy" && health != "unhealthy" {
			return nil, fmt.Errorf("健康状态只支持 unknown/healthy/unhealthy")
		}
		if p.Weight < 0 || p.Weight > 10000 || p.MaxHits < 0 {
			return nil, fmt.Errorf("目标权重或命中上限无效")
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
			return nil, fmt.Errorf("目标过期时间必须晚于开始时间")
		}
		out = append(out, model.ShortLinkTarget{ID: p.ID, Name: strings.TrimSpace(p.Name), TargetURL: url, Status: status, Weight: p.Weight, SortOrder: p.SortOrder, StartsAt: starts, ExpiresAt: expires, MaxHits: p.MaxHits, HealthStatus: health})
	}
	return out, nil
}

func (s *Server) handleTenantLiveQRs(w http.ResponseWriter, r *http.Request, path string) {
	ta, ok := s.requireTenantActor(w, r)
	if !ok {
		return
	}
	if path == "/live-qrs/bundle" && r.Method == http.MethodPost {
		if !ta.canWrite() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色不能创建活码"))
			return
		}
		if err := s.store().CheckTenantQuota(r.Context(), ta.Tenant.ID, "live_qrs", 1); err != nil {
			writeSaaSError(w, err)
			return
		}
		var p extLiveQRBundlePayload
		if !decodeBody(w, r, &p) {
			return
		}
		in, items, err := s.extBundlePayloadToModels(p, true)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_request", err.Error()))
			return
		}
		in.OwnerAccountID = ta.Actor.Account.ID
		created, err := s.store().SaveLiveQRBundle(r.Context(), 0, in, items, nil)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		if err := s.store().BindLiveQRToTenantAndReset(r.Context(), created.ID, ta.Tenant.ID, true); err != nil {
			_ = s.store().DeleteLiveQR(r.Context(), created.ID)
			writeSaaSError(w, err)
			return
		}
		view, _ := s.store().GetLiveQRForTenant(r.Context(), created.ID, ta.Tenant.ID)
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": view, "public_url": publicLiveURL(s.publicBaseURL(r), created.Code)})
		return
	}
	if path == "/live-qrs" || path == "/live-qrs/" {
		switch r.Method {
		case http.MethodGet:
			limit, offset := pagination(r)
			items, err := s.store().ListLiveQRsForTenant(r.Context(), ta.Tenant.ID, r.URL.Query().Get("q"), limit, offset)
			if err != nil {
				writeSaaSError(w, err)
				return
			}
			links := map[int64]string{}
			for _, item := range items {
				links[item.ID] = publicLiveURL(s.publicBaseURL(r), item.Code)
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items, "links": links})
		case http.MethodPost:
			if !ta.canWrite() {
				writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色不能创建活码"))
				return
			}
			if err := s.store().CheckTenantQuota(r.Context(), ta.Tenant.ID, "live_qrs", 1); err != nil {
				writeSaaSError(w, err)
				return
			}
			var p extLiveQRPayload
			if !decodeBody(w, r, &p) {
				return
			}
			in, err := s.extLivePayloadToModel(p, true)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, apiErr("bad_request", err.Error()))
				return
			}
			in.OwnerAccountID = ta.Actor.Account.ID
			created, err := s.store().CreateLiveQR(r.Context(), in)
			if err != nil {
				writeSaaSError(w, err)
				return
			}
			if err := s.store().BindLiveQRToTenantAndReset(r.Context(), created.ID, ta.Tenant.ID, true); err != nil {
				_ = s.store().DeleteLiveQR(r.Context(), created.ID)
				writeSaaSError(w, err)
				return
			}
			view, _ := s.store().GetLiveQRForTenant(r.Context(), created.ID, ta.Tenant.ID)
			writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": view, "public_url": publicLiveURL(s.publicBaseURL(r), created.Code)})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "method not allowed"))
		}
		return
	}
	id, tail, err := pathIDFromAPI(path, "/live-qrs/")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiErr("bad_id", "活码 ID 不正确"))
		return
	}
	current, err := s.store().GetLiveQRForTenant(r.Context(), id, ta.Tenant.ID)
	if err != nil {
		writeSaaSError(w, err)
		return
	}
	switch {
	case tail == "" && r.Method == http.MethodGet:
		children, _ := s.store().ListLiveQRItems(r.Context(), id)
		current.Items = children
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": current, "public_url": publicLiveURL(s.publicBaseURL(r), current.Code)})
	case tail == "" && r.Method == http.MethodPut:
		if !ta.canWrite() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色不能修改活码"))
			return
		}
		var p extLiveQRPayload
		if !decodeBody(w, r, &p) {
			return
		}
		in, err := s.extLivePayloadToModel(p, false)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_request", err.Error()))
			return
		}
		updated, err := s.store().UpdateLiveQR(r.Context(), id, in)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		if err := s.store().BindLiveQRToTenantAndReset(r.Context(), id, ta.Tenant.ID, false); err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated, "public_url": publicLiveURL(s.publicBaseURL(r), updated.Code)})
	case tail == "bundle" && r.Method == http.MethodPut:
		if !ta.canWrite() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色不能修改活码"))
			return
		}
		var p extLiveQRBundlePayload
		if !decodeBody(w, r, &p) {
			return
		}
		in, items, err := s.extBundlePayloadToModels(p, false)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_request", err.Error()))
			return
		}
		in.OwnerAccountID = current.OwnerAccountID
		updated, err := s.store().SaveLiveQRBundle(r.Context(), id, in, items, p.DeleteItemIDs)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		if err := s.store().BindLiveQRToTenantAndReset(r.Context(), id, ta.Tenant.ID, false); err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated, "public_url": publicLiveURL(s.publicBaseURL(r), updated.Code)})
	case tail == "" && r.Method == http.MethodDelete:
		if !ta.canWrite() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色不能删除活码"))
			return
		}
		if err := s.store().DeleteLiveQR(r.Context(), id); err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case tail == "items" && r.Method == http.MethodGet:
		items, err := s.store().ListLiveQRItems(r.Context(), id)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items})
	case tail == "items" && r.Method == http.MethodPost:
		if !ta.canWrite() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色不能新增活码项"))
			return
		}
		var p liveQRItemPayload
		if !decodeBody(w, r, &p) {
			return
		}
		in, err := s.itemPayloadToModel(p)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_request", err.Error()))
			return
		}
		in.LiveQRID = id
		created, err := s.store().CreateLiveQRItem(r.Context(), in)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		if err := s.store().BindLiveQRItemAndReset(r.Context(), created.ID, id, ta.Tenant.ID, true); err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "data": created})
	case tail == "review" && r.Method == http.MethodPost:
		if !ta.canReview() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色没有租户审批权限"))
			return
		}
		var p reviewPayload
		if !decodeBody(w, r, &p) {
			return
		}
		if err := s.store().ReviewResourceTenant(r.Context(), ta.Tenant.ID, ta.Actor.Account.ID, "live_qr", id, normalizeReviewAction("", p.Status), p.Note, p.IncludeItems); err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case tail == "stats" && r.Method == http.MethodGet:
		days, _ := strconv.Atoi(r.URL.Query().Get("days"))
		stats, err := s.store().Stats(r.Context(), "live_qr", id, days)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": stats})
	case tail == "approval-events" && r.Method == http.MethodGet:
		items, err := s.store().ApprovalEventsByResource(r.Context(), ta.Tenant.ID, "live_qr", id)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": items})
	default:
		writeJSON(w, http.StatusNotFound, apiErr("not_found", "活码接口不存在"))
	}
}

func (s *Server) handleTenantLiveQRItem(w http.ResponseWriter, r *http.Request, path string) {
	ta, ok := s.requireTenantActor(w, r)
	if !ok {
		return
	}
	id, tail, err := pathIDFromAPI(path, "/live-qr-items/")
	if err != nil || tail != "" {
		writeJSON(w, http.StatusBadRequest, apiErr("bad_id", "活码项 ID 不正确"))
		return
	}
	item, err := s.store().GetLiveQRItemByID(r.Context(), id)
	if err != nil {
		writeSaaSError(w, err)
		return
	}
	if _, err := s.store().GetLiveQRForTenant(r.Context(), item.LiveQRID, ta.Tenant.ID); err != nil {
		writeSaaSError(w, err)
		return
	}
	switch r.Method {
	case http.MethodPut:
		if !ta.canWrite() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色不能修改活码项"))
			return
		}
		var p liveQRItemPayload
		if !decodeBody(w, r, &p) {
			return
		}
		in, err := s.itemPayloadToModel(p)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiErr("bad_request", err.Error()))
			return
		}
		updated, err := s.store().UpdateLiveQRItem(r.Context(), id, in)
		if err != nil {
			writeSaaSError(w, err)
			return
		}
		if err := s.store().BindLiveQRItemAndReset(r.Context(), id, item.LiveQRID, ta.Tenant.ID, false); err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": updated})
	case http.MethodDelete:
		if !ta.canWrite() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色不能删除活码项"))
			return
		}
		if err := s.store().DeleteLiveQRItem(r.Context(), id); err != nil {
			writeSaaSError(w, err)
			return
		}
		if err := s.store().BindLiveQRToTenantAndReset(r.Context(), item.LiveQRID, ta.Tenant.ID, false); err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case http.MethodPost:
		if !ta.canReview() {
			writeJSON(w, http.StatusForbidden, apiErr("forbidden", "当前角色没有租户审批权限"))
			return
		}
		var p reviewPayload
		if !decodeBody(w, r, &p) {
			return
		}
		if err := s.store().ReviewResourceTenant(r.Context(), ta.Tenant.ID, ta.Actor.Account.ID, "live_qr_item", id, normalizeReviewAction("", p.Status), p.Note, false); err != nil {
			writeSaaSError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiErr("method", "method not allowed"))
	}
}

func safeTenantSlug(slug, name string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		slug = strings.ToLower(strings.TrimSpace(name))
	}
	var b strings.Builder
	lastDash := false
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "workspace"
	}
	if len(out) > 80 {
		out = strings.Trim(out[:80], "-")
	}
	return out + "-" + strconv.FormatInt(time.Now().UnixNano()%1000000, 10)
}

func normalizeReviewAction(action, status string) string {
	value := strings.ToLower(strings.TrimSpace(action))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(status))
	}
	if value == "approved" {
		value = "approve"
	}
	if value == "rejected" {
		value = "reject"
	}
	return value
}

func splitPath(v string) []string {
	raw := strings.Split(strings.Trim(v, "/"), "/")
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		if strings.TrimSpace(part) != "" {
			out = append(out, part)
		}
	}
	return out
}

func pathIDFromAPI(path, prefix string) (int64, string, error) {
	parts := splitPath(strings.TrimPrefix(path, prefix))
	if len(parts) == 0 {
		return 0, "", fmt.Errorf("missing id")
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, "", fmt.Errorf("bad id")
	}
	tail := ""
	if len(parts) > 1 {
		tail = strings.Join(parts[1:], "/")
	}
	return id, tail, nil
}
