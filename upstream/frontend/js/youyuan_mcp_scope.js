// MCP 服务器的「归属 + 连接方式」面板。
//
// 为什么不去改 youyuan_workspace.js：那块是工作区渲染，由别人维护；
// 这里用挂载的方式在「外部 MCP」折叠块后面补一块自己的面板，
// 工作区每轮重绘后自动重新挂载，不侵入别人的 innerHTML 拼接。
//
// 回答的三个问题：
//   1. 这台 MCP 是「公用」（放服务器上，按权限给所有群友）还是「专属」（绑某个人自己的机器）
//   2. 谁能调：all / admin / owner_only
//   3. 怎么连：http / sse / stdio，地址或命令，请求头放密钥，点一下连通性测试真连

var yyMCPScopeState = { servers: [], editing: "" };

function yyMcpScopeEscape(v) {
  return String(v == null ? "" : v).replace(/[&<>"']/g, function (c) {
    return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
  });
}

function yyMcpScopeRoot() {
  return document.getElementById("yy-mcp-scope");
}

function yyMcpScopePanelHTML() {
  return (
    '<div class="yy-mcps-head"><strong>MCP 归属与连接</strong><span class="yy-badge">公用 / 专属</span></div>' +
    '<p class="yy-hint">公用 = 配在这台机器上，按权限给群友用；专属 = 绑在某个群友自己的机器上，只有他能调。' +
    "连接方式按你自己的情况选：公网直连、内网穿透、或本机进程。</p>" +
    '<div id="yy-mcps-list" class="yy-mcps-list"></div>' +
    '<fieldset class="yy-mcps-form">' +
    '<legend>新增 / 修改</legend>' +
    '<label for="yy-mcps-id">标识（英数，AI 用它认服务器）</label>' +
    '<input id="yy-mcps-id" placeholder="例如：filesystem 或 my-pc">' +
    '<label for="yy-mcps-name">显示名</label>' +
    '<input id="yy-mcps-name" placeholder="例如：我自己电脑上的文件服务">' +
    '<label for="yy-mcps-scope">归属</label>' +
    '<select id="yy-mcps-scope"><option value="public">公用（放这台机器上，按权限给群友）</option><option value="private">专属（绑某个群友自己的机器）</option></select>' +
    '<label for="yy-mcps-owner">归属人 QQ（专属必填）</label>' +
    '<input id="yy-mcps-owner" placeholder="群友的 QQ 号">' +
    '<label for="yy-mcps-access">谁可以调</label>' +
    '<select id="yy-mcps-access"><option value="all">所有群友</option><option value="admin">只有管理员</option><option value="owner_only">只有归属人</option></select>' +
    '<label for="yy-mcps-transport">连接方式</label>' +
    '<select id="yy-mcps-transport"><option value="http">HTTP（公网直连 / 官方通道）</option><option value="sse">SSE（内网穿透常见）</option><option value="stdio">本机进程（stdio）</option></select>' +
    '<div id="yy-mcps-url-wrap"><label for="yy-mcps-url">地址</label>' +
    '<input id="yy-mcps-url" placeholder="https://你的地址/mcp"></div>' +
    '<div id="yy-mcps-cmd-wrap"><label for="yy-mcps-cmd">启动命令</label>' +
    '<input id="yy-mcps-cmd" placeholder="例如：npx 或 C:/tools/mcp.exe">' +
    '<label for="yy-mcps-args">参数（一行一个）</label>' +
    '<textarea id="yy-mcps-args" rows="3" placeholder="-y&#10;@modelcontextprotocol/server-filesystem"></textarea></div>' +
    '<label for="yy-mcps-headers">请求头（每行 Key: Value，密钥放这里）</label>' +
    '<textarea id="yy-mcps-headers" rows="2" placeholder="Authorization: Bearer 你的token"></textarea>' +
    '<label for="yy-mcps-note">备注（方便换机器时想起来怎么打通的）</label>' +
    '<input id="yy-mcps-note" placeholder="例如：家里电脑 Cloudflare 隧道">' +
    '<label class="yy-mcps-ack"><input id="yy-mcps-enabled" type="checkbox" checked><span>启用</span></label>' +
    '<div class="yy-actions">' +
    '<button class="btn-s" type="button" onclick="yyMcpScopeSave()">保存</button>' +
    '<button class="btn-s" type="button" onclick="yyMcpScopeTest()">连通性测试</button>' +
    '<button class="btn-s" type="button" onclick="yyMcpScopeReset()">清空表单</button>' +
    "</div></fieldset>" +
    '<pre id="yy-mcps-result" class="yy-readout">还没测试过。点「连通性测试」会真的连一次，把握手的协议版本和工具数量回显出来。</pre>'
  );
}

