(function() {
  // 群服互通助手面板。曾经挂在 yyQQPage 包装链上，但 console.js 后加载把
  // yyQQPage 覆盖为 yyQQConsolePage 且从不调用保存的 yyQQConsoleBase，
  // 整条链断掉，面板从未注入；旧锚点 #yy-qq-result 在新版 console 页面
  // 也不存在。现改为与知识库/触发词/服务器信息/危险指令一致的方式：
  // 包装 yyQQRenderAll，往 #yyq-root .yyq-fold-grid 插自己的折叠块。
  // yyQQRenderAll 现在按区块增量写，不再整体重建 #yyq-root，正常刷新不会
  // 拆掉本块；但首屏/切页仍会整壳重建一次。所以面板内容必须是状态的
  // 纯函数（body()），注入时按 id 判重，memory 面板随面板重建而重新挂载。
  var yyA = {
    cfg: {}, models: [], groups: [],
    info: { help: "", members: {}, bindings: {} },
    memberCards: '<p class="yy-hint">尚未收到成员事件。请在群里 @机器人 发送 .身份。</p>',
    // 判重基准记「上次自己写进去的 HTML」，不记 host.innerHTML——成员目录
    // UI（member_ui.js）接管后容器 markup 就变了，跟 innerHTML 比永远不相等，
    // 反而每次全量重写、页面一跳一跳（用户第 19 条）。
    memberCardsShown: null,
    messagesHtml: "", messagesShown: null,
    tools: [], skills: [], toolsLoaded: 0,
    proactive: null, proactiveErr: "",
    open: 0, loaded: 0, loading: 0
  };
  function icon(n) { return typeof yyQQIcon === "function" ? yyQQIcon(n) : ""; }
  function membersText() {
    var info = yyA.info;
    return Object.keys(info.members || {}).map(function (id) {
      var name = info.members[id];
      return name + (info.bindings && info.bindings[id] ? " \u2192 " + info.bindings[id] : " \u00B7 \u672A\u7ED1\u5B9A\u6E38\u620F\u73A9\u5BB6");
    }).join("\n") || "\u5C1A\u672A\u6536\u5230\u6210\u5458\u4E8B\u4EF6";
  }
  // 消息头像：后端 /api/qq/messages 现在返回 avatar——OneBot 的 sender_id 就是
  // QQ 号，q.qlogo.cn 是公网 CDN、免鉴权，<img src> 直接可用（不能走
  // data-skin-media：那条路是带 Authorization 的 fetch，跨域会被 CORS 挡下，
  // 真实头像反而取不到）。官方适配器的 member_openid 拼不出头像，后端留空，
  // 此时退回本地占位：yySkinDefaultHead(首字) 的等距三面头 data URI，仍然走
  // data-skin-media（YYSkinMediaURL 对 data: 直接放行，不发请求）。
  // 加载失败（断网/CDN 抽风）时 onerror 走 yyAvatarFailed，换 data-fallback 占位。
  function yyQqAvatarSrc(m) {
    var avatar = m && typeof m.avatar === "string" ? m.avatar.trim() : "";
    if (/^https?:\/\//.test(avatar)) return avatar;
    if (typeof yySkinDefaultHead !== "function") return "";
    var seed = String((m && (m.name || m.sender_id)) || "?").trim().charAt(0) || "?";
    return yySkinDefaultHead(seed);
  }
  function yyQqMsgAvatar(m) {
    var src = yyQqAvatarSrc(m);
    var real = /^https?:\/\//.test(src);
    var seed = String((m && (m.name || m.sender_id)) || "?").trim().charAt(0) || "?";
    var fb = typeof yySkinDefaultHead === "function" ? yySkinDefaultHead(seed) : "";
    return '<span style="flex:0 0 32px;width:32px;height:32px;border-radius:8px;overflow:hidden;display:block;background:rgba(127,127,127,.18)">' +
      (src ? '<img ' + (real ? 'src="' + yyEscape(src) + '"' : 'data-skin-media="' + yyEscape(src) + '"') +
        ' data-fallback="' + yyEscape(fb) + '" alt="" width="32" height="32" decoding="async"' +
        ' style="width:32px;height:32px;object-fit:cover;display:block"' +
        ' onload="if(window.yyAvatarLoaded)yyAvatarLoaded(this)" onerror="if(window.yyAvatarFailed)yyAvatarFailed(this)">' : "") +
      "</span>";
  }
  function messageSection() {
    var opts = yyA.groups.map(function (g) {
      return '<option value="' + yyEscape(g.id) + '">' + yyEscape(g.name || g.number || "\u5DF2\u8FDE\u63A5\u7FA4") + "</option>";
    }).join("");
    return '<section class="yy-card"><h3>\u6700\u8FD1\u6536\u5230\u7684\u7FA4\u6D88\u606F</h3><select id="yy-qq-message-group">' + opts + '</select><div id="yy-qq-group-messages">' + yyA.messagesHtml + "</div></section>";
  }
  function summaryText() {
    if (yyA.loading) return "\u8BFB\u53D6\u4E2D\u2026";
    return yyA.cfg.enabled === false ? "\u5DF2\u505C\u7528" : "\u5DF2\u542F\u7528";
  }
  // ---------- 提示词工具侧边栏（用户第 16 条） ----------
  // selfcheck 只回工具名数组（tools/social_tools），中文名与一句话说明在前端补。
  var YY_TOOL_META = {
    online_players: ["在线玩家", "查询真实在线玩家（仅游戏ID）"],
    public_scores: ["公开分数", "查询允许公开的计分项与分数"],
    scoreboard_count: ["记分板数量", "只查记分板数量，不暴露私密项"],
    game_skill: ["执行技能", "执行服务器预定义操作（管理员）"],
    game_call: ["游戏API", "调用真实游戏API（管理员）"],
    load_skill: ["读开发说明", "按需读取本地开发说明（管理员）"],
    mcp_list: ["MCP目录", "列出外部MCP工具目录（管理员）"],
    mcp_describe: ["MCP文档", "查看外部MCP工具参数（管理员）"],
    mcp_call: ["MCP调用", "调用外部MCP工具（管理员）"],
    image_generate: ["画图", "生成并发送一张图片"],
    poke: ["拍一拍", "拍拍群里的某个人"],
    list_stickers: ["收藏表情", "列出机器人收藏的表情包"],
    send_sticker: ["发表情", "发一张收藏表情到当前群"],
    group_summary: ["群聊记录", "读取最近群消息的原始记录"],
    view_image: ["看图", "用视觉模型看清一张图片"],
    post_qzone: ["发动态", "以机器人身份发QQ空间动态（管理员）"],
    ban: ["禁言", "禁言群成员，要有理由有分寸（管理员）"],
    kick: ["踢人", "把严重违规成员移出群（管理员）"]
  };
  function toolChip(main, sub, title, off) {
    return '<button type="button" class="yy-a-chip' + (off ? " is-off" : "") + '" data-insert="' + yyEscape(main) + '"' +
      (title ? ' title="' + yyEscape(title) + '"' : "") + '><b>' + yyEscape(main) + "</b><i>" + yyEscape(sub || "") + "</i></button>";
  }
  // 默认折叠成一行，点开才展开（用户：要展开的时候再展开，不能太挤）。
  function toolsBlock() {
    return '<details class="yy-a-tools" id="yy-qq-ai-tools"><summary>＋ 插入工具（<span id="yy-qq-ai-tools-n">' +
      (yyA.tools.length + yyA.skills.length) + '</span>）</summary><div class="yy-a-tools-body">' +
      '<div class="yy-a-chips-lab">AI 工具</div><div class="yy-a-chips" id="yy-qq-ai-tool-chips">' +
      (yyA.toolsLoaded ? "" : '<span class="yy-hint">读取中…</span>') + "</div>" +
      '<div class="yy-a-chips-lab">技能（写进提示词后 AI 可自主使用）</div><div class="yy-a-chips" id="yy-qq-ai-skill-chips"></div>' +
      '<p class="yy-hint">点一下插入到上方提示词的光标处：工具直接写工具名；技能写「技能[功能词]」，AI 输出该格式即自动执行。灰色 = 当前开关或能力闸门关闭。</p>' +
      "</div></details>";
  }
  function yyARenderToolChips() {
    var host = document.getElementById("yy-qq-ai-tool-chips");
    if (host) {
      host.innerHTML = yyA.tools.length ? yyA.tools.map(function (t) {
        var meta = YY_TOOL_META[t] || [t, ""];
        return toolChip(t, meta[0], meta[1]);
      }).join("") : '<span class="yy-hint">暂无可用工具</span>';
    }
    var sk = document.getElementById("yy-qq-ai-skill-chips");
    if (sk) {
      sk.innerHTML = yyA.skills.length ? yyA.skills.map(function (s) {
        var word = "技能[" + (s.word || s.id) + "]";
        return toolChip(word, s.label || s.id, s.desc || "", !s.enabled || s.cap_off);
      }).join("") : '<span class="yy-hint">暂无技能</span>';
    }
    var n = document.getElementById("yy-qq-ai-tools-n");
    if (n) n.textContent = yyA.tools.length + yyA.skills.length;
  }
  window.yyAInsertTool = function (text) {
    var ta = document.getElementById("yy-qq-ai-prompt");
    if (!ta || !text) return;
    var v = ta.value, s = ta.selectionStart || 0, e = ta.selectionEnd || 0;
    var ins = text;
    if (s > 0 && !/\s$/.test(v.slice(0, s))) ins = " " + ins;
    if (e < v.length && !/^\s/.test(v.slice(e))) ins += " ";
    ta.value = v.slice(0, s) + ins + v.slice(e);
    ta.selectionStart = ta.selectionEnd = s + ins.length;
    ta.focus();
    // 插入工具等于改了提示词：走统一入口，另一个输入框跟着同步，
    // 保存时才会真的提交（否则这次插入会被当成「没动过」丢掉）。
    if (typeof yyQQPromptInput === "function") yyQQPromptInput(ta);
  };
  // ---------- 指令格式自定义（用户第 16 条：前缀后缀让玩家自己配） ----------
  // 默认值与 youyuan_qq_commands.go 的 yyQQCommandDefaults 一一对应；
  // 保存走 POST /api/qq/config 的 assistant.command_aliases 深合并，
  // 空串 = 恢复默认（后端 yyQQCommandText 对空白别名回退内置值）。
  var YY_CMD_DEFAULTS = { relay: ".通", identity: ".身份", help: "帮助", status: ".状态", ai: ".AI", actions: ".能力", skills: ".技能", poke: ".拍", sticker: ".发图", faces: ".表情包", emoji: ".表情", like: ".点赞", qzone: ".动态", delete_qzone: ".删动态", avatar: ".改头像", long_nick: ".改签名", nick: ".改昵称", kick: ".踢", summary: ".总结", claim: ".认领", log: ".日志", group_log: ".群日志", query: ".查询", bind: "/绑定", unbind: "/解绑", speak: "/说", authorize: "/授权", revoke: "/撤权", command: "/命令", replay: ".再来一次", bindgame: ".绑定", permission: ".权限", player: ".玩家", select_group: ".Q群选择" };
  var YY_CMD_NAMES = { relay: "互通转发", identity: "身份", help: "帮助", status: "状态", ai: "AI对话", actions: "能力清单", skills: "技能", poke: "拍一拍", sticker: "发图", faces: "表情包", emoji: "贴表情", like: "点赞", qzone: "发动态", delete_qzone: "删动态", avatar: "改头像", long_nick: "改签名", nick: "改昵称", kick: "踢人", summary: "总结", claim: "认领", log: "日志", group_log: "群日志", query: "查询", bind: "绑定", unbind: "解绑", speak: "机器人说话", authorize: "授权", revoke: "撤权", command: "命令菜单", replay: "再来一次", bindgame: "绑定游戏", permission: "权限", player: "玩家", select_group: "Q群选择" };
  var YY_CMD_HOT = ["relay", "avatar", "long_nick", "nick", "like", "poke", "sticker", "faces", "emoji", "kick", "qzone", "identity"];
  var YY_CMD_ORDER = ["relay", "identity", "help", "status", "ai", "actions", "skills", "poke", "sticker", "faces", "emoji", "like", "qzone", "delete_qzone", "avatar", "long_nick", "nick", "kick", "summary", "claim", "log", "group_log", "query", "bind", "unbind", "speak", "authorize", "revoke", "command", "replay", "bindgame", "permission", "player", "select_group"];
  function aliasRow(id) {
    var eff = (yyA.cfg.command_aliases && yyA.cfg.command_aliases[id]) || YY_CMD_DEFAULTS[id] || "";
    return '<div class="yy-a-alias-row"><span class="yy-a-alias-name">' + yyEscape(YY_CMD_NAMES[id] || id) + '</span>' +
      '<input class="yy-a-alias-input" id="yy-a-alias-' + id + '" data-cmd="' + id + '" type="text" maxlength="32" value="' + yyEscape(eff) + '" placeholder="' + yyEscape(YY_CMD_DEFAULTS[id] || "") + '">' +
      '<button type="button" class="btn-s yy-a-alias-reset" data-reset="' + id + '">默认</button></div>';
  }
  function aliasBlock() {
    var hot = YY_CMD_HOT.map(aliasRow).join("");
    var rest = YY_CMD_ORDER.filter(function (id) { return YY_CMD_HOT.indexOf(id) < 0; }).map(aliasRow).join("");
    return '<details class="yy-a-alias" id="yy-qq-cmd-alias"><summary>指令格式自定义</summary><div class="yy-a-alias-body">' +
      '<p class="yy-hint">指令的前缀、后缀、功能词都归你定（例如把 .通 改成 .群服、.改头像 改成 /换头像）。留空 = 恢复默认。</p>' +
      '<div class="yy-a-alias-rows">' + hot + "</div>" +
      '<button type="button" class="btn-s" id="yy-a-alias-more">展开全部（' + (YY_CMD_ORDER.length - YY_CMD_HOT.length) + "）</button>" +
      '<div class="yy-a-alias-rows" id="yy-a-alias-rest" hidden>' + rest + "</div>" +
      '<div class="yy-a-alias-actions"><button type="button" class="btn-s" onclick="yyQQSaveCommandAliases()">保存指令格式</button>' +
      '<button type="button" class="btn-s yy-a-alias-resetall" onclick="yyQQResetCommandAliases()">全部恢复默认</button>' +
      '<span class="yy-a-alias-status" id="yy-a-alias-status" role="status"></span></div>' +
      '<p class="yy-hint">保存后下方「指令帮助」会立即换成新指令；游戏侧的 .通 / .Q群选择 / .绑定 门禁同步生效。</p></div></details>';
  }
  function wireAliasEditor() {
    var box = document.getElementById("yy-qq-cmd-alias");
    if (!box) return;
    var more = document.getElementById("yy-a-alias-more");
    if (more) more.onclick = function () {
      var rest = document.getElementById("yy-a-alias-rest");
      if (!rest) return;
      rest.hidden = !rest.hidden;
      more.textContent = rest.hidden ? "展开全部（" + (YY_CMD_ORDER.length - YY_CMD_HOT.length) + "）" : "收起全部";
    };
    box.querySelectorAll("[data-reset]").forEach(function (b) {
      b.onclick = function () {
        var inp = document.getElementById("yy-a-alias-" + b.getAttribute("data-reset"));
        // 填入默认指令串；保存时发这个值与会发空串等价（后端对空白别名回退默认）。
        if (inp) { inp.value = YY_CMD_DEFAULTS[b.getAttribute("data-reset")] || ""; inp.focus(); }
      };
    });
  }
  window.yyQQSaveCommandAliases = async function () {
    var aliases = {};
    document.querySelectorAll("#yy-qq-cmd-alias input[data-cmd]").forEach(function (inp) {
      aliases[inp.getAttribute("data-cmd")] = inp.value.trim();
    });
    var st = document.getElementById("yy-a-alias-status");
    try {
      var r = await A("POST", "/api/qq/config", { assistant: { command_aliases: aliases } });
      if (!r.ok) throw Error(r.error);
      yyA.cfg.command_aliases = aliases;
      // 重新拉 help：后端按别名生成帮助文本，让新指令立刻显示出来。
      var info = await A("GET", "/api/qq/assistant");
      if (info && info.ok !== false) {
        yyA.info.help = info.help || "";
        var help = document.getElementById("yy-qq-helper-help");
        if (help) help.textContent = yyA.info.help;
      }
      if (st) st.textContent = "已保存，帮助文本已刷新";
      T("指令格式已保存", "s");
    } catch (e) {
      if (st) st.textContent = "保存失败：" + e.message;
      T(e.message, "e");
    }
  };
  // 全部恢复默认：后端 yyMergePatch（youyuan_qq.go:333）对 map 是逐 key 深合并，
  // 提交 {} 不会清掉已有别名，所以按 YY_CMD_ORDER 逐 key 送空串；
  // 空串在 yyQQCommandText 里等于回退内置默认。
  window.yyQQResetCommandAliases = async function () {
    if (!window.confirm("把全部指令恢复成内置默认？保存后立即生效。")) return;
    var st = document.getElementById("yy-a-alias-status");
    var aliases = {};
    YY_CMD_ORDER.forEach(function (id) { aliases[id] = ""; });
    try {
      var r = await A("POST", "/api/qq/config", { assistant: { command_aliases: aliases } });
      if (!r.ok) throw Error(r.error);
      yyA.cfg.command_aliases = aliases;
      document.querySelectorAll("#yy-qq-cmd-alias input[data-cmd]").forEach(function (inp) {
        var id = inp.getAttribute("data-cmd");
        inp.value = YY_CMD_DEFAULTS[id] || "";
      });
      var info = await A("GET", "/api/qq/assistant");
      if (info && info.ok !== false) {
        yyA.info.help = info.help || "";
        var help = document.getElementById("yy-qq-helper-help");
        if (help) help.textContent = yyA.info.help;
      }
      if (st) st.textContent = "已全部恢复默认，帮助文本已刷新";
      T("指令格式已全部恢复默认", "s");
    } catch (e) {
      if (st) st.textContent = "恢复失败：" + e.message;
      T(e.message, "e");
    }
  };
  // ---------- 主动搭话（用户第 16 条：AI 还没办法主动聊天） ----------
  // 后端有真开关：GET/POST /api/qq/proactive（youyuan_qq_proactive.go），
  // 字段 config.enabled / cooldown_sec / daily_cap / min_silence_sec / threshold，
  // 默认 enabled=false、冷却 1800 秒。POST 是整体替换，所以先 GET 再改 enabled 回灌。
  function proactiveBlock() {
    var p = yyA.proactive;
    var on = !!(p && p.enabled);
    return '<details class="yy-a-proactive" id="yy-qq-proactive"><summary>主动搭话 · ' + (on ? "已开启" : "已关闭") + "</summary>" +
      '<div class="yy-a-proactive-body">' + yySwitch("yy-qq-proactive-enabled", "允许 AI 主动在群里搭话", on) + '<label for="yy-qq-proactive-silence">群静默多久后开始打分 · 秒</label><input id="yy-qq-proactive-silence" type="number" min="0" max="86400" step="1" value="' + ((p && p.min_silence_sec != null) ? p.min_silence_sec : "") + '">' + '<label for="yy-qq-proactive-cooldown">两次主动搭话最小间隔 · 秒</label><input id="yy-qq-proactive-cooldown" type="number" min="0" max="86400" step="1" value="' + ((p && p.cooldown_sec != null) ? p.cooldown_sec : "") + '">' + '<label for="yy-qq-proactive-daily">每群每日上限 · 条</label><input id="yy-qq-proactive-daily" type="number" min="0" max="200" step="1" value="' + ((p && p.daily_cap != null) ? p.daily_cap : "") + '">' + '<label for="yy-qq-proactive-threshold">开口阈值</label><input id="yy-qq-proactive-threshold" type="number" min="0" max="100" step="0.1" value="' + ((p && p.threshold != null) ? p.threshold : "") + '">' +
      '<p class="yy-hint" id="yy-qq-proactive-info"></p>' +
      '<p class="yy-hint">群静默一段时间后，AI 按静默时长、热点话题、群活跃度、时段四个维度打分，超过阈值才开口，深夜降权按打分权重 weights.night 生效；具体几点到几点，用下面「AI 接话模式」里的深夜时段。关闭时机器人只被动应答。</p>' +
      "</div></details>";
  }
  function yyAProactiveInfo() {
    var el = document.getElementById("yy-qq-proactive-info");
    if (!el) return;
    var p = yyA.proactive;
    if (!p) {
      el.textContent = yyA.proactiveErr ? "主动搭话状态读取失败：" + yyA.proactiveErr : "读取中…";
      return;
    }
    el.textContent = (p.enabled ? "已开启：" : "已关闭：") + "群静默 " + (p.min_silence_sec || 0) + " 秒后开始打分，开口阈值 " + p.threshold +
      "；两次主动搭话至少间隔 " + (p.cooldown_sec || 0) + " 秒，每群每日最多 " + p.daily_cap + " 条。";
  }
  function wireProactive() {
    var cb = document.getElementById("yy-qq-proactive-enabled");
    if (cb) {
      cb.checked = !!(yyA.proactive && yyA.proactive.enabled);
      cb.onchange = yyQQToggleProactive;
    }
    wireProactiveFields();
    yyAProactiveInfo();
  }

  var YY_PROACTIVE_FIELDS = [
    ["yy-qq-proactive-silence", "min_silence_sec"],
    ["yy-qq-proactive-cooldown", "cooldown_sec"],
    ["yy-qq-proactive-daily", "daily_cap"],
    ["yy-qq-proactive-threshold", "threshold"]
  ];
  function wireProactiveFields() {
    YY_PROACTIVE_FIELDS.forEach(function (p) {
      var el = document.getElementById(p[0]);
      if (!el) return;
      var save = function () { window.yyQQProactiveSaveField(p[1], p[0]); };
      el.onchange = save;
      el.onblur = save;
    });
  }
  // 这个接口是整体替换，必须先 GET 再改字段回灌（和 yyQQToggleProactive 同一套路）。
  window.yyQQProactiveSaveField = async function (key, elId) {
    var el = document.getElementById(elId);
    if (!el) return;
    var value = parseFloat(el.value);
    if (isNaN(value)) return;
    try {
      var cur = await A("GET", "/api/qq/proactive");
      if (!cur.ok) throw Error(cur.error);
      var cfg = Object.assign({}, cur.config || {});
      cfg[key] = value;
      var r = await A("POST", "/api/qq/proactive", { config: cfg });
      if (!r.ok) throw Error(r.error);
      yyA.proactive = r.config || cfg;
      yyAProactiveInfo();
      T("已保存", "s");
    } catch (e) {
      T(e.message, "e");
    }
  };
  window.yyQQToggleProactive = async function () {
    var cb = document.getElementById("yy-qq-proactive-enabled");
    var on = !!(cb && cb.checked);
    try {
      var cur = await A("GET", "/api/qq/proactive");
      if (!cur.ok) throw Error(cur.error);
      var cfg = Object.assign({}, cur.config || {}, { enabled: on });
      var r = await A("POST", "/api/qq/proactive", { config: cfg });
      if (!r.ok) throw Error(r.error);
      yyA.proactive = r.config || cfg;
      yyA.proactiveErr = "";
      var sum = document.querySelector("#yy-qq-proactive summary");
      if (sum) sum.textContent = "主动搭话 · " + (on ? "已开启" : "已关闭");
      yyAProactiveInfo();
      T(on ? "主动搭话已开启" : "主动搭话已关闭", "s");
    } catch (e) {
      if (cb) cb.checked = !on;
      yyAProactiveInfo();
      T(e.message, "e");
    }
  };

  // ---------- AI 接话三档模式（用户要求：程序只给间隔，回不回由 AI 自己决定） ----------
  // mention = 只在被叫时说（.AI / @机器人）；interval = 定时自检；auto = 全自动。
  // 三种并存、随时可换；保存一律走 POST /api/qq/config 的 assistant 深合并，
  // 改完即存，不额外点「保存」按钮（用户原话：点进去改数字，改好了就直接保存）。
  function aimodeBlock(c) {
    var mode = c.ai_mode === "interval" || c.ai_mode === "auto" ? c.ai_mode : "mention";
    var num = function (id, label, value, min, max, step) {
      return '<label for="' + id + '">' + label + '</label><input id="' + id + '" type="number" min="' + min +
        '" max="' + max + '" step="' + (step || 1) + '" value="' + yyEscape(String(value == null ? "" : value)) + '">';
    };
    var title = mode === "interval" ? "定时自检" : mode === "auto" ? "全自动" : "只在被叫时回答";
    return '<details class="yy-a-aimode" id="yy-qq-ai-mode-box"><summary>AI 接话模式 · ' + title + '</summary>' +
      '<label for="yy-qq-ai-mode">接话模式</label><select id="yy-qq-ai-mode">' +
      '<option value="mention"' + (mode === "mention" ? " selected" : "") + '>只在被叫时回答（.AI / @机器人）—— 现状</option>' +
      '<option value="interval"' + (mode === "interval" ? " selected" : "") + '>定时自检（每隔一段时间 AI 自己翻聊天记录挑想回的）</option>' +
      '<option value="auto"' + (mode === "auto" ? " selected" : "") + '>全自动（每条群消息都判断要不要接）</option>' +
      "</select>" +
      '<p class="yy-hint">三种模式并存，随时可换。程序只负责「多久看一次」；回不回、回哪一条、说什么，全由 AI 按提示词自己决定。选「定时自检」或「全自动」时，上面「自然群聊」的会话窗口不生效——同一条消息不会被回两次。</p>' +
      num("yy-qq-ai-interval", "自检间隔 · 分钟（1–1440，默认 10）", c.ai_interval_minutes || 10, 1, 1440) +
      num("yy-qq-ai-night-start", "深夜时段开始 · 点（0–23，默认 23）", c.ai_night_start_hour == null ? 23 : c.ai_night_start_hour, 0, 23) +
      num("yy-qq-ai-night-end", "深夜时段结束 · 点（0–23，默认 7）", c.ai_night_end_hour == null ? 7 : c.ai_night_end_hour, 0, 23) +
      num("yy-qq-ai-night-factor", "深夜降权倍数（1–10，默认 2，越大夜里判断越稀疏）", c.ai_night_factor || 2, 1, 10) +
      '<label for="yy-qq-ai-quiet-start">静默窗口开始 · 留空表示不设</label><input id="yy-qq-ai-quiet-start" type="time" value="' + yyEscape(c.ai_quiet_start || "") + '">' +
      '<label for="yy-qq-ai-quiet-end">静默窗口结束 · 留空表示不设</label><input id="yy-qq-ai-quiet-end" type="time" value="' + yyEscape(c.ai_quiet_end || "") + '">' +
      '<p class="yy-hint">静默窗口内 AI 不主动开口（定时自检与全自动都停），但 @机器人 和 .AI 照常回。深夜时段只影响「定时自检」与「全自动」的判断频率。</p>' +
      "</details>";
  }
  var YY_AI_MODE_FIELDS = [
    ["yy-qq-companion-window", "companion_window", "int"],
    ["yy-qq-ai-mode", "ai_mode", "str"],
    ["yy-qq-ai-interval", "ai_interval_minutes", "int"],
    ["yy-qq-ai-night-start", "ai_night_start_hour", "int"],
    ["yy-qq-ai-night-end", "ai_night_end_hour", "int"],
    ["yy-qq-ai-night-factor", "ai_night_factor", "int"],
    ["yy-qq-ai-quiet-start", "ai_quiet_start", "str"],
    ["yy-qq-ai-quiet-end", "ai_quiet_end", "str"]
  ];
  function wireAIMode() {
    YY_AI_MODE_FIELDS.forEach(function (p) {
      var el = document.getElementById(p[0]);
      if (!el) return;
      var save = function () { window.yyQQAISaveField(p[1], p[0], p[2]); };
      el.onchange = save;
      if (el.type === "number" || el.type === "time") el.onblur = save;
    });
    var mode = document.getElementById("yy-qq-ai-mode"), interval = document.getElementById("yy-qq-ai-interval");
    if (mode && interval) {
      var sync = function () { interval.disabled = mode.value !== "interval"; };
      sync();
      mode.addEventListener("change", sync);
    }
  }
  // 改完即存：不设「保存」按钮，输入框的 onchange/onblur 直接落到群服互通配置里。
  window.yyQQAISaveField = async function (key, elId, cast) {
    var el = document.getElementById(elId);
    if (!el) return;
    var value = cast === "int" ? parseInt(el.value, 10) : String(el.value || "").trim();
    if (cast === "int") {
      if (isNaN(value)) return; // 空着或乱填就不提交，等用户改对，别把默认值写死到他配置里
      var lo = parseInt(el.min, 10), hi = parseInt(el.max, 10);
      if (!isNaN(lo) && value < lo) value = lo;
      if (!isNaN(hi) && value > hi) value = hi;
      el.value = value;
    }
    try {
      var patch = {};
      patch[key] = value;
      var r = await A("POST", "/api/qq/config", { assistant: patch });
      if (!r.ok) throw Error(r.error);
      yyA.cfg = yyA.cfg || {};
      yyA.cfg[key] = value;
      if (key === "ai_mode") {
        var cur = yyA.cfg, m = document.getElementById("yy-qq-ai-interval");
        if (m) m.disabled = value !== "interval";
        var t = document.querySelector("#yy-qq-ai-mode-box summary");
        if (t) t.textContent = "AI 接话模式 · " + (value === "interval" ? "定时自检" : value === "auto" ? "全自动" : "只在被叫时回答");
        cur.ai_mode = value;
      }
      T("已保存", "s");
    } catch (e) {
      T(e.message, "e");
    }
  };
  function body() {
    var _a, _b;
    var c = yyA.cfg, m = yyA.models;
    var field = (id, label, value, type) => '<label for="' + id + '">' + label + '</label><input id="' + id + '" type="' + (type || "text") + '" value="' + yyEscape(value || "") + '">';
    var html = '<section class="yy-card" id="yy-qq-assistant-panel"><h3>\u7FA4\u670D\u4E92\u901A\u52A9\u624B</h3>' + yySwitch("yy-qq-helper-enabled", "\u542F\u7528\u6307\u4EE4\u9762\u677F \xB7 \u53EA\u7528 .\u901A \u8F6C\u53D1\u804A\u5929", c.enabled !== false) + '<p class="yy-hint">\u7FA4\u5185 @\u673A\u5668\u4EBA\u540E\u53D1\u9001 \u5E2E\u52A9\uFF0C\u6240\u6709\u4EBA\u53EF\u67E5\u770B\u6307\u4EE4\uFF1B\u666E\u901A\u6210\u5458\u4E0D\u80FD\u6267\u884C\u547D\u4EE4\u6216\u8BFB\u53D6\u5750\u6807\u3002\u5404\u8D26\u53F7\u7684\u7ED1\u5B9A\u3001\u6743\u9650\u3001AI\u4E0E\u8BB0\u5F55\u76F8\u4E92\u72EC\u7ACB\u3002</p><details><summary>\u9AD8\u7EA7\u6743\u9650\u6807\u8BC6</summary><label for="yy-qq-super">\u8D85\u7EA7\u7BA1\u7406\u5458 \xB7 \u6BCF\u884C\u4E00\u4E2A\u6210\u5458\u6807\u8BC6</label><textarea id="yy-qq-super">' + yyEscape((c.super_admins || []).join("\n")) + '</textarea><label for="yy-qq-admins">\u666E\u901A\u7BA1\u7406\u5458 \xB7 \u6BCF\u884C\u4E00\u4E2A\u6210\u5458\u6807\u8BC6</label><textarea id="yy-qq-admins">' + yyEscape((c.admins || []).join("\n")) + '</textarea><p class="yy-hint">QQ\u5B98\u65B9\u4F7F\u7528\u5185\u90E8 member_openid \u6388\u6743\uFF0C\u4E0D\u80FD\u76F4\u63A5\u586BQQ\u53F7\u3002\u8BF7\u4ECE\u4E0B\u65B9\u771F\u5B9E\u6210\u5458\u5217\u8868\u6388\u6743\uFF1B\u5185\u90E8\u6807\u8BC6\u4EC5\u5728\u8BE6\u60C5\u4E2D\u67E5\u770B\u3002\u4E5F\u53EF\u751F\u6210\u8BA4\u9886\u7801\u6307\u5B9A\u8D85\u7EA7\u7BA1\u7406\u5458\u3002</p></details><button class="btn-s" onclick="yyQQCreateClaim()">\u751F\u6210\u8D85\u7EA7\u7BA1\u7406\u5458\u8BA4\u9886\u7801</button><pre id="yy-qq-claim" style="white-space:pre-wrap;word-break:break-all"></pre>' + yySwitch("yy-qq-death", "\u540C\u6B65\u6B7B\u4EA1\u6D88\u606F", !!c.death) + yySwitch("yy-qq-join", "\u540C\u6B65\u8FDB\u5165\u670D\u52A1\u5668", !!c.join) + yySwitch("yy-qq-leave", "\u540C\u6B65\u9000\u51FA\u670D\u52A1\u5668", !!c.leave) + '<p class="yy-hint">\u5B98\u65B9QQ\u65E0\u4E3B\u52A8\u63A8\u9001\u989D\u5EA6\uFF1B\u901A\u77E5\u9700\u8981\u6709\u6548\u7684\u7FA4\u56DE\u590D\u7A97\u53E3\uFF0C\u5426\u5219\u8BB0\u5F55\u5931\u8D25\u539F\u56E0\u3002OneBot\u4F7F\u7528\u5176\u81EA\u8EAB\u80FD\u529B\u3002</p><label for="yy-qq-public-scores">\u516C\u5F00\u8BA1\u5206\u9879 \xB7 \u6BCF\u884C\u4E00\u4E2A\uFF0C\u7559\u7A7A\u4E0D\u516C\u5F00\u4EFB\u4F55\u5206\u6570</label><textarea id="yy-qq-public-scores">' + yyEscape((c.public_objectives || []).join("\n")) + "</textarea>" + yySwitch("yy-qq-companion", "自然群聊 · 连续无新消息后结束会话", !!c.companion) + '<label for="yy-qq-companion-window">自然群聊结束窗口 · 秒（5–600，默认 30）</label><input id="yy-qq-companion-window" type="number" min="5" max="600" step="1" value="' + (c.companion_window || 30) + '">' + yySwitch("yy-qq-companion-all", "\u6536\u5230\u666E\u901A\u7FA4\u6D88\u606F\u65F6\u4E5F\u53EF\u81EA\u7136\u56DE\u590D", !!c.companion_all_messages) + field("yy-qq-companion-owner", "\u670D\u4E3B\u6635\u79F0 \xB7 \u4ECE\u4E0B\u65B9\u6210\u5458\u5217\u8868\u9009\u62E9", ((_b = c.nicknames) == null ? void 0 : _b[c.companion_owner]) || "\u672A\u9009\u62E9") + '<label for="yy-qq-companion-words">\u89E6\u53D1\u5173\u952E\u8BCD \xB7 \u6BCF\u884C\u4E00\u4E2A</label><textarea id="yy-qq-companion-words">' + yyEscape((c.companion_keywords || ["\u75A3", "\u8150\u7AF9", "\u623F\u4E3B", "\u670D\u4E3B"]).join("\n")) + '</textarea><p class="yy-hint">首次触发先等一小段（接话窗口的40%，最多10秒）\uFF0C\u6536\u5230\u670D\u4E3B\u56DE\u590D\u5373\u505C\u6B62\uFF1B\u5BF9\u8BDD\u53EA\u968F\u6536\u5230\u7684\u6D88\u606F\u7EE7\u7EED\uFF0C超过上面填的窗口秒数没有新消息就结束\u3002\u5B98\u65B9QQ\u901A\u5E38\u53EA\u4E0B\u53D1 @\u673A\u5668\u4EBA \u6D88\u606F\uFF1A\u53EA\u6709 @\u75A3\u3001\u666E\u901A\u804A\u5929\u6216\u670D\u4E3B\u56DE\u590D\u672A\u4E0B\u53D1\u65F6\u65E0\u6CD5\u68C0\u6D4B\u3002OneBot\u53EF\u4F7F\u7528\u5B83\u5B9E\u9645\u6536\u5230\u7684\u666E\u901A\u7FA4\u6D88\u606F\u3002\u7FA4\u804A\u4EC5\u6709\u6210\u5458\u67E5\u8BE2\u6743\u9650\u3002\u6700\u8FD1\u7FA4\u6D88\u606F\u53EF\u4EE5\u5728\u4E0B\u65B9\u67E5\u770B\uFF0C\u6A21\u578B\u53EA\u83B7\u5F97\u672C\u7FA4\u8FD1\u671F12\u6761\u5B9E\u9645\u6536\u5230\u7684\u6D88\u606F\u3002</p>' + aimodeBlock(c) + proactiveBlock() + '<h3>\u7FA4\u804A AI \u4E0E\u5916\u90E8 MCP</h3><label for="yy-qq-ai-model">\u5DF2\u914D\u7F6E\u7684\u6A21\u578B</label><select id="yy-qq-ai-model"><option value="">\u672A\u542F\u7528</option>' + (m || []).map((x) => '<option value="' + yyEscape(x.name) + '">' + yyEscape(x.name) + " \xB7 " + yyEscape(x.model_id) + "</option>").join("") + '</select><label for="yy-qq-image-model">\u7ED8\u56FE\u6A21\u578B \xB7 \u652F\u6301 images/generations</label><select id="yy-qq-image-model"><option value="">\u672A\u914D\u7F6E\u7ED8\u56FE</option>' + (m || []).map((x) => '<option value="' + yyEscape(x.name) + '">' + yyEscape(x.name) + " \xB7 " + yyEscape(x.model_id) + "</option>").join("") + '</select><p class="yy-hint">\u7ED8\u56FE\u4F7F\u7528\u5355\u72EC\u7684\u56FE\u50CF\u63A5\u53E3\uFF0C\u9700\u8FD4\u56DE b64_json\u3002\u6587\u5B57\u804A\u5929\u6A21\u578B\u4E0D\u4F1A\u81EA\u52A8\u83B7\u5F97\u7ED8\u56FE\u80FD\u529B\uFF1B\u6210\u529F\u4E0A\u4F20QQ\u540E\u624D\u62A5\u544A\u5DF2\u53D1\u9001\u3002</p><details><summary>\u6DFB\u52A0\u6216\u66F4\u65B0\u7ED8\u56FE\u63A5\u53E3</summary>' + field("yy-qq-image-name", "\u7ED8\u56FE\u914D\u7F6E\u540D\u79F0", "\u7FA4\u804A\u7ED8\u56FE") + field("yy-qq-image-base", "\u7ED8\u56FEAPI\u5730\u5740\uFF08\u4F8B\u5982 https://\u4F60\u7684\u670D\u52A1/v1\uFF09", "") + '<label for="yy-qq-image-id">\u56FE\u50CF\u751F\u6210\u6A21\u578BID</label><div class="yyb-fetch-row"><select id="yy-qq-image-id"><option value="">\u5148\u586B\u5199 API \u5730\u5740\u548C\u5BC6\u94A5\uFF0C\u70B9\u300C\u83B7\u53D6\u6A21\u578B\u5217\u8868\u300D</option></select><button type="button" class="btn-s" id="yy-qq-image-fetch-btn" onclick="yyQQFetchImageModels()">\u83B7\u53D6\u6A21\u578B\u5217\u8868</button></div><p class="yy-hint" id="yy-qq-image-fetch-status">\u6A21\u578BID \u7531\u670D\u52A1\u7AEF\u8FD4\u56DE\u540E\u4E0B\u62C9\u9009\u62E9\uFF0C\u4E0D\u7528\u624B\u6253\u3002</p><details class="yyb-manual"><summary>\u670D\u52A1\u7AEF\u4E0D\u8FD4\u56DE\u5217\u8868\uFF1F\u624B\u52A8\u8F93\u5165\u6A21\u578BID</summary><div class="yyb-fetch-row"><input id="yy-qq-image-id-manual" type="text" placeholder="\u4F8B\u5982 doubao-seedream-4-0"><button type="button" class="btn-s" onclick="yyQQManualModel(\'image\')">\u4F7F\u7528\u8BE5ID</button></div></details>' + field("yy-qq-image-key", "\u7ED8\u56FEAPI\u5BC6\u94A5 \xB7 \u7559\u7A7A\u4FDD\u7559\u5DF2\u6709\u5BC6\u94A5", "", "password") + '<button class="btn-s" onclick="yyQQSaveImageModel()">\u4FDD\u5B58\u5E76\u9009\u7528\u7ED8\u56FE\u63A5\u53E3</button><p id="yy-qq-image-result" role="status"></p></details><label for="yy-qq-ai-prompt">\u81EA\u5B9A\u4E49\u8EAB\u4EFD\u4E0E\u4EFB\u52A1</label><textarea id="yy-qq-ai-prompt" class="yy-a-prompt" rows="6" oninput="yyQQPromptInput(this)" placeholder="\u4F8B\u5982\uFF1A\u4F60\u662F\u672C\u670D\u52A1\u5668\u7684\u52A9\u624B\uFF0C\u8D1F\u8D23\u89E3\u7B54\u73A9\u6CD5\u4E0E\u7BA1\u7406\u95EE\u9898\u3002">' + yyEscape(typeof yyQQPromptValue === "function" ? yyQQPromptValue() : (c.prompt || "")) + "</textarea>" + toolsBlock() + field("yy-qq-mcp-url", "\u7BA1\u7406\u5458\u4F7F\u7528\u7684\u5916\u90E8 MCP \xB7 Streamable HTTP \u5730\u5740", c.mcp_url) + field("yy-qq-mcp-token", "MCP\u8FDE\u63A5\u5BC6\u94A5 \xB7 \u7559\u7A7A\u4FDD\u7559", "", "password") + '<p class="yy-hint">' + (c.has_mcp_token ? "MCP\u5BC6\u94A5\u5DF2\u4FDD\u5B58\u3002" : "\u5C1A\u672A\u4FDD\u5B58MCP\u5BC6\u94A5\u3002") + "\u6210\u5458AI\u53EA\u83B7\u5F97\u5728\u7EBF\u540D\u5355\u3001\u516C\u5F00\u5206\u6570\u548C\u8BB0\u5206\u677F\u6570\u91CF\u5DE5\u5177\uFF1B\u7BA1\u7406\u5458\u6309\u9700\u8BFB\u53D6Skill\uFF0C\u5E76\u8C03\u7528\u6E38\u620F\u53CA\u8FD9\u4E2AMCP\u3002</p><details><summary>\u6DFB\u52A0\u6216\u66F4\u65B0\u7FA4\u804A\u6A21\u578B\u63A5\u53E3</summary>" + field("yy-qq-model-name", "\u914D\u7F6E\u540D\u79F0", "\u7FA4\u670DAI") + field("yy-qq-model-base", "API\u5730\u5740", "https://api.deepseek.com/v1") + '<label for="yy-qq-model-id">\u6A21\u578BID</label><div class="yyb-fetch-row"><select id="yy-qq-model-id"><option value="">\u5148\u586B\u5199 API \u5730\u5740\u548C\u5BC6\u94A5\uFF0C\u70B9\u300C\u83B7\u53D6\u6A21\u578B\u5217\u8868\u300D</option></select><button type="button" class="btn-s" id="yy-qq-model-fetch-btn" onclick="yyQQFetchChatModels()">\u83B7\u53D6\u6A21\u578B\u5217\u8868</button></div><p class="yy-hint" id="yy-qq-model-fetch-status">\u6A21\u578BID \u7531\u670D\u52A1\u7AEF\u8FD4\u56DE\u540E\u4E0B\u62C9\u9009\u62E9\uFF0C\u4E0D\u7528\u624B\u6253\u3002</p><details class="yyb-manual"><summary>\u670D\u52A1\u7AEF\u4E0D\u8FD4\u56DE\u5217\u8868\uFF1F\u624B\u52A8\u8F93\u5165\u6A21\u578BID</summary><div class="yyb-fetch-row"><input id="yy-qq-model-id-manual" type="text" placeholder="\u4F8B\u5982 deepseek-chat"><button type="button" class="btn-s" onclick="yyQQManualModel(\'chat\')">\u4F7F\u7528\u8BE5ID</button></div></details>' + field("yy-qq-model-key", "API\u5BC6\u94A5 \xB7 \u7559\u7A7A\u4FDD\u7559\u5DF2\u6709\u5BC6\u94A5", "", "password") + '<button class="btn-s" onclick="yyQQSaveModel()">\u4FDD\u5B58\u6A21\u578B\u63A5\u53E3</button><pre id="yy-qq-model-result"></pre></details><button class="btn-s" onclick="yyQQAssistantSave()">\u4FDD\u5B58\u52A9\u624B\u8BBE\u7F6E</button><details><summary>\u6307\u4EE4\u5E2E\u52A9</summary><pre id="yy-qq-helper-help" style="white-space:pre-wrap">' + yyEscape(yyA.info.help || "") + '</pre></details>' + aliasBlock() + '<section><h3>\u4ECE\u771F\u5B9E\u7FA4\u6210\u5458\u6388\u6743</h3><p class="yy-hint">\u5148\u8BA9\u76EE\u6807\u7528\u6237\u5728\u7FA4\u91CC @\u673A\u5668\u4EBA\u53D1\u9001 .\u8EAB\u4EFD\uFF0C\u518D\u70B9\u51FB\u5237\u65B0\u3002\u9009\u62E9\u5BF9\u5E94\u7684\u6210\u5458\u6807\u8BC6\u6388\u6743\uFF0CQQ\u53F7\u4E0D\u80FD\u8F6C\u6362\u4E3A\u5B98\u65B9\u6807\u8BC6\u3002</p><button class="btn-s" onclick="yyQQRefreshAll()">\u5237\u65B0\u7FA4\u6210\u5458\u4E0E\u6D88\u606F</button><div id="yy-qq-member-cards">' + yyA.memberCards + '</div></section><details><summary>\u7ED1\u5B9A\u7528\u6237\u4E0E\u6700\u8FD1\u6210\u5458\uFF08\u6635\u79F0\uFF09</summary><pre id="yy-qq-helper-members" style="white-space:pre-wrap;word-break:break-all">' + membersText() + '</pre></details>' + messageSection() + '</section>';
    return html;
  }

  function block() {
    return '<details class="yyq-fold" id="yyq-assistant"' + (yyA.open ? " open" : "") + ' ontoggle="yyQQAFold(this)">' +
      '<summary><span class="yyq-fold-ico">' + icon("face") + "</span>" +
      '<span class="yyq-fold-t">群服互通助手</span>' +
      '<span class="yyq-fold-sum">' + yyEscape(summaryText()) + "</span>" +
      '<span class="yyq-fold-caret">' + icon("arrow") + "</span></summary>" +
      '<div class="yyq-fold-body" id="yyq-a-body">' + body() + "</div></details>";
  }

  function wireMemberButtons() {
    var host = document.getElementById("yy-qq-member-cards");
    if (!host) return;
    var c = yyA.cfg, info = yyA.info;
    host.querySelectorAll("button[data-owner]").forEach((b) => b.onclick = () => {
      var own = document.getElementById("yy-qq-companion-owner");
      if (!own) return;
      own.dataset.user = b.dataset.owner;
      own.value = (c.nicknames && c.nicknames[b.dataset.owner]) || info.members[b.dataset.owner] || "群成员";
      T("服主已选择，请保存助手设置", "s");
    });
    host.querySelectorAll("button[data-nickname]").forEach((b) => b.onclick = async () => {
      var n = prompt("显示昵称", (c.nicknames && c.nicknames[b.dataset.nickname]) || "");
      if (n === null) return;
      var aliases = { ...(c.nicknames || {}) };
      aliases[b.dataset.nickname] = n.trim().slice(0, 64);
      var r = await A("POST", "/api/qq/config", { assistant: { nicknames: aliases } });
      if (r.ok) { yyA.cfg.nicknames = aliases; await yyQQRefreshMembers(); }
      else T(r.error, "e");
    });
    host.querySelectorAll("button[data-user]").forEach((b) => b.onclick = async () => {
      b.disabled = true;
      try {
        var r = await A("POST", "/api/qq/assistant", { action: "authorize", user: b.dataset.user, role: b.dataset.role });
        if (!r.ok) throw Error(r.error);
        var conf = await A("GET", "/api/qq/config");
        var a = (conf.config && conf.config.assistant) || {};
        yyA.cfg.super_admins = a.super_admins || [];
        yyA.cfg.admins = a.admins || [];
        var sup = document.getElementById("yy-qq-super"), adm = document.getElementById("yy-qq-admins");
        if (sup) sup.value = yyA.cfg.super_admins.join("\n");
        if (adm) adm.value = yyA.cfg.admins.join("\n");
        await yyQQRefreshMembers();
        T("成员权限已保存", "s");
      } catch (e) {
        T(e.message, "e");
        b.disabled = false;
      }
    });
  }

  function wire() {
    if (!document.getElementById("yy-qq-assistant-panel")) return;
    var c = yyA.cfg, m = yyA.models;
    var ai = document.getElementById("yy-qq-ai-model");
    if (ai) ai.value = c.model || "";
    var im = document.getElementById("yy-qq-image-model");
    if (im) im.value = c.image_model || "";
    if (ai && window.yyBrandAttach) window.yyBrandAttach(ai, m);
    if (im && window.yyBrandAttach) window.yyBrandAttach(im, m);
    var own = document.getElementById("yy-qq-companion-owner");
    if (own) { own.dataset.user = c.companion_owner || ""; own.readOnly = true; }
    var sel = document.getElementById("yy-qq-message-group");
    if (sel) sel.onchange = yyQQRefreshMessages;
    wireMemberButtons();
    wireAliasEditor();
    wireProactive();
    wireAIMode();
    // 工具 chips 是数据回来后动态渲染的，用委托接点击，不用逐个绑。
    var box = document.getElementById("yy-qq-ai-tools");
    if (box) box.addEventListener("click", function (ev) {
      var b = ev.target && ev.target.closest ? ev.target.closest("[data-insert]") : null;
      if (b) yyAInsertTool(b.getAttribute("data-insert"));
    });
  }

  function render() {
    var el = document.getElementById("yyq-a-body");
    if (el) el.innerHTML = body();
    wire();
  }

  function mountMemory() {
    if (typeof window.yyQQMemoryMount !== "function") return;
    var panel = document.getElementById("yy-qq-assistant-panel");
    if (!panel || document.getElementById("yy-qq-memory-panel")) return;
    window.yyQQMemoryMount(yyA.cfg, yyA.models, yyA.groups);
  }

  function load() {
    yyA.loading = 1;
    var jobs = [
      A("GET", "/api/qq/config"),
      A("GET", "/api/ai/models"),
      A("GET", "/api/qq/assistant"),
      A("GET", "/api/qq/groups"),
      A("GET", "/api/qq/ai/selfcheck"),
      A("GET", "/api/qq/assistant/skills"),
      A("GET", "/api/qq/proactive")
    ];
    Promise.all(jobs.map((p) => p && p.then ? p.catch(() => null) : Promise.resolve(null))).then(async (rs) => {
      yyA.loading = 0;
      yyA.loaded = 1;
      var cfgR = rs[0], mR = rs[1], infoR = rs[2], gR = rs[3], scR = rs[4], skR = rs[5], prR = rs[6];
      if (cfgR && cfgR.config) yyA.cfg = cfgR.config.assistant || {};
      if (mR && mR.models) yyA.models = mR.models;
      if (infoR && infoR.ok !== false) yyA.info = { help: infoR.help || "", members: infoR.members || {}, bindings: infoR.bindings || {} };
      if (gR && gR.groups) yyA.groups = gR.groups;
      if (scR && scR.ok !== false) {
        var seen = {};
        yyA.tools = (scR.tools || []).concat(scR.social_tools || []).filter(function (n) {
          return n && !seen[n] && (seen[n] = 1);
        });
        yyA.toolsLoaded = 1;
      }
      if (skR && skR.ok !== false) yyA.skills = (skR.skills || []).filter(function (s) { return s && s.id; });
      if (prR && prR.ok !== false) { yyA.proactive = prR.config || null; yyA.proactiveErr = ""; }
      else yyA.proactiveErr = (prR && prR.error) || "\u8BFB\u53D6\u5931\u8D25";
      render();
      yyARenderToolChips();
      await yyQQRefreshAll();
      mountMemory();
    });
  }
  window.yyQQRefreshMessages = async function() {
    var select = document.getElementById("yy-qq-message-group"), host = document.getElementById("yy-qq-group-messages");
    if (!host) return;
    var r = await A("GET", "/api/qq/messages?limit=30&group=" + encodeURIComponent((select == null ? void 0 : select.value) || ""));
    if (!r.ok) {
      yyA.messagesHtml = r.error || "\u8BFB\u53D6\u5931\u8D25";
      yyA.messagesShown = null;
      host.textContent = yyA.messagesHtml;
      return;
    }
    yyA.messagesHtml = (r.messages || []).slice().reverse().map(function (m) {
      // 头像和昵称同一行：32px 圆角，图片走 data-skin-media（见 yyQqMsgAvatar）。
      return '<article data-msg="' + yyEscape(m.id || "") + '" style="display:flex;gap:8px;padding:12px;border-bottom:1px solid var(--border);overflow-wrap:anywhere">' +
        yyQqMsgAvatar(m) +
        '<div style="min-width:0;flex:1">' +
          '<div style="display:flex;align-items:baseline;gap:6px;flex-wrap:wrap"><strong>' + yyEscape(m.name) + '</strong> <small>' + yyEscape(new Date(m.timestamp).toLocaleString()) + "</small></div>" +
          '<p style="margin:4px 0 0">' + yyEscape(m.text) + "</p>" +
        "</div></article>";
    }).join("") || "<p>\u5C1A\u672A\u6536\u5230\u7FA4\u6D88\u606F\uFF1B\u6B64\u5904\u4E0D\u8BFB\u53D6QQ\u5BA8\u6237\u7AEF\u5B8C\u6574\u5386\u53F2\u3002</p>";
    // 消息判重：与上次写进去的 HTML 一致就不重写，无变化刷新不再冲页面。
    if (yyA.messagesHtml !== yyA.messagesShown) {
      host.innerHTML = yyA.messagesHtml;
      yyA.messagesShown = yyA.messagesHtml;
    }
  };
  // 整体刷新：成员 + 消息一次刷完（用户第 19 条：刷新能不能整体刷，
  // 一动一动地很难受）。保留单个函数供切群、授权后等场景单独调用。
  window.yyQQRefreshAll = async function() {
    await window.yyQQRefreshMembers();
    await window.yyQQRefreshMessages();
  };
  window.yyQQRefreshMembers = async function() {
    var _a;
    var info = await A("GET", "/api/qq/assistant"), config = await A("GET", "/api/qq/config"), c = ((_a = config.config) == null ? void 0 : _a.assistant) || {}, host = document.getElementById("yy-qq-member-cards");
    if (info && info.ok !== false) yyA.info = { help: info.help || "", members: info.members || {}, bindings: info.bindings || {} };
    yyA.cfg.super_admins = c.super_admins || [];
    yyA.cfg.admins = c.admins || [];
    yyA.cfg.nicknames = c.nicknames || {};
    var help = document.getElementById("yy-qq-helper-help");
    if (help) help.textContent = yyA.info.help || "";
    var members = document.getElementById("yy-qq-helper-members");
    if (members) members.textContent = membersText();
    if (!host) return;
    var ids = Object.keys(info.members || {});
    yyA.memberCards = ids.length ? ids.map((id) => {
      var _a2;
      var level = (c.super_admins || []).includes(id) ? "\u8D85\u7EA7\u7BA1\u7406\u5458" : (c.admins || []).includes(id) ? "\u7BA1\u7406\u5458" : "\u6210\u5458";
      return '<section class="yy-card"><strong>' + yyEscape(((_a2 = c.nicknames) == null ? void 0 : _a2[id]) || (info.members[id] === id ? "\u7FA4\u6210\u5458" : info.members[id])) + " \xB7 " + level + '</strong><details><summary>\u5185\u90E8\u8EAB\u4EFD\u8BE6\u60C5</summary><pre style="white-space:pre-wrap;word-break:break-all">' + yyEscape(id) + '</pre></details><button class="btn-s" data-owner="' + yyEscape(id) + '">\u8BBE\u4E3A\u670D\u4E3B</button><button class="btn-s" data-nickname="' + yyEscape(id) + '">\u4FEE\u6539\u663E\u793A\u6635\u79F0</button><button class="btn-s" data-user="' + yyEscape(id) + '" data-role="admin">\u8BBE\u4E3A\u7BA1\u7406\u5458</button><button class="btn-s" data-user="' + yyEscape(id) + '" data-role="super">\u8BBE\u4E3A\u8D85\u7EA7\u7BA1\u7406\u5458</button><button class="btn-s" data-user="' + yyEscape(id) + '" data-role="member">\u64A4\u9500\u6388\u6743</button></section>';
    }).join("") : '<p class="yy-hint">\u5C1A\u672A\u6536\u5230\u6210\u5458\u4E8B\u4EF6\u3002\u8BF7\u5728\u7FA4\u91CC @\u673A\u5668\u4EBA\u53D1\u9001 .\u8EAB\u4EFD\u3002</p>';
    // 成员目录判重：和「上次自己写进去的 HTML」比，不一致才重写，滚动位置与
    // 按钮事件都不动。不能和 host.innerHTML 比——成员目录 UI（member_ui.js）
    // 接管后容器里的 markup 就变了，那样比永远不相等，反而每次全量重写、
    // 页面一跳一跳（用户第 19 条）。拼串顺序稳定（后端 map 键有序）就判得准。
    if (yyA.memberCards !== yyA.memberCardsShown) {
      host.innerHTML = yyA.memberCards;
      yyA.memberCardsShown = yyA.memberCards;
      wireMemberButtons();
    }
  };
  window.yyQQAssistantDraft = function() {
    if (!document.getElementById("yy-qq-assistant-panel")) return void 0;
    var v = (id) => document.getElementById(id).value, lines = (id) => v(id).split("\n").map((x) => x.trim()).filter(Boolean), on = (id) => document.getElementById(id).checked;
    var draft = { memory: typeof yyQQMemoryDraft === "function" ? yyQQMemoryDraft() : void 0, enabled: on("yy-qq-helper-enabled"), companion: on("yy-qq-companion"), companion_all_messages: on("yy-qq-companion-all"), companion_owner: document.getElementById("yy-qq-companion-owner").dataset.user || "", image_model: v("yy-qq-image-model"), companion_keywords: lines("yy-qq-companion-words"), super_admins: lines("yy-qq-super"), admins: lines("yy-qq-admins"), death: on("yy-qq-death"), join: on("yy-qq-join"), leave: on("yy-qq-leave"), public_objectives: lines("yy-qq-public-scores"), model: v("yy-qq-ai-model"), mcp_url: v("yy-qq-mcp-url").trim(), mcp_token: v("yy-qq-mcp-token").trim() };
    // 提示词只在用户真改过时才提交。没改过就整段不发，后端深合并会保留原值——
    // 否则这里会把「载入时的旧快照」（多半是空串）当成新值写回去，
    // 在另一个面板存好的提示词就这样被清空了。
    if (typeof yyQQPromptDirty === "function" && yyQQPromptDirty()) draft.prompt = v("yy-qq-ai-prompt");
    var numv = function (id) {
      var el = document.getElementById(id);
      if (!el || el.value === "") return void 0;
      var n = parseInt(el.value, 10);
      return isNaN(n) ? void 0 : n;
    };
    var txtv = function (id) {
      var el = document.getElementById(id);
      return el ? String(el.value || "").trim() : void 0;
    };
    var extras = {
      companion_window: numv("yy-qq-companion-window"),
      ai_mode: txtv("yy-qq-ai-mode"),
      ai_interval_minutes: numv("yy-qq-ai-interval"),
      ai_night_start_hour: numv("yy-qq-ai-night-start"),
      ai_night_end_hour: numv("yy-qq-ai-night-end"),
      ai_night_factor: numv("yy-qq-ai-night-factor"),
      ai_quiet_start: txtv("yy-qq-ai-quiet-start"),
      ai_quiet_end: txtv("yy-qq-ai-quiet-end")
    };
    Object.keys(extras).forEach(function (k) { if (extras[k] !== void 0) draft[k] = extras[k]; });
    return draft;
  };
  // console.js 的「人设与试跑」面板保存后回调这里，让本面板的内存快照跟着更新。
  window.yyQQAssistantPromptSync = function (v) {
    yyA.cfg = yyA.cfg || {};
    yyA.cfg.prompt = String(v == null ? "" : v);
  };
  window.yyQQAssistantSave = async function() {
    try {
      var draft = yyQQAssistantDraft();
      var r = await A("POST", "/api/qq/config", { assistant: draft });
      if (!r.ok) throw Error(r.error);
      document.getElementById("yy-qq-mcp-token").value = "";
      // 保存成功：把两个输入框与两份内存快照一起对齐，谁也不会再拿旧值覆盖。
      if (draft.prompt !== void 0 && typeof yyQQPromptSync === "function") yyQQPromptSync(draft.prompt);
      else yyA.cfg.prompt = typeof yyQQPromptValue === "function" ? yyQQPromptValue() : yyA.cfg.prompt;
      if (draft.model !== void 0) yyA.cfg.model = draft.model;
      T("\u52A9\u624B\u8BBE\u7F6E\u5DF2\u4FDD\u5B58", "s");
      return true;
    } catch (e) {
      T(e.message, "e");
      return false;
    }
  };
  window.yyQQCreateClaim = async function() {
    var r = await A("POST", "/api/qq/assistant", { action: "claim" });
    document.getElementById("yy-qq-claim").textContent = r.ok ? r.instruction + "\n10\u5206\u949F\u6709\u6548\uFF0C\u4E00\u6B21\u4F7F\u7528\uFF1B\u8BF7\u7531\u4F60\u5E0C\u671B\u6388\u6743\u7684\u4EBA\u53D1\u9001\u3002" : r.error;
  };
  // 模型ID 不再手填：带 base_url + api_key 走后端代理 POST /api/ai/models/fetch
  // （服务端转发 <base>/models，OpenAI 兼容协议；浏览器不直连第三方，无 CORS 问题），
  // 拿回 data[].id 填进下拉框供选择。密钥只进请求体，不回显。
  function yyQQFetchErrorText(err) {
    var t = String(err || "\u83B7\u53D6\u5931\u8D25");
    if (/401/.test(t)) return "\u5BC6\u94A5\u4E0D\u5BF9\uFF08HTTP 401\uFF09\uFF1A\u8BF7\u68C0\u67E5 API \u5BC6\u94A5\u662F\u5426\u6B63\u786E\u3001\u8FC7\u671F\uFF1B\u90E8\u5206\u5E73\u53F0\u5BC6\u94A5\u9700\u8981 sk- \u5F00\u5934\u3002";
    if (/403/.test(t)) return "\u6CA1\u6743\u9650\uFF08HTTP 403\uFF09\uFF1A\u8BE5\u5BC6\u94A5\u53EF\u80FD\u6CA1\u6709\u67E5\u770B\u6A21\u578B\u5217\u8868\u7684\u6743\u9650\u3002";
    if (/404/.test(t)) return "\u5730\u5740\u4E0D\u5BF9\uFF08HTTP 404\uFF09\uFF1AAPI \u5730\u5740\u4E00\u822C\u8981\u5E26\u5230 /v1\uFF0C\u4F8B\u5982 https://api.deepseek.com/v1\uFF1B\u4E5F\u786E\u8BA4\u8BE5\u670D\u52A1\u5546\u652F\u6301 /models \u63A5\u53E3\u3002";
    if (/\u8D85\u65F6|timeout|deadline/i.test(t)) return "\u8FDE\u63A5\u8D85\u65F6\uFF1A\u5730\u5740\u4E0D\u53EF\u8FBE\u6216\u7F51\u7EDC\u4E0D\u901A\uFF1B\u81EA\u5EFA\u670D\u52A1\u8BF7\u786E\u8BA4\u7AEF\u53E3\u5F00\u653E\u3002";
    if (/\u89E3\u6790\u5931\u8D25/.test(t)) return "\u670D\u52A1\u7AEF\u8FD4\u56DE\u7684\u4E0D\u662F\u6A21\u578B\u5217\u8868\u683C\u5F0F\uFF0C\u8BE5\u5730\u5740\u53EF\u80FD\u4E0D\u662F OpenAI \u517C\u5BB9\u63A5\u53E3\u3002";
    return t;
  }
  async function yyQQFetchModels(kind) {
    var isImg = kind === "image";
    var base = document.getElementById(isImg ? "yy-qq-image-base" : "yy-qq-model-base").value.trim();
    var key = document.getElementById(isImg ? "yy-qq-image-key" : "yy-qq-model-key").value.trim();
    var name = document.getElementById(isImg ? "yy-qq-image-name" : "yy-qq-model-name").value.trim();
    var sel = document.getElementById(isImg ? "yy-qq-image-id" : "yy-qq-model-id");
    var status = document.getElementById(isImg ? "yy-qq-image-fetch-status" : "yy-qq-model-fetch-status");
    var btn = document.getElementById(isImg ? "yy-qq-image-fetch-btn" : "yy-qq-model-fetch-btn");
    if (!base) { T("\u5148\u586B\u5199 API \u5730\u5740\u518D\u83B7\u53D6\u6A21\u578B\u5217\u8868", "e"); return; }
    if (btn) { btn.disabled = true; btn.textContent = "\u83B7\u53D6\u4E2D\u2026"; }
    if (status) status.textContent = "\u6B63\u5728\u5411 " + base + " \u83B7\u53D6\u6A21\u578B\u5217\u8868\u2026";
    try {
      var r = await A("POST", "/api/ai/models/fetch", { provider: "openai", base_url: base, api_key: key, name: name });
      if (!r || !r.ok) throw Error((r && r.error) || "\u83B7\u53D6\u5931\u8D25");
      var ids = r.models || [];
      if (!ids.length) {
        if (status) status.textContent = "\u8BE5\u5730\u5740\u6CA1\u6709\u8FD4\u56DE\u4EFB\u4F55\u6A21\u578B\u3002";
        T("\u8BE5\u5730\u5740\u6CA1\u6709\u8FD4\u56DE\u6A21\u578B", "w");
        return;
      }
      sel.innerHTML = "";
      ids.forEach(function (id) {
        var o = new Option(id, id);
        o.setAttribute("data-yy-mid", id);
        o.setAttribute("data-yy-base", base);
        sel.add(o);
      });
      if (window.yyBrandRefresh) window.yyBrandRefresh(sel);
      if (status) status.textContent = "\u627E\u5230 " + ids.length + " \u4E2A\u6A21\u578B\uFF0C\u70B9\u4E0A\u65B9\u4E0B\u62C9\u9009\u62E9\u3002";
      T("\u627E\u5230 " + ids.length + " \u4E2A\u6A21\u578B", "o");
    } catch (e) {
      var msg = yyQQFetchErrorText(e && e.message);
      if (status) status.textContent = "\u83B7\u53D6\u5931\u8D25\uFF1A" + msg;
      T("\u83B7\u53D6\u6A21\u578B\u5217\u8868\u5931\u8D25", "e");
    } finally {
      if (btn) { btn.disabled = false; btn.textContent = "\u83B7\u53D6\u6A21\u578B\u5217\u8868"; }
    }
  }
  window.yyQQFetchImageModels = function () { return yyQQFetchModels("image"); };
  window.yyQQFetchChatModels = function () { return yyQQFetchModels("chat"); };
  // 手动输入兑底：服务端不返回 /models 时也能填模型ID
  window.yyQQManualModel = function (kind) {
    var isImg = kind === "image";
    var input = document.getElementById(isImg ? "yy-qq-image-id-manual" : "yy-qq-model-id-manual");
    var sel = document.getElementById(isImg ? "yy-qq-image-id" : "yy-qq-model-id");
    var id = (input && input.value || "").trim();
    if (!id) { T("\u5148\u8F93\u5165\u6A21\u578BID", "e"); return; }
    var exists = Array.prototype.some.call(sel.options, function (o) { return o.value === id; });
    if (!exists) {
      var o = new Option(id, id);
      o.setAttribute("data-yy-mid", id);
      o.setAttribute("data-yy-base", document.getElementById(isImg ? "yy-qq-image-base" : "yy-qq-model-base").value.trim());
      sel.add(o);
    }
    sel.value = id;
    if (window.yyBrandRefresh) window.yyBrandRefresh(sel);
    T("\u5DF2\u9009\u4E2D " + id, "s");
  };
  window.yyQQSaveImageModel = async function() {
    var v = (id) => document.getElementById(id).value.trim(), status = document.getElementById("yy-qq-image-result");
    try {
      var name = v("yy-qq-image-name"), base = v("yy-qq-image-base"), model = v("yy-qq-image-id");
      if (!name || !base || !model) throw Error("\u8BF7\u586B\u5199\u914D\u7F6E\u540D\u79F0\u3001API\u5730\u5740\u548C\u56FE\u50CF\u6A21\u578BID");
      var r = await A("POST", "/api/ai/models", { name, provider: "openai", base_url: base, model_id: model, api_key: v("yy-qq-image-key"), thinking: "off", streaming: false });
      if (!r.ok) throw Error(r.error);
      var select = document.getElementById("yy-qq-image-model");
      if (!Array.from(select.options).some((o) => o.value === name)) {
        var added = new Option(name + " \xB7 " + model, name);
        added.setAttribute("data-yy-base", base);
        added.setAttribute("data-yy-mid", model);
        select.add(added);
      }
      select.value = name;
      if (window.yyBrandRefresh) window.yyBrandRefresh(select);
      var saved = await A("POST", "/api/qq/config", { assistant: { image_model: name } });
      if (!saved.ok) throw Error(saved.error);
      document.getElementById("yy-qq-image-key").value = "";
      status.textContent = "\u7ED8\u56FE\u63A5\u53E3\u5DF2\u4FDD\u5B58\u5E76\u9009\u7528\u3002\u7FA4\u91CC\u53EF\u7528 .AI \u5E2E\u6211\u753B\u2026\uFF1B\u56FE\u50CF\u5B9E\u9645\u751F\u6210\u4E0E\u4E0A\u4F20\u6210\u529F\u540E\u624D\u786E\u8BA4\u53D1\u9001\u3002";
    } catch (e) {
      status.textContent = "\u4FDD\u5B58\u5931\u8D25\uFF1A" + e.message;
    }
  };
  window.yyQQSaveModel = async function() {
    var v = (id) => document.getElementById(id).value.trim(), result = document.getElementById("yy-qq-model-result");
    try {
      var name = v("yy-qq-model-name"), r = await A("POST", "/api/ai/models", { name, provider: "openai", base_url: v("yy-qq-model-base"), model_id: v("yy-qq-model-id"), api_key: v("yy-qq-model-key"), thinking: "off", streaming: false, max_output_tokens: 1800, context_window: 32e3 });
      if (!r.ok) throw Error(r.error);
      document.getElementById("yy-qq-model-key").value = "";
      var select = document.getElementById("yy-qq-ai-model");
      if (!Array.from(select.options).some((x) => x.value === name)) {
        var added = new Option(name, name);
        added.setAttribute("data-yy-base", v("yy-qq-model-base"));
        added.setAttribute("data-yy-mid", v("yy-qq-model-id"));
        select.add(added);
      }
      select.value = name;
      if (window.yyBrandRefresh) window.yyBrandRefresh(select);
      if (!await yyQQAssistantSave()) throw Error("\u6A21\u578B\u5DF2\u4FDD\u5B58\uFF0C\u4F46\u7FA4\u52A9\u624B\u9009\u62E9\u672A\u4FDD\u5B58\uFF0C\u8BF7\u91CD\u8BD5\u4FDD\u5B58\u52A9\u624B\u8BBE\u7F6E");
      result.textContent = "\u6A21\u578B\u63A5\u53E3\u5DF2\u4FDD\u5B58\uFF0C\u7FA4\u52A9\u624B\u5DF2\u9009\u7528\u6B64\u914D\u7F6E\u3002";
    } catch (e) {
      result.textContent = "\u4FDD\u5B58\u5931\u8D25\uFF1A" + e.message;
    }
  };

  // ---------- 挂载 ----------
  // 与知识库/触发词/服务器信息/危险指令一致：包装 yyQQRenderAll，
  // 往 #yyq-root .yyq-fold-grid 插折叠块；幂等防重入。
  function mount() {
    if (typeof window.yyQQRegisterModule !== "function") return;
    window.yyQQRegisterModule({
      id: "ai",
      __key: "assistant",
      order: 50,
      render: function () { return block(); },
      load: function () {
        // 模块展开/壳重建后 DOM 已插入，这时才能 wire 到真实元素。
        wire();
        mountMemory();
        if (!yyA.loaded && !yyA.loading) load();
      }
    });
  }

  window.yyQQAFold = function (el) { yyA.open = el.open ? 1 : 0; };

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", mount);
  else mount();

})();
