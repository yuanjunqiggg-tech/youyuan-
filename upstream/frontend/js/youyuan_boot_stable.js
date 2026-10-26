// 首屏稳定层：消除「点进工具箱乱跳 / 乱刷新」的三处时序竞争。
// 本文件在 index.html 所有业务脚本之后加载，只做「合并重复触发 + 占位 + 去重」，
// 不改变任何功能路径：被合并的第二次调用与首次完全同源，数据一致。
(function () {
  "use strict";
  var BOOT_AT = Date.now();
  var START_WINDOW_MS = 10000; // 启动窗口：只在前 10 秒做激进去重，之后恢复实时语义
  var READ_TTL_MS = 3000;

  function later(fn, ms) {
    setTimeout(fn, ms);
  }

  // ── 1. 首页渲染去重 ────────────────────────────────────────────
  // 根因：tabTo('home') 首次渲染后，youyuan.js 还有一个 350ms 的补挂
  // setTimeout 会再跑一遍 Rhome()：innerHTML 整体重写（闪一下）、概览数据
  // 再拉一轮（乱刷新）。网络慢时第一轮概览还没回来，第二轮插入正好错开
  // inflight 去重窗口，snapshot 就会 genuinely 重发。
  // 修法：正在渲染中的调用复用同一个 Promise；刚渲染完 1.2 秒内的重复
  // 调用直接跳过（内容已由首次完整渲染，含入口卡片补挂）。
  function hookRhome() {
    if (typeof window.Rhome !== "function" || window.Rhome.__yyStable) return false;
    var base = window.Rhome;
    var running = null;
    var lastDone = 0;
    function wrapped() {
      if (running) return running;
      if (Date.now() - lastDone < 1200) return Promise.resolve();
      running = (async function () {
        try {
          return await base.apply(this, arguments);
        } finally {
          lastDone = Date.now();
          running = null;
        }
      })();
      return running;
    }
    wrapped.__yyStable = true;
    window.Rhome = wrapped;
    return true;
  }

  // ── 2. GET 读请求启动期去重（补 api.js 结果缓存）────────────────
  // 线上 api.js 只有 inflight 共享（并发同 key 只发一次），没有结果缓存：
  // 串行的重复读（如双跑概览错开窗口后的第二轮）仍会再发一次。
  // 这里在启动窗口内加 3 秒结果缓存：写操作不可能发生在启动 10 秒内的
  // 正常流程中，不会把刚改完的状态读成旧值；窗口结束后恢复实时语义。
  function hookA() {
    if (typeof window.A !== "function" || window.A.__yyStable) return false;
    var base = window.A;
    var cache = new Map();
    function wrapped(m, p, b) {
      if (m === "GET" && Date.now() - BOOT_AT < START_WINDOW_MS &&
          !/\/events|\/fly\/|\/mcfunction\//.test(p)) {
        var hit = cache.get(p);
        if (hit && Date.now() - hit.at < READ_TTL_MS) return Promise.resolve(hit.value);
        var r = base.call(this, m, p, b);
        if (r && typeof r.then === "function") {
          r.then(function (v) { cache.set(p, { at: Date.now(), value: v }); }, function () {});
        }
        return r;
      }
      return base.call(this, m, p, b);
    }
    wrapped.__yyStable = true;
    window.A = wrapped;
    return true;
  }

  // ── 3. 互通页即时骨架 ─────────────────────────────────────────
  // 根因：群服互通页要 await 完 /api/qq/config 与 /api/qq/status 才
  // openSubPage，两个请求（Worker→DO→设备往返）期间页面白屏无反馈。
  // 修法：点开瞬间先渲染与最终结构一致的骨架，数据到达后原函数原样
  // 替换内容（它会先 delete 缓存再 openSubPage，衔接无副作用）。
  var QQ_SKELETON =
    '<div class="yy-page-intro"><h2>群服互通</h2><p>让群聊与游戏保持连接，选择机器人适配器，配置消息转发与群指令权限。</p></div>' +
    '<div class="yy-form-grid"><div class="yy-form-stack">' +
    '<section class="card"><div class="yy-form-head"><h3>机器人接入</h3><span class="yy-badge">读取中</span></div>' +
    '<div class="yy-skeleton" aria-hidden="true"><i class="yy-sk-line w60"></i><i class="yy-sk-line w90"></i><i class="yy-sk-line w40"></i></div></section>' +
    '<section class="card"><div class="yy-form-head"><h3>发送测试消息</h3></div>' +
    '<div class="yy-skeleton" aria-hidden="true"><i class="yy-sk-line w80"></i><i class="yy-sk-line w30"></i></div></section>' +
    "</div></div>";
  function hookOpenModule() {
    if (typeof window.yyOpenModule !== "function" || window.yyOpenModule.__yyStable) return false;
    var base = window.yyOpenModule;
    function wrapped(id) {
      if (id === "qq" && !window.__yyQQSketchDone) {
        window.__yyQQSketchDone = true;
        try {
          delete subPageCache["\u7FA4\u670D\u4E92\u901A"];
          openSubPage("\u7FA4\u670D\u4E92\u901A", QQ_SKELETON);
        } catch (e) { /* 骨架失败不影响原路径 */ }
      }
      return base.apply(this, arguments);
    }
    wrapped.__yyStable = true;
    window.yyOpenModule = wrapped;
    return true;
  }

  // ── 4. 登录卡片 Turnstile 占位（CSS 已兜底 :empty，这里补禁用态回收）──
  // .yy-turnstile-slot:empty 已由 youyuan_boot_stable.css 保持 65px，
  // widget 异步插入时无缝填充；仅当 turnstile 配置返回禁用时，slot 会
  // 一直 hidden，无需处理。这里只在异常（配置请求悬挂）时兜底撤占位。
  function watchTurnstile() {
    var deadline = Date.now() + 12000;
    var iv = setInterval(function () {
      var slot = document.getElementById("yy-turnstile-slot");
      if (!slot) {
        if (Date.now() > deadline) clearInterval(iv);
        return;
      }
      var cfg = window.yyTurnstileCfg;
      if (cfg && cfg.enabled === false) {
        // 明确禁用：撤掉 :empty 占位类（若曾被加上）
        slot.classList.remove("yy-ts-hold");
      }
      if (Date.now() > deadline) clearInterval(iv);
    }, 250);
  }

  function hookAll() {
    var ok = hookRhome() && hookA() && hookOpenModule();
    if (ok) watchTurnstile();
    return ok;
  }
  if (!hookAll()) {
    var t = setInterval(function () { if (hookAll()) clearInterval(t); }, 10);
    later(function () { clearInterval(t); }, 5000);
  }
})();
