const app = document.getElementById('app');
let baseURL = app.dataset.baseUrl || location.origin;
const content = document.getElementById('content');
const pageTitle = document.getElementById('pageTitle');
const pageDesc = document.getElementById('pageDesc');
const createBtn = document.getElementById('createBtn');
const refreshBtn = document.getElementById('refreshBtn');
const modal = document.getElementById('modal');
const modalCard = modal.querySelector('.modal-card');
const modalTitle = document.getElementById('modalTitle');
const modalBody = document.getElementById('modalBody');

const state = { view: localStorage.getItem('asl_view') || 'dashboard', q: '', me: null, liveEditor: null, locale: 'zh' };

const I18N = {
  zh: {
    'nav.dashboard':'总览','nav.shorts':'短链','nav.lives':'活码','nav.users':'用户管理','nav.settings':'系统设置','common.refresh':'刷新','common.logout':'退出登录','common.loading':'加载中...','common.create':'新建','common.copy':'复制','common.edit':'编辑','common.config':'配置','common.stats':'统计','common.delete':'删除','common.cancel':'取消','common.save':'保存','common.close':'关闭','common.enabled':'启用','common.disabled':'停用','common.search':'搜索','common.actions':'操作','common.status':'状态','common.updated':'更新时间','common.visits':'访问','common.approve':'通过','common.reject':'驳回','common.pending':'待审','common.review':'审核','common.approved':'已通过','common.rejected':'已驳回','common.backPending':'退回待审','msg.copied':'已复制','msg.saved':'已保存，等待管理员审核后才会生效','msg.deleted':'已删除','msg.reviewed':'审核状态已更新','msg.accountSaved':'账户信息已保存','msg.settingsSaved':'系统设置已保存','msg.needNote':'请输入驳回原因（可留空）：',
    'page.dashboard':'总览','page.dashboardDesc':'短链、活码、审核与访问数据一屏掌控。','page.shorts':'短链管理','page.shortsDesc':'把长链接转换为可复用、可统计、可控制的短链。','page.lives':'活码管理','page.livesDesc':'固定二维码入口，多套二维码自动轮替，适合微信群、客服码和活动码。','page.settings':'系统设置','page.settingsDesc':'集中维护域名、登录方式、SMTP 和管理员邮箱。','btn.newShort':'新建短链','btn.newLive':'新建活码',
    'dash.shortLinks':'短链总数','dash.liveQRs':'活码总数','dash.liveItems':'可用二维码','dash.today':'今日访问','dash.total':'累计访问','dash.pendingShort':'待审短链','dash.pendingLive':'待审活码','dash.pendingItem':'待审二维码','dash.recoveryTitle':'账户恢复','dash.recoveryOK':'当前浏览器已保存长期登录令牌。恢复 Key 只在首页入口中查看，建议复制到密码管理器。','dash.recoveryWarn':'恢复 Key 暂不可显示，通常是 APP_SECRET 变更导致旧 Key 无法解密。可以重新生成一个新的恢复 Key。','dash.viewRecovery':'查看 / 复制恢复 Key','dash.rotateRecovery':'重新生成恢复 Key','dash.hints':'上线检查','dash.noEmail':'尚未绑定管理员邮箱。建议在系统设置里绑定，用于 Magic Link 登录。','dash.noSMTP':'SMTP 尚未配置。公网部署建议启用 Magic Link。','dash.noBase':'尚未设置公网域名。短链、活码和邮件链接会使用当前访问地址。','dash.hasPending':'有内容等待审核，未通过审核前公开访问会显示无效。','dash.shortAbility':'短链能力','dash.shortAbilityDesc':'长链接转换短链、自动跳转、有效期、访问上限、备用链接、访问统计、二维码分享。','dash.liveAbility':'活码能力','dash.liveAbilityDesc':'一个固定活码维护多套二维码，支持过期时间、展示上限、轮询/随机/最少展示策略与长按识别指引。','dash.createShort':'创建短链','dash.createLive':'创建活码',
    'short.title':'标题','short.code':'短链','short.target':'原链接','short.empty':'暂无短链','short.search':'搜索短码 / 标题 / 原链接','short.qr':'二维码','short.modalNew':'新建短链','short.modalEdit':'编辑短链','short.customCode':'自定义短码','short.targetURL':'长链接 / 目标链接','short.redirect':'跳转方式','short.starts':'开始时间','short.expires':'过期时间','short.max':'访问上限（0不限）','short.fallback':'失效备用链接','short.remark':'备注','short.pendingTip':'新建或修改后会进入待审状态，审核通过前不会跳转。',
    'live.title':'标题','live.link':'活码链接','live.strategy':'策略','live.empty':'暂无活码','live.search':'搜索活码 / 标题','live.qr':'活码图','live.modalNew':'新建活码','live.modalEdit':'配置活码','live.base':'1. 基础信息','live.items':'2. 二维码组','live.publish':'3. 发布确认','live.description':'描述','live.guideTitle':'引导标题','live.fallback':'无可用二维码时备用链接','live.guideText':'引导文案','live.itemConfig':'二维码配置','live.itemHint':'可以连续添加多张二维码，最后统一保存。','live.itemTitle':'二维码标题','live.itemImage':'二维码图片','live.upload':'上传图片','live.itemTarget':'可选目标链接','live.sort':'排序','live.maxViews':'展示上限','live.weight':'权重（随机策略）','live.addItem':'加入 / 更新列表','live.clear':'清空表单','live.validity':'有效期','live.views':'展示','live.noItems':'还没有二维码。可以在左侧上传后加入列表，最后统一保存。','live.saved':'已保存','live.draft':'待新增','live.publishEntry':'发布入口','live.saveAll':'保存全部','live.saveClose':'保存并关闭','live.unsavedLink':'保存后自动生成；也可以在基础信息里填写自定义短码。','live.noCode':'还没有生成短码。点击“保存全部”后系统会自动生成活码链接。','live.approvalWarn':'当前活码或二维码尚未审核通过，公开访问不会展示二维码。','live.noItemWarn':'提示：当前没有二维码。活码访问时会走备用链接；未设置备用链接时会显示“暂无可用二维码”。',
    'settings.account':'管理员账户','settings.accountDesc':'绑定邮箱后可使用 Magic Link 登录；浏览器一键登录仍然保留。','settings.email':'邮箱','settings.name':'名称','settings.saveAccount':'保存账户','settings.system':'系统参数','settings.appName':'站点名称','settings.appNameZH':'中文站点名称','settings.appNameEN':'English site name','settings.brandI18nHint':'站点名称会按当前界面语言显示，邮件标题和公共页面也会使用对应语言。','settings.baseUrl':'公网域名','settings.locale':'默认语言','settings.autoLocale':'自动匹配浏览器','settings.loginMode':'登录模式','settings.hybrid':'Magic Link + 浏览器一键','settings.magic':'仅 Magic Link','settings.oneClick':'仅浏览器一键','settings.database':'数据库','settings.smtp':'SMTP / Magic Link','settings.smtpDeliverabilityHint':'建议使用与 SMTP 账号同域的发信邮箱，并在域名 DNS 配置 SPF、DKIM、DMARC，避免登录邮件被判定为垃圾邮件。','settings.smtpEnabled':'启用 SMTP','settings.smtpHost':'SMTP 主机','settings.smtpPort':'端口','settings.smtpSecurity':'安全协议','settings.smtpUsername':'用户名','settings.smtpPassword':'密码 / 授权码','settings.smtpPasswordHint':'留空则保持不变','settings.smtpFrom':'发信邮箱','settings.smtpSet':'密码已保存','settings.smtpUnset':'尚未保存密码','settings.saveSettings':'保存系统设置',
    'strategy.round_robin':'轮询','strategy.random':'按权重随机','strategy.least_used':'最少展示优先'
  },
  en: {
    'nav.dashboard':'Dashboard','nav.shorts':'Short links','nav.lives':'Live QR','nav.users':'Users','nav.settings':'Settings','common.refresh':'Refresh','common.logout':'Logout','common.loading':'Loading...','common.create':'Create','common.copy':'Copy','common.edit':'Edit','common.config':'Configure','common.stats':'Stats','common.delete':'Delete','common.cancel':'Cancel','common.save':'Save','common.close':'Close','common.enabled':'Active','common.disabled':'Disabled','common.search':'Search','common.actions':'Actions','common.status':'Status','common.updated':'Updated','common.visits':'Visits','common.approve':'Approve','common.reject':'Reject','common.pending':'Pending','common.review':'Review','common.approved':'Approved','common.rejected':'Rejected','common.backPending':'Back to pending','msg.copied':'Copied','msg.saved':'Saved. It will work after admin approval.','msg.deleted':'Deleted','msg.reviewed':'Review status updated','msg.accountSaved':'Account saved','msg.settingsSaved':'Settings saved','msg.needNote':'Reject note (optional):',
    'page.dashboard':'Dashboard','page.dashboardDesc':'Manage links, live QR, reviews and traffic from one screen.','page.shorts':'Short links','page.shortsDesc':'Turn long URLs into reusable, trackable and controllable short links.','page.lives':'Live QR','page.livesDesc':'One fixed QR entry rotates multiple QR images for groups, support or campaigns.','page.settings':'System settings','page.settingsDesc':'Manage domain, login mode, SMTP and admin email.','btn.newShort':'New short link','btn.newLive':'New live QR',
    'dash.shortLinks':'Short links','dash.liveQRs':'Live QR','dash.liveItems':'Available QR','dash.today':'Today','dash.total':'Total visits','dash.pendingShort':'Pending links','dash.pendingLive':'Pending live QR','dash.pendingItem':'Pending QR items','dash.recoveryTitle':'Account recovery','dash.recoveryOK':'This browser has a long-lived token. View the recovery key only here and save it in a password manager.','dash.recoveryWarn':'The recovery key is not readable, usually because APP_SECRET changed. Rotate a new key.','dash.viewRecovery':'View / copy recovery key','dash.rotateRecovery':'Rotate recovery key','dash.hints':'Launch checklist','dash.noEmail':'Admin email is not bound. Bind it in settings for Magic Link login.','dash.noSMTP':'SMTP is not configured. Magic Link is recommended for public deployment.','dash.noBase':'Public domain is not set. Links and emails will use the current origin.','dash.hasPending':'Some content is pending review. Public access is invalid before approval.','dash.shortAbility':'Short link','dash.shortAbilityDesc':'URL shortening, redirects, expiry, visit limits, fallback URL, analytics and QR sharing.','dash.liveAbility':'Live QR','dash.liveAbilityDesc':'Maintain multiple QR images behind one fixed entry with expiry, limits, weighted/random rotation and guidance.','dash.createShort':'Create short link','dash.createLive':'Create live QR',
    'short.title':'Title','short.code':'Short URL','short.target':'Target URL','short.empty':'No short links','short.search':'Search code / title / target','short.qr':'QR','short.modalNew':'New short link','short.modalEdit':'Edit short link','short.customCode':'Custom code','short.targetURL':'Long / target URL','short.redirect':'Redirect type','short.starts':'Starts at','short.expires':'Expires at','short.max':'Visit limit (0 = unlimited)','short.fallback':'Fallback URL','short.remark':'Remark','short.pendingTip':'New or edited links become pending and will not redirect until approved.',
    'live.title':'Title','live.link':'Live QR link','live.strategy':'Strategy','live.empty':'No live QR','live.search':'Search live QR / title','live.qr':'QR image','live.modalNew':'New live QR','live.modalEdit':'Configure live QR','live.base':'1. Basic','live.items':'2. QR items','live.publish':'3. Publish','live.description':'Description','live.guideTitle':'Guide title','live.fallback':'Fallback when no QR is available','live.guideText':'Guide text','live.itemConfig':'QR item','live.itemHint':'Add multiple QR images and save them in one transaction.','live.itemTitle':'QR title','live.itemImage':'QR image','live.upload':'Upload image','live.itemTarget':'Optional target URL','live.sort':'Sort','live.maxViews':'View limit','live.weight':'Weight (random)','live.addItem':'Add / update list','live.clear':'Clear form','live.validity':'Validity','live.views':'Views','live.noItems':'No QR items yet. Upload and add on the left, then save all.','live.saved':'Saved','live.draft':'New draft','live.publishEntry':'Publish entry','live.saveAll':'Save all','live.saveClose':'Save and close','live.unsavedLink':'Generated after saving; or fill a custom code in Basic.','live.noCode':'No code yet. Save all to generate the public link.','live.approvalWarn':'This live QR or its items are not approved yet, so public access will not show a QR.','live.noItemWarn':'No QR items. The fallback URL will be used; if absent, visitors see an unavailable message.',
    'settings.account':'Admin account','settings.accountDesc':'After binding an email, Magic Link login is available. Browser one-click login is preserved.','settings.email':'Email','settings.name':'Name','settings.saveAccount':'Save account','settings.system':'System','settings.appName':'Site name','settings.appNameZH':'Chinese site name','settings.appNameEN':'English site name','settings.brandI18nHint':'The product name follows the current interface language, including email subjects and public pages.','settings.baseUrl':'Public domain','settings.locale':'Default language','settings.autoLocale':'Auto match browser','settings.loginMode':'Login mode','settings.hybrid':'Magic Link + browser one-click','settings.magic':'Magic Link only','settings.oneClick':'Browser one-click only','settings.database':'Database','settings.smtp':'SMTP / Magic Link','settings.smtpDeliverabilityHint':'Use a From address on the same domain as the SMTP account, and configure SPF, DKIM, and DMARC in DNS to keep login emails out of spam.','settings.smtpEnabled':'Enable SMTP','settings.smtpHost':'SMTP host','settings.smtpPort':'Port','settings.smtpSecurity':'Security','settings.smtpUsername':'Username','settings.smtpPassword':'Password / app token','settings.smtpPasswordHint':'Leave empty to keep current','settings.smtpFrom':'From email','settings.smtpSet':'Password saved','settings.smtpUnset':'No password saved','settings.saveSettings':'Save settings',
    'strategy.round_robin':'Round robin','strategy.random':'Weighted random','strategy.least_used':'Least used'
  }
};

