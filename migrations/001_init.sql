CREATE TABLE IF NOT EXISTS schema_migrations (
  version VARCHAR(64) PRIMARY KEY,
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS admin_devices (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  label VARCHAR(120) NOT NULL DEFAULT '',
  browser_hash CHAR(64) NOT NULL,
  ip_hash CHAR(64) NOT NULL,
  ip_last VARCHAR(45) NOT NULL DEFAULT '',
  user_agent_last VARCHAR(512) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen_at DATETIME NULL,
  revoked_at DATETIME NULL,
  UNIQUE KEY uk_admin_device_browser_ip (browser_hash, ip_hash),
  KEY idx_admin_devices_revoked (revoked_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS short_links (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  code VARCHAR(64) NOT NULL,
  title VARCHAR(200) NOT NULL DEFAULT '',
  target_url TEXT NOT NULL,
  status ENUM('active','disabled') NOT NULL DEFAULT 'active',
  redirect_type SMALLINT NOT NULL DEFAULT 302,
  starts_at DATETIME NULL,
  expires_at DATETIME NULL,
  max_visits BIGINT UNSIGNED NOT NULL DEFAULT 0,
  visit_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
  fallback_url TEXT NULL,
  remark TEXT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_short_links_code (code),
  KEY idx_short_links_status (status),
  KEY idx_short_links_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS live_qrs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  code VARCHAR(64) NOT NULL,
  title VARCHAR(200) NOT NULL DEFAULT '',
  description TEXT NULL,
  status ENUM('active','disabled') NOT NULL DEFAULT 'active',
  rotation_strategy ENUM('round_robin','random','least_used') NOT NULL DEFAULT 'round_robin',
  current_cursor BIGINT UNSIGNED NOT NULL DEFAULT 0,
  visit_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
  guide_title VARCHAR(200) NOT NULL DEFAULT '长按识别二维码',
  guide_text VARCHAR(500) NOT NULL DEFAULT '请长按下方二维码图片，选择“识别图中二维码”完成添加或访问。',
  fallback_url TEXT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_live_qrs_code (code),
  KEY idx_live_qrs_status (status),
  KEY idx_live_qrs_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS live_qr_items (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  live_qr_id BIGINT UNSIGNED NOT NULL,
  title VARCHAR(200) NOT NULL DEFAULT '',
  qr_image_url TEXT NOT NULL,
  target_url TEXT NULL,
  status ENUM('active','disabled') NOT NULL DEFAULT 'active',
  starts_at DATETIME NULL,
  expires_at DATETIME NULL,
  max_views BIGINT UNSIGNED NOT NULL DEFAULT 0,
  view_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
  sort_order INT NOT NULL DEFAULT 100,
  weight INT NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_live_qr_items_live_qr FOREIGN KEY (live_qr_id) REFERENCES live_qrs(id) ON DELETE CASCADE,
  KEY idx_live_qr_items_parent (live_qr_id, status, sort_order),
  KEY idx_live_qr_items_expires (expires_at),
  KEY idx_live_qr_items_views (view_count)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS visit_logs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  resource_type ENUM('short_link','live_qr') NOT NULL,
  resource_id BIGINT UNSIGNED NOT NULL,
  item_id BIGINT UNSIGNED NULL,
  code VARCHAR(64) NOT NULL DEFAULT '',
  event_type VARCHAR(40) NOT NULL DEFAULT 'visit',
  status VARCHAR(40) NOT NULL DEFAULT 'ok',
  target_url TEXT NULL,
  ip VARCHAR(45) NOT NULL DEFAULT '',
  ip_hash CHAR(64) NOT NULL DEFAULT '',
  user_agent VARCHAR(1024) NOT NULL DEFAULT '',
  referer TEXT NULL,
  device_type VARCHAR(40) NOT NULL DEFAULT '',
  browser VARCHAR(80) NOT NULL DEFAULT '',
  os VARCHAR(80) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_visit_resource_time (resource_type, resource_id, created_at),
  KEY idx_visit_item_time (item_id, created_at),
  KEY idx_visit_code_time (code, created_at),
  KEY idx_visit_created (created_at),
  KEY idx_visit_ip_hash (ip_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  actor_device_id BIGINT UNSIGNED NULL,
  action VARCHAR(80) NOT NULL,
  resource_type VARCHAR(80) NOT NULL DEFAULT '',
  resource_id BIGINT UNSIGNED NULL,
  detail TEXT NULL,
  ip VARCHAR(45) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_audit_created (created_at),
  KEY idx_audit_resource (resource_type, resource_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
