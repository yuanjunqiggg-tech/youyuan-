// 生成账号页真实 DOM 的验证页：CSS 加载顺序与 index.html 完全一致，DOM 结构照抄 youyuan_accounts.js 的渲染输出。
const fs = require("fs");
const path = require("path");

const FE = "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/upstream/frontend";
const idx = fs.readFileSync(path.join(FE, "index.html"), "utf8");
const links = (idx.match(/<link[^>]*rel="stylesheet"[^>]*>/g) || []).join("\n");

const accounts = [
  { name: "雨落青山", server: "mc.yoyuan.example:19132", status: "已连接", online: "on", role: "main", roleLabel: "主账号", initial: "雨", uid: "2947 1183 5520 4410" },
  { name: "半盏灯", server: "mc.yoyuan.example:19132", status: "内核在线 · 游戏未连接", online: "unknown", role: "owner", roleLabel: "主控分类", initial: "半", uid: "2947 1183 5520 4411" },
  { name: "北岸", server: "survival.yoyuan.example:19133", status: "状态待查询", online: "unknown", role: "bot", roleLabel: "机器人账号", initial: "北", uid: "未绑定" },
  { name: "南风知我意", server: "survival.yoyuan.example:19133", status: "内核未启动", online: "off", role: "bot", roleLabel: "机器人账号", initial: "南", uid: "2947 1183 5520 4412" },
  { name: "一只咸鱼", server: "尚未配置服务器", status: "未托管", online: "off", role: "cloud", roleLabel: "云端保存", initial: "一", uid: "2947 1183 5520 4413" },
  { name: "凌晨四点", server: "mc.yoyuan.example:19132", status: "已连接", online: "on", role: "bot", roleLabel: "机器人账号", initial: "凌", uid: "2947 1183 5520 4414" }
];

const chipIcon = '<svg class="yy-acct-chip-icon" viewBox="0 0 12 12" aria-hidden="true"><circle cx="6" cy="6" r="5"/></svg>';

function avatar(a, cls) {
  return '<span class="yy-compact-avatar yy-avatar-frame yy-avatar-fallback ' + (cls || "") + '" role="img" aria-label="账号头像">' +
    '<span class="yy-avatar-initial" aria-hidden="true">' + a.initial + '</span></span>';
}

function hero(a) {
  return '<div class="yy-acct-hero">' +
    '<div class="yy-acct-figure" role="img" aria-label="' + a.name + ' 的皮肤形象">' +
      '<svg class="yy-acct-figure-ph" viewBox="0 0 24 34" aria-hidden="true" fill="currentColor">' +
        '<ellipse cx="12" cy="32.6" rx="7.6" ry="1.2" opacity=".1"/>' +
        '<path opacity=".26" d="M3.5 25 12 20.5 20.5 25 12 29.5Z"/>' +
        '<path opacity=".34" d="M13.89 21.5 13.89 11.65 3.82 8.13 3.82 17.98Z"/>' +
        '<path opacity=".9" d="M20.18 6.02 13.89 11.65 3.82 8.13 10.11 2.5Z"/>' +
        '<path opacity=".62" d="M20.18 15.87 13.89 21.5 13.89 11.65 20.18 6.02Z"/>' +
      '</svg>' +
      '<span class="yy-acct-figure-tip">还没有可用的游戏身份</span>' +
    '</div>' +
    '<div class="yy-acct-hero-main">' +
      '<div class="yy-acct-hero-line">' + avatar(a, "yy-acct-hero-avatar") +
        '<div class="yy-acct-hero-text"><strong>' + a.name + '</strong>' +
        '<span class="yy-acct-hero-tags"><em class="yy-acct-chip" data-role="' + a.role + '">' + chipIcon + '<span>' + a.roleLabel + '</span></em>' +
        '<em class="yy-acct-live" data-online="' + a.online + '">' + a.status + '</em></span>' +
        '<div class="yy-acct-hero-facts">' +
          '<div><span>服务器</span><b>' + a.server + '</b></div>' +
          '<div><span>游戏 UID</span><b>' + a.uid + '</b></div>' +
          '<div><span>连接</span><b>' + a.status + '</b></div>' +
        '</div></div>' +
      '</div>' +
      '<div class="yy-acct-hero-acts">' +
        '<button class="btn">切换到此账号</button>' +
        '<button class="btn-s">解绑玩家</button>' +
        '<button class="btn-s">查看令牌</button>' +
      '</div>' +
      '<div class="yy-acct-token"><span class="yy-acct-token-label">令牌</span>' +
        '<code data-acct-token="' + a.name + '" data-shown="0">已保存 · 默认打码</code>' +
        '<button class="btn-s">复制</button></div>' +
    '</div></div>' +
    '<div class="yy-acct-skin-bar">' +
      '<button type="button" class="yy-acct-skin-btn is-on">走动</button>' +
      '<button type="button" class="yy-acct-skin-btn">纤细</button>' +
      '<button type="button" class="yy-acct-skin-btn">正视角</button>' +
      '<span class="yy-acct-skin-hint">拖动旋转 · 双击复位</span></div>';
}

