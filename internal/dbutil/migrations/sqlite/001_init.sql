CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS admin_accounts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT NULL,
  name TEXT NOT NULL DEFAULT '',
  recovery_key_hash TEXT NOT NULL,
  recovery_key_cipher TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_admin_accounts_recovery_key_hash ON admin_accounts(recovery_key_hash);
CREATE UNIQUE INDEX IF NOT EXISTS uk_admin_accounts_email ON admin_accounts(email);

CREATE TABLE IF NOT EXISTS admin_devices (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account_id INTEGER NULL,
  label TEXT NOT NULL DEFAULT '',
  browser_hash TEXT NOT NULL,
  ip_hash TEXT NOT NULL,
  ip_last TEXT NOT NULL DEFAULT '',
  user_agent_last TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen_at DATETIME NULL,
  revoked_at DATETIME NULL,
  FOREIGN KEY(account_id) REFERENCES admin_accounts(id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_admin_device_browser ON admin_devices(browser_hash);
CREATE INDEX IF NOT EXISTS idx_admin_devices_account ON admin_devices(account_id);
CREATE INDEX IF NOT EXISTS idx_admin_devices_revoked ON admin_devices(revoked_at);

CREATE TABLE IF NOT EXISTS system_settings (
  setting_key TEXT PRIMARY KEY,
  setting_value TEXT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS magic_login_tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account_id INTEGER NOT NULL,
  email TEXT NOT NULL,
  token_hash TEXT NOT NULL,
  expires_at DATETIME NOT NULL,
  used_at DATETIME NULL,
  ip TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(account_id) REFERENCES admin_accounts(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_magic_login_token_hash ON magic_login_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_magic_login_account_created ON magic_login_tokens(account_id, created_at);
CREATE INDEX IF NOT EXISTS idx_magic_login_expires ON magic_login_tokens(expires_at);

CREATE TABLE IF NOT EXISTS short_links (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  target_url TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  approval_status TEXT NOT NULL DEFAULT 'pending',
  approved_at DATETIME NULL,
  reviewed_at DATETIME NULL,
  review_note TEXT NULL,
  redirect_type INTEGER NOT NULL DEFAULT 302,
  starts_at DATETIME NULL,
  expires_at DATETIME NULL,
  max_visits INTEGER NOT NULL DEFAULT 0,
  visit_count INTEGER NOT NULL DEFAULT 0,
  fallback_url TEXT NULL,
  remark TEXT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_short_links_code ON short_links(code);
CREATE INDEX IF NOT EXISTS idx_short_links_status ON short_links(status);
CREATE INDEX IF NOT EXISTS idx_short_links_approval ON short_links(approval_status);
CREATE INDEX IF NOT EXISTS idx_short_links_created ON short_links(created_at);

CREATE TABLE IF NOT EXISTS live_qrs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  description TEXT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  approval_status TEXT NOT NULL DEFAULT 'pending',
  approved_at DATETIME NULL,
  reviewed_at DATETIME NULL,
  review_note TEXT NULL,
  rotation_strategy TEXT NOT NULL DEFAULT 'round_robin',
  current_cursor INTEGER NOT NULL DEFAULT 0,
  visit_count INTEGER NOT NULL DEFAULT 0,
  guide_title TEXT NOT NULL DEFAULT '长按识别二维码',
  guide_text TEXT NOT NULL DEFAULT '请长按下方二维码图片，选择“识别图中二维码”完成添加或访问。',
  fallback_url TEXT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_live_qrs_code ON live_qrs(code);
CREATE INDEX IF NOT EXISTS idx_live_qrs_status ON live_qrs(status);
CREATE INDEX IF NOT EXISTS idx_live_qrs_approval ON live_qrs(approval_status);
CREATE INDEX IF NOT EXISTS idx_live_qrs_created ON live_qrs(created_at);

CREATE TABLE IF NOT EXISTS live_qr_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  live_qr_id INTEGER NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  qr_image_url TEXT NOT NULL,
  target_url TEXT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  approval_status TEXT NOT NULL DEFAULT 'pending',
  approved_at DATETIME NULL,
  reviewed_at DATETIME NULL,
  review_note TEXT NULL,
  starts_at DATETIME NULL,
  expires_at DATETIME NULL,
  max_views INTEGER NOT NULL DEFAULT 0,
  view_count INTEGER NOT NULL DEFAULT 0,
  sort_order INTEGER NOT NULL DEFAULT 100,
  weight INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(live_qr_id) REFERENCES live_qrs(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_live_qr_items_parent ON live_qr_items(live_qr_id, status, approval_status, sort_order);
CREATE INDEX IF NOT EXISTS idx_live_qr_items_expires ON live_qr_items(expires_at);
CREATE INDEX IF NOT EXISTS idx_live_qr_items_views ON live_qr_items(view_count);
CREATE INDEX IF NOT EXISTS idx_live_qr_items_approval ON live_qr_items(approval_status);

CREATE TABLE IF NOT EXISTS visit_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  resource_type TEXT NOT NULL,
  resource_id INTEGER NOT NULL,
  item_id INTEGER NULL,
  code TEXT NOT NULL DEFAULT '',
  event_type TEXT NOT NULL DEFAULT 'visit',
  status TEXT NOT NULL DEFAULT 'ok',
  target_url TEXT NULL,
  ip TEXT NOT NULL DEFAULT '',
  ip_hash TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT '',
  referer TEXT NULL,
  device_type TEXT NOT NULL DEFAULT '',
  browser TEXT NOT NULL DEFAULT '',
  os TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_visit_resource_time ON visit_logs(resource_type, resource_id, created_at);
CREATE INDEX IF NOT EXISTS idx_visit_item_time ON visit_logs(item_id, created_at);
CREATE INDEX IF NOT EXISTS idx_visit_code_time ON visit_logs(code, created_at);
CREATE INDEX IF NOT EXISTS idx_visit_created ON visit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_visit_ip_hash ON visit_logs(ip_hash);

CREATE TABLE IF NOT EXISTS audit_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  actor_device_id INTEGER NULL,
  action TEXT NOT NULL,
  resource_type TEXT NOT NULL DEFAULT '',
  resource_id INTEGER NULL,
  detail TEXT NULL,
  ip TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_logs(resource_type, resource_id);
