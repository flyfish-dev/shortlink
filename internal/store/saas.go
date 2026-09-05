package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"ai-shortlink/internal/model"
)

var (
	ErrTenantForbidden      = errors.New("tenant access forbidden")
	ErrQuotaExceeded        = errors.New("tenant quota exceeded")
	ErrSubscriptionInactive = errors.New("tenant subscription inactive")
	ErrApprovalState        = errors.New("invalid approval state")
	ErrNotPublished         = errors.New("resource version is not published")
	ErrVisitLimitReached    = errors.New("visit limit reached")
)

func (s *Store) lockSuffix() string {
	if s.mode == "mysql" {
		return " FOR UPDATE"
	}
	return ""
}

func normalizeTenantRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner":
		return "owner"
	case "admin":
		return "admin"
	case "reviewer":
		return "reviewer"
	case "analyst":
		return "analyst"
	default:
		return "member"
	}
}

func tenantCanWrite(role string) bool {
	switch normalizeTenantRole(role) {
	case "owner", "admin", "member":
		return true
	default:
		return false
	}
}

func tenantCanReview(role string) bool {
	switch normalizeTenantRole(role) {
	case "owner", "admin", "reviewer":
		return true
	default:
		return false
	}
}

func scanTenant(scanner interface{ Scan(...any) error }) (*model.Tenant, error) {
	var t model.Tenant
	var owner sql.NullInt64
	if err := scanner.Scan(&t.ID, &t.Slug, &t.Name, &t.Kind, &t.Status, &owner, &t.CreatedAt, &t.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if owner.Valid {
		t.OwnerAccountID = owner.Int64
	}
	return &t, nil
}

const tenantSelectSQL = `SELECT id,slug,name,kind,status,owner_account_id,created_at,updated_at FROM tenants`

func (s *Store) EnsurePersonalTenant(ctx context.Context, accountID int64) (*model.Tenant, string, error) {
	if accountID <= 0 {
		return nil, "", ErrTenantForbidden
	}
	row := s.db.QueryRowContext(ctx, tenantSelectSQL+` WHERE id=(SELECT tenant_id FROM tenant_members WHERE account_id=? AND status='active' ORDER BY CASE role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,id LIMIT 1)`, accountID)
	if tenant, err := scanTenant(row); err == nil {
		var role string
		if err := s.db.QueryRowContext(ctx, `SELECT role FROM tenant_members WHERE tenant_id=? AND account_id=? AND status='active'`, tenant.ID, accountID).Scan(&role); err != nil {
			return nil, "", err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE short_links SET tenant_id=? WHERE tenant_id IS NULL AND owner_account_id=?`, tenant.ID, accountID); err != nil {
			return nil, "", err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE live_qrs SET tenant_id=? WHERE tenant_id IS NULL AND owner_account_id=?`, tenant.ID, accountID); err != nil {
			return nil, "", err
		}
		return tenant, normalizeTenantRole(role), nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, "", err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = tx.Rollback() }()

	var tenantID int64
	err = tx.QueryRowContext(ctx, `SELECT t.id FROM tenants t JOIN tenant_members m ON m.tenant_id=t.id WHERE m.account_id=? AND m.status='active' ORDER BY t.id LIMIT 1`+s.lockSuffix(), accountID).Scan(&tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		var displayName, email string
		if err := tx.QueryRowContext(ctx, `SELECT name,COALESCE(email,'') FROM admin_accounts WHERE id=?`, accountID).Scan(&displayName, &email); err != nil {
			return nil, "", err
		}
		if strings.TrimSpace(displayName) == "" {
			displayName = email
		}
		if strings.TrimSpace(displayName) == "" {
			displayName = "Personal"
		}
		slug := "personal-" + strconv.FormatInt(accountID, 10)
		res, insertErr := tx.ExecContext(ctx, `INSERT INTO tenants(slug,name,kind,status,owner_account_id) VALUES(?,?,?,?,?)`, slug, displayName+" Workspace", "personal", "active", accountID)
		if insertErr != nil {
			if err := tx.QueryRowContext(ctx, `SELECT id FROM tenants WHERE slug=?`, slug).Scan(&tenantID); err != nil {
				return nil, "", insertErr
			}
		} else {
			tenantID, _ = res.LastInsertId()
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO tenant_members(tenant_id,account_id,role,status) VALUES(?,?,?,?)`, tenantID, accountID, "owner", "active"); err != nil {
			var exists int
			if qerr := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenant_members WHERE tenant_id=? AND account_id=?`, tenantID, accountID).Scan(&exists); qerr != nil || exists == 0 {
				return nil, "", err
			}
		}
	} else if err != nil {
		return nil, "", err
	}
	if err := ensureFreeSubscriptionTx(ctx, tx, tenantID); err != nil {
		return nil, "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE short_links SET tenant_id=? WHERE tenant_id IS NULL AND owner_account_id=?`, tenantID, accountID); err != nil {
		return nil, "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE live_qrs SET tenant_id=? WHERE tenant_id IS NULL AND owner_account_id=?`, tenantID, accountID); err != nil {
		return nil, "", err
	}
	if err := tx.Commit(); err != nil {
		return nil, "", err
	}
	tenant, err := s.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, "", err
	}
	return tenant, "owner", nil
}

func ensureFreeSubscriptionTx(ctx context.Context, tx *sql.Tx, tenantID int64) error {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenant_subscriptions WHERE tenant_id=?`, tenantID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	var planID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM plans WHERE code='free' AND status='active' LIMIT 1`).Scan(&planID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO tenant_subscriptions(tenant_id,plan_id,status,provider,current_period_start) VALUES(?,?,?,?,?)`, tenantID, planID, "active", "manual", now())
	return err
}

func (s *Store) GetTenant(ctx context.Context, tenantID int64) (*model.Tenant, error) {
	return scanTenant(s.db.QueryRowContext(ctx, tenantSelectSQL+` WHERE id=?`, tenantID))
}

func (s *Store) GetTenantAccess(ctx context.Context, tenantID, accountID int64) (*model.Tenant, *model.TenantMember, error) {
	tenant, err := s.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	var member model.TenantMember
	var email sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT m.id,m.tenant_id,m.account_id,COALESCE(a.email,''),a.name,m.role,m.status,m.created_at,m.updated_at FROM tenant_members m JOIN admin_accounts a ON a.id=m.account_id WHERE m.tenant_id=? AND m.account_id=? AND m.status='active' AND a.status='active'`, tenantID, accountID).Scan(&member.ID, &member.TenantID, &member.AccountID, &email, &member.Name, &member.Role, &member.Status, &member.CreatedAt, &member.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return tenant, nil, ErrTenantForbidden
		}
		return nil, nil, err
	}
	if email.Valid {
		member.Email = email.String
	}
	member.Role = normalizeTenantRole(member.Role)
	return tenant, &member, nil
}

