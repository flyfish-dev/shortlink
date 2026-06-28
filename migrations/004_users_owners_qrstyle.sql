ALTER TABLE admin_accounts ADD COLUMN role TEXT NOT NULL DEFAULT 'admin';
ALTER TABLE admin_accounts ADD COLUMN status TEXT NOT NULL DEFAULT 'active';
CREATE INDEX IF NOT EXISTS idx_admin_accounts_role_status ON admin_accounts(role, status);

ALTER TABLE short_links ADD COLUMN owner_account_id INTEGER NULL;
ALTER TABLE short_links ADD COLUMN qr_style TEXT NOT NULL DEFAULT 'rounded';
ALTER TABLE short_links ADD COLUMN qr_foreground TEXT NOT NULL DEFAULT '#111827';
ALTER TABLE short_links ADD COLUMN qr_background TEXT NOT NULL DEFAULT '#ffffff';
CREATE INDEX IF NOT EXISTS idx_short_links_owner_created ON short_links(owner_account_id, created_at);

ALTER TABLE live_qrs ADD COLUMN owner_account_id INTEGER NULL;
ALTER TABLE live_qrs ADD COLUMN qr_style TEXT NOT NULL DEFAULT 'rounded';
ALTER TABLE live_qrs ADD COLUMN qr_foreground TEXT NOT NULL DEFAULT '#111827';
ALTER TABLE live_qrs ADD COLUMN qr_background TEXT NOT NULL DEFAULT '#ffffff';
CREATE INDEX IF NOT EXISTS idx_live_qrs_owner_created ON live_qrs(owner_account_id, created_at);