function yyMcpScopeMount() {
  var anchor = document.getElementById("yy-mcp-external");
  if (!anchor || document.getElementById("yy-mcp-scope")) return;
  var panel = document.createElement("details");
  panel.className = "yy-mcp-external";
  panel.id = "yy-mcp-scope";
  panel.open = true;
  panel.innerHTML = "<summary>MCP 归属与连接方式（公用还是某个人自己的机器）</summary>" + yyMcpScopePanelHTML();
  anchor.parentNode.insertBefore(panel, anchor.nextSibling);
  document.getElementById("yy-mcps-scope").addEventListener("change", yyMcpScopeDerive);
  document.getElementById("yy-mcps-transport").addEventListener("change", yyMcpScopeDerive);
  yyMcpScopeLoad();
}

// private 的含义就是「只有主人能用」，所以归属人一填，访问范围自动收到 owner_only。
function yyMcpScopeDerive() {
  var scope = document.getElementById("yy-mcps-scope").value;
  var transport = document.getElementById("yy-mcps-transport").value;
  var access = document.getElementById("yy-mcps-access");
  var stdio = transport === "stdio";
  document.getElementById("yy-mcps-url-wrap").style.display = stdio ? "none" : "";
  document.getElementById("yy-mcps-cmd-wrap").style.display = stdio ? "" : "none";
  if (scope === "private") {
    access.value = "owner_only";
    access.disabled = true;
  } else {
    access.disabled = false;
  }
}

function yyMcpScopeDraw() {
  var host = document.getElementById("yy-mcps-list");
  if (!host) return;
  if (!yyMCPScopeState.servers.length) {
    host.innerHTML = '<p class="yy-hint">还没有登记 MCP。下面的「新增 / 修改」填完点保存即可。</p>';
    return;
  }
  var transportText = { http: "HTTP", sse: "SSE", stdio: "stdio" };
  var scopeText = { public: "公用", private: "专属" };
  var accessText = { all: "所有群友", admin: "管理员", owner_only: "归属人" };
  host.innerHTML = yyMCPScopeState.servers
    .map(function (s) {
      var target = s.transport === "stdio" ? s.command + " " + (s.args || []).join(" ") : s.url;
      var who = scopeText[s.scope] || s.scope;
      if (s.scope === "private") who += " · " + (s.owner || "未填归属人");
      return (
        '<div class="yy-mcps-row">' +
        "<div><strong>" + yyMcpScopeEscape(s.id) + "</strong>" +
        '<span class="yy-badge">' + yyMcpScopeEscape(transportText[s.transport] || s.transport) + "</span>" +
        '<span class="yy-badge">' + yyMcpScopeEscape(who) + "</span>" +
        '<span class="yy-badge">' + yyMcpScopeEscape(accessText[s.access] || s.access) + "</span>" +
        (s.enabled ? "" : '<span class="yy-badge yy-badge-risk">已停用</span>') +
        "<small>" + yyMcpScopeEscape(target) + (s.note ? " · " + yyMcpScopeEscape(s.note) : "") +
        (s.has_headers ? " · 请求头已设置" : "") + "</small></div>" +
        '<div class="yy-mcps-row-actions">' +
        '<button class="yy-text-btn" type="button" onclick="yyMcpScopeEdit(\'' + yyMcpScopeEscape(s.id) + "')\">编辑</button>" +
        '<button class="yy-text-btn" type="button" onclick="yyMcpScopeToggle(\'' + yyMcpScopeEscape(s.id) + "'," + (s.enabled ? "false" : "true") + ')">' + (s.enabled ? "停用" : "启用") + "</button>" +
        '<button class="yy-text-btn" type="button" onclick="yyMcpScopeTestId(\'' + yyMcpScopeEscape(s.id) + "')\">测试</button>" +
        '<button class="yy-text-btn" type="button" onclick="yyMcpScopeDelete(\'' + yyMcpScopeEscape(s.id) + "')\">删除</button>" +
        "</div></div>"
      );
    })
    .join("");
}