function localBlock(a) {
  return '<section class="yy-linked-local"><h4>主连接</h4>' +
    '<div class="yy-account-meta">' +
      '<span>游戏 UID <b>' + a.uid + '</b></span>' +
      '<span>服务器 <b>' + a.server + '</b></span>' +
      '<span>连接状态 <b>' + a.status + '</b></span>' +
      '<span>账号凭据 <b>已保存令牌</b></span>' +
      '<span>自动重连 <b>已启用</b></span>' +
      '<span>账号标识 <b>yy_' + a.initial + '9f3a2c</b></span>' +
    '</div>' +
    '<p class="yy-hint">插件：自动巡逻、自动补给</p>' +
    '<div class="yy-actions"><button class="btn-s">编辑连接</button><button class="btn-s">启动内核</button><button class="btn-s yy-cred-danger">删除</button></div>' +
    '</section>';
}

function card(a, open) {
  return '<details class="card yy-account-card" data-account-id="' + a.initial + '" data-account-key="' + a.initial + '"' + (open ? " open" : "") + '>' +
    '<summary class="yy-account-summary">' + avatar(a) +
      '<span class="yy-compact-identity"><span class="yy-acct-nameline">' +
        '<i class="yy-acct-dot" data-online="' + a.online + '" aria-hidden="true"></i>' +
        '<strong>' + a.name + '</strong>' +
        '<em class="yy-acct-chip" data-role="' + a.role + '">' + chipIcon + '<span>' + a.roleLabel + '</span></em>' +
      '</span><small>' + a.server + '</small></span>' +
      '<span class="yy-compact-status">' + a.status + '</span>' +
      '<span class="yy-row-chevron" aria-hidden="true">›</span>' +
    '</summary>' +
    '<div class="yy-account-detail">' + hero(a) + localBlock(a) + '</div>' +
    '</details>';
}

const overview =
  '<div><strong>6</strong><span>游戏账号</span></div>' +
  '<div><strong>2</strong><span>已确认连接 · 待查询 2</span></div>' +
  '<div><strong>2</strong><span>服务器项目</span></div>' +
  '<div><strong>1</strong><span>绑定中的机器人</span></div>';

const projects =
  '<span>游戏项目</span><button class="btn-s selected">全部账号</button>' +
  '<button class="btn-s">mc.yoyuan.example:19132 · 3 个</button>' +
  '<button class="btn-s">survival.yoyuan.example:19133 · 2 个</button>';

const heading = '<div class="yy-heading"><div><h2>账号与项目</h2><p>游戏账号与服务器项目，统一管理。</p></div></div>';

const toolbar =
  '<div class="yy-actions yy-account-toolbar yy-cred-toolbar">' +
  '<button class="yy-primary">添加账号</button>' +
  '<button class="yy-secondary">刷新账号</button>' +
  '<button class="yy-secondary">皮肤库</button>' +
  '<button class="yy-secondary yy-acct-toolbar-wide" hidden>进入主账号</button></div>' +
  '<p class="yy-hint">第 5 个工作空间 · 当前操作：<b>雨落青山</b>。选择账号后，可连接服务器并管理对应的机器人。</p>';

