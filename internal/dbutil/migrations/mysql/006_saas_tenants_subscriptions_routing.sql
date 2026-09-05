CREATE TABLE IF NOT EXISTS tenants (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  slug VARCHAR(128) NOT NULL,
  name VARCHAR(255) NOT NULL,
  kind VARCHAR(32) NOT NULL DEFAULT 'organization',
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  owner_account_id BIGINT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_tenants_slug(slug),
  KEY idx_tenants_owner(owner_account_id),
  CONSTRAINT fk_tenants_owner FOREIGN KEY(owner_account_id) REFERENCES admin_accounts(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tenant_members (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  account_id BIGINT NOT NULL,
  role VARCHAR(32) NOT NULL DEFAULT 'member',
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_tenant_members_account(tenant_id,account_id),
  KEY idx_tenant_members_account_status(account_id,status),
  CONSTRAINT fk_tenant_members_tenant FOREIGN KEY(tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
  CONSTRAINT fk_tenant_members_account FOREIGN KEY(account_id) REFERENCES admin_accounts(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS plans (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  description TEXT NOT NULL,
  price_monthly_cents BIGINT NOT NULL DEFAULT 0,
  currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
  max_members BIGINT NOT NULL DEFAULT 0,
  max_short_links BIGINT NOT NULL DEFAULT 0,
  max_live_qrs BIGINT NOT NULL DEFAULT 0,
  max_targets_per_link BIGINT NOT NULL DEFAULT 0,
  monthly_visits BIGINT NOT NULL DEFAULT 0,
  features_json LONGTEXT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_plans_code(code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tenant_subscriptions (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  plan_id BIGINT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  provider VARCHAR(64) NOT NULL DEFAULT 'manual',
  external_customer_id VARCHAR(255) NULL,
  external_subscription_id VARCHAR(255) NULL,
  current_period_start DATETIME NULL,
  current_period_end DATETIME NULL,
  cancel_at_period_end TINYINT(1) NOT NULL DEFAULT 0,
  trial_ends_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_tenant_subscriptions_tenant(tenant_id),
  KEY idx_tenant_subscriptions_status(status),
  CONSTRAINT fk_subscriptions_tenant FOREIGN KEY(tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
  CONSTRAINT fk_subscriptions_plan FOREIGN KEY(plan_id) REFERENCES plans(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS subscription_change_requests (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  from_plan_id BIGINT NOT NULL,
  to_plan_id BIGINT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  pending_guard VARCHAR(32) NULL,
  note TEXT NOT NULL,
  review_note TEXT NOT NULL,
  requested_by BIGINT NOT NULL,
  reviewed_by BIGINT NULL,
  reviewed_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_subscription_request_pending(tenant_id,pending_guard),
  KEY idx_subscription_requests_status(status,created_at),
  CONSTRAINT fk_subscription_requests_tenant FOREIGN KEY(tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
  CONSTRAINT fk_subscription_requests_from_plan FOREIGN KEY(from_plan_id) REFERENCES plans(id),
  CONSTRAINT fk_subscription_requests_to_plan FOREIGN KEY(to_plan_id) REFERENCES plans(id),
  CONSTRAINT fk_subscription_requests_requested_by FOREIGN KEY(requested_by) REFERENCES admin_accounts(id),
  CONSTRAINT fk_subscription_requests_reviewed_by FOREIGN KEY(reviewed_by) REFERENCES admin_accounts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tenant_usage_monthly (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  period_key VARCHAR(7) NOT NULL,
  visits BIGINT NOT NULL DEFAULT 0,
  short_links_created BIGINT NOT NULL DEFAULT 0,
  live_qrs_created BIGINT NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_tenant_usage_period(tenant_id,period_key),
  CONSTRAINT fk_tenant_usage_tenant FOREIGN KEY(tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS approval_events (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  resource_type VARCHAR(32) NOT NULL,
  resource_id BIGINT NOT NULL,
  content_version BIGINT NOT NULL,
  stage VARCHAR(32) NOT NULL,
  action VARCHAR(32) NOT NULL,
  actor_account_id BIGINT NOT NULL,
  note TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_approval_events_resource(resource_type,resource_id,content_version,created_at),
  KEY idx_approval_events_tenant(tenant_id,created_at),
  CONSTRAINT fk_approval_events_tenant FOREIGN KEY(tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
  CONSTRAINT fk_approval_events_actor FOREIGN KEY(actor_account_id) REFERENCES admin_accounts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS short_link_targets (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  short_link_id BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL DEFAULT '',
  target_url TEXT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  weight INT NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 100,
  starts_at DATETIME NULL,
  expires_at DATETIME NULL,
  max_hits BIGINT NOT NULL DEFAULT 0,
  hit_count BIGINT NOT NULL DEFAULT 0,
  health_status VARCHAR(32) NOT NULL DEFAULT 'unknown',
  last_hit_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_short_targets_select(short_link_id,status,health_status,sort_order,id),
  KEY idx_short_targets_tenant(tenant_id,short_link_id),
  CONSTRAINT fk_short_targets_tenant FOREIGN KEY(tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
  CONSTRAINT fk_short_targets_link FOREIGN KEY(short_link_id) REFERENCES short_links(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE short_links
  ADD COLUMN tenant_id BIGINT NULL,
  ADD COLUMN routing_strategy VARCHAR(32) NOT NULL DEFAULT 'single',
  ADD COLUMN current_target_cursor BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN content_version BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN approved_version BIGINT NOT NULL DEFAULT 0,
  ADD KEY idx_short_links_tenant_created(tenant_id,created_at),
  ADD KEY idx_short_links_tenant_approval(tenant_id,approval_status,updated_at);
ALTER TABLE live_qrs
  ADD COLUMN tenant_id BIGINT NULL,
  ADD COLUMN content_version BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN approved_version BIGINT NOT NULL DEFAULT 0,
  ADD KEY idx_live_qrs_tenant_created(tenant_id,created_at),
  ADD KEY idx_live_qrs_tenant_approval(tenant_id,approval_status,updated_at);
ALTER TABLE live_qr_items
  ADD COLUMN content_version BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN approved_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE visit_logs
  ADD COLUMN tenant_id BIGINT NULL,
  ADD COLUMN target_id BIGINT NULL,
  ADD KEY idx_visit_tenant_time(tenant_id,created_at),
  ADD KEY idx_visit_target_time(target_id,created_at);
ALTER TABLE audit_logs
  ADD COLUMN tenant_id BIGINT NULL,
  ADD KEY idx_audit_tenant_time(tenant_id,created_at);

INSERT IGNORE INTO plans(code,name,description,price_monthly_cents,currency,max_members,max_short_links,max_live_qrs,max_targets_per_link,monthly_visits,features_json,status) VALUES
('free','Free','适合个人和试用',0,'CNY',3,100,20,3,10000,'{"analytics":true,"approval":true}','active'),
('pro','Pro','适合小团队运营',4900,'CNY',10,2000,300,20,500000,'{"analytics":true,"approval":true,"routing":true}','active'),
('business','Business','适合多成员业务团队',19900,'CNY',50,20000,3000,100,5000000,'{"analytics":true,"approval":true,"routing":true,"audit":true}','active'),
('enterprise','Enterprise','私有化与定制额度',0,'CNY',0,0,0,0,0,'{"analytics":true,"approval":true,"routing":true,"audit":true,"custom":true}','active');

INSERT IGNORE INTO tenants(slug,name,kind,status,owner_account_id)
SELECT CONCAT('personal-',id),CASE WHEN TRIM(name)='' THEN COALESCE(email,'Personal Workspace') ELSE CONCAT(name,' Workspace') END,'personal','active',id FROM admin_accounts;
INSERT IGNORE INTO tenant_members(tenant_id,account_id,role,status)
SELECT t.id,t.owner_account_id,'owner','active' FROM tenants t WHERE t.kind='personal' AND t.owner_account_id IS NOT NULL;
INSERT IGNORE INTO tenants(slug,name,kind,status,owner_account_id)
SELECT 'platform-default','Platform Workspace','organization','active',id FROM admin_accounts ORDER BY CASE WHEN role='admin' THEN 0 ELSE 1 END,id LIMIT 1;
INSERT IGNORE INTO tenant_members(tenant_id,account_id,role,status)
SELECT t.id,t.owner_account_id,'owner','active' FROM tenants t WHERE t.slug='platform-default' AND t.owner_account_id IS NOT NULL;
INSERT IGNORE INTO tenant_subscriptions(tenant_id,plan_id,status,provider,current_period_start)
SELECT t.id,p.id,'active','manual',CURRENT_TIMESTAMP FROM tenants t JOIN plans p ON p.code='free';

UPDATE short_links s JOIN tenants t ON t.owner_account_id=s.owner_account_id AND t.kind='personal' SET s.tenant_id=t.id WHERE s.tenant_id IS NULL AND s.owner_account_id IS NOT NULL;
UPDATE short_links s JOIN tenants t ON t.slug='platform-default' SET s.tenant_id=t.id WHERE s.tenant_id IS NULL;
UPDATE live_qrs l JOIN tenants t ON t.owner_account_id=l.owner_account_id AND t.kind='personal' SET l.tenant_id=t.id WHERE l.tenant_id IS NULL AND l.owner_account_id IS NOT NULL;
UPDATE live_qrs l JOIN tenants t ON t.slug='platform-default' SET l.tenant_id=t.id WHERE l.tenant_id IS NULL;
UPDATE short_links SET approved_version=CASE WHEN approval_status='approved' THEN content_version ELSE 0 END,
  approval_status=CASE WHEN approval_status='pending' THEN 'tenant_pending' WHEN approval_status='rejected' THEN 'tenant_rejected' ELSE approval_status END;
UPDATE live_qrs SET approved_version=CASE WHEN approval_status='approved' THEN content_version ELSE 0 END,
  approval_status=CASE WHEN approval_status='pending' THEN 'tenant_pending' WHEN approval_status='rejected' THEN 'tenant_rejected' ELSE approval_status END;
UPDATE live_qr_items SET approved_version=CASE WHEN approval_status='approved' THEN content_version ELSE 0 END,
  approval_status=CASE WHEN approval_status='pending' THEN 'tenant_pending' WHEN approval_status='rejected' THEN 'tenant_rejected' ELSE approval_status END;