const esc = (s) => String(s ?? '').replace(/[&<>'"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));
const val = (id) => document.getElementById(id)?.value?.trim() || '';
const rawVal = (id) => document.getElementById(id)?.value || '';
const num = (id) => Number(document.getElementById(id)?.value || 0);
const checked = (id) => !!document.getElementById(id)?.checked;
const localeCode = () => state.locale === 'zh' ? 'zh-CN' : 'en-US';
const t = (key, fallback = '') => I18N[state.locale]?.[key] || I18N.zh[key] || fallback || key;
const tx = (zh, en) => state.locale === 'zh' ? zh : en;
const fmtDate = (s) => s ? new Date(s).toLocaleString(localeCode(), { hour12: false }) : '-';
const themeToggleHTML = '<span class="theme-option theme-sun" aria-hidden="true"><svg viewBox="0 0 24 24"><circle class="theme-icon-fill" cx="12" cy="12" r="4.4"/><path class="theme-icon-accent" d="M12 2.9v2.2M12 18.9v2.2M4.3 4.3l1.6 1.6M18.1 18.1l1.6 1.6M2.9 12h2.2M18.9 12h2.2M4.3 19.7l1.6-1.6M18.1 5.9l1.6-1.6"/></svg></span><span class="theme-option theme-moon" aria-hidden="true"><svg viewBox="0 0 24 24"><path class="theme-icon-fill" d="M19.3 14.4A7.4 7.4 0 0 1 9.6 4.7 8.2 8.2 0 1 0 19.3 14.4Z"/><path class="theme-icon-accent" d="M16.8 3.7l.5 1.5 1.5.5-1.5.5-.5 1.5-.5-1.5-1.5-.5 1.5-.5.5-1.5Z"/></svg></span>';
const fmtInputDate = (s) => {
  if (!s) return '';
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return String(s).slice(0, 16);
  const pad = n => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

function detectLocale(preferred = 'auto') {
  const saved = localStorage.getItem('asl_lang');
  if (saved === 'zh' || saved === 'en') return saved;
  if (preferred && preferred !== 'auto') return String(preferred).toLowerCase().startsWith('zh') ? 'zh' : 'en';
  return (navigator.language || 'zh').toLowerCase().startsWith('zh') ? 'zh' : 'en';
}
function applyI18n() {
  document.documentElement.lang = state.locale === 'zh' ? 'zh-CN' : 'en';
  document.querySelectorAll('[data-i18n]').forEach(el => { el.textContent = t(el.dataset.i18n); });
  updateAppName();
  const langBtn = document.getElementById('langToggle');
  if (langBtn) {
    langBtn.dataset.locale = state.locale;
    langBtn.setAttribute('aria-label', state.locale === 'zh' ? '切换为英文' : 'Switch to Chinese');
    langBtn.setAttribute('title', state.locale === 'zh' ? '切换为英文' : 'Switch to Chinese');
    langBtn.innerHTML = '<span>中文</span><span>EN</span>';
  }
  applyTheme();
  if (state.view) setHeaderForView();
}
function applyTheme() {
  const saved = localStorage.getItem('asl_theme');
  const isDark = saved ? saved === 'dark' : window.matchMedia?.('(prefers-color-scheme: dark)').matches;
  document.documentElement.dataset.theme = isDark ? 'dark' : 'light';
  const btn = document.getElementById('themeToggle');
  if (btn) {
    btn.innerHTML = themeToggleHTML;
    btn.dataset.themeMode = isDark ? 'dark' : 'light';
    btn.setAttribute('aria-label', isDark ? (state.locale === 'zh' ? '当前夜间模式，切换到白天模式' : 'Dark mode active, switch to light mode') : (state.locale === 'zh' ? '当前白天模式，切换到夜间模式' : 'Light mode active, switch to dark mode'));
    btn.setAttribute('title', btn.getAttribute('aria-label'));
  }
}
function toggleTheme() {
  localStorage.setItem('asl_theme', document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark');
  applyTheme();
}
function toggleLocale() {
  state.locale = state.locale === 'zh' ? 'en' : 'zh';
  localStorage.setItem('asl_lang', state.locale);
  applyI18n();
  render();
}

const api = async (path, options = {}) => {
  const res = await fetch(path, {
    credentials: 'same-origin',
    headers: options.body instanceof FormData ? {} : { 'Content-Type': 'application/json' },
    ...options,
  });
  if (res.status === 401) location.href = '/login';
  if (res.status === 428) location.href = '/setup';
  const data = await res.json().catch(() => ({}));
  if (!res.ok || data.ok === false) throw new Error(data.message || data.error || '请求失败');
  return data;
};

const toast = (msg, kind = '') => {
  const el = document.createElement('div');
  el.className = `toast${kind ? ' ' + kind : ''}`;
  el.textContent = msg;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), 1900);
};
const setInlineMessage = (id, msg = '', kind = '') => {
  const el = document.getElementById(id);
  if (!el) return;
  el.textContent = msg;
  el.className = `message${kind ? ' ' + kind : ''}`;
};
const setBusy = (buttons, busy, label = '') => {
  const list = Array.isArray(buttons) ? buttons : [buttons];
  list.filter(Boolean).forEach(btn => {
    if (busy) {
      btn.dataset.originalText = btn.textContent;
      btn.disabled = true;
      btn.textContent = label || (state.locale === 'zh' ? '保存中...' : 'Saving...');
    } else {
      btn.disabled = false;
      if (btn.dataset.originalText) btn.textContent = btn.dataset.originalText;
      delete btn.dataset.originalText;
    }
  });
};
const copy = async (text) => {
  await navigator.clipboard.writeText(text);
  toast(t('msg.copied'));
};
const publicShort = code => `${baseURL}/s/${code}`;
const publicLive = code => `${baseURL}/q/${code}`;
const qrPath = (kind, code, format = 'svg') => `/qr/${kind}/${encodeURIComponent(code)}.${format}`;
const qrPreviewPath = (content, cfg = {}) => {
  const q = new URLSearchParams({
    content,
    style: cfg.qr_style || 'rounded',
    foreground: cfg.qr_foreground || '#111827',
    background: cfg.qr_background || '#ffffff',
  });
  if (cfg.qr_logo_url) q.set('logo_url', cfg.qr_logo_url);
  return `/api/admin/qr-preview?${q.toString()}`;
};
function qrConfigFrom(prefix) {
  return {
    qr_style: val(`${prefix}QRStyle`) || 'rounded',
    qr_foreground: val(`${prefix}QRForeground`) || '#111827',
    qr_background: val(`${prefix}QRBackground`) || '#ffffff',
    qr_logo_url: val(`${prefix}QRLogoURL`),
  };
}
function qrDesignerHTML(prefix, cfg = {}, content = '') {
  const style = cfg.qr_style || 'rounded';
  const fg = cfg.qr_foreground || '#111827';
  const bg = cfg.qr_background || '#ffffff';
  const logo = cfg.qr_logo_url || '';
  const preview = qrPreviewPath(content || `${baseURL}/q/preview`, { qr_style: style, qr_foreground: fg, qr_background: bg, qr_logo_url: logo });
  return `<div class="qr-designer">
    <div class="section-title"><h3>${tx('入口二维码定制','Entry QR design')}</h3><p class="muted">${tx('统一设置活码入口的视觉风格、品牌色和中心贴图。','Control the public entry QR style, brand colors and center mark.')}</p></div>
    <div class="qr-designer-grid">
      <div class="qr-designer-controls">
        <label class="field"><span>${tx('样式','Style')}</span><select id="${prefix}QRStyle"><option value="rounded">${tx('圆角模块','Rounded modules')}</option><option value="dots">${tx('圆点模块','Dot modules')}</option><option value="classic">${tx('经典方块','Classic blocks')}</option></select></label>
        <div class="mini-grid">
          <label class="field"><span>${tx('前景色','Foreground')}</span><input id="${prefix}QRForeground" type="color" value="${esc(fg)}"></label>
          <label class="field"><span>${tx('背景色','Background')}</span><input id="${prefix}QRBackground" type="color" value="${esc(bg)}"></label>
        </div>
        <div class="qr-logo-row">
          <label class="field"><span>${tx('中心贴图','Center mark')}</span><input id="${prefix}QRLogoURL" value="${esc(logo)}" placeholder="/uploads/brand.png"></label>
          <div class="qr-logo-thumb" id="${prefix}QRLogoThumb">${logo ? `<img src="${esc(logo)}" alt="">` : `<span>${tx('贴图','Mark')}</span>`}</div>
        </div>
        ${fileUploadHTML(`${prefix}QRLogoFile`, tx('上传贴图','Upload mark'), tx('PNG / JPG / WEBP','PNG / JPG / WEBP'))}
      </div>
      <div class="qr-preview-panel">
        <img id="${prefix}QRPreview" src="${esc(preview)}" alt="${tx('二维码风格预览','QR style preview')}" loading="lazy">
        <div class="qr-preview-meta"><span id="${prefix}QRMeta">${esc(style)} · ${esc(fg)} / ${esc(bg)}</span></div>
      </div>
    </div>
  </div>`;
}
function fileUploadHTML(id, label, hint) {
  return `<label class="field file-field"><span>${label}</span><span class="file-picker"><span class="file-picker-button">${tx('选择文件','Choose file')}</span><span class="file-picker-name" id="${id}Name" data-placeholder="${esc(hint)}">${esc(hint)}</span><input id="${id}" type="file" accept="image/png,image/jpeg,image/gif,image/webp"></span></label>`;
}
function bindFilePickerName(input) {
  if (!input) return;
  const name = input.closest('.file-picker')?.querySelector('.file-picker-name');
  if (!name) return;
  const placeholder = name.dataset.placeholder || name.textContent;
  input.addEventListener('change', () => {
    name.textContent = input.files?.[0]?.name || placeholder;
  });
}
function resetFilePickerName(input) {
  const name = input?.closest('.file-picker')?.querySelector('.file-picker-name');
  if (name) name.textContent = name.dataset.placeholder || name.textContent;
}
function bindQRDesigner(prefix, contentGetter) {
  const sync = () => updateQRPreview(prefix, contentGetter());
  [`${prefix}QRStyle`, `${prefix}QRForeground`, `${prefix}QRBackground`, `${prefix}QRLogoURL`].forEach(id => document.getElementById(id)?.addEventListener('input', sync));
  const logoFile = document.getElementById(`${prefix}QRLogoFile`);
  bindFilePickerName(logoFile);
  logoFile?.addEventListener('change', e => uploadImageInto(e, `${prefix}QRLogoURL`, sync));
  sync();
}
function updateQRPreview(prefix, content) {
  const img = document.getElementById(`${prefix}QRPreview`);
  if (!img) return;
  const cfg = qrConfigFrom(prefix);
  img.src = qrPreviewPath(content || `${baseURL}/q/preview`, cfg);
  const meta = document.getElementById(`${prefix}QRMeta`);
  if (meta) meta.textContent = `${cfg.qr_style} · ${cfg.qr_foreground} / ${cfg.qr_background}${cfg.qr_logo_url ? ' · logo' : ''}`;
  const thumb = document.getElementById(`${prefix}QRLogoThumb`);
  if (thumb) thumb.innerHTML = cfg.qr_logo_url ? `<img src="${esc(cfg.qr_logo_url)}" alt="">` : `<span>${tx('贴图','Mark')}</span>`;
}
function qrDownloadButtonsHTML(kind, code, compact = false) {
  if (!code) return '';
  const label = compact ? '' : `<span>${tx('下载','Download')}</span>`;
  return `<div class="download-group" data-qr-downloads="${esc(kind)}:${esc(code)}">${label}<a class="button ghost" download href="${qrPath(kind, code, 'svg')}">SVG</a><a class="button ghost" download href="${qrPath(kind, code, 'png')}">PNG</a><button class="ghost" type="button" data-webp-qr="${esc(kind)}:${esc(code)}">WEBP</button></div>`;
}
function bindQRDownloadButtons(root = document) {
  root.querySelectorAll('[data-webp-qr]').forEach(btn => {
    btn.onclick = () => {
      const [kind, code] = btn.dataset.webpQr.split(':');
      downloadQRWebP(kind, code);
    };
  });
}
async function downloadQRWebP(kind, code) {
  try {
    const pngBlob = await fetch(qrPath(kind, code, 'png'), { credentials: 'same-origin' }).then(r => {
      if (!r.ok) throw new Error('QR PNG unavailable');
      return r.blob();
    });
    const url = URL.createObjectURL(pngBlob);
    const img = new Image();
    img.decoding = 'async';
    img.onload = () => {
      const canvas = document.createElement('canvas');
      canvas.width = img.naturalWidth || 1024;
      canvas.height = img.naturalHeight || 1024;
      const ctx = canvas.getContext('2d');
      ctx.drawImage(img, 0, 0, canvas.width, canvas.height);
      canvas.toBlob(out => {
        URL.revokeObjectURL(url);
        if (!out) return toast(tx('当前浏览器不支持导出 WEBP','This browser cannot export WEBP'));
        const a = document.createElement('a');
        a.href = URL.createObjectURL(out);
        a.download = `${code}.webp`;
        a.click();
        setTimeout(() => URL.revokeObjectURL(a.href), 1000);
      }, 'image/webp', .96);
    };
    img.onerror = () => { URL.revokeObjectURL(url); toast(tx('WEBP 生成失败','WEBP export failed')); };
    img.src = url;
  } catch (err) {
    toast(err.message || tx('WEBP 生成失败','WEBP export failed'));
  }
}
const localizedAppName = (settings = state.me?.settings || {}) => {
  if (state.locale === 'en') return settings.app_name_en || settings.app_name || 'AI Shortlink';
  return settings.app_name_zh || settings.app_name || 'AI短链平台';
};
function updateAppName() {
  const name = localizedAppName();
  const brand = document.getElementById('brandName');
  if (brand) brand.textContent = name;
}

function setHeader(title, desc, createText = '', showCreate = true) {
  pageTitle.textContent = title;
  pageDesc.textContent = desc;
  createBtn.textContent = createText || t('common.create');
  createBtn.style.display = showCreate ? '' : 'none';
}
function setHeaderForView() {
  if (state.view === 'dashboard') setHeader(t('page.dashboard'), t('page.dashboardDesc'), t('btn.newShort'), true);
  if (state.view === 'shorts') setHeader(t('page.shorts'), t('page.shortsDesc'), t('btn.newShort'), true);
  if (state.view === 'lives') setHeader(t('page.lives'), t('page.livesDesc'), t('btn.newLive'), true);
  if (state.view === 'settings') setHeader(t('page.settings'), t('page.settingsDesc'), '', false);
}
function setView(view) {
  state.view = view;
  localStorage.setItem('asl_view', view);
  document.querySelectorAll('.nav').forEach(b => b.classList.toggle('active', b.dataset.view === view));
  render();
}
function openModal(title, html, sizeClass = '') {
  modalTitle.textContent = title;
  modalBody.innerHTML = html;
  modalCard.className = `modal-card${sizeClass ? ' ' + sizeClass : ''}`;
  modal.hidden = false;
  modalCard.scrollTop = 0;
  enhanceTables(modalBody);
}
function closeModal() {
  modal.hidden = true;
  modalBody.innerHTML = '';
  modalCard.className = 'modal-card';
  state.liveEditor = null;
}
function enhanceTables(root = document) {
  root.querySelectorAll('.table-card table').forEach(table => {
    table.classList.add('responsive-table');
    const headers = [...table.querySelectorAll('thead th')].map(th => th.textContent.trim());
    table.querySelectorAll('tbody tr').forEach(tr => {
      [...tr.children].forEach((td, i) => {
        if (td.tagName === 'TD' && !td.hasAttribute('colspan') && headers[i]) td.dataset.label = headers[i];
      });
    });
  });
}

async function loadMe() {
  const me = await api('/api/admin/me');
  state.me = me;
  baseURL = me.base_url || baseURL;
  app.dataset.baseUrl = baseURL;
  const pref = me.settings?.default_locale || 'auto';
  state.locale = detectLocale(pref);
  applyI18n();
}

async function render() {
  setHeaderForView();
  content.innerHTML = document.getElementById('loadingTpl').innerHTML;
  try {
    if (state.view === 'dashboard') await renderDashboard();
    else if (state.view === 'shorts') await renderShorts();
    else if (state.view === 'lives') await renderLives();
    else if (state.view === 'settings') await renderSettings();
    enhanceTables(content);
  } catch (err) {
    content.innerHTML = `<div class="empty">${esc(err.message)}</div>`;
  }
}
function metric(label, value, extra = '') { return `<div class="card"><h3>${esc(label)}</h3><div class="metric">${Number(value || 0).toLocaleString()}</div>${extra ? `<p class="muted">${esc(extra)}</p>` : ''}</div>`; }

async function renderDashboard() {
  setHeader(t('page.dashboard'), t('page.dashboardDesc'), t('btn.newShort'), true);
  const { data } = await api('/api/admin/overview');
  content.innerHTML = `
    ${accountRecoveryCardHTML()}
    ${dashboardHintsHTML(data)}
    <div class="grid cards">
      ${metric(t('dash.shortLinks'), data.short_links)}
      ${metric(t('dash.liveQRs'), data.live_qrs)}
      ${metric(t('dash.liveItems'), data.live_items_active)}
      ${metric(t('dash.today'), data.visits_today)}
      ${metric(t('dash.total'), data.visits_total)}
    </div>
    <div class="grid cards secondary-cards">
      ${metric(t('dash.pendingShort'), data.short_pending)}
      ${metric(t('dash.pendingLive'), data.live_pending)}
      ${metric(t('dash.pendingItem'), data.live_items_pending)}
    </div>
    <div class="grid dashboard-actions">
      <div class="card"><h2>${t('dash.shortAbility')}</h2><p class="muted">${t('dash.shortAbilityDesc')}</p><button class="primary" id="dashNewShort">${t('dash.createShort')}</button></div>
      <div class="card"><h2>${t('dash.liveAbility')}</h2><p class="muted">${t('dash.liveAbilityDesc')}</p><button class="primary" id="dashNewLive">${t('dash.createLive')}</button></div>
    </div>`;
  document.getElementById('dashNewShort').onclick = () => openShortModal();
  document.getElementById('dashNewLive').onclick = () => openLiveEditor();
  bindAccountRecoveryCard();
}
function dashboardHintsHTML(data) {
  const hints = [];
  if (!state.me?.account?.email) hints.push(t('dash.noEmail'));
  if (!data.smtp_configured) hints.push(t('dash.noSMTP'));
  if (!data.base_url_configured) hints.push(t('dash.noBase'));
  if (Number(data.short_pending || 0) + Number(data.live_pending || 0) + Number(data.live_items_pending || 0) > 0) hints.push(t('dash.hasPending'));
  if (!hints.length) return '';
  return `<div class="card notice-card"><h2>${t('dash.hints')}</h2><ul class="notice-list">${hints.map(h => `<li>${esc(h)}</li>`).join('')}</ul></div>`;
}
function accountRecoveryCardHTML() {
  const keyAvailable = !!state.me?.account?.recovery_key_available || !!state.me?.account?.recovery_key;
  return `<div class="recovery-mini${keyAvailable ? '' : ' warning'}"><div><strong>${t('dash.recoveryTitle')}</strong><p>${keyAvailable ? t('dash.recoveryOK') : t('dash.recoveryWarn')}</p></div><div class="actions"><button class="ghost" id="viewRecoveryKey">${keyAvailable ? t('dash.viewRecovery') : t('dash.rotateRecovery')}</button></div></div>`;
}
function bindAccountRecoveryCard() {
  const btn = document.getElementById('viewRecoveryKey');
  if (btn) btn.onclick = () => openRecoveryKeyModal();
}
function openRecoveryKeyModal() {
  const account = state.me?.account || {};
  const key = account.recovery_key || '';
  if (!key) {
    openModal(t('dash.recoveryTitle'), `<div class="recovery-dialog warning"><p class="muted">${t('dash.recoveryWarn')}</p><div class="form-actions compact"><button class="ghost" data-close="1">${t('common.cancel')}</button><button class="primary" id="rotateRecoveryKey">${t('dash.rotateRecovery')}</button></div></div>`, 'modal-small');
  } else {
    openModal(t('dash.recoveryTitle'), `<div class="recovery-dialog"><p class="muted">${state.locale === 'zh' ? '换设备、清理 Cookie 或长期登录令牌丢失时，使用这个 Key 恢复同一个后台账户。' : 'Use this key to recover the same admin account when device tokens are lost.'}</p><code class="secret-line" id="recoveryKeyText">${esc(key)}</code><div class="form-actions compact"><button class="ghost" id="rotateRecoveryKey">${t('dash.rotateRecovery')}</button><button class="primary" id="copyRecoveryKey">${t('common.copy')}</button></div></div>`, 'modal-small');
    document.getElementById('copyRecoveryKey').onclick = () => copy(key);
  }
  document.getElementById('rotateRecoveryKey').onclick = rotateRecoveryKey;
}
async function rotateRecoveryKey() {
  if (!confirm(state.locale === 'zh' ? '重新生成后，旧恢复 Key 会立即失效。确定继续？' : 'Rotating invalidates the old recovery key. Continue?')) return;
  const data = await api('/api/admin/account/recovery-key/rotate', { method: 'POST', body: '{}' });
  state.me.account = { ...state.me.account, ...data.account };
  toast(state.locale === 'zh' ? '已生成新的恢复 Key，请立即保存' : 'New recovery key generated. Save it now.');
  openRecoveryKeyModal();
  if (state.view === 'dashboard') renderDashboard();
}

function statusBadge(s) {
  const st = s === 'disabled' ? 'disabled' : 'active';
  return `<span class="status ${st}">${st === 'active' ? t('common.enabled') : t('common.disabled')}</span>`;
}
function approvalBadge(s) {
  const v = s || 'pending';
  const label = v === 'approved' ? t('common.approved') : v === 'rejected' ? t('common.rejected') : t('common.pending');
  return `<span class="status review-${esc(v)}">${label}</span>`;
}
function reviewButtons(type, id, current, includeItems = false, itemIndex = null) {
  const cur = current || 'pending';
  const attrs = itemIndex === null ? '' : ` data-item-index="${itemIndex}"`;
  const include = includeItems ? ' data-include-items="1"' : '';
  const prefix = `data-review-${type}="${id}"${attrs}${include}`;
  const out = [];
  if (cur !== 'approved') out.push(`<button class="ghost" ${prefix} data-review-status="approved">${t('common.approve')}</button>`);
  if (cur !== 'rejected') out.push(`<button class="ghost" ${prefix} data-review-status="rejected">${t('common.reject')}</button>`);
  if (cur === 'approved') out.push(`<button class="ghost" ${prefix} data-review-status="pending">${t('common.backPending')}</button>`);
  return out.join('');
}
async function reviewResource(type, id, status, includeItems = false, itemIndex = null) {
  const note = status === 'rejected' ? (prompt(t('msg.needNote')) || '') : '';
  const endpoints = { short: `/api/admin/short-links/${id}/review`, live: `/api/admin/live-qrs/${id}/review`, item: `/api/admin/live-qr-items/${id}` };
  const data = await api(endpoints[type], { method: 'POST', body: JSON.stringify({ status, note, include_items: includeItems }) });
  toast(t('msg.reviewed'));
  if (type === 'item' && state.liveEditor && itemIndex !== null) {
    state.liveEditor.items[Number(itemIndex)] = normalizeEditorItem({ ...state.liveEditor.items[Number(itemIndex)], ...data.data });
    renderLiveEditorItems();
    updateLivePublishSummary();
    return;
  }
  if (type === 'short') renderShorts();
  if (type === 'live') renderLives();
}
function bindReviewButtons() {
  document.querySelectorAll('[data-review-short]').forEach(b => b.onclick = () => reviewResource('short', b.dataset.reviewShort, b.dataset.reviewStatus));
  document.querySelectorAll('[data-review-live]').forEach(b => b.onclick = () => reviewResource('live', b.dataset.reviewLive, b.dataset.reviewStatus, b.dataset.includeItems === '1'));
  document.querySelectorAll('[data-review-item]').forEach(b => b.onclick = () => reviewResource('item', b.dataset.reviewItem, b.dataset.reviewStatus, false, b.dataset.itemIndex));
}

function actionMenuHTML(kind, row) {
  const isShort = kind === 'short';
  const review = reviewButtons(kind, row.id, row.approval_status, !isShort);
  return `<details class="action-menu"><summary aria-label="${tx('更多操作','More actions')}">${tx('更多','More')}</summary><div class="action-menu-panel">
    <div class="action-menu-section"><button class="ghost" data-stats-${kind}="${row.id}" data-title="${esc(row.title || row.code)}">${t('common.stats')}</button></div>
    <div class="action-menu-section">${qrDownloadButtonsHTML(kind, row.code, false)}</div>
    ${review ? `<div class="action-menu-section"><div class="action-menu-title">${t('common.review')}</div>${review}</div>` : ''}
    <div class="action-menu-section danger-section"><button class="danger" data-del-${kind}="${row.id}">${t('common.delete')}</button></div>
  </div></details>`;
}
function rowActionsHTML(kind, row) {
  const isShort = kind === 'short';
  const url = isShort ? publicShort(row.code) : publicLive(row.code);
  const edit = isShort
    ? `<button class="ghost" data-edit-short="${row.id}">${t('common.edit')}</button>`
    : `<button class="ghost" data-edit-live="${row.id}">${t('common.config')}</button>`;
  return `<div class="row-actions">
    <button class="ghost" data-copy="${esc(url)}">${t('common.copy')}</button>
    ${edit}
    ${actionMenuHTML(kind, row)}
  </div>`;
}
function shortRowHTML(row) {
  return `<tr class="resource-row"><td class="resource-title-cell"><strong>${esc(row.title || row.code)}</strong><br><span class="muted">${esc(row.code)}</span></td><td class="resource-link-cell"><div class="copy">${esc(publicShort(row.code))}</div></td><td class="resource-target-cell"><div class="copy" title="${esc(row.target_url)}">${esc(row.target_url)}</div></td><td class="resource-status-cell"><div class="badge-stack">${statusBadge(row.status)}${approvalBadge(row.approval_status)}</div></td><td class="resource-count-cell">${Number(row.visit_count || 0).toLocaleString()}</td><td class="resource-date-cell">${fmtDate(row.updated_at)}</td><td class="action-cell">${rowActionsHTML('short', row)}</td></tr>`;
}
function liveRowHTML(row) {
  return `<tr class="resource-row"><td class="resource-title-cell"><strong>${esc(row.title || row.code)}</strong><br><span class="muted">${esc(row.code)}</span></td><td class="resource-link-cell"><div class="copy">${esc(publicLive(row.code))}</div></td><td class="resource-target-cell">${strategyName(row.rotation_strategy)}</td><td class="resource-status-cell"><div class="badge-stack">${statusBadge(row.status)}${approvalBadge(row.approval_status)}</div></td><td class="resource-count-cell">${Number(row.visit_count || 0).toLocaleString()}</td><td class="resource-date-cell">${fmtDate(row.updated_at)}</td><td class="action-cell">${rowActionsHTML('live', row)}</td></tr>`;
}
function closeActionMenus(root = document) {
  root.querySelectorAll('.action-menu[open]').forEach(menu => { menu.open = false; });
}
function bindActionMenus(root = document) {
  root.querySelectorAll('.action-menu').forEach(menu => {
    menu.ontoggle = () => {
      menu.closest('tr')?.classList.toggle('row-menu-open', menu.open);
      if (!menu.open) return;
      root.querySelectorAll('.action-menu[open]').forEach(other => {
        if (other !== menu) other.open = false;
      });
    };
  });
}
document.addEventListener('click', e => {
  if (!e.target.closest('.action-menu')) closeActionMenus();
});
document.addEventListener('keydown', e => {
  if (e.key === 'Escape') closeActionMenus();
});

async function renderShorts() {
  setHeader(t('page.shorts'), t('page.shortsDesc'), t('btn.newShort'), true);
  const { data } = await api('/api/admin/short-links?limit=100&q=' + encodeURIComponent(state.q || ''));
  content.innerHTML = `<div class="toolbar"><input id="q" placeholder="${t('short.search')}" value="${esc(state.q)}"><button class="ghost" id="searchBtn">${t('common.search')}</button></div><div class="card table-card has-row-menus"><table><thead><tr><th>${t('short.title')}</th><th>${t('short.code')}</th><th>${t('short.target')}</th><th>${t('common.status')} / ${t('common.review')}</th><th>${t('common.visits')}</th><th>${t('common.updated')}</th><th>${t('common.actions')}</th></tr></thead><tbody>${data.map(shortRowHTML).join('') || `<tr><td colspan="7"><div class="empty">${t('short.empty')}</div></td></tr>`}</tbody></table></div>`;
  document.getElementById('searchBtn').onclick = () => { state.q = val('q'); renderShorts(); };
  document.getElementById('q').addEventListener('keydown', e => { if (e.key === 'Enter') { state.q = val('q'); renderShorts(); } });
  bindShortActions(data);
  bindQRDownloadButtons(content);
  bindActionMenus(content);
}
function bindShortActions(rows) {
  document.querySelectorAll('[data-copy]').forEach(b => b.onclick = () => copy(b.dataset.copy));
  document.querySelectorAll('[data-edit-short]').forEach(b => b.onclick = () => openShortModal(rows.find(x => String(x.id) === b.dataset.editShort)));
  document.querySelectorAll('[data-stats-short]').forEach(b => b.onclick = () => openStats('short', b.dataset.statsShort, b.dataset.title));
  document.querySelectorAll('[data-del-short]').forEach(b => b.onclick = async () => {
    if (!confirm(state.locale === 'zh' ? '确定删除这个短链？访问记录会保留，但短链配置将删除。' : 'Delete this short link? Visit logs remain.')) return;
    await api('/api/admin/short-links/' + b.dataset.delShort, { method: 'DELETE' });
    toast(t('msg.deleted')); renderShorts();
  });
  bindReviewButtons();
}
function openShortModal(row = null) {
  const previewContent = () => val('slCode') ? publicShort(val('slCode')) : (val('slTarget') || `${baseURL}/s/preview`);
  openModal(row ? t('short.modalEdit') : t('short.modalNew'), `<div class="form-grid"><label class="field"><span>${t('short.title')}</span><input id="slTitle" value="${esc(row?.title || '')}" placeholder="618 landing"></label><label class="field"><span>${t('short.customCode')}</span><input id="slCode" value="${esc(row?.code || '')}" placeholder="${state.locale === 'zh' ? '留空自动生成' : 'Auto generated if empty'}"></label><label class="field wide"><span>${t('short.targetURL')}</span><input id="slTarget" value="${esc(row?.target_url || '')}" placeholder="https://example.com/landing"></label><label class="field"><span>${t('common.status')}</span><select id="slStatus"><option value="active">${t('common.enabled')}</option><option value="disabled">${t('common.disabled')}</option></select></label><label class="field"><span>${t('short.redirect')}</span><select id="slRedirect"><option value="302">302</option><option value="301">301</option><option value="307">307</option><option value="308">308</option></select></label><label class="field"><span>${t('short.starts')}</span><input id="slStarts" type="datetime-local" value="${fmtInputDate(row?.starts_at)}"></label><label class="field"><span>${t('short.expires')}</span><input id="slExpires" type="datetime-local" value="${fmtInputDate(row?.expires_at)}"></label><label class="field"><span>${t('short.max')}</span><input id="slMax" type="number" min="0" value="${row?.max_visits || 0}"></label><label class="field"><span>${t('short.fallback')}</span><input id="slFallback" value="${esc(row?.fallback_url || '')}" placeholder="https://..."></label><label class="field wide"><span>${t('short.remark')}</span><textarea id="slRemark">${esc(row?.remark || '')}</textarea></label><div class="wide">${qrDesignerHTML('sl', row || {}, previewContent())}</div><p class="muted wide">${t('short.pendingTip')}</p></div><div class="form-actions"><button class="ghost" data-close="1">${t('common.cancel')}</button><button class="primary" id="saveShort">${t('common.save')}</button></div>`);
  document.getElementById('slStatus').value = row?.status || 'active';
  document.getElementById('slRedirect').value = String(row?.redirect_type || 302);
  document.getElementById('slQRStyle').value = row?.qr_style || 'rounded';
  ['slCode','slTarget'].forEach(id => document.getElementById(id)?.addEventListener('input', () => updateQRPreview('sl', previewContent())));
  bindQRDesigner('sl', previewContent);
  document.getElementById('saveShort').onclick = () => submitShort(row?.id);
}
async function submitShort(id) {
  const starts = val('slStarts'), expires = val('slExpires');
  if (starts && expires && new Date(starts) >= new Date(expires)) return toast(state.locale === 'zh' ? '过期时间必须晚于开始时间' : 'Expiry must be later than start time');
  if (num('slMax') < 0) return toast(state.locale === 'zh' ? '访问上限不能为负数' : 'Visit limit cannot be negative');
  const payload = { title: val('slTitle'), code: val('slCode'), target_url: val('slTarget'), status: val('slStatus'), redirect_type: num('slRedirect'), starts_at: starts, expires_at: expires, max_visits: num('slMax'), fallback_url: val('slFallback'), remark: val('slRemark'), ...qrConfigFrom('sl') };
  const path = id ? `/api/admin/short-links/${id}` : '/api/admin/short-links';
  await api(path, { method: id ? 'PUT' : 'POST', body: JSON.stringify(payload) });
  closeModal(); toast(t('msg.saved')); renderShorts();
}

async function renderLives() {
  setHeader(t('page.lives'), t('page.livesDesc'), t('btn.newLive'), true);
  const { data } = await api('/api/admin/live-qrs?limit=100&q=' + encodeURIComponent(state.q || ''));
  content.innerHTML = `<div class="toolbar"><input id="q" placeholder="${t('live.search')}" value="${esc(state.q)}"><button class="ghost" id="searchBtn">${t('common.search')}</button></div><div class="card table-card has-row-menus"><table><thead><tr><th>${t('live.title')}</th><th>${t('live.link')}</th><th>${t('live.strategy')}</th><th>${t('common.status')} / ${t('common.review')}</th><th>${t('common.visits')}</th><th>${t('common.updated')}</th><th>${t('common.actions')}</th></tr></thead><tbody>${data.map(liveRowHTML).join('') || `<tr><td colspan="7"><div class="empty">${t('live.empty')}</div></td></tr>`}</tbody></table></div>`;
  document.getElementById('searchBtn').onclick = () => { state.q = val('q'); renderLives(); };
  document.getElementById('q').addEventListener('keydown', e => { if (e.key === 'Enter') { state.q = val('q'); renderLives(); } });
  bindLiveActions(data);
  bindQRDownloadButtons(content);
  bindActionMenus(content);
}
function strategyName(s) { return t('strategy.' + (s || 'round_robin'), s); }
function bindLiveActions(rows) {
  document.querySelectorAll('[data-copy]').forEach(b => b.onclick = () => copy(b.dataset.copy));
  document.querySelectorAll('[data-edit-live]').forEach(b => b.onclick = () => openLiveEditor(rows.find(x => String(x.id) === b.dataset.editLive)));
  document.querySelectorAll('[data-stats-live]').forEach(b => b.onclick = () => openStats('live', b.dataset.statsLive, b.dataset.title));
  document.querySelectorAll('[data-del-live]').forEach(b => b.onclick = async () => {
    if (!confirm(state.locale === 'zh' ? '确定删除这个活码？它下面的二维码组也会删除。' : 'Delete this live QR and all its items?')) return;
    await api('/api/admin/live-qrs/' + b.dataset.delLive, { method: 'DELETE' });
    toast(t('msg.deleted')); renderLives();
  });
  bindReviewButtons();
}

function defaultLiveData() {
  return { id: null, title: '', code: '', status: 'active', approval_status: 'pending', rotation_strategy: 'round_robin', description: '', guide_title: state.locale === 'zh' ? '长按识别二维码' : 'Long press to scan QR', guide_text: state.locale === 'zh' ? '请长按下方二维码图片，选择“识别图中二维码”完成添加或访问。' : 'Long press the QR image below and choose scan or recognize QR.', fallback_url: '', qr_style: 'rounded', qr_foreground: '#111827', qr_background: '#ffffff', qr_logo_url: '', items: [] };
}
function normalizeEditorItem(it = {}) {
  return { id: it.id || null, title: it.title || '', qr_image_url: it.qr_image_url || '', target_url: it.target_url || '', status: it.status || 'active', approval_status: it.approval_status || 'pending', review_note: it.review_note || '', starts_at: fmtInputDate(it.starts_at), expires_at: fmtInputDate(it.expires_at), max_views: Number(it.max_views || 0), view_count: Number(it.view_count || 0), sort_order: Number(it.sort_order || 100), weight: Number(it.weight || 1) };
}
async function openLiveEditor(row = null) {
  let data = defaultLiveData();
  if (row?.id) {
    const res = await api(`/api/admin/live-qrs/${row.id}`);
    data = { ...data, ...res.data, items: res.data.items || [] };
  }
  state.liveEditor = { id: data.id || null, approvalStatus: data.approval_status || 'pending', items: (data.items || []).map(normalizeEditorItem), deletedItemIds: [], activeTab: 'base' };
  openModal(data.id ? `${t('live.modalEdit')} · ${data.title || data.code}` : t('live.modalNew'), liveEditorHTML(data), 'live-editor-modal');
  document.getElementById('lStatus').value = data.status || 'active';
  document.getElementById('lStrategy').value = data.rotation_strategy || 'round_robin';
  document.getElementById('lQRStyle').value = data.qr_style || 'rounded';
  bindLiveEditor();
  renderLiveEditorItems();
  updateLiveLinkBlocks();
  updateLivePoolSummary();
  updateLivePublishSummary();
}
function liveEditorHTML(data) {
  const link = data.code ? publicLive(data.code) : '';
  const previewContent = link || `${baseURL}/q/preview`;
  return `<div class="live-editor">
    <div class="tabs live-tabs"><button class="tab active" data-live-tab="base">${tx('1. 基础与入口','1. Entry setup')}</button><button class="tab" data-live-tab="items">${tx('2. 二维码池','2. QR pool')}</button><button class="tab" data-live-tab="publish">${tx('3. 发布与下载','3. Publish')}</button></div>
    <section class="tab-panel" id="liveTab-base">
      <div class="live-base-layout">
        <div class="card live-config-card">
          <div class="section-title"><h3>${tx('活码配置','Live QR configuration')}</h3><p class="muted">${tx('先确定入口、轮换策略和访客看到的引导文案。','Set the public entry, rotation policy and visitor guidance first.')}</p></div>
          <div class="form-grid editor-grid compact-form-grid"><label class="field"><span>${t('live.title')}</span><input id="lTitle" value="${esc(data.title || '')}" placeholder="WeChat group live QR"></label><label class="field"><span>${t('short.customCode')}</span><input id="lCode" value="${esc(data.code || '')}" placeholder="${state.locale === 'zh' ? '留空自动生成' : 'Auto generated if empty'}"></label><label class="field"><span>${t('common.status')}</span><select id="lStatus"><option value="active">${t('common.enabled')}</option><option value="disabled">${t('common.disabled')}</option></select></label><label class="field"><span>${t('live.strategy')}</span><select id="lStrategy"><option value="round_robin">${t('strategy.round_robin')}</option><option value="random">${t('strategy.random')}</option><option value="least_used">${t('strategy.least_used')}</option></select></label><label class="field wide"><span>${t('live.description')}</span><textarea id="lDesc">${esc(data.description || '')}</textarea></label><label class="field"><span>${t('live.guideTitle')}</span><input id="lGuideTitle" value="${esc(data.guide_title || defaultLiveData().guide_title)}"></label><label class="field"><span>${t('live.fallback')}</span><input id="lFallback" value="${esc(data.fallback_url || '')}" placeholder="https://..."></label><label class="field wide"><span>${t('live.guideText')}</span><textarea id="lGuideText">${esc(data.guide_text || defaultLiveData().guide_text)}</textarea></label></div>
        </div>
        <div class="card live-entry-card">
          <div class="live-link-box" id="liveLinkBox">${link ? linkBoxHTML(link, data.code) : unsavedLinkHTML()}</div>
          ${qrDesignerHTML('l', data, previewContent)}
        </div>
      </div>
    </section>
    <section class="tab-panel" id="liveTab-items" hidden>
      <div class="live-pool-summary" id="livePoolSummary"></div>
      <div class="editor-split live-pool-layout">
        <div class="card item-form-card">
          <div class="section-title"><h3>${t('live.itemConfig')}</h3><p class="muted">${t('live.itemHint')}</p></div>
          <input type="hidden" id="itIndex">
          <div class="item-image-uploader">
            <div class="item-image-preview" id="itImagePreview">${tx('预览','Preview')}</div>
            <div>
              <label class="field"><span>${t('live.itemImage')}</span><input id="itImage" placeholder="/uploads/... / https://..."></label>
              ${fileUploadHTML('itFile', t('live.upload'), tx('PNG / JPG / WEBP','PNG / JPG / WEBP'))}
            </div>
          </div>
          <label class="field"><span>${t('live.itemTitle')}</span><input id="itTitle" placeholder="Group 1 / Support A"></label>
          <label class="field"><span>${t('live.itemTarget')}</span><input id="itTarget" placeholder="https://..."></label>
          <div class="mini-grid"><label class="field"><span>${t('common.status')}</span><select id="itStatus"><option value="active">${t('common.enabled')}</option><option value="disabled">${t('common.disabled')}</option></select></label><label class="field"><span>${t('live.sort')}</span><input id="itSort" type="number" value="100"></label><label class="field"><span>${t('short.starts')}</span><input id="itStarts" type="datetime-local"></label><label class="field"><span>${t('short.expires')}</span><input id="itExpires" type="datetime-local"></label><label class="field"><span>${t('live.maxViews')}</span><input id="itMax" type="number" min="0" value="0"></label><label class="field"><span>${t('live.weight')}</span><input id="itWeight" type="number" min="1" value="1"></label></div>
          <div class="actions item-form-actions"><button class="ghost" id="resetItem">${t('live.clear')}</button><button class="primary" id="saveDraftItem">${t('live.addItem')}</button></div>
        </div>
        <div class="card item-list-card"><div class="section-title"><h3>${tx('二维码池','QR pool')}</h3><p class="muted">${tx('按排序从小到大展示；保存前均为草稿变更。','Sorted ascending. Changes stay in draft until saved.')}</p></div><div class="item-card-list" id="itemsGrid"></div></div>
      </div>
    </section>
    <section class="tab-panel" id="liveTab-publish" hidden><div id="livePublishSummary" class="publish-summary"></div></section>
  </div><div class="form-actions live-editor-actions"><button class="ghost" data-close="1">${t('common.close')}</button><button class="ghost" id="saveLiveClose">${t('live.saveClose')}</button><button class="primary" id="saveLiveBundle">${t('live.saveAll')}</button></div>`;
}
function bindLiveEditor() {
  document.querySelectorAll('[data-live-tab]').forEach(btn => btn.onclick = () => setLiveEditorTab(btn.dataset.liveTab));
  const itemFile = document.getElementById('itFile');
  bindFilePickerName(itemFile);
  itemFile.onchange = uploadSelectedImage;
  document.getElementById('resetItem').onclick = resetItemForm;
  document.getElementById('saveDraftItem').onclick = saveDraftItem;
  document.getElementById('saveLiveBundle').onclick = () => saveLiveBundle(false);
  document.getElementById('saveLiveClose').onclick = () => saveLiveBundle(true);
  ['lTitle','lCode','lStatus','lStrategy','lFallback'].forEach(id => document.getElementById(id)?.addEventListener('input', updateLivePublishSummary));
  document.getElementById('lCode')?.addEventListener('input', updateLiveLinkBlocks);
  document.getElementById('itImage')?.addEventListener('input', updateItemImagePreview);
  bindQRDesigner('l', () => val('lCode') ? publicLive(val('lCode')) : `${baseURL}/q/preview`);
}
function setLiveEditorTab(tab) {
  state.liveEditor.activeTab = tab;
  document.querySelectorAll('[data-live-tab]').forEach(btn => btn.classList.toggle('active', btn.dataset.liveTab === tab));
  document.querySelectorAll('.tab-panel').forEach(panel => panel.hidden = panel.id !== `liveTab-${tab}`);
  if (tab === 'publish') updateLivePublishSummary();
}
function linkBoxHTML(link, code) { return `<div><strong>${t('live.link')}</strong><p>${esc(link)}</p></div><div class="actions"><button class="ghost" id="copyLiveLink">${t('common.copy')}</button>${qrDownloadButtonsHTML('live', code, true)}</div>`; }
function unsavedLinkHTML() { return `<div><strong>${t('live.link')}</strong><p>${t('live.unsavedLink')}</p></div>`; }
function updateLiveLinkBlocks() {
  const code = val('lCode');
  const box = document.getElementById('liveLinkBox');
  if (box) {
    box.innerHTML = code ? linkBoxHTML(publicLive(code), code) : unsavedLinkHTML();
    const copyBtn = document.getElementById('copyLiveLink');
    if (copyBtn) copyBtn.onclick = () => copy(publicLive(code));
    bindQRDownloadButtons(box);
  }
  updateQRPreview('l', code ? publicLive(code) : `${baseURL}/q/preview`);
}
function livePayloadFromForm() { return { title: val('lTitle'), code: val('lCode'), description: val('lDesc'), status: val('lStatus'), rotation_strategy: val('lStrategy'), guide_title: val('lGuideTitle'), guide_text: val('lGuideText'), fallback_url: val('lFallback'), ...qrConfigFrom('l') }; }
function itemFromForm() {
  const starts = val('itStarts'), expires = val('itExpires'), maxViews = num('itMax'), weight = num('itWeight') || 1;
  if (!val('itImage')) throw new Error(state.locale === 'zh' ? '请先上传或填写二维码图片' : 'Upload or fill a QR image first');
  if (starts && expires && new Date(starts) >= new Date(expires)) throw new Error(state.locale === 'zh' ? '二维码过期时间必须晚于开始时间' : 'QR expiry must be later than start');
  if (maxViews < 0) throw new Error(state.locale === 'zh' ? '展示上限不能为负数' : 'View limit cannot be negative');
  if (weight < 1) throw new Error(state.locale === 'zh' ? '权重不能小于 1' : 'Weight must be at least 1');
  return { title: val('itTitle'), qr_image_url: val('itImage'), target_url: val('itTarget'), status: val('itStatus') || 'active', sort_order: num('itSort') || 100, starts_at: starts, expires_at: expires, max_views: maxViews, weight };
}
function saveDraftItem() {
  try {
    const idxValue = val('itIndex');
    const item = itemFromForm();
    if (idxValue !== '') {
      const idx = Number(idxValue);
      const current = state.liveEditor.items[idx];
      state.liveEditor.items[idx] = { ...current, ...item, approval_status: current?.id ? 'pending' : 'pending' };
      toast(state.locale === 'zh' ? '二维码已更新到待保存列表' : 'QR item updated in draft list');
    } else {
      state.liveEditor.items.push({ id: null, view_count: 0, approval_status: 'pending', ...item });
      toast(state.locale === 'zh' ? '二维码已加入待保存列表' : 'QR item added to draft list');
    }
    resetItemForm(); renderLiveEditorItems(); updateLivePublishSummary();
  } catch (err) { toast(err.message); }
}
function resetItemForm() {
  ['itIndex','itTitle','itImage','itTarget','itStarts','itExpires'].forEach(id => { const el = document.getElementById(id); if (el) el.value = ''; });
  document.getElementById('itStatus').value = 'active';
  document.getElementById('itSort').value = 100;
  document.getElementById('itMax').value = 0;
  document.getElementById('itWeight').value = 1;
  const itemFile = document.getElementById('itFile');
  itemFile.value = '';
  resetFilePickerName(itemFile);
  updateItemImagePreview();
}
async function uploadSelectedImage(e) {
  const file = e.target.files?.[0];
  if (!file) return;
  await uploadImageInto(e, 'itImage', updateItemImagePreview);
}
async function uploadImageInto(e, targetID, afterUpload) {
  const file = e.target.files?.[0];
  if (!file) return;
  const fd = new FormData();
  fd.append('file', file);
  const data = await api('/api/admin/uploads/images', { method: 'POST', body: fd });
  document.getElementById(targetID).value = data.url;
  if (typeof afterUpload === 'function') afterUpload();
  toast(state.locale === 'zh' ? '图片已上传' : 'Image uploaded');
}
function sortedEditorItems() { return state.liveEditor.items.map((it, index) => ({ it, index })).sort((a, b) => (Number(a.it.sort_order || 100) - Number(b.it.sort_order || 100)) || ((a.it.id || 0) - (b.it.id || 0)) || (a.index - b.index)); }
function renderLiveEditorItems() {
  const grid = document.getElementById('itemsGrid');
  if (!grid || !state.liveEditor) return;
  const rows = sortedEditorItems();
  grid.innerHTML = rows.map(({ it, index }) => {
    const title = it.title || tx('未命名二维码','Untitled QR');
    const validity = `${fmtDate(it.starts_at)} → ${fmtDate(it.expires_at)}`;
    const limit = `${Number(it.view_count || 0).toLocaleString()} / ${it.max_views ? Number(it.max_views).toLocaleString() : tx('不限','Unlimited')}`;
    return `<article class="item-card">
      <div class="item-card-img">${it.qr_image_url ? `<img src="${esc(it.qr_image_url)}" alt="${esc(title)}">` : `<span>${tx('未上传','No image')}</span>`}</div>
      <div class="item-card-main"><div class="item-card-head"><strong>${esc(title)}</strong><div class="badge-stack">${statusBadge(it.status)}${approvalBadge(it.approval_status)}</div></div><p class="muted item-card-subline">${t('live.sort')} ${Number(it.sort_order || 100)} · ${it.id ? t('live.saved') : t('live.draft')} · ${tx('权重','Weight')} ${Number(it.weight || 1)}</p>${it.target_url ? `<p class="copy">${esc(it.target_url)}</p>` : ''}${it.review_note ? `<p class="muted">${esc(it.review_note)}</p>` : ''}<div class="item-card-footer"><div class="item-card-meta"><span>${t('live.validity')}: ${esc(validity)}</span><span>${t('live.views')}: ${esc(limit)}</span></div><div class="item-card-actions"><button class="ghost" data-edit-draft-item="${index}">${t('common.edit')}</button>${it.id ? reviewButtons('item', it.id, it.approval_status, false, index) : ''}<button class="danger" data-del-draft-item="${index}">${tx('移除','Remove')}</button></div></div></div>
    </article>`;
  }).join('') || `<div class="empty">${t('live.noItems')}</div>`;
  document.querySelectorAll('[data-edit-draft-item]').forEach(b => b.onclick = () => fillDraftItem(Number(b.dataset.editDraftItem)));
  document.querySelectorAll('[data-del-draft-item]').forEach(b => b.onclick = () => deleteDraftItem(Number(b.dataset.delDraftItem)));
  bindReviewButtons();
  updateLivePoolSummary();
}
function updateItemImagePreview() {
  const box = document.getElementById('itImagePreview');
  if (!box) return;
  const src = val('itImage');
  box.innerHTML = src ? `<img src="${esc(src)}" alt="">` : tx('预览','Preview');
}
function updateLivePoolSummary() {
  const box = document.getElementById('livePoolSummary');
  if (!box || !state.liveEditor) return;
  const items = state.liveEditor.items || [];
  const active = items.filter(it => it.status === 'active').length;
  const approved = items.filter(it => it.approval_status === 'approved').length;
  const limited = items.filter(it => Number(it.max_views || 0) > 0).length;
  box.innerHTML = `<div class="pool-metric"><strong>${items.length}</strong><span>${t('live.items')}</span></div><div class="pool-metric"><strong>${active}</strong><span>${t('common.enabled')}</span></div><div class="pool-metric"><strong>${approved}</strong><span>${t('common.approved')}</span></div><div class="pool-metric"><strong>${limited}</strong><span>${tx('有限展示','Limited')}</span></div>`;
}
function fillDraftItem(index) {
  const it = state.liveEditor.items[index];
  if (!it) return;
  document.getElementById('itIndex').value = String(index);
  document.getElementById('itTitle').value = it.title || '';
  document.getElementById('itImage').value = it.qr_image_url || '';
  document.getElementById('itTarget').value = it.target_url || '';
  document.getElementById('itStatus').value = it.status || 'active';
  document.getElementById('itSort').value = it.sort_order || 100;
  document.getElementById('itStarts').value = it.starts_at || '';
  document.getElementById('itExpires').value = it.expires_at || '';
  document.getElementById('itMax').value = it.max_views || 0;
  document.getElementById('itWeight').value = it.weight || 1;
  updateItemImagePreview();
  setLiveEditorTab('items');
  toast(state.locale === 'zh' ? '已载入到左侧表单' : 'Loaded into form');
}
function deleteDraftItem(index) {
  const it = state.liveEditor.items[index];
  if (!it) return;
  if (it.id && !confirm(state.locale === 'zh' ? '保存后将删除这张已保存的二维码。确定移除？' : 'This saved QR will be deleted after saving. Continue?')) return;
  if (it.id) state.liveEditor.deletedItemIds.push(it.id);
  state.liveEditor.items.splice(index, 1);
  resetItemForm(); renderLiveEditorItems(); updateLivePublishSummary();
  toast(it.id ? (state.locale === 'zh' ? '已加入删除队列，保存后生效' : 'Added to delete queue') : (state.locale === 'zh' ? '已移除' : 'Removed'));
}
function validateEditorItems() {
  for (const it of state.liveEditor.items) {
    const name = it.title || (state.locale === 'zh' ? '未命名' : 'Untitled');
    if (!it.qr_image_url) throw new Error(`${state.locale === 'zh' ? '二维码' : 'QR'}「${name}」${state.locale === 'zh' ? '缺少图片' : 'has no image'}`);
    if (it.starts_at && it.expires_at && new Date(it.starts_at) >= new Date(it.expires_at)) throw new Error(`${name}: ${state.locale === 'zh' ? '过期时间必须晚于开始时间' : 'expiry must be later than start'}`);
    if (Number(it.max_views || 0) < 0) throw new Error(`${name}: ${state.locale === 'zh' ? '展示上限不能为负数' : 'view limit cannot be negative'}`);
    if (Number(it.weight || 1) < 1) throw new Error(`${name}: ${state.locale === 'zh' ? '权重不能小于 1' : 'weight must be at least 1'}`);
  }
}
function itemPayloadForAPI(it) { return { title: it.title || '', qr_image_url: it.qr_image_url || '', target_url: it.target_url || '', status: it.status || 'active', starts_at: it.starts_at || '', expires_at: it.expires_at || '', max_views: Number(it.max_views || 0), sort_order: Number(it.sort_order || 100), weight: Number(it.weight || 1) }; }
async function saveLiveBundle(closeAfter) {
  const editor = state.liveEditor;
  if (!editor) return;
  try {
    validateEditorItems();
    const bundle = { live: livePayloadFromForm(), items: editor.items.map(item => ({ id: item.id || 0, ...itemPayloadForAPI(item) })), delete_item_ids: [...new Set(editor.deletedItemIds)] };
    const path = editor.id ? `/api/admin/live-qrs/${editor.id}/bundle` : '/api/admin/live-qrs/bundle';
    const saved = await api(path, { method: editor.id ? 'PUT' : 'POST', body: JSON.stringify(bundle) });
    const live = saved.data;
    editor.id = live.id;
    editor.approvalStatus = live.approval_status || 'pending';
    editor.deletedItemIds = [];
    document.getElementById('lCode').value = live.code;
    editor.items = (live.items || []).map(normalizeEditorItem);
    modalTitle.textContent = `${t('live.modalEdit')} · ${live.title || live.code}`;
    renderLiveEditorItems(); updateLiveLinkBlocks(); updateLivePublishSummary();
    toast(t('msg.saved'));
    if (state.view === 'lives') renderLives().catch(() => {});
    if (closeAfter) closeModal(); else setLiveEditorTab('publish');
  } catch (err) { toast(err.message || '保存失败'); }
}
function updateLivePublishSummary() {
  const wrap = document.getElementById('livePublishSummary');
  if (!wrap || !state.liveEditor) return;
  const payload = livePayloadFromForm();
  const code = payload.code;
  const link = code ? publicLive(code) : '';
  const items = state.liveEditor.items || [];
  const activeItems = items.filter(x => x.status === 'active').length;
  const approvedItems = items.filter(x => x.approval_status === 'approved').length;
  const needsApproval = state.liveEditor.approvalStatus !== 'approved' || approvedItems < items.length;
  const cfg = payload;
  wrap.innerHTML = `<div class="grid publish-grid"><div class="card"><h3>${t('live.title')}</h3><div class="publish-value">${esc(payload.title || tx('未命名活码','Untitled'))}</div></div><div class="card"><h3>${t('live.strategy')}</h3><div class="publish-value">${strategyName(payload.rotation_strategy)}</div></div><div class="card"><h3>${t('live.items')}</h3><div class="publish-value">${items.length} ${state.locale === 'zh' ? '张' : ''}</div><p class="muted">${t('common.enabled')} ${activeItems} · ${t('common.approved')} ${approvedItems}</p></div><div class="card"><h3>${t('common.review')}</h3><div class="publish-value">${approvalBadge(state.liveEditor.approvalStatus)}</div></div></div><div class="card publish-link-card"><div class="publish-layout"><div><h2>${t('live.publishEntry')}</h2>${link ? `<p class="copy publish-link">${esc(link)}</p><div class="actions"><button class="primary" id="copyPublishLive">${t('common.copy')}</button>${qrDownloadButtonsHTML('live', code, false)}</div>` : `<p class="muted">${t('live.noCode')}</p>`}${needsApproval ? `<p class="muted warn-text">${t('live.approvalWarn')}</p>` : ''}${items.length ? '' : `<p class="muted warn-text">${t('live.noItemWarn')}</p>`}</div><div class="publish-qr-preview"><img src="${esc(qrPreviewPath(link || `${baseURL}/q/preview`, cfg))}" alt="${esc(t('live.qr'))}"><span>${esc(cfg.qr_style || 'rounded')}</span></div></div></div>`;
  const copyBtn = document.getElementById('copyPublishLive');
  if (copyBtn) copyBtn.onclick = () => copy(link);
  bindQRDownloadButtons(wrap);
}

async function renderSettings() {
  setHeader(t('page.settings'), t('page.settingsDesc'), '', false);
  const res = await api('/api/admin/settings');
  const st = res.data || {};
  state.me.settings = st;
  const smtpPasswordState = st.smtp_password_set ? t('settings.smtpSet') : t('settings.smtpUnset');
  content.innerHTML = `
    <div class="settings-layout">
      <div class="card settings-card account-card">
        <h2>${t('settings.account')}</h2>
        <p class="muted">${t('settings.accountDesc')}</p>
        <label class="field"><span>${t('settings.email')}</span><input id="accountEmail" type="email" value="${esc(state.me?.account?.email || st.admin_email || '')}" placeholder="admin@example.com"></label>
        <label class="field"><span>${t('settings.name')}</span><input id="accountName" value="${esc(state.me?.account?.name || '')}" placeholder="Admin"></label>
        <p class="message" id="accountMsg" role="status"></p>
        <button class="primary" id="saveAccount">${t('settings.saveAccount')}</button>
      </div>
      <div class="card settings-card">
        <div class="settings-section-head">
          <h2>${t('settings.system')}</h2>
          <button class="primary" data-save-settings="1">${t('settings.saveSettings')}</button>
        </div>
        <p class="message" id="settingsMsg" role="status"></p>
        <div class="form-grid editor-grid">
          <label class="field"><span>${t('settings.appNameZH')}</span><input id="setAppNameZH" value="${esc(st.app_name_zh || st.app_name || 'AI短链平台')}" placeholder="AI短链平台"></label>
          <label class="field"><span>${t('settings.appNameEN')}</span><input id="setAppNameEN" value="${esc(st.app_name_en || 'AI Shortlink')}" placeholder="AI Shortlink"></label>
          <p class="muted wide">${t('settings.brandI18nHint')}</p>
          <label class="field"><span>${t('settings.locale')}</span><select id="setLocale"><option value="auto">${t('settings.autoLocale')}</option><option value="zh-CN">中文</option><option value="en-US">English</option></select></label>
          <label class="field wide"><span>${t('settings.baseUrl')}</span><input id="setBaseURL" value="${esc(st.base_url || '')}" placeholder="https://s.example.com"></label>
          <label class="field"><span>${t('settings.loginMode')}</span><select id="setLoginMode"><option value="hybrid">${t('settings.hybrid')}</option><option value="magic">${t('settings.magic')}</option><option value="one_click">${t('settings.oneClick')}</option></select></label>
          <label class="field"><span>${t('settings.database')}</span><input value="${esc(res.database_mode || '')}" disabled></label>
        </div>
      </div>
      <div class="card settings-card wide-card smtp-card">
        <div class="settings-section-head smtp-head">
          <div>
            <h2>${t('settings.smtp')}</h2>
            <p class="muted">${t('settings.smtpDeliverabilityHint')}</p>
          </div>
          <button class="primary" data-save-settings="1">${t('settings.saveSettings')}</button>
        </div>
        <div class="smtp-toolbar">
          <label class="checkline smtp-enable"><input id="setSMTPEnabled" type="checkbox"> <span>${t('settings.smtpEnabled')}</span></label>
          <span class="smtp-state">${smtpPasswordState}</span>
        </div>
        <div class="form-grid editor-grid smtp-grid">
          <label class="field smtp-span-8"><span>${t('settings.smtpHost')}</span><input id="setSMTPHost" value="${esc(st.smtp_host || '')}"></label>
          <label class="field smtp-span-4"><span>${t('settings.smtpPort')}</span><input id="setSMTPPort" type="number" value="${Number(st.smtp_port || 465)}"></label>
          <label class="field smtp-span-6"><span>${t('settings.smtpUsername')}</span><input id="setSMTPUsername" value="${esc(st.smtp_username || '')}"></label>
          <label class="field smtp-span-6"><span>${t('settings.smtpFrom')}</span><input id="setSMTPFrom" type="email" value="${esc(st.smtp_from || '')}"></label>
          <label class="field smtp-span-4"><span>${t('settings.smtpSecurity')}</span><select id="setSMTPSecurity"><option value="tls">TLS/SSL</option><option value="starttls">STARTTLS</option><option value="plain">Plain</option></select></label>
          <label class="field smtp-span-8"><span>${t('settings.smtpPassword')}</span><input id="setSMTPPassword" type="password" placeholder="${t('settings.smtpPasswordHint')}"></label>
        </div>
      </div>
    </div>`;
  document.getElementById('setLocale').value = st.default_locale || 'auto';
  document.getElementById('setLoginMode').value = st.login_mode || 'hybrid';
  document.getElementById('setSMTPSecurity').value = st.smtp_security || 'tls';
  document.getElementById('setSMTPEnabled').checked = !!st.smtp_enabled;
  document.getElementById('saveAccount').onclick = e => saveAccount(e.currentTarget);
  document.querySelectorAll('[data-save-settings]').forEach(btn => btn.onclick = () => saveSettings());
}
async function saveAccount(button) {
  setInlineMessage('accountMsg');
  setBusy(button, true);
  try {
    const data = await api('/api/admin/account', { method: 'PUT', body: JSON.stringify({ email: val('accountEmail'), name: val('accountName') }) });
    state.me.account = { ...state.me.account, ...data.account };
    toast(t('msg.accountSaved'));
    await renderSettings();
  } catch (err) {
    setInlineMessage('accountMsg', err.message, 'error');
    toast(err.message, 'error');
  } finally {
    setBusy(button, false);
  }
}
async function saveSettings() {
  const buttons = [...document.querySelectorAll('[data-save-settings]')];
  setInlineMessage('settingsMsg');
  setBusy(buttons, true);
  const payload = { app_name_zh: val('setAppNameZH'), app_name_en: val('setAppNameEN'), base_url: val('setBaseURL'), default_locale: val('setLocale'), login_mode: val('setLoginMode'), smtp_enabled: checked('setSMTPEnabled'), smtp_host: val('setSMTPHost'), smtp_port: num('setSMTPPort'), smtp_security: val('setSMTPSecurity'), smtp_username: val('setSMTPUsername'), smtp_password: rawVal('setSMTPPassword'), smtp_from: val('setSMTPFrom') };
  payload.app_name = payload.app_name_zh || payload.app_name_en;
  const previousLocale = state.me?.settings?.default_locale || 'auto';
  try {
    if (payload.login_mode === 'magic' && (!payload.smtp_enabled || !payload.smtp_host || !payload.smtp_from || payload.smtp_port <= 0)) {
      throw new Error(state.locale === 'zh' ? '仅 Magic Link 登录需要先完整启用 SMTP。' : 'Magic Link only requires SMTP to be enabled and complete.');
    }
    const data = await api('/api/admin/settings', { method: 'PUT', body: JSON.stringify(payload) });
    if (payload.default_locale !== previousLocale) localStorage.removeItem('asl_lang');
    state.me.settings = data.data;
    baseURL = data.data.base_url || location.origin;
    state.locale = detectLocale(data.data.default_locale || 'auto');
    applyI18n();
    toast(t('msg.settingsSaved'));
    await renderSettings();
  } catch (err) {
    setInlineMessage('settingsMsg', err.message, 'error');
    toast(err.message, 'error');
  } finally {
    setBusy(buttons, false);
  }
}

async function openStats(type, id, title) {
  const resource = type === 'short' ? 'short-links' : 'live-qrs';
  const { data } = await api(`/api/admin/${resource}/${id}/stats?days=30`);
  openModal(`${t('common.stats')} · ${title}`, `<div class="form-grid"><div class="card"><h3>30 ${state.locale === 'zh' ? '日访问' : 'days'}</h3><div class="metric">${Number(data.total || 0).toLocaleString()}</div></div><div class="card"><h3>${state.locale === 'zh' ? '独立 IP' : 'Unique IP'}</h3><div class="metric">${Number(data.unique_ips || 0).toLocaleString()}</div></div></div><div class="stat-layout" style="padding: 0 24px 24px"><div class="card"><h2>${state.locale === 'zh' ? '按日期' : 'By date'}</h2>${bars(data.by_date || [], 'date')}</div><div class="card"><h2>${state.locale === 'zh' ? '设备 / 浏览器' : 'Device / browser'}</h2><h3>${state.locale === 'zh' ? '设备' : 'Device'}</h3>${bars(data.by_device || [], 'name')}<h3 style="margin-top:22px">${state.locale === 'zh' ? '浏览器' : 'Browser'}</h3>${bars(data.by_browser || [], 'name')}</div></div><div class="card table-card" style="margin: 0 24px 24px"><table><thead><tr><th>${state.locale === 'zh' ? '时间' : 'Time'}</th><th>${state.locale === 'zh' ? '事件' : 'Event'}</th><th>${t('common.status')}</th><th>${state.locale === 'zh' ? '设备' : 'Device'}</th><th>${state.locale === 'zh' ? '浏览器' : 'Browser'}</th><th>IP</th></tr></thead><tbody>${(data.recent || []).map(v => `<tr><td>${fmtDate(v.created_at)}</td><td>${esc(v.event_type)}</td><td>${esc(v.status)}</td><td>${esc(v.device_type)}</td><td>${esc(v.browser)}</td><td>${esc(v.ip)}</td></tr>`).join('') || `<tr><td colspan="6"><div class="empty">${state.locale === 'zh' ? '暂无访问记录' : 'No logs'}</div></td></tr>`}</tbody></table></div>`);
}
function bars(rows, key) {
  if (!rows.length) return `<p class="muted">${state.locale === 'zh' ? '暂无数据' : 'No data'}</p>`;
  const max = Math.max(...rows.map(x => Number(x.count || 0)), 1);
  return `<div class="bars">${rows.map(x => `<div class="bar"><span title="${esc(x[key])}">${esc(x[key])}</span><div class="bar-line"><div class="bar-fill" style="width:${Math.max(2, Number(x.count || 0) / max * 100)}%"></div></div><strong>${Number(x.count || 0).toLocaleString()}</strong></div>`).join('')}</div>`;
}

async function init() {
  applyTheme();
  document.getElementById('themeToggle')?.addEventListener('click', toggleTheme);
  document.getElementById('langToggle')?.addEventListener('click', toggleLocale);
  document.querySelectorAll('.nav').forEach(btn => btn.addEventListener('click', () => setView(btn.dataset.view)));
  refreshBtn.addEventListener('click', () => render());
  createBtn.addEventListener('click', () => { if (state.view === 'lives') openLiveEditor(); else openShortModal(); });
  modal.addEventListener('click', (e) => { if (e.target.dataset.close) closeModal(); });
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape') closeModal(); });
  try { await loadMe(); } catch (err) { toast(err.message || '账户信息加载失败'); }
  document.querySelectorAll('.nav').forEach(b => b.classList.toggle('active', b.dataset.view === state.view));
  await render();
}

init();
