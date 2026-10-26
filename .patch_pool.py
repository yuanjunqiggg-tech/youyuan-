# -*- coding: utf-8 -*-
import io, sys

path = "upstream/frontend/js/youyuan_accounts.js"
with io.open(path, "r", encoding="utf-8", newline="") as fh:
    raw = fh.read()
s = raw.replace("\r\n", "\n")

NEW_BLOCK = r"""var yyPoolRows = [], yyPoolQuery = "", yyPoolFilter = "all", yyPoolTime = "all", yyPoolRole = "all", yyPoolPlayer = "all", yyPoolPage = 1, yyPoolPageSize = 20, yyPoolReadEpoch = 0, yyPoolSelectedID = "", yyPoolTokenShown = "", yyPoolTokenPlain = "";
// 托管池缓存只存静态资料（昵称 / 服务器 / UID 依据 / 绑定 / 头像依据），键 yy_pool_cache_v1。
// 进页面先画缓存，再后台拉一次比对；内容没变就不重绘 —— 不做 10 秒级轮询，手动刷新保留。
var yyPoolCacheKey = "yy_pool_cache_v1", yyPoolCacheStamp = 0, yyPoolSeen = {};
function yyPoolBoundPlayer(account) {
  const lease = account && (account.lease || account.pool_binding);
  return lease && lease.phase === "bound" ? String(lease.player || "") : "";
}
function yyPoolProfileOf(account) {
  if (!account) return null;
  return yyAllAccountRows.find((row) => row.id === account.id) || null;
}
function yyPoolIdentityKey(account) {
  if (!account) return "bot";
  if (account.id === "main") return "main";
  const profile = yyPoolProfileOf(account);
  return profile && profile.profile_role === "owner" ? "owner" : "bot";
}
function yyPoolUID(account) {
  const profile = yyPoolProfileOf(account);
  if (profile && profile.game_id) return String(profile.game_id);
  return yyAccountUID(yyLinkedCredential(account, yyCenterCloudRows)) || yyAccountUID(account) || "";
}
// 最近上线：优先用本机实际查询到的在线时间，其次托管状态变更时间。
// 两者都没有就返回 0：这种账号只出现在「全部」里，不会被算进「最近 7 天」。
function yyPoolLastSeen(account) {
  if (!account) return 0;
  const seen = Number(yyPoolSeen[account.id]) || 0;
  const lease = account.lease || account.pool_binding;
  const leaseAt = lease && Number(lease.updated_at) || 0;
  const created = account.created_at ? Date.parse(String(account.created_at)) : 0;
  return Math.max(seen, leaseAt, isNaN(created) ? 0 : created);
}
function yyPoolWhenText(stamp) {
  if (!stamp) return "";
  const diff = Date.now() - stamp;
  if (diff < 6e4) return "刚刚";
  if (diff < 36e5) return Math.floor(diff / 6e4) + " 分钟前";
  if (diff < 864e5) return Math.floor(diff / 36e5) + " 小时前";
  if (diff < 2592e6) return Math.floor(diff / 864e5) + " 天前";
  return yyAcctDateText(stamp);
}
function yyPoolSearchText(account) {
  const profile = yyPoolProfileOf(account);
  return [account.name, account.id, account.player_name, account.uuid, account.server, yyPoolUID(account), profile && profile.name, yyPoolBoundPlayer(account)].filter(Boolean).join(" ").toLowerCase();
}
function yyPoolCacheRow(row) {
  const lease = row.lease || row.pool_binding || null;
  return {
    id: String(row.id || ""),
    name: row.name || "",
    owner_id: row.owner_id || "",
    server: row.server || "",
    created_at: row.created_at || "",
    enabled: !!row.enabled,
    keep_alive: !!row.keep_alive,
    has_token: !!row.has_token,
    token_masked: row.token_masked || "",
    player_name: row.player_name || "",
    uuid: row.uuid || "",
    lease: lease ? { phase: lease.phase || "", player: lease.player || "", server: lease.server || "", updated_at: Number(lease.updated_at) || 0 } : null
  };
}
function yyPoolSignature(rows) {
  return JSON.stringify((rows || []).map(yyPoolCacheRow));
}
function yyPoolCacheRead() {
  try {
    if (typeof localStorage === "undefined" || !localStorage) return null;
    const raw = localStorage.getItem(yyPoolCacheKey);
    if (!raw) return null;
    const data = JSON.parse(raw);
    if (!data || data.v !== 1 || !Array.isArray(data.rows)) return null;
    const rows = data.rows.filter((row) => row && row.id);
    if (!rows.length) return null;
    yyPoolSeen = data.seen && typeof data.seen === "object" ? data.seen : {};
    yyPoolCacheStamp = Number(data.at) || 0;
    return rows;
  } catch (error) {
    return null;
  }
}
function yyPoolCacheWrite(rows) {
  try {
    if (typeof localStorage === "undefined" || !localStorage) return;
    const keep = {};
    for (const row of rows || []) keep[row.id] = 1;
    for (const id of Object.keys(yyPoolSeen)) if (!keep[id]) delete yyPoolSeen[id];
    yyPoolCacheStamp = Date.now();
    localStorage.setItem(yyPoolCacheKey, JSON.stringify({ v: 1, at: yyPoolCacheStamp, rows: (rows || []).map(yyPoolCacheRow), seen: yyPoolSeen }));
  } catch (error) {
    // 缓存写不进去不影响使用，下次进页面重新拉一次。
  }
}
function yyPoolMarkSeen(id) {
  if (!id) return;
  yyPoolSeen[String(id)] = Date.now();
  if (yyPoolRows.length) yyPoolCacheWrite(yyPoolRows);
}
function yyPoolChip(facet, value, label) {
  return '<button type="button" class="yy-pool-chip" data-facet="' + yyEscape(facet) + '" data-value="' + yyEscape(value) + '" data-on="0" aria-pressed="false" onclick="yyPoolFilterSet(' + yyEscape(JSON.stringify(facet)) + "," + yyEscape(JSON.stringify(value)) + ')">' + yyEscape(label) + "</button>";
}
function yyPoolFacetHTML(facet, label, options) {
  return '<div class="yy-pool-facet"><span class="yy-pool-facet-label">' + label + '</span><span class="yy-pool-chips">' + options.map(function(option) {
    return yyPoolChip(facet, option[0], option[1]);
  }).join("") + "</span></div>";
}
function yyPoolPanelHTML() {
  return '<section class="card yy-admin-list-panel yy-pool-panel">' +
    '<div class="yy-pool-searchbar"><label class="yy-search"><input id="yy-pool-search" type="search" aria-label="搜索托管机器人" placeholder="搜索昵称、UID、备注或玩家" oninput="yyPoolQuery=this.value;yyPoolPage=1;yyDrawPoolRows()"></label><button class="btn-s" type="button" onclick="yyPoolRefresh(true)">刷新</button></div>' +
    yyPoolFacetHTML("time", "最近上线", [["all", "全部"], ["1", "24 小时"], ["7", "7 天"], ["30", "30 天"]]) +
    yyPoolFacetHTML("state", "状态", [["all", "全部"], ["enabled", "已启用"], ["disabled", "未启用"], ["bound", "已绑定"], ["idle", "空闲"], ["credential_missing", "缺令牌"]]) +
    yyPoolFacetHTML("role", "身份", [["all", "全部"], ["main", "主账号"], ["owner", "主控"], ["bot", "机器人"]]) +
    '<div class="yy-pool-facet"><span class="yy-pool-facet-label">玩家</span><span class="yy-pool-chips" id="yy-pool-players"></span></div>' +
    '<p id="yy-pool-feedback" class="yy-hint" role="status">正在读取托管池…</p>' +
    '<div id="yy-pool-list" class="yy-compact-list"></div>' +
    '<div id="yy-pool-pages" class="yy-pagination"></div>' +
    "</section>";
}
async function yyPoolPanel() {
  if (!yyAccountAdmin()) {
    T("机器人托管池由管理员管理；自己的账号请在游戏账号列表操作", "e");
    return;
  }
  delete subPageCache["机器人托管池"];
  openSubPage("机器人托管池", yyIntro("机器人托管池", "统一查看多台机器人，按时间、身份和玩家筛选。") + yyPoolPanelHTML());
  const search = document.getElementById("yy-pool-search");
  if (search) search.value = yyPoolQuery;
  if (!yyPoolRows.length) {
    const cached = yyPoolCacheRead();
    if (cached) yyPoolRows = cached;
  }
  yyDrawPoolRows(yyPoolRows.length ? "已显示本机缓存，正在核对…" : "");
  await yyPoolRefresh();
}
function yyPoolFilteredRows(rows, query, state, time, role, player) {
  const q = String(query || "").trim().toLowerCase();
  const timeKey = time || "all", roleKey = role || "all", playerKey = player === void 0 || player === null ? "all" : player;
  const now = Date.now();
  return (rows || []).filter(function(account) {
    if (!account) return false;
    const bound = yyPoolBoundPlayer(account);
    if (state === "enabled" && !account.enabled) return false;
    if (state === "disabled" && account.enabled) return false;
    if (state === "bound" && !bound) return false;
    if (state === "idle" && bound) return false;
    if (state === "credential_missing" && account.has_token) return false;
    if (timeKey !== "all") {
      const stamp = yyPoolLastSeen(account), days = Number(timeKey);
      if (!stamp || !(now - stamp <= days * 864e5)) return false;
    }
    if (roleKey !== "all" && yyPoolIdentityKey(account) !== roleKey) return false;
    if (playerKey !== "all") {
      if (playerKey === "none") {
        if (bound) return false;
      } else if (bound !== playerKey) return false;
    }
    if (q && yyPoolSearchText(account).indexOf(q) < 0) return false;
    return true;
  });
}
function yyPoolSummaryText(total, matched, pages) {
  let text = "共 " + total + " 个托管账号 · 匹配 " + matched + " 个";
  if (pages > 1) text += " · 第 " + yyPoolPage + " / " + pages + " 页";
  if (yyPoolCacheStamp) text += " · 同步 " + new Date(yyPoolCacheStamp).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
  return text;
}
async function yyPoolRefresh(force) {
  if (!yyAccountAdmin()) return;
  // 刷新列表时丢掉已读出的明文令牌，回到默认脱敏状态。
  yyPoolTokenShown = "";
  yyPoolTokenPlain = "";
  const epoch = ++yyPoolReadEpoch, scope = yyReadScope(), host = document.getElementById("yy-pool-list"), out = document.getElementById("yy-pool-feedback");
  if (!host) return;
  if (!yyPoolRows.length) {
    const cached = yyPoolCacheRead();
    if (cached) {
      yyPoolRows = cached;
      yyDrawPoolRows("已显示本机缓存，正在核对…");
    }
  }
  const before = yyPoolSignature(yyPoolRows);
  if (out && !yyPoolRows.length) out.textContent = "正在读取托管池…";
  try {
    const result = await yyPoolCall("list", [{ all: true }]);
    if (epoch !== yyPoolReadEpoch || scope !== yyReadScope() || document.getElementById("yy-pool-list") !== host) return;
    const next = (result.accounts || []).map(yyPoolCacheRow), changed = !!force || yyPoolSignature(next) !== before;
    yyPoolRows = next;
    yyPoolCacheWrite(next);
    if (changed) {
      yyDrawPoolRows();
      return;
    }
    const matched = yyPoolFilteredRows(yyPoolRows, yyPoolQuery, yyPoolFilter, yyPoolTime, yyPoolRole, yyPoolPlayer).length;
    if (out) out.textContent = yyPoolSummaryText(yyPoolRows.length, matched, Math.max(1, Math.ceil(matched / yyPoolPageSize)));
  } catch (error) {
    if (document.getElementById("yy-pool-list") !== host) return;
    if (out) out.textContent = yyPoolRows.length ? "刷新失败，仍显示上次结果：" + (error.message || "") : (error.message || "托管池读取失败");
  }
}
function yyDrawPoolRows(statusText) {
  const host = document.getElementById("yy-pool-list");
  if (!host) return;
  const rows = yyPoolFilteredRows(yyPoolRows, yyPoolQuery, yyPoolFilter, yyPoolTime, yyPoolRole, yyPoolPlayer);
  const pages = Math.max(1, Math.ceil(rows.length / yyPoolPageSize));
  yyPoolPage = Math.min(pages, Math.max(1, yyPoolPage));
  const start = (yyPoolPage - 1) * yyPoolPageSize, shown = rows.slice(start, start + yyPoolPageSize);
  if (typeof yySkinWarm === "function") yySkinWarm(shown);
  if (typeof yyAcctWarmFigures === "function") yyAcctWarmFigures(shown);
  yyPoolRenderPlayers();
  host.innerHTML = shown.map(yyPoolRowHTML).join("") || '<div class="yy-empty">没有符合条件的托管账号</div>';
  yyPoolRenderPages(rows.length, pages);
  yyPoolSyncChips();
  const out = document.getElementById("yy-pool-feedback");
  if (out) out.textContent = statusText || yyPoolSummaryText(yyPoolRows.length, rows.length, pages);
}
function yyPoolRenderPlayers() {
  const host = document.getElementById("yy-pool-players");
  if (!host) return;
  const names = [], seen = {};
  for (const account of yyPoolRows) {
    const player = yyPoolBoundPlayer(account);
    if (!player || seen[player]) continue;
    seen[player] = 1;
    names.push(player);
  }
  names.sort(function(a, b) {
    return a.localeCompare(b, "zh-CN");
  });
  if (yyPoolPlayer !== "all" && yyPoolPlayer !== "none" && names.indexOf(yyPoolPlayer) < 0) yyPoolPlayer = "all";
  host.innerHTML = [yyPoolChip("player", "all", "全部"), yyPoolChip("player", "none", "未绑定")].concat(names.map(function(name) {
    return yyPoolChip("player", name, name);
  })).join("");
}
function yyPoolRenderPages(matched, pages) {
  const host = document.getElementById("yy-pool-pages");
  if (!host) return;
  if (!matched) {
    host.innerHTML = "";
    return;
  }
  host.innerHTML = '<button class="btn-s"' + (yyPoolPage <= 1 ? " disabled" : "") + ' onclick="yyPoolGoPage(-1)">上一页</button><span>第 ' + yyPoolPage + " / " + pages + ' 页</span><button class="btn-s"' + (yyPoolPage >= pages ? " disabled" : "") + ' onclick="yyPoolGoPage(1)">下一页</button><span class="yy-pool-page-size"><span>每页</span>' + ["20", "50"].map(function(size) {
    return yyPoolChip("size", size, size);
  }).join("") + "</span>";
}
function yyPoolSyncChips() {
  const chips = document.querySelectorAll ? document.querySelectorAll(".yy-pool-chip[data-facet]") : [];
  const current = { time: yyPoolTime, state: yyPoolFilter, role: yyPoolRole, player: yyPoolPlayer, size: String(yyPoolPageSize) };
  for (const chip of chips) {
    const facet = chip.getAttribute("data-facet"), value = chip.getAttribute("data-value");
    if (!Object.prototype.hasOwnProperty.call(current, facet)) continue;
    const on = current[facet] === value;
    chip.setAttribute("data-on", on ? "1" : "0");
    chip.setAttribute("aria-pressed", on ? "true" : "false");
  }
}
function yyPoolFilterSet(facet, value) {
  if (facet === "size") {
    yyPoolPageSizeSet(value);
    return;
  }
  if (facet === "time") yyPoolTime = value;
  else if (facet === "state") yyPoolFilter = value;
  else if (facet === "role") yyPoolRole = value;
  else if (facet === "player") yyPoolPlayer = value;
  else return;
  yyPoolPage = 1;
  yyPoolSyncChips();
  yyDrawPoolRows();
}
function yyPoolPageSizeSet(size) {
  yyPoolPageSize = Number(size) === 50 ? 50 : 20;
  yyPoolPage = 1;
  yyDrawPoolRows();
}
function yyPoolGoPage(delta) {
  yyPoolPage = Math.max(1, yyPoolPage + delta);
  yyDrawPoolRows();
}
function yyPoolRefreshIcon() {
  return '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M13.2 8a5.2 5.2 0 1 1-1.7-3.9" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/><path d="M13.4 2.2v3.2H10.2" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>';
}
function yyPoolEyeIcon() {
  return '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M1.6 8S4.1 3.9 8 3.9 14.4 8 14.4 8 11.9 12.1 8 12.1 1.6 8 1.6 8Z" fill="none" stroke="currentColor" stroke-width="1.3"/><circle cx="8" cy="8" r="1.8" fill="currentColor"/></svg>';
}
function yyPoolRowHTML(account) {
  const profile = yyPoolProfileOf(account), uid = yyPoolUID(account), bound = yyPoolBoundPlayer(account);
  // 在线点只认本机账号列表里的真实连接状态；托管池自己只报启用与否，别把"已启用"画成"在线"。
  const onlineState = profile ? yyAcctOnlineState(profile, true) : account.enabled ? "unknown" : "off";
  const status = account.has_token ? account.enabled ? "已启用" : "未启用" : "缺少令牌";
  const meta = [account.server || "未选服务器", uid ? "UID " + uid : "", bound ? "玩家 " + bound : ""].filter(Boolean).join(" · ");
  const when = yyPoolWhenText(yyPoolLastSeen(account)), idJSON = yyEscape(JSON.stringify(String(account.id)));
  return '<div class="yy-pool-row" data-pool-row="' + yyEscape(String(account.id)) + '"><button type="button" class="yy-pool-hit" onclick="yyPoolDetailByID(' + idJSON + ')" title="查看详情与实时状态">' + yyPoolAvatarHTML(account, profile) + '<span class="yy-compact-identity"><span class="yy-acct-nameline"><i class="yy-acct-dot" data-online="' + onlineState + '" aria-hidden="true"></i><strong>' + yyEscape(account.name || account.id) + "</strong>" + yyAcctRoleChip(profile || account, false) + "</span><small>" + yyEscape(meta) + '</small></span></button><span class="yy-pool-side"><span class="yy-pool-live">' + yyEscape(status) + (when ? "<em>" + yyEscape(when) + "</em>" : "") + '</span><button type="button" class="yy-pool-act" title="刷新状态" aria-label="刷新状态" onclick="yyPoolRowRefresh(' + idJSON + ', this)">' + yyPoolRefreshIcon() + '</button><button type="button" class="yy-pool-act" title="查看详情" aria-label="查看详情" onclick="yyPoolDetailByID(' + idJSON + ')">' + yyPoolEyeIcon() + "</button></span></div>";
}
// 行内「刷新状态」：只查这一台，不轮询；在线时把时间记进缓存，供「最近上线」筛选。
async function yyPoolRowRefresh(id, button) {
  if (!yyAccountAdmin()) {
    T("需要管理员权限", "e");
    return;
  }
  if (button && button.dataset.busy === "1") return;
  const rowEl = button && button.closest ? button.closest(".yy-pool-row") : null, live = rowEl ? rowEl.querySelector(".yy-pool-live") : null;
  if (button) {
    button.dataset.busy = "1";
    button.classList.add("is-busy");
  }
  if (live) live.textContent = "查询中…";
  try {
    const status = await yyPoolCall("status", [id]) || {}, connected = status.connected === true;
    if (connected) yyPoolMarkSeen(id);
    if (rowEl && rowEl.isConnected !== false) {
      const dot = rowEl.querySelector(".yy-acct-dot");
      if (dot) dot.setAttribute("data-online", connected ? "on" : "off");
      const target = rowEl.querySelector(".yy-pool-live");
      if (target) target.innerHTML = yyEscape(yyAccountStatusLabel(status)) + "<em>" + yyEscape((status.server || "未选服务器") + (status.online == null ? "" : " · 在线 " + status.online)) + "</em>";
    }
  } catch (error) {
    const target = rowEl && rowEl.querySelector ? rowEl.querySelector(".yy-pool-live") : null;
    if (target) target.textContent = error.message || "状态查询失败";
    else T(error.message || "状态查询失败", "e");
  } finally {
    if (button) {
      button.dataset.busy = "0";
      button.classList.remove("is-busy");
    }
  }
}
function yyPoolDetailByID(id) {
  const index = yyPoolRows.findIndex((account) => String(account.id) === String(id));
  if (index >= 0) yyPoolDetail(index);
}
// 账号选择器：原生 <select> 在 Android WebView 里会弹系统暗色列表，没有头像也不能搜索。
// 这里复用皮肤库那套抽屉结构（.yy-skin-drawer），选中后写回原 <select> 并派发 change，
// 其余读 .value 的代码不用改。
var yyAcctPickerTargetId = "", yyAcctPickerQuery = "";
function yyAcctPickerAvatar(row) {
  if (!row) return "";
  return yyPoolProfileOf(row) || row.lease || row.pool_binding ? yyPoolAvatarHTML(row, yyPoolProfileOf(row)) : yyAccountAvatarHTML(row);
}
function yyAcctPickerCandidates() {
  const target = document.getElementById(yyAcctPickerTargetId);
  if (!target) return [];
  const allowed = {};
  for (const option of Array.from(target.options || [])) allowed[option.value] = option.textContent || option.value;
  return Object.keys(allowed).map(function(id) {
    const row = yyPoolRows.find((a) => String(a.id) === id) || yyAllAccountRows.find((a) => String(a.id) === id) || yyCenterCloudRows.find((a) => String(a.id) === id);
    const uid = row ? yyPoolUID(row) : "";
    return {
      id,
      name: row && (row.nickname || row.name) || allowed[id] || id,
      meta: [row && row.server, uid ? "UID " + uid : "", row ? yyPoolBoundPlayer(row) : ""].filter(Boolean).join(" · "),
      avatar: yyAcctPickerAvatar(row) || '<span class="yy-compact-avatar yy-avatar-frame yy-avatar-fallback"><span class="yy-avatar-initial" aria-hidden="true">' + yyEscape(String(allowed[id] || id).slice(0, 1)) + "</span></span>"
    };
  });
}
function yyAcctPickerHTML() {
  return '<div class="yy-skin-drawer-mask" onclick="yyAcctPickerClose()"></div><aside class="yy-skin-drawer-panel yy-acct-picker-panel" role="dialog" aria-modal="true" aria-label="选择账号"><header class="yy-skin-drawer-head"><h3>选择账号</h3><button class="btn-s" type="button" onclick="yyAcctPickerClose()">关闭</button></header><label class="yy-search"><input id="yy-acct-picker-search" type="search" aria-label="搜索账号" placeholder="搜索昵称、UID 或服务器" oninput="yyAcctPickerQuery=this.value;yyAcctPickerRender()"></label><div id="yy-acct-picker-list" class="yy-acct-picker-list"></div></aside>';
}
function yyAcctPickerOpen(targetId) {
  const target = document.getElementById(targetId);
  if (!target) return;
  yyAcctPickerTargetId = targetId;
  yyAcctPickerQuery = "";
  let host = document.getElementById("yy-acct-picker");
  if (!host) {
    host = document.createElement("div");
    host.id = "yy-acct-picker";
    host.className = "yy-skin-drawer yy-acct-picker";
    document.body.append(host);
  }
  host.hidden = false;
  host.innerHTML = yyAcctPickerHTML();
  yyAcctPickerRender();
  const search = document.getElementById("yy-acct-picker-search");
  if (search && search.focus) search.focus();
}
function yyAcctPickerRender() {
  const host = document.getElementById("yy-acct-picker-list");
  if (!host) return;
  const q = String(yyAcctPickerQuery || "").trim().toLowerCase();
  const rows = yyAcctPickerCandidates().filter((row) => !q || (row.name + " " + row.meta + " " + row.id).toLowerCase().indexOf(q) >= 0);
  host.innerHTML = rows.map(function(row) {
    return '<button type="button" class="yy-acct-picker-row" onclick="yyAcctPickerChoose(' + yyEscape(JSON.stringify(row.id)) + ')"><span class="yy-acct-picker-avatar">' + row.avatar + '</span><span class="yy-compact-identity"><strong>' + yyEscape(row.name) + "</strong>" + (row.meta ? "<small>" + yyEscape(row.meta) + "</small>" : "") + "</span></button>";
  }).join("") || '<p class="yy-empty">没有匹配的账号</p>';
}
function yyAcctPickerChoose(id) {
  const target = document.getElementById(yyAcctPickerTargetId);
  if (target) {
    target.value = String(id);
    target.dispatchEvent(new Event("change", { bubbles: true }));
    yyAcctPickerSync(yyAcctPickerTargetId);
  }
  yyAcctPickerClose();
}
function yyAcctPickerClose() {
  const host = document.getElementById("yy-acct-picker");
  if (host) {
    host.hidden = true;
    host.innerHTML = "";
  }
}
function yyAcctPickerSync(targetId) {
  const target = document.getElementById(targetId), label = document.querySelector('[data-acct-picker-label="' + targetId + '"]');
  if (!label) return;
  const option = target && target.options ? target.options[target.selectedIndex] : null;
  label.textContent = option ? String(option.textContent || option.value || "") || "选择账号" : "选择账号";
}
"""