func (s *Store) ListTenantAccessForAccount(ctx context.Context, accountID int64) ([]model.TenantAccess, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id,t.slug,t.name,t.kind,t.status,t.owner_account_id,t.created_at,t.updated_at,m.role FROM tenants t JOIN tenant_members m ON m.tenant_id=t.id WHERE m.account_id=? AND m.status='active' AND t.status='active' ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,t.id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.TenantAccess{}
	for rows.Next() {
		var t model.Tenant
		var owner sql.NullInt64
		var role string
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Kind, &t.Status, &owner, &t.CreatedAt, &t.UpdatedAt, &role); err != nil {
			return nil, err
		}
		if owner.Valid {
			t.OwnerAccountID = owner.Int64
		}
		out = append(out, model.TenantAccess{Tenant: t, Role: normalizeTenantRole(role)})
	}
	return out, rows.Err()
}

func (s *Store) CreateTenant(ctx context.Context, accountID int64, slug, name string) (*model.Tenant, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	name = strings.TrimSpace(name)
	if slug == "" || name == "" {
		return nil, fmt.Errorf("tenant slug and name are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `INSERT INTO tenants(slug,name,kind,status,owner_account_id) VALUES(?,?,?,?,?)`, slug, name, "organization", "active", accountID)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx, `INSERT INTO tenant_members(tenant_id,account_id,role,status) VALUES(?,?,?,?)`, id, accountID, "owner", "active"); err != nil {
		return nil, err
	}
	if err := ensureFreeSubscriptionTx(ctx, tx, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetTenant(ctx, id)
}

func (s *Store) UpdateTenant(ctx context.Context, tenantID int64, name, status string) (*model.Tenant, error) {
	name = strings.TrimSpace(name)
	status = strings.ToLower(strings.TrimSpace(status))
	if name == "" {
		return nil, fmt.Errorf("tenant name is required")
	}
	if status != "active" && status != "disabled" {
		return nil, fmt.Errorf("invalid tenant status")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE tenants SET name=?,status=?,updated_at=? WHERE id=?`, name, status, now(), tenantID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetTenant(ctx, tenantID)
}

func (s *Store) ListTenantMembers(ctx context.Context, tenantID int64) ([]model.TenantMember, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT m.id,m.tenant_id,m.account_id,COALESCE(a.email,''),a.name,m.role,m.status,m.created_at,m.updated_at FROM tenant_members m JOIN admin_accounts a ON a.id=m.account_id WHERE m.tenant_id=? ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 WHEN 'reviewer' THEN 2 WHEN 'member' THEN 3 ELSE 4 END,m.id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.TenantMember{}
	for rows.Next() {
		var m model.TenantMember
		if err := rows.Scan(&m.ID, &m.TenantID, &m.AccountID, &m.Email, &m.Name, &m.Role, &m.Status, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.Role = normalizeTenantRole(m.Role)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) UpsertTenantMember(ctx context.Context, tenantID, accountID int64, role, status string) (*model.TenantMember, error) {
	role = normalizeTenantRole(role)
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "disabled" {
		return nil, fmt.Errorf("invalid member status")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var existingID int64
	var existingRole string
	err = tx.QueryRowContext(ctx, `SELECT id,role FROM tenant_members WHERE tenant_id=? AND account_id=?`+s.lockSuffix(), tenantID, accountID).Scan(&existingID, &existingRole)
	isNew := errors.Is(err, sql.ErrNoRows)
	if err != nil && !isNew {
		return nil, err
	}
	if isNew && status == "active" {
		if err := checkQuotaTx(ctx, tx, s.mode, tenantID, "members", 1); err != nil {
			return nil, err
		}
	}
	if !isNew && normalizeTenantRole(existingRole) == "owner" && (role != "owner" || status != "active") {
		var owners int64
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenant_members WHERE tenant_id=? AND role='owner' AND status='active'`, tenantID).Scan(&owners); err != nil {
			return nil, err
		}
		if owners <= 1 {
			return nil, fmt.Errorf("the last active owner cannot be changed")
		}
	}
	if isNew {
		res, err := tx.ExecContext(ctx, `INSERT INTO tenant_members(tenant_id,account_id,role,status) VALUES(?,?,?,?)`, tenantID, accountID, role, status)
		if err != nil {
			return nil, err
		}
		existingID, _ = res.LastInsertId()
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE tenant_members SET role=?,status=?,updated_at=? WHERE id=?`, role, status, now(), existingID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	members, err := s.ListTenantMembers(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for i := range members {
		if members[i].AccountID == accountID {
			return &members[i], nil
		}
	}
	return nil, ErrNotFound
}

func (s *Store) DeleteTenantMember(ctx context.Context, tenantID, accountID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var role, status string
	if err := tx.QueryRowContext(ctx, `SELECT role,status FROM tenant_members WHERE tenant_id=? AND account_id=?`+s.lockSuffix(), tenantID, accountID).Scan(&role, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if normalizeTenantRole(role) == "owner" && status == "active" {
		var owners int64
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenant_members WHERE tenant_id=? AND role='owner' AND status='active'`, tenantID).Scan(&owners); err != nil {
			return err
		}
		if owners <= 1 {
			return fmt.Errorf("the last active owner cannot be removed")
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM tenant_members WHERE tenant_id=? AND account_id=?`, tenantID, accountID); err != nil {
		return err
	}
	return tx.Commit()
}

func scanPlan(scanner interface{ Scan(...any) error }) (*model.Plan, error) {
	var p model.Plan
	if err := scanner.Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.PriceMonthlyCents, &p.Currency, &p.MaxMembers, &p.MaxShortLinks, &p.MaxLiveQRs, &p.MaxTargetsPerLink, &p.MonthlyVisits, &p.FeaturesJSON, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

const planSelectSQL = `SELECT id,code,name,description,price_monthly_cents,currency,max_members,max_short_links,max_live_qrs,max_targets_per_link,monthly_visits,features_json,status,created_at,updated_at FROM plans`

func (s *Store) ListPlans(ctx context.Context, includeDisabled bool) ([]model.Plan, error) {
	query := planSelectSQL
	if !includeDisabled {
		query += ` WHERE status='active'`
	}
	query += ` ORDER BY price_monthly_cents,id`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Plan{}
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (s *Store) UpdatePlan(ctx context.Context, id int64, in model.Plan) (*model.Plan, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, fmt.Errorf("plan name is required")
	}
	for _, v := range []int64{in.PriceMonthlyCents, in.MaxMembers, in.MaxShortLinks, in.MaxLiveQRs, in.MaxTargetsPerLink, in.MonthlyVisits} {
		if v < 0 {
			return nil, fmt.Errorf("plan limits cannot be negative")
		}
	}
	if in.Status != "active" && in.Status != "disabled" {
		return nil, fmt.Errorf("invalid plan status")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE plans SET name=?,description=?,price_monthly_cents=?,currency=?,max_members=?,max_short_links=?,max_live_qrs=?,max_targets_per_link=?,monthly_visits=?,features_json=?,status=?,updated_at=? WHERE id=?`, strings.TrimSpace(in.Name), in.Description, in.PriceMonthlyCents, strings.ToUpper(strings.TrimSpace(in.Currency)), in.MaxMembers, in.MaxShortLinks, in.MaxLiveQRs, in.MaxTargetsPerLink, in.MonthlyVisits, in.FeaturesJSON, in.Status, now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return scanPlan(s.db.QueryRowContext(ctx, planSelectSQL+` WHERE id=?`, id))
}

func scanSubscription(scanner interface{ Scan(...any) error }) (*model.TenantSubscription, error) {
	var sub model.TenantSubscription
	var externalCustomer, externalSub sql.NullString
	var start, end, trial sql.NullTime
	var cancel int
	if err := scanner.Scan(&sub.ID, &sub.TenantID, &sub.PlanID, &sub.PlanCode, &sub.PlanName, &sub.Status, &sub.Provider, &externalCustomer, &externalSub, &start, &end, &cancel, &trial, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if externalCustomer.Valid {
		sub.ExternalCustomerID = externalCustomer.String
	}
	if externalSub.Valid {
		sub.ExternalSubscriptionID = externalSub.String
	}
	if start.Valid {
		sub.CurrentPeriodStart = &start.Time
	}
	if end.Valid {
		sub.CurrentPeriodEnd = &end.Time
	}
	if trial.Valid {
		sub.TrialEndsAt = &trial.Time
	}
	sub.CancelAtPeriodEnd = cancel != 0
	return &sub, nil
}

const subscriptionSelectSQL = `SELECT s.id,s.tenant_id,s.plan_id,p.code,p.name,s.status,s.provider,s.external_customer_id,s.external_subscription_id,s.current_period_start,s.current_period_end,s.cancel_at_period_end,s.trial_ends_at,s.created_at,s.updated_at FROM tenant_subscriptions s JOIN plans p ON p.id=s.plan_id`

func (s *Store) GetTenantSubscription(ctx context.Context, tenantID int64) (*model.TenantSubscription, error) {
	return scanSubscription(s.db.QueryRowContext(ctx, subscriptionSelectSQL+` WHERE s.tenant_id=?`, tenantID))
}

func subscriptionUsable(status string, periodEnd *time.Time, trialEnd *time.Time, n time.Time) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "active" && status != "trialing" && status != "trial" {
		return false
	}
	if status == "trialing" || status == "trial" {
		if trialEnd != nil && !trialEnd.After(n) {
			return false
		}
	}
	if periodEnd != nil && !periodEnd.After(n) {
		return false
	}
	return true
}

func monthKey(t time.Time) string { return t.UTC().Format("2006-01") }

func (s *Store) GetTenantQuotaSnapshot(ctx context.Context, tenantID int64) (*model.TenantQuotaSnapshot, error) {
	sub, err := s.GetTenantSubscription(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	plan, err := scanPlan(s.db.QueryRowContext(ctx, planSelectSQL+` WHERE id=?`, sub.PlanID))
	if err != nil {
		return nil, err
	}
	var snap model.TenantQuotaSnapshot
	snap.Plan = *plan
	snap.Subscription = *sub
	snap.PeriodKey = monthKey(now())
	queries := []struct {
		q string
		d *int64
	}{
		{`SELECT COUNT(*) FROM tenant_members WHERE tenant_id=? AND status='active'`, &snap.MembersUsed},
		{`SELECT COUNT(*) FROM short_links WHERE tenant_id=?`, &snap.ShortLinksUsed},
		{`SELECT COUNT(*) FROM live_qrs WHERE tenant_id=?`, &snap.LiveQRsUsed},
	}
	for _, q := range queries {
		if err := s.db.QueryRowContext(ctx, q.q, tenantID).Scan(q.d); err != nil {
			return nil, err
		}
	}
	_ = s.db.QueryRowContext(ctx, `SELECT visits FROM tenant_usage_monthly WHERE tenant_id=? AND period_key=?`, tenantID, snap.PeriodKey).Scan(&snap.MonthlyVisitsUsed)
	return &snap, nil
}

func planAndSubscriptionTx(ctx context.Context, tx *sql.Tx, mode string, tenantID int64) (*model.Plan, *model.TenantSubscription, error) {
	lock := ""
	if mode == "mysql" {
		lock = " FOR UPDATE"
	}
	sub, err := scanSubscription(tx.QueryRowContext(ctx, subscriptionSelectSQL+` WHERE s.tenant_id=?`+lock, tenantID))
	if err != nil {
		return nil, nil, err
	}
	plan, err := scanPlan(tx.QueryRowContext(ctx, planSelectSQL+` WHERE id=?`, sub.PlanID))
	if err != nil {
		return nil, nil, err
	}
	if !subscriptionUsable(sub.Status, sub.CurrentPeriodEnd, sub.TrialEndsAt, now()) {
		return nil, nil, ErrSubscriptionInactive
	}
	return plan, sub, nil
}

func checkQuotaTx(ctx context.Context, tx *sql.Tx, mode string, tenantID int64, resource string, delta int64) error {
	plan, _, err := planAndSubscriptionTx(ctx, tx, mode, tenantID)
	if err != nil {
		return err
	}
	var limit int64
	var count int64
	switch resource {
	case "members":
		limit = plan.MaxMembers
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenant_members WHERE tenant_id=? AND status='active'`, tenantID).Scan(&count)
	case "short_links":
		limit = plan.MaxShortLinks
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM short_links WHERE tenant_id=?`, tenantID).Scan(&count)
	case "live_qrs":
		limit = plan.MaxLiveQRs
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM live_qrs WHERE tenant_id=?`, tenantID).Scan(&count)
	default:
		return fmt.Errorf("unknown quota resource %q", resource)
	}
	if err != nil {
		return err
	}
	if limit > 0 && count+delta > limit {
		return fmt.Errorf("%w: %s limit is %d", ErrQuotaExceeded, resource, limit)
	}
	return nil
}

func (s *Store) CheckTenantQuota(ctx context.Context, tenantID int64, resource string, delta int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := checkQuotaTx(ctx, tx, s.mode, tenantID, resource, delta); err != nil {
		return err
	}
	return tx.Commit()
}

func ensureUsageTx(ctx context.Context, tx *sql.Tx, tenantID int64, period string) error {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenant_usage_monthly WHERE tenant_id=? AND period_key=?`, tenantID, period).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO tenant_usage_monthly(tenant_id,period_key) VALUES(?,?)`, tenantID, period); err != nil {
			var after int
			if qerr := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenant_usage_monthly WHERE tenant_id=? AND period_key=?`, tenantID, period).Scan(&after); qerr != nil || after == 0 {
				return err
			}
		}
	}
	return nil
}

