CREATE TABLE IF NOT EXISTS system_settings (
  setting_key VARCHAR(120) NOT NULL PRIMARY KEY,
  setting_value TEXT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS magic_login_tokens (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  account_id BIGINT UNSIGNED NOT NULL,
  email VARCHAR(320) NOT NULL,
  token_hash CHAR(64) NOT NULL,
  expires_at DATETIME NOT NULL,
  used_at DATETIME NULL,
  ip VARCHAR(45) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_magic_login_token_hash (token_hash),
  KEY idx_magic_login_account_created (account_id, created_at),
  KEY idx_magic_login_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE admin_accounts ADD COLUMN IF NOT EXISTS email VARCHAR(320) NULL AFTER id;
ALTER TABLE admin_accounts ADD COLUMN IF NOT EXISTS name VARCHAR(120) NOT NULL DEFAULT '' AFTER email;
CREATE UNIQUE INDEX uk_admin_accounts_email ON admin_accounts(email);

ALTER TABLE short_links ADD COLUMN IF NOT EXISTS approval_status ENUM('pending','approved','rejected') NOT NULL DEFAULT 'pending' AFTER status;
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS approved_at DATETIME NULL AFTER approval_status;
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS reviewed_at DATETIME NULL AFTER approved_at;
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS review_note TEXT NULL AFTER reviewed_at;
UPDATE short_links SET approval_status='approved', approved_at=COALESCE(approved_at,NOW()), reviewed_at=COALESCE(reviewed_at,NOW()) WHERE approval_status='pending';
CREATE INDEX idx_short_links_approval ON short_links(approval_status);

ALTER TABLE live_qrs ADD COLUMN IF NOT EXISTS approval_status ENUM('pending','approved','rejected') NOT NULL DEFAULT 'pending' AFTER status;
ALTER TABLE live_qrs ADD COLUMN IF NOT EXISTS approved_at DATETIME NULL AFTER approval_status;
ALTER TABLE live_qrs ADD COLUMN IF NOT EXISTS reviewed_at DATETIME NULL AFTER approved_at;
ALTER TABLE live_qrs ADD COLUMN IF NOT EXISTS review_note TEXT NULL AFTER reviewed_at;
UPDATE live_qrs SET approval_status='approved', approved_at=COALESCE(approved_at,NOW()), reviewed_at=COALESCE(reviewed_at,NOW()) WHERE approval_status='pending';
CREATE INDEX idx_live_qrs_approval ON live_qrs(approval_status);

ALTER TABLE live_qr_items ADD COLUMN IF NOT EXISTS approval_status ENUM('pending','approved','rejected') NOT NULL DEFAULT 'pending' AFTER status;
ALTER TABLE live_qr_items ADD COLUMN IF NOT EXISTS approved_at DATETIME NULL AFTER approval_status;
ALTER TABLE live_qr_items ADD COLUMN IF NOT EXISTS reviewed_at DATETIME NULL AFTER approved_at;
ALTER TABLE live_qr_items ADD COLUMN IF NOT EXISTS review_note TEXT NULL AFTER reviewed_at;
UPDATE live_qr_items SET approval_status='approved', approved_at=COALESCE(approved_at,NOW()), reviewed_at=COALESCE(reviewed_at,NOW()) WHERE approval_status='pending';
CREATE INDEX idx_live_qr_items_approval ON live_qr_items(approval_status);
