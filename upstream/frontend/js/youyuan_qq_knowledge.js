// 互通 · 知识库面板（AI 备忘录 / 知识库 / Skill）
// ---------------------------------------------------------------------------
// 用户原话：「AI的相关知识库呢？ not_found 只显示这个 我希望点进去的话，
// 默认的就是有几个目录」+「AI自己的备忘录 知识库 Skill 而且玩家都可以添加进来
// 可以上传文档 压缩包这些 就点那个窗口就行」。
//
// 为什么以前只有 not_found：前端打的是 /api/qq/rag/*，而 youyuan_qq_rag.go 的
// yyQQRagRoutes **只定义不注册**（全仓零调用点），请求落到 main.go handleIndex
// 的 /api/ 兜底，返回 {"ok":false,"error":"not_found"}，这里把 error 原样显示。
// 现在的接口是 /api/qq/kb/*（youyuan_qq_knowledge.go），真的注册过，打开就有三类目录。
//
// 接口：
//   GET    /api/qq/kb/tree            三类目录 + 条目 + 限制值
//   GET    /api/qq/kb/doc?id=         预览正文（内置技能恒可读）
//   DELETE /api/qq/kb/doc?id=         删除自己传的
//   POST   /api/qq/kb/text            粘贴保存
//   POST   /api/qq/kb/upload          multipart 上传（点窗口就选文件）
//   POST   /api/qq/kb/search          试用检索
// ---------------------------------------------------------------------------

