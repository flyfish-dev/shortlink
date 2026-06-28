package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"ai-shortlink/internal/model"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db   *sql.DB
	mode string
}

func New(db *sql.DB, mode ...string) *Store {
	m := "mysql"
	if len(mode) > 0 && strings.TrimSpace(mode[0]) != "" {
		m = strings.ToLower(strings.TrimSpace(mode[0]))
	}
	return &Store{db: db, mode: m}
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) CountAdminDevices(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_devices WHERE revoked_at IS NULL").Scan(&n)
	return n, err
}

func (s *Store) CountAdminAccounts(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_accounts").Scan(&n)
	return n, err
}

func (s *Store) CreateAdminAccount(ctx context.Context, email, name, recoveryKeyHash, recoveryKeyCipher string) (*model.AdminAccount, error) {
	return s.CreateAccount(ctx, email, name, "admin", "active", recoveryKeyHash, recoveryKeyCipher)
}

func (s *Store) CreateUserAccount(ctx context.Context, email, name, recoveryKeyHash, recoveryKeyCipher string) (*model.AdminAccount, error) {
	return s.CreateAccount(ctx, email, name, "user", "active", recoveryKeyHash, recoveryKeyCipher)
}

func (s *Store) CreateAccount(ctx context.Context, email, name, role, status, recoveryKeyHash, recoveryKeyCipher string) (*model.AdminAccount, error) {
	role = normalizeRole(role)
	status = normalizeAccountStatus(status)
	res, err := s.db.ExecContext(ctx, `INSERT INTO admin_accounts(email,name,role,status,recovery_key_hash,recovery_key_cipher) VALUES(?,?,?,?,?,?)`, nullString(email), strings.TrimSpace(name), role, status, recoveryKeyHash, recoveryKeyCipher)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetAdminAccount(ctx, id)
}

func (s *Store) GetAdminAccount(ctx context.Context, id int64) (*model.AdminAccount, error) {
	row := s.db.QueryRowContext(ctx, adminAccountSelectSQL()+` WHERE id=? LIMIT 1`, id)
	return scanAdminAccount(row)
}

func (s *Store) FindAdminAccountByRecoveryHash(ctx context.Context, recoveryKeyHash string) (*model.AdminAccount, error) {
	row := s.db.QueryRowContext(ctx, adminAccountSelectSQL()+` WHERE recovery_key_hash=? LIMIT 1`, recoveryKeyHash)
	return scanAdminAccount(row)
}

func (s *Store) FindAdminAccountByEmail(ctx context.Context, email string) (*model.AdminAccount, error) {
	row := s.db.QueryRowContext(ctx, adminAccountSelectSQL()+` WHERE LOWER(email)=LOWER(?) LIMIT 1`, strings.TrimSpace(email))
	return scanAdminAccount(row)
}

func (s *Store) UpdateAdminAccountRecoveryKey(ctx context.Context, id int64, recoveryKeyHash, recoveryKeyCipher string) (*model.AdminAccount, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE admin_accounts SET recovery_key_hash=?, recovery_key_cipher=?, updated_at=? WHERE id=?`, recoveryKeyHash, recoveryKeyCipher, now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetAdminAccount(ctx, id)
}

func (s *Store) UpdateAdminAccountEmailName(ctx context.Context, id int64, email, name string) (*model.AdminAccount, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE admin_accounts SET email=?, name=?, updated_at=? WHERE id=?`, nullString(email), strings.TrimSpace(name), now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetAdminAccount(ctx, id)
}

func (s *Store) ListAdminAccounts(ctx context.Context, q string, limit, offset int) ([]model.AdminAccount, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	base := adminAccountSelectSQL()
	var rows *sql.Rows
	var err error
	if strings.TrimSpace(q) != "" {
		like := "%" + strings.TrimSpace(q) + "%"
		rows, err = s.db.QueryContext(ctx, base+` WHERE email LIKE ? OR name LIKE ? OR role LIKE ? OR status LIKE ? ORDER BY id DESC LIMIT ? OFFSET ?`, like, like, like, like, limit, offset)
	} else {
		rows, err = s.db.QueryContext(ctx, base+` ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.AdminAccount{}
	for rows.Next() {
		acct, err := scanAdminAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *acct)
	}
	return out, rows.Err()
}

func (s *Store) UpdateAdminAccountRoleStatus(ctx context.Context, id int64, name, role, status string) (*model.AdminAccount, error) {
	role = normalizeRole(role)
	status = normalizeAccountStatus(status)
	res, err := s.db.ExecContext(ctx, `UPDATE admin_accounts SET name=?, role=?, status=?, updated_at=? WHERE id=?`, strings.TrimSpace(name), role, status, now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetAdminAccount(ctx, id)
}

func (s *Store) CountActiveAdmins(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_accounts WHERE role='admin' AND status='active'`).Scan(&n)
	return n, err
}

func adminAccountSelectSQL() string {
	return `SELECT id, email, name, role, status, recovery_key_hash, recovery_key_cipher, created_at, updated_at FROM admin_accounts`
}

func scanAdminAccount(scanner interface{ Scan(dest ...any) error }) (*model.AdminAccount, error) {
	var a model.AdminAccount
	var email sql.NullString
	if err := scanner.Scan(&a.ID, &email, &a.Name, &a.Role, &a.Status, &a.RecoveryKeyHash, &a.RecoveryKeyCipher, &a.CreatedAt, &a.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if email.Valid {
		a.Email = email.String
	}
	a.Role = normalizeRole(a.Role)
	a.Status = normalizeAccountStatus(a.Status)
	return &a, nil
}

func normalizeRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "admin" {
		return "admin"
	}
	return "user"
}

func normalizeAccountStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "disabled" {
		return "disabled"
	}
	return "active"
}

func (s *Store) FindAdminDevice(ctx context.Context, browserHash, ipHash string) (*model.AdminDevice, error) {
	row := s.db.QueryRowContext(ctx, adminDeviceSelectSQL()+` WHERE browser_hash=? AND ip_hash=? AND revoked_at IS NULL LIMIT 1`, browserHash, ipHash)
	return scanAdminDevice(row)
}

