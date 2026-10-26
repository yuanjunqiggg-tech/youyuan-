// 互通 · 群规（违禁词与 AI 长期禁令）
// ---------------------------------------------------------------------------
// 用户原话：「一些违禁词啊什么的，或者我不想要的一些词 和我叫AI记住不要干什么
// 不准玩家干什么……」
// 后端把这件事拆成两层：
//   ① 群规规则表（本面板可配）：命中就拦 / 撤 / 禁言，规则写死在磁盘上；
//   ② AI 长期禁令（persona.taboos）：不拦消息，但 AI 会永久记住，重启不丢。
// 两层都必须能在面板上增、改、删——用户原话：「有些东西还不能删掉 比如违规词
// 不能修改」，只读列表等于没做。
//
// 挂载手法与 youyuan_qq_guard.js / youyuan_qq_knowledge.js 相同：包装
// window.yyQQRenderAll，往 #yyq-root 的 .yyq-fold-grid 里 insertAdjacentHTML
// 注入 <details class="yyq-fold" id="yyq-moderation">。console 每次整体重建
// #yyq-root，所以 body() 必须是模块状态的纯函数；render() 双写：更新
// #yym-body 子容器 + 同步 summary 文案。
//
// 接口契约（后端 youyuan_qq_moderation.go）：
//   GET  /api/qq/moderation
//     -> {ok, rules:[{pattern,kind,action,reason,enabled,source,added_at}],
//         taboos:[...], disk_error, note}
//     kind:   substring=子串匹配 | regex=正则
//     action: warn=只AI提醒不拦 | block=直接拦截 | recall=拦截+撤回
//             recall_ban=拦截+撤回+禁言10分钟
//     source: manual=面板加的 | natural=超管自然语言禁令生成
//   POST /api/qq/moderation  body {"rules":[...]} 或 {"taboos":[...]} 或两者都给
//     rules 整表替换（增、删、改都是「读全表 → 改 → 写回」）；
//     taboos 整表替换（写进人格卡 qq-persona.json）。
//     两个字段都是可选的：只传 taboos 不会动规则表，反之亦然。
// ---------------------------------------------------------------------------