start = s.index("var yyPoolRows = [],")
end = s.index("function yyPoolDetail(index) {")
s = s[:start] + NEW_BLOCK + s[end:]

# 头像失败态：托管池专用，失败后只留首字占位 + title
start = s.index("function yyPoolAvatarHTML(account, profile) {")
end = s.index("}\n", start) + 2
NEW_AVATAR = r"""function yyPoolAvatarHTML(account, profile) {
  const url = yyAccountPortrait(profile || account) || yyPoolAvatarURL(account);
  return yyAvatarMarkup(account, url, "托管账号头像", "yyPoolAvatarError(this)");
}
// 头像取不到时只退首字占位，并把原因收进 title，界面上不写解释性长句。
function yyPoolAvatarError(image) {
  yyAvatarFailed(image);
  const frame = image.parentElement;
  if (frame) frame.setAttribute("title", "使用默认头像");
}
"""
s = s[:start] + NEW_AVATAR + s[end:]

s = s.replace(
    "function yyAvatarMarkup(account, url, label) {",
    "function yyAvatarMarkup(account, url, label, errorHandler) {",
    1,
)
s = s.replace(
    'data-fallback="\' + fallback + \'" onload="yyAvatarLoaded(this)" onerror="yyAvatarFailed(this)">\' : "";',
    'data-fallback="\' + fallback + \'" onload="yyAvatarLoaded(this)" onerror="\' + (errorHandler || "yyAvatarFailed(this)") + \'">\' : "";',
    1,
)