func (s *Store) FindAdminDeviceByBrowserHash(ctx context.Context, browserHash string) (*model.AdminDevice, error) {
	row := s.db.QueryRowContext(ctx, adminDeviceSelectSQL()+` WHERE browser_hash=? AND revoked_at IS NULL ORDER BY COALESCE(last_seen_at, created_at) DESC LIMIT 1`, browserHash)
	return scanAdminDevice(row)
}

func (s *Store) GetAdminDevice(ctx context.Context, id int64) (*model.AdminDevice, error) {
	row := s.db.QueryRowContext(ctx, adminDeviceSelectSQL()+` WHERE id=? AND revoked_at IS NULL LIMIT 1`, id)
	return scanAdminDevice(row)
}

func (s *Store) CreateAdminDevice(ctx context.Context, accountID int64, label, browserHash, ipHash, ip, ua string) (*model.AdminDevice, error) {
	n := now()
	res, err := s.db.ExecContext(ctx, `INSERT INTO admin_devices(account_id,label,browser_hash,ip_hash,ip_last,user_agent_last,last_seen_at) VALUES(?,?,?,?,?,?,?)`, nullInt64(accountID), label, browserHash, ipHash, ip, ua, n)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetAdminDevice(ctx, id)
}

func (s *Store) UpdateAdminDeviceAccount(ctx context.Context, deviceID, accountID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE admin_devices SET account_id=? WHERE id=? AND revoked_at IS NULL`, nullInt64(accountID), deviceID)
	return err
}

func (s *Store) TouchAdminDevice(ctx context.Context, id int64, ip, ua string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE admin_devices SET ip_last=?, user_agent_last=?, last_seen_at=? WHERE id=? AND revoked_at IS NULL`, ip, ua, now(), id)
	return err
}

func (s *Store) Audit(ctx context.Context, actorDeviceID *int64, action, resourceType string, resourceID *int64, detail, ip string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_logs(actor_device_id,action,resource_type,resource_id,detail,ip) VALUES(?,?,?,?,?,?)`, actorDeviceID, action, resourceType, resourceID, detail, ip)
	return err
}

func adminDeviceSelectSQL() string {
	return `SELECT id, account_id, label, browser_hash, ip_hash, ip_last, user_agent_last, created_at, last_seen_at, revoked_at FROM admin_devices`
}

func scanAdminDevice(scanner interface{ Scan(dest ...any) error }) (*model.AdminDevice, error) {
	var d model.AdminDevice
	var accountID sql.NullInt64
	var last, revoked sql.NullTime
	if err := scanner.Scan(&d.ID, &accountID, &d.Label, &d.BrowserHash, &d.IPHash, &d.IPLast, &d.UserAgentLast, &d.CreatedAt, &last, &revoked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if accountID.Valid {
		d.AccountID = accountID.Int64
	}
	if last.Valid {
		d.LastSeenAt = &last.Time
	}
	if revoked.Valid {
		d.RevokedAt = &revoked.Time
	}
	return &d, nil
}

func (s *Store) GetSettings(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT setting_key, setting_value FROM system_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k string
		var v sql.NullString
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		if v.Valid {
			out[k] = v.String
		} else {
			out[k] = ""
		}
	}
	return out, rows.Err()
}

func (s *Store) SetSettings(ctx context.Context, vals map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for k, v := range vals {
		if strings.TrimSpace(k) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `REPLACE INTO system_settings(setting_key, setting_value, updated_at) VALUES(?,?,?)`, k, v, now()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var v sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT setting_value FROM system_settings WHERE setting_key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if v.Valid {
		return v.String, nil
	}
	return "", nil
}

func (s *Store) Overview(ctx context.Context) (*model.Overview, error) {
	var o model.Overview
	n := now()
	start := beginningOfDay(n)
	queries := []struct {
		sql  string
		args []any
		dest *int64
	}{
		{"SELECT COUNT(*) FROM short_links", nil, &o.ShortLinks},
		{"SELECT COUNT(*) FROM short_links WHERE approval_status='pending'", nil, &o.ShortPending},
		{"SELECT COUNT(*) FROM live_qrs", nil, &o.LiveQRs},
		{"SELECT COUNT(*) FROM live_qrs WHERE approval_status='pending'", nil, &o.LivePending},
		{"SELECT COUNT(*) FROM live_qr_items WHERE approval_status='pending'", nil, &o.LiveItemsPending},
		{"SELECT COUNT(*) FROM live_qr_items WHERE status='active' AND approval_status='approved' AND (expires_at IS NULL OR expires_at > ?) AND (starts_at IS NULL OR starts_at <= ?)", []any{n, n}, &o.LiveItemsActive},
		{"SELECT COUNT(*) FROM visit_logs WHERE created_at >= ?", []any{start}, &o.VisitsToday},
		{"SELECT COUNT(*) FROM visit_logs", nil, &o.VisitsTotal},
	}
	for _, q := range queries {
		args := q.args
		if args == nil {
			args = []any{}
		}
		if err := s.db.QueryRowContext(ctx, q.sql, args...).Scan(q.dest); err != nil {
			return nil, err
		}
	}
	settings, _ := s.GetSettings(ctx)
	o.SMTPConfigured = settingsBool(settings, "smtp_enabled") && strings.TrimSpace(settings["smtp_host"]) != "" && strings.TrimSpace(settings["smtp_from"]) != "" && strings.TrimSpace(settings["smtp_password_cipher"]) != ""
	o.BaseURLConfigured = strings.TrimSpace(settings["base_url"]) != ""
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_accounts WHERE role='user'").Scan(&o.Users)
	return &o, nil
}

func (s *Store) OverviewForAccount(ctx context.Context, accountID int64, isAdmin bool) (*model.Overview, error) {
	if isAdmin {
		return s.Overview(ctx)
	}
	var o model.Overview
	n := now()
	start := beginningOfDay(n)
	queries := []struct {
		sql  string
		args []any
		dest *int64
	}{
		{"SELECT COUNT(*) FROM short_links WHERE owner_account_id=?", []any{accountID}, &o.ShortLinks},
		{"SELECT COUNT(*) FROM short_links WHERE owner_account_id=? AND approval_status='pending'", []any{accountID}, &o.ShortPending},
		{"SELECT COUNT(*) FROM live_qrs WHERE owner_account_id=?", []any{accountID}, &o.LiveQRs},
		{"SELECT COUNT(*) FROM live_qrs WHERE owner_account_id=? AND approval_status='pending'", []any{accountID}, &o.LivePending},
		{"SELECT COUNT(*) FROM live_qr_items i JOIN live_qrs l ON l.id=i.live_qr_id WHERE l.owner_account_id=? AND i.approval_status='pending'", []any{accountID}, &o.LiveItemsPending},
		{"SELECT COUNT(*) FROM live_qr_items i JOIN live_qrs l ON l.id=i.live_qr_id WHERE l.owner_account_id=? AND i.status='active' AND i.approval_status='approved' AND (i.expires_at IS NULL OR i.expires_at > ?) AND (i.starts_at IS NULL OR i.starts_at <= ?)", []any{accountID, n, n}, &o.LiveItemsActive},
		{"SELECT COUNT(*) FROM visit_logs v JOIN short_links s ON s.id=v.resource_id AND v.resource_type='short_link' WHERE s.owner_account_id=? AND v.created_at >= ?", []any{accountID, start}, &o.VisitsToday},
		{"SELECT COUNT(*) FROM visit_logs v JOIN short_links s ON s.id=v.resource_id AND v.resource_type='short_link' WHERE s.owner_account_id=?", []any{accountID}, &o.VisitsTotal},
	}
	for _, q := range queries {
		if err := s.db.QueryRowContext(ctx, q.sql, q.args...).Scan(q.dest); err != nil {
			return nil, err
		}
	}
	var todayLive, totalLive int64
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM visit_logs v JOIN live_qrs l ON l.id=v.resource_id AND v.resource_type='live_qr' WHERE l.owner_account_id=? AND v.created_at >= ?", accountID, start).Scan(&todayLive)
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM visit_logs v JOIN live_qrs l ON l.id=v.resource_id AND v.resource_type='live_qr' WHERE l.owner_account_id=?", accountID).Scan(&totalLive)
	o.VisitsToday += todayLive
	o.VisitsTotal += totalLive
	settings, _ := s.GetSettings(ctx)
	o.SMTPConfigured = settingsBool(settings, "smtp_enabled") && strings.TrimSpace(settings["smtp_host"]) != "" && strings.TrimSpace(settings["smtp_from"]) != "" && strings.TrimSpace(settings["smtp_password_cipher"]) != ""
	o.BaseURLConfigured = strings.TrimSpace(settings["base_url"]) != ""
	return &o, nil
}

func (s *Store) CreateShortLink(ctx context.Context, in *model.ShortLink) (*model.ShortLink, error) {
	normalizeShortLink(in)
	res, err := s.db.ExecContext(ctx, `INSERT INTO short_links(owner_account_id,code,title,target_url,status,approval_status,redirect_type,starts_at,expires_at,max_visits,fallback_url,remark,qr_style,qr_foreground,qr_background,qr_logo_url) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, nullInt64(in.OwnerAccountID), in.Code, in.Title, in.TargetURL, in.Status, "pending", in.RedirectType, in.StartsAt, in.ExpiresAt, in.MaxVisits, nullString(in.FallbackURL), nullString(in.Remark), in.QRStyle, in.QRForeground, in.QRBackground, nullString(in.QRLogoURL))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetShortLinkByID(ctx, id)
}

