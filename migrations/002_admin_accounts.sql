CREATE TABLE IF NOT EXISTS admin_accounts (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  recovery_key_hash CHAR(64) NOT NULL,
  recovery_key_cipher TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_admin_accounts_recovery_key_hash (recovery_key_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE admin_devices ADD COLUMN IF NOT EXISTS account_id BIGINT UNSIGNED NULL AFTER id;

CREATE INDEX IF NOT EXISTS idx_admin_devices_account ON admin_devices(account_id);