# 详情里查询到在线时，也记一次最近上线
s = s.replace(
    "    host.textContent = yyAccountStatusLabel(status)",
    "    if (status.connected === true) yyPoolMarkSeen(id);\n    host.textContent = yyAccountStatusLabel(status)",
    1,
)

# 详情操作改动租约后写回缓存
s = s.replace(
    "      if (action === \"start\") {\n        row.enabled = true;\n        row.server = args[1].server || row.server;\n      }\n    }",
    "      if (action === \"start\") {\n        row.enabled = true;\n        row.server = args[1].server || row.server;\n      }\n      yyPoolCacheWrite(yyPoolRows);\n    }",
    1,
)

# 皮肤库：账号选择改成「搜索 + 头像」的抽屉，原生 select 只留数据
old_select = "'<label>换到哪个官方账号<select id=\"yy-skin-target\">' + (options || '<option value=\"\">还没有官方游戏账号</option>') + \"</select></label>\" +"
new_select = "'<label>换到哪个官方账号<select id=\"yy-skin-target\" hidden>' + (options || '<option value=\"\">还没有官方游戏账号</option>') + '</select><button type=\"button\" class=\"yy-acct-picker-trigger\" onclick=\"yyAcctPickerOpen(\\'yy-skin-target\\')\"><span data-acct-picker-label=\"yy-skin-target\">选择账号</span><i aria-hidden=\"true\">▾</i></button></label>' +"
assert old_select in s, "skin target select not found"
s = s.replace(old_select, new_select, 1)

s = s.replace(
    "  if (row && row.id) {\n    const select = document.getElementById(\"yy-skin-target\");\n    if (select) select.value = row.id;\n  }\n  yySkinLibraryRender();",
    "  if (row && row.id) {\n    const select = document.getElementById(\"yy-skin-target\");\n    if (select) select.value = row.id;\n  }\n  yyAcctPickerSync(\"yy-skin-target\");\n  yySkinLibraryRender();",
    1,
)

with io.open(path, "w", encoding="utf-8", newline="") as fh:
    fh.write(s.replace("\n", "\r\n"))
print("patched", path)
