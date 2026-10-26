// 群服互通 · 触发词规则 + 生图配置
//
// 设计：用户要的「能力可配置，不是固定模板」。规则 = 匹配方式 + 一串动作，
// 动作参数全部开放可改（点赞次数、回复模板、画图提示词/尺寸/是否用头像…）。
// 挂在 youyuan_qq_console.js 之上：包装 yyQQRenderAll 注入两个折叠块，
// 不修改 console 本体，避免被后续更新覆盖。
//
// 接口：GET/POST /api/qq/triggers（整体替换）、GET/POST /api/qq/image（扁平 patch）。

(function () {
  "use strict";

  var yyTCfg = {
    loading: 1, started: 0, loaded: 0, err: "", rules: [], presets: [], types: [], actions: [], vars: [],
    open: 0, edit: "", draft: null, busy: 0, saved: 0, regexErr: ""
  };
  var yyICfg = {
    loading: 1, started: 0, loaded: 0, err: "", cfg: null, providers: [], open: 0, busy: 0, saved: 0, newKey: ""
  };

  // ---------- A1：编辑保护（整页定时刷新不要打断输入） ----------
  // 以前 console 每 10 分钟、切回前台都会 yyQQRenderAll 整页重绘，用户正在
  // 触发词规则里展开/输入时 DOM 被换掉，表现就是「一直刷新，点不过去」。
  // 这里在区块内任一 input/textarea/select 获得焦点时立一个全局标记，
  // console 的重绘看到标记直接跳过；失焦 2 秒后才解除，不会长期卡住刷新。
  if (typeof window.__yyQQEditing !== "boolean") window.__yyQQEditing = false;
  var yyEditHoldTimer = null;
  function yyTrigHoldIn(el) {
    while (el) {
      if (el.id === "yyq-triggers" || el.id === "yyq-relay") return true;
      el = el.parentNode;
    }
    return false;
  }
  function yyTrigEditGuard() {
    if (window.__yyQQEditGuard) return;
    window.__yyQQEditGuard = 1;
    function hit(e) {
      var t = e.target;
      if (!t || !t.tagName) return false;
      if (!/^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName)) return false;
      return yyTrigHoldIn(t);
    }
    document.addEventListener("focusin", function (e) {
      if (!hit(e)) return;
      if (yyEditHoldTimer) { clearTimeout(yyEditHoldTimer); yyEditHoldTimer = null; }
      window.__yyQQEditing = true;
    }, true);
    document.addEventListener("focusout", function (e) {
      if (!hit(e)) return;
      if (yyEditHoldTimer) clearTimeout(yyEditHoldTimer);
      yyEditHoldTimer = setTimeout(function () {
        yyEditHoldTimer = null;
        window.__yyQQEditing = false;
      }, 2000);
    }, true);
    // 点区块外面立刻解除：选中项被脚本重绘、焦点被丢掉时不会触发 focusout，
    // 没有这一条标记可能一直挂着，整页刷新就被永久跳过了。
    document.addEventListener("mousedown", function (e) {
      if (yyTrigHoldIn(e.target)) return;
      if (yyEditHoldTimer) { clearTimeout(yyEditHoldTimer); yyEditHoldTimer = null; }
      window.__yyQQEditing = false;
    }, true);
  }

  function yyTEsc(v) {
    return typeof yyQQE === "function" ? yyQQE(v == null ? "" : String(v)) : String(v == null ? "" : v);
  }
  function yyTIcon(n) {
    return typeof yyQQIcon === "function" ? yyQQIcon(n) : "";
  }
  function yyTGroups() {
    if (typeof yyQQEnabledGroups === "function") return yyQQEnabledGroups();
    var cfg = (window.yyQQC && yyQQC.cfg) || {};
    return ((cfg && cfg.groups) || []).filter(function (g) { return g && (g.id || g.group_id); });
  }
  function yyTGroupLabel(g) {
    if (typeof yyQQGroupName === "function") return yyQQGroupName(g);
    return (g && (g.name || g.id || g.group_id)) || "";
  }

  // ---------- 触发词：数据 ----------

  function yyTLoad() {
    yyTCfg.loading = 1;
    yyTCfg.err = "";
    A("GET", "/api/qq/triggers").then(function (r) {
      yyTCfg.loading = 0;
      if (!r || r.ok === false) {
        yyTCfg.err = (r && r.error) || "读取规则失败";
        yyTRender();
        return;
      }
      yyTCfg.rules = r.triggers || [];
      yyTCfg.presets = r.presets || [];
      yyTCfg.types = r.types || ["keyword", "regex", "at_bot", "any"];
      yyTCfg.actions = r.actions || [];
      yyTCfg.vars = r.vars || [];
      yyTRender();
    })["catch"](function () {
      yyTCfg.loading = 0;
      yyTCfg.err = "读取规则失败";
      yyTRender();
    });
  }

  var yyTTypeLabel = { keyword: "包含词", regex: "正则", at_bot: "@机器人", any: "任意消息" };
  var yyTActionIcon = { reply: "chat", like: "thumb", emoji: "face", command: "list", relay: "send", image: "image", capability: "sliders" };

  // capability 动作的参数提示（按能力 kind 给例子，别让用户猜格式）。
  // kind 选项不写死——从 GET /api/qq/triggers 的 actions 元数据动态取。
  var yyTCapArgHints = {
    like: "QQ号 [次数1-10]，留空给提问者点 1 次",
    emoji_like: "表情ID（留空用默认点赞表情）",
    qzone: "动态内容（1-500字，所有人可见）",
    delete_qzone: "要删除的动态 tid",
    avatar: "图片URL或base64",
    long_nick: "签名内容（1-100字）",
    nick: "新昵称（1-64字）",
    kick: "QQ号 [原因]",
    ban: "QQ号 [分钟数，默认10]",
    recall: "消息ID（留空撤回最近一条群消息）",
    poke: "QQ号（留空拍提问者）",
    sticker: "图片URL，或收藏表情序号（先发 .表情包 看序号）",
    relay_history: "条数1-50，留空5条"
  };
  // 从后端元数据里取 capability 动作的某个参数定义（kind/arg）。
  function yyTCapParam(key) {
    var list = yyTCfg.actions || [];
    for (var i = 0; i < list.length; i++) {
      if (!list[i] || list[i].type !== "capability") continue;
      var ps = list[i].params || [];
      for (var j = 0; j < ps.length; j++) {
        if (ps[j] && ps[j].key === key) return ps[j];
      }
    }
    return null;
  }
  // 切换能力 kind 时只更新参数框 placeholder，不整块重渲（保住焦点和已填内容）。
  function yyTCapArgHint(i, kind) {
    var inp = document.getElementById("yyq-act-arg-" + i);
    if (inp) inp.setAttribute("placeholder", yyTCapArgHints[kind] || "按上面选的能力填写参数");
  }

  function yyTActionLabel(type) {
    var list = yyTCfg.actions || [];
    for (var i = 0; i < list.length; i++) if (list[i] && list[i].type === type) return list[i].label || type;
    return type;
  }
  function yyTRuleSummary(t) {
    var bits = [];
    bits.push(yyTTypeLabel[t.Type || t.type] || t.Type || t.type || "包含词");
    var pats = t.Patterns || t.patterns || [];
    if (pats.length) bits.push(pats.slice(0, 2).join(" / ") + (pats.length > 2 ? " 等" : ""));
    if ((t.Type || t.type) === "at_bot") bits.push("被 @ 时");
    if ((t.Type || t.type) === "any") bits.push("每条消息");
    var acts = t.Actions || t.actions || [];
    if (acts.length) bits.push(acts.map(function (a) { return yyTActionLabel(a.Type || a.type); }).join("+"));
    return bits.join(" · ");
  }
  function yyTRuleMeta(t) {
    var cd = t.Cooldown || t.cooldown || 0, dl = t.Daily || t.daily || 0;
    var s = "";
    if (cd) s += "冷却 " + (cd >= 60 ? Math.round(cd / 60) + " 分" : cd + " 秒");
    if (dl) s += (s ? " · " : "") + "每日 " + dl + " 次";
    return s || "不限";
  }

  // ---------- 触发词：块渲染 ----------

  function yyTrigBlock() {
    var sum = yyTCfg.loading ? "读取中…" : (yyTCfg.rules.length ? yyTCfg.rules.length + " 条规则生效" : "还没有规则");
    return '<section class="yyq-sec yyq-sec-wide" id="yyq-triggers">' +
      '<div class="yyq-sec-head"><h4>触发词规则</h4><small>' + yyTEsc(sum) + "</small></div>" +
      '<div id="yyq-trig-body">' + yyTrigBody() + "</div>" +
      "</section>";
  }

  function yyTrigBody() {
    if (yyTCfg.loading) {
      var sk = "";
      for (var i = 0; i < 4; i++) sk += '<div class="yyq-skel"></div>';
      return sk;
    }
    if (yyTCfg.err) {
      return '<p class="yyq-inline-err"><span class="yyq-inline-err-ico">' + yyTIcon("ban") + "</span>" +
        yyTEsc(yyTCfg.err) + '<button type="button" class="yyq-linkbtn" onclick="yyTLoad()">重试</button></p>';
    }
    var h = '<div class="yyq-cfg-toolbar">' +
      '<button type="button" class="btn" onclick="yyTNew()">+ 新建规则</button>' +
      (yyTCfg.saved ? '<span class="yyq-cfg-saved">已保存</span>' : "") +
      '<span class="yyq-cfg-toolbar-hint">规则命中后系统直接执行，不再经过 AI</span>' +
      "</div>";
    if (yyTCfg.presets.length) {
      h += '<div class="yyq-cfg-chips"><span class="yyq-cfg-chips-t">从模板创建：</span>';
      yyTCfg.presets.forEach(function (p) {
        h += '<button type="button" class="yyq-chip" onclick="yyTFromPreset(\'' + yyTEsc(p.ID || p.id) + "')\">" + yyTEsc(p.Name || p.name) + "</button>";
      });
      h += "</div>";
    }
    if (!yyTCfg.rules.length) {
      h += '<div class="yyq-empty"><span class="yyq-empty-ico">' + yyTIcon("spark") + "</span>" +
        "<p>还没有触发词规则</p><small>比如群里有人发「赞我」，机器人就去点赞——点上面的模板或「新建规则」开始</small></div>";
    } else {
      h += '<ul class="yyq-cap-list yyq-cfg-list">';
      yyTCfg.rules.forEach(function (t) {
        var id = t.ID || t.id;
        var en = (t.Enabled !== false);
        var acts = t.Actions || t.actions || [];
        var ico = acts.length ? (yyTActionIcon[acts[0].Type || acts[0].type] || "spark") : "spark";
        var rmin = parseInt(t.min_role != null ? t.min_role : (t.MinRole || 0), 10) || 0;
        h += '<li class="yyq-cap yyq-cfg-rule" data-on="' + (en ? 1 : 0) + '" onclick="yyTEdit(\'' + yyTEsc(id) + "',event)\">" +
          '<span class="yyq-cap-ico">' + yyTIcon(ico) + "</span>" +
          '<span class="yyq-cap-body"><strong>' + yyTEsc(t.Name || t.name || "未命名") + "</strong>" +
          "<small>" + yyTEsc(yyTRuleSummary(t)) + "</small></span>" +
          // 规则分层：谁能触发（所有人 / 管理员 / 超管）。后端 yyQQTrigger.MinRole。
          '<select class="yyq-cfg-role" aria-label="谁能触发这条规则" onclick="event.stopPropagation()"' +
            ' onchange="yyTSetRuleRole(\'' + yyTEsc(id) + "',this)\">" +
            '<option value="0"' + (rmin === 0 ? " selected" : "") + ">所有人</option>" +
            '<option value="1"' + (rmin === 1 ? " selected" : "") + ">管理员</option>" +
            '<option value="2"' + (rmin === 2 ? " selected" : "") + ">超管</option>" +
          "</select>" +
          '<span class="yyq-cap-meta">' + yyTEsc(yyTRuleMeta(t)) + "</span>" +
          '<label class="yyq-sw" onclick="event.stopPropagation()"><input type="checkbox"' + (en ? " checked" : "") +
          ' onchange="yyTToggle(\'' + yyTEsc(id) + "',this)\"><span></span></label>" +
          "</li>";
      });
      h += "</ul>";
    }
    if (yyTCfg.edit && yyTCfg.draft) h += yyTrigForm();
    return h;
  }

  // ---------- 触发词：表单 ----------

  function yyTNew() {
    yyTCfg.edit = "new";
    yyTCfg.draft = { ID: "", Enabled: true, Name: "", Type: "keyword", Patterns: [], Groups: [], Cooldown: 60, Daily: 5, min_role: 0, Actions: [{ Type: "reply", Params: { text: "" } }] };
    yyTCfg.regexErr = "";
    yyTRender();
    yyTScrollTo();
  }
  function yyTFromPreset(id) {
    var p = null;
    (yyTCfg.presets || []).forEach(function (x) { if ((x.ID || x.id) === id) p = x; });
    if (!p) return;
    yyTCfg.edit = "new";
    yyTCfg.draft = {
      ID: "", Enabled: p.Enabled !== false, Name: p.Name || p.name || "",
      Type: p.Type || p.type || "keyword",
      Patterns: (p.Patterns || p.patterns || []).slice(),
      Groups: (p.Groups || p.groups || []).slice(),
      Cooldown: p.Cooldown || p.cooldown || 0, Daily: p.Daily || p.daily || 0,
      min_role: p.min_role || p.MinRole || 0,
      Actions: (p.Actions || p.actions || []).map(function (a) {
        return { Type: a.Type || a.type, Params: Object.assign({}, a.Params || a.params || {}) };
      })
    };
    yyTCfg.regexErr = "";
    yyTRender();
    yyTScrollTo();
  }
  function yyTEdit(id, ev) {
    if (ev && ev.target && /INPUT|LABEL|BUTTON|TEXTAREA|SELECT/.test(ev.target.tagName)) return;
    var t = null;
    (yyTCfg.rules || []).forEach(function (x) { if ((x.ID || x.id) === id) t = x; });
    if (!t) return;
    yyTCfg.edit = id;
    yyTCfg.draft = {
      ID: t.ID || t.id, Enabled: t.Enabled !== false, Name: t.Name || t.name || "",
      Type: t.Type || t.type || "keyword",
      Patterns: (t.Patterns || t.patterns || []).slice(),
      Groups: (t.Groups || t.groups || []).slice(),
      Cooldown: t.Cooldown || t.cooldown || 0, Daily: t.Daily || t.daily || 0,
      min_role: t.min_role || t.MinRole || 0,
      Actions: (t.Actions || t.actions || []).map(function (a) {
        return { Type: a.Type || a.type, Params: Object.assign({}, a.Params || a.params || {}) };
      })
    };
    yyTCfg.regexErr = "";
    yyTRender();
    yyTScrollTo();
  }
  function yyTScrollTo() {
    var el = document.getElementById("yyq-trig-body");
    if (el && el.scrollIntoView) el.scrollIntoView({ behavior: "smooth", block: "center" });
  }
  function yyTCancel() {
    yyTCfg.edit = "";
    yyTCfg.draft = null;
    yyTCfg.regexErr = "";
    yyTRender();
  }

  function yyTrigForm() {
    var d = yyTCfg.draft;
    var h = '<div class="yyq-cfg-form" id="yyq-trig-form">' +
      '<div class="yyq-cfg-form-head"><strong>' + (yyTCfg.edit === "new" ? "新建规则" : "编辑规则") + "</strong>" +
      '<button type="button" class="yyq-iconbtn" onclick="yyTCancel()" aria-label="关闭">' + yyTIcon("ban") + "</button></div>";
    // 名称
    h += '<label class="yyq-field"><span>规则名称</span>' +
      '<input id="yyq-tf-name" value="' + yyTEsc(d.Name) + '" placeholder="比如：赞我" oninput="yyTField(\'Name\',this.value)"></label>';
    // 类型 segmented
    h += '<div class="yyq-field"><span>匹配方式</span><div class="yyq-seg yyq-seg-wide">';
    (yyTCfg.types.length ? yyTCfg.types : ["keyword", "regex", "at_bot", "any"]).forEach(function (tp) {
      h += '<button type="button" class="' + (d.Type === tp ? "is-on" : "") + '" onclick="yyTSetType(\'' + tp + "')\">" + yyTEsc(yyTTypeLabel[tp] || tp) + "</button>";
    });
    h += "</div></div>";
    // patterns
    if (d.Type !== "at_bot" && d.Type !== "any") {
      h += '<label class="yyq-field"><span>' + (d.Type === "regex" ? "正则（一行一个）" : "触发词（一行一个）") + "</span>" +
        '<textarea id="yyq-tf-pats" rows="3" placeholder="' + (d.Type === "regex" ? "^赞我$" : "赞我") + '" oninput="yyTField(\'Patterns\',this.value)">' +
        yyTEsc((d.Patterns || []).join("\n")) + "</textarea></label>";
      if (d.Type === "regex" && yyTCfg.regexErr) {
        h += '<p class="yyq-inline-err"><span class="yyq-inline-err-ico">' + yyTIcon("ban") + "</span>" + yyTEsc(yyTCfg.regexErr) + "</p>";
      }
    } else {
      h += '<p class="yyq-cfg-note">' + (d.Type === "at_bot" ? "只在有人 @ 机器人时触发，不需要触发词。" : "任意消息模式：群里任何人说的任何话都会触发这条规则，完全不看内容——只适合做全局自动回复，且务必设置每人冷却和每日上限。") + "</p>";
      // any 模式会静默忽略 patterns，而选 any 时输入框又被藏掉，用户根本
      // 看不出为什么「每句话都触发」（实际踩过：一条叫「赞我」的 any 规则
      // 让全群每句话都引来一句回复）。带着残留触发词时必须显式警告。
      if (d.Type === "any" && (d.Patterns || []).length) {
        h += '<p class="yyq-inline-err"><span class="yyq-inline-err-ico">' + yyTIcon("ban") + "</span>这条规则还带着 " + (d.Patterns || []).length + " 个触发词，但任意消息模式下它们既不生效也不参与匹配。想按词触发，请改用「包含词」。</p>";
      }
    }
    // 谁能触发（后端 min_role）：0 所有人 / 1 管理员 / 2 超管
    h += '<label class="yyq-field"><span>谁能触发</span>' +
      '<select id="yyq-tf-role" onchange="yyTField(\'min_role\',this.value)">' +
        '<option value="0"' + ((d.min_role || 0) === 0 ? " selected" : "") + ">所有人（含普通成员）</option>" +
        '<option value="1"' + (d.min_role === 1 ? " selected" : "") + ">仅管理员及以上</option>" +
        '<option value="2"' + (d.min_role === 2 ? " selected" : "") + ">仅超级管理员</option>" +
      "</select></label>";
    // 生效群
    var groups = yyTGroups();
    if (groups.length) {
      h += '<div class="yyq-field"><span>生效群（不勾选 = 全部群）</span><div class="yyq-cfg-groups">';
      groups.forEach(function (g) {
        var gid = g.id || g.group_id;
        var on = (d.Groups || []).indexOf(String(gid)) >= 0;
        h += '<label class="yyq-cfg-group' + (on ? " is-picked" : "") + '" data-picked="' + (on ? "1" : "0") + '" aria-pressed="' + (on ? "true" : "false") + '"><input type="checkbox"' + (on ? " checked" : "") +
          ' onchange="yyTToggleGroup(\'' + yyTEsc(gid) + "',this)\"><span>" + yyTEsc(yyTGroupLabel(g)) + "</span></label>";
      });
      h += "</div></div>";
    }
    // 冷却 / 日限
    h += '<div class="yyq-cfg-grid2">' +
      '<label class="yyq-field"><span>每人冷却（秒，0=不限）</span><input id="yyq-tf-cd" type="number" min="0" value="' + (d.Cooldown || 0) + '" oninput="yyTField(\'Cooldown\',this.value)"></label>' +
      '<label class="yyq-field"><span>每人每日上限（0=不限）</span><input id="yyq-tf-dl" type="number" min="0" value="' + (d.Daily || 0) + '" oninput="yyTField(\'Daily\',this.value)"></label>' +
      "</div>";
    // 动作编辑器
    h += '<div class="yyq-field"><span>命中后执行（按顺序，可多条）</span><div class="yyq-cfg-actions">';
    (d.Actions || []).forEach(function (a, i) { h += yyTActionForm(a, i); });
    h += '</div><button type="button" class="yyq-ghost yyq-cfg-addact" onclick="yyTAddAction()">+ 添加动作</button></div>';
    // 保存条
    h += '<div class="yyq-fold-actions">' +
      '<button type="button" class="btn" data-busy="' + (yyTCfg.busy ? 1 : 0) + '" onclick="yyTSave()">' + (yyTCfg.busy ? "保存中…" : "保存规则") + "</button>" +
      '<button type="button" class="btn-s" onclick="yyTCancel()">取消</button>' +
      (yyTCfg.edit !== "new" ? '<button type="button" class="yyq-ghost yyq-danger" onclick="yyTDel()">删除这条规则</button>' : "") +
      "</div></div>";
    return h;
  }

  // 单条动作编辑器：类型选择 + 按类型出参数
  function yyTActionForm(a, i) {
    var h = '<div class="yyq-cfg-action" data-i="' + i + '">' +
      '<div class="yyq-cfg-action-head">' +
      '<select class="yyq-cfg-act-type" onchange="yyTSetActionType(' + i + ',this.value)">';
    (yyTCfg.actions.length ? yyTCfg.actions : [
      { type: "reply", label: "回复文字" }, { type: "like", label: "点赞" },
      { type: "emoji", label: "贴表情" }, { type: "command", label: "执行游戏指令" },
      { type: "relay", label: "转发到游戏" }, { type: "image", label: "生成图片" },
      { type: "capability", label: "调用机器人能力" }
    ]).forEach(function (op) {
      h += '<option value="' + yyTEsc(op.type) + '"' + (a.Type === op.type ? " selected" : "") + ">" + yyTEsc(op.label || op.type) + "</option>";
    });
    h += "</select>" +
      '<button type="button" class="yyq-iconbtn" onclick="yyTDelAction(' + i + ')" aria-label="删除动作" title="删除这个动作">' + yyTIcon("trash") + "</button>" +
      "</div>";
    var p = a.Params || {};
    if (a.Type === "reply" || a.Type === "relay") {
      h += '<label class="yyq-field"><span>' + (a.Type === "relay" ? "转发文本" : "回复内容") + "</span>" +
        '<textarea rows="2" oninput="yyTSetParam(' + i + ",'" + (a.Type === "relay" ? "text" : "text") + "',this.value)\">" + yyTEsc(p.text || "") + "</textarea></label>";
      if (a.Type === "reply") h += yyTVarChips(i, "text");
    } else if (a.Type === "like") {
      h += '<label class="yyq-field"><span>点赞次数（1-10）</span><input type="number" min="1" max="10" value="' + yyTEsc(p.times || "1") + '" oninput="yyTSetParam(' + i + ",'times',this.value)\"></label>";
    } else if (a.Type === "emoji") {
      h += '<label class="yyq-field"><span>表情 ID</span><input value="' + yyTEsc(p.emoji_id || "") + '" placeholder="QQ 表情 id，如 4" oninput="yyTSetParam(' + i + ",'emoji_id',this.value)\"></label>";
    } else if (a.Type === "command") {
      h += '<label class="yyq-field"><span>游戏指令</span><input value="' + yyTEsc(p.input || "") + '" placeholder="如 list" oninput="yyTSetParam(' + i + ",'input',this.value)\"></label>";
    } else if (a.Type === "capability") {
      // 调用机器人能力：kind 下拉选项来自后端元数据（/api/qq/triggers 的
      // actions[].params[0].options），arg 输入框按 kind 给格式提示。
      var kindMeta = yyTCapParam("kind");
      var capOpts = (kindMeta && kindMeta.options) || [];
      var capKind = p.kind || (capOpts.length ? capOpts[0].value : "");
      h += '<div class="yyq-cfg-grid2">' +
        '<label class="yyq-field"><span>调用哪个能力</span>' +
        '<select onchange="yyTSetParam(' + i + ",'kind',this.value);yyTCapArgHint(" + i + ',this.value)">';
      capOpts.forEach(function (op) {
        h += '<option value="' + yyTEsc(op.value) + '"' + (capKind === op.value ? " selected" : "") + ">" + yyTEsc(op.label || op.value) + "</option>";
      });
      h += "</select></label>" +
        '<label class="yyq-field"><span>参数</span>' +
        '<input id="yyq-act-arg-' + i + '" value="' + yyTEsc(p.arg || "") + '" placeholder="' + yyTEsc(yyTCapArgHints[capKind] || "按上面选的能力填写参数") + '"' +
        ' oninput="yyTSetParam(' + i + ",'arg',this.value)\"></label>" +
        "</div>" +
        '<p class="yyq-cfg-note">命中后由机器人直接执行这个能力，与「能力」里的开关、限流、留痕共用；参数留空时按各能力的默认规矩来。</p>';
    } else if (a.Type === "image") {      h += '<label class="yyq-field"><span>画面描述（提示词）</span>' +
        '<textarea rows="2" oninput="yyTSetParam(' + i + ",'prompt',this.value)\">" + yyTEsc(p.prompt || "") + "</textarea></label>";
      if (a.Type === "image") h += yyTVarChips(i, "prompt");
      h += '<div class="yyq-cfg-grid2">' +
        '<label class="yyq-field"><span>尺寸</span><input value="' + yyTEsc(p.size || "1024x1024") + '" placeholder="1024x1024" oninput="yyTSetParam(' + i + ",'size',this.value)\"></label>" +
        '<label class="yyq-field"><span>模型（留空用默认）</span><input value="' + yyTEsc(p.model || "") + '" placeholder="默认模型" oninput="yyTSetParam(' + i + ",'model',this.value)\"></label>" +
        "</div>" +
        '<label class="yyq-switchrow"><input type="checkbox"' + (p.use_avatar === "true" ? " checked" : "") +
        ' onchange="yyTSetParam(' + i + ",'use_avatar',this.checked?'true':'false')\"><span>用提问者的头像做参考图</span></label>";
    }
    return h + "</div>";
  }

  // 变量 chips：点击插入到对应 textarea 光标处
  function yyTVarChips(i, param) {
    var vars = yyTCfg.vars.length ? yyTCfg.vars : ["{user}", "{qq}", "{group}", "{arg}"];
    var h = '<div class="yyq-cfg-chips yyq-cfg-chips-sm">';
    vars.forEach(function (v) {
      h += '<button type="button" class="yyq-chip" onclick="yyTInsertVar(' + i + ",'" + param + "','" + yyTEsc(v) + "')\">" + yyTEsc(v) + "</button>";
    });
    return h + "</div>";
  }

  // ---------- 触发词：表单交互 ----------

  function yyTField(key, val) {
    if (!yyTCfg.draft) return;
    if (key === "Patterns") {
      yyTCfg.draft.Patterns = String(val == null ? "" : val).split("\n")
        .map(function (s) { return s.trim(); })
        .filter(function (s) { return s.length; });
      if (yyTCfg.draft.Type === "regex") yyTRegexCheck();
    } else if (key === "Cooldown" || key === "Daily") {
      yyTCfg.draft[key] = Math.max(0, parseInt(val || "0", 10) || 0);
    } else if (key === "min_role") {
      // 后端字段名就是 min_role（yyQQTrigger.MinRole），别写成 MinRole——
      // encoding/json 匹配 json tag 时不做下划线归一化。
      yyTCfg.draft.min_role = Math.max(0, Math.min(2, parseInt(val || "0", 10) || 0));
    } else {
      yyTCfg.draft[key] = val;
    }
    // 输入过程不整块重渲染，保住焦点；结构变化才重渲。
  }
  function yyTRegexCheck() {
    yyTCfg.regexErr = "";
    var pats = (yyTCfg.draft && yyTCfg.draft.Patterns) || [];
    for (var i = 0; i < pats.length; i++) {
      try {
        new RegExp(pats[i]);
      } catch (e) {
        yyTCfg.regexErr = "第 " + (i + 1) + " 行不是有效的正则";
        return;
      }
    }
  }
  function yyTFoldToggle(el) {
    yyTCfg.open = !!el.open;
    if (el.open && !yyTCfg.loaded && !yyTCfg.loading) yyTLoad();
  }
  function yyTSetType(tp) {
    if (!yyTCfg.draft) return;
    yyTCfg.draft.Type = tp;
    if (tp !== "regex") yyTCfg.regexErr = "";
    yyTRender();
    yyTScrollTo();
  }
  function yyTToggleGroup(gid, el) {
    if (!yyTCfg.draft) return;
    var g = yyTCfg.draft.Groups || (yyTCfg.draft.Groups = []);
    var i = g.indexOf(String(gid));
    if (el.checked) {
      if (i < 0) g.push(String(gid));
    } else if (i >= 0) {
      g.splice(i, 1);
    }
    // A2：多选没有选中反馈是用户明确抱怨的点，勾上必须立刻看得出。
    var chip = el.parentNode;
    if (chip && chip.classList) {
      if (el.checked) chip.classList.add("is-picked");
      else chip.classList.remove("is-picked");
      chip.setAttribute("data-picked", el.checked ? "1" : "0");
      chip.setAttribute("aria-pressed", el.checked ? "true" : "false");
    }
  }
  function yyTSetParam(i, key, val) {
    if (!yyTCfg.draft || !yyTCfg.draft.Actions[i]) return;
    var a = yyTCfg.draft.Actions[i];
    a.Params = a.Params || {};
    a.Params[key] = val;
  }
  function yyTSetActionType(i, tp) {
    if (!yyTCfg.draft || !yyTCfg.draft.Actions[i]) return;
    var a = yyTCfg.draft.Actions[i];
    a.Type = tp;
    if (tp === "reply" || tp === "relay") a.Params = { text: "" };
    else if (tp === "like") a.Params = { times: "1" };
    else if (tp === "emoji") a.Params = { emoji_id: "" };
    else if (tp === "command") a.Params = { input: "" };
    else if (tp === "capability") {
      // 默认选中后端元数据里的第一个能力，别留下空 kind 让执行端报「未知能力」
      var km = yyTCapParam("kind");
      var os = (km && km.options) || [];
      a.Params = { kind: os.length ? os[0].value : "", arg: "" };
    }
    else if (tp === "image") a.Params = { prompt: "", size: "1024x1024", model: "", use_avatar: "false" };
    yyTRender();
    yyTScrollTo();
  }
  function yyTAddAction() {
    if (!yyTCfg.draft) return;
    yyTCfg.draft.Actions = yyTCfg.draft.Actions || [];
    yyTCfg.draft.Actions.push({ Type: "reply", Params: { text: "" } });
    yyTRender();
    yyTScrollTo();
  }
  function yyTDelAction(i) {
    if (!yyTCfg.draft || !yyTCfg.draft.Actions) return;
    yyTCfg.draft.Actions.splice(i, 1);
    yyTRender();
    yyTScrollTo();
  }
  function yyTInsertVar(i, param, v) {
    var ta = document.querySelector(".yyq-cfg-action[data-i='" + i + "'] textarea");
    if (!ta) return;
    var s = ta.selectionStart || 0;
    var e = ta.selectionEnd || 0;
    ta.value = ta.value.slice(0, s) + v + ta.value.slice(e);
    yyTSetParam(i, param, ta.value);
    var pos = s + v.length;
    ta.focus();
    try { ta.setSelectionRange(pos, pos); } catch (err) { /* 旧内核忽略 */ }
  }

  // ---------- 触发词：保存 / 开关 / 删除 ----------

  function yyTSave() {
    if (!yyTCfg.draft) return;
    var d = yyTCfg.draft;
    yyTCfg.err = "";
    if (!String(d.Name || "").trim()) { yyTCfg.err = "先填一个规则名称"; yyTRender(); return; }
    if (d.Type !== "at_bot" && d.Type !== "any" && !(d.Patterns || []).length) {
      yyTCfg.err = "至少填一个触发词"; yyTRender(); return;
    }
    if (!(d.Actions || []).length) { yyTCfg.err = "至少添加一个动作"; yyTRender(); return; }
    if (d.Type === "regex") {
      yyTRegexCheck();
      if (yyTCfg.regexErr) { yyTRender(); yyTScrollTo(); return; }
    }
    var rule = {
      ID: d.ID || "", Enabled: d.Enabled !== false, Name: String(d.Name).trim(),
      Type: d.Type, Patterns: d.Patterns || [], Groups: d.Groups || [],
      Cooldown: d.Cooldown || 0, Daily: d.Daily || 0,
      min_role: Math.max(0, Math.min(2, parseInt(d.min_role || 0, 10) || 0)),
      Actions: (d.Actions || []).map(function (a) {
        var params = a.Params || {};
        // capability 没选过能力时兜底填第一个选项，别把空 kind 存进去
        if (a.Type === "capability" && !String(params.kind || "").trim()) {
          var km = yyTCapParam("kind");
          var os = (km && km.options) || [];
          if (os.length) params = Object.assign({}, params, { kind: os[0].value });
        }
        return { Type: a.Type, Params: params };
      })
    };
    var list = (yyTCfg.rules || []).slice();
    var hit = 0;
    for (var i = 0; i < list.length; i++) {
      if (rule.ID && (list[i].ID || list[i].id) === rule.ID) { list[i] = rule; hit = 1; break; }
    }
    if (!hit) {
      rule.ID = "t" + Date.now().toString(36);
      list.push(rule);
    }
    yyTSaveList(list, function () {
      yyTCfg.edit = "";
      yyTCfg.draft = null;
      yyTCfg.saved = 1;
      setTimeout(function () { yyTCfg.saved = 0; yyTRender(); }, 1800);
    });
  }
  function yyTToggle(id, el) {
    var list = (yyTCfg.rules || []).map(function (x) {
      var y = {};
      for (var k in x) y[k] = x[k];
      if ((x.ID || x.id) === id) y.Enabled = !!el.checked;
      return y;
    });
    yyTSaveList(list);
  }
  // 规则表上的「谁能触发」下拉：改完立刻整表保存（与能力开关同一套即时生效逻辑）。
  function yyTSetRuleRole(id, el) {
    var role = Math.max(0, Math.min(2, parseInt(el.value || "0", 10) || 0));
    var list = (yyTCfg.rules || []).map(function (x) {
      var y = {};
      for (var k in x) y[k] = x[k];
      if ((x.ID || x.id) === id) y.min_role = role;
      return y;
    });
    yyTCfg.rules = list;
    yyTSaveList(list);
  }
  function yyTDel() {
    if (!yyTCfg.draft) return;
    var id = yyTCfg.draft.ID;
    if (!id) { yyTCancel(); return; }
    if (!window.confirm("删除这条规则？删除后立即生效。")) return;
    var list = (yyTCfg.rules || []).filter(function (x) { return (x.ID || x.id) !== id; });
    yyTCancel();
    yyTSaveList(list);
  }
  function yyTSaveList(list, done) {
    yyTCfg.busy = 1;
    yyTRender();
    A("POST", "/api/qq/triggers", { triggers: list }).then(function (r) {
      yyTCfg.busy = 0;
      if (!r || r.ok === false) {
        yyTCfg.err = (r && r.error) || "保存失败";
        yyTRender();
        return;
      }
      yyTCfg.err = "";
      yyTLoad();
      if (done) done();
    })["catch"](function () {
      yyTCfg.busy = 0;
      yyTCfg.err = "保存失败：网络错误";
      yyTRender();
    });
  }
  // 局部重渲染：替换整个折叠块但保留展开状态（输入中不调用，保焦点）。
  // 指纹判重：序列化后的整块 HTML 与上次写进 DOM 的完全一样就不碰 DOM。
  // 以前每次都 replaceChild 整块，块会「消失再出现」跳一下。
  var yyTBlockFp = "";
  function yyTRender() {
    var wrap = document.getElementById("yyq-triggers");
    if (!wrap || !wrap.parentNode) return;
    var wasOpen = wrap.hasAttribute("open");
    var html = yyTrigBlock();
    if (html === yyTBlockFp) return;
    yyTBlockFp = html;
    var tmp = document.createElement("div");
    tmp.innerHTML = html;
    var fresh = tmp.firstChild;
    if (fresh && wasOpen) fresh.setAttribute("open", "");
    if (fresh) wrap.parentNode.replaceChild(fresh, wrap);
    // A1：自己重绘会把焦点丢掉，focusout 不一定触发；补一次判定，别让标记长挂。
    var ae = document.activeElement;
    if (!yyTrigHoldIn(ae)) {
      if (yyEditHoldTimer) clearTimeout(yyEditHoldTimer);
      yyEditHoldTimer = setTimeout(function () {
        yyEditHoldTimer = null;
        window.__yyQQEditing = false;
      }, 2000);
    }
  }

  // ---------- 生图配置 ----------

  function yyImgBlock() {
    var c = yyICfg.cfg;
    var sum = yyICfg.loading ? "读取中…" : (yyICfg.err ? "读取失败" : (c && c.enabled
      ? ((c.provider === "cloudflare" ? "Cloudflare · " : "OpenAI 兼容 · ") + (c.model || "默认模型"))
      : "未开启"));
    return '<section class="yyq-sec yyq-sec-wide" id="yyq-image">' +
      '<div class="yyq-sec-head"><h4>生图配置</h4><small>' + yyTEsc(sum) + "</small></div>" +
      '<div id="yyq-img-body">' + yyImgBody() + "</div>" +
      "</section>";
  }

  function yyImgBody() {
    if (yyICfg.loading) {
      var sk = "";
      for (var i = 0; i < 5; i++) sk += '<div class="yyq-skel"></div>';
      return sk;
    }
    if (yyICfg.err) {
      return '<p class="yyq-inline-err"><span class="yyq-inline-err-ico">' + yyTIcon("ban") + "</span>" +
        yyTEsc(yyICfg.err) + '<button type="button" class="yyq-linkbtn" onclick="yyILoad()">重试</button></p>';
    }
    var c = yyICfg.cfg || {};
    var h = '<div class="yyq-cfg-form yyq-cfg-form-flat">';
    h += '<label class="yyq-switchrow"><input type="checkbox"' + (c.enabled ? " checked" : "") +
      ' onchange="yyIField(\'enabled\',this.checked)"><span>开启生图能力</span></label>';
    h += '<div class="yyq-field"><span>服务提供方</span><div class="yyq-seg yyq-seg-wide">';
    (yyICfg.providers.length ? yyICfg.providers : [{ id: "openai" }, { id: "cloudflare" }]).forEach(function (p) {
      h += '<button type="button" class="' + (c.provider === p.id ? "is-on" : "") +
        '" onclick="yyIField(\'provider\',\'' + yyTEsc(p.id) + "')\">" + yyTEsc(p.label || p.id) + "</button>";
    });
    h += "</div></div>";
    var models = (c.models || []).slice();
    h += '<div class="yyq-field"><span>可选模型（点 × 移除，可添加）</span><div class="yyq-cfg-chips">';
    if (!models.length) h += '<span class="yyq-cfg-note">还没有模型，在下面添加</span>';
    models.forEach(function (m, i) {
      h += '<span class="yyq-chip yyq-chip-del">' + yyTEsc(m) +
        '<button type="button" onclick="yyIModelDel(' + i + ')" aria-label="移除">×</button></span>';
    });
    h += '</div><div class="yyq-cfg-inline"><input id="yyq-img-newmodel" placeholder="模型 ID，如 stable-diffusion-xl" ' +
      'onkeydown="if(event.key===\'Enter\'){event.preventDefault();yyIModelAdd();}">' +
      '<button type="button" class="btn-s" onclick="yyIModelAdd()">添加</button></div></div>';
    h += '<label class="yyq-field"><span>默认模型（留空用上面第一个）</span>' +
      '<input id="yyq-img-model" value="' + yyTEsc(c.model || "") + '" list="yyq-img-models" ' +
      'placeholder="默认模型" oninput="yyIField(\'model\',this.value)">' +
      '<datalist id="yyq-img-models">';
    models.forEach(function (m) { h += "<option value=\"" + yyTEsc(m) + "\"></option>"; });
    h += "</datalist></label>";
    if (c.provider !== "cloudflare") {
      h += '<label class="yyq-field"><span>API 地址（BaseURL）</span>' +
        '<input id="yyq-img-base" value="' + yyTEsc(c.base_url || "") + '" placeholder="https://api.siliconflow.cn/v1" ' +
        'oninput="yyIField(\'base_url\',this.value)"></label>';
    } else {
      h += '<label class="yyq-field"><span>Cloudflare Account ID</span>' +
        '<input id="yyq-img-acc" value="' + yyTEsc(c.account_id || "") + '" placeholder="32 位账号 ID" ' +
        'oninput="yyIField(\'account_id\',this.value)"></label>';
    }
    h += '<label class="yyq-field"><span>API Key</span>' +
      '<input id="yyq-img-key" type="password" value="' + yyTEsc(yyICfg.newKey || "") + '" ' +
      'placeholder="' + (c.has_key ? "已保存，留空则不修改" : "填写 API Key") + '" ' +
      'oninput="yyICfg.newKey=this.value"></label>';
    h += '<div class="yyq-cfg-grid2">' +
      '<label class="yyq-field"><span>图片尺寸</span><input id="yyq-img-size" value="' + yyTEsc(c.size || "1024x1024") +
      '" placeholder="1024x1024" oninput="yyIField(\'size\',this.value)"></label>' +
      '<label class="yyq-field"><span>步数 Steps（可留空）</span><input id="yyq-img-steps" type="number" min="0" value="' +
      yyTEsc(c.steps == null ? "" : String(c.steps)) + '" oninput="yyIField(\'steps\',this.value)"></label>' +
      "</div>";
    h += '<label class="yyq-switchrow"><input type="checkbox"' + (c.use_avatar ? " checked" : "") +
      ' onchange="yyIField(\'use_avatar\',this.checked)"><span>默认用提问者的头像做参考图</span></label>';
    h += '<div class="yyq-cfg-grid2">' +
      '<label class="yyq-field"><span>提示词前缀（每次生图自动加）</span>' +
      '<textarea id="yyq-img-pre" rows="2" oninput="yyIField(\'prompt_prefix\',this.value)">' + yyTEsc(c.prompt_prefix || "") + "</textarea></label>" +
      '<label class="yyq-field"><span>提示词后缀</span>' +
      '<textarea id="yyq-img-suf" rows="2" oninput="yyIField(\'prompt_suffix\',this.value)">' + yyTEsc(c.prompt_suffix || "") + "</textarea></label>" +
      "</div>";
    h += '<label class="yyq-switchrow"><input type="checkbox"' + (c.reply_on_start ? " checked" : "") +
      ' onchange="yyIField(\'reply_on_start\',this.checked)"><span>开始画时先回一句（队列人多时有用）</span></label>';
    h += '<div class="yyq-fold-actions">' +
      '<button type="button" class="btn" data-busy="' + (yyICfg.busy ? 1 : 0) + '" onclick="yyISave()">' +
      (yyICfg.busy ? "保存中…" : "保存生图配置") + "</button>" +
      (yyICfg.saved ? '<span class="yyq-cfg-saved">已保存</span>' : "") +
      '<span class="yyq-cfg-toolbar-hint">Key 只存服务器，不明文回显</span>' +
      "</div></div>";
    return h;
  }

  // ---------- 生图配置：数据与保存 ----------

  function yyILoad() {
    yyICfg.loading = 1;
    yyICfg.err = "";
    A("GET", "/api/qq/image").then(function (r) {
      yyICfg.loading = 0;
      if (!r || r.ok === false) {
        yyICfg.err = (r && r.error) || "读取生图配置失败";
        yyIRender();
        return;
      }
      yyICfg.cfg = r.config || {};
      yyICfg.providers = r.providers || [];
      yyICfg.newKey = "";
      yyICfg.loaded = 1;
      yyIRender();
    })["catch"](function () {
      yyICfg.loading = 0;
      yyICfg.err = "读取生图配置失败";
      yyIRender();
    });
  }
  function yyIFoldToggle(el) {
    yyICfg.open = !!el.open;
    if (el.open && !yyICfg.loaded && !yyICfg.loading) yyILoad();
  }
  // 文本类只更新状态不重渲（保焦点）；开关/提供方切换才重渲。
  function yyIField(key, val) {
    if (!yyICfg.cfg) return;
    yyICfg.cfg[key] = val;
    if (key === "enabled" || key === "use_avatar" || key === "reply_on_start" || key === "provider") yyIRender();
  }
  function yyIModelAdd() {
    if (!yyICfg.cfg) return;
    var el = document.getElementById("yyq-img-newmodel");
    var v = el ? String(el.value || "").trim() : "";
    if (!v) return;
    var ms = (yyICfg.cfg.models || []).slice();
    if (ms.indexOf(v) < 0) ms.push(v);
    yyICfg.cfg.models = ms;
    yyIRender();
  }
  function yyIModelDel(i) {
    if (!yyICfg.cfg || !yyICfg.cfg.models) return;
    yyICfg.cfg.models.splice(i, 1);
    yyIRender();
  }
  function yyISave() {
    if (!yyICfg.cfg) return;
    var c = yyICfg.cfg;
    yyICfg.busy = 1;
    yyIRender();
    var patch = {
      enabled: !!c.enabled, provider: c.provider || "openai",
      model: c.model || "", base_url: c.base_url || "", account_id: c.account_id || "",
      size: c.size || "", steps: c.steps, use_avatar: !!c.use_avatar,
      prompt_prefix: c.prompt_prefix || "", prompt_suffix: c.prompt_suffix || "",
      reply_on_start: !!c.reply_on_start, models: (c.models || []).slice()
    };
    if (yyICfg.newKey) patch.api_key = yyICfg.newKey;
    A("POST", "/api/qq/image", patch).then(function (r) {
      yyICfg.busy = 0;
      if (!r || r.ok === false) {
        yyICfg.err = (r && r.error) || "保存失败";
        yyIRender();
        return;
      }
      yyICfg.newKey = "";
      yyICfg.saved = 1;
      yyIRender();
      setTimeout(function () { yyICfg.saved = 0; yyIRender(); }, 1800);
    })["catch"](function () {
      yyICfg.busy = 0;
      yyICfg.err = "保存失败：网络错误";
      yyIRender();
    });
  }
  // 生图配置同理：整块 HTML 没变就不重写，避免「消失再出现」。
  var yyIBlockFp = "";
  function yyIRender() {
    var wrap = document.getElementById("yyq-image");
    if (!wrap || !wrap.parentNode) return;
    var wasOpen = wrap.hasAttribute("open");
    var html = yyImgBlock();
    if (html === yyIBlockFp) return;
    yyIBlockFp = html;
    var tmp = document.createElement("div");
    tmp.innerHTML = html;
    var fresh = tmp.firstChild;
    if (fresh && wasOpen) fresh.setAttribute("open", "");
    if (fresh) wrap.parentNode.replaceChild(fresh, wrap);
  }

  // ---------- 群→游戏 转发（四模式：关闭/触发词/全自动/定时打包） ----------
  //
  // 用户原话：「什么特定触发词，或者它能不能自动触发、自动转发开启了之后，
  // 它某一时间自动转发，这你都不想一下吗？」所以四档并存，谁也不替代谁，
  // 每一档后面的参数都是可编辑的输入框（不是写死的常量）。
  // 接口：GET/POST /api/qq/relay，改动即时保存（onchange/oninput 防抖）。

  var yyRCfg = {
    loading: 1, started: 0, loaded: 0, err: "", cfg: null, open: 0, saved: 0, busy: 0,
    kwText: "", timer: null
  };

  var yyRelayModes = [
    { id: "off", label: "关闭", tip: "不转发。默认就是这一档。" },
    { id: "keyword", label: "触发词", tip: "群里消息含任一触发词才转这一条。" },
    { id: "auto", label: "全自动", tip: "群里每条消息都转进游戏，最热闹也最吵。" },
    { id: "interval", label: "定时打包", tip: "每 N 分钟把这段时间的新消息按时间合并转一次。" }
  ];

  function yyRModeLabel(id) {
    for (var i = 0; i < yyRelayModes.length; i++) {
      if (yyRelayModes[i].id === id) return yyRelayModes[i].label;
    }
    return "关闭";
  }

  function yyRelayBlock() {
    var c = yyRCfg.cfg;
    var sum = yyRCfg.loading ? "读取中…" : (yyRCfg.err ? "读取失败" : (c
      ? (yyRModeLabel(c.mode) + (c.mode === "interval"
        ? " · 每 " + (c.interval_minutes || 10) + " 分钟 / 上限 " + (c.max_messages || 20) + " 条"
        : ""))
      : "未读取"));
    return '<section class="yyq-sec yyq-sec-wide" id="yyq-relay">' +
      '<div class="yyq-sec-head"><h4>群 → 游戏 转发</h4><small>' + yyTEsc(sum) + "</small></div>" +
      '<div id="yyq-relay-body">' + yyRelayBody() + "</div>" +
      "</section>";
  }

  function yyRelayBody() {
    if (yyRCfg.loading) {
      var sk = "";
      for (var i = 0; i < 3; i++) sk += '<div class="yyq-skel"></div>';
      return sk;
    }
    if (yyRCfg.err) {
      return '<p class="yyq-inline-err"><span class="yyq-inline-err-ico">' + yyTIcon("ban") + "</span>" +
        yyTEsc(yyRCfg.err) + '<button type="button" class="yyq-linkbtn" onclick="yyRLoad()">重试</button></p>';
    }
    var c = yyRCfg.cfg || {};
    var mode = c.mode || "off";
    var h = '<div class="yyq-cfg-form yyq-cfg-form-flat">';
    h += '<div class="yyq-field"><span>转发模式</span><div class="yyq-seg yyq-seg-wide">';
    yyRelayModes.forEach(function (m) {
      h += "<button type=\"button\" class=\"" + (mode === m.id ? "is-on" : "") +
        "\" onclick=\"yyRSetMode('" + m.id + "')\" title=\"" + yyTEsc(m.tip) + "\">" +
        yyTEsc(m.label) + "</button>";
    });
    h += "</div>";
    for (var t = 0; t < yyRelayModes.length; t++) {
      if (yyRelayModes[t].id === mode) h += '<span class="yyq-cfg-note">' + yyTEsc(yyRelayModes[t].tip) + "</span>";
    }
    h += "</div>";

    if (mode === "keyword") {
      h += '<label class="yyq-field"><span>触发词（命中任意一个就转这一条）</span>' +
        '<textarea id="yyq-relay-kw" rows="4" placeholder="换行或逗号分隔，例如：&#10;进服&#10;开服了吗,什么时候开" ' +
        'oninput="yyRKeyword(this.value)">' + yyTEsc(yyRCfg.kwText) + "</textarea>" +
        '<span class="yyq-cfg-note">换行、逗号、顿号、分号都算分隔符；不区分大小写。</span></label>';
    }
    if (mode === "interval") {
      h += '<div class="yyq-cfg-grid2">' +
        '<label class="yyq-field"><span>每多少分钟打包一次（1–1440）</span>' +
        '<input id="yyq-relay-min" type="number" min="1" max="1440" step="1" value="' +
        yyTEsc(String(c.interval_minutes || 10)) + '" oninput="yyRNumber(\'interval_minutes\',this.value)"></label>' +
        '<label class="yyq-field"><span>单次最多多少条（1–200）</span>' +
        '<input id="yyq-relay-max" type="number" min="1" max="200" step="1" value="' +
        yyTEsc(String(c.max_messages || 20)) + '" oninput="yyRNumber(\'max_messages\',this.value)"></label>' +
        "</div>" +
        '<span class="yyq-cfg-note">攒下的消息按时间排序，每行「昵称：内容」；超过上限的留到下一轮，不会丢。</span>';
    }
    if (mode === "auto") {
      h += '<span class="yyq-cfg-note">群里每条非指令消息都会转进游戏。群里刷屏时游戏也会刷屏，建议配「定时打包」。</span>';
    }
    if (mode === "off") {
      h += '<span class="yyq-cfg-note">当前不转发。游戏内仍可手动 .通 内容。</span>';
    }

    var queued = c.queued || 0;
    h += '<div class="yyq-cfg-inline"><span class="yyq-cfg-note">' +
      (queued > 0 ? "当前排队 " + queued + " 条待打包" : "暂无排队消息") +
      " ｜ 累计已转 " + (c.sent_total || 0) + " 条</span></div>";
    h += '<div class="yyq-cfg-inline"><span class="yyq-cfg-note">' + yyTEsc(c.hint || "") + "</span></div>";
    h += '<div class="yyq-fold-actions">' +
      (yyRCfg.busy ? '<span class="yyq-cfg-note">保存中…</span>' : "") +
      (yyRCfg.saved ? '<span class="yyq-cfg-saved">已保存</span>' : "") +
      '<span class="yyq-cfg-toolbar-hint">改动即时保存，不用点按钮</span>' +
      "</div></div>";
    return h;
  }

  function yyRLoad() {
    yyRCfg.loading = 1;
    yyRCfg.err = "";
    A("GET", "/api/qq/relay").then(function (r) {
      yyRCfg.loading = 0;
      if (!r || r.ok === false) {
        yyRCfg.err = (r && r.error) || "读取转发配置失败";
        yyRRender();
        return;
      }
      yyRCfg.cfg = r.config || {};
      var kw = yyRCfg.cfg.keywords;
      yyRCfg.kwText = (kw && kw.length) ? kw.join("\n") : "";
      yyRCfg.loaded = 1;
      yyRRender();
    })["catch"](function () {
      yyRCfg.loading = 0;
      yyRCfg.err = "读取转发配置失败";
      yyRRender();
    });
  }

  function yyRFoldToggle(el) {
    yyRCfg.open = !!el.open;
    if (el.open && !yyRCfg.loaded && !yyRCfg.loading) yyRLoad();
  }

  // 模式切换：立刻落库并重渲染（参数框跟着模式换）。
  function yyRSetMode(id) {
    if (!yyRCfg.cfg) return;
    yyRCfg.cfg.mode = id;
    yyRRender();
    yyRSave();
  }

  // 文本/数字：只更新状态，不重渲染（保焦点），700ms 后自动保存。
  function yyRKeyword(v) {
    yyRCfg.kwText = String(v || "");
    yyRSaveSoon();
  }
  function yyRNumber(key, v) {
    if (!yyRCfg.cfg) return;
    var n = parseInt(String(v || "").replace(/[^0-9]/g, ""), 10);
    if (isNaN(n)) return;
    yyRCfg.cfg[key] = n;
    yyRSaveSoon();
  }
  function yyRSaveSoon() {
    if (yyRCfg.timer) clearTimeout(yyRCfg.timer);
    yyRCfg.timer = setTimeout(function () {
      yyRCfg.timer = null;
      yyRSave();
    }, 700);
  }

  function yyRSave() {
    if (!yyRCfg.cfg) return;
    var c = yyRCfg.cfg;
    var im = parseInt(String(c.interval_minutes || 10), 10);
    var mx = parseInt(String(c.max_messages || 20), 10);
    if (isNaN(im) || im < 1) im = 1;
    if (im > 1440) im = 1440;
    if (isNaN(mx) || mx < 1) mx = 1;
    if (mx > 200) mx = 200;
    yyRCfg.busy = 1;
    A("POST", "/api/qq/relay", {
      mode: c.mode || "off",
      keywords: yyRCfg.kwText,
      interval_minutes: im,
      max_messages: mx
    }).then(function (r) {
      yyRCfg.busy = 0;
      if (!r || r.ok === false) {
        yyRCfg.err = (r && r.error) || "保存失败";
        yyRRender();
        return;
      }
      if (r.config) {
        yyRCfg.cfg = r.config;
        // 正在输入触发词时不要把 textarea 的内容按服务端结果重写（会跳光标）。
        var kwEl = document.getElementById("yyq-relay-kw");
        if (kwEl !== document.activeElement) {
          var kw = r.config.keywords;
          yyRCfg.kwText = (kw && kw.length) ? kw.join("\n") : "";
        }
      }
      yyRCfg.saved = 1;
      yyRRender();
      setTimeout(function () { yyRCfg.saved = 0; yyRRender(); }, 1600);
    })["catch"](function () {
      yyRCfg.busy = 0;
      yyRCfg.err = "保存失败：网络错误";
      yyRRender();
    });
  }

  var yyRBlockFp = "";
  // 正在本块里打字时不重写 DOM：自动保存是 700ms 后触发的，
  // 一旦重绘焦点和光标就没了（用户三番五次强调输入框要能正常编辑）。
  function yyRInEdit() {
    var ae = document.activeElement;
    if (!ae || !ae.tagName) return false;
    if (!/^(INPUT|TEXTAREA|SELECT)$/.test(ae.tagName)) return false;
    var el = ae;
    while (el) {
      if (el.id === "yyq-relay") return true;
      el = el.parentNode;
    }
    return false;
  }
  function yyRRender() {
    var wrap = document.getElementById("yyq-relay");
    if (!wrap || !wrap.parentNode) return;
    if (yyRInEdit()) return;
    var wasOpen = wrap.hasAttribute("open");
    var html = yyRelayBlock();
    if (html === yyRBlockFp) return;
    yyRBlockFp = html;
    var tmp = document.createElement("div");
    tmp.innerHTML = html;
    var fresh = tmp.firstChild;
    if (fresh && wasOpen) fresh.setAttribute("open", "");
    if (fresh) wrap.parentNode.replaceChild(fresh, wrap);
  }

  // ---------- 挂载：包装 console 的渲染，注入两个配置折叠块 ----------

  // 注册到首页模块（不再包装 yyQQRenderAll）：
  //   触发词 / 生图 → 🤖 机器人能力；群→游戏转发 → 💬 聊天与账号。
  function yyCfgMount() {
    if (typeof window.yyQQRegisterModule !== "function") return;
    if (window.__yyqCfgRegistered) return;
    window.__yyqCfgRegistered = 1;
    function trigSum() {
      return yyTCfg.loading ? "读取中…" : (yyTCfg.rules.length ? yyTCfg.rules.length + " 条规则" : "还没有规则");
    }
    function imgSum() {
      if (yyICfg.loading) return "读取中…";
      if (yyICfg.err) return "读取失败";
      var c = yyICfg.cfg;
      return c && c.enabled ? (c.model || "已开启") : "未开启";
    }
    function relaySum() {
      if (yyRCfg.loading) return "读取中…";
      if (yyRCfg.err) return "读取失败";
      return yyRCfg.cfg ? yyRModeLabel(yyRCfg.cfg.mode) : "未读取";
    }
    window.yyQQRegisterModule({
      id: "robot", order: 20, minRole: 0, __key: "triggers",
      render: yyTrigBlock, summary: trigSum,
      load: function () { if (!yyTCfg.started) { yyTCfg.started = 1; yyTLoad(); } }
    });
    window.yyQQRegisterModule({
      id: "robot", order: 21, minRole: 0, __key: "image",
      render: yyImgBlock, summary: imgSum,
      load: function () { if (!yyICfg.started) { yyICfg.started = 1; yyILoad(); } }
    });
    window.yyQQRegisterModule({
      id: "chat", order: 20, minRole: 0, __key: "relay",
      render: yyRelayBlock, summary: relaySum,
      load: function () { if (!yyRCfg.started) { yyRCfg.started = 1; yyRLoad(); } }
    });
  }

  // ---------- 导出（inline onclick 需要全局函数） ----------

  var yyCfgExports = {
    yyTLoad: yyTLoad, yyTNew: yyTNew, yyTFromPreset: yyTFromPreset, yyTEdit: yyTEdit,
    yyTCancel: yyTCancel, yyTField: yyTField, yyTRegexCheck: yyTRegexCheck,
    yyTFoldToggle: yyTFoldToggle, yyTSetType: yyTSetType, yyTToggleGroup: yyTToggleGroup,
    yyTSetParam: yyTSetParam, yyTSetActionType: yyTSetActionType, yyTAddAction: yyTAddAction,
    yyTDelAction: yyTDelAction, yyTInsertVar: yyTInsertVar, yyTSave: yyTSave,
    yyTCapArgHint: yyTCapArgHint,
    yyTToggle: yyTToggle, yyTDel: yyTDel, yyTScrollTo: yyTScrollTo, yyTSetRuleRole: yyTSetRuleRole,
    yyILoad: yyILoad, yyIFoldToggle: yyIFoldToggle, yyIField: yyIField,
    yyIModelAdd: yyIModelAdd, yyIModelDel: yyIModelDel, yyISave: yyISave,
    yyRLoad: yyRLoad, yyRFoldToggle: yyRFoldToggle, yyRSetMode: yyRSetMode,
    yyRKeyword: yyRKeyword, yyRNumber: yyRNumber
  };
  for (var ek in yyCfgExports) {
    if (!window[ek]) window[ek] = yyCfgExports[ek];
  }

  // ---------- 启动 ----------

  function yyCfgInit() {
    yyTrigEditGuard();
    yyCfgMount();
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", yyCfgMount);
    }
  }
  yyCfgInit();
})();