const centerTabs =
  '<div class="yy-center-tabs">' +
  '<button class="btn-s">游戏账号</button>' +
  '<button class="btn-s">管理员托管池</button>' +
  '<button class="btn-s">注册与认证任务</button></div>';

const searchbar =
  '<div class="yy-account-toolbar">' +
  '<input placeholder="搜索昵称、UID 或服务器">' +
  '<select><option>每页 15 个</option><option>每页 30 个</option></select>' +
  '<label class="yy-cred-mainonly"><input type="checkbox" checked><span>只看主账号</span></label></div>';

// 分页区与 youyuan_accounts.js:656 现版输出一致：上一页 / 页码 span / 隐藏提示 / 下一页。
const pagination =
  '<div class="yy-pagination"><button class="btn-s" disabled>上一页</button>' +
  '<span>第 1 / 1 页 · 6 个账号 · 只看主账号</span>' +
  '<span class="yy-account-hidden-note">已开启「只看主账号」，另有 4 个账号未显示 · <a href="javascript:void(0)">查看全部</a></span>' +
  '<button class="btn-s" disabled>下一页</button></div>';

const monitor =
  '<section class="card yy-monitor">' +
  '<div class="yy-form-head"><h3>运行记录</h3>' +
  '<button type="button" class="yy-acct-picker-trigger yy-monitor-picker"><span>选择账号</span><i aria-hidden="true">▾</i></button></div>' +
  '<div class="yy-metrics"><span class="yy-badge">在线 3</span><span class="yy-badge">峰值 11</span><span class="yy-badge">采样 42 次</span></div>' +
  '<p class="yy-hint">数据来自本机内核，每 10 秒刷新一次。</p>' +
  '<details class="yy-monitor-trends"><summary>在线人数趋势与分布</summary>' +
  '<h4>在线人数 · 时间线</h4><div class="yy-chart"></div>' +
  '<h4>每小时人数分布 · 观测均值</h4><div class="yy-chart"></div></details>' +
  '<div class="yy-section-head"><h3>聊天记录</h3></div>' +
  '<div class="yy-chat-filters">' +
  '<label>搜索消息<input placeholder="输入消息关键词"></label>' +
  '<label>玩家<select><option>所有玩家</option></select></label>' +
  '<label>开始时间<input type="datetime-local"></label>' +
  '<label>结束时间（含此分钟）<input type="datetime-local"></label>' +
  '<label>汇总间隔<select><option>每小时</option></select></label>' +
  '<button class="btn-s">清空筛选</button></div>' +
  '<div class="yy-section-head"><h4>时段消息量 · 点击查看</h4></div>' +
  '<div class="yy-chat-bars"></div>' +
  '<div class="yy-chat-messages"></div></section>';

const page =
  heading + toolbar +
  '<div id="yy-account-overview" class="yy-account-overview">' + overview + '</div>' +
  centerTabs +
  '<div id="yy-pool-panel" hidden></div>' +
  '<div id="yy-game-accounts-panel">' +
    '<div id="yy-projects" class="yy-projects">' + projects + '</div>' +
    searchbar +
    '<div id="yy-accounts-grid" class="yy-form-grid yy-account-list">' +
      accounts.map((a, i) => card(a, i === 0)).join("") +
    '</div>' +
    pagination +
    '<p id="yy-account-cloud-status" class="yy-hint" role="status">正在同步已授权游戏凭据…</p>' +
  '</div>' +
  monitor;

