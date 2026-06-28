ALTER TABLE admin_accounts ADD COLUMN IF NOT EXISTS role ENUM('admin','user') NOT NULL DEFAULT 'admin' AFTER name;
ALTER TABLE admin_accounts ADD COLUMN IF NOT EXISTS status ENUM('active','disabled') NOT NULL DEFAULT 'active' AFTER role;
UPDATE admin_accounts SET role='admin' WHERE role IS NULL OR role='';
CREATE INDEX IF NOT EXISTS idx_admin_accounts_role_status ON admin_accounts(role, status);

ALTER TABLE short_links ADD COLUMN IF NOT EXISTS owner_account_id BIGINT UNSIGNED NULL AFTER id;
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS qr_style VARCHAR(32) NOT NULL DEFAULT 'rounded' AFTER remark;
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS qr_foreground VARCHAR(16) NOT NULL DEFAULT '#111827' AFTER qr_style;
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS qr_background VARCHAR(16) NOT NULL DEFAULT '#ffffff' AFTER qr_foreground;
CREATE INDEX IF NOT EXISTS idx_short_links_owner_created ON short_links(owner_account_id, created_at);

ALTER TABLE live_qrs ADD COLUMN IF NOT EXISTS owner_account_id BIGINT UNSIGNED NULL AFTER id;
ALTER TABLE live_qrs ADD COLUMN IF NOT EXISTS qr_style VARCHAR(32) NOT NULL DEFAULT 'rounded' AFTER fallback_url;
ALTER TABLE live_qrs ADD COLUMN IF NOT EXISTS qr_foreground VARCHAR(16) NOT NULL DEFAULT '#111827' AFTER qr_style;
ALTER TABLE live_qrs ADD COLUMN IF NOT EXISTS qr_background VARCHAR(16) NOT NULL DEFAULT '#ffffff' AFTER qr_foreground;
CREATE INDEX IF NOT EXISTS idx_live_qrs_owner_created ON live_qrs(owner_account_id, created_at);
