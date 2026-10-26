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
  var yyTActionIcon = { reply: "chat", like: "thumb", emoji: "face", command: "list", relay: "send", image: "image" };

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
    var open = yyTCfg.open ? " open" : "";
    var sum = yyTCfg.loading ? "读取中…" : (yyTCfg.rules.length ? yyTCfg.rules.length + " 条规则生效" : "还没有规则");
    return '<details class="yyq-fold" id="yyq-triggers"' + open + ' ontoggle="yyTFoldToggle(this)">' +
      '<summary><span class="yyq-fold-ico">' + yyTIcon("spark") + '</span>' +
      '<span class="yyq-fold-t">触发词规则</span>' +
      '<span class="yyq-fold-sum">' + yyTEsc(sum) + "</span>" +
      '<span class="yyq-fold-caret">' + yyTIcon("arrow") + "</span></summary>" +
      '<div class="yyq-fold-body" id="yyq-trig-body">' + yyTrigBody() + "</div>" +
      "</details>";
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
        h += '<li class="yyq-cap yyq-cfg-rule" data-on="' + (en ? 1 : 0) + '" onclick="yyTEdit(\'' + yyTEsc(id) + "',event)\">" +
          '<span class="yyq-cap-ico">' + yyTIcon(ico) + "</span>" +
          '<span class="yyq-cap-body"><strong>' + yyTEsc(t.Name || t.name || "未命名") + "</strong>" +
          "<small>" + yyTEsc(yyTRuleSummary(t)) + "</small></span>" +
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
    yyTCfg.draft = { ID: "", Enabled: true, Name: "", Type: "keyword", Patterns: [], Groups: [], Cooldown: 60, Daily: 5, Actions: [{ Type: "reply", Params: { text: "" } }] };
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
      h += '<p class="yyq-cfg-note">' + (d.Type === "at_bot" ? "只在有人 @ 机器人时触发，不需要触发词。" : "对每条消息都尝试匹配，请谨慎搭配动作（AI 动作会大幅增加消耗）。") + "</p>";
    }
    // 生效群
    var groups = yyTGroups();
    if (groups.length) {
      h += '<div class="yyq-field"><span>生效群（不勾选 = 全部群）</span><div class="yyq-cfg-groups">';
      groups.forEach(function (g) {
        var gid = g.id || g.group_id;
        var on = (d.Groups || []).indexOf(String(gid)) >= 0;
        h += '<label class="yyq-cfg-group"><input type="checkbox"' + (on ? " checked" : "") +
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
      { type: "relay", label: "转发到游戏" }, { type: "image", label: "生成图片" }
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
    } else if (a.Type === "image") {
      h += '<label class="yyq-field"><span>画面描述（提示词）</span>' +
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
      Actions: (d.Actions || []).map(function (a) { return { Type: a.Type, Params: a.Params || {} }; })
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
  function yyTRender() {
    var wrap = document.getElementById("yyq-triggers");
    if (!wrap || !wrap.parentNode) return;
    var wasOpen = wrap.hasAttribute("open");
    var tmp = document.createElement("div");
    tmp.innerHTML = yyTrigBlock();
    var fresh = tmp.firstChild;
    if (fresh && wasOpen) fresh.setAttribute("open", "");
    if (fresh) wrap.parentNode.replaceChild(fresh, wrap);
  }

  // ---------- 生图配置 ----------

  function yyImgBlock() {
    var open = yyICfg.open ? " open" : "";
    var c = yyICfg.cfg;
    var sum = yyICfg.loading ? "读取中…" : (yyICfg.err ? "读取失败" : (c && c.enabled
      ? ((c.provider === "cloudflare" ? "Cloudflare · " : "OpenAI 兼容 · ") + (c.model || "默认模型"))
      : "未开启"));
    return '<details class="yyq-fold" id="yyq-image"' + open + ' ontoggle="yyIFoldToggle(this)">' +
      '<summary><span class="yyq-fold-ico">' + yyTIcon("image") + "</span>" +
      '<span class="yyq-fold-t">生图配置</span>' +
      '<span class="yyq-fold-sum">' + yyTEsc(sum) + "</span>" +
      '<span class="yyq-fold-caret">' + yyTIcon("arrow") + "</span></summary>" +
      '<div class="yyq-fold-body" id="yyq-img-body">' + yyImgBody() + "</div>" +
      "</details>";
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
  function yyIRender() {
    var wrap = document.getElementById("yyq-image");
    if (!wrap || !wrap.parentNode) return;
    var wasOpen = wrap.hasAttribute("open");
    var tmp = document.createElement("div");
    tmp.innerHTML = yyImgBlock();
    var fresh = tmp.firstChild;
    if (fresh && wasOpen) fresh.setAttribute("open", "");
    if (fresh) wrap.parentNode.replaceChild(fresh, wrap);
  }

  // ---------- 挂载：包装 console 的渲染，注入两个配置折叠块 ----------

  function yyCfgMount() {
    if (typeof window.yyQQRenderAll !== "function") return;
    if (window.yyQQRenderAll.__yyqCfgWrapped) return;
    var orig = window.yyQQRenderAll;
    var wrapped = function () {
      orig();
      var grid = document.querySelector("#yyq-root .yyq-fold-grid");
      if (!grid) return;
      if (!document.getElementById("yyq-triggers")) grid.insertAdjacentHTML("beforeend", yyTrigBlock());
      if (!document.getElementById("yyq-image")) grid.insertAdjacentHTML("beforeend", yyImgBlock());
      if (!yyTCfg.started) { yyTCfg.started = 1; yyTLoad(); }
      if (!yyICfg.started) { yyICfg.started = 1; yyILoad(); }
    };
    wrapped.__yyqCfgWrapped = 1;
    window.yyQQRenderAll = wrapped;
    // console 可能已渲染过（先于本脚本首绘），立即补一次注入
    if (document.getElementById("yyq-root")) wrapped();
  }

  // ---------- 导出（inline onclick 需要全局函数） ----------

  var yyCfgExports = {
    yyTLoad: yyTLoad, yyTNew: yyTNew, yyTFromPreset: yyTFromPreset, yyTEdit: yyTEdit,
    yyTCancel: yyTCancel, yyTField: yyTField, yyTRegexCheck: yyTRegexCheck,
    yyTFoldToggle: yyTFoldToggle, yyTSetType: yyTSetType, yyTToggleGroup: yyTToggleGroup,
    yyTSetParam: yyTSetParam, yyTSetActionType: yyTSetActionType, yyTAddAction: yyTAddAction,
    yyTDelAction: yyTDelAction, yyTInsertVar: yyTInsertVar, yyTSave: yyTSave,
    yyTToggle: yyTToggle, yyTDel: yyTDel, yyTScrollTo: yyTScrollTo,
    yyILoad: yyILoad, yyIFoldToggle: yyIFoldToggle, yyIField: yyIField,
    yyIModelAdd: yyIModelAdd, yyIModelDel: yyIModelDel, yyISave: yyISave
  };
  for (var ek in yyCfgExports) {
    if (!window[ek]) window[ek] = yyCfgExports[ek];
  }

  // ---------- 启动 ----------

  function yyCfgInit() {
    yyCfgMount();
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", yyCfgMount);
    }
  }
  yyCfgInit();
})();
