/* Plugin inventory: one row per installed plugin, with its bound workspaces and
 * account instances inside that row. Loaded after the older Rplg decorators. */
(function () {
  'use strict';
  const labels = {lua:'Lua',wordbank:'词库',python:'Python',native_cpp:'C++'};
  const states = {running:'运行中',waiting:'等待连接',stopped:'未启动',disabled:'已停用',error:'异常',workspace:'开发中'};
  const scopes = new Map();
  let sequence = 0;
  const pendingClicks = new Set();
  const actionIcons = {create:'plus',imports:'upload',market:'globe',refresh:'loop',directory:'folder',runtimeDirectory:'folder',backup:'file',runtimeExport:'file',delete:'trash',runtimeDelete:'trash',reload:'loop',chainTool:'next',downloads:'folder'};
  const icon = name => typeof ICONS === 'object' && ICONS[name] ? '<span class="yps-icon" aria-hidden="true">' + ICONS[name] + '</span>' : '';
  const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
  const scope = () => yyReadScope();
  const admin = () => typeof yyAccountAdmin === 'function' ? yyAccountAdmin() : typeof yyPlatformUser === 'object' && yyPlatformUser?.role === 'admin';
  function errorHint(error) {
    if (/权限|permission|forbidden|unauthorized/i.test(error)) return '暂无操作权限，请联系管理员开通后重试。';
    if (/连接|离线|network|timeout|超时/i.test(error)) return '暂时无法连接，请检查连接状态后刷新重试。';
    return '暂时无法完成，请检查插件配置后重试。';
  }
  function diagnostics(error, id) { return admin() ? '<details class="yps-doc"><summary>管理员诊断</summary><pre>' + esc(id ? '插件：' + id + '\n' : '') + esc(error) + '</pre></details>' : ''; }
  function view() {
    const key = scope();
    if (!scopes.has(key)) {
      if (scopes.size >= 8) scopes.delete(scopes.keys().next().value);
      scopes.set(key, {query:'',language:'all',state:'all',source:'all',group:'state',open:new Set(),rows:[],data:null});
    }
    return scopes.get(key);
  }
  function preferences() {
    const user = typeof yyPlatformUser === 'object' && yyPlatformUser ? yyPlatformUser : {};
    let device = 'local';
    try { device = new URL(location.href).searchParams.get('device') || 'local'; } catch (_) {}
    const account = typeof yyActiveAccount === 'string' ? yyActiveAccount : 'main';
    const key = 'yy-plugin-updates:' + JSON.stringify([device,account,String(user.id || user.username || 'local-owner')]);
    try { return JSON.parse(localStorage.getItem(key) || '{}') || {}; } catch (_) { return {}; }
  }
  function collect(data, prefs) {
    const rows = [], spaces = data[0]?.ok ? (data[0].workspaces || []).filter(w => w.language !== 'mcfunction') : [];
    const runtime = data[1]?.ok ? data[1] : {};
    for (const p of data[2]?.ok ? data[2].plugins || [] : []) {
      if (p.type === 'mcfunction' || p.type === 'chain') continue;
      rows.push({key:'compat:' + p.id,id:p.id,name:p.name || p.id,language:p.type || 'lua',kind:'compat',
        state:p.disabled ? 'disabled' : p.state === 'error' ? 'error' : p.running ? 'running' : 'stopped',
        source:prefs.sources?.[p.id] ? 'linked' : 'installed',description:p.description || '',version:p.version || '',raw:p,instances:[],spaces:[]});
    }
    const installed = (runtime.installed || []).slice();
    for (const instance of runtime.instances || []) {
      if (instance.language === 'mcfunction' || instance.language === 'chain') continue;
      if (!installed.some(p => p.id === instance.id)) installed.push({id:instance.id,language:instance.language});
    }
    for (const p of installed) {
      if (p.language === 'mcfunction' || p.language === 'chain') continue;
      const instances = (runtime.instances || []).filter(item => item.id === p.id);
      rows.push({key:'runtime:' + p.id,id:p.id,name:p.name || p.id,language:p.language,kind:'runtime',
        state:instances.some(i => i.last_error) ? 'error' : instances.length ? 'running' : 'stopped',
        source:'installed',description:'已安装，可随时启动',version:p.version || '',raw:p,instances,spaces:[]});
    }
    for (const w of spaces) {
      const target = rows.find(p => p.language === w.language && (w.plugin_id ? p.id === w.plugin_id : p.kind === 'runtime' && p.id === w.id));
      if (target) target.spaces.push(w);
      else rows.push({key:'workspace:' + w.id,id:w.id,name:w.name || w.id,language:w.language,kind:'workspace',
        state:'workspace',source:'workspace',description:'继续编写，完成后发布为插件',version:'',raw:w,instances:[],spaces:[w]});
    }
    return rows.sort((a,b) => a.name.localeCompare(b.name,'zh'));
  }
  function filtered(rows, state) {
    const query = state.query.trim().toLocaleLowerCase();
    return rows.filter(p => (state.language === 'all' || p.language === state.language) &&
      (state.state === 'all' || p.state === state.state) && (state.source === 'all' || (state.source === 'installed' ? p.kind !== 'workspace' : p.source === state.source)) &&
      (!query || [p.name,p.id,p.description,labels[p.language] || p.language,...p.instances.map(i => i.account)].join(' ').toLocaleLowerCase().includes(query)));
  }
  function button(action, text, args, cls) {
    return '<button type="button" class="yps-button ' + (cls || '') + '" data-yps-action="' + action + '" data-yps-args="' + esc(JSON.stringify(args || [])) + '">' + icon(actionIcons[action]) + '<span>' + esc(text) + '</span></button>';
  }
  function stateLabel(p) {
    return p.kind === 'runtime' && p.instances.length > 1 ? p.instances.length + ' 个实例' : states[p.state];
  }
  function primary(p) {
    if (p.kind === 'workspace') return button('workspace','AI 帮写',[p.id,'agent'],'yps-primary');
    if (p.kind === 'compat') return button(p.state === 'disabled' ? 'enable' : 'switch',p.state === 'disabled' ? '启用' : p.raw.running ? '停止' : '启动',[p.id,p.state !== 'running' && !p.raw.running],p.raw.running ? '' : 'yps-primary');
    if (p.instances.length > 1) return button('expand','管理实例',[p.key]);
    const instance = p.instances[0];
    return button('runtime',instance ? '停止' : '启动',[p.id,instance?.account || currentAccount(),instance ? 'stop' : 'start'],instance ? '' : 'yps-primary');
  }
  function currentAccount() { return typeof yyActiveAccount === 'string' && yyActiveAccount ? yyActiveAccount : 'main'; }
  function detail(p) {
    let html = '<div class="yps-detail" id="yps-detail-' + esc(p.key) + '"><div class="yps-detail-info">' + (p.name.length > 25 ? '<strong>' + esc(p.name) + '</strong>' : '') + '<p>' + esc(p.description || '此插件未填写说明。') + '</p>' + (p.version ? '<span>版本：' + esc(p.version) + '</span>' : '') + '</div>';
    if (p.kind === 'compat') {
      if (p.raw.error) html += '<p class="yps-error">' + errorHint(p.raw.error) + '</p>' + diagnostics(p.raw.error,p.id);
      html += '<div class="yps-detail-section"><h4>编辑与资料</h4><div class="yps-actions">' + button('edit','编辑代码',[p.id]) + (p.language === 'lua' ? button('assist','AI 帮写',[p.id]) : '') + button('config','配置',[p.id]) + button('directory','文件与数据',[p.id]) + button('rename','名称与说明',[p.id,p.name,p.raw.description || '']) + '</div></div>';
      html += '<div class="yps-detail-section"><h4>运行与维护</h4><div class="yps-actions">' + button('reload','重新加载',[p.id]) + button(p.raw.disabled ? 'enable' : 'disable',p.raw.disabled ? '恢复自动启动' : '停用并关闭自动启动',[p.id]) + button('backup','备份 ZIP',[p.id]) + button('upload','上传到市场',[p.id]) + button('delete','删除插件',[p.id],'yps-danger') + '</div><p class="yps-note">停用后，下次打开也不会自动启动。删除前请先备份。</p></div>';
      html += '<details class="yps-doc"><summary>操作文档</summary><pre>' + esc(p.raw.docs || '此插件暂未提供操作文档。') + '</pre></details>';
    } else if (p.kind === 'runtime') {
      html += '<div class="yps-detail-section"><h4>账号实例</h4>';
      for (const i of p.instances) html += '<div class="yps-instance"><div><strong>' + esc(i.account === 'main' ? '主账号' : i.account) + '</strong><span>' + (i.last_error ? '需要检查' : '正在运行') + '</span>' + (i.last_error ? '<p class="yps-error">' + errorHint(i.last_error) + '</p>' + diagnostics(i.last_error,p.id) : '') + '</div><div class="yps-actions">' + button('runtime','触发',[p.id,i.account,'event']) + button('panel','控制面板',[p.id,i.account]) + button('runtime','重载',[p.id,i.account,'restart']) + button('runtime','停止',[p.id,i.account,'stop']) + '</div></div>';
      if (!p.instances.length) html += '<p class="yps-note">当前没有运行实例。启动目标：' + esc(currentAccount()) + '。</p>';
      html += '<div class="yps-actions">' + button('runtime','启动当前账号',[p.id,currentAccount(),'start']) + button('runtimeEdit','编辑 / AI',[p.id,p.language]) + button('runtimeDirectory','文件与数据',[p.id]) + button('runtimeExport','导出 ZIP',[p.id]) + button('runtimeDelete','删除插件',[p.id],'yps-danger') + '</div></div>';
    }
    if (p.spaces.length) {
      html += '<div class="yps-detail-section"><h4>创作草稿' + (p.kind !== 'workspace' ? ' · 已关联' : '') + '</h4>';
      for (const w of p.spaces) html += '<div class="yps-workspace"><div><strong>' + esc(w.name || w.id) + '</strong><span>' + esc(w.state || '等待编辑') + '</span></div><div class="yps-actions">' + button('workspace','代码',[w.id,'code']) + button('workspace','AI 帮写',[w.id,'agent']) + button('archive','归档工作区',[w.id]) + '</div></div>';
      html += '</div>';
    }
    return html + '</div>';
  }
  function row(p, state) {
    const language = labels[p.language] || p.language || '其他';
    const mark = ({lua:'Lu',python:'Py',native_cpp:'C++',wordbank:'词'})[p.language] || '＋';
    return '<details class="yps-row" data-yps-language="' + esc(p.language) + '" data-yps-key="' + esc(p.key) + '" data-yy-plugin="' + esc(p.id) + '"' + (state.open.has(p.key) ? ' open' : '') + '><summary aria-controls="yps-detail-' + esc(p.key) + '"><span class="yps-chevron" aria-hidden="true">›</span><span class="yps-language" aria-hidden="true"><b>' + esc(mark) + '</b><i></i></span><span class="yps-identity"><strong>' + esc(p.name) + '</strong><small>' + esc(p.description || language + ' 插件') + '</small><span class="yps-item-tags"><span>' + esc(language) + '</span>' + (p.version ? '<span>v' + esc(p.version) + '</span>' : '') + (p.spaces.length && p.kind !== 'workspace' ? '<span>已关联草稿</span>' : '') + '</span></span><span class="yps-row-meta"><span class="yps-badge yps-state-' + p.state + '" data-yps-label="' + esc(stateLabel(p)) + '">' + esc(stateLabel(p)) + '</span><small>' + esc(p.kind === 'workspace' ? '等待发布' : p.kind === 'runtime' && p.instances.length ? p.instances.map(i => i.account === 'main' ? '主账号' : i.account).join(' · ') : p.raw.running ? '插件已启动' : '按需启动') + '</small></span><span class="yps-primary-action">' + primary(p) + '</span></summary><p class="yps-row-status" role="status" hidden></p>' + detail(p) + '</details>';
  }
  function groups(rows, by) {
    const names = by === 'language' ? labels : by === 'source' ? {installed:'已安装',linked:'已关联更新',workspace:'创作草稿'} : states;
    const order = by === 'state' ? ['running','waiting','error','stopped','disabled','workspace'] : [];
    const map = new Map();
    for (const p of rows) { const key = p[by]; if (!map.has(key)) map.set(key,[]); map.get(key).push(p); }
    return Array.from(map, ([key,items]) => ({key,label:names[key] || key,items})).sort((a,b) => order.length ? order.indexOf(a.key) - order.indexOf(b.key) : a.label.localeCompare(b.label,'zh'));
  }
  function paintRows() {
    const state = view(), host = document.getElementById('yps-inventory');
    if (!host) return;
    const rows = filtered(state.rows,state);
    host.innerHTML = rows.length ? groups(rows,state.group).map(g => '<section class="yps-group"><div class="yps-group-heading"><h3>' + esc(g.label) + '</h3><span>' + g.items.length + '</span></div>' + g.items.map(p => row(p,state)).join('') + '</section>').join('') : '<div class="yps-empty">' + icon(state.rows.length ? 'search' : 'folder') + '<strong>' + (state.rows.length ? '没有匹配的插件' : '从第一个插件开始') + '</strong><p>' + (state.rows.length ? '试试其他名称、语言或状态。' : '导入已有插件，或创建一个新插件。') + '</p>' + button(state.rows.length ? 'reset' : 'create',state.rows.length ? '清除筛选' : '创建插件') + '</div>';
    document.getElementById('yps-match-count').textContent = rows.length + ' / ' + state.rows.length + ' 项';
    host.querySelectorAll('.yps-row').forEach(el => el.addEventListener('toggle',() => { if (el.isConnected) el.open ? state.open.add(el.dataset.ypsKey) : state.open.delete(el.dataset.ypsKey); }));
    yyPluginOperationUI();
  }
  function select(id, title, options, value) {
    return '<label><span>' + title + '</span><select id="' + id + '" data-yps-filter="' + id.replace('yps-','') + '">' + Object.entries(options).map(([key,text]) => '<option value="' + key + '"' + (value === key ? ' selected' : '') + '>' + text + '</option>').join('') + '</select></label>';
  }
  function render(data, failures) {
    const host = document.getElementById('tab-tasks');
    if (!host) return;
    const state = view(), prefs = preferences();
    // A failed refresh keeps the last successful source for this account.
    data = data.map((result,index) => result.ok ? result : state.data?.[index]?.ok ? state.data[index] : result);
    state.data = data; state.failures = failures; state.rows = collect(data,prefs);
    if (!failures.length) state.resultGeneration = typeof yyPluginResultSequence === 'number' ? yyPluginResultSequence : 0;
    const running = state.rows.filter(p => p.state === 'running' || p.instances.length).length;
    const installed = state.rows.filter(p => p.kind !== 'workspace').length;
    const drafts = data[0]?.ok ? (data[0].workspaces || []).filter(w => w.language !== 'mcfunction').length : '—';
    host.innerHTML = '<section class="yps-shell content-inner">' +
      '<header class="yps-header"><div class="yps-title-wrap"><span class="yps-title-icon" aria-hidden="true">' + icon('folder') + '</span><div><span class="yps-eyebrow">游园工具箱 / 扩展管理</span><h2>我的插件</h2><p>常用插件随手启动，代码、配置与文件一处管理。</p></div></div><div class="yps-header-actions">' + button('imports','导入插件') + button('create','创建插件',[],'yps-primary') + '</div></header>' +
      '<div class="yps-overview"><div class="yps-metric yps-metric-installed">' + icon('folder') + '<div><strong>' + installed + '</strong><span>已安装插件</span></div></div><div class="yps-metric yps-metric-running">' + icon('radio') + '<div><strong>' + running + '</strong><span>正在运行</span></div></div><div class="yps-metric yps-metric-drafts">' + icon('file') + '<div><strong>' + drafts + '</strong><span>创作草稿</span></div></div><div class="yps-overview-note">' + button('refresh','刷新状态') + '<span>以最近一次读取为准</span></div></div>' +
      '<div id="yy-plugin-operation-status" class="yps-operation" role="status" aria-live="polite" hidden></div><div id="yps-last-result" class="yps-result" role="status" aria-live="polite" hidden></div>' +
      (failures.length ? '<div class="yps-error yps-load-error" role="alert">' + failures.map(error => esc(errorHint(error))).join('<br>') + diagnostics(failures.join('\n')) + '<p>部分列表暂时无法读取，请刷新重试。</p></div>' : '') +
      '<div class="yps-layout"><aside class="yps-side"><span class="yps-section-label">插件库</span><nav aria-label="插件来源">' + [['all','全部项目'],['installed','已安装'],['linked','已关联更新'],['workspace','创作草稿']].map(([key,title]) => '<button type="button" data-yps-source="' + key + '" aria-pressed="' + (state.source === key) + '"><span>' + title + '</span><b>' + state.rows.filter(p => key === 'all' || (key === 'installed' ? p.kind !== 'workspace' : p.source === key)).length + '</b></button>').join('') + '</nav>' +
      '<div class="yps-discover"><span class="yps-section-label">发现与工具</span>' + button('market','浏览插件市场') + button('chainTool','命令链工具') + '</div><details class="yps-resources"><summary>资料与开发</summary><div>' + button('library','我的插件市场') + button('downloads','插件生成文件') + button('doc','Python 手册',['python-runtime']) + button('doc','C++ 手册',['cpp-runtime']) + '</div></details></aside>' +
      '<main class="yps-main"><div class="yps-toolbar"><label class="yps-search"><span>搜索插件</span><span class="yps-search-field">' + icon('search') + '<input id="yps-search" type="search" autocomplete="off" placeholder="名称、说明或账号…" value="' + esc(state.query) + '"></span></label><div class="yps-selects">' + select('yps-language','语言',{all:'全部语言',...labels},state.language) + select('yps-state','状态',{all:'全部状态',...states},state.state) + select('yps-group','分组',{state:'按运行状态',language:'按语言',source:'按来源'},state.group) + '</div></div>' +
      '<div class="yps-inventory-meta"><span id="yps-match-count"></span><span>展开查看配置与文件</span></div><div id="yps-inventory"></div>' +
      '<details class="yps-updates"><summary><span>' + icon('loop') + '来源与更新</span><small>检查版本 · 管理关联</small></summary><div><p class="yps-note">检查已关联的插件版本，有更新后再决定是否安装。</p><div class="yps-actions"><button type="button" class="yps-button" id="yy-plugin-check" onclick="yyCheckPluginUpdates()">检查更新</button>' + button('sources','关联更新来源') + '</div><label class="yps-auto"><input id="yy-plugin-auto-check" type="checkbox"' + (prefs.auto === true ? ' checked' : '') + '>进入插件页时自动检查更新</label><div id="yy-plugin-update-result" role="status"></div></div></details><pre id="yy-runtime-result" hidden></pre></main></div></section>';
    host.querySelector('#yps-search').addEventListener('input',event => { state.query = event.target.value; paintRows(); });
    host.querySelectorAll('[data-yps-filter]').forEach(el => el.addEventListener('change',() => { state[el.dataset.ypsFilter] = el.value; paintRows(); }));
    host.querySelectorAll('[data-yps-source]').forEach(el => el.addEventListener('click',() => { state.source = el.dataset.ypsSource; host.querySelectorAll('[data-yps-source]').forEach(b => b.setAttribute('aria-pressed',String(b === el))); paintRows(); }));
    host.querySelector('#yy-plugin-auto-check').addEventListener('change',event => yyPluginAutoCheck(event.target.checked));
    host.querySelector('.yps-shell').addEventListener('click',click);
    paintRows();
    if (prefs.auto === true && Date.now() - (prefs.lastCheck || 0) > 15 * 60 * 1000) yyCheckPluginUpdates();
  }
  async function click(event) {
    const el = event.target.closest('[data-yps-action]');
    if (!el) return;
    event.stopPropagation();
    // A button inside summary must not also toggle its parent details.
    if (el.closest('summary')) event.preventDefault();
    const action = el.dataset.ypsAction, args = JSON.parse(el.dataset.ypsArgs || '[]');
    if (action === 'expand') { el.closest('.yps-row').open = true; return; }
    if (action === 'reset') { const state = view(); state.query = ''; state.language = state.state = state.source = 'all'; render(state.data,state.failures || []); return; }
    const handlers = {create:yyCreateWorkspaceDialog,refresh:Rplg,imports:importDialog,market:pmOpenMarket,library:yyLibraryPage,downloads:yyOpenExportCenter,doc:yyOpenLanguageDoc,sources:yyPluginSourceDialog,workspace:yyOpenWorkspace,archive:yyArchiveProject,edit:plgEdit,assist:openAIAssist,config:plgEditConfig,directory:plgDataDir,rename:plgRename,reload:plgReload,enable:plgEnable,disable:plgDisable,switch:plgSwitch,backup:plgExportSAF,upload:pmOpenUpload,delete:plgDel,runtime:yyRuntimeAction,panel:yySDKOpenPanel,runtimeEdit:yyOpenInstalledRuntime,runtimeDirectory:yyRuntimeDirectoryView,runtimeExport:yyExportRuntimePlugin,runtimeDelete:yyDeleteInstalledRuntime,chainTool:yyChainsPage};
    const fn = handlers[action];
    if (!fn) return;
    const owner = scope(), html = el.innerHTML, key = JSON.stringify([owner,action,args]);
    const pluginRow = el.closest('.yps-row');
    if (el.disabled || pendingClicks.has(key) || pluginRow?.getAttribute('aria-busy') === 'true') return;
    pendingClicks.add(key);
    let timer;
    try {
      const result = fn.apply(null,args);
      if (result && typeof result.then === 'function') {
        el.disabled = true; el.textContent = '处理中…';
        timer = setTimeout(() => { if (el.isConnected && scope() === owner) el.textContent = '仍在处理…'; },5000);
        await result;
      }
    } catch (error) { if (scope() === owner) T(error.message || '操作失败','e'); }
    finally { pendingClicks.delete(key); clearTimeout(timer); if (el.isConnected && scope() === owner) { el.disabled = pluginRow?.getAttribute('aria-busy') === 'true'; el.innerHTML = html; } }
  }
  function importDialog() {
    const overlay = document.createElement('div'); overlay.className = 'modal-overlay';
    overlay.innerHTML = '<div class="modal-box yps-import-dialog"><h3>导入插件</h3><p class="yps-note">选择你要导入的插件类型。</p><button type="button" class="yps-button" data-import="runtime">Python / C++ 插件包</button><button type="button" class="yps-button" data-import="compat">Lua / 词库插件包</button><p class="yps-note">TXT / mcfunction 文件请到命令链工具管理。</p><button type="button" class="yps-button" data-import="chain-tool">打开命令链工具</button><button type="button" class="yps-button" data-import="close">取消</button></div>';
    overlay.addEventListener('click',event => { const kind = event.target.dataset.import; if (!kind) return; overlay.remove(); if (kind === 'runtime') yyImportPython(); if (kind === 'compat') plgImport(); if (kind === 'chain-tool') yyChainsPage(); });
    document.body.appendChild(overlay);
  }
  const baseOperationUI = yyPluginOperationUI;
  yyPluginOperationUI = function () {
    baseOperationUI();
    const ops = Array.from(yyPluginOperations.values()).filter(op => op.scope === scope());
    const results = typeof yyPluginLastResults === 'object' ? Array.from(yyPluginLastResults.values()).filter(op => op.scope === scope()) : [];
    function resultText(result) {
      return result.label + (result.status === 'success' ? '完成' : result.status === 'unknown' ? '结果未确认，请刷新核对，勿重复提交。' : '失败，请检查后重试。');
    }
    const latest = results[results.length - 1], summary = document.getElementById('yps-last-result');
    if (summary) {
      summary.hidden = !latest || ops.some(op => op.id === latest.id);
      if (!summary.hidden) { summary.dataset.result = latest.status; summary.textContent = latest.id + ' · ' + resultText(latest); }
    }
    document.querySelectorAll('.yps-row').forEach(row => {
      const op = ops.find(op => op.id === row.getAttribute('data-yy-plugin'));
      const result = results.find(op => op.id === row.getAttribute('data-yy-plugin'));
      const status = row.querySelector('.yps-row-status');
      const badge = row.querySelector('.yps-badge');
      status.hidden = !op && !result;
      if (op) { const seconds = Math.floor((Date.now() - op.started) / 1000); status.textContent = op.label + ' · 已等待 ' + seconds + ' 秒' + (seconds >= 5 ? '。请求仍在处理，请勿重复提交。' : '…'); badge.textContent = op.label.replace('插件','') + '中 · ' + seconds + '秒'; }
      else {
        if (result) status.textContent = resultText(result);
        badge.textContent = result && result.generation > (view().resultGeneration || 0) ? '正在核对状态' : badge.dataset.ypsLabel;
      }
    });
  };
  Rplg = async function () {
    const owner = scope(), epoch = ++yyPluginRenderEpoch, ticket = ++sequence;
    if (yyPluginScopeBusy(owner)) { yyPluginOperationUI(); return; }
    const host = document.getElementById('tab-tasks');
    if (host && !host.querySelector('.yps-shell')) host.innerHTML = '<div class="yps-loading" role="status">正在读取插件列表…</div>';
    const paths = ['/api/youyuan/agent/workspaces','/api/youyuan/runtime/list','/api/plugin/list'];
    const responses = await Promise.allSettled(paths.map(path => A('GET',path)));
    if (owner !== scope() || epoch !== yyPluginRenderEpoch || ticket !== sequence || yyPluginScopeBusy(owner) || (typeof curTab !== 'undefined' && curTab !== 'tasks')) return;
    const data = responses.map(result => result.status === 'fulfilled' ? result.value || {ok:false,error:'响应为空'} : {ok:false,error:result.reason?.message || '请求失败'});
    const failures = data.map((r,i) => r.ok ? '' : ['创作草稿','多语言运行环境','兼容插件'][i] + '：' + (r.error || '读取失败')).filter(Boolean);
    render(data,failures);
  };
  // Pure model is shared with the local inventory contract checks.
  window.yyPluginStudioModel = {collect,filtered,groups};
})();