func (s *Store) RequestPlanChange(ctx context.Context, tenantID, accountID, toPlanID int64, note string) (*model.SubscriptionChangeRequest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var fromPlanID int64
	if err := tx.QueryRowContext(ctx, `SELECT plan_id FROM tenant_subscriptions WHERE tenant_id=?`+s.lockSuffix(), tenantID).Scan(&fromPlanID); err != nil {
		return nil, err
	}
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plans WHERE id=? AND status='active'`, toPlanID).Scan(&active); err != nil || active == 0 {
		if err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	if fromPlanID == toPlanID {
		return nil, fmt.Errorf("tenant is already on this plan")
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO subscription_change_requests(tenant_id,from_plan_id,to_plan_id,status,pending_guard,note,requested_by) VALUES(?,?,?,?,?,?,?)`, tenantID, fromPlanID, toPlanID, "pending", "pending", strings.TrimSpace(note), accountID)
	if err != nil {
		return nil, fmt.Errorf("pending subscription request already exists: %w", err)
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetSubscriptionRequest(ctx, id)
}

func scanSubscriptionRequest(scanner interface{ Scan(...any) error }) (*model.SubscriptionChangeRequest, error) {
	var r model.SubscriptionChangeRequest
	var reviewedBy sql.NullInt64
	var reviewedAt sql.NullTime
	if err := scanner.Scan(&r.ID, &r.TenantID, &r.TenantName, &r.FromPlanID, &r.FromPlan, &r.ToPlanID, &r.ToPlan, &r.Status, &r.Note, &r.ReviewNote, &r.RequestedBy, &reviewedBy, &reviewedAt, &r.CreatedAt, &r.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if reviewedBy.Valid {
		r.ReviewedBy = reviewedBy.Int64
	}
	if reviewedAt.Valid {
		r.ReviewedAt = &reviewedAt.Time
	}
	return &r, nil
}

const subscriptionRequestSelectSQL = `SELECT r.id,r.tenant_id,t.name,r.from_plan_id,pf.name,r.to_plan_id,pt.name,r.status,r.note,r.review_note,r.requested_by,r.reviewed_by,r.reviewed_at,r.created_at,r.updated_at FROM subscription_change_requests r JOIN tenants t ON t.id=r.tenant_id JOIN plans pf ON pf.id=r.from_plan_id JOIN plans pt ON pt.id=r.to_plan_id`

func (s *Store) GetSubscriptionRequest(ctx context.Context, id int64) (*model.SubscriptionChangeRequest, error) {
	return scanSubscriptionRequest(s.db.QueryRowContext(ctx, subscriptionRequestSelectSQL+` WHERE r.id=?`, id))
}

func (s *Store) ListSubscriptionRequests(ctx context.Context, tenantID int64, platform bool) ([]model.SubscriptionChangeRequest, error) {
	query := subscriptionRequestSelectSQL
	args := []any{}
	if !platform {
		query += ` WHERE r.tenant_id=?`
		args = append(args, tenantID)
	}
	query += ` ORDER BY CASE r.status WHEN 'pending' THEN 0 ELSE 1 END,r.id DESC LIMIT 200`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.SubscriptionChangeRequest{}
	for rows.Next() {
		r, err := scanSubscriptionRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func (s *Store) ReviewSubscriptionRequest(ctx context.Context, requestID, reviewerID int64, action, note string) (*model.SubscriptionChangeRequest, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "approve" && action != "reject" {
		return nil, fmt.Errorf("action must be approve or reject")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var tenantID, toPlanID int64
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT tenant_id,to_plan_id,status FROM subscription_change_requests WHERE id=?`+s.lockSuffix(), requestID).Scan(&tenantID, &toPlanID, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if status != "pending" {
		return nil, ErrApprovalState
	}
	newStatus := "rejected"
	if action == "approve" {
		newStatus = "approved"
		if _, err := tx.ExecContext(ctx, `UPDATE tenant_subscriptions SET plan_id=?,status='active',provider='manual',current_period_start=?,current_period_end=NULL,cancel_at_period_end=0,updated_at=? WHERE tenant_id=?`, toPlanID, now(), now(), tenantID); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE subscription_change_requests SET status=?,pending_guard=NULL,review_note=?,reviewed_by=?,reviewed_at=?,updated_at=? WHERE id=?`, newStatus, strings.TrimSpace(note), reviewerID, now(), now(), requestID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetSubscriptionRequest(ctx, requestID)
}

func (s *Store) hydrateShortWorkspace(ctx context.Context, link *model.ShortLink, includeTargets bool) (*model.ShortLinkWorkspace, error) {
	var w model.ShortLinkWorkspace
	w.ShortLink = *link
	if err := s.db.QueryRowContext(ctx, `SELECT tenant_id,routing_strategy,current_target_cursor,content_version,approved_version FROM short_links WHERE id=?`, link.ID).Scan(&w.TenantID, &w.RoutingStrategy, &w.CurrentTargetCursor, &w.ContentVersion, &w.ApprovedVersion); err != nil {
		return nil, err
	}
	if includeTargets {
		targets, err := s.ListShortLinkTargets(ctx, link.ID, w.TenantID)
		if err != nil {
			return nil, err
		}
		w.Targets = targets
	}
	return &w, nil
}

func (s *Store) GetShortLinkForTenant(ctx context.Context, id, tenantID int64, includeTargets bool) (*model.ShortLinkWorkspace, error) {
	link, err := scanShort(s.db.QueryRowContext(ctx, shortSelectSQL()+` WHERE id=? AND tenant_id=?`, id, tenantID))
	if err != nil {
		return nil, err
	}
	return s.hydrateShortWorkspace(ctx, link, includeTargets)
}

func (s *Store) ListShortLinksForTenant(ctx context.Context, tenantID int64, q string, limit, offset int) ([]model.ShortLinkWorkspace, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := shortSelectSQL() + ` WHERE tenant_id=?`
	args := []any{tenantID}
	if strings.TrimSpace(q) != "" {
		like := "%" + strings.TrimSpace(q) + "%"
		query += ` AND (code LIKE ? OR title LIKE ? OR target_url LIKE ? OR approval_status LIKE ?)`
		args = append(args, like, like, like, like)
	}
	query += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := []*model.ShortLink{}
	for rows.Next() {
		link, err := scanShort(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	out := make([]model.ShortLinkWorkspace, 0, len(links))
	for _, link := range links {
		w, err := s.hydrateShortWorkspace(ctx, link, false)
		if err != nil {
			return nil, err
		}
		out = append(out, *w)
	}
	return out, nil
}

func normalizeRoutingStrategy(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "round_robin", "random", "weighted_random", "least_used", "ip_hash":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "single"
	}
}

func (s *Store) CreateShortLinkForTenant(ctx context.Context, tenantID int64, in *model.ShortLink, strategy string) (*model.ShortLinkWorkspace, error) {
	normalizeShortLink(in)
	strategy = normalizeRoutingStrategy(strategy)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := checkQuotaTx(ctx, tx, s.mode, tenantID, "short_links", 1); err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO short_links(owner_account_id,tenant_id,code,title,target_url,status,approval_status,redirect_type,starts_at,expires_at,max_visits,fallback_url,remark,qr_style,qr_foreground,qr_background,qr_logo_url,routing_strategy,content_version,approved_version) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,0)`, nullInt64(in.OwnerAccountID), tenantID, in.Code, in.Title, in.TargetURL, in.Status, "tenant_pending", in.RedirectType, in.StartsAt, in.ExpiresAt, in.MaxVisits, nullString(in.FallbackURL), nullString(in.Remark), in.QRStyle, in.QRForeground, in.QRBackground, nullString(in.QRLogoURL), strategy)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	period := monthKey(now())
	if err := ensureUsageTx(ctx, tx, tenantID, period); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tenant_usage_monthly SET short_links_created=short_links_created+1,updated_at=? WHERE tenant_id=? AND period_key=?`, now(), tenantID, period); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetShortLinkForTenant(ctx, id, tenantID, true)
}

func (s *Store) UpdateShortLinkForTenant(ctx context.Context, id, tenantID int64, in *model.ShortLink, strategy string) (*model.ShortLinkWorkspace, error) {
	normalizeShortLink(in)
	strategy = normalizeRoutingStrategy(strategy)
	res, err := s.db.ExecContext(ctx, `UPDATE short_links SET code=?,title=?,target_url=?,status=?,approval_status='tenant_pending',approved_at=NULL,reviewed_at=NULL,review_note=NULL,redirect_type=?,starts_at=?,expires_at=?,max_visits=?,fallback_url=?,remark=?,qr_style=?,qr_foreground=?,qr_background=?,qr_logo_url=?,routing_strategy=?,content_version=content_version+1,approved_version=0,updated_at=? WHERE id=? AND tenant_id=?`, in.Code, in.Title, in.TargetURL, in.Status, in.RedirectType, in.StartsAt, in.ExpiresAt, in.MaxVisits, nullString(in.FallbackURL), nullString(in.Remark), in.QRStyle, in.QRForeground, in.QRBackground, nullString(in.QRLogoURL), strategy, now(), id, tenantID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetShortLinkForTenant(ctx, id, tenantID, true)
}

func (s *Store) DeleteShortLinkForTenant(ctx context.Context, id, tenantID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM short_links WHERE id=? AND tenant_id=?`, id, tenantID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanShortTarget(scanner interface{ Scan(...any) error }) (*model.ShortLinkTarget, error) {
	var t model.ShortLinkTarget
	var starts, expires, lastHit sql.NullTime
	if err := scanner.Scan(&t.ID, &t.TenantID, &t.ShortLinkID, &t.Name, &t.TargetURL, &t.Status, &t.Weight, &t.SortOrder, &starts, &expires, &t.MaxHits, &t.HitCount, &t.HealthStatus, &lastHit, &t.CreatedAt, &t.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if starts.Valid {
		t.StartsAt = &starts.Time
	}
	if expires.Valid {
		t.ExpiresAt = &expires.Time
	}
	if lastHit.Valid {
		t.LastHitAt = &lastHit.Time
	}
	return &t, nil
}

const shortTargetSelectSQL = `SELECT id,tenant_id,short_link_id,name,target_url,status,weight,sort_order,starts_at,expires_at,max_hits,hit_count,health_status,last_hit_at,created_at,updated_at FROM short_link_targets`

func listShortTargetsQuery(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, shortID, tenantID int64, onlyAvailable bool, lock bool, mode string) ([]model.ShortLinkTarget, error) {
	query := shortTargetSelectSQL + ` WHERE short_link_id=? AND tenant_id=?`
	args := []any{shortID, tenantID}
	if onlyAvailable {
		n := now()
		query += ` AND status='active' AND health_status<>'unhealthy' AND (starts_at IS NULL OR starts_at<=?) AND (expires_at IS NULL OR expires_at>?) AND (max_hits=0 OR hit_count<max_hits)`
		args = append(args, n, n)
	}
	query += ` ORDER BY sort_order,id`
	if lock && mode == "mysql" {
		query += ` FOR UPDATE`
	}
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ShortLinkTarget{}
	for rows.Next() {
		t, err := scanShortTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (s *Store) ListShortLinkTargets(ctx context.Context, shortID, tenantID int64) ([]model.ShortLinkTarget, error) {
	return listShortTargetsQuery(ctx, s.db, shortID, tenantID, false, false, s.mode)
}

func (s *Store) SaveShortLinkTargets(ctx context.Context, shortID, tenantID int64, strategy string, targets []model.ShortLinkTarget) ([]model.ShortLinkTarget, error) {
	strategy = normalizeRoutingStrategy(strategy)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM short_links WHERE id=? AND tenant_id=?`, shortID, tenantID).Scan(&exists); err != nil || exists == 0 {
		if err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	plan, _, err := planAndSubscriptionTx(ctx, tx, s.mode, tenantID)
	if err != nil {
		return nil, err
	}
	if plan.MaxTargetsPerLink > 0 && int64(len(targets)) > plan.MaxTargetsPerLink {
		return nil, fmt.Errorf("%w: target limit is %d", ErrQuotaExceeded, plan.MaxTargetsPerLink)
	}
	existing, err := listShortTargetsQuery(ctx, tx, shortID, tenantID, false, true, s.mode)
	if err != nil {
		return nil, err
	}
	keep := map[int64]bool{}
	for i := range targets {
		t := &targets[i]
		if t.Weight <= 0 {
			t.Weight = 1
		}
		if t.SortOrder == 0 {
			t.SortOrder = 100
		}
		if t.Status == "" {
			t.Status = "active"
		}
		if t.HealthStatus == "" {
			t.HealthStatus = "unknown"
		}
		if t.ID > 0 {
			res, err := tx.ExecContext(ctx, `UPDATE short_link_targets SET name=?,target_url=?,status=?,weight=?,sort_order=?,starts_at=?,expires_at=?,max_hits=?,health_status=?,updated_at=? WHERE id=? AND short_link_id=? AND tenant_id=?`, t.Name, t.TargetURL, t.Status, t.Weight, t.SortOrder, t.StartsAt, t.ExpiresAt, t.MaxHits, t.HealthStatus, now(), t.ID, shortID, tenantID)
			if err != nil {
				return nil, err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return nil, ErrNotFound
			}
			keep[t.ID] = true
		} else {
			res, err := tx.ExecContext(ctx, `INSERT INTO short_link_targets(tenant_id,short_link_id,name,target_url,status,weight,sort_order,starts_at,expires_at,max_hits,health_status) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, tenantID, shortID, t.Name, t.TargetURL, t.Status, t.Weight, t.SortOrder, t.StartsAt, t.ExpiresAt, t.MaxHits, t.HealthStatus)
			if err != nil {
				return nil, err
			}
			t.ID, _ = res.LastInsertId()
			keep[t.ID] = true
		}
	}
	for _, old := range existing {
		if !keep[old.ID] {
			if _, err := tx.ExecContext(ctx, `DELETE FROM short_link_targets WHERE id=? AND short_link_id=? AND tenant_id=?`, old.ID, shortID, tenantID); err != nil {
				return nil, err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE short_links SET routing_strategy=?,current_target_cursor=0,approval_status='tenant_pending',approved_at=NULL,reviewed_at=NULL,review_note=NULL,content_version=content_version+1,approved_version=0,updated_at=? WHERE id=? AND tenant_id=?`, strategy, now(), shortID, tenantID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.ListShortLinkTargets(ctx, shortID, tenantID)
}

func uniformIndex(length int) int {
	if length <= 1 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(length)))
	if err != nil {
		return int(time.Now().UnixNano() % int64(length))
	}
	return int(n.Int64())
}

func weightedTargetIndex(items []model.ShortLinkTarget) int {
	var total int64
	for _, item := range items {
		weight := item.Weight
		if weight <= 0 {
			weight = 1
		}
		total += int64(weight)
	}
	if total <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(total))
	if err != nil {
		return uniformIndex(len(items))
	}
	threshold := n.Int64()
	var running int64
	for i, item := range items {
		weight := item.Weight
		if weight <= 0 {
			weight = 1
		}
		running += int64(weight)
		if threshold < running {
			return i
		}
	}
	return len(items) - 1
}

func ipHashIndex(key string, length int) int {
	if length <= 1 {
		return 0
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum64() % uint64(length))
}

func selectTargetIndex(strategy string, cursor int64, clientKey string, items []model.ShortLinkTarget) int {
	if len(items) == 0 {
		return -1
	}
	switch normalizeRoutingStrategy(strategy) {
	case "round_robin":
		return int(cursor % int64(len(items)))
	case "random":
		return uniformIndex(len(items))
	case "weighted_random":
		return weightedTargetIndex(items)
	case "least_used":
		idx := 0
		for i := 1; i < len(items); i++ {
			if items[i].HitCount < items[idx].HitCount {
				idx = i
			}
		}
		return idx
	case "ip_hash":
		return ipHashIndex(clientKey, len(items))
	default:
		return 0
	}
}

func (s *Store) SelectShortTargetForVisit(ctx context.Context, shortID int64, clientKey string) (*model.ShortRouteDecision, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	query := `SELECT tenant_id,target_url,status,approval_status,starts_at,expires_at,max_visits,visit_count,routing_strategy,current_target_cursor,content_version,approved_version FROM short_links WHERE id=?` + s.lockSuffix()
	var tenantID, maxVisits, visits, cursor, contentVersion, approvedVersion int64
	var legacyURL, status, approval, strategy string
	var starts, expires sql.NullTime
	if err := tx.QueryRowContext(ctx, query, shortID).Scan(&tenantID, &legacyURL, &status, &approval, &starts, &expires, &maxVisits, &visits, &strategy, &cursor, &contentVersion, &approvedVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	n := now()
	if approval != "approved" || contentVersion != approvedVersion {
		return nil, ErrNotPublished
	}
	if status != "active" {
		return nil, fmt.Errorf("resource disabled")
	}
	if starts.Valid && n.Before(starts.Time) {
		return nil, fmt.Errorf("resource not started")
	}
	if expires.Valid && !n.Before(expires.Time) {
		return nil, fmt.Errorf("resource expired")
	}
	if maxVisits > 0 && visits >= maxVisits {
		return nil, ErrVisitLimitReached
	}
	plan, _, err := planAndSubscriptionTx(ctx, tx, s.mode, tenantID)
	if err != nil {
		return nil, err
	}
	period := monthKey(n)
	if err := ensureUsageTx(ctx, tx, tenantID, period); err != nil {
		return nil, err
	}
	var used int64
	usageQuery := `SELECT visits FROM tenant_usage_monthly WHERE tenant_id=? AND period_key=?`
	if s.mode == "mysql" {
		usageQuery += ` FOR UPDATE`
	}
	if err := tx.QueryRowContext(ctx, usageQuery, tenantID, period).Scan(&used); err != nil {
		return nil, err
	}
	if plan.MonthlyVisits > 0 && used >= plan.MonthlyVisits {
		return nil, fmt.Errorf("%w: monthly visit limit is %d", ErrQuotaExceeded, plan.MonthlyVisits)
	}
	targets, err := listShortTargetsQuery(ctx, tx, shortID, tenantID, true, true, s.mode)
	if err != nil {
		return nil, err
	}
	decision := &model.ShortRouteDecision{TenantID: tenantID, TargetURL: legacyURL, Strategy: normalizeRoutingStrategy(strategy), Counted: true}
	idx := selectTargetIndex(strategy, cursor, clientKey, targets)
	if idx >= 0 {
		chosen := targets[idx]
		decision.TargetURL = chosen.TargetURL
		decision.TargetID = &chosen.ID
		if _, err := tx.ExecContext(ctx, `UPDATE short_link_targets SET hit_count=hit_count+1,last_hit_at=?,updated_at=? WHERE id=?`, n, n, chosen.ID); err != nil {
			return nil, err
		}
	}
	if normalizeRoutingStrategy(strategy) == "round_robin" && len(targets) > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE short_links SET current_target_cursor=current_target_cursor+1 WHERE id=?`, shortID); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE short_links SET visit_count=visit_count+1 WHERE id=?`, shortID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tenant_usage_monthly SET visits=visits+1,updated_at=? WHERE tenant_id=? AND period_key=?`, n, tenantID, period); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return decision, nil
}

func (s *Store) RecordVisitSaaS(ctx context.Context, v *model.VisitLog, tenantID int64, targetID *int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO visit_logs(resource_type,resource_id,item_id,code,event_type,status,target_url,ip,ip_hash,user_agent,referer,device_type,browser,os,tenant_id,target_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, v.ResourceType, v.ResourceID, v.ItemID, v.Code, v.EventType, v.Status, nullString(v.TargetURL), v.IP, v.IPHash, v.UserAgent, nullString(v.Referer), v.DeviceType, v.Browser, v.OS, nullInt64(tenantID), targetID)
	return err
}

func (s *Store) AuditTenant(ctx context.Context, actorDeviceID *int64, tenantID int64, action, resourceType string, resourceID *int64, detail, ip string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_logs(actor_device_id,tenant_id,action,resource_type,resource_id,detail,ip) VALUES(?,?,?,?,?,?,?)`, actorDeviceID, nullInt64(tenantID), action, resourceType, resourceID, detail, ip)
	return err
}

func resourceTable(resourceType string) (table string, parentJoin string, err error) {
	switch resourceType {
	case "short_link":
		return "short_links", "", nil
	case "live_qr":
		return "live_qrs", "", nil
	case "live_qr_item":
		return "live_qr_items", "live_qrs", nil
	default:
		return "", "", fmt.Errorf("unsupported resource type")
	}
}

func getResourceReviewStateTx(ctx context.Context, tx *sql.Tx, mode, resourceType string, resourceID int64) (tenantID, version int64, status string, err error) {
	table, parent, err := resourceTable(resourceType)
	if err != nil {
		return 0, 0, "", err
	}
	lock := ""
	if mode == "mysql" {
		lock = " FOR UPDATE"
	}
	if parent == "" {
		query := fmt.Sprintf(`SELECT tenant_id,content_version,approval_status FROM %s WHERE id=?%s`, table, lock)
		err = tx.QueryRowContext(ctx, query, resourceID).Scan(&tenantID, &version, &status)
	} else {
		query := fmt.Sprintf(`SELECT l.tenant_id,i.content_version,i.approval_status FROM %s i JOIN %s l ON l.id=i.live_qr_id WHERE i.id=?%s`, table, parent, lock)
		err = tx.QueryRowContext(ctx, query, resourceID).Scan(&tenantID, &version, &status)
	}
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return
}

func updateReviewStateTx(ctx context.Context, tx *sql.Tx, resourceType string, resourceID int64, status string, approved bool, note string) error {
	table, _, err := resourceTable(resourceType)
	if err != nil {
		return err
	}
	approvedExpr := "0"
	approvedAt := any(nil)
	if approved {
		approvedExpr = "content_version"
		approvedAt = now()
	}
	query := fmt.Sprintf(`UPDATE %s SET approval_status=?,approved_version=%s,approved_at=?,reviewed_at=?,review_note=?,updated_at=? WHERE id=?`, table, approvedExpr)
	_, err = tx.ExecContext(ctx, query, status, approvedAt, now(), nullString(note), now(), resourceID)
	return err
}

func insertApprovalEventTx(ctx context.Context, tx *sql.Tx, tenantID int64, resourceType string, resourceID, version int64, stage, action string, actorID int64, note string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO approval_events(tenant_id,resource_type,resource_id,content_version,stage,action,actor_account_id,note) VALUES(?,?,?,?,?,?,?,?)`, tenantID, resourceType, resourceID, version, stage, action, actorID, strings.TrimSpace(note))
	return err
}

func (s *Store) ReviewResourceTenant(ctx context.Context, tenantID, actorID int64, resourceType string, resourceID int64, action, note string, includeItems bool) error {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "approve" && action != "reject" {
		return fmt.Errorf("action must be approve or reject")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	actualTenant, version, current, err := getResourceReviewStateTx(ctx, tx, s.mode, resourceType, resourceID)
	if err != nil {
		return err
	}
	if actualTenant != tenantID {
		return ErrTenantForbidden
	}
	if current != "tenant_pending" && current != "tenant_rejected" {
		return fmt.Errorf("%w: expected tenant_pending, got %s", ErrApprovalState, current)
	}
	newStatus := "tenant_rejected"
	if action == "approve" {
		newStatus = "platform_pending"
	}
	if err := updateReviewStateTx(ctx, tx, resourceType, resourceID, newStatus, false, note); err != nil {
		return err
	}
	if err := insertApprovalEventTx(ctx, tx, tenantID, resourceType, resourceID, version, "tenant", action, actorID, note); err != nil {
		return err
	}
	if resourceType == "live_qr" && includeItems {
		rows, err := tx.QueryContext(ctx, `SELECT id,content_version FROM live_qr_items WHERE live_qr_id=?`, resourceID)
		if err != nil {
			return err
		}
		type itemVersion struct{ id, version int64 }
		items := []itemVersion{}
		for rows.Next() {
			var item itemVersion
			if err := rows.Scan(&item.id, &item.version); err != nil {
				rows.Close()
				return err
			}
			items = append(items, item)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, item := range items {
			if err := updateReviewStateTx(ctx, tx, "live_qr_item", item.id, newStatus, false, note); err != nil {
				return err
			}
			if err := insertApprovalEventTx(ctx, tx, tenantID, "live_qr_item", item.id, item.version, "tenant", action, actorID, note); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) ReviewResourcePlatform(ctx context.Context, actorID int64, resourceType string, resourceID int64, action, note string, includeItems bool) error {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "approve" && action != "reject" {
		return fmt.Errorf("action must be approve or reject")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	tenantID, version, current, err := getResourceReviewStateTx(ctx, tx, s.mode, resourceType, resourceID)
	if err != nil {
		return err
	}
	if current != "platform_pending" && current != "platform_rejected" {
		return fmt.Errorf("%w: expected platform_pending, got %s", ErrApprovalState, current)
	}
	newStatus := "platform_rejected"
	approved := false
	if action == "approve" {
		newStatus = "approved"
		approved = true
	}
	if err := updateReviewStateTx(ctx, tx, resourceType, resourceID, newStatus, approved, note); err != nil {
		return err
	}
	if err := insertApprovalEventTx(ctx, tx, tenantID, resourceType, resourceID, version, "platform", action, actorID, note); err != nil {
		return err
	}
	if resourceType == "live_qr" && includeItems {
		rows, err := tx.QueryContext(ctx, `SELECT id,content_version,approval_status FROM live_qr_items WHERE live_qr_id=?`, resourceID)
		if err != nil {
			return err
		}
		type itemState struct {
			id, version int64
			status      string
		}
		items := []itemState{}
		for rows.Next() {
			var item itemState
			if err := rows.Scan(&item.id, &item.version, &item.status); err != nil {
				rows.Close()
				return err
			}
			items = append(items, item)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, item := range items {
			if item.status != "platform_pending" && item.status != "platform_rejected" {
				continue
			}
			if err := updateReviewStateTx(ctx, tx, "live_qr_item", item.id, newStatus, approved, note); err != nil {
				return err
			}
			if err := insertApprovalEventTx(ctx, tx, tenantID, "live_qr_item", item.id, item.version, "platform", action, actorID, note); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) ListApprovalQueue(ctx context.Context, tenantID int64, stage string, platform bool) ([]model.ApprovalQueueItem, error) {
	stage = strings.TrimSpace(stage)
	if stage == "" {
		if platform {
			stage = "platform_pending"
		} else {
			stage = "tenant_pending"
		}
	}
	whereTenant := ""
	args := []any{stage}
	if !platform {
		whereTenant = " AND x.tenant_id=?"
		args = append(args, tenantID)
	}
	query := `SELECT x.tenant_id,t.name,x.resource_type,x.resource_id,x.code,x.title,x.approval_status,x.content_version,x.owner_account_id,x.updated_at FROM (` +
		`SELECT tenant_id,'short_link' resource_type,id resource_id,code,title,approval_status,content_version,owner_account_id,updated_at FROM short_links ` +
		`UNION ALL SELECT tenant_id,'live_qr' resource_type,id resource_id,code,title,approval_status,content_version,owner_account_id,updated_at FROM live_qrs` +
		`) x JOIN tenants t ON t.id=x.tenant_id WHERE x.approval_status=?` + whereTenant + ` ORDER BY x.updated_at ASC LIMIT 500`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ApprovalQueueItem{}
	for rows.Next() {
		var item model.ApprovalQueueItem
		var owner sql.NullInt64
		if err := rows.Scan(&item.TenantID, &item.TenantName, &item.ResourceType, &item.ResourceID, &item.Code, &item.Title, &item.ApprovalStatus, &item.ContentVersion, &owner, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if owner.Valid {
			item.OwnerAccountID = owner.Int64
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ListApprovalEvents(ctx context.Context, tenantID int64, resourceType string, resourceID int64) ([]model.ApprovalEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,tenant_id,resource_type,resource_id,content_version,stage,action,actor_account_id,note,created_at FROM approval_events WHERE tenant_id=? AND resource_type=? AND resource_id=? ORDER BY id DESC`, tenantID, resourceType, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ApprovalEvent{}
	for rows.Next() {
		var e model.ApprovalEvent
		if err := rows.Scan(&e.ID, &e.TenantID, &e.ResourceType, &e.ResourceID, &e.ContentVersion, &e.Stage, &e.Action, &e.ActorAccountID, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) hydrateLiveWorkspace(ctx context.Context, live *model.LiveQR) (*model.LiveQRWorkspace, error) {
	var w model.LiveQRWorkspace
	w.LiveQR = *live
	if err := s.db.QueryRowContext(ctx, `SELECT tenant_id,content_version,approved_version FROM live_qrs WHERE id=?`, live.ID).Scan(&w.TenantID, &w.ContentVersion, &w.ApprovedVersion); err != nil {
		return nil, err
	}
	return &w, nil
}

func (s *Store) GetLiveQRForTenant(ctx context.Context, id, tenantID int64) (*model.LiveQRWorkspace, error) {
	live, err := scanLive(s.db.QueryRowContext(ctx, liveSelectSQL()+` WHERE id=? AND tenant_id=?`, id, tenantID))
	if err != nil {
		return nil, err
	}
	return s.hydrateLiveWorkspace(ctx, live)
}

func (s *Store) ListLiveQRsForTenant(ctx context.Context, tenantID int64, q string, limit, offset int) ([]model.LiveQRWorkspace, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := liveSelectSQL() + ` WHERE tenant_id=?`
	args := []any{tenantID}
	if strings.TrimSpace(q) != "" {
		like := "%" + strings.TrimSpace(q) + "%"
		query += ` AND (code LIKE ? OR title LIKE ? OR description LIKE ? OR approval_status LIKE ?)`
		args = append(args, like, like, like, like)
	}
	query += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lives := []*model.LiveQR{}
	for rows.Next() {
		live, err := scanLive(rows)
		if err != nil {
			return nil, err
		}
		lives = append(lives, live)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	out := make([]model.LiveQRWorkspace, 0, len(lives))
	for _, live := range lives {
		w, err := s.hydrateLiveWorkspace(ctx, live)
		if err != nil {
			return nil, err
		}
		out = append(out, *w)
	}
	return out, nil
}

func (s *Store) BindLiveQRToTenantAndReset(ctx context.Context, liveID, tenantID int64, created bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if created {
		if _, err := tx.ExecContext(ctx, `UPDATE live_qrs SET tenant_id=?,approval_status='tenant_pending',content_version=1,approved_version=0,approved_at=NULL,reviewed_at=NULL,review_note=NULL,updated_at=? WHERE id=?`, tenantID, now(), liveID); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE live_qrs SET tenant_id=?,approval_status='tenant_pending',content_version=content_version+1,approved_version=0,approved_at=NULL,reviewed_at=NULL,review_note=NULL,updated_at=? WHERE id=? AND tenant_id=?`, tenantID, now(), liveID, tenantID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE live_qr_items SET approval_status='tenant_pending',content_version=content_version+1,approved_version=0,approved_at=NULL,reviewed_at=NULL,review_note=NULL,updated_at=? WHERE live_qr_id=?`, now(), liveID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) BindLiveQRItemAndReset(ctx context.Context, itemID, liveID, tenantID int64, created bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var actualTenant int64
	if err := tx.QueryRowContext(ctx, `SELECT tenant_id FROM live_qrs WHERE id=?`, liveID).Scan(&actualTenant); err != nil {
		return err
	}
	if actualTenant != tenantID {
		return ErrTenantForbidden
	}
	if created {
		if _, err := tx.ExecContext(ctx, `UPDATE live_qr_items SET approval_status='tenant_pending',content_version=1,approved_version=0,approved_at=NULL,reviewed_at=NULL,review_note=NULL,updated_at=? WHERE id=? AND live_qr_id=?`, now(), itemID, liveID); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE live_qr_items SET approval_status='tenant_pending',content_version=content_version+1,approved_version=0,approved_at=NULL,reviewed_at=NULL,review_note=NULL,updated_at=? WHERE id=? AND live_qr_id=?`, now(), itemID, liveID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE live_qrs SET approval_status='tenant_pending',content_version=content_version+1,approved_version=0,approved_at=NULL,reviewed_at=NULL,review_note=NULL,updated_at=? WHERE id=? AND tenant_id=?`, now(), liveID, tenantID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetLiveTenantID(ctx context.Context, liveID int64) (int64, error) {
	var tenantID int64
	if err := s.db.QueryRowContext(ctx, `SELECT tenant_id FROM live_qrs WHERE id=?`, liveID).Scan(&tenantID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return tenantID, nil
}

func (s *Store) IncrementTenantVisitForLive(ctx context.Context, tenantID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	plan, _, err := planAndSubscriptionTx(ctx, tx, s.mode, tenantID)
	if err != nil {
		return err
	}
	period := monthKey(now())
	if err := ensureUsageTx(ctx, tx, tenantID, period); err != nil {
		return err
	}
	var used int64
	q := `SELECT visits FROM tenant_usage_monthly WHERE tenant_id=? AND period_key=?`
	if s.mode == "mysql" {
		q += ` FOR UPDATE`
	}
	if err := tx.QueryRowContext(ctx, q, tenantID, period).Scan(&used); err != nil {
		return err
	}
	if plan.MonthlyVisits > 0 && used >= plan.MonthlyVisits {
		return ErrQuotaExceeded
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tenant_usage_monthly SET visits=visits+1,updated_at=? WHERE tenant_id=? AND period_key=?`, now(), tenantID, period); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CountTenantResources(ctx context.Context, tenantID int64) (map[string]int64, error) {
	out := map[string]int64{}
	for key, query := range map[string]string{
		"short_links": `SELECT COUNT(*) FROM short_links WHERE tenant_id=?`,
		"live_qrs":    `SELECT COUNT(*) FROM live_qrs WHERE tenant_id=?`,
		"members":     `SELECT COUNT(*) FROM tenant_members WHERE tenant_id=? AND status='active'`,
	} {
		var count int64
		if err := s.db.QueryRowContext(ctx, query, tenantID).Scan(&count); err != nil {
			return nil, err
		}
		out[key] = count
	}
	return out, nil
}

func (s *Store) ApprovalEventsByResource(ctx context.Context, tenantID int64, resourceType string, resourceID int64) ([]model.ApprovalEvent, error) {
	return s.ListApprovalEvents(ctx, tenantID, resourceType, resourceID)
}

func sortedTargetIDs(items []model.ShortLinkTarget) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

var _ = tenantCanWrite
var _ = tenantCanReview
var _ = sortedTargetIDs

func (s *Store) ListAllTenants(ctx context.Context, q string, limit, offset int) ([]model.Tenant, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := tenantSelectSQL
	args := []any{}
	if strings.TrimSpace(q) != "" {
		like := "%" + strings.TrimSpace(q) + "%"
		query += ` WHERE slug LIKE ? OR name LIKE ? OR status LIKE ?`
		args = append(args, like, like, like)
	}
	query += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Tenant{}
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (s *Store) ResourceTenantID(ctx context.Context, resourceType string, resourceID int64) (int64, error) {
	table, parent, err := resourceTable(resourceType)
	if err != nil {
		return 0, err
	}
	var tenantID int64
	if parent == "" {
		err = s.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT tenant_id FROM %s WHERE id=?`, table), resourceID).Scan(&tenantID)
	} else {
		err = s.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT l.tenant_id FROM %s i JOIN %s l ON l.id=i.live_qr_id WHERE i.id=?`, table, parent), resourceID).Scan(&tenantID)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return tenantID, err
}