(function () {
  "use strict";

  var yyM = {
    loading: 0, loaded: 0, err: "",
    rules: [], taboos: [], note: "", diskErr: "",
    msg: "", bad: 0,
    // "" | "add" | "toggle:i" | "del:i" | "edit:i" | "taboo"
    busy: "",
    open: 0,
    form: { pattern: "", kind: "substring", action: "block", reason: "" },
    edit: -1,                                            // 正在编辑的规则下标
    editForm: { pattern: "", kind: "substring", action: "block", reason: "" },
    tabooNew: "",                                        // 新增长期禁令输入框
    tabooEdit: -1,                                       // 正在编辑的禁令下标
    tabooForm: ""                                        // 正在编辑的禁令内容
  };

  var KIND_LABEL = { substring: "子串匹配", regex: "正则" };
  var ACTION_LABEL = {
    warn: "只AI提醒",
    block: "直接拦截",
    recall: "拦截并撤回",
    recall_ban: "拦截撤回+禁言"
  };
  var ACTION_TIP = {
    warn: "不拦消息，只提醒 AI 这条有问题，AI 回答时会知道",
    block: "消息不会进 AI，也不会转发进游戏",
    recall: "拦截并撤回原消息（需在「机器人能力」里开「撤回消息」）",
    recall_ban: "拦截、撤回并禁言 10 分钟（需在「机器人能力」里开「撤回消息」「禁言」）"
  };
  var SOURCE_LABEL = { manual: "面板", natural: "AI禁令" };

  function esc(v) {
    return typeof yyQQE === "function" ? yyQQE(v == null ? "" : String(v)) : String(v == null ? "" : v);
  }
  function icon(n) {
    return typeof yyQQIcon === "function" ? yyQQIcon(n) : "";
  }

  // ---------- 数据 ----------

  function load() {
    yyM.loading = 1;
    render();
    A("GET", "/api/qq/moderation").then(function (r) {
      yyM.loading = 0;
      yyM.loaded = 1;
      if (!r || !r.ok) {
        yyM.err = (r && r.error) || "读取失败";
        render();
        return;
      }
      yyM.rules = r.rules || [];
      yyM.taboos = r.taboos || [];
      yyM.note = r.note || "";
      yyM.diskErr = r.disk_error || "";
      yyM.err = "";
      render();
    })["catch"](function () {
      yyM.loading = 0;
      yyM.loaded = 1;
      yyM.err = "网络错误";
      render();
    });
  }

  function enabledCount() {
    var n = 0;
    for (var i = 0; i < yyM.rules.length; i++) if (yyM.rules[i].enabled) n++;
    return n;
  }

  function summary() {
    if (yyM.loading) return "读取中…";
    if (!yyM.loaded) return "点开查看";
    return "生效 " + enabledCount() + " 条 · AI 长期禁令 " + yyM.taboos.length + " 条";
  }

  // 写回前统一整形：只发契约里的五个字段，kind/action 白名单防呆
  function payloadOf(list) {
    return (list || yyM.rules).map(function (r) {
      return {
        pattern: String(r.pattern || "").trim(),
        kind: r.kind === "regex" ? "regex" : "substring",
        action: ACTION_LABEL[r.action] ? r.action : "block",
        reason: String(r.reason || "").trim(),
        enabled: !!r.enabled
      };
    });
  }

  // 规则表整表写回。只提交 rules，不动 taboos。
  function writeRules(list) {
    return A("POST", "/api/qq/moderation", { rules: payloadOf(list) }).then(function (r) {
      if (!r || !r.ok) throw new Error((r && r.error) || "保存失败");
      if (r.rules) yyM.rules = r.rules;
      return r;
    });
  }

  // AI 长期禁令整表写回。只提交 taboos，不动规则表。
  function writeTaboos(list) {
    return A("POST", "/api/qq/moderation", { taboos: list }).then(function (r) {
      if (!r || !r.ok) throw new Error((r && r.error) || "保存失败");
      if (r.taboos) yyM.taboos = r.taboos;
      return r;
    });
  }

  // ---------- 渲染（body() 必须是 yyM 状态的纯函数） ----------

  function block() {
    return '<details class="yyq-fold" id="yyq-moderation"' + (yyM.open ? " open" : "") + ' ontoggle="yyMFold(this)">' +
      '<summary><span class="yyq-fold-ico">' + icon("ban") + "</span>" +
      '<span class="yyq-fold-t">群规</span>' +
      '<span class="yyq-fold-sum">' + esc(summary()) + "</span>" +
      '<span class="yyq-fold-caret">' + icon("arrow") + "</span></summary>" +
      '<div class="yyq-fold-body" id="yym-body">' + body() + "</div></details>";
  }

  function body() {
    if (yyM.loading) return '<div class="yyk-skel">正在读取群规…</div>';
    if (yyM.err) {
      return '<p class="yyk-warn">' + esc(yyM.err) + "</p>" +
        '<div class="yyq-fold-actions"><button type="button" class="btn-s" onclick="yyMReload()"' + (yyM.busy ? " disabled" : "") + ">重新读取</button></div>";
    }
    var busy = !!yyM.busy;
    var dis = busy ? " disabled" : "";

    // 操作结果 / 错误提示：红字在表单上方
    var msg = yyM.msg ? '<p class="yyk-msg" data-bad="' + (yyM.bad ? "1" : "0") + '">' + esc(yyM.msg) + "</p>" : "";

    // 规则表
    var table = '<h4 class="yyk-h4">群规规则（' + yyM.rules.length + " 条）</h4>" +
      (yyM.rules.length ? ruleTable(busy) : '<p class="yyk-hint">还没有规则。加一条，比如把骂人的词设成「直接拦截」。</p>');

    // 添加表单（.yyk-add 的 CSS 不覆盖 select，这里用 --phx-* 变量内联防丑）
    var selStyle = "flex:1 1 auto;min-width:0;min-height:40px;padding:0 8px;border:1px solid var(--phx-border-light);border-radius:8px;background:var(--phx-bg-card);color:var(--phx-text);font-size:12px";
    var form = '<div class="yyk-add">' +
      '<input id="yym-new" maxlength="120" placeholder="匹配内容，例如 傻逼 或 你好[啊阿]" value="' + esc(yyM.form.pattern) + '"' + dis + ' oninput="yyMForm(this)">' +
      '<div class="yyk-row">' +
      '<select id="yym-kind" aria-label="匹配类型" style="' + selStyle + '"' + dis + ' onchange="yyMForm(this)">' +
      option("substring", yyM.form.kind, "子串匹配") + option("regex", yyM.form.kind, "正则") +
      "</select>" +
      '<select id="yym-action" aria-label="处置方式" style="' + selStyle + '"' + dis + ' onchange="yyMForm(this)">' +
      option("warn", yyM.form.action, "只AI提醒") + option("block", yyM.form.action, "直接拦截") +
      option("recall", yyM.form.action, "拦截并撤回") + option("recall_ban", yyM.form.action, "拦截撤回+禁言") +
      "</select>" +
      "</div>" +
      '<input id="yym-reason" maxlength="60" placeholder="理由（可空，写给未来的自己）" value="' + esc(yyM.form.reason) + '"' + dis + ' oninput="yyMForm(this)">' +
      '<button type="button" class="btn-s" onclick="yyMAdd(this)"' + dis + ">添加</button>" +
      "</div>";

    // AI 长期禁令（可加、可改、可删）
    var taboos = '<h4 class="yyk-h4">AI 长期禁令（' + yyM.taboos.length + " 条）</h4>" +
      tabooRows(busy) +
      '<div class="yyk-add">' +
        '<input id="yym-taboo-new" maxlength="40" placeholder="加一条，例如 不准提别的服务器" value="' + esc(yyM.tabooNew) + '"' + dis +
        ' oninput="yyMTabooInput(this)" onkeydown="if(event.key===\'Enter\')yyMTabooAdd()">' +
        '<button type="button" class="btn-s" onclick="yyMTabooAdd()"' + dis + ">添加</button>" +
      "</div>" +
      '<p class="yyk-hint">这层不拦消息，是让 AI 自己记住并劝阻。超管在群里说「以后不准 玩家XXX」也会自动加到这里；说「取消禁令 XXX」解除。</p>';

    // 底部说明
    var notes = '<p class="yyk-hint">命中「直接拦截 / 拦截并撤回 / 拦截撤回+禁言」的消息不会进 AI，也不会转发进游戏；「只AI提醒」不拦消息，只让 AI 回答时知道这条有问题。</p>' +
      '<p class="yyk-hint">撤回、禁言要先在「机器人能力」里打开「撤回消息」「禁言」开关；没开时自动降级成只拦不撤。</p>' +
      '<p class="yyk-hint">匹配前会做小写、半角、去零宽字符处理，「ＳＢ」和「sb」都能中。超管和管理员发的消息不拦。</p>';
    if (yyM.note) notes += '<p class="yyk-hint">' + esc(yyM.note) + "</p>";
    var disk = yyM.diskErr ? '<p class="yyk-warn">规则落盘失败：' + esc(yyM.diskErr) + "。改完当时有效，重启可能丢，先检查磁盘权限。</p>" : "";

    return msg + table + form + disk + taboos + notes;
  }

  function option(v, cur, label) {
    return '<option value="' + v + '"' + (v === cur ? " selected" : "") + ">" + esc(label) + "</option>";
  }

  function tabooText(t) {
    if (t == null) return "";
    if (typeof t === "string") return t;
    return String(t.text || t.pattern || t.word || t.content || t.target || t.rule || "");
  }

  // AI 长期禁令列表：每条都带「改」和「×」。正在编辑的那条换成输入框。
  function tabooRows(busy) {
    if (!yyM.taboos.length) return '<p class="yyk-hint">还没有长期禁令。</p>';
    var dis = busy ? " disabled" : "";
    var btn = "border:1px solid var(--phx-border-light);border-radius:8px;background:var(--phx-bg-card);color:var(--phx-text-secondary);font-size:11px;min-width:32px;min-height:32px;padding:0 8px;cursor:pointer;margin-left:6px";
    return '<ul class="yyk-docs">' + yyM.taboos.map(function (t, i) {
      if (yyM.tabooEdit === i) {
        return '<li data-edit="1" style="display:flex;gap:6px;align-items:center">' +
          '<input id="yym-taboo-edit" maxlength="40" value="' + esc(yyM.tabooForm) + '"' + dis +
          ' style="flex:1 1 auto;min-width:0;min-height:36px;padding:0 8px;border:1px solid var(--phx-border-light);border-radius:8px;background:var(--phx-bg-card);color:var(--phx-text);font-size:12px"' +
          ' oninput="yyMTabooEditInput(this)" onkeydown="if(event.key===\'Enter\')yyMTabooSaveEdit()">' +
          '<button type="button" class="btn-s" onclick="yyMTabooSaveEdit()"' + dis + ">保存</button>" +
          '<button type="button" class="btn-s" onclick="yyMTabooCancel()"' + dis + ">取消</button></li>";
      }
      return '<li data-taboo="' + i + '"><strong>' + esc(tabooText(t)) + "</strong>" +
        '<button type="button" aria-label="编辑这条禁令" style="' + btn + '"' + dis + ' onclick="yyMTabooEdit(' + i + ')">改</button>' +
        '<button type="button" aria-label="删除这条禁令" style="' + btn + ';color:var(--phx-error,#d25050)"' + dis + ' onclick="yyMTabooDel(' + i + ')">×</button></li>';
    }).join("") + "</ul>";
  }

  function ruleTable(busy) {
    var dis = busy ? ' disabled' : "";
    var rows = yyM.rules.map(function (r, i) {
      if (yyM.edit === i) return ruleEditRow(dis);
      var rowBusy = yyM.busy === "toggle:" + i || yyM.busy === "del:" + i;
      var tone = ACTION_TONE(r.action);
      var src = r.source === "natural";
      return '<tr data-i="' + i + '"' + (rowBusy ? ' data-busy="1" style="opacity:.6"' : "") + ">" +
        td('<label class="yyq-sw"' + (rowBusy ? ' data-busy="1"' : "") + ">" +
          '<input type="checkbox" aria-label="启用"' + (r.enabled ? " checked" : "") + dis +
          ' onchange="yyMToggle(' + i + ',this.checked)"><span></span></label>', "width:52px") +
        td('<code style="font:11px/1.5 ui-monospace,Consolas,monospace;color:var(--phx-text);overflow-wrap:anywhere">' + esc(r.pattern) + "</code>" +
          '<span style="display:inline-block;margin-left:4px;padding:1px 6px;border-radius:999px;background:var(--phx-bg-secondary);color:var(--phx-text-secondary);font-size:9px;white-space:nowrap">' + esc(KIND_LABEL[r.kind] || r.kind || "子串匹配") + "</span>") +
        td('<span style="color:' + tone[0] + (tone[1] ? ";font-weight:600" : "") + ';white-space:nowrap" title="' + esc(ACTION_TIP[r.action] || "") + '">' + esc(ACTION_LABEL[r.action] || r.action) + "</span>") +
        td(r.reason ? esc(r.reason) : '<span style="color:var(--phx-text-disabled)">—</span>') +
        td('<span style="display:inline-block;padding:1px 6px;border-radius:999px;font-size:9px;white-space:nowrap;' +
          (src ? "background:var(--phx-primary-bg);color:var(--phx-primary)" : "background:var(--phx-bg-secondary);color:var(--phx-text-secondary)") +
          '"' + (src ? ' title="超管在群里用自然语言生成的禁令"' : ' title="在本面板添加"') + ">" + esc(SOURCE_LABEL[r.source] || "面板") + "</span>") +
        td('<button type="button" aria-label="编辑" style="border:1px solid var(--phx-border-light);border-radius:8px;background:var(--phx-bg-card);color:var(--phx-text-secondary);font-size:11px;min-width:32px;min-height:32px;padding:0 6px;cursor:pointer"' + dis +
          ' onclick="yyMEdit(' + i + ')">改</button>' +
          '<button type="button" class="yyk-del" aria-label="删除"' + dis + ' style="margin-left:4px" onclick="yyMDel(' + i + ',this)">×</button>', "width:80px") +
        "</tr>";
    }).join("");
    return '<div style="overflow-x:auto;-webkit-overflow-scrolling:touch;margin:8px 0 0">' +
      '<table style="width:100%;min-width:560px;border-collapse:collapse;font-size:11px">' +
      "<thead><tr>" +
      '<th style="width:52px;text-align:left;font-weight:600;font-size:10px;color:var(--phx-text-secondary);padding:4px 8px;border-bottom:1px solid var(--phx-border-light);white-space:nowrap">启用</th>' +
      '<th style="text-align:left;font-weight:600;font-size:10px;color:var(--phx-text-secondary);padding:4px 8px;border-bottom:1px solid var(--phx-border-light);white-space:nowrap">匹配内容</th>' +
      '<th style="text-align:left;font-weight:600;font-size:10px;color:var(--phx-text-secondary);padding:4px 8px;border-bottom:1px solid var(--phx-border-light);white-space:nowrap">处置</th>' +
      '<th style="text-align:left;font-weight:600;font-size:10px;color:var(--phx-text-secondary);padding:4px 8px;border-bottom:1px solid var(--phx-border-light);white-space:nowrap">理由</th>' +
      '<th style="text-align:left;font-weight:600;font-size:10px;color:var(--phx-text-secondary);padding:4px 8px;border-bottom:1px solid var(--phx-border-light);white-space:nowrap">来源</th>' +
      '<th style="width:80px"></th>' +
      "</tr></thead><tbody>" + rows + "</tbody></table></div>";
  }

  // 编辑态的行：整行铺开成一行输入，改完「保存」才提交，取消就回原样。
  function ruleEditRow(dis) {
    var f = yyM.editForm;
    var inp = "min-height:36px;padding:0 8px;border:1px solid var(--phx-border-light);border-radius:8px;background:var(--phx-bg-card);color:var(--phx-text);font-size:12px";
    var sel = "min-height:36px;padding:0 6px;border:1px solid var(--phx-border-light);border-radius:8px;background:var(--phx-bg-card);color:var(--phx-text);font-size:12px";
    return '<tr data-edit="1"><td colspan="6" style="padding:8px;border-bottom:1px solid var(--phx-border-light)">' +
      '<div style="display:flex;flex-wrap:wrap;gap:6px;align-items:center">' +
        '<input id="yym-e-pattern" maxlength="120" placeholder="匹配内容" value="' + esc(f.pattern) + '"' +
          ' style="' + inp + ';flex:2 1 180px;min-width:140px"' + dis + ' oninput="yyMEditInput(this)"' +
          ' onkeydown="if(event.key===\'Enter\')yyMSaveEdit()">' +
        '<select id="yym-e-kind" aria-label="匹配类型" style="' + sel + '"' + dis + ' onchange="yyMEditInput(this)">' +
          option("substring", f.kind, "子串匹配") + option("regex", f.kind, "正则") + "</select>" +
        '<select id="yym-e-action" aria-label="处置方式" style="' + sel + '"' + dis + ' onchange="yyMEditInput(this)">' +
          option("warn", f.action, "只AI提醒") + option("block", f.action, "直接拦截") +
          option("recall", f.action, "拦截并撤回") + option("recall_ban", f.action, "拦截撤回+禁言") + "</select>" +
        '<input id="yym-e-reason" maxlength="60" placeholder="理由（可空）" value="' + esc(f.reason) + '"' +
          ' style="' + inp + ';flex:1 1 140px;min-width:120px"' + dis + ' oninput="yyMEditInput(this)"' +
          ' onkeydown="if(event.key===\'Enter\')yyMSaveEdit()">' +
        '<button type="button" class="btn-s" onclick="yyMSaveEdit()"' + dis + ">保存</button>" +
        '<button type="button" class="btn-s" onclick="yyMCancelEdit()"' + dis + ">取消</button>" +
      "</div></td></tr>";
  }

  // 单元格统一样式：行分隔线 + 内边距，颜色全部走 --phx-* 变量
  function td(html, w) {
    return '<td style="' + (w ? "width:" + w + ";" : "") +
      "padding:6px 8px;border-bottom:1px solid var(--phx-border-light);color:var(--phx-text);vertical-align:middle\">" +
      html + "</td>";
  }

  // 处置颜色：warn 次要色，block 警示色，recall/recall_ban 错误色
  function ACTION_TONE(a) {
    if (a === "block") return ["var(--phx-warning,#c48424)", false];
    if (a === "recall" || a === "recall_ban") return ["var(--phx-error,#d25050)", true];
    return ["var(--phx-text-secondary)", false];
  }

  function render() {
    var el = document.getElementById("yym-body");
    if (!el) return;
    el.innerHTML = body();
    var sum = document.querySelector("#yyq-moderation .yyq-fold-sum");
    if (sum) sum.textContent = summary();
  }

  // ---------- 交互 ----------

  function flash(text, bad) {
    yyM.msg = text || "";
    yyM.bad = bad ? 1 : 0;
    render();
    if (text) setTimeout(function () {
      if (yyM.msg === text) { yyM.msg = ""; render(); }
    }, 2000);
  }

  function fold(el) {
    yyM.open = el.open ? 1 : 0;
    if (yyM.open && !yyM.loaded && !yyM.loading) load();
  }

  function formSync(el) {
    if (!el || !el.id) return;
    if (el.id === "yym-new") yyM.form.pattern = el.value || "";
    else if (el.id === "yym-kind") yyM.form.kind = el.value || "substring";
    else if (el.id === "yym-action") yyM.form.action = el.value || "block";
    else if (el.id === "yym-reason") yyM.form.reason = el.value || "";
  }

  function fieldVal(id) {
    var el = document.getElementById(id);
    return el && el.value ? String(el.value).trim() : "";
  }

  function toggle(i, on) {
    if (yyM.busy) return;
    var r = yyM.rules[i];
    if (!r) return;
    var prev = r.enabled;
    r.enabled = !!on;
    yyM.busy = "toggle:" + i;
    render();
    writeRules(yyM.rules).then(function () {
      yyM.busy = "";
      flash(r.enabled ? "已启用，立即生效" : "已停用，立即生效");
      load();
    })["catch"](function (e) {
      r.enabled = prev;
      yyM.busy = "";
      flash((e && e.message) || "网络错误", 1);
    });
  }

  function add(btn) {
    if (yyM.busy) return;
    var p = fieldVal("yym-new");
    var k = fieldVal("yym-kind") || "substring";
    var a = fieldVal("yym-action") || "block";
    var reason = fieldVal("yym-reason");
    if (!p) { flash("先填写匹配内容", 1); return; }
    if (k === "regex") {
      try { new RegExp(p); } catch (e) {
        flash("正则写错了：" + ((e && e.message) || e), 1);
        return;
      }
    }
    for (var i = 0; i < yyM.rules.length; i++) {
      if (yyM.rules[i].pattern === p && yyM.rules[i].kind === k) { flash("这条已经在表里了", 1); return; }
    }
    var next = yyM.rules.concat([{ pattern: p, kind: k, action: a, reason: reason, enabled: true, source: "manual" }]);
    yyM.busy = "add";
    render();
    writeRules(next).then(function () {
      yyM.form = { pattern: "", kind: "substring", action: "block", reason: "" };
      yyM.busy = "";
      flash("已添加，立即生效");
      load();
    })["catch"](function (e) {
      yyM.busy = "";
      flash((e && e.message) || "网络错误", 1);
    });
  }

  function del(i, btn) {
    if (yyM.busy) return;
    var r = yyM.rules[i];
    if (!r) return;
    // 一次确认就够，别堆一堆文案。删的是整条规则，删完立即生效。
    if (!window.confirm("删除规则「" + r.pattern + "」？删掉后这条不再拦消息。")) return;
    var next = yyM.rules.filter(function (_, j) { return j !== i; });
    yyM.busy = "del:" + i;
    render();
    writeRules(next).then(function () {
      yyM.busy = "";
      yyM.edit = -1;
      flash("已删除「" + r.pattern + "」");
      load();
    })["catch"](function (e) {
      yyM.busy = "";
      flash((e && e.message) || "网络错误", 1);
    });
  }

  // ---- 规则行内编辑 ----

  function editStart(i) {
    if (yyM.busy) return;
    var r = yyM.rules[i];
    if (!r) return;
    yyM.edit = i;
    yyM.editForm = {
      pattern: String(r.pattern || ""),
      kind: r.kind === "regex" ? "regex" : "substring",
      action: ACTION_LABEL[r.action] ? r.action : "block",
      reason: String(r.reason || "")
    };
    render();
    var el = document.getElementById("yym-e-pattern");
    if (el) { el.focus(); try { el.setSelectionRange(el.value.length, el.value.length); } catch (_) {} }
  }

  function editInput(el) {
    if (!el || !el.id) return;
    if (el.id === "yym-e-pattern") yyM.editForm.pattern = el.value || "";
    else if (el.id === "yym-e-kind") yyM.editForm.kind = el.value || "substring";
    else if (el.id === "yym-e-action") yyM.editForm.action = el.value || "block";
    else if (el.id === "yym-e-reason") yyM.editForm.reason = el.value || "";
  }

  function editCancel() {
    yyM.edit = -1;
    render();
  }

  function editSave() {
    if (yyM.busy) return;
    var i = yyM.edit;
    var r = yyM.rules[i];
    if (!r) { yyM.edit = -1; render(); return; }
    var f = yyM.editForm;
    var p = String(f.pattern || "").trim();
    var k = f.kind === "regex" ? "regex" : "substring";
    if (!p) { flash("匹配内容不能为空", 1); return; }
    if (k === "regex") {
      try { new RegExp(p); } catch (e) {
        flash("正则写错了：" + ((e && e.message) || e), 1);
        return;
      }
    }
    for (var j = 0; j < yyM.rules.length; j++) {
      if (j === i) continue;
      if (String(yyM.rules[j].pattern || "") === p && (yyM.rules[j].kind === "regex" ? "regex" : "substring") === k) {
        flash("已经有一条一模一样的了", 1);
        return;
      }
    }
    var next = yyM.rules.slice();
    next[i] = {
      pattern: p, kind: k,
      action: ACTION_LABEL[f.action] ? f.action : "block",
      reason: String(f.reason || "").trim(),
      enabled: !!r.enabled,
      source: r.source
    };
    yyM.busy = "edit:" + i;
    render();
    writeRules(next).then(function () {
      yyM.busy = "";
      yyM.edit = -1;
      flash("已修改，立即生效");
      load();
    })["catch"](function (e) {
      yyM.busy = "";
      flash((e && e.message) || "网络错误", 1);
    });
  }

  // ---- AI 长期禁令：增 / 改 / 删 ----

  function tabooInput(el) { yyM.tabooNew = (el && el.value) || ""; }
  function tabooEditInput(el) { yyM.tabooForm = (el && el.value) || ""; }

  function tabooPlain() {
    return yyM.taboos.map(function (t) { return tabooText(t); });
  }

  function tabooAdd() {
    if (yyM.busy) return;
    var v = String(yyM.tabooNew || "").trim();
    if (!v) { flash("先填写禁令内容", 1); return; }
    if (v.length > 40) { flash("一条最多 40 字", 1); return; }
    var cur = tabooPlain();
    if (cur.indexOf(v) >= 0) { flash("这条已经在表里了", 1); return; }
    if (cur.length >= 12) { flash("长期禁令最多 12 条，先删一条再加", 1); return; }
    yyM.busy = "taboo";
    render();
    writeTaboos(cur.concat([v])).then(function () {
      yyM.tabooNew = "";
      yyM.busy = "";
      flash("已添加，AI 下次回复就遵守");
      load();
    })["catch"](function (e) {
      yyM.busy = "";
      flash((e && e.message) || "网络错误", 1);
    });
  }

  function tabooEdit(i) {
    if (yyM.busy) return;
    var t = tabooText(yyM.taboos[i]);
    yyM.tabooEdit = i;
    yyM.tabooForm = t;
    render();
    var el = document.getElementById("yym-taboo-edit");
    if (el) { el.focus(); try { el.setSelectionRange(el.value.length, el.value.length); } catch (_) {} }
  }

  function tabooCancel() {
    yyM.tabooEdit = -1;
    yyM.tabooForm = "";
    render();
  }

  function tabooSaveEdit() {
    if (yyM.busy) return;
    var i = yyM.tabooEdit;
    if (i < 0) return;
    var v = String(yyM.tabooForm || "").trim();
    if (!v) { flash("禁令内容不能为空", 1); return; }
    if (v.length > 40) { flash("一条最多 40 字", 1); return; }
    var cur = tabooPlain();
    for (var j = 0; j < cur.length; j++) {
      if (j !== i && cur[j] === v) { flash("已经有一条一模一样的了", 1); return; }
    }
    cur[i] = v;
    yyM.busy = "taboo";
    render();
    writeTaboos(cur).then(function () {
      yyM.busy = "";
      yyM.tabooEdit = -1;
      yyM.tabooForm = "";
      flash("已修改，AI 下次回复就遵守");
      load();
    })["catch"](function (e) {
      yyM.busy = "";
      flash((e && e.message) || "网络错误", 1);
    });
  }

  function tabooDel(i) {
    if (yyM.busy) return;
    var t = tabooText(yyM.taboos[i]);
    if (!t) return;
    if (!window.confirm("删除禁令「" + t + "」？删掉后 AI 不再记这条。")) return;
    var cur = tabooPlain().filter(function (_, j) { return j !== i; });
    yyM.busy = "taboo";
    render();
    writeTaboos(cur).then(function () {
      yyM.busy = "";
      yyM.tabooEdit = -1;
      flash("已删除「" + t + "」");
      load();
    })["catch"](function (e) {
      yyM.busy = "";
      flash((e && e.message) || "网络错误", 1);
    });
  }

  function reload() {
    yyM.loaded = 0;
    load();
  }

  // ---------- 挂载（与 guard.js 同一手法） ----------

  function mount() {
    if (typeof window.yyQQRegisterModule !== "function") return;
    window.yyQQRegisterModule({
      id: "robot",
      __key: "moderation",
      order: 30,
      render: function () { return block(); },
      load: function () { if (!yyM.loaded && !yyM.loading) load(); }
    });
  }

  window.yyMFold = fold;
  window.yyMForm = formSync;
  window.yyMToggle = toggle;
  window.yyMAdd = add;
  window.yyMDel = del;
  window.yyMReload = reload;
  window.yyMEdit = editStart;
  window.yyMEditInput = editInput;
  window.yyMSaveEdit = editSave;
  window.yyMCancelEdit = editCancel;
  window.yyMTabooInput = tabooInput;
  window.yyMTabooAdd = tabooAdd;
  window.yyMTabooEdit = tabooEdit;
  window.yyMTabooEditInput = tabooEditInput;
  window.yyMTabooSaveEdit = tabooSaveEdit;
  window.yyMTabooCancel = tabooCancel;
  window.yyMTabooDel = tabooDel;

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", mount);
  else mount();
})();