async function yyMcpScopeLoad() {
  var r = await A("GET", "/api/youyuan/mcp/scope");
  if (!r || r.ok !== true) {
    var out = document.getElementById("yy-mcps-result");
    if (out) out.textContent = "读取 MCP 登记表失败：" + ((r && r.error) || "服务没有响应");
    return;
  }
  yyMCPScopeState.servers = r.servers || [];
  yyMcpScopeDraw();
}

function yyMcpScopeHeadersText(s) {
  return (s.header_keys || []).map(function (k) {
    return k + ": （已保存，留空不改）";
  }).join("\n");
}

// 请求头解析：一行一条 "Key: Value"。整行留空视为不改（密钥不回显）。
function yyMcpScopeParseHeaders(text) {
  var out = {};
  String(text || "").split("\n").forEach(function (line) {
    var trimmed = line.trim();
    if (!trimmed) return;
    var at = trimmed.indexOf(":");
    if (at <= 0) return;
    var value = trimmed.slice(at + 1).trim();
    if (/（已保存，留空不改）|\(已保存\)/.test(value)) return;
    out[trimmed.slice(0, at).trim()] = value;
  });
  return out;
}

function yyMcpScopeForm() {
  var headers = yyMcpScopeParseHeaders(document.getElementById("yy-mcps-headers").value);
  var args = document.getElementById("yy-mcps-args").value.split("\n").map(function (v) {
    return v.trim();
  }).filter(Boolean);
  var server = {
    id: document.getElementById("yy-mcps-id").value.trim(),
    name: document.getElementById("yy-mcps-name").value.trim(),
    scope: document.getElementById("yy-mcps-scope").value,
    owner: document.getElementById("yy-mcps-owner").value.trim(),
    access: document.getElementById("yy-mcps-access").value,
    transport: document.getElementById("yy-mcps-transport").value,
    url: document.getElementById("yy-mcps-url").value.trim(),
    command: document.getElementById("yy-mcps-cmd").value.trim(),
    args: args,
    note: document.getElementById("yy-mcps-note").value.trim(),
    enabled: document.getElementById("yy-mcps-enabled").checked
  };
  if (Object.keys(headers).length) server.headers = headers;
  return server;
}

function yyMcpScopeFill(s) {
  document.getElementById("yy-mcps-id").value = s.id || "";
  document.getElementById("yy-mcps-name").value = s.name || "";
  document.getElementById("yy-mcps-scope").value = s.scope || "public";
  document.getElementById("yy-mcps-owner").value = s.owner || "";
  document.getElementById("yy-mcps-access").value = s.access || "all";
  document.getElementById("yy-mcps-transport").value = s.transport || "http";
  document.getElementById("yy-mcps-url").value = s.url || "";
  document.getElementById("yy-mcps-cmd").value = s.command || "";
  document.getElementById("yy-mcps-args").value = (s.args || []).join("\n");
  document.getElementById("yy-mcps-headers").value = yyMcpScopeHeadersText(s);
  document.getElementById("yy-mcps-note").value = s.note || "";
  document.getElementById("yy-mcps-enabled").checked = s.enabled !== false;
  yyMCPScopeState.editing = s.id || "";
  yyMcpScopeDerive();
}