func (s *Store) UpdateShortLink(ctx context.Context, id int64, in *model.ShortLink) (*model.ShortLink, error) {
	normalizeShortLink(in)
	current, err := s.GetShortLinkByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var res sql.Result
	if shortLinkConfigEqual(current, in) {
		res, err = s.db.ExecContext(ctx, `UPDATE short_links SET code=?, title=?, target_url=?, status=?, redirect_type=?, starts_at=?, expires_at=?, max_visits=?, fallback_url=?, remark=?, qr_style=?, qr_foreground=?, qr_background=?, qr_logo_url=?, updated_at=? WHERE id=?`, in.Code, in.Title, in.TargetURL, in.Status, in.RedirectType, in.StartsAt, in.ExpiresAt, in.MaxVisits, nullString(in.FallbackURL), nullString(in.Remark), in.QRStyle, in.QRForeground, in.QRBackground, nullString(in.QRLogoURL), now(), id)
	} else {
		res, err = s.db.ExecContext(ctx, `UPDATE short_links SET code=?, title=?, target_url=?, status=?, approval_status='pending', approved_at=NULL, reviewed_at=NULL, review_note=NULL, redirect_type=?, starts_at=?, expires_at=?, max_visits=?, fallback_url=?, remark=?, qr_style=?, qr_foreground=?, qr_background=?, qr_logo_url=?, updated_at=? WHERE id=?`, in.Code, in.Title, in.TargetURL, in.Status, in.RedirectType, in.StartsAt, in.ExpiresAt, in.MaxVisits, nullString(in.FallbackURL), nullString(in.Remark), in.QRStyle, in.QRForeground, in.QRBackground, nullString(in.QRLogoURL), now(), id)
	}
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetShortLinkByID(ctx, id)
}

func normalizeShortLink(in *model.ShortLink) {
	if in.RedirectType == 0 {
		in.RedirectType = 302
	}
	if in.Status == "" {
		in.Status = "active"
	}
	normalizeQRStyle(&in.QRStyle, &in.QRForeground, &in.QRBackground)
	in.QRLogoURL = strings.TrimSpace(in.QRLogoURL)
}

