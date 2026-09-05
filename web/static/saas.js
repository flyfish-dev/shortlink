(() => {
  'use strict';

  const nativeFetch = window.fetch.bind(window);
  const tenantStorageKey = 'asl_tenant_id';
  let bootstrap = null;

  function currentTenantID() {
    return localStorage.getItem(tenantStorageKey) || '';
  }

  window.fetch = (input, init = {}) => {
    const url = typeof input === 'string' ? input : (input && input.url) || '';
    if (!url.includes('/api/admin/')) return nativeFetch(input, init);
    const headers = new Headers(init.headers || (typeof input !== 'string' ? input.headers : undefined));
    const tenantID = currentTenantID();
    if (tenantID) headers.set('X-Tenant-ID', tenantID);
    return nativeFetch(input, { ...init, headers, credentials: init.credentials || 'same-origin' });
  };

  const $ = (selector, root = document) => root.querySelector(selector);
  const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
  const esc = value => String(value ?? '').replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
  const isZh = () => (localStorage.getItem('asl_lang') || navigator.language || 'zh').toLowerCase().startsWith('zh');
  const text = (zh, en) => isZh() ? zh : en;
  const content = () => $('#content');

  async function api(path, options = {}) {
    const headers = new Headers(options.headers || {});
    if (!(options.body instanceof FormData) && options.body !== undefined) headers.set('Content-Type', 'application/json');
    const response = await window.fetch(path, { ...options, headers });
    const data = await response.json().catch(() => ({}));
    if (!response.ok || data.ok === false) throw new Error(data.message || data.error || `HTTP ${response.status}`);
    return data;
  }

  function toast(message, error = false) {
    const node = document.createElement('div');
    node.className = `toast${error ? ' error' : ''}`;
    node.textContent = message;
    document.body.appendChild(node);
    setTimeout(() => node.remove(), 3000);
  }

  async function loadBootstrap(force = false) {
    if (bootstrap && !force) return bootstrap;
    bootstrap = await api('/api/admin/saas/bootstrap');
    const selected = String(bootstrap.current_tenant?.id || '');
    if (selected && currentTenantID() !== selected) localStorage.setItem(tenantStorageKey, selected);
    updateWorkspaceSwitcher();
    updateSaaSNav();
    return bootstrap;
  }

  function updateWorkspaceSwitcher() {
    let host = $('#saasWorkspaceSwitcher');
    if (!host) {
      host = document.createElement('label');
      host.id = 'saasWorkspaceSwitcher';
      host.className = 'saas-workspace-switcher';
      host.innerHTML = `<span>${text('工作空间', 'Workspace')}</span><select aria-label="${text('切换工作空间', 'Switch workspace')}"></select>`;
      const brand = $('.sidebar .brand');
      brand?.insertAdjacentElement('afterend', host);
      $('select', host)?.addEventListener('change', async event => {
        localStorage.setItem(tenantStorageKey, event.target.value);
        bootstrap = null;
        try {
          await loadBootstrap(true);
          location.reload();
        } catch (err) {
          toast(err.message, true);
        }
      });
    }
    const select = $('select', host);
    if (!select || !bootstrap) return;
    select.innerHTML = (bootstrap.tenants || []).map(access => {
      const tenant = access.tenant || access.Tenant || access;
      const role = access.role || '';
      return `<option value="${tenant.id}" ${String(tenant.id) === String(bootstrap.current_tenant?.id) ? 'selected' : ''}>${esc(tenant.name)} · ${esc(role)}</option>`;
    }).join('');
  }

  function updateSaaSNav() {
    if (!bootstrap || $('#saasNavGroup')) return;
    const nav = $('.sidebar nav');
    if (!nav) return;
    const group = document.createElement('div');
    group.id = 'saasNavGroup';
    group.className = 'nav-group';
    group.innerHTML = `
      <p class="nav-section-title">${text('工作空间', 'Workspace')}</p>
      <button class="nav" data-saas-view="workspace"><i class="ph ph-buildings"></i><span>${text('租户与成员', 'Tenant & members')}</span></button>
      <button class="nav" data-saas-view="routing"><i class="ph ph-git-branch"></i><span>${text('流量路由', 'Traffic routing')}</span></button>
      <button class="nav" data-saas-view="subscription"><i class="ph ph-credit-card"></i><span>${text('订阅与额度', 'Subscription')}</span></button>
      <button class="nav" data-saas-view="approvals"><i class="ph ph-seal-check"></i><span>${text('租户审批', 'Tenant approval')}</span></button>
      ${bootstrap.platform_admin ? `<button class="nav" data-saas-view="platform"><i class="ph ph-shield-check"></i><span>${text('平台终审', 'Platform review')}</span></button>` : ''}`;
    const management = $$('.nav-group', nav).find(item => item.querySelector('.admin-only'));
    if (management) nav.insertBefore(group, management); else nav.appendChild(group);
  }

  function setPage(title, description, view) {
    $('#pageTitle').textContent = title;
    $('#pageDesc').textContent = description;
    $$('.nav').forEach(item => item.classList.toggle('active', item.dataset.saasView === view));
    const create = $('#createBtn');
    if (create) create.style.display = 'none';
    const menu = $('#createMenu');
    if (menu) menu.style.display = 'none';
  }

  function quotaBar(label, used, limit) {
    const unlimited = Number(limit || 0) === 0;
    const ratio = unlimited ? 0 : Math.min(100, Math.round((Number(used || 0) / Number(limit)) * 100));
    return `<div class="saas-quota-row"><div><strong>${esc(label)}</strong><span>${Number(used || 0).toLocaleString()} / ${unlimited ? '∞' : Number(limit).toLocaleString()}</span></div><div class="saas-meter"><i style="width:${ratio}%"></i></div></div>`;
  }

  async function renderWorkspace() {
    const data = await loadBootstrap(true);
    setPage(text('租户与成员', 'Tenant & members'), text('切换工作空间、维护成员及租户角色。', 'Switch workspaces and manage tenant roles.'), 'workspace');
    let members = [];
    if (['owner', 'admin'].includes(data.tenant_role)) {
      members = (await api(`/api/admin/saas/tenants/${data.current_tenant.id}/members`)).data || [];
    }
    content().innerHTML = `<section class="saas-page">
      <div class="saas-grid two">
        <article class="card saas-card"><div class="saas-card-head"><div><small>${text('当前工作空间', 'Current workspace')}</small><h2>${esc(data.current_tenant.name)}</h2></div><span class="type-pill">${esc(data.tenant_role)}</span></div><dl class="saas-definition"><div><dt>Slug</dt><dd>${esc(data.current_tenant.slug)}</dd></div><div><dt>${text('类型', 'Kind')}</dt><dd>${esc(data.current_tenant.kind)}</dd></div><div><dt>${text('状态', 'Status')}</dt><dd>${esc(data.current_tenant.status)}</dd></div></dl></article>
        <article class="card saas-card"><div class="saas-card-head"><div><small>${text('可访问工作空间', 'Accessible workspaces')}</small><h2>${(data.tenants || []).length}</h2></div><button class="primary" id="saasCreateTenant">${text('新建工作空间', 'New workspace')}</button></div><div class="saas-tenant-list">${(data.tenants || []).map(access => { const t = access.tenant; return `<button data-switch-tenant="${t.id}" class="saas-tenant-item ${t.id === data.current_tenant.id ? 'active' : ''}"><strong>${esc(t.name)}</strong><span>${esc(access.role)}</span></button>`; }).join('')}</div></article>
      </div>
      <article class="card saas-card"><div class="saas-card-head"><div><small>${text('成员权限', 'Member permissions')}</small><h2>${text('租户成员', 'Tenant members')}</h2></div>${['owner', 'admin'].includes(data.tenant_role) ? `<button class="primary" id="saasAddMember">${text('添加成员', 'Add member')}</button>` : ''}</div>
      ${members.length ? `<div class="saas-table-wrap"><table><thead><tr><th>${text('账户', 'Account')}</th><th>${text('角色', 'Role')}</th><th>${text('状态', 'Status')}</th><th>${text('操作', 'Actions')}</th></tr></thead><tbody>${members.map(m => `<tr><td><strong>${esc(m.name || '-')}</strong><small>${esc(m.email || `#${m.account_id}`)}</small></td><td><span class="type-pill">${esc(m.role)}</span></td><td>${esc(m.status)}</td><td><button class="ghost" data-edit-member="${m.account_id}" data-member-role="${esc(m.role)}" data-member-status="${esc(m.status)}">${text('编辑', 'Edit')}</button><button class="ghost danger" data-remove-member="${m.account_id}">${text('移除', 'Remove')}</button></td></tr>`).join('')}</tbody></table></div>` : `<div class="empty">${text('当前角色无成员管理权限。', 'Your role cannot manage members.')}</div>`}
      </article>
    </section>`;

    $('#saasCreateTenant')?.addEventListener('click', createTenant);
    $('#saasAddMember')?.addEventListener('click', addMember);
    $$('[data-switch-tenant]').forEach(button => button.addEventListener('click', () => switchTenant(button.dataset.switchTenant)));
    $$('[data-edit-member]').forEach(button => button.addEventListener('click', () => editMember(button.dataset.editMember, button.dataset.memberRole, button.dataset.memberStatus)));
    $$('[data-remove-member]').forEach(button => button.addEventListener('click', () => removeMember(button.dataset.removeMember)));
  }

  async function createTenant() {
    const name = prompt(text('工作空间名称', 'Workspace name'));
    if (!name?.trim()) return;
    try {
      const result = await api('/api/admin/saas/tenants', { method: 'POST', body: JSON.stringify({ name: name.trim() }) });
      await switchTenant(result.data.id);
    } catch (err) { toast(err.message, true); }
  }

  async function switchTenant(id) {
    localStorage.setItem(tenantStorageKey, String(id));
    bootstrap = null;
    await loadBootstrap(true);
    location.reload();
  }

  async function addMember() {
    const email = prompt(text('输入系统中已存在账户的邮箱', 'Enter an existing account email'));
    if (!email?.trim()) return;
    const role = prompt(text('角色：owner / admin / reviewer / member / analyst', 'Role: owner / admin / reviewer / member / analyst'), 'member');
    if (!role) return;
    try {
      await api(`/api/admin/saas/tenants/${bootstrap.current_tenant.id}/members`, { method: 'POST', body: JSON.stringify({ email: email.trim(), role: role.trim(), status: 'active' }) });
      toast(text('成员已添加', 'Member added'));
      await renderWorkspace();
    } catch (err) { toast(err.message, true); }
  }

  async function editMember(accountID, oldRole, oldStatus) {
    const role = prompt(text('新角色', 'New role'), oldRole);
    if (!role) return;
    const status = prompt(text('状态：active / disabled', 'Status: active / disabled'), oldStatus);
    if (!status) return;
    try {
      await api(`/api/admin/saas/tenants/${bootstrap.current_tenant.id}/members/${accountID}`, { method: 'PUT', body: JSON.stringify({ role, status }) });
      toast(text('成员权限已更新', 'Member updated'));
      await renderWorkspace();
    } catch (err) { toast(err.message, true); }
  }

  async function removeMember(accountID) {
    if (!confirm(text('确定移除该成员？', 'Remove this member?'))) return;
    try {
      await api(`/api/admin/saas/tenants/${bootstrap.current_tenant.id}/members/${accountID}`, { method: 'DELETE' });
      toast(text('成员已移除', 'Member removed'));
      await renderWorkspace();
    } catch (err) { toast(err.message, true); }
  }

  async function renderSubscription() {
    const data = await loadBootstrap(true);
    const [plansResult, requestsResult] = await Promise.all([api('/api/admin/saas/plans'), api('/api/admin/saas/subscription/requests')]);
    const quota = data.quota;
    const plan = quota.plan;
    setPage(text('订阅与额度', 'Subscription & quota'), text('查看套餐、当前用量并提交套餐变更申请。', 'Review limits and request a plan change.'), 'subscription');
    content().innerHTML = `<section class="saas-page">
      <div class="saas-grid two"><article class="card saas-card"><div class="saas-card-head"><div><small>${text('当前套餐', 'Current plan')}</small><h2>${esc(plan.name)}</h2></div><span class="status active">${esc(quota.subscription.status)}</span></div><p>${esc(plan.description)}</p><strong class="saas-price">${plan.price_monthly_cents ? `¥${(plan.price_monthly_cents / 100).toFixed(2)} / ${text('月', 'month')}` : text('免费或人工报价', 'Free or custom')}</strong></article>
      <article class="card saas-card"><h2>${text('本周期使用量', 'Current usage')}</h2>${quotaBar(text('成员', 'Members'), quota.members_used, plan.max_members)}${quotaBar(text('短链', 'Short links'), quota.short_links_used, plan.max_short_links)}${quotaBar(text('活码', 'Live QR'), quota.live_qrs_used, plan.max_live_qrs)}${quotaBar(text('月访问量', 'Monthly visits'), quota.monthly_visits_used, plan.monthly_visits)}</article></div>
      <div class="saas-plan-grid">${(plansResult.data || []).map(item => `<article class="card saas-card ${item.id === plan.id ? 'selected' : ''}"><small>${esc(item.code)}</small><h2>${esc(item.name)}</h2><p>${esc(item.description)}</p><ul><li>${text('成员', 'Members')}: ${item.max_members || '∞'}</li><li>${text('短链', 'Links')}: ${item.max_short_links || '∞'}</li><li>${text('每条目标', 'Targets/link')}: ${item.max_targets_per_link || '∞'}</li><li>${text('月访问', 'Monthly visits')}: ${item.monthly_visits || '∞'}</li></ul>${item.id !== plan.id ? `<button class="primary" data-request-plan="${item.id}">${text('申请此套餐', 'Request plan')}</button>` : `<span class="status active">${text('当前套餐', 'Current')}</span>`}</article>`).join('')}</div>
      <article class="card saas-card"><h2>${text('变更记录', 'Change requests')}</h2><div class="saas-table-wrap"><table><thead><tr><th>${text('套餐', 'Plan')}</th><th>${text('状态', 'Status')}</th><th>${text('说明', 'Note')}</th><th>${text('时间', 'Time')}</th></tr></thead><tbody>${(requestsResult.data || []).map(item => `<tr><td>${esc(item.from_plan)} → ${esc(item.to_plan)}</td><td><span class="type-pill">${esc(item.status)}</span></td><td>${esc(item.review_note || item.note || '-')}</td><td>${new Date(item.created_at).toLocaleString()}</td></tr>`).join('') || `<tr><td colspan="4">${text('暂无申请', 'No requests')}</td></tr>`}</tbody></table></div></article>
    </section>`;
    $$('[data-request-plan]').forEach(button => button.addEventListener('click', async () => {
      const note = prompt(text('申请说明（可选）', 'Request note (optional)')) || '';
      try {
        await api('/api/admin/saas/subscription/requests', { method: 'POST', body: JSON.stringify({ plan_id: Number(button.dataset.requestPlan), note }) });
        toast(text('套餐申请已提交平台审核', 'Plan request submitted'));
        await renderSubscription();
      } catch (err) { toast(err.message, true); }
    }));
  }

  async function renderRouting() {
    await loadBootstrap(true);
    setPage(text('流量路由', 'Traffic routing'), text('一个稳定短码对应多个目标，支持轮询、随机、权重、最少使用和 IP Hash。', 'Route one stable code across multiple destinations.'), 'routing');
    const result = await api('/api/admin/short-links?limit=200');
    const items = result.data || [];
    content().innerHTML = `<section class="saas-page"><article class="card saas-card"><div class="saas-card-head"><div><small>${text('多目标短链', 'Multi-target links')}</small><h2>${items.length}</h2></div></div><div class="saas-table-wrap"><table><thead><tr><th>${text('短码', 'Code')}</th><th>${text('标题', 'Title')}</th><th>${text('策略', 'Strategy')}</th><th>${text('审批', 'Approval')}</th><th>${text('操作', 'Actions')}</th></tr></thead><tbody>${items.map(item => `<tr><td><code>${esc(item.code)}</code></td><td>${esc(item.title || '-')}</td><td><span class="type-pill">${esc(item.routing_strategy || 'single')}</span></td><td>${esc(item.approval_status)}</td><td><button class="primary" data-manage-targets="${item.id}">${text('管理目标池', 'Manage targets')}</button></td></tr>`).join('') || `<tr><td colspan="5">${text('暂无短链', 'No short links')}</td></tr>`}</tbody></table></div></article></section>`;
    $$('[data-manage-targets]').forEach(button => button.addEventListener('click', () => openTargets(Number(button.dataset.manageTargets))));
  }

  function targetRow(item = {}) {
    return `<div class="saas-target-row" data-target-id="${Number(item.id || 0)}"><input data-field="name" placeholder="${text('名称', 'Name')}" value="${esc(item.name || '')}"><input data-field="url" type="url" placeholder="https://..." value="${esc(item.target_url || '')}"><select data-field="status"><option value="active" ${item.status !== 'disabled' ? 'selected' : ''}>active</option><option value="disabled" ${item.status === 'disabled' ? 'selected' : ''}>disabled</option></select><input data-field="weight" type="number" min="1" max="10000" value="${Number(item.weight || 1)}"><input data-field="max" type="number" min="0" value="${Number(item.max_hits || 0)}"><select data-field="health"><option value="unknown" ${item.health_status === 'unknown' || !item.health_status ? 'selected' : ''}>unknown</option><option value="healthy" ${item.health_status === 'healthy' ? 'selected' : ''}>healthy</option><option value="unhealthy" ${item.health_status === 'unhealthy' ? 'selected' : ''}>unhealthy</option></select><button class="ghost danger" data-remove-target>×</button></div>`;
  }

  async function openTargets(id) {
    try {
      const result = await api(`/api/admin/short-links/${id}/targets`);
      const modal = $('#modal');
      $('#modalTitle').textContent = text('短链目标池', 'Short-link target pool');
      $('#modalBody').innerHTML = `<div class="saas-target-editor"><label class="field"><span>${text('路由策略', 'Routing strategy')}</span><select id="saasRoutingStrategy"><option value="single">single</option><option value="round_robin">round_robin</option><option value="random">random</option><option value="weighted_random">weighted_random</option><option value="least_used">least_used</option><option value="ip_hash">ip_hash</option></select></label><div class="saas-target-head"><span>${text('名称', 'Name')}</span><span>URL</span><span>${text('状态', 'Status')}</span><span>${text('权重', 'Weight')}</span><span>${text('上限', 'Limit')}</span><span>${text('健康', 'Health')}</span><span></span></div><div id="saasTargetRows">${(result.data || []).map(targetRow).join('')}</div><div class="form-actions"><button class="ghost" id="saasAddTarget">${text('添加目标', 'Add target')}</button><button class="primary" id="saasSaveTargets">${text('保存并重新送审', 'Save & resubmit')}</button></div></div>`;
      $('#saasRoutingStrategy').value = result.routing_strategy || 'single';
      modal.hidden = false;
      modal.querySelector('.modal-card')?.classList.add('wide');
      $('#saasAddTarget').onclick = () => $('#saasTargetRows').insertAdjacentHTML('beforeend', targetRow());
      $('#saasTargetRows').onclick = event => event.target.closest('[data-remove-target]')?.closest('.saas-target-row')?.remove();
      $('#saasSaveTargets').onclick = async () => {
        const targets = $$('.saas-target-row').map((row, index) => ({ id: Number(row.dataset.targetId || 0), name: $('[data-field="name"]', row).value.trim(), target_url: $('[data-field="url"]', row).value.trim(), status: $('[data-field="status"]', row).value, weight: Number($('[data-field="weight"]', row).value || 1), max_hits: Number($('[data-field="max"]', row).value || 0), health_status: $('[data-field="health"]', row).value, sort_order: (index + 1) * 10 }));
        await api(`/api/admin/short-links/${id}/targets`, { method: 'PUT', body: JSON.stringify({ routing_strategy: $('#saasRoutingStrategy').value, targets }) });
        modal.hidden = true;
        toast(text('目标池已保存，内容已回到租户待审状态', 'Targets saved and resubmitted'));
        await renderRouting();
      };
    } catch (err) { toast(err.message, true); }
  }

  function approvalTable(items, platform) {
    return `<div class="saas-table-wrap"><table><thead><tr><th>${text('租户', 'Tenant')}</th><th>${text('资源', 'Resource')}</th><th>${text('内容', 'Content')}</th><th>${text('版本', 'Version')}</th><th>${text('操作', 'Actions')}</th></tr></thead><tbody>${items.map(item => `<tr><td>${esc(item.tenant_name)}</td><td>${esc(item.resource_type)} #${item.resource_id}</td><td><strong>${esc(item.title || '-')}</strong><small>${esc(item.code || '')}</small></td><td>v${item.content_version}</td><td><button class="primary" data-review-resource="${esc(item.resource_type)}" data-review-id="${item.resource_id}" data-review-stage="${platform ? 'platform' : 'tenant'}" data-review-action="approve">${text('通过', 'Approve')}</button><button class="ghost danger" data-review-resource="${esc(item.resource_type)}" data-review-id="${item.resource_id}" data-review-stage="${platform ? 'platform' : 'tenant'}" data-review-action="reject">${text('驳回', 'Reject')}</button></td></tr>`).join('') || `<tr><td colspan="5">${text('暂无待审内容', 'No pending content')}</td></tr>`}</tbody></table></div>`;
  }

  async function renderApprovals() {
    const data = await loadBootstrap(true);
    setPage(text('租户审批', 'Tenant approval'), text('租户审核通过后，内容才会进入平台终审队列。', 'Tenant approval submits content to platform review.'), 'approvals');
    if (!['owner', 'admin', 'reviewer'].includes(data.tenant_role)) {
      content().innerHTML = `<div class="empty">${text('当前角色没有审批权限。', 'Your role cannot review content.')}</div>`;
      return;
    }
    const result = await api('/api/admin/saas/reviews?stage=tenant_pending');
    content().innerHTML = `<section class="saas-page"><article class="card saas-card"><div class="saas-card-head"><div><small>tenant_pending</small><h2>${text('等待租户初审', 'Pending tenant review')}</h2></div><span class="type-pill">${(result.data || []).length}</span></div>${approvalTable(result.data || [], false)}</article></section>`;
    bindReviewButtons(renderApprovals);
  }

  async function renderPlatform() {
    await loadBootstrap(true);
    setPage(text('平台终审', 'Platform review'), text('平台总管理员终审租户内容并审批套餐变更。', 'Final content and subscription review.'), 'platform');
    const [reviews, subscriptions, tenants] = await Promise.all([api('/api/admin/saas/platform/reviews?stage=platform_pending'), api('/api/admin/saas/platform/subscription-requests'), api('/api/admin/saas/platform/tenants?limit=100')]);
    content().innerHTML = `<section class="saas-page"><article class="card saas-card"><div class="saas-card-head"><div><small>platform_pending</small><h2>${text('内容终审', 'Content final review')}</h2></div><span class="type-pill">${(reviews.data || []).length}</span></div>${approvalTable(reviews.data || [], true)}</article>
      <article class="card saas-card"><div class="saas-card-head"><h2>${text('套餐申请', 'Plan requests')}</h2><span class="type-pill">${(subscriptions.data || []).filter(item => item.status === 'pending').length}</span></div><div class="saas-table-wrap"><table><thead><tr><th>${text('租户', 'Tenant')}</th><th>${text('变更', 'Change')}</th><th>${text('说明', 'Note')}</th><th>${text('操作', 'Actions')}</th></tr></thead><tbody>${(subscriptions.data || []).filter(item => item.status === 'pending').map(item => `<tr><td>${esc(item.tenant_name)}</td><td>${esc(item.from_plan)} → ${esc(item.to_plan)}</td><td>${esc(item.note || '-')}</td><td><button class="primary" data-subscription-id="${item.id}" data-subscription-action="approve">${text('通过', 'Approve')}</button><button class="ghost danger" data-subscription-id="${item.id}" data-subscription-action="reject">${text('驳回', 'Reject')}</button></td></tr>`).join('') || `<tr><td colspan="4">${text('暂无待审申请', 'No pending requests')}</td></tr>`}</tbody></table></div></article>
      <article class="card saas-card"><div class="saas-card-head"><h2>${text('平台租户', 'Platform tenants')}</h2><span class="type-pill">${(tenants.data || []).length}</span></div><div class="saas-tenant-list">${(tenants.data || []).map(item => `<div class="saas-tenant-item"><strong>${esc(item.name)}</strong><span>${esc(item.slug)} · ${esc(item.status)}</span></div>`).join('')}</div></article></section>`;
    bindReviewButtons(renderPlatform);
    $$('[data-subscription-id]').forEach(button => button.addEventListener('click', async () => {
      const note = prompt(text('审批意见（可选）', 'Review note (optional)')) || '';
      try {
        await api(`/api/admin/saas/platform/subscription-requests/${button.dataset.subscriptionId}`, { method: 'POST', body: JSON.stringify({ action: button.dataset.subscriptionAction, note }) });
        toast(text('订阅申请已处理', 'Subscription request reviewed'));
        await renderPlatform();
      } catch (err) { toast(err.message, true); }
    }));
  }

  function bindReviewButtons(refresh) {
    $$('[data-review-resource]').forEach(button => button.addEventListener('click', async () => {
      const note = prompt(text('审批意见（驳回时建议填写）', 'Review note')) || '';
      const platform = button.dataset.reviewStage === 'platform';
      const base = platform ? '/api/admin/saas/platform/reviews' : '/api/admin/saas/reviews';
      try {
        await api(`${base}/${button.dataset.reviewResource}/${button.dataset.reviewId}`, { method: 'POST', body: JSON.stringify({ action: button.dataset.reviewAction, note, include_items: button.dataset.reviewResource === 'live_qr' }) });
        toast(text('审批状态已更新', 'Review completed'));
        await refresh();
      } catch (err) { toast(err.message, true); }
    }));
  }

  async function renderView(view) {
    try {
      if (view === 'workspace') return await renderWorkspace();
      if (view === 'routing') return await renderRouting();
      if (view === 'subscription') return await renderSubscription();
      if (view === 'approvals') return await renderApprovals();
      if (view === 'platform') return await renderPlatform();
    } catch (err) {
      content().innerHTML = `<div class="empty"><strong>${text('加载失败', 'Load failed')}</strong><p>${esc(err.message)}</p></div>`;
      toast(err.message, true);
    }
  }

  document.addEventListener('click', event => {
    const button = event.target.closest('[data-saas-view]');
    if (!button) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    const view = button.dataset.saasView;
    localStorage.setItem('asl_view', `saas:${view}`);
    renderView(view);
  }, true);

  async function start() {
    try {
      await loadBootstrap(true);
      const saved = localStorage.getItem('asl_view') || '';
      if (saved.startsWith('saas:')) await renderView(saved.slice(5));
    } catch (err) {
      console.error('SaaS bootstrap failed', err);
    }
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start);
  else start();
})();
