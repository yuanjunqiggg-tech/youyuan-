// 互通 · 危险指令名单（玩家自己配）
// ---------------------------------------------------------------------------
// 用户原话：「一些黑名单行为，也让玩家来配置，你别全部给他独揽了。」
// 后端接口：GET/POST /api/qq/guard
//   GET  -> {ok, rules:[{prefix,reason,locked,enabled}], extra:[], note}
//   POST -> {disabled:[], extra:[]}；默认锁定的项也可关，玩家自己说了算
// 挂载手法与 youyuan_qq_knowledge.js 相同：包装 yyQQRenderAll 注入折叠块。
// ---------------------------------------------------------------------------

(function () {
  "use strict";

  var yyG = { loading: 0, loaded: 0, err: "", rules: [], extra: [], note: "", msg: "", bad: 0, open: 0, showAll: 0, busy: 0 };

  function esc(v) {
    return typeof yyQQE === "function" ? yyQQE(v == null ? "" : String(v)) : String(v);
  }
  function icon(n) {
    return typeof yyQQIcon === "function" ? yyQQIcon(n) : "";
  }

  function load() {
    yyG.loading = 1;
    render();
    A("GET", "/api/qq/guard").then(function (r) {
      yyG.loading = 0;
      yyG.loaded = 1;
      if (!r || !r.ok) {
        yyG.err = (r && r.error) || "读取失败";
        render();
        return;
      }
      yyG.rules = r.rules || [];
      yyG.extra = r.extra || [];
      yyG.note = r.note || "";
      yyG.err = "";
      render();
    })["catch"](function () {
      yyG.loading = 0;
      yyG.loaded = 1;
      yyG.err = "网络错误";
      render();
    });
  }

  function enabledCount() {
    var n = 0;
    for (var i = 0; i < yyG.rules.length; i++) if (yyG.rules[i].enabled) n++;
    return n;
  }

  function summary() {
    if (!yyG.loaded) return "点开配置";
    return "已开 " + enabledCount() + " 条 · 自定义 " + yyG.extra.length + " 条";
  }

  function block() {
    return '<details class="yyq-fold" id="yyq-guard"' + (yyG.open ? " open" : "") + ' ontoggle="yyGFold(this)">' +
      '<summary><span class="yyq-fold-ico">' + icon("ban") + "</span>" +
      '<span class="yyq-fold-t">危险指令</span>' +
      '<span class="yyq-fold-sum">' + esc(summary()) + "</span>" +
      '<span class="yyq-fold-caret">' + icon("arrow") + "</span></summary>" +
      '<div class="yyq-fold-body" id="yyq-g-body">' + body() + "</div></details>";
  }

  function body() {
    if (yyG.loading) return '<div class="yyk-skel">正在读取名单…</div>';
    if (yyG.err) {
      return '<p class="yyk-warn">' + esc(yyG.err) + "</p>" +
        '<div class="yyq-fold-actions"><button type="button" class="btn-s" onclick="yyGReload()">重新读取</button></div>';
    }
    var rows = yyG.rules;
    var shown = yyG.showAll ? rows : rows.slice(0, 10);
    var list = shown.map(function (r) {
      return '<li class="yyg-row" data-locked="' + (r.locked ? "1" : "0") + '">' +
        '<label class="yyg-sw"><input type="checkbox"' + (r.enabled ? " checked" : "") +
          ' onchange="yyGToggle(\'' + esc(r.prefix) + '\',this.checked)"><span></span></label>' +
        '<code class="yyg-prefix">/' + esc(r.prefix.trim()) + "</code>" +
        "<small>" + esc(r.reason) + (r.locked ? ' <i class="yyg-lock">默认锁</i>' : "") + "</small>" +
        (r.locked ? '<small class="yyg-locknote">默认锁定；解锁后群内任何人可经授权流程使用该指令，风险自负</small>' : "") +
        "</li>";
    }).join("");
    var more = rows.length > 10
      ? '<button type="button" class="btn-s" onclick="yyGShowAll()">' + (yyG.showAll ? "收起" : "展开全部 " + rows.length + " 条") + "</button>"
      : "";
    var extra = yyG.extra.length
      ? '<ul class="yyg-extra">' + yyG.extra.map(function (p) {
          return '<li><code>/' + esc(p) + "</code>" +
            '<button type="button" class="yyg-x" onclick="yyGExtraDel(\'' + esc(p) + '\')" aria-label="删除">×</button></li>';
        }).join("") + "</ul>"
      : "";
    return '<p class="yyk-hint">命中的指令不会直接执行，会先在群里问超管同不同意。关掉的会直接执行。' +
      '标了「默认锁」的几项（op/deop/stop/ban/ban-ip/whitelist off）默认开着，但锁不锁你说了算——' +
      '解锁后群内任何人可经授权流程使用该指令，风险自负。</p>' +
      '<ul class="yyg-list">' + list + "</ul>" + more + extra +
      '<div class="yyg-add">' +
        '<input id="yyg-new" maxlength="64" placeholder="再加一个前缀，例如 title ">' +
        '<button type="button" class="btn-s" onclick="yyGExtraAdd()">添加</button>' +
      "</div>" +
      '<div class="yyq-fold-actions">' +
        '<button type="button" class="btn-s" onclick="yyGSave(this)">保存</button>' +
        '<button type="button" class="btn-s" onclick="yyGReset(this)">恢复默认</button>' +
        '<span class="yyg-msg" data-bad="' + (yyG.bad ? "1" : "0") + '">' + esc(yyG.msg) + "</span>" +
      "</div>" + (yyG.note ? '<p class="yyk-hint">' + esc(yyG.note) + "</p>" : "");
  }

  function render() {
    var el = document.getElementById("yyq-g-body");
    if (!el) return;
    el.innerHTML = body();
    var sum = document.querySelector("#yyq-guard .yyq-fold-sum");
    if (sum) sum.textContent = summary();
  }

  function flash(text, bad) {
    yyG.msg = text || "";
    yyG.bad = bad ? 1 : 0;
    render();
    if (text) setTimeout(function () {
      if (yyG.msg === text) { yyG.msg = ""; render(); }
    }, 3000);
  }

  function fold(el) {
    yyG.open = el.open ? 1 : 0;
    if (yyG.open && !yyG.loaded && !yyG.loading) load();
  }

  function toggle(prefix, on) {
    for (var i = 0; i < yyG.rules.length; i++) {
      if (yyG.rules[i].prefix === prefix) { yyG.rules[i].enabled = !!on; break; }
    }
  }

  function showAll() { yyG.showAll = yyG.showAll ? 0 : 1; render(); }

  function extraAdd() {
    var el = document.getElementById("yyg-new");
    var v = ((el && el.value) || "").trim().toLowerCase();
    if (!v) { flash("先输入前缀", 1); return; }
    for (var i = 0; i < yyG.extra.length; i++) if (yyG.extra[i] === v) { flash("已经有了", 1); return; }
    yyG.extra.push(v);
    if (el) el.value = "";
    render();
  }

  function extraDel(p) {
    yyG.extra = yyG.extra.filter(function (v) { return v !== p; });
    render();
  }

  function payload() {
    var disabled = [];
    for (var i = 0; i < yyG.rules.length; i++) {
      if (!yyG.rules[i].enabled) disabled.push(yyG.rules[i].prefix);
    }
    return { disabled: disabled, extra: yyG.extra };
  }

  function save(btn) {
    if (yyG.busy) return;
    yyG.busy = 1;
    if (btn) btn.disabled = true;
    A("POST", "/api/qq/guard", payload()).then(function (r) {
      yyG.busy = 0;
      if (btn) btn.disabled = false;
      if (!r || !r.ok) { flash((r && r.error) || "保存失败", 1); return; }
      flash("已保存");
      load();
    })["catch"](function () {
      yyG.busy = 0;
      if (btn) btn.disabled = false;
      flash("网络错误", 1);
    });
  }

  function reset(btn) {
    yyG.extra = [];
    for (var i = 0; i < yyG.rules.length; i++) yyG.rules[i].enabled = true;
    render();
    save(btn);
  }

  function reload() { yyG.loaded = 0; load(); }

  function mount() {
    if (typeof window.yyQQRegisterModule !== "function") return;
    window.yyQQRegisterModule({
      id: "admin",
      __key: "guard",
      order: 10,
      minRole: 1,
      render: function () { return block(); },
      load: function () { if (!yyG.loaded && !yyG.loading) load(); }
    });
  }

  window.yyGFold = fold;
  window.yyGToggle = toggle;
  window.yyGShowAll = showAll;
  window.yyGExtraAdd = extraAdd;
  window.yyGExtraDel = extraDel;
  window.yyGSave = save;
  window.yyGReset = reset;
  window.yyGReload = reload;

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", mount);
  else mount();
})();