function yyMcpScopeEdit(id) {
  var s = yyMCPScopeState.servers.filter(function (v) {
    return v.id === id;
  })[0];
  if (!s) return;
  yyMcpScopeFill(s);
  document.getElementById("yy-mcps-id").focus();
}

function yyMcpScopeReset() {
  yyMcpScopeFill({ id: "", name: "", scope: "public", owner: "", access: "all", transport: "http", url: "", command: "", args: [], note: "", enabled: true });
  yyMCPScopeState.editing = "";
}

function yyMcpScopeShow(data, prefix) {
  var out = document.getElementById("yy-mcps-result");
  if (!out) return;
  out.textContent = prefix + JSON.stringify(data, null, 2);
}

async function yyMcpScopeSave() {
  var server = yyMcpScopeForm();
  if (!server.id) {
    yyMcpScopeShow({ ok: false, error: "请先填标识" }, "");
    return;
  }
  var r = await A("POST", "/api/youyuan/mcp/scope", { op: "save", server: server });
  if (!r || r.ok !== true) {
    yyMcpScopeShow(r || { ok: false, error: "服务没有响应" }, "");
    return;
  }
  yyMcpScopeShow({ ok: true, saved: server.id, hint: "点「连通性测试」确认真的连得上" }, "");
  await yyMcpScopeLoad();
}

async function yyMcpScopeTest() {
  var server = yyMcpScopeForm();
  if (!server.id) {
    yyMcpScopeShow({ ok: false, error: "请先填标识" }, "");
    return;
  }
  var out = document.getElementById("yy-mcps-result");
  if (out) out.textContent = "正在连 " + (server.transport === "stdio" ? server.command : server.url) + " …";
  var r = await A("POST", "/api/youyuan/mcp/scope", { op: "test", id: server.id, server: server });
  yyMcpScopeShow(r || { ok: false, error: "服务没有响应" }, "");
}

async function yyMcpScopeTestId(id) {
  var out = document.getElementById("yy-mcps-result");
  if (out) out.textContent = "正在测试 " + id + " …";
  var r = await A("POST", "/api/youyuan/mcp/scope", { op: "test", id: id });
  yyMcpScopeShow(r || { ok: false, error: "服务没有响应" }, "");
}

async function yyMcpScopeToggle(id, enabled) {
  var r = await A("POST", "/api/youyuan/mcp/scope", { op: "quick", id: id, enabled: enabled });
  if (!r || r.ok !== true) {
    yyMcpScopeShow(r || { ok: false, error: "服务没有响应" }, "");
    return;
  }
  await yyMcpScopeLoad();
}

async function yyMcpScopeDelete(id) {
  if (!window.confirm("删除 MCP「" + id + "」？工作区里引用它的任务会连不上。")) return;
  var r = await A("POST", "/api/youyuan/mcp/scope", { op: "delete", id: id });
  if (!r || r.ok !== true) {
    yyMcpScopeShow(r || { ok: false, error: "服务没有响应" }, "");
    return;
  }
  await yyMcpScopeLoad();
}

// 工作区每轮重绘都会把我们的面板一起冲掉，所以监听 DOM 变化后补挂。
// 面板自己在 DOM 里时这里什么都不做，不会形成循环。
function yyMcpScopeBoot() {
  yyMcpScopeMount();
  if (typeof MutationObserver !== "function") return;
  var pending = false;
  var observer = new MutationObserver(function () {
    if (pending) return;
    pending = true;
    window.requestAnimationFrame(function () {
      pending = false;
      yyMcpScopeMount();
    });
  });
  observer.observe(document.body, { childList: true, subtree: true });
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", yyMcpScopeBoot);
} else {
  yyMcpScopeBoot();
}
