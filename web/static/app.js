// Maildex front end: plain ES modules, no build step, no dependencies.
import { PROVIDERS, providerFor, detectProvider } from '/providers.js';

// ---------------------------------------------------------------- helpers
const app = document.getElementById('app');

function h(tag, props, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(props || {})) {
    if (v == null || v === false) continue;
    if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else if (k === 'class') el.className = v;
    else if (k in el && k !== 'list') el[k] = v;
    else el.setAttribute(k, v === true ? '' : v);
  }
  append(el, ...kids);
  return el;
}

function append(el, ...kids) {
  for (const k of kids.flat(Infinity)) {
    if (k == null || k === false) continue;
    el.append(k.nodeType ? k : document.createTextNode(String(k)));
  }
}

function clear(el) { while (el.firstChild) el.removeChild(el.firstChild); }

const ICONS = {
  search: '<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>',
  mail: '<rect x="2" y="4" width="20" height="16" rx="2"/><path d="m22 7-10 6L2 7"/>',
  grid: '<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>',
  settings: '<path d="M4 6h9M17 6h3M4 12h3M11 12h9M4 18h11M19 18h1"/><circle cx="15" cy="6" r="2"/><circle cx="9" cy="12" r="2"/><circle cx="17" cy="18" r="2"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  lock: '<rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/>',
  fingerprint: '<path d="M12 10a2 2 0 0 0-2 2c0 1.02-.1 2.51-.26 4"/><path d="M14 13.12c0 2.38 0 6.38-1 8.88"/><path d="M17.29 21.02c.12-.6.43-2.3.5-3.02"/><path d="M2 12a10 10 0 0 1 18-6"/><path d="M2 16h.01"/><path d="M21.8 16c.2-2 .131-5.354 0-6"/><path d="M5 19.5C5.5 18 6 15 6 12a6 6 0 0 1 .34-2"/><path d="M8.65 22c.21-.66.45-1.32.57-2"/><path d="M9 6.8a6 6 0 0 1 9 5.2v2"/>',
  copy: '<rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>',
  x: '<path d="M18 6 6 18M6 6l12 12"/>',
  alert: '<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4M12 17h.01"/>',
  calendar: '<rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/>',
  external: '<path d="M15 3h6v6M10 14 21 3M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>',
  download: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>',
  upload: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M17 8l-5-5-5 5M12 3v12"/>',
  logout: '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9"/>',
  shield: '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>',
  globe: '<circle cx="12" cy="12" r="10"/><path d="M2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>',
  folder: '<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
  check: '<path d="M20 6 9 17l-5-5"/>',
  back: '<path d="M19 12H5M12 19l-7-7 7-7"/>',
  trash: '<path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/>',
};

// Folder colours: keep in step with FolderColors in internal/store/store.go.
const FOLDER_COLORS = ['#6366f1', '#ec4899', '#f59e0b', '#10b981', '#06b6d4', '#ef4444', '#8b5cf6', '#84cc16', '#f97316', '#14b8a6'];

// Provider logo tile: brand colour with the logo, a coloured letter, or a neutral envelope.
function providerTile(value, size = 44, address = '') {
  const p = providerFor(value);
  const px = `width:${size}px;height:${size}px;border-radius:${Math.round(size * 0.3)}px;`;
  if (p && p.path) {
    const t = document.createElement('template');
    t.innerHTML = `<svg class="glyph" viewBox="0 0 24 24" aria-hidden="true"><path d="${p.path}"/></svg>`;
    return h('div', { class: 'plogo', style: `${px}background:#${p.hex}`, title: p.name }, t.content.firstChild);
  }
  if (p) return h('div', { class: 'plogo letter', style: `${px}background:#${p.hex};font-size:${Math.round(size * 0.45)}px`, title: p.name }, p.name[0]);
  if (value) return h('div', { class: 'plogo letter', style: `${px}background:${avatarColor(value)};font-size:${Math.round(size * 0.45)}px`, title: value }, value.trim()[0].toUpperCase());
  const own = address && !detectProvider(address);
  return h('div', { class: 'plogo neutral', style: px, title: own ? 'Own domain' : '' }, icon(own ? 'globe' : 'mail'));
}

const providerName = (value) => (providerFor(value) ? providerFor(value).name : value || '');

// Second factors a service can have, in display order.
const TWOFA = [
  ['totp', 'Authenticator app', 'OTP'],
  ['passkey', 'Passkey', 'Passkey'],
  ['security-key', 'Security key', 'Key'],
  ['sms', 'SMS code', 'SMS'],
  ['email', 'Email code', 'Email'],
  ['push', 'App prompt', 'Push'],
  ['backup', 'Backup codes', 'Backup'],
];
const TWOFA_SHORT = Object.fromEntries(TWOFA.map(([k, , s]) => [k, s]));

function icon(name) {
  const t = document.createElement('template');
  t.innerHTML = `<svg class="i" viewBox="0 0 24 24" aria-hidden="true">${ICONS[name] || ''}</svg>`;
  return t.content.firstChild;
}

function toast(msg, bad) {
  const el = h('div', { class: 'toast' + (bad ? ' bad' : '') }, msg);
  document.getElementById('toasts').append(el);
  setTimeout(() => el.remove(), bad ? 5000 : 2600);
}

async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const ta = h('textarea', { value: text, style: 'position:fixed;opacity:0' });
    document.body.append(ta);
    ta.select();
    try { document.execCommand('copy'); } catch { /* ignore */ }
    ta.remove();
  }
  toast('Copied ' + text);
}

const b64u = {
  enc(buf) {
    let s = '';
    for (const b of new Uint8Array(buf)) s += String.fromCharCode(b);
    return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  },
  dec(str) {
    str = str.replace(/-/g, '+').replace(/_/g, '/');
    str += '='.repeat((4 - (str.length % 4)) % 4);
    return Uint8Array.from(atob(str), (c) => c.charCodeAt(0)).buffer;
  },
};