const html = `<!DOCTYPE html><html lang="zh-CN" data-theme="light"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>账号页验证</title>
${links}
<style>
  /* 验证页专用：只关掉滚动容器的高度限制，让整页可截图；不参与被测样式。 */
  html,body{height:auto!important}
  #app{height:auto!important;display:flex;flex-direction:column;min-height:100vh}
  .content{overflow:visible!important;display:block!important}
  .nav{position:static!important;transform:none!important;left:auto!important;margin:24px auto 40px!important}
</style>
</head><body>
<div id="app">
<div class="conn-bar" id="conn-bar">
  <div class="cb-left"><h1>游园工具箱</h1></div>
  <div class="cb-center"><span class="cb-dot" id="gdot"></span><span class="cb-status" id="conn-st">已连接 · 本机内核</span></div>
  <div class="cb-right"><span class="cb-server" id="conn-srv">mc.yoyuan.example</span></div>
</div>
<div class="content show" id="tab-accounts"><div class="content-inner">${page}</div></div>
<div class="nav" id="bottom-nav">
<button class="active" data-tab="home"><i class="ni">▤</i>首页</button>
<button data-tab="tasks"><i class="ni">▦</i>插件</button>
<button data-tab="terminal"><i class="ni">›_</i>终端</button>
<button data-tab="settings"><i class="ni">⚙</i>设置</button>
<button data-tab="accounts"><i class="ni">◍</i>账号</button>
</div>
</div>
</body></html>`;

fs.writeFileSync(path.join(__dirname, "harness.html"), html, "utf8");
console.log("harness.html 已生成", html.length);