(function () {
  "use strict";

  var yyK = {
    loading: 0, loaded: 0, err: "",
    cats: [], limits: null, stats: null, persistErr: "",
    tab: "memo",
    open: 0,
    busy: "", msg: "", bad: 0,
    preview: {},          // id -> {open:1, text:"", err:""}
    draft: "",            // 手写草稿
    q: "", hits: null, searching: 0
  };

  function esc(v) {
    return typeof yyQQE === "function" ? yyQQE(v == null ? "" : String(v)) : String(v == null ? "" : v);
  }
  function icon(n) {
    return typeof yyQQIcon === "function" ? yyQQIcon(n) : "";
  }
  function bytes(n) {
    n = Number(n) || 0;
    if (!n) return "0 B";
    if (n < 1024) return n + " B";
    if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
    return (n / 1024 / 1024).toFixed(2) + " MB";
  }
  function cat(id) {
    for (var i = 0; i < yyK.cats.length; i++) {
      if (yyK.cats[i].id === id) return yyK.cats[i];
    }
    return null;
  }

  // ---------- 数据 ----------

  function load() {
    if (yyK.loading) return;
    yyK.loading = 1;
    yyK.err = "";
    render();
    A("GET", "/api/qq/kb/tree").then(function (r) {
      yyK.loading = 0;
      yyK.loaded = 1;
      if (!r || !r.ok) {
        // 以前这里直接把 "not_found" 显示出来，等于让用户猜。现在给能行动的话。
        yyK.err = (r && r.error) || "读知识库失败";
        yyK.cats = [];
        render();
        return;
      }
      yyK.cats = r.categories || [];
      yyK.limits = r.limits || null;
      yyK.stats = r.stats || null;
      yyK.persistErr = r.persistence_error || "";
      if (!cat(yyK.tab) && yyK.cats.length) yyK.tab = yyK.cats[0].id;
      render();
    })["catch"](function () {
      yyK.loading = 0;
      yyK.loaded = 1;
      yyK.err = "网络错误，读不到知识库";
      render();
    });
  }

  // ---------- 渲染 ----------

  function block() {
    // 只返回一段分区：外层 <details class="yyq-mod"> 由首页模块壳提供。
    return '<section class="yyq-sec yyq-sec-wide" id="yyq-knowledge">' +
      '<div class="yyq-sec-head"><h4>知识库</h4><small>' + esc(summaryText()) + "</small></div>" +
      '<div id="yyq-k-body">' + body() + "</div>" +
      "</section>";
  }

  function summaryText() {
    if (yyK.loading) return "读取中…";
    if (!yyK.loaded) return "点开查看";
    if (yyK.err) return yyK.err;
    var bits = [];
    for (var i = 0; i < yyK.cats.length; i++) {
      bits.push(yyK.cats[i].name + " " + (yyK.cats[i].count || 0));
    }
    return bits.length ? bits.join(" · ") : "还没有目录";
  }

  function body() {
    if (yyK.loading && !yyK.loaded) return '<div class="yyk-skel">正在读知识库…</div>';
    if (yyK.err) {
      return '<p class="yyk-warn">' + esc(yyK.err) + "</p>" +
        '<div class="yyq-fold-actions"><button type="button" class="btn-s" onclick="yyKReload()">重新读取</button></div>';
    }
    return catBar() + panel() + footer();
  }

  function catBar() {
    if (!yyK.cats.length) return "";
    return '<div class="yykb-cats" role="tablist">' + yyK.cats.map(function (c) {
      var on = c.id === yyK.tab;
      return '<button type="button" role="tab" aria-selected="' + (on ? "true" : "false") + '"' +
        ' class="yykb-cat' + (on ? " is-on" : "") + '" onclick="yyKCat(\'' + esc(c.id) + '\')">' +
        "<b>" + esc(c.name) + "</b><span>" + (c.count || 0) + " 条</span></button>";
    }).join("") + "</div>";
  }

  function panel() {
    var c = cat(yyK.tab);
    if (!c) return '<p class="yyk-hint">没有可用的目录。</p>';
    var lim = yyK.limits || {};
    var maxMB = Math.round((lim.file_bytes || 4194304) / 1048576);
    var drop =
      '<div class="yykb-drop" onclick="yyKPick()" ondragover="yyKDrag(event,1)" ondragleave="yyKDrag(event,0)" ondrop="yyKDrop(event)">' +
      "<strong>" + esc(yyK.busy || "点这里上传，或把文件拖进来") + "</strong>" +
      "<span>.md / .txt / .json / .csv / .zip · 单个 ≤ " + maxMB + " MB · zip 只取里面的文本，图片和程序会自动跳过</span>" +
      "</div>" +
      '<input type="file" id="yykb-file" accept=".md,.markdown,.txt,.json,.csv,.zip" hidden onchange="yyKFile(this)">';

    var list = "";
    if (c.items && c.items.length) {
      list = '<ul class="yykb-list">' + c.items.map(function (it) {
        var p = yyK.preview[it.id];
        var row = '<li class="yykb-row' + (p && p.open ? " is-open" : "") + '">' +
          '<button type="button" class="yykb-item" onclick="yyKOpen(\'' + esc(it.id) + '\')">' +
          "<b>" + esc(it.title) + "</b>" +
          "<span>" + esc(it.builtin ? "内置" : ((it.chunks || 0) + " 段 · " + bytes(it.bytes || 0))) +
          (it.updated_at ? " · " + esc(it.updated_at) : "") + "</span></button>" +
          (it.builtin ? "" : '<button type="button" class="yyk-del" onclick="yyKDel(\'' + esc(it.id) + '\')" aria-label="删除">×</button>') +
          "</li>";
        if (p && p.open) {
          row += '<li class="yykb-pre-li">' +
            (p.err ? '<p class="yyk-warn">' + esc(p.err) + "</p>" :
              '<pre class="yykb-pre">' + esc(p.text || "读取中…") + "</pre>") + "</li>";
        }
        return row;
      }).join("") + "</ul>";
    } else {
      list = '<p class="yyk-hint">这个目录还是空的——上面点一下传个文件，或下面手写一条。</p>';
    }

    var writer = c.writable ? '<details class="yykb-new"><summary>手写一条（存进「' + esc(c.name) + "」）</summary>" +
      '<div class="yyk-add">' +
      '<input id="yykb-title" placeholder="标题（留空就用正文第一行）">' +
      '<textarea id="yykb-text" rows="5" placeholder="用「## 小标题」分段，检索会按小标题切片">' + esc(yyK.draft) + "</textarea>" +
      '<button type="button" class="btn-s" onclick="yyKSave(this)">存进' + esc(c.name) + "</button>" +
      "</div></details>" : "";

    return '<p class="yykb-desc">' + esc(c.desc) + "</p>" + drop + list + writer;
  }

  function footer() {
    var warn = "";
    if (yyK.persistErr) {
      warn = '<p class="yyk-warn">知识库落盘失败（' + esc(yyK.persistErr) + "），重启后会丢，先看磁盘权限。</p>";
    }
    var msg = yyK.msg ? '<p class="yyk-msg" data-bad="' + (yyK.bad ? "1" : "0") + '">' + esc(yyK.msg) + "</p>" : "";
    var search = '<div class="yyk-row yykb-search">' +
      '<input id="yykb-q" placeholder="试一下检索：比如「白名单怎么申请」" value="' + esc(yyK.q) + '">' +
      '<button type="button" class="btn-s" onclick="yyKSearch(this)">' + (yyK.searching ? "检索中" : "检索") + "</button></div>";
    var hits = "";
    if (yyK.hits) {
      if (!yyK.hits.length) {
        hits = '<p class="yyk-hint">没命中。换个说法，或先在当前目录加一条。</p>';
      } else {
        hits = '<ul class="yyk-hits">' + yyK.hits.slice(0, 5).map(function (h) {
          var t = h.text || h.Text || "";
          if (t.length > 160) t = t.slice(0, 160) + "…";
          var s = h.score != null ? h.score : h.Score;
          return '<li><span class="yyk-score">' + (s != null ? Number(s).toFixed(2) : "-") + "</span>" +
            "<p>" + esc(t) + "</p></li>";
        }).join("") + "</ul>";
      }
    }
    return warn + search + hits + msg +
      '<p class="yyk-hint">机器人回答前会先查这里：「AI 备忘录」是它自己攒的结论，「知识库」放规则和资料，「Skill」放怎么做。</p>';
  }

  function render() {
    var el = document.getElementById("yyq-k-body");
    if (el) el.innerHTML = body();
  }

  // ---------- 交互 ----------

  function foldToggle(el) {
    yyK.open = el.open ? 1 : 0;
    if (yyK.open && !yyK.loaded) load();
  }

  function switchCat(id) {
    if (yyK.tab === id) return;
    yyK.tab = id;
    yyK.hits = null;
    yyK.msg = "";
    render();
  }

  function reload() {
    yyK.loaded = 0;
    yyK.hits = null;
    load();
  }

  function flash(text, bad) {
    yyK.msg = text || "";
    yyK.bad = bad ? 1 : 0;
    render();
    if (text) {
      setTimeout(function () {
        if (yyK.msg !== text) return;
        yyK.msg = "";
        render();
      }, 5000);
    }
  }

  function pick() {
    if (yyK.busy) return;
    var input = document.getElementById("yykb-file");
    if (input) input.click();
  }

  function drag(ev, on) {
    if (ev && ev.preventDefault) ev.preventDefault();
    if (ev && ev.currentTarget) {
      if (on) ev.currentTarget.classList.add("is-over");
      else ev.currentTarget.classList.remove("is-over");
    }
  }

  function drop(ev) {
    if (ev && ev.preventDefault) ev.preventDefault();
    drag(ev, 0);
    var files = ev && ev.dataTransfer && ev.dataTransfer.files;
    if (files && files.length) upload(files[0]);
  }

  function file(input) {
    var f = input && input.files && input.files[0];
    if (!f) return;
    input.value = "";
    upload(f);
  }

  function upload(f) {
    if (!f) return;
    var max = (yyK.limits && yyK.limits.file_bytes) || 4 * 1024 * 1024;
    if (f.size > max) {
      flash("「" + f.name + "」有 " + bytes(f.size) + "，超过 " + bytes(max) + " 上限，请拆分或压缩后再传", 1);
      return;
    }
    var fd = new FormData();
    fd.append("file", f);
    fd.append("category", yyK.tab);
    yyK.busy = "正在上传 " + f.name + "…";
    render();
    fetch("/api/qq/kb/upload", { method: "POST", body: fd }).then(function (resp) {
      return resp.json();
    }).then(function (r) {
      yyK.busy = "";
      if (!r || !r.ok) {
        flash((r && r.error) || "上传失败", 1);
        return;
      }
      var text = "已存入 " + (r.added || 1) + " 篇";
      if (r.skipped) text += "，跳过 " + r.skipped + " 个非文本条目";
      if (r.replaced) text += "（同名旧文档已覆盖）";
      flash(text);
      yyK.loaded = 0;
      load();
    })["catch"](function () {
      yyK.busy = "";
      flash("网络错误，上传没成功", 1);
    });
  }

  function open(id) {
    var p = yyK.preview[id];
    if (p && p.open) {
      delete yyK.preview[id];
      render();
      return;
    }
    yyK.preview[id] = { open: 1, text: "", err: "" };
    render();
    A("GET", "/api/qq/kb/doc?id=" + encodeURIComponent(id)).then(function (r) {
      if (!r || !r.ok) {
        yyK.preview[id] = { open: 1, text: "", err: (r && r.error) || "读不到这篇文档" };
      } else {
        yyK.preview[id] = { open: 1, text: r.text || "（空）", err: "" };
      }
      render();
    })["catch"](function () {
      yyK.preview[id] = { open: 1, text: "", err: "网络错误" };
      render();
    });
  }

  function del(id) {
    if (!id) return;
    if (!window.confirm("删掉这篇？它的检索片段会一起删，不能恢复。")) return;
    A("DELETE", "/api/qq/kb/doc?id=" + encodeURIComponent(id)).then(function (r) {
      if (!r || !r.ok) {
        flash((r && r.error) || "删除失败", 1);
        return;
      }
      flash("已删除 " + (r.deleted_chunks || 0) + " 段");
      delete yyK.preview[id];
      yyK.loaded = 0;
      load();
    })["catch"](function () {
      flash("网络错误", 1);
    });
  }

  function save(btn) {
    var t = document.getElementById("yykb-title");
    var x = document.getElementById("yykb-text");
    var text = (x && x.value) || "";
    if (!text.trim()) {
      flash("先写点内容", 1);
      return;
    }
    btn.disabled = true;
    A("POST", "/api/qq/kb/text", { category: yyK.tab, title: (t && t.value) || "", text: text }).then(function (r) {
      btn.disabled = false;
      if (!r || !r.ok) {
        flash((r && r.error) || "保存失败", 1);
        return;
      }
      yyK.draft = "";
      flash("已存入 " + (r.chunks || 0) + " 段" + (r.replaced ? "（同名已覆盖）" : ""));
      yyK.loaded = 0;
      load();
    })["catch"](function () {
      btn.disabled = false;
      flash("网络错误", 1);
    });
  }

  function search(btn) {
    var el = document.getElementById("yykb-q");
    var q = (el && el.value) || "";
    if (!q.trim()) {
      flash("先输入要检索的词", 1);
      return;
    }
    yyK.q = q;
    yyK.searching = 1;
    btn.disabled = true;
    A("POST", "/api/qq/kb/search", { query: q, category: yyK.tab }).then(function (r) {
      btn.disabled = false;
      yyK.searching = 0;
      if (!r || !r.ok) {
        flash((r && r.error) || "检索失败", 1);
        return;
      }
      yyK.hits = r.hits || [];
      yyK.msg = "";
      render();
    })["catch"](function () {
      btn.disabled = false;
      yyK.searching = 0;
      flash("网络错误", 1);
    });
  }

  // ---------- 挂载 ----------

  function mount() {
    if (typeof window.yyQQRegisterModule !== "function") return;
    window.yyQQRegisterModule({
      id: "ai",
      __key: "knowledge",
      order: 40,
      render: function () { return block(); },
      load: function () { if (!yyK.loaded && !yyK.loading) load(); }
    });
  }

  // inline onclick 需要全局函数
  window.yyKFoldToggle = foldToggle;
  window.yyKCat = switchCat;
  window.yyKReload = reload;
  window.yyKPick = pick;
  window.yyKDrag = drag;
  window.yyKDrop = drop;
  window.yyKFile = file;
  window.yyKOpen = open;
  window.yyKDel = del;
  window.yyKSave = save;
  window.yyKSearch = search;

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount);
  } else {
    mount();
  }
})();
