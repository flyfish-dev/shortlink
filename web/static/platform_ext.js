(() => {
  const $ = (s, root = document) => root.querySelector(s);
  const $$ = (s, root = document) => [...root.querySelectorAll(s)];
  const esc = s => String(s ?? '').replace(/[&<>'"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));
  const val = id => document.getElementById(id)?.value?.trim() || '';
  const isZh = () => (localStorage.getItem('asl_lang') || navigator.language || 'zh').toLowerCase().startsWith('zh');
  const txt = (zh, en) => isZh() ? zh : en;
  let me = null;
  const isAdmin = () => !!(me?.account?.is_admin || me?.account?.role === 'admin');

  const originalFetch = window.fetch.bind(window);
  window.fetch = (input, init = {}) => {
    try {
      const url = typeof input === 'string' ? input : input?.url || '';
      if (init && typeof init.body === 'string' && init.headers && String(url).includes('/api/admin/')) {
        const body = JSON.parse(init.body);
        if ((String(url).includes('/short-links')) && $('#slQRStyle')) {
          body.qr_style = val('slQRStyle') || 'rounded';
          body.qr_foreground = val('slQRForeground') || '#111827';
          body.qr_background = val('slQRBackground') || '#ffffff';
          body.qr_logo_url = val('slQRLogoURL') || '';
        }
        if ((String(url).includes('/live-qrs')) && $('#lQRStyle')) {
          const target = body.live || body;
          target.qr_style = val('lQRStyle') || 'rounded';
          target.qr_foreground = val('lQRForeground') || '#111827';
          target.qr_background = val('lQRBackground') || '#ffffff';
          target.qr_logo_url = val('lQRLogoURL') || '';
        }
        init = { ...init, body: JSON.stringify(body) };
      }
    } catch (_) {}
    return originalFetch(input, init);
  };

  function styleFields(prefix) {
    return `<div class="qr-style-card wide" data-qr-style-ext="${prefix}">
      <div><strong>${txt('二维码样式','QR style')}</strong><p class="muted">${txt('为入口二维码选择形状和品牌色，保存后公开二维码自动生效。','Choose shape and brand colors for the public entry QR.')}</p></div>
      <div class="mini-grid">
        <label class="field"><span>${txt('形状','Shape')}</span><select id="${prefix}QRStyle"><option value="rounded">${txt('圆角','Rounded')}</option><option value="dots">${txt('圆点','Dots')}</option><option value="classic">${txt('经典','Classic')}</option></select></label>
        <label class="field"><span>${txt('前景色','Foreground')}</span><input id="${prefix}QRForeground" type="color" value="#111827"></label>
        <label class="field"><span>${txt('背景色','Background')}</span><input id="${prefix}QRBackground" type="color" value="#ffffff"></label>
      </div>
      <div class="qr-preview" id="${prefix}QRPreview"></div>
    </div>`;
  }
  function updatePreview(prefix) {
    const wrap = document.getElementById(prefix + 'QRPreview');
    if (!wrap) return;
    const fg = val(prefix+'QRForeground') || '#111827';
    const bg = val(prefix+'QRBackground') || '#ffffff';
    const shape = val(prefix+'QRStyle') || 'rounded';
    wrap.innerHTML = `<div class="qr-preview-box" style="background:${esc(bg)};color:${esc(fg)}"><span style="font-weight:800;font-size:22px">QR</span></div><small class="muted">${esc(shape)} · ${esc(fg)} / ${esc(bg)}</small>`;
  }
  function injectQRFields() {
    if ($('#slRemark') && !$('#slQRStyle')) {
      const host = $('#slRemark').closest('label');
      host?.insertAdjacentHTML('afterend', styleFields('sl'));
      ['slQRStyle','slQRForeground','slQRBackground'].forEach(id => document.getElementById(id)?.addEventListener('input', () => updatePreview('sl')));
      updatePreview('sl');
    }
    if ($('#lGuideText') && !$('#lQRStyle')) {
      const host = $('#lGuideText').closest('label');
      host?.insertAdjacentHTML('beforebegin', styleFields('l'));
      ['lQRStyle','lQRForeground','lQRBackground'].forEach(id => document.getElementById(id)?.addEventListener('input', () => updatePreview('l')));
      updatePreview('l');
    }
  }

  async function refreshMe() {
    try {
      const res = await originalFetch('/api/admin/me', { credentials: 'same-origin' });
      me = await res.json();
      $$('.admin-only').forEach(el => { el.hidden = !isAdmin(); });
      if (!isAdmin() && (localStorage.getItem('asl_view') === 'settings' || localStorage.getItem('asl_view') === 'users')) localStorage.setItem('asl_view', 'dashboard');
    } catch (_) {}
  }

  async function api(path, options = {}) {
    const res = await originalFetch(path, { credentials: 'same-origin', headers: options.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }, ...options });
    const data = await res.json().catch(() => ({}));
    if (!res.ok || data.ok === false) throw new Error(data.message || data.error || 'request failed');
    return data;
  }
  function setUsersHeader() {
    $('#pageTitle').textContent = txt('用户管理','Users');
    $('#pageDesc').textContent = txt('管理来自互联网的普通用户与管理员权限。','Manage public users and admin permissions.');
    $('#createBtn').style.display = 'none';
    $$('.nav').forEach(n => n.classList.toggle('active', n.dataset.view === 'users'));
  }
  async function renderUsers() {
    if (!isAdmin()) return;
    setUsersHeader();
    const { data } = await api('/api/admin/users?limit=100');
    const headers = ['ID', txt('邮箱','Email'), txt('名称','Name'), txt('角色','Role'), txt('状态','Status'), txt('操作','Actions')];
    $('#content').innerHTML = `<div class="toolbar"><button class="primary" id="newUserBtn">${txt('新建用户','New user')}</button></div><div class="card table-card user-table-card"><table class="responsive-table user-table"><thead><tr>${headers.map(h => `<th>${h}</th>`).join('')}</tr></thead><tbody>${data.map(u => `<tr class="user-row"><td data-label="${headers[0]}">${u.id}</td><td data-label="${headers[1]}">${esc(u.email || '-')}</td><td data-label="${headers[2]}">${esc(u.name || '-')}</td><td data-label="${headers[3]}"><span class="badge">${esc(u.role)}</span></td><td data-label="${headers[4]}"><span class="badge">${esc(u.status)}</span></td><td class="action-cell" data-label="${headers[5]}"><button class="ghost" data-edit-user="${u.id}">${txt('编辑','Edit')}</button></td></tr>`).join('') || `<tr><td colspan="6"><div class="empty">${txt('暂无用户','No users')}</div></td></tr>`}</tbody></table></div>`;
    $('#newUserBtn').onclick = () => openUserModal();
    $$('[data-edit-user]').forEach(btn => btn.onclick = () => openUserModal(data.find(u => String(u.id) === btn.dataset.editUser)));
  }
  function openUserModal(row) {
    const modal = $('#modal'), card = modal.querySelector('.modal-card');
    $('#modalTitle').textContent = row ? txt('编辑用户','Edit user') : txt('新建用户','New user');
    $('#modalBody').innerHTML = `<div class="form-grid"><label class="field"><span>${txt('邮箱','Email')}</span><input id="extUserEmail" type="email" value="${esc(row?.email || '')}" ${row ? 'disabled' : ''}></label><label class="field"><span>${txt('名称','Name')}</span><input id="extUserName" value="${esc(row?.name || '')}"></label><label class="field"><span>${txt('角色','Role')}</span><select id="extUserRole"><option value="user">User</option><option value="admin">Admin</option></select></label><label class="field"><span>${txt('状态','Status')}</span><select id="extUserStatus"><option value="active">${txt('启用','Active')}</option><option value="disabled">${txt('停用','Disabled')}</option></select></label></div><div class="form-actions"><button class="ghost" data-close="1">${txt('取消','Cancel')}</button><button class="primary" id="extSaveUser">${txt('保存','Save')}</button></div>`;
    card.className = 'modal-card'; modal.hidden = false;
    $('#extUserRole').value = row?.role || 'user'; $('#extUserStatus').value = row?.status || 'active';
    $('#extSaveUser').onclick = async () => { const payload = { email: val('extUserEmail'), name: val('extUserName'), role: val('extUserRole'), status: val('extUserStatus') }; const out = await api(row ? `/api/admin/users/${row.id}` : '/api/admin/users', { method: row ? 'PUT' : 'POST', body: JSON.stringify(payload) }); modal.hidden = true; alert(out.recovery_key ? `${txt('恢复 Key','Recovery key')}: ${out.recovery_key}` : txt('已保存','Saved')); renderUsers(); };
  }

  document.addEventListener('click', e => {
    const btn = e.target.closest('[data-view="users"]');
    if (btn) { e.preventDefault(); e.stopPropagation(); localStorage.setItem('asl_view', 'users'); renderUsers(); }
  }, true);
  new MutationObserver(injectQRFields).observe(document.body, { childList: true, subtree: true });
  document.addEventListener('DOMContentLoaded', async () => { await refreshMe(); injectQRFields(); if (location.pathname.startsWith('/admin') && localStorage.getItem('asl_view') === 'users') setTimeout(renderUsers, 200); });
})();
