CREATE TABLE IF NOT EXISTS tenants (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'organization',
  status TEXT NOT NULL DEFAULT 'active',
  owner_account_id INTEGER NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(owner_account_id) REFERENCES admin_accounts(id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_tenants_slug ON tenants(slug);
CREATE INDEX IF NOT EXISTS idx_tenants_owner ON tenants(owner_account_id);

CREATE TABLE IF NOT EXISTS tenant_members (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  account_id INTEGER NOT NULL,
  role TEXT NOT NULL DEFAULT 'member',
  status TEXT NOT NULL DEFAULT 'active',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
  FOREIGN KEY(account_id) REFERENCES admin_accounts(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_members_account ON tenant_members(tenant_id, account_id);
CREATE INDEX IF NOT EXISTS idx_tenant_members_account_status ON tenant_members(account_id, status);

CREATE TABLE IF NOT EXISTS plans (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  price_monthly_cents INTEGER NOT NULL DEFAULT 0,
  currency TEXT NOT NULL DEFAULT 'CNY',
  max_members INTEGER NOT NULL DEFAULT 0,
  max_short_links INTEGER NOT NULL DEFAULT 0,
  max_live_qrs INTEGER NOT NULL DEFAULT 0,
  max_targets_per_link INTEGER NOT NULL DEFAULT 0,
  monthly_visits INTEGER NOT NULL DEFAULT 0,
  features_json TEXT NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'active',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_plans_code ON plans(code);

CREATE TABLE IF NOT EXISTS tenant_subscriptions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  plan_id INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  provider TEXT NOT NULL DEFAULT 'manual',
  external_customer_id TEXT NULL,
  external_subscription_id TEXT NULL,
  current_period_start DATETIME NULL,
  current_period_end DATETIME NULL,
  cancel_at_period_end INTEGER NOT NULL DEFAULT 0,
  trial_ends_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
  FOREIGN KEY(plan_id) REFERENCES plans(id)
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_subscriptions_tenant ON tenant_subscriptions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_subscriptions_status ON tenant_subscriptions(status);

CREATE TABLE IF NOT EXISTS subscription_change_requests (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  from_plan_id INTEGER NOT NULL,
  to_plan_id INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  pending_guard TEXT NULL,
  note TEXT NOT NULL DEFAULT '',
  review_note TEXT NOT NULL DEFAULT '',
  requested_by INTEGER NOT NULL,
  reviewed_by INTEGER NULL,
  reviewed_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
  FOREIGN KEY(from_plan_id) REFERENCES plans(id),
  FOREIGN KEY(to_plan_id) REFERENCES plans(id),
  FOREIGN KEY(requested_by) REFERENCES admin_accounts(id),
  FOREIGN KEY(reviewed_by) REFERENCES admin_accounts(id)
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_subscription_request_pending ON subscription_change_requests(tenant_id, pending_guard);
CREATE INDEX IF NOT EXISTS idx_subscription_requests_status ON subscription_change_requests(status, created_at);

CREATE TABLE IF NOT EXISTS tenant_usage_monthly (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  period_key TEXT NOT NULL,
  visits INTEGER NOT NULL DEFAULT 0,
  short_links_created INTEGER NOT NULL DEFAULT 0,
  live_qrs_created INTEGER NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_usage_period ON tenant_usage_monthly(tenant_id, period_key);

CREATE TABLE IF NOT EXISTS approval_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  resource_type TEXT NOT NULL,
  resource_id INTEGER NOT NULL,
  content_version INTEGER NOT NULL,
  stage TEXT NOT NULL,
  action TEXT NOT NULL,
  actor_account_id INTEGER NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
  FOREIGN KEY(actor_account_id) REFERENCES admin_accounts(id)
);
CREATE INDEX IF NOT EXISTS idx_approval_events_resource ON approval_events(resource_type, resource_id, content_version, created_at);
CREATE INDEX IF NOT EXISTS idx_approval_events_tenant ON approval_events(tenant_id, created_at);

CREATE TABLE IF NOT EXISTS short_link_targets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  short_link_id INTEGER NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  target_url TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  weight INTEGER NOT NULL DEFAULT 1,
  sort_order INTEGER NOT NULL DEFAULT 100,
  starts_at DATETIME NULL,
  expires_at DATETIME NULL,
  max_hits INTEGER NOT NULL DEFAULT 0,
  hit_count INTEGER NOT NULL DEFAULT 0,
  health_status TEXT NOT NULL DEFAULT 'unknown',
  last_hit_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
  FOREIGN KEY(short_link_id) REFERENCES short_links(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_short_targets_select ON short_link_targets(short_link_id, status, health_status, sort_order, id);
CREATE INDEX IF NOT EXISTS idx_short_targets_tenant ON short_link_targets(tenant_id, short_link_id);

ALTER TABLE short_links ADD COLUMN tenant_id INTEGER NULL;
ALTER TABLE short_links ADD COLUMN routing_strategy TEXT NOT NULL DEFAULT 'single';
ALTER TABLE short_links ADD COLUMN current_target_cursor INTEGER NOT NULL DEFAULT 0;
ALTER TABLE short_links ADD COLUMN content_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE short_links ADD COLUMN approved_version INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_short_links_tenant_created ON short_links(tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_short_links_tenant_approval ON short_links(tenant_id, approval_status, updated_at);

ALTER TABLE live_qrs ADD COLUMN tenant_id INTEGER NULL;
ALTER TABLE live_qrs ADD COLUMN content_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE live_qrs ADD COLUMN approved_version INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_live_qrs_tenant_created ON live_qrs(tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_live_qrs_tenant_approval ON live_qrs(tenant_id, approval_status, updated_at);

ALTER TABLE live_qr_items ADD COLUMN content_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE live_qr_items ADD COLUMN approved_version INTEGER NOT NULL DEFAULT 0;

ALTER TABLE visit_logs ADD COLUMN tenant_id INTEGER NULL;
ALTER TABLE visit_logs ADD COLUMN target_id INTEGER NULL;
CREATE INDEX IF NOT EXISTS idx_visit_tenant_time ON visit_logs(tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_visit_target_time ON visit_logs(target_id, created_at);

ALTER TABLE audit_logs ADD COLUMN tenant_id INTEGER NULL;
CREATE INDEX IF NOT EXISTS idx_audit_tenant_time ON audit_logs(tenant_id, created_at);

INSERT OR IGNORE INTO plans(code,name,description,price_monthly_cents,currency,max_members,max_short_links,max_live_qrs,max_targets_per_link,monthly_visits,features_json,status)
VALUES('free','Free','适合个人和试用',0,'CNY',3,100,20,3,10000,'{"analytics":true,"approval":true}','active');
INSERT OR IGNORE INTO plans(code,name,description,price_monthly_cents,currency,max_members,max_short_links,max_live_qrs,max_targets_per_link,monthly_visits,features_json,status)
VALUES('pro','Pro','适合小团队运营',4900,'CNY',10,2000,300,20,500000,'{"analytics":true,"approval":true,"routing":true}','active');
INSERT OR IGNORE INTO plans(code,name,description,price_monthly_cents,currency,max_members,max_short_links,max_live_qrs,max_targets_per_link,monthly_visits,features_json,status)
VALUES('business','Business','适合多成员业务团队',19900,'CNY',50,20000,3000,100,5000000,'{"analytics":true,"approval":true,"routing":true,"audit":true}','active');
INSERT OR IGNORE INTO plans(code,name,description,price_monthly_cents,currency,max_members,max_short_links,max_live_qrs,max_targets_per_link,monthly_visits,features_json,status)
VALUES('enterprise','Enterprise','私有化与定制额度',0,'CNY',0,0,0,0,0,'{"analytics":true,"approval":true,"routing":true,"audit":true,"custom":true}','active');

INSERT OR IGNORE INTO tenants(slug,name,kind,status,owner_account_id)
SELECT 'personal-' || id, CASE WHEN TRIM(name)='' THEN COALESCE(email,'Personal Workspace') ELSE name || ' Workspace' END, 'personal', 'active', id
FROM admin_accounts;
INSERT OR IGNORE INTO tenant_members(tenant_id,account_id,role,status)
SELECT t.id,t.owner_account_id,'owner','active' FROM tenants t WHERE t.kind='personal' AND t.owner_account_id IS NOT NULL;
INSERT OR IGNORE INTO tenants(slug,name,kind,status,owner_account_id)
SELECT 'platform-default','Platform Workspace','organization','active',id FROM admin_accounts ORDER BY CASE WHEN role='admin' THEN 0 ELSE 1 END,id LIMIT 1;
INSERT OR IGNORE INTO tenant_members(tenant_id,account_id,role,status)
SELECT t.id,t.owner_account_id,'owner','active' FROM tenants t WHERE t.slug='platform-default' AND t.owner_account_id IS NOT NULL;
INSERT OR IGNORE INTO tenant_subscriptions(tenant_id,plan_id,status,provider,current_period_start)
SELECT t.id,p.id,'active','manual',CURRENT_TIMESTAMP FROM tenants t JOIN plans p ON p.code='free';

UPDATE short_links SET tenant_id=(SELECT t.id FROM tenants t WHERE t.owner_account_id=short_links.owner_account_id AND t.kind='personal' LIMIT 1) WHERE tenant_id IS NULL AND owner_account_id IS NOT NULL;
UPDATE short_links SET tenant_id=(SELECT id FROM tenants WHERE slug='platform-default' LIMIT 1) WHERE tenant_id IS NULL;
UPDATE live_qrs SET tenant_id=(SELECT t.id FROM tenants t WHERE t.owner_account_id=live_qrs.owner_account_id AND t.kind='personal' LIMIT 1) WHERE tenant_id IS NULL AND owner_account_id IS NOT NULL;
UPDATE live_qrs SET tenant_id=(SELECT id FROM tenants WHERE slug='platform-default' LIMIT 1) WHERE tenant_id IS NULL;
UPDATE short_links SET approved_version=CASE WHEN approval_status='approved' THEN content_version ELSE 0 END,
  approval_status=CASE WHEN approval_status='pending' THEN 'tenant_pending' WHEN approval_status='rejected' THEN 'tenant_rejected' ELSE approval_status END;
UPDATE live_qrs SET approved_version=CASE WHEN approval_status='approved' THEN content_version ELSE 0 END,
  approval_status=CASE WHEN approval_status='pending' THEN 'tenant_pending' WHEN approval_status='rejected' THEN 'tenant_rejected' ELSE approval_status END;
UPDATE live_qr_items SET approved_version=CASE WHEN approval_status='approved' THEN content_version ELSE 0 END,
  approval_status=CASE WHEN approval_status='pending' THEN 'tenant_pending' WHEN approval_status='rejected' THEN 'tenant_rejected' ELSE approval_status END;