func (s *Store) ReviewShortLink(ctx context.Context, id int64, status, note string) (*model.ShortLink, error) {
	if err := validateReviewStatus(status); err != nil {
		return nil, err
	}
	approved := any(nil)
	if status == "approved" {
		approved = now()
	}
	res, err := s.db.ExecContext(ctx, `UPDATE short_links SET approval_status=?, approved_at=?, reviewed_at=?, review_note=?, updated_at=? WHERE id=?`, status, approved, now(), nullString(note), now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetShortLinkByID(ctx, id)
}

func (s *Store) DeleteShortLink(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM short_links WHERE id=?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetShortLinkByID(ctx context.Context, id int64) (*model.ShortLink, error) {
	row := s.db.QueryRowContext(ctx, shortSelectSQL()+" WHERE id=?", id)
	return scanShort(row)
}

func (s *Store) GetShortLinkByCode(ctx context.Context, code string) (*model.ShortLink, error) {
	row := s.db.QueryRowContext(ctx, shortSelectSQL()+" WHERE code=?", code)
	return scanShort(row)
}

func (s *Store) ListShortLinks(ctx context.Context, q string, limit, offset int) ([]model.ShortLink, error) {
	return s.ListShortLinksForAccount(ctx, q, limit, offset, 0, true)
}

func (s *Store) ListShortLinksForAccount(ctx context.Context, q string, limit, offset int, accountID int64, isAdmin bool) ([]model.ShortLink, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	base := shortSelectSQL()
	clauses := []string{}
	args := []any{}
	if !isAdmin {
		clauses = append(clauses, "owner_account_id=?")
		args = append(args, accountID)
	}
	if strings.TrimSpace(q) != "" {
		like := "%" + strings.TrimSpace(q) + "%"
		clauses = append(clauses, "(code LIKE ? OR title LIKE ? OR target_url LIKE ? OR approval_status LIKE ?)")
		args = append(args, like, like, like, like)
	}
	query := base
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ShortLink{}
	for rows.Next() {
		item, err := scanShort(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (s *Store) IncrementShortVisit(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE short_links SET visit_count=visit_count+1 WHERE id=?", id)
	return err
}

func shortSelectSQL() string {
	return `SELECT id, owner_account_id, code, title, target_url, status, approval_status, approved_at, reviewed_at, review_note, redirect_type, starts_at, expires_at, max_visits, visit_count, fallback_url, remark, qr_style, qr_foreground, qr_background, qr_logo_url, created_at, updated_at FROM short_links`
}

func scanShort(scanner interface{ Scan(dest ...any) error }) (*model.ShortLink, error) {
	var x model.ShortLink
	var starts, expires, approved, reviewed sql.NullTime
	var owner sql.NullInt64
	var fallback, remark, note, qrStyle, qrForeground, qrBackground, qrLogoURL sql.NullString
	if err := scanner.Scan(&x.ID, &owner, &x.Code, &x.Title, &x.TargetURL, &x.Status, &x.ApprovalStatus, &approved, &reviewed, &note, &x.RedirectType, &starts, &expires, &x.MaxVisits, &x.VisitCount, &fallback, &remark, &qrStyle, &qrForeground, &qrBackground, &qrLogoURL, &x.CreatedAt, &x.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if owner.Valid {
		x.OwnerAccountID = owner.Int64
	}
	if qrStyle.Valid {
		x.QRStyle = qrStyle.String
	}
	if qrForeground.Valid {
		x.QRForeground = qrForeground.String
	}
	if qrBackground.Valid {
		x.QRBackground = qrBackground.String
	}
	if qrLogoURL.Valid {
		x.QRLogoURL = strings.TrimSpace(qrLogoURL.String)
	}
	normalizeQRStyle(&x.QRStyle, &x.QRForeground, &x.QRBackground)
	if x.ApprovalStatus == "" {
		x.ApprovalStatus = "pending"
	}
	if approved.Valid {
		x.ApprovedAt = &approved.Time
	}
	if reviewed.Valid {
		x.ReviewedAt = &reviewed.Time
	}
	if note.Valid {
		x.ReviewNote = note.String
	}
	if starts.Valid {
		x.StartsAt = &starts.Time
	}
	if expires.Valid {
		x.ExpiresAt = &expires.Time
	}
	if fallback.Valid {
		x.FallbackURL = fallback.String
	}
	if remark.Valid {
		x.Remark = remark.String
	}
	return &x, nil
}

func (s *Store) CreateLiveQR(ctx context.Context, in *model.LiveQR) (*model.LiveQR, error) {
	normalizeLiveQR(in)
	res, err := s.db.ExecContext(ctx, `INSERT INTO live_qrs(owner_account_id,code,title,description,status,approval_status,rotation_strategy,guide_title,guide_text,fallback_url,qr_style,qr_foreground,qr_background,qr_logo_url) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, nullInt64(in.OwnerAccountID), in.Code, in.Title, nullString(in.Description), in.Status, "pending", in.RotationStrategy, in.GuideTitle, in.GuideText, nullString(in.FallbackURL), in.QRStyle, in.QRForeground, in.QRBackground, nullString(in.QRLogoURL))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetLiveQRByID(ctx, id)
}

func (s *Store) UpdateLiveQR(ctx context.Context, id int64, in *model.LiveQR) (*model.LiveQR, error) {
	normalizeLiveQR(in)
	res, err := s.db.ExecContext(ctx, `UPDATE live_qrs SET code=?, title=?, description=?, status=?, approval_status='pending', approved_at=NULL, reviewed_at=NULL, review_note=NULL, rotation_strategy=?, guide_title=?, guide_text=?, fallback_url=?, qr_style=?, qr_foreground=?, qr_background=?, qr_logo_url=?, updated_at=? WHERE id=?`, in.Code, in.Title, nullString(in.Description), in.Status, in.RotationStrategy, in.GuideTitle, in.GuideText, nullString(in.FallbackURL), in.QRStyle, in.QRForeground, in.QRBackground, nullString(in.QRLogoURL), now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetLiveQRByID(ctx, id)
}

func (s *Store) SaveLiveQRBundle(ctx context.Context, id int64, in *model.LiveQR, items []model.LiveQRItem, deleteItemIDs []int64) (*model.LiveQR, error) {
	normalizeLiveQR(in)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if id == 0 {
		res, err := tx.ExecContext(ctx, `INSERT INTO live_qrs(owner_account_id,code,title,description,status,approval_status,rotation_strategy,guide_title,guide_text,fallback_url,qr_style,qr_foreground,qr_background,qr_logo_url) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, nullInt64(in.OwnerAccountID), in.Code, in.Title, nullString(in.Description), in.Status, "pending", in.RotationStrategy, in.GuideTitle, in.GuideText, nullString(in.FallbackURL), in.QRStyle, in.QRForeground, in.QRBackground, nullString(in.QRLogoURL))
		if err != nil {
			return nil, err
		}
		id, _ = res.LastInsertId()
	} else {
		current, err := scanLive(tx.QueryRowContext(ctx, liveSelectSQL()+" WHERE id=?", id))
		if err != nil {
			return nil, err
		}
		var res sql.Result
		if liveQRConfigEqual(current, in) {
			res, err = tx.ExecContext(ctx, `UPDATE live_qrs SET code=?, title=?, description=?, status=?, rotation_strategy=?, guide_title=?, guide_text=?, fallback_url=?, qr_style=?, qr_foreground=?, qr_background=?, qr_logo_url=?, updated_at=? WHERE id=?`, in.Code, in.Title, nullString(in.Description), in.Status, in.RotationStrategy, in.GuideTitle, in.GuideText, nullString(in.FallbackURL), in.QRStyle, in.QRForeground, in.QRBackground, nullString(in.QRLogoURL), now(), id)
		} else {
			res, err = tx.ExecContext(ctx, `UPDATE live_qrs SET code=?, title=?, description=?, status=?, approval_status='pending', approved_at=NULL, reviewed_at=NULL, review_note=NULL, rotation_strategy=?, guide_title=?, guide_text=?, fallback_url=?, qr_style=?, qr_foreground=?, qr_background=?, qr_logo_url=?, updated_at=? WHERE id=?`, in.Code, in.Title, nullString(in.Description), in.Status, in.RotationStrategy, in.GuideTitle, in.GuideText, nullString(in.FallbackURL), in.QRStyle, in.QRForeground, in.QRBackground, nullString(in.QRLogoURL), now(), id)
		}
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil, ErrNotFound
		}
	}
	for _, itemID := range deleteItemIDs {
		if itemID <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM live_qr_items WHERE id=? AND live_qr_id=?`, itemID, id); err != nil {
			return nil, err
		}
	}
	for i := range items {
		item := items[i]
		normalizeLiveQRItem(&item)
		if item.ID > 0 {
			current, err := scanItem(tx.QueryRowContext(ctx, itemSelectSQL()+` WHERE id=? AND live_qr_id=?`, item.ID, id))
			if err != nil {
				return nil, err
			}
			if liveQRItemConfigEqual(current, &item) {
				if _, err := tx.ExecContext(ctx, `UPDATE live_qr_items SET title=?, qr_image_url=?, target_url=?, status=?, starts_at=?, expires_at=?, max_views=?, sort_order=?, weight=?, updated_at=? WHERE id=? AND live_qr_id=?`, item.Title, item.QRImageURL, nullString(item.TargetURL), item.Status, item.StartsAt, item.ExpiresAt, item.MaxViews, item.SortOrder, item.Weight, now(), item.ID, id); err != nil {
					return nil, err
				}
			} else {
				if _, err := tx.ExecContext(ctx, `UPDATE live_qr_items SET title=?, qr_image_url=?, target_url=?, status=?, approval_status='pending', approved_at=NULL, reviewed_at=NULL, review_note=NULL, starts_at=?, expires_at=?, max_views=?, sort_order=?, weight=?, updated_at=? WHERE id=? AND live_qr_id=?`, item.Title, item.QRImageURL, nullString(item.TargetURL), item.Status, item.StartsAt, item.ExpiresAt, item.MaxViews, item.SortOrder, item.Weight, now(), item.ID, id); err != nil {
					return nil, err
				}
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO live_qr_items(live_qr_id,title,qr_image_url,target_url,status,approval_status,starts_at,expires_at,max_views,sort_order,weight) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, id, item.Title, item.QRImageURL, nullString(item.TargetURL), item.Status, "pending", item.StartsAt, item.ExpiresAt, item.MaxViews, item.SortOrder, item.Weight); err != nil {
			return nil, err
		}
	}
	saved, err := scanLive(tx.QueryRowContext(ctx, liveSelectSQL()+" WHERE id=?", id))
	if err != nil {
		return nil, err
	}
	children, err := listLiveQRItemsTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	saved.Items = children
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return saved, nil
}

func normalizeLiveQR(in *model.LiveQR) {
	if in.Status == "" {
		in.Status = "active"
	}
	if in.RotationStrategy == "" {
		in.RotationStrategy = "round_robin"
	}
	if in.GuideTitle == "" {
		in.GuideTitle = "长按识别二维码"
	}
	if in.GuideText == "" {
		in.GuideText = "请长按下方二维码图片，选择“识别图中二维码”完成添加或访问。"
	}
	normalizeQRStyle(&in.QRStyle, &in.QRForeground, &in.QRBackground)
	in.QRLogoURL = strings.TrimSpace(in.QRLogoURL)
}

func (s *Store) ReviewLiveQR(ctx context.Context, id int64, status, note string, includeItems bool) (*model.LiveQR, error) {
	if err := validateReviewStatus(status); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	approved := any(nil)
	if status == "approved" {
		approved = now()
	}
	res, err := tx.ExecContext(ctx, `UPDATE live_qrs SET approval_status=?, approved_at=?, reviewed_at=?, review_note=?, updated_at=? WHERE id=?`, status, approved, now(), nullString(note), now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	if includeItems {
		if _, err := tx.ExecContext(ctx, `UPDATE live_qr_items SET approval_status=?, approved_at=?, reviewed_at=?, review_note=?, updated_at=? WHERE live_qr_id=?`, status, approved, now(), nullString(note), now(), id); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetLiveQRByID(ctx, id)
}

func (s *Store) DeleteLiveQR(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM live_qrs WHERE id=?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetLiveQRByID(ctx context.Context, id int64) (*model.LiveQR, error) {
	row := s.db.QueryRowContext(ctx, liveSelectSQL()+" WHERE id=?", id)
	return scanLive(row)
}

func (s *Store) GetLiveQRByCode(ctx context.Context, code string) (*model.LiveQR, error) {
	row := s.db.QueryRowContext(ctx, liveSelectSQL()+" WHERE code=?", code)
	return scanLive(row)
}

func (s *Store) ListLiveQRs(ctx context.Context, q string, limit, offset int) ([]model.LiveQR, error) {
	return s.ListLiveQRsForAccount(ctx, q, limit, offset, 0, true)
}

func (s *Store) ListLiveQRsForAccount(ctx context.Context, q string, limit, offset int, accountID int64, isAdmin bool) ([]model.LiveQR, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	base := liveSelectSQL()
	clauses := []string{}
	args := []any{}
	if !isAdmin {
		clauses = append(clauses, "owner_account_id=?")
		args = append(args, accountID)
	}
	if strings.TrimSpace(q) != "" {
		like := "%" + strings.TrimSpace(q) + "%"
		clauses = append(clauses, "(code LIKE ? OR title LIKE ? OR description LIKE ? OR approval_status LIKE ?)")
		args = append(args, like, like, like, like)
	}
	query := base
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.LiveQR{}
	for rows.Next() {
		item, err := scanLive(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func liveSelectSQL() string {
	return `SELECT id, owner_account_id, code, title, description, status, approval_status, approved_at, reviewed_at, review_note, rotation_strategy, current_cursor, visit_count, guide_title, guide_text, fallback_url, qr_style, qr_foreground, qr_background, qr_logo_url, created_at, updated_at FROM live_qrs`
}

func scanLive(scanner interface{ Scan(dest ...any) error }) (*model.LiveQR, error) {
	var x model.LiveQR
	var description, fallback, note, qrStyle, qrForeground, qrBackground, qrLogoURL sql.NullString
	var owner sql.NullInt64
	var approved, reviewed sql.NullTime
	if err := scanner.Scan(&x.ID, &owner, &x.Code, &x.Title, &description, &x.Status, &x.ApprovalStatus, &approved, &reviewed, &note, &x.RotationStrategy, &x.CurrentCursor, &x.VisitCount, &x.GuideTitle, &x.GuideText, &fallback, &qrStyle, &qrForeground, &qrBackground, &qrLogoURL, &x.CreatedAt, &x.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if owner.Valid {
		x.OwnerAccountID = owner.Int64
	}
	if description.Valid {
		x.Description = description.String
	}
	if fallback.Valid {
		x.FallbackURL = fallback.String
	}
	if note.Valid {
		x.ReviewNote = note.String
	}
	if approved.Valid {
		x.ApprovedAt = &approved.Time
	}
	if reviewed.Valid {
		x.ReviewedAt = &reviewed.Time
	}
	if qrStyle.Valid {
		x.QRStyle = qrStyle.String
	}
	if qrForeground.Valid {
		x.QRForeground = qrForeground.String
	}
	if qrBackground.Valid {
		x.QRBackground = qrBackground.String
	}
	if qrLogoURL.Valid {
		x.QRLogoURL = strings.TrimSpace(qrLogoURL.String)
	}
	normalizeQRStyle(&x.QRStyle, &x.QRForeground, &x.QRBackground)
	if x.ApprovalStatus == "" {
		x.ApprovalStatus = "pending"
	}
	return &x, nil
}

func (s *Store) CreateLiveQRItem(ctx context.Context, in *model.LiveQRItem) (*model.LiveQRItem, error) {
	normalizeLiveQRItem(in)
	res, err := s.db.ExecContext(ctx, `INSERT INTO live_qr_items(live_qr_id,title,qr_image_url,target_url,status,approval_status,starts_at,expires_at,max_views,sort_order,weight) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, in.LiveQRID, in.Title, in.QRImageURL, nullString(in.TargetURL), in.Status, "pending", in.StartsAt, in.ExpiresAt, in.MaxViews, in.SortOrder, in.Weight)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetLiveQRItemByID(ctx, id)
}

func (s *Store) UpdateLiveQRItem(ctx context.Context, id int64, in *model.LiveQRItem) (*model.LiveQRItem, error) {
	normalizeLiveQRItem(in)
	res, err := s.db.ExecContext(ctx, `UPDATE live_qr_items SET title=?, qr_image_url=?, target_url=?, status=?, approval_status='pending', approved_at=NULL, reviewed_at=NULL, review_note=NULL, starts_at=?, expires_at=?, max_views=?, sort_order=?, weight=?, updated_at=? WHERE id=?`, in.Title, in.QRImageURL, nullString(in.TargetURL), in.Status, in.StartsAt, in.ExpiresAt, in.MaxViews, in.SortOrder, in.Weight, now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetLiveQRItemByID(ctx, id)
}

func (s *Store) ReviewLiveQRItem(ctx context.Context, id int64, status, note string) (*model.LiveQRItem, error) {
	if err := validateReviewStatus(status); err != nil {
		return nil, err
	}
	approved := any(nil)
	if status == "approved" {
		approved = now()
	}
	res, err := s.db.ExecContext(ctx, `UPDATE live_qr_items SET approval_status=?, approved_at=?, reviewed_at=?, review_note=?, updated_at=? WHERE id=?`, status, approved, now(), nullString(note), now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetLiveQRItemByID(ctx, id)
}

func normalizeLiveQRItem(in *model.LiveQRItem) {
	if in.Status == "" {
		in.Status = "active"
	}
	if in.Weight <= 0 {
		in.Weight = 1
	}
	if in.SortOrder == 0 {
		in.SortOrder = 100
	}
}

func (s *Store) DeleteLiveQRItem(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM live_qr_items WHERE id=?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetLiveQRItemByID(ctx context.Context, id int64) (*model.LiveQRItem, error) {
	row := s.db.QueryRowContext(ctx, itemSelectSQL()+" WHERE id=?", id)
	return scanItem(row)
}

func (s *Store) ListLiveQRItems(ctx context.Context, liveID int64) ([]model.LiveQRItem, error) {
	return listLiveQRItemsTx(ctx, s.db, liveID)
}

type itemQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func listLiveQRItemsTx(ctx context.Context, q itemQueryer, liveID int64) ([]model.LiveQRItem, error) {
	rows, err := q.QueryContext(ctx, itemSelectSQL()+" WHERE live_qr_id=? ORDER BY sort_order ASC, id ASC", liveID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.LiveQRItem{}
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func itemSelectSQL() string {
	return `SELECT id, live_qr_id, title, qr_image_url, target_url, status, approval_status, approved_at, reviewed_at, review_note, starts_at, expires_at, max_views, view_count, sort_order, weight, created_at, updated_at FROM live_qr_items`
}

func scanItem(scanner interface{ Scan(dest ...any) error }) (*model.LiveQRItem, error) {
	var x model.LiveQRItem
	var starts, expires, approved, reviewed sql.NullTime
	var target, note sql.NullString
	if err := scanner.Scan(&x.ID, &x.LiveQRID, &x.Title, &x.QRImageURL, &target, &x.Status, &x.ApprovalStatus, &approved, &reviewed, &note, &starts, &expires, &x.MaxViews, &x.ViewCount, &x.SortOrder, &x.Weight, &x.CreatedAt, &x.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if target.Valid {
		x.TargetURL = target.String
	}
	if note.Valid {
		x.ReviewNote = note.String
	}
	if approved.Valid {
		x.ApprovedAt = &approved.Time
	}
	if reviewed.Valid {
		x.ReviewedAt = &reviewed.Time
	}
	if starts.Valid {
		x.StartsAt = &starts.Time
	}
	if expires.Valid {
		x.ExpiresAt = &expires.Time
	}
	if x.ApprovalStatus == "" {
		x.ApprovalStatus = "pending"
	}
	return &x, nil
}

func (s *Store) SelectLiveQRItemForVisit(ctx context.Context, liveID int64) (*model.LiveQR, *model.LiveQRItem, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	live, err := scanLive(tx.QueryRowContext(ctx, liveSelectSQL()+" WHERE id=?", liveID))
	if err != nil {
		return nil, nil, err
	}
	if live.Status != "active" || live.ApprovalStatus != "approved" {
		if err := tx.Commit(); err != nil {
			return nil, nil, err
		}
		return live, nil, nil
	}
	n := now()
	rows, err := tx.QueryContext(ctx, itemSelectSQL()+` WHERE live_qr_id=? AND status='active' AND approval_status='approved' AND (starts_at IS NULL OR starts_at <= ?) AND (expires_at IS NULL OR expires_at > ?) AND (max_views = 0 OR view_count < max_views) ORDER BY sort_order ASC, id ASC`, liveID, n, n)
	if err != nil {
		return nil, nil, err
	}
	items := []model.LiveQRItem{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			rows.Close()
			return nil, nil, err
		}
		items = append(items, *it)
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	if len(items) == 0 {
		_, _ = tx.ExecContext(ctx, "UPDATE live_qrs SET visit_count=visit_count+1 WHERE id=?", liveID)
		if err := tx.Commit(); err != nil {
			return nil, nil, err
		}
		return live, nil, nil
	}
	idx := 0
	switch live.RotationStrategy {
	case "least_used":
		min := items[0].ViewCount
		for i := range items {
			if items[i].ViewCount < min {
				min = items[i].ViewCount
				idx = i
			}
		}
	case "random":
		idx = weightedRandomIndex(items)
	default:
		idx = int(live.CurrentCursor % int64(len(items)))
		_, _ = tx.ExecContext(ctx, "UPDATE live_qrs SET current_cursor=current_cursor+1 WHERE id=?", liveID)
	}
	chosen := items[idx]
	if _, err := tx.ExecContext(ctx, "UPDATE live_qr_items SET view_count=view_count+1 WHERE id=?", chosen.ID); err != nil {
		return nil, nil, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE live_qrs SET visit_count=visit_count+1 WHERE id=?", liveID); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	chosen.ViewCount++
	live.VisitCount++
	return live, &chosen, nil
}

func weightedRandomIndex(items []model.LiveQRItem) int {
	total := 0
	for _, item := range items {
		w := item.Weight
		if w <= 0 {
			w = 1
		}
		total += w
	}
	if total <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(total)))
	if err != nil {
		return int(time.Now().UnixNano() % int64(len(items)))
	}
	threshold := int(n.Int64())
	running := 0
	for i, item := range items {
		w := item.Weight
		if w <= 0 {
			w = 1
		}
		running += w
		if threshold < running {
			return i
		}
	}
	return len(items) - 1
}

func (s *Store) RecordVisit(ctx context.Context, v *model.VisitLog) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO visit_logs(resource_type,resource_id,item_id,code,event_type,status,target_url,ip,ip_hash,user_agent,referer,device_type,browser,os) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, v.ResourceType, v.ResourceID, v.ItemID, v.Code, v.EventType, v.Status, nullString(v.TargetURL), v.IP, v.IPHash, v.UserAgent, nullString(v.Referer), v.DeviceType, v.Browser, v.OS)
	return err
}

func (s *Store) Stats(ctx context.Context, resourceType string, resourceID int64, days int) (*model.StatsBundle, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	since := beginningOfDay(time.Now().AddDate(0, 0, -days+1))
	var st model.StatsBundle
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*), COUNT(DISTINCT NULLIF(ip_hash,'')) FROM visit_logs WHERE resource_type=? AND resource_id=? AND created_at >= ?", resourceType, resourceID, since).Scan(&st.Total, &st.UniqueIPs); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT created_at FROM visit_logs WHERE resource_type=? AND resource_id=? AND created_at >= ? ORDER BY created_at ASC`, resourceType, resourceID, since)
	if err != nil {
		return nil, err
	}
	byDate := map[string]int64{}
	for rows.Next() {
		var t time.Time
		if err := rows.Scan(&t); err != nil {
			rows.Close()
			return nil, err
		}
		byDate[t.Format("2006-01-02")]++
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	for _, d := range dates {
		st.ByDate = append(st.ByDate, model.DateStat{Date: d, Count: byDate[d]})
	}
	var err2 error
	st.ByDevice, err2 = s.dimStats(ctx, resourceType, resourceID, since, "device_type")
	if err2 != nil {
		return nil, err2
	}
	st.ByBrowser, err2 = s.dimStats(ctx, resourceType, resourceID, since, "browser")
	if err2 != nil {
		return nil, err2
	}
	st.Recent, err2 = s.RecentVisits(ctx, resourceType, resourceID, 30)
	if err2 != nil {
		return nil, err2
	}
	return &st, nil
}

func (s *Store) dimStats(ctx context.Context, resourceType string, resourceID int64, since time.Time, dim string) ([]model.DimStat, error) {
	allow := map[string]bool{"device_type": true, "browser": true, "os": true, "status": true}
	if !allow[dim] {
		return nil, fmt.Errorf("bad dim")
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT COALESCE(NULLIF(%s,''),'Unknown') name, COUNT(*) c FROM visit_logs WHERE resource_type=? AND resource_id=? AND created_at >= ? GROUP BY name ORDER BY c DESC LIMIT 10`, dim), resourceType, resourceID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.DimStat{}
	for rows.Next() {
		var x model.DimStat
		if err := rows.Scan(&x.Name, &x.Count); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) RecentVisits(ctx context.Context, resourceType string, resourceID int64, limit int) ([]model.VisitLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 30
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, resource_type, resource_id, item_id, code, event_type, status, target_url, ip, ip_hash, user_agent, referer, device_type, browser, os, created_at FROM visit_logs WHERE resource_type=? AND resource_id=? ORDER BY id DESC LIMIT ?`, resourceType, resourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.VisitLog{}
	for rows.Next() {
		v, err := scanVisit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

func scanVisit(scanner interface{ Scan(dest ...any) error }) (*model.VisitLog, error) {
	var v model.VisitLog
	var itemID sql.NullInt64
	var target, referer sql.NullString
	if err := scanner.Scan(&v.ID, &v.ResourceType, &v.ResourceID, &itemID, &v.Code, &v.EventType, &v.Status, &target, &v.IP, &v.IPHash, &v.UserAgent, &referer, &v.DeviceType, &v.Browser, &v.OS, &v.CreatedAt); err != nil {
		return nil, err
	}
	if itemID.Valid {
		v.ItemID = &itemID.Int64
	}
	if target.Valid {
		v.TargetURL = target.String
	}
	if referer.Valid {
		v.Referer = referer.String
	}
	return &v, nil
}

func (s *Store) CreateMagicLoginToken(ctx context.Context, accountID int64, email, tokenHash string, expiresAt time.Time, ip string) (*model.MagicLoginToken, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO magic_login_tokens(account_id,email,token_hash,expires_at,ip) VALUES(?,?,?,?,?)`, accountID, strings.TrimSpace(email), tokenHash, expiresAt, ip)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetMagicLoginToken(ctx, id)
}

func (s *Store) GetMagicLoginToken(ctx context.Context, id int64) (*model.MagicLoginToken, error) {
	row := s.db.QueryRowContext(ctx, magicSelectSQL()+` WHERE id=?`, id)
	return scanMagic(row)
}

func (s *Store) FindMagicLoginTokenByHash(ctx context.Context, tokenHash string) (*model.MagicLoginToken, error) {
	row := s.db.QueryRowContext(ctx, magicSelectSQL()+` WHERE token_hash=? LIMIT 1`, tokenHash)
	return scanMagic(row)
}

func (s *Store) MarkMagicLoginTokenUsed(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE magic_login_tokens SET used_at=? WHERE id=? AND used_at IS NULL`, now(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func magicSelectSQL() string {
	return `SELECT id, account_id, email, token_hash, expires_at, used_at, ip, created_at FROM magic_login_tokens`
}

func scanMagic(scanner interface{ Scan(dest ...any) error }) (*model.MagicLoginToken, error) {
	var m model.MagicLoginToken
	var used sql.NullTime
	if err := scanner.Scan(&m.ID, &m.AccountID, &m.Email, &m.TokenHash, &m.ExpiresAt, &used, &m.IP, &m.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if used.Valid {
		m.UsedAt = &used.Time
	}
	return &m, nil
}

func normalizeQRStyle(style, foreground, background *string) {
	*style = strings.ToLower(strings.TrimSpace(*style))
	switch *style {
	case "rounded", "dots", "classic":
	default:
		*style = "rounded"
	}
	*foreground = normalizeHexColor(*foreground, "#111827")
	*background = normalizeHexColor(*background, "#ffffff")
}

func normalizeHexColor(v, fallback string) string {
	v = strings.TrimSpace(v)
	if len(v) == 4 && strings.HasPrefix(v, "#") {
		return "#" + strings.Repeat(v[1:2], 2) + strings.Repeat(v[2:3], 2) + strings.Repeat(v[3:4], 2)
	}
	if len(v) != 7 || !strings.HasPrefix(v, "#") {
		return fallback
	}
	for _, r := range v[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return fallback
		}
	}
	return strings.ToLower(v)
}

func validateReviewStatus(status string) error {
	if status != "approved" && status != "rejected" && status != "pending" {
		return fmt.Errorf("审核状态只支持 pending/approved/rejected")
	}
	return nil
}

func shortLinkConfigEqual(a *model.ShortLink, b *model.ShortLink) bool {
	return a.Code == b.Code && a.Title == b.Title && a.TargetURL == b.TargetURL && a.Status == b.Status && a.RedirectType == b.RedirectType && sameTimePtr(a.StartsAt, b.StartsAt) && sameTimePtr(a.ExpiresAt, b.ExpiresAt) && a.MaxVisits == b.MaxVisits && a.FallbackURL == b.FallbackURL && a.Remark == b.Remark && a.QRStyle == b.QRStyle && a.QRForeground == b.QRForeground && a.QRBackground == b.QRBackground && a.QRLogoURL == b.QRLogoURL
}

func liveQRConfigEqual(a *model.LiveQR, b *model.LiveQR) bool {
	return a.Code == b.Code && a.Title == b.Title && a.Description == b.Description && a.Status == b.Status && a.RotationStrategy == b.RotationStrategy && a.GuideTitle == b.GuideTitle && a.GuideText == b.GuideText && a.FallbackURL == b.FallbackURL && a.QRStyle == b.QRStyle && a.QRForeground == b.QRForeground && a.QRBackground == b.QRBackground && a.QRLogoURL == b.QRLogoURL
}

func liveQRItemConfigEqual(a *model.LiveQRItem, b *model.LiveQRItem) bool {
	return a.Title == b.Title && a.QRImageURL == b.QRImageURL && a.TargetURL == b.TargetURL && a.Status == b.Status && sameTimePtr(a.StartsAt, b.StartsAt) && sameTimePtr(a.ExpiresAt, b.ExpiresAt) && a.MaxViews == b.MaxViews && a.SortOrder == b.SortOrder && a.Weight == b.Weight
}

func sameTimePtr(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Truncate(time.Second).Equal(b.Truncate(time.Second))
}

func now() time.Time { return time.Now().Truncate(time.Second) }
func beginningOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func settingsBool(m map[string]string, k string) bool {
	v := strings.ToLower(strings.TrimSpace(m[k]))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func nullString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullInt64(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}