/* ===== 托管池子页面（youyuan_accounts.js:981-996 + 1128 + 1256 的 DOM 复刻）===== */
const poolAccounts = [
  { name: "巡逻甲", server: "mc.yoyuan.example:19132", uid: "2947 1183 5520 4410", player: "雨落青山", status: "已启用", when: "3 分钟前", online: "on", role: "bot", roleLabel: "机器人账号", initial: "巡" },
  { name: "巡逻乙", server: "mc.yoyuan.example:19132", uid: "2947 1183 5520 4411", player: "雨落青山", status: "未启用", when: "2 小时前", online: "off", role: "bot", roleLabel: "机器人账号", initial: "逻" },
  { name: "挖石机", server: "survival.yoyuan.example:19133", uid: "未绑定", player: "半盏灯", status: "缺少令牌", when: "", online: "unknown", role: "bot", roleLabel: "机器人账号", initial: "挖" }
];
const refreshIcon = '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M13.2 8a5.2 5.2 0 1 1-1.7-3.9" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/><path d="M13.4 2.2v3.2H10.2" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>';
const eyeIcon = '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M1.6 8S4.1 3.9 8 3.9 14.4 8 14.4 8 11.9 12.1 8 12.1 1.6 8 1.6 8Z" fill="none" stroke="currentColor" stroke-width="1.3"/><circle cx="8" cy="8" r="1.8" fill="currentColor"/></svg>';
function poolChip(label, on) {
  return '<button type="button" class="yy-pool-chip" data-on="' + (on ? "1" : "0") + '">' + label + '</button>';
}
function poolRow(a) {
  const meta = [a.server, a.uid ? "UID " + a.uid : "", "玩家 " + a.player].filter(Boolean).join(" · ");
  return '<div class="yy-pool-row" data-pool-row="' + a.initial + '">' +
    '<button type="button" class="yy-pool-hit">' + avatar(a) +
    '<span class="yy-compact-identity"><span class="yy-acct-nameline">' +
    '<i class="yy-acct-dot" data-online="' + a.online + '" aria-hidden="true"></i>' +
    '<strong>' + a.name + '</strong>' +
    '<em class="yy-acct-chip" data-role="' + a.role + '">' + chipIcon + '<span>' + a.roleLabel + '</span></em>' +
    '</span><small>' + meta + '</small></span></button>' +
    '<span class="yy-pool-side"><span class="yy-pool-live">' + a.status + (a.when ? "<em>" + a.when + "</em>" : "") + '</span>' +
    '<button type="button" class="yy-pool-act" aria-label="刷新状态">' + refreshIcon + '</button>' +
    '<button type="button" class="yy-pool-act" aria-label="查看详情">' + eyeIcon + '</button></span></div>';
}
function poolFacet(label, chips) {
  return '<div class="yy-pool-facet"><span class="yy-pool-facet-label">' + label + '</span><span class="yy-pool-chips">' + chips + '</span></div>';
}
const poolPage =
  '<div class="sp-header"><button class="sp-back">‹</button><span class="sp-title">机器人托管池</span></div>' +
  '<div class="sp-body"><div class="yy-page-intro"><h2>机器人托管池</h2><p>统一查看多台机器人，按时间、身份和玩家筛选。</p></div>' +
  '<section class="card yy-admin-list-panel yy-pool-panel">' +
  '<div class="yy-pool-searchbar"><label class="yy-search"><input type="search" placeholder="搜索昵称、UID、备注或玩家"></label><button class="btn-s" type="button">刷新</button></div>' +
  '<div class="yy-pool-facet"><span class="yy-pool-facet-label">最近上线</span><span class="yy-pool-chips">' + poolChip("全部", true) + poolChip("今天") + poolChip("7 天") + poolChip("30 天") + '</span></div>' +
  poolFacet("分组", poolChip("平铺", true) + poolChip("按玩家")) +
  poolFacet("状态", poolChip("全部", true) + poolChip("已启用") + poolChip("未启用") + poolChip("已绑定") + poolChip("空闲") + poolChip("缺令牌")) +
  poolFacet("身份", poolChip("全部", true) + poolChip("主账号") + poolChip("主控") + poolChip("机器人")) +
  '<div class="yy-pool-facet"><span class="yy-pool-facet-label">玩家</span><span class="yy-pool-chips">' + poolChip("雨落青山 · 2", true) + poolChip("半盏灯 · 1") + '</span></div>' +
  '<p class="yy-hint" role="status">共 3 个托管账号</p>' +
  '<div id="yy-pool-list" class="yy-compact-list">' +
  '<details class="yy-pool-group" open><summary class="yy-pool-group-sum"><strong>雨落青山</strong><span class="yy-pool-group-meta">2 个账号</span><span class="yy-row-chevron" aria-hidden="true">›</span></summary><div class="yy-pool-group-body">' + poolRow(poolAccounts[0]) + poolRow(poolAccounts[1]) + '</div></details>' +
  '<details class="yy-pool-group"><summary class="yy-pool-group-sum"><strong>半盏灯</strong><span class="yy-pool-group-meta">1 个账号</span><span class="yy-row-chevron" aria-hidden="true">›</span></summary><div class="yy-pool-group-body">' + poolRow(poolAccounts[2]) + '</div></details>' +
  '</div>' +
  '<div class="yy-pagination"><button class="btn-s" disabled>上一页</button><span>第 1 / 1 页 · 3 个账号</span><button class="btn-s" disabled>下一页</button></div>' +
  '</section></div>';
const poolHtml = `<!DOCTYPE html><html lang="zh-CN" data-theme="light"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>托管池验证</title>
${links}
<style>
  html,body{height:auto!important}
  #app{height:auto!important;display:flex;flex-direction:column;min-height:100vh}
  .sub-page{position:static!important;display:flex!important;overflow:visible!important;height:auto!important}
  .sp-body{overflow:visible!important;flex:1!important}
  .content{display:none!important}
  .nav{display:none!important}
</style>
</head><body class="sp-open">
<div id="app">
<div class="conn-bar" id="conn-bar">
  <div class="cb-left"><h1>游园工具箱</h1></div>
  <div class="cb-center"><span class="cb-dot" id="gdot"></span><span class="cb-status" id="conn-st">已连接 · 本机内核</span></div>
  <div class="cb-right"><span class="cb-server" id="conn-srv">mc.yoyuan.example</span></div>
</div>
<div class="sub-page show" id="sub-page">${poolPage}</div>
</div>
</body></html>`;
fs.writeFileSync(path.join(__dirname, "harness-pool.html"), poolHtml, "utf8");
console.log("harness-pool.html 已生成", poolHtml.length);