// ---------------------------------------------------------------- api
async function api(method, url, body) {
  const res = await fetch(url, {
    method,
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', 'X-Maildex': '1' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  let data = null;
  try { data = await res.json(); } catch { /* empty body */ }
  if (!res.ok) {
    const err = new Error((data && data.error) || res.statusText);
    err.status = res.status;
    err.data = data;
    const authEndpoint = /^\/api\/(login|unlock|setup|password|passkey)/.test(url);
    if ((res.status === 423 || (res.status === 401 && !authEndpoint)) && S.session && S.session.authenticated) boot();
    throw err;
  }
  return data;
}

// ---------------------------------------------------------------- state
const S = {
  session: null,
  data: { emails: [], services: [], folders: [], settings: { lockMinutes: 5 } },
  tab: 'services',
  q: '',
  fEmail: '',
  fStatus: '',
  fFolder: '', // '' = all, 'none' = unfiled, or a folder id
  f2fa: '',
  openFolder: null, // folder id, 'none', or null while browsing the folder grid
  selMode: false,
  sel: new Set(), // selected service ids
  visibleIds: [], // services currently listed, for "select all"
  passkeys: [],
  version: null,
  lastActivity: Date.now(),
  lastPing: 0,
};

const emailById = (id) => S.data.emails.find((e) => e.id === id);
const servicesOf = (emailId) => S.data.services.filter((s) => s.emailId === emailId);
const folderById = (id) => (S.data.folders || []).find((f) => f.id === id);
const foldersOf = (s) => (s.folders || []).map(folderById).filter(Boolean);
const inFolder = (folderId) => S.data.services.filter((s) => foldersOf(s).some((f) => f.id === folderId));
const unfiled = () => S.data.services.filter((s) => !foldersOf(s).length);

async function loadData() { S.data = await api('GET', '/api/data'); }

async function boot() {
  try {
    S.session = await api('GET', '/api/session');
  } catch {
    app.replaceChildren(gate(h('h1', null, 'Cannot reach Maildex'), h('p', null, 'Check that the server is running, then retry.'), h('button', { class: 'btn primary block', onclick: boot }, 'Retry')));
    return;
  }
  document.querySelectorAll('.overlay').forEach((o) => o.remove());
  if (S.session.setup) return renderSetup();
  if (!S.session.authenticated) return renderLogin();
  if (S.session.locked) return renderLock();
  try {
    await loadData();
  } catch {
    return;
  }
  renderApp();
}

// ---------------------------------------------------------------- passkeys
const passkeyOK = () => !!(window.PublicKeyCredential && window.isSecureContext);

async function passkeyLogin() {
  const { publicKey } = await api('POST', '/api/passkey/login/begin');
  publicKey.challenge = b64u.dec(publicKey.challenge);
  publicKey.allowCredentials = publicKey.allowCredentials.map((c) => ({ ...c, id: b64u.dec(c.id) }));
  const cred = await navigator.credentials.get({ publicKey });
  await api('POST', '/api/passkey/login/finish', {
    id: cred.id,
    response: {
      clientDataJSON: b64u.enc(cred.response.clientDataJSON),
      authenticatorData: b64u.enc(cred.response.authenticatorData),
      signature: b64u.enc(cred.response.signature),
    },
  });
}

async function passkeyRegister(name) {
  const { publicKey } = await api('POST', '/api/passkeys/begin');
  publicKey.challenge = b64u.dec(publicKey.challenge);
  publicKey.user.id = b64u.dec(publicKey.user.id);
  publicKey.excludeCredentials = publicKey.excludeCredentials.map((c) => ({ ...c, id: b64u.dec(c.id) }));
  const cred = await navigator.credentials.create({ publicKey });
  await api('POST', '/api/passkeys/finish', {
    name,
    response: {
      clientDataJSON: b64u.enc(cred.response.clientDataJSON),
      attestationObject: b64u.enc(cred.response.attestationObject),
    },
  });
}

function passkeyError(e) {
  if (e && (e.name === 'NotAllowedError' || e.name === 'AbortError')) return 'Cancelled.';
  return (e && e.message) || 'Biometric check failed.';
}

function deviceName() {
  const ua = navigator.userAgent;
  if (/iPhone/.test(ua)) return 'iPhone';
  if (/iPad/.test(ua)) return 'iPad';
  if (/Android/.test(ua)) return 'Android phone';
  if (/Windows/.test(ua)) return 'Windows PC';
  if (/Mac/.test(ua)) return 'Mac';
  return 'This device';
}

// ---------------------------------------------------------------- gates
function gate(...kids) {
  return h('div', { class: 'gate' }, h('div', { class: 'gate-card' }, ...kids));
}

function logo() { return h('img', { class: 'logo', src: '/icons/icon-192.png', alt: '' }); }

function renderSetup() {
  const err = h('p', { class: 'error' });
  const pw = h('input', { class: 'input', type: 'password', placeholder: 'Password (min. 8 characters)', autocomplete: 'new-password' });
  const pw2 = h('input', { class: 'input', type: 'password', placeholder: 'Repeat password', autocomplete: 'new-password' });
  const form = h('form', { style: 'display:grid;gap:12px', onsubmit: async (e) => {
    e.preventDefault();
    err.textContent = '';
    if (pw.value !== pw2.value) { err.textContent = 'Passwords do not match.'; return; }
    try {
      await api('POST', '/api/setup', { password: pw.value });
      boot();
    } catch (x) { err.textContent = x.message; }
  } }, pw, pw2, err, h('button', { class: 'btn primary block', type: 'submit' }, 'Create password'));
  app.replaceChildren(gate(logo(), h('h1', null, 'Welcome to Maildex'), h('p', null, 'Choose a password to protect your catalogue. You can add fingerprint or Face ID unlock afterwards.'), form));
  pw.focus();
}

function renderLogin() {
  const err = h('p', { class: 'error' });
  const pw = h('input', { class: 'input', type: 'password', placeholder: 'Password', autocomplete: 'current-password' });
  const kids = [
    pw, err,
    h('button', { class: 'btn primary block', type: 'submit' }, 'Sign in'),
  ];
  if (S.session.biometric && passkeyOK()) {
    kids.push(h('button', { class: 'btn block', type: 'button', onclick: async () => {
      err.textContent = '';
      try { await passkeyLogin(); boot(); } catch (x) { err.textContent = passkeyError(x); }
    } }, icon('fingerprint'), 'Use biometrics'));
  }
  const form = h('form', { style: 'display:grid;gap:12px', onsubmit: async (e) => {
    e.preventDefault();
    err.textContent = '';
    try { await api('POST', '/api/login', { password: pw.value }); boot(); } catch (x) { err.textContent = x.message; pw.select(); }
  } }, ...kids);
  app.replaceChildren(gate(logo(), h('h1', null, 'Maildex'), h('p', null, 'Sign in to see your accounts.'), form));
  pw.focus();
}

function renderLock() {
  S.data = { emails: [], services: [], folders: [], settings: S.data.settings }; // nothing sensitive stays in memory while locked
  S.sel.clear();
  S.selMode = false;
  S.openFolder = null;
  const err = h('p', { class: 'error' });
  const pwBox = h('form', { style: 'display:none;gap:12px', onsubmit: async (e) => {
    e.preventDefault();
    err.textContent = '';
    try { await api('POST', '/api/unlock', { password: pw.value }); boot(); } catch (x) { err.textContent = x.message; pw.select(); }
  } });
  const pw = h('input', { class: 'input', type: 'password', placeholder: 'Password', autocomplete: 'current-password' });
  append(pwBox, [pw, h('button', { class: 'btn primary block', type: 'submit' }, 'Unlock')]);
  const unlock = async () => {
    err.textContent = '';
    try { await passkeyLogin(); boot(); } catch (x) { err.textContent = passkeyError(x); }
  };
  const showPw = () => { pwBox.style.display = 'grid'; pw.focus(); };
  const bio = S.session.biometric && passkeyOK();
  app.replaceChildren(gate(
    h('div', { class: 'big-icon' }, icon(bio ? 'fingerprint' : 'lock')),
    h('h1', null, 'Maildex is locked'),
    bio ? h('button', { class: 'btn primary block', onclick: unlock }, icon('fingerprint'), 'Unlock with biometrics') : null,
    bio ? h('button', { class: 'btn ghost block', onclick: showPw }, 'Use password instead') : null,
    pwBox, err,
  ));
  if (bio) unlock(); else showPw();
}

// ---------------------------------------------------------------- shell
const TABS = [
  ['services', 'Services', 'grid'],
  ['folders', 'Folders', 'folder'],
  ['emails', 'Emails', 'mail'],
  ['settings', 'Settings', 'settings'],
];

function setTab(tab) {
  S.tab = tab;
  S.openFolder = null;
  exitSelection(false);
  if (tab === 'settings') S.settingsLoaded = false;
  renderView();
  window.scrollTo(0, 0);
}

function renderApp() {
  const side = h('aside', { class: 'side' },
    h('div', { class: 'brand' }, h('img', { src: '/icons/icon-192.png', alt: '' }), 'Maildex'),
    ...TABS.map(([id, label, ic]) => h('button', { class: 'nav', 'data-tab': id, onclick: () => setTab(id) }, icon(ic), label)),
    h('div', { class: 'grow' }),
    h('button', { class: 'nav', onclick: lockNow }, icon('lock'), 'Lock'),
  );
  const tabbar = h('nav', { class: 'tabbar' },
    ...TABS.map(([id, label, ic]) => h('button', { class: 'tab', 'data-tab': id, onclick: () => setTab(id) }, icon(ic), label)));
  const fab = h('button', { class: 'fab', 'aria-label': 'Add', onclick: addForTab }, icon('plus'));
  app.replaceChildren(h('div', { class: 'shell' }, side, h('main', { class: 'main' }, h('div', { id: 'view' })), tabbar, fab, h('div', { id: 'selbar', class: 'selbar' })));
  renderView();
}

function renderView() {
  const v = document.getElementById('view');
  if (!v) return;
  // Forget selected services that no longer exist (deleted here or on another device).
  const live = new Set(S.data.services.map((s) => s.id));
  S.sel = new Set([...S.sel].filter((id) => live.has(id)));
  if (S.openFolder && S.openFolder !== 'none' && !folderById(S.openFolder)) S.openFolder = null;
  clear(v);
  ({ services: viewServices, folders: viewFolders, emails: viewEmails, settings: viewSettings })[S.tab](v);
  document.querySelectorAll('[data-tab]').forEach((b) => b.classList.toggle('active', b.dataset.tab === S.tab));
  renderSelectionBar();
}

function addForTab() {
  if (S.tab === 'emails') return emailForm();
  if (S.tab === 'folders' && !S.openFolder) return folderForm();
  if (!S.data.emails.length) { toast('Add an email first'); S.tab = 'emails'; renderView(); return emailForm(); }
  serviceForm(undefined, S.tab === 'folders' && S.openFolder && S.openFolder !== 'none' ? { folders: [S.openFolder] } : undefined);
}

async function lockNow() {
  try { await api('POST', '/api/lock'); } catch { /* boot() below shows the right screen */ }
  boot();
}

function pageHead(title, sub, action) {
  return h('div', { class: 'page-head' }, h('div', null, h('h1', null, title), sub ? h('p', null, sub) : null), action);
}

const addBtn = (label, fn) => h('button', { class: 'btn primary only-desktop', onclick: fn }, icon('plus'), label);

// ---------------------------------------------------------------- modal
function openModal(title, content, actions) {
  const close = () => { overlay.remove(); document.removeEventListener('keydown', onKey); };
  const onKey = (e) => { if (e.key === 'Escape') close(); };
  const overlay = h('div', { class: 'overlay', onmousedown: (e) => { if (e.target === overlay) close(); } },
    h('div', { class: 'modal', role: 'dialog', 'aria-label': title },
      h('h2', null, title, h('button', { class: 'btn ghost small', type: 'button', 'aria-label': 'Close', onclick: close }, icon('x'))),
      content, actions ? h('div', { class: 'actions' }, ...actions(close)) : null));
  document.addEventListener('keydown', onKey);
  document.body.append(overlay);
  return close;
}

const field = (label, input) => h('label', { class: 'field' }, h('span', null, label), input);

// ---------------------------------------------------------------- services view
const STATUS_LABEL = { active: 'Active', 'to-close': 'To close', closed: 'Closed' };
const METHOD_LABEL = { password: 'Password', google: 'Google', apple: 'Apple', microsoft: 'Microsoft', github: 'GitHub', facebook: 'Facebook', passkey: 'Passkey', sso: 'SSO', other: 'Other' };

function avatarColor(name) {
  let hash = 0;
  for (const c of name) hash = (hash * 31 + c.charCodeAt(0)) % 360;
  return `hsl(${hash} 62% 52%)`;
}

const avatar = (name) => h('div', { class: 'avatar', style: `background:${avatarColor(name)}` }, (name.trim()[0] || '?').toUpperCase());

function daysUntil(date) {
  const d = new Date(date + 'T00:00:00');
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  return Math.round((d - today) / 86400000);
}

function renewalText(date) {
  const n = daysUntil(date);
  if (n < 0) return `Renewal passed ${-n} d ago`;
  if (n === 0) return 'Renews today';
  if (n === 1) return 'Renews tomorrow';
  if (n <= 60) return `Renews in ${n} days`;
  return 'Renews ' + new Date(date + 'T00:00:00').toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
}

const safeColor = (c) => (/^#[0-9a-f]{6}$/i.test(c || '') ? c : '#6366f1');
const folderDot = (f) => h('i', { class: 'fdot', style: `background:${safeColor(f.color)}` });
const folderTag = (f) => h('span', { class: 'ftag' }, folderDot(f), f.name);

function filteredServices(folderKey = S.fFolder) {
  const toks = S.q.toLowerCase().split(/\s+/).filter(Boolean);
  return S.data.services
    .filter((s) => {
      if (S.fEmail && s.emailId !== S.fEmail) return false;
      if (S.fStatus && s.status !== S.fStatus) return false;
      const fl = foldersOf(s);
      if (folderKey === 'none' ? fl.length > 0 : folderKey && !fl.some((f) => f.id === folderKey)) return false;
      const tf = s.twoFactor || [];
      if (S.f2fa === 'none' ? tf.length > 0 : S.f2fa && !tf.includes(S.f2fa)) return false;
      const e = emailById(s.emailId);
      const hay = [s.name, s.username, s.url, s.notes, METHOD_LABEL[s.loginMethod], e && e.address, e && e.label, ...fl.map((f) => f.name)].join(' ').toLowerCase();
      return toks.every((t) => hay.includes(t));
    })
    .sort((a, b) => a.name.localeCompare(b.name));
}

// ---------------------------------------------------------------- selection
function toggleSelected(id) {
  if (S.sel.has(id)) S.sel.delete(id); else S.sel.add(id);
  refreshSelection();
}

function enterSelection(id) {
  S.selMode = true;
  if (id) S.sel.add(id);
  refreshSelection();
}

function exitSelection(redraw = true) {
  S.selMode = false;
  S.sel.clear();
  document.body.classList.remove('selecting');
  if (redraw) refreshSelection();
}

// Updates checkmarks and the action bar in place, so lists and scroll positions are not rebuilt.
function refreshSelection() {
  document.body.classList.toggle('selecting', S.selMode);
  document.querySelectorAll('.item[data-id]').forEach((el) => {
    const on = S.sel.has(el.dataset.id);
    el.classList.toggle('selected', on);
    const box = el.querySelector('.sel');
    if (box) box.setAttribute('aria-checked', on ? 'true' : 'false');
  });
  renderSelectionBar();
}

function selectAllVisible() {
  const all = S.visibleIds.length && S.visibleIds.every((id) => S.sel.has(id));
  S.visibleIds.forEach((id) => (all ? S.sel.delete(id) : S.sel.add(id)));
  refreshSelection();
}

function renderSelectionBar() {
  const bar = document.getElementById('selbar');
  if (!bar) return;
  document.body.classList.toggle('selecting', S.selMode);
  clear(bar);
  if (!S.selMode) return;
  const n = S.sel.size;
  const act = (label, ic, fn, cls = '') => h('button', { class: 'btn small ' + cls, disabled: !n, onclick: fn }, icon(ic), label);
  const allOn = S.visibleIds.length > 0 && S.visibleIds.every((id) => S.sel.has(id));
  append(bar,
    h('div', { class: 'sel-info' },
      h('strong', null, n ? `${n} selected` : 'Select services'),
      S.visibleIds.length ? h('button', { class: 'btn ghost small', onclick: selectAllVisible }, allOn ? 'Clear' : 'Select all') : null),
    h('div', { class: 'sel-actions' },
      act('Folder', 'folder', bulkFolderModal),
      act('2FA', 'shield', bulkTwoFactorModal),
      act('Status', 'check', bulkStatusModal),
      act('Delete', 'trash', bulkDelete, 'danger')),
    h('button', { class: 'btn small', onclick: () => exitSelection() }, 'Done'));
}

async function bulk(body) {
  const r = await api('POST', '/api/services/bulk', { ids: [...S.sel], ...body });
  await loadData();
  return r.affected;
}

const plural = (n, word) => `${n} ${word}${n === 1 ? '' : 's'}`;

// Finds a folder by name or creates it, so "type a new folder name" works everywhere.
async function folderIdForName(name) {
  const hit = S.data.folders.find((f) => f.name.toLowerCase() === name.toLowerCase());
  if (hit) return hit.id;
  const f = await api('POST', '/api/folders', { name });
  S.data.folders.push(f);
  return f.id;
}

function chipPicker(items, chosen, render) {
  const box = h('div', { class: 'chips' });
  const draw = () => {
    clear(box);
    for (const [id, label, extra] of items) {
      box.append(h('button', { type: 'button', class: 'chip' + (chosen.has(id) ? ' on' : ''), 'aria-pressed': chosen.has(id) ? 'true' : 'false',
        onclick: () => { if (chosen.has(id)) chosen.delete(id); else chosen.add(id); draw(); if (render) render(); } }, extra || null, label));
    }
  };
  draw();
  return box;
}

function bulkFolderModal() {
  const chosen = new Set();
  const err = h('p', { class: 'error' });
  const newName = h('input', { class: 'input', placeholder: 'Or type a new folder name', maxLength: 40 });
  const body = h('div', { style: 'display:grid;gap:12px' },
    h('p', { class: 'note' }, `Applies to ${plural(S.sel.size, 'selected service')}.`),
    S.data.folders.length ? chipPicker(S.data.folders.map((f) => [f.id, f.name, folderDot(f)]), chosen) : h('p', { class: 'note' }, 'No folders yet: type a name below to create the first one.'),
    newName, err);
  const run = async (mode, close) => {
    err.textContent = '';
    try {
      const ids = [...chosen];
      const name = newName.value.trim();
      if (name) {
        if (mode === 'remove' && !S.data.folders.some((f) => f.name.toLowerCase() === name.toLowerCase())) throw new Error(`There is no folder called "${name}".`);
        ids.push(await folderIdForName(name));
      }
      if (!ids.length) { err.textContent = 'Choose or name a folder first.'; return; }
      let n = 0;
      for (const id of new Set(ids)) n = await bulk({ action: mode === 'add' ? 'addFolder' : 'removeFolder', folderId: id });
      close();
      renderView();
      toast(`${mode === 'add' ? 'Added' : 'Removed'} ${plural(n, 'service')} ${mode === 'add' ? 'to' : 'from'} ${plural(new Set(ids).size, 'folder')}`);
    } catch (x) { err.textContent = x.message; }
  };
  openModal('Folders', body, (close) => [
    h('button', { class: 'btn', type: 'button', onclick: () => run('remove', close) }, 'Remove from'),
    h('span', { class: 'spacer' }),
    h('button', { class: 'btn primary', type: 'button', onclick: () => run('add', close) }, 'Add to'),
  ]);
}

function bulkTwoFactorModal() {
  const chosen = new Set();
  const err = h('p', { class: 'error' });
  const body = h('div', { style: 'display:grid;gap:12px' },
    h('p', { class: 'note' }, `Adding keeps the methods a service already has. Applies to ${plural(S.sel.size, 'selected service')}.`),
    chipPicker(TWOFA.map(([k, label]) => [k, label]), chosen), err);
  const run = async (mode, close) => {
    err.textContent = '';
    if (!chosen.size) { err.textContent = 'Choose at least one method.'; return; }
    try {
      const n = await bulk({ action: mode === 'add' ? 'addTwoFactor' : 'removeTwoFactor', twoFactor: [...chosen] });
      close();
      renderView();
      toast(`${mode === 'add' ? 'Added to' : 'Removed from'} ${plural(n, 'service')}`);
    } catch (x) { err.textContent = x.message; }
  };
  openModal('Two-step verification', body, (close) => [
    h('button', { class: 'btn', type: 'button', onclick: () => run('remove', close) }, 'Remove from all'),
    h('span', { class: 'spacer' }),
    h('button', { class: 'btn primary', type: 'button', onclick: () => run('add', close) }, 'Add to all'),
  ]);
}

function bulkStatusModal() {
  const err = h('p', { class: 'error' });
  const body = h('div', { style: 'display:grid;gap:10px' },
    h('p', { class: 'note' }, `Set the status of ${plural(S.sel.size, 'selected service')}.`),
    ...Object.entries(STATUS_LABEL).map(([k, label]) => h('button', { class: 'btn block', type: 'button', onclick: async () => {
      try { const n = await bulk({ action: 'setStatus', status: k }); close(); renderView(); toast(`${plural(n, 'service')} set to ${label.toLowerCase()}`); } catch (x) { err.textContent = x.message; }
    } }, label)), err);
  const close = openModal('Status', body);
}

async function bulkDelete() {
  const n = S.sel.size;
  if (!n || !confirm(`Delete ${plural(n, 'service')}? This cannot be undone.`)) return;
  try {
    await bulk({ action: 'delete' });
    S.sel.clear();
    renderView();
    toast(`Deleted ${plural(n, 'service')}`);
  } catch (x) { toast(x.message, true); }
}

// ---------------------------------------------------------------- service list
function serviceCard(s) {
  const e = emailById(s.emailId);
  const meta = h('div', { class: 'meta' },
    e ? h('button', { class: 'email-chip', title: 'Copy email', onclick: (ev) => { ev.stopPropagation(); if (S.selMode) toggleSelected(s.id); else copyText(e.address); } }, providerTile(e.provider || (detectProvider(e.address) || {}).id, 18, e.address), h('span', null, e.address)) : null,
    s.username && (!e || s.username.toLowerCase() !== e.address) ? h('span', null, s.username) : null,
    h('span', null, METHOD_LABEL[s.loginMethod] || s.loginMethod),
  );
  const tf = s.twoFactor || [];
  const security = h('div', { class: 'meta sec' }, icon('shield'),
    tf.length ? tf.map((k) => h('span', { class: 'badge ok' }, TWOFA_SHORT[k] || k)) : h('span', { class: 'badge' }, 'No 2FA'));
  const tags = foldersOf(s);
  const folderRow = tags.length ? h('div', { class: 'meta ftags' }, ...tags.slice(0, 4).map(folderTag), tags.length > 4 ? h('span', null, `+${tags.length - 4}`) : null) : null;
  const soon = s.renewal && daysUntil(s.renewal) <= 14;

  const activate = () => (S.selMode ? toggleSelected(s.id) : serviceForm(s));
  let timer = null, longPressed = false, touching = false;
  const card = h('div', { class: 'card item' + (s.status === 'closed' ? ' closed' : '') + (S.sel.has(s.id) ? ' selected' : ''), role: 'button', tabindex: '0', 'data-id': s.id,
    onclick: () => { if (longPressed) { longPressed = false; return; } activate(); },
    onkeydown: (ev) => { if ((ev.key === 'Enter' || ev.key === ' ') && ev.target === card) { ev.preventDefault(); activate(); } },
    // Touch: press and hold selects (and starts selection mode), like a gallery app.
    onpointerdown: (ev) => {
      touching = ev.pointerType !== 'mouse';
      if (!touching) return;
      longPressed = false;
      timer = setTimeout(() => { longPressed = true; if (navigator.vibrate) navigator.vibrate(15); S.selMode ? toggleSelected(s.id) : enterSelection(s.id); }, 450);
    },
    onpointerup: () => clearTimeout(timer),
    onpointerleave: () => clearTimeout(timer),
    onpointercancel: () => clearTimeout(timer),
    oncontextmenu: (ev) => { if (touching) ev.preventDefault(); } },
    h('span', { class: 'sel', role: 'checkbox', 'aria-label': 'Select ' + s.name, 'aria-checked': S.sel.has(s.id) ? 'true' : 'false',
      onclick: (ev) => { ev.stopPropagation(); toggleSelected(s.id); } }, icon('check')),
    avatar(s.name),
    h('div', { class: 'body' },
      h('div', { class: 'title' }, h('span', null, s.name),
        s.status !== 'active' ? h('span', { class: 'badge ' + (s.status === 'to-close' ? 'warn' : '') }, STATUS_LABEL[s.status]) : null,
        e && e.atRisk ? h('span', { class: 'badge danger' }, 'Email at risk') : null),
      meta, security, folderRow),
    s.renewal ? h('div', { class: 'renew' + (soon ? ' soon' : '') }, renewalText(s.renewal)) : null);
  return card;
}

const headActions = (...kids) => h('div', { class: 'head-actions' }, ...kids);
const selectBtn = () => h('button', { class: 'btn', onclick: () => (S.selMode ? exitSelection() : enterSelection()) }, icon('check'), 'Select');

// Search, filters and the list. `folderScope` pins the list to one folder (or 'none'); without it a folder filter is shown.
function serviceBrowser(v, folderScope) {
  const { emails, services, folders } = S.data;
  const scoped = folderScope !== undefined;

  const search = h('div', { class: 'search' }, icon('search'),
    h('input', { class: 'input', type: 'search', placeholder: 'Search a service, email, username or folder…', value: S.q, autocomplete: 'off', enterkeyhint: 'search',
      oninput: (e) => { S.q = e.target.value; updateList(); } }));

  const sel = (key, label, options) => h('select', { class: 'input', 'aria-label': label,
    onchange: (e) => { S[key] = e.target.value; updateList(); } },
    h('option', { value: '' }, label), ...options.map(([val, text]) => h('option', { value: val, selected: S[key] === val }, text)));
  const filters = h('div', { class: 'filters' },
    sel('fEmail', 'All emails', emails.map((e) => [e.id, e.address])),
    sel('fStatus', 'Any status', Object.entries(STATUS_LABEL)),
    sel('f2fa', 'Any 2FA', [['none', 'No 2FA'], ...TWOFA.map(([k, label]) => [k, 'With ' + label.toLowerCase()])]),
    scoped || !folders.length ? null : sel('fFolder', 'All folders', [['none', 'No folder'], ...folders.map((f) => [f.id, f.name])]));

  const alerts = h('div', { class: 'alerts' });
  if (!scoped) {
    const no2fa = services.filter((s) => s.status === 'active' && !(s.twoFactor || []).length).length;
    if (no2fa) {
      alerts.append(h('button', { class: 'alert-chip info', onclick: () => { S.f2fa = 'none'; S.fStatus = 'active'; renderView(); } }, icon('shield'), `${no2fa} active service${no2fa === 1 ? '' : 's'} without 2FA`));
    }
    const risky = emails.filter((e) => e.atRisk);
    if (risky.length) {
      const n = risky.reduce((sum, e) => sum + servicesOf(e.id).length, 0);
      alerts.append(h('button', { class: 'alert-chip', onclick: () => setTab('emails') }, icon('alert'), `${risky.length} email${risky.length === 1 ? '' : 's'} at risk · ${n} service${n === 1 ? '' : 's'} to secure`));
    }
    services.filter((s) => s.renewal && s.status !== 'closed' && daysUntil(s.renewal) <= 30)
      .sort((a, b) => a.renewal.localeCompare(b.renewal)).slice(0, 4)
      .forEach((s) => alerts.append(h('button', { class: 'alert-chip info', onclick: () => serviceForm(s) }, icon('calendar'), `${s.name} · ${renewalText(s.renewal).toLowerCase()}`)));
  }

  const count = h('p', { class: 'count' });
  const list = h('div', { class: 'list' });
  append(v, search, filters, alerts.children.length ? alerts : null, count, list);

  function updateList() {
    const rows = filteredServices(scoped ? folderScope : S.fFolder);
    S.visibleIds = rows.map((s) => s.id);
    clear(list);
    count.textContent = rows.length === services.length ? '' : `${rows.length} match${rows.length === 1 ? '' : 'es'}`;
    if (!rows.length) {
      const nothingYet = scoped ? !(folderScope === 'none' ? unfiled() : inFolder(folderScope)).length : !services.length;
      list.append(h('div', { class: 'empty' },
        h('h3', null, nothingYet ? (scoped ? 'Nothing in this folder yet' : 'No services yet') : 'No matches'),
        nothingYet
          ? (scoped ? h('p', null, 'Open the Services tab, tap Select, pick some services and choose Folder.') : h('button', { class: 'btn primary', onclick: () => serviceForm() }, icon('plus'), 'Add your first service'))
          : h('p', null, 'Try a different search or clear the filters.')));
    } else {
      rows.forEach((s) => list.append(serviceCard(s)));
    }
    refreshSelection();
  }
  updateList();
}

function viewServices(v) {
  const { emails, services } = S.data;
  append(v, pageHead('Services', `${plural(services.length, 'service')} across ${plural(emails.length, 'email')}`, headActions(selectBtn(), addBtn('Add service', addForTab))));

  if (!emails.length) {
    append(v, h('div', { class: 'card empty' }, h('h3', null, 'Start with your emails'), h('p', null, 'Add the email addresses you use, then record which services each one has.'),
      h('button', { class: 'btn primary', onclick: () => { S.tab = 'emails'; renderView(); emailForm(); } }, icon('plus'), 'Add your first email')));
    return;
  }
  serviceBrowser(v);
}

// ---------------------------------------------------------------- folders view
function folderCard(f, count) {
  const c = f ? safeColor(f.color) : '';
  return h('button', { class: 'card folder-card', onclick: () => {
    S.q = ''; S.fEmail = ''; S.fStatus = ''; S.f2fa = '';
    S.openFolder = f ? f.id : 'none';
    renderView();
    window.scrollTo(0, 0);
  } },
    h('div', { class: 'folder-ic', style: f ? `background:${c}26;color:${c}` : '' }, icon('folder')),
    h('div', { class: 'folder-name' }, f ? f.name : 'No folder'),
    h('div', { class: 'note' }, plural(count, 'service')));
}

function viewFolders(v) {
  if (S.openFolder) return viewFolderOpen(v);
  const { folders, services } = S.data;
  append(v, pageHead('Folders', `${plural(folders.length, 'folder')} · a service can be in several`, headActions(addBtn('New folder', () => folderForm()))));
  if (!folders.length) {
    append(v, h('div', { class: 'card empty' }, h('h3', null, 'No folders yet'),
      h('p', null, 'Group services however you like: Banks, Work, Family… A service can be in more than one folder.'),
      h('button', { class: 'btn primary', onclick: () => folderForm() }, icon('plus'), 'Create a folder')));
  }
  const grid = h('div', { class: 'folder-grid' });
  [...folders].sort((a, b) => a.name.localeCompare(b.name)).forEach((f) => grid.append(folderCard(f, inFolder(f.id).length)));
  if (services.length && folders.length) grid.append(folderCard(null, unfiled().length));
  append(v, grid);
}

function viewFolderOpen(v) {
  const f = S.openFolder === 'none' ? null : folderById(S.openFolder);
  const count = (f ? inFolder(f.id) : unfiled()).length;
  append(v, h('div', { class: 'page-head' },
    h('div', { class: 'crumb' },
      h('button', { class: 'btn small', 'aria-label': 'Back to folders', onclick: () => { S.openFolder = null; exitSelection(false); renderView(); } }, icon('back')),
      h('div', null, h('h1', null, f ? h('span', { class: 'folder-title' }, folderDot(f), f.name) : 'No folder'), h('p', null, plural(count, 'service')))),
    headActions(f ? h('button', { class: 'btn', onclick: () => folderForm(f) }, 'Edit') : null, selectBtn(), addBtn('Add service', addForTab))));
  serviceBrowser(v, S.openFolder);
}

function folderForm(existing) {
  const f = existing || { name: '', color: FOLDER_COLORS[S.data.folders.length % FOLDER_COLORS.length] };
  let color = safeColor(f.color);
  const err = h('p', { class: 'error' });
  const name = h('input', { class: 'input', value: f.name, maxLength: 40, placeholder: 'Banks, Work, Family…', required: true, autocomplete: 'off' });
  const swatches = h('div', { class: 'swatches' });
  const draw = () => {
    clear(swatches);
    FOLDER_COLORS.forEach((c) => swatches.append(h('button', { type: 'button', class: 'swatch' + (c === color ? ' on' : ''), style: `background:${c}`, 'aria-label': 'Color ' + c, 'aria-pressed': c === color ? 'true' : 'false', onclick: () => { color = c; draw(); } })));
  };
  draw();
  const form = h('form', { style: 'display:grid;gap:12px' }, field('Name', name), h('div', { class: 'field' }, h('span', null, 'Color'), swatches), err);
  const save = async (close) => {
    err.textContent = '';
    try {
      if (existing) await api('PUT', '/api/folders/' + existing.id, { name: name.value, color });
      else await api('POST', '/api/folders', { name: name.value, color });
      await loadData();
      close();
      renderView();
      toast(existing ? 'Saved' : 'Folder created');
    } catch (x) { err.textContent = x.message; }
  };
  const close = openModal(existing ? 'Edit folder' : 'New folder', form, (closeFn) => [
    existing ? h('button', { class: 'btn danger', type: 'button', onclick: async () => {
      const n = inFolder(existing.id).length;
      if (!confirm(`Delete the folder "${existing.name}"?${n ? ` Its ${plural(n, 'service')} will be kept.` : ''}`)) return;
      try {
        await api('DELETE', '/api/folders/' + existing.id);
        if (S.openFolder === existing.id) S.openFolder = null;
        await loadData();
        closeFn();
        renderView();
        toast('Folder deleted');
      } catch (x) { err.textContent = x.message; }
    } }, 'Delete') : null,
    h('span', { class: 'spacer' }),
    h('button', { class: 'btn primary', type: 'button', onclick: () => save(closeFn) }, 'Save'),
  ]);
  form.addEventListener('submit', (e) => { e.preventDefault(); save(close); });
  if (!existing) name.focus();
}

// ---------------------------------------------------------------- service form
function serviceForm(existing, preset) {
  const s = existing || { name: '', emailId: S.fEmail || (S.data.emails[0] && S.data.emails[0].id) || '', username: '', loginMethod: 'password', folders: (preset && preset.folders) || [], status: 'active', twoFactor: [], url: '', renewal: '', notes: '' };
  const err = h('p', { class: 'error' });
  const tfSel = new Set(s.twoFactor || []);
  const tfChips = h('div', { class: 'chips' }, ...TWOFA.map(([k, label]) => {
    const chip = h('button', { type: 'button', class: 'chip' + (tfSel.has(k) ? ' on' : ''), 'aria-pressed': tfSel.has(k) ? 'true' : 'false', onclick: () => {
      if (tfSel.has(k)) tfSel.delete(k); else tfSel.add(k);
      chip.classList.toggle('on', tfSel.has(k));
      chip.setAttribute('aria-pressed', tfSel.has(k) ? 'true' : 'false');
    } }, label);
    return chip;
  }));

  // Folders: toggle existing ones, or type a name to create one on the spot.
  const fSel = new Set((s.folders || []).filter((id) => folderById(id)));
  const folderChips = h('div', { class: 'chips' });
  const drawFolders = () => {
    clear(folderChips);
    if (!S.data.folders.length) folderChips.append(h('span', { class: 'note' }, 'No folders yet. Type a name below to create one.'));
    S.data.folders.forEach((f) => folderChips.append(h('button', { type: 'button', class: 'chip' + (fSel.has(f.id) ? ' on' : ''), 'aria-pressed': fSel.has(f.id) ? 'true' : 'false',
      onclick: () => { if (fSel.has(f.id)) fSel.delete(f.id); else fSel.add(f.id); drawFolders(); } }, folderDot(f), f.name)));
  };
  drawFolders();
  const newFolder = h('input', { class: 'input', placeholder: 'New folder…', maxLength: 40, autocomplete: 'off',
    onkeydown: (ev) => { if (ev.key === 'Enter') { ev.preventDefault(); addFolder(); } } });
  const addFolder = async () => {
    const n = newFolder.value.trim();
    if (!n) return;
    try { fSel.add(await folderIdForName(n)); newFolder.value = ''; err.textContent = ''; drawFolders(); } catch (x) { err.textContent = x.message; }
  };

  const name = h('input', { class: 'input', value: s.name, placeholder: 'Netflix, GitHub, bank…', required: true, maxLength: 120, autocomplete: 'off' });
  const email = h('select', { class: 'input' }, ...S.data.emails.map((e) => h('option', { value: e.id, selected: e.id === s.emailId }, e.address + (e.label ? ` (${e.label})` : ''))));
  const username = h('input', { class: 'input', value: s.username, placeholder: 'If different from the email', autocomplete: 'off', autocapitalize: 'none' });
  const method = h('select', { class: 'input' }, ...Object.entries(METHOD_LABEL).map(([k, t]) => h('option', { value: k, selected: k === s.loginMethod }, t)));
  const status = h('select', { class: 'input' }, ...Object.entries(STATUS_LABEL).map(([k, t]) => h('option', { value: k, selected: k === s.status }, t)));
  const url = h('input', { class: 'input', value: s.url, placeholder: 'example.com', inputmode: 'url', autocapitalize: 'none' });
  const renewal = h('input', { class: 'input', type: 'date', value: s.renewal });
  const notes = h('textarea', { class: 'input', placeholder: 'Plan, recovery hints, anything useful…' }, s.notes);
  notes.value = s.notes;

  const form = h('form', { style: 'display:grid;gap:12px' },
    field('Service', name),
    field('Email used', email),
    h('div', { class: 'row2' }, field('Sign-in method', method), field('Status', status)),
    field('Username', username),
    h('div', { class: 'field' }, h('span', null, 'Two-step verification (2FA)'), tfChips),
    h('div', { class: 'field' }, h('span', null, 'Folders'), folderChips,
      h('div', { class: 'row-add' }, newFolder, h('button', { class: 'btn small', type: 'button', onclick: addFolder }, 'Add'))),
    field('Renewal date', renewal),
    field('Website', url),
    field('Notes', notes),
    err);

  const body = () => ({ name: name.value, emailId: email.value, username: username.value, loginMethod: method.value, folders: [...fSel], status: status.value, twoFactor: [...tfSel], url: url.value, renewal: renewal.value, notes: notes.value });
  const save = async (close) => {
    err.textContent = '';
    if (!name.value.trim()) { err.textContent = 'Give the service a name.'; return; }
    try {
      if (newFolder.value.trim()) await addFolder(); // a typed but not yet added folder still counts
      if (existing) await api('PUT', '/api/services/' + existing.id, body());
      else await api('POST', '/api/services', body());
      await loadData();
      close();
      renderView();
      toast(existing ? 'Saved' : 'Service added');
    } catch (x) { err.textContent = x.message; }
  };
  form.addEventListener('submit', (e) => { e.preventDefault(); form.dispatchEvent(new CustomEvent('save')); });

  const close = openModal(existing ? 'Edit service' : 'New service', form, (closeFn) => [
    existing ? h('button', { class: 'btn danger', type: 'button', onclick: async () => {
      if (!confirm(`Delete ${existing.name}?`)) return;
      try { await api('DELETE', '/api/services/' + existing.id); await loadData(); closeFn(); renderView(); toast('Deleted'); } catch (x) { err.textContent = x.message; }
    } }, 'Delete') : null,
    h('span', { class: 'spacer' }),
    existing && existing.url ? h('a', { class: 'btn', href: existing.url, target: '_blank', rel: 'noopener noreferrer' }, icon('external'), 'Open') : null,
    h('button', { class: 'btn primary', type: 'button', onclick: () => save(closeFn) }, 'Save'),
  ]);
  form.addEventListener('save', () => save(close));
  if (!existing) name.focus();
}

// ---------------------------------------------------------------- emails view
function viewEmails(v) {
  append(v, pageHead('Emails', 'Every address you use and what depends on it', addBtn('Add email', () => emailForm())));
  const emails = [...S.data.emails].sort((a, b) => servicesOf(b.id).length - servicesOf(a.id).length || a.address.localeCompare(b.address));
  if (!emails.length) {
    append(v, h('div', { class: 'card empty' }, h('h3', null, 'No emails yet'), h('p', null, 'Add the addresses you sign up with.'),
      h('button', { class: 'btn primary', onclick: () => emailForm() }, icon('plus'), 'Add an email')));
    return;
  }
  const list = h('div', { class: 'list' });
  for (const e of emails) {
    const svcs = servicesOf(e.id).sort((a, b) => a.name.localeCompare(b.name));
    list.append(h('div', { class: 'card email-card' },
      h('div', { class: 'email-top' },
        providerTile(e.provider || (detectProvider(e.address) || {}).id, 44, e.address),
        h('div', { class: 'body' },
          h('div', { class: 'email-addr' }, e.address),
          h('div', { class: 'meta' }, [e.label, providerName(e.provider) || (detectProvider(e.address) || {}).name].filter(Boolean).join(' · ') || null, e.atRisk ? h('span', { class: 'badge danger' }, 'At risk') : null)),
        h('button', { class: 'btn small', onclick: () => copyText(e.address), 'aria-label': 'Copy email' }, icon('copy')),
        h('button', { class: 'btn small', onclick: () => emailForm(e) }, 'Edit')),
      e.recovery ? h('p', { class: 'note' }, 'Recovery: ' + e.recovery) : null,
      svcs.length
        ? h('details', { class: 'svc-list' }, h('summary', null, `${svcs.length} service${svcs.length === 1 ? '' : 's'}`),
          h('div', { class: 'pills' }, ...svcs.map((s) => h('button', { class: 'pill', onclick: () => serviceForm(s) }, s.name))))
        : h('p', { class: 'note' }, 'No services recorded.')));
  }
  append(v, list);
}

function emailForm(existing) {
  const e = existing || { address: '', label: '', provider: '', recovery: '', notes: '', atRisk: false };
  const err = h('p', { class: 'error' });
  const address = h('input', { class: 'input', type: 'email', value: e.address, placeholder: 'name@example.com', required: true, autocapitalize: 'none', autocomplete: 'off' });
  const label = h('input', { class: 'input', value: e.label, placeholder: 'Personal, work, old…', maxLength: 80 });
  // Provider picker: known providers show their logo; "Own domain" and "Other" cover the rest.
  const known = providerFor(e.provider);
  let choice = known ? known.id : !e.provider ? '' : e.provider.toLowerCase() === 'own domain' ? 'own' : 'other';
  let touched = !!existing && !!e.provider; // stop auto-detecting once the user has decided
  const custom = h('input', { class: 'input', value: choice === 'other' ? e.provider : '', placeholder: 'Provider name', maxLength: 80, style: choice === 'other' ? '' : 'display:none' });
  const grid = h('div', { class: 'prov-grid' });
  const options = [...PROVIDERS.map((p) => [p.id, p.name]), ['own', 'Own domain'], ['other', 'Other']];
  const drawPicker = () => {
    clear(grid);
    for (const [id, name] of options) {
      grid.append(h('button', { type: 'button', class: 'prov' + (choice === id ? ' on' : ''), 'aria-pressed': choice === id ? 'true' : 'false', onclick: () => {
        touched = true;
        choice = choice === id ? '' : id;
        custom.style.display = choice === 'other' ? '' : 'none';
        drawPicker();
        if (choice === 'other') custom.focus();
      } }, id === 'own' ? providerTile('', 36, 'me@own.example') : id === 'other' ? providerTile('', 36, '') : providerTile(id, 36), h('span', null, name)));
    }
  };
  drawPicker();
  address.addEventListener('input', () => {
    const p = touched ? null : detectProvider(address.value);
    if (p && p.id !== choice) { choice = p.id; custom.style.display = 'none'; drawPicker(); }
  });
  const providerValue = () => (choice === 'own' ? 'Own domain' : choice === 'other' ? custom.value : choice);
  const recovery = h('input', { class: 'input', value: e.recovery, placeholder: 'Recovery phone or backup email', maxLength: 300 });
  const notes = h('textarea', { class: 'input', placeholder: 'Anything worth remembering' });
  notes.value = e.notes;
  const risk = h('input', { type: 'checkbox', checked: e.atRisk });
  const form = h('form', { style: 'display:grid;gap:12px' },
    field('Email address', address),
    field('Label', label),
    h('div', { class: 'field' }, h('span', null, 'Email provider'), grid, custom),
    field('Recovery', recovery),
    field('Notes', notes),
    h('label', { class: 'check' }, risk, h('span', null, 'Mark as at risk (leaked, old or insecure)')),
    err);
  const body = () => ({ address: address.value, label: label.value, provider: providerValue(), recovery: recovery.value, notes: notes.value, atRisk: risk.checked });
  const save = async (close) => {
    err.textContent = '';
    try {
      if (existing) await api('PUT', '/api/emails/' + existing.id, body());
      else await api('POST', '/api/emails', body());
      await loadData();
      close();
      renderView();
      toast(existing ? 'Saved' : 'Email added');
    } catch (x) { err.textContent = x.message; }
  };
  const close = openModal(existing ? 'Edit email' : 'New email', form, (closeFn) => [
    existing ? h('button', { class: 'btn danger', type: 'button', onclick: async () => {
      const n = servicesOf(existing.id).length;
      if (!confirm(n ? `${existing.address} has ${n} service${n === 1 ? '' : 's'}. Delete the email and all of them?` : `Delete ${existing.address}?`)) return;
      try { await api('DELETE', `/api/emails/${existing.id}?force=1`); await loadData(); closeFn(); renderView(); toast('Deleted'); } catch (x) { err.textContent = x.message; }
    } }, 'Delete') : null,
    h('span', { class: 'spacer' }),
    h('button', { class: 'btn primary', type: 'button', onclick: () => save(closeFn) }, 'Save'),
  ]);
  form.addEventListener('submit', (ev) => { ev.preventDefault(); save(close); });
  if (!existing) address.focus();
}

// ---------------------------------------------------------------- settings view
const LOCK_OPTIONS = [[1, '1 minute'], [5, '5 minutes'], [15, '15 minutes'], [30, '30 minutes'], [60, '1 hour'], [0, 'Never (manual lock only)']];

async function refreshSettingsData() {
  try { S.passkeys = await api('GET', '/api/passkeys'); } catch { S.passkeys = []; }
  if (S.session) S.session.biometric = S.passkeys.length > 0;
  api('GET', '/api/version').then((x) => { S.version = x; if (S.tab === 'settings') renderView(); }).catch(() => {});
}

function viewSettings(v) {
  append(v, pageHead('Settings', 'Security, backup and about'));
  if (!S.settingsLoaded) { S.settingsLoaded = true; refreshSettingsData().then(() => { if (S.tab === 'settings') renderView(); }); }

  // Biometrics
  const bio = h('div', { class: 'card section' }, h('h2', null, icon('fingerprint'), 'Biometric unlock'));
  if (!passkeyOK()) {
    bio.append(h('div', { class: 'banner' }, window.isSecureContext
      ? 'This browser does not support biometric unlock.'
      : 'Biometric unlock needs HTTPS and a hostname (not a bare IP address). Serve Maildex through a reverse proxy such as Caddy, Nginx Proxy Manager or Tailscale Serve, then open it from that address.'));
  } else {
    bio.append(h('p', { class: 'note' }, 'Use your fingerprint, Face ID or Windows Hello to unlock the app. Maildex locks itself after a period of inactivity.'));
    if (S.passkeys.length) {
      bio.append(h('div', { class: 'kv' }, ...S.passkeys.map((p) => h('div', { class: 'row' },
        h('div', null, h('strong', null, p.name), h('div', { class: 'note' }, 'Added ' + new Date(p.created).toLocaleDateString())),
        h('button', { class: 'btn small danger', onclick: async () => {
          if (!confirm(`Remove ${p.name}?`)) return;
          try { await api('DELETE', '/api/passkeys/' + encodeURIComponent(p.id)); await refreshSettingsData(); renderView(); } catch (x) { toast(x.message, true); }
        } }, 'Remove')))));
      const lockSel = h('select', { class: 'input', style: 'width:auto', onchange: async (e) => {
        try { S.data.settings = await api('PUT', '/api/settings', { lockMinutes: Number(e.target.value) }); toast('Saved'); } catch (x) { toast(x.message, true); }
      } }, ...LOCK_OPTIONS.map(([m, t]) => h('option', { value: m, selected: m === S.data.settings.lockMinutes }, t)));
      bio.append(h('div', { class: 'row' }, h('span', null, 'Lock after'), lockSel));
    }
    bio.append(h('div', { class: 'row' },
      h('button', { class: 'btn primary', onclick: async () => {
        const name = prompt('Name this device', deviceName());
        if (name === null) return;
        try { await passkeyRegister(name); toast('Biometrics enabled'); await refreshSettingsData(); renderView(); } catch (x) { toast(passkeyError(x), true); }
      } }, icon('plus'), 'Add this device'),
      S.passkeys.length ? h('button', { class: 'btn', onclick: lockNow }, icon('lock'), 'Lock now') : null));
  }

  // Password
  const cur = h('input', { class: 'input', type: 'password', placeholder: 'Current password', autocomplete: 'current-password' });
  const next = h('input', { class: 'input', type: 'password', placeholder: 'New password (min. 8 characters)', autocomplete: 'new-password' });
  const perr = h('p', { class: 'error' });
  const pwForm = h('form', { class: 'card section', onsubmit: async (e) => {
    e.preventDefault();
    perr.textContent = '';
    try { await api('POST', '/api/password', { current: cur.value, new: next.value }); cur.value = next.value = ''; toast('Password changed, other devices signed out'); } catch (x) { perr.textContent = x.message; }
  } }, h('h2', null, icon('shield'), 'Password'), cur, next, perr, h('div', null, h('button', { class: 'btn', type: 'submit' }, 'Change password')));

  // Data
  const file = h('input', { type: 'file', accept: '.json,.csv,application/json,text/csv', style: 'display:none', onchange: importFile });
  const data = h('div', { class: 'card section' }, h('h2', null, icon('download'), 'Backup & import'),
    h('p', { class: 'note' }, 'Export everything as JSON, or import a JSON export or a CSV (name, email, username, folder, url, 2fa…). Bitwarden and Vaultwarden exports work too. The file is read only in your browser and is never uploaded: passwords, TOTP secrets and notes are left out.'),
    h('div', { class: 'row', style: 'justify-content:flex-start' },
      h('button', { class: 'btn', onclick: exportData }, icon('download'), 'Export JSON'),
      h('button', { class: 'btn', onclick: () => file.click() }, icon('upload'), 'Import file'), file));

  // About
  const ver = S.version;
  const about = h('div', { class: 'card section' }, h('h2', null, 'About'),
    h('div', { class: 'kv' }, h('div', { class: 'row' }, h('span', null, 'Version'), h('strong', null, (ver && ver.version) || S.session.version))),
    ver && ver.updateAvailable ? h('div', { class: 'banner info' }, `Version ${ver.latest} is available. Run `, h('code', null, 'update'), ' inside the container.') : null,
    h('div', { class: 'row', style: 'justify-content:flex-start' },
      h('a', { class: 'btn', href: 'https://github.com/Bagareal/Maildex', target: '_blank', rel: 'noopener noreferrer' }, icon('external'), 'GitHub'),
      h('button', { class: 'btn', onclick: async () => { try { await api('POST', '/api/logout'); } catch { /* ignore */ } S.session = null; boot(); } }, icon('logout'), 'Sign out')));

  append(v, bio, pwForm, data, about);
}

async function exportData() {
  try {
    const res = await fetch('/api/export', { credentials: 'same-origin' });
    if (!res.ok) throw new Error('Export failed');
    const url = URL.createObjectURL(await res.blob());
    const a = h('a', { href: url, download: `maildex-export-${new Date().toISOString().slice(0, 10)}.json` });
    document.body.append(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  } catch (x) { toast(x.message, true); }
}

// ---------------------------------------------------------------- import
function parseCSV(text) {
  text = text.replace(/^﻿/, '');
  const first = text.split(/\r?\n/, 1)[0];
  const delim = [',', ';', '\t'].map((d) => [d, first.split(d).length]).sort((a, b) => b[1] - a[1])[0][0];
  const rows = [];
  let row = [], cur = '', quoted = false;
  const endRow = () => { row.push(cur); cur = ''; if (row.some((c) => c.trim() !== '')) rows.push(row); row = []; };
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (quoted) {
      if (c === '"') { if (text[i + 1] === '"') { cur += '"'; i++; } else quoted = false; } else cur += c;
    } else if (c === '"') quoted = true;
    else if (c === delim) { row.push(cur); cur = ''; }
    else if (c === '\n' || c === '\r') { if (c === '\r' && text[i + 1] === '\n') i++; endRow(); }
    else cur += c;
  }
  endRow();
  return rows;
}

const CSV_ALIASES = {
  name: ['name', 'service', 'site', 'title', 'website name'],
  email: ['email', 'e-mail', 'mail', 'account email'],
  username: ['username', 'login', 'login_username', 'user', 'user name'],
  url: ['url', 'website', 'login_uri', 'uri', 'link'],
  category: ['category', 'folder', 'group', 'type'],
  status: ['status'],
  notes: ['notes', 'note'],
  renewal: ['renewal', 'renews', 'expires', 'renewal date'],
  loginMethod: ['method', 'login method', 'loginmethod', 'sign-in method'],
  twoFactor: ['2fa', 'mfa', 'two factor', 'two-factor', 'twofactor', '2-step', 'otp'],
};

// Turns free text like "OTP, passkey; sms" into the app's 2FA ids.
function parseTwoFactor(text) {
  const out = new Set();
  for (const tok of text.toLowerCase().split(/[,;|/+]+/).map((s) => s.trim()).filter(Boolean)) {
    if (/passkey/.test(tok)) out.add('passkey');
    else if (/otp|authenticator/.test(tok)) out.add('totp');
    else if (/security|fido|yubi|u2f|hardware|key/.test(tok)) out.add('security-key');
    else if (/sms|text/.test(tok)) out.add('sms');
    else if (/mail/.test(tok)) out.add('email');
    else if (/push|prompt/.test(tok)) out.add('push');
    else if (/backup|recovery/.test(tok)) out.add('backup');
  }
  return [...out];
}

// Password-manager exports contain real secrets. Everything below runs in the
// browser and builds the upload from an explicit list of harmless fields, so
// passwords, TOTP secrets, cards and notes never leave the file picker. The
// file itself is never sent to the server.
const SERVICE_FIELDS = ['name', 'email', 'username', 'loginMethod', 'folders', 'category' /* pre-folders exports */, 'status', 'twoFactor', 'url', 'renewal', 'notes'];
const FOLDER_FIELDS = ['name', 'color'];
const EMAIL_FIELDS = ['address', 'label', 'provider', 'recovery', 'notes', 'atRisk'];
const pick = (obj, keys) => Object.fromEntries(keys.filter((k) => obj && obj[k] !== undefined).map((k) => [k, obj[k]]));

// A login's username is usually the email; when it is, it becomes the email and the username is left blank.
function splitIdentity(email, username) {
  email = (email || '').trim().toLowerCase();
  username = (username || '').trim();
  if (!email && username.includes('@')) { email = username.toLowerCase(); username = ''; }
  return { email, username };
}

function csvToImport(text) {
  const rows = parseCSV(text);
  if (rows.length < 2) throw new Error('The CSV has no data rows.');
  const head = rows[0].map((c) => c.trim().toLowerCase());
  const col = {};
  for (const [key, names] of Object.entries(CSV_ALIASES)) col[key] = head.findIndex((c) => names.includes(c));
  if (col.name < 0) throw new Error('The CSV needs a "name" (or "service") column.');
  const get = (r, k) => (col[k] >= 0 ? (r[col[k]] || '').trim() : '');
  // Exports with a password column (Bitwarden, Chrome, ...) often hide secrets in notes too: leave notes out.
  const hasSecrets = head.some((c) => /password|totp/.test(c));
  const typeCol = head.includes('login_password') ? head.indexOf('type') : -1; // Bitwarden: login/note/card/identity
  const totpCol = head.findIndex((c) => c === 'login_totp' || c === 'totp');
  const services = [];
  let skipped = 0;
  for (const r of rows.slice(1)) {
    if (typeCol >= 0 && (r[typeCol] || '').trim().toLowerCase() !== 'login') continue;
    const { email, username } = splitIdentity(get(r, 'email'), get(r, 'username'));
    if (!email) { skipped++; continue; }
    const twoFactor = parseTwoFactor(get(r, 'twoFactor'));
    if (totpCol >= 0 && (r[totpCol] || '').trim() && !twoFactor.includes('totp')) twoFactor.push('totp'); // only the fact, never the secret
    services.push({ name: get(r, 'name'), email, username, url: get(r, 'url'), folders: get(r, 'category').split('|').map((x) => x.trim()).filter(Boolean), status: get(r, 'status'), notes: hasSecrets ? '' : get(r, 'notes'), renewal: get(r, 'renewal'), loginMethod: get(r, 'loginMethod'), twoFactor });
  }
  return { payload: { emails: [], services }, skipped, notesLeftOut: hasSecrets };
}

function bitwardenToImport(j) {
  if (j.encrypted) throw new Error('This Bitwarden export is encrypted. Export it again as unencrypted JSON or CSV.');
  const folders = new Map((j.folders || []).map((f) => [f.id, f.name]));
  const services = [];
  let skipped = 0;
  for (const it of j.items || []) {
    if (it.type !== 1) continue; // logins only: notes, cards and identities are ignored
    const login = it.login || {};
    const { email, username } = splitIdentity('', login.username);
    if (!email) { skipped++; continue; }
    const twoFactor = [];
    if (login.totp) twoFactor.push('totp'); // only the fact, never the secret
    if ((login.fido2Credentials || []).length) twoFactor.push('passkey');
    const uri = ((login.uris || [])[0] || {}).uri || '';
    services.push({ name: it.name || '', email, username, url: /^https?:\/\//i.test(uri) ? uri : '', folders: folders.get(it.folderId) ? [folders.get(it.folderId)] : [], twoFactor });
  }
  return { payload: { emails: [], services }, skipped, notesLeftOut: true };
}

function parseImport(name, text) {
  if (/\.csv$/i.test(name)) return csvToImport(text);
  const j = JSON.parse(text);
  if (Array.isArray(j.items)) return bitwardenToImport(j); // Bitwarden / Vaultwarden JSON
  if (Array.isArray(j.services) || Array.isArray(j.emails)) { // Maildex's own export
    return { payload: { emails: (j.emails || []).map((e) => pick(e, EMAIL_FIELDS)), folders: (j.folders || []).map((f) => pick(f, FOLDER_FIELDS)), services: (j.services || []).map((s) => pick(s, SERVICE_FIELDS)) }, skipped: 0, notesLeftOut: false };
  }
  throw new Error('Unrecognised file. Use a Maildex JSON export, a Bitwarden/Vaultwarden export, or a CSV.');
}

async function importFile(e) {
  const f = e.target.files[0];
  e.target.value = '';
  if (!f) return;
  try {
    const { payload, skipped, notesLeftOut } = parseImport(f.name, await f.text());
    const r = await api('POST', '/api/import', payload);
    await loadData();
    renderView();
    const noEmail = skipped ? ` ${skipped} had no email address and were skipped.` : '';
    toast(`Imported ${r.servicesAdded} service${r.servicesAdded === 1 ? '' : 's'}, ${r.emailsAdded} new email${r.emailsAdded === 1 ? '' : 's'}${r.foldersAdded ? ` and ${r.foldersAdded} folder${r.foldersAdded === 1 ? '' : 's'}` : ''}.${noEmail}${notesLeftOut ? ' Notes were left out.' : ''}`);
  } catch (x) { toast(x.message || 'Import failed', true); }
}

// ---------------------------------------------------------------- activity & lifecycle
function noteActivity() {
  const now = Date.now();
  S.lastActivity = now;
  // Keep the server-side idle timer in step with real use (typing in the search box makes no requests).
  if (S.session && S.session.authenticated && !S.session.locked && now - S.lastPing > 30000) {
    S.lastPing = now;
    api('POST', '/api/ping').catch(() => {});
  }
}
['pointerdown', 'keydown', 'scroll'].forEach((ev) => addEventListener(ev, noteActivity, { passive: true, capture: true }));

setInterval(() => {
  const mins = S.data.settings.lockMinutes;
  if (S.session && S.session.authenticated && !S.session.locked && S.session.biometric && mins > 0 && Date.now() - S.lastActivity > mins * 60000) lockNow();
}, 15000);

document.addEventListener('visibilitychange', async () => {
  if (document.visibilityState !== 'visible' || !S.session || !S.session.authenticated) return;
  // Re-check on return: the server may have locked the session meanwhile.
  if (document.querySelector('.overlay')) return;
  boot();
});

if ('serviceWorker' in navigator) navigator.serviceWorker.register('/sw.js').catch(() => {});

boot();
