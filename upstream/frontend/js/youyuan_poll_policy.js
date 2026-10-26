// 云端额度护栏：统一的轮询注册表。
// 部署在 Cloudflare Workers + Durable Objects 上时，每条 /api/* 都是「Worker 1 次 + DO 1 次」双扣，
// 一个标签页的 3 秒长轮询一天就能吃掉 2.8 万次额度。所以所有定时轮询都必须过这一层：
//
//   规则 A：页面不可见（document.hidden）→ 全部暂停，回到前台立刻补一次。
//   规则 B：SPA 路由切走 → 不属于当前页的定时器直接清掉，不在后台空转。
//   规则 C：同一 URL 的并发/短时间重复请求由 api.js 合并，这一层只管调度。
//
// 用法：
//   yyPollDefine('overview', {scopes:['home'], ms:60000, fn:yyRefreshOverview});
//   yyPollDefine('social',   {ms:5000, fn:yyPollSocial});            // 不写 scopes = 全局
//   yyPollKick('overview');                                           // 立即执行一次（手动刷新用）
//   yyPollWatch('sse', ['home','tasks','terminal'], function(on){});  // 只需要知道「该不该跑」
//
// 所有 owner 都是幂等的：重复 define 会先停掉旧的。
//
// 兜底守卫（规则 A 的全面落实）：
//   本文件之后加载的所有脚本，setTimeout(>=500ms)/setInterval 都会被登记。
//   页面一隐藏，全部原生定时器直接清掉；回到前台按原间隔重新武装。
//   这样无需逐个改 N 个文件，那些不归本文件管的轮询（飞行面板、命令方块进度、
//   连接诊断、服务器信息……）在后台标签页里也一次都不跑。自己清自己的定时器
//   （clearInterval/clearTimeout）照常工作，会同步从登记表移除。
(function () {
  'use strict';
  if (window.yyPollDefine) return;
  if (typeof window === 'undefined' || typeof document === 'undefined') return;

  // ---- 原生定时器先抓引用：内部排程用原生，全局守卫用包装 ----
  var nativeSetTimeout = window.setTimeout;
  var nativeClearTimeout = window.clearTimeout;
  var nativeSetInterval = window.setInterval;
  var nativeClearInterval = window.clearInterval;

  var entries = {};   // owner -> {scopes, ms, fn, timer, running, startedAt, active}
  var watchers = {};  // owner -> {scopes, fn, active}
  var routeBound = false;

  function visible() {
    return !(typeof document !== 'undefined' && document.hidden);
  }
  function tabName() {
    return typeof curTab === 'string' && curTab ? curTab : 'home';
  }
  function subOpen() {
    return typeof subPageOpen !== 'undefined' && !!subPageOpen;
  }
  function scopeMatches(scopes) {
    if (!scopes || !scopes.length) return true;          // 全局
    if (subOpen()) return scopes.indexOf('sub') >= 0;    // 子页面只跑标了 sub 的
    return scopes.indexOf(tabName()) >= 0;
  }
  function ownerActive(scopes) {
    return visible() && scopeMatches(scopes);
  }

  function clearEntry(owner) {
    var e = entries[owner];
    if (!e) return;
    if (e.timer) { nativeClearTimeout(e.timer); e.timer = null; }
  }

  function schedule(owner, delay) {
    var e = entries[owner];
    if (!e) return;
    clearEntry(owner);
    if (!ownerActive(e.scopes)) return;
    e.timer = nativeSetTimeout(function () { run(owner); }, delay);
  }

  function run(owner) {
    var e = entries[owner];
    if (!e) return;
    e.timer = null;
    if (!ownerActive(e.scopes)) return;
    if (e.running) { schedule(owner, e.ms); return; }
    e.running = true;
    var done = function () {
      e.running = false;
      // 执行期间页面可能被切走/隐藏，重新判断后再排下一轮。
      if (entries[owner] === e) schedule(owner, e.ms);
    };
    try {
      var r = e.fn();
      if (r && typeof r.then === 'function') r.then(done, done);
      else done();
    } catch (err) {
      done();
    }
  }

  // ---- 全局定时器守卫：页面不可见时，登记表内的定时器全部停摆 ----
  // 只接管 >= GUARD_MIN_MS 的定时器：更短的多是动画/转场微任务，交给浏览器节流即可。
  var GUARD_MIN_MS = 500;
  var guardTimers = {};                 // id -> {fn, ms, interval}
  var guardPaused = !visible();         // 加载时就是隐藏页（少见）则直接按暂停态登记
  var guardSeq = 1;

  function guardPause() {
    if (guardPaused) return;
    guardPaused = true;
    Object.keys(guardTimers).forEach(function (id) {
      var t = guardTimers[id];
      if (t.interval) nativeClearInterval(Number(id)); else nativeClearTimeout(Number(id));
    });
  }

  function guardResume() {
    if (!guardPaused) return;
    guardPaused = false;
    var pending = [];
    Object.keys(guardTimers).forEach(function (id) {
      pending.push(guardTimers[id]);
    });
    guardTimers = {};
    // 恢复即补跑：每个定时器按原间隔重新武装，不等到原相位。
    pending.forEach(function (t) {
      var id = t.interval
        ? nativeSetInterval(t.fn, t.ms)
        : nativeSetTimeout(function () { delete guardTimers[id]; t.fn(); }, t.ms);
      guardTimers[id] = t;
    });
  }

  window.setTimeout = function (fn, ms) {
    if (typeof fn !== 'function' || !(ms >= GUARD_MIN_MS)) return nativeSetTimeout(fn, ms);
    var t = { fn: fn, ms: ms, interval: false };
    if (guardPaused) {
      // 隐藏期间新建的定时器只登记不启动，恢复时统一武装。
      var parked = 0x7ffffff0 + (guardSeq++);
      guardTimers[parked] = t;
      return parked;
    }
    var id = 0;
    id = nativeSetTimeout(function () { delete guardTimers[id]; fn(); }, ms);
    guardTimers[id] = t;
    return id;
  };
  window.clearTimeout = function (id) {
    delete guardTimers[id];
    return nativeClearTimeout(id);
  };
  window.setInterval = function (fn, ms) {
    if (typeof fn !== 'function') return nativeSetInterval(fn, ms);
    var t = { fn: fn, ms: ms, interval: true };
    if (guardPaused) {
      var parkedI = 0x7ffffff0 + (guardSeq++);
      guardTimers[parkedI] = t;
      return parkedI;
    }
    var idI = nativeSetInterval(fn, ms);
    guardTimers[idI] = t;
    return idI;
  };
  window.clearInterval = function (id) {
    delete guardTimers[id];
    return nativeClearInterval(id);
  };

  function evaluate() {
    if (visible()) guardResume(); else guardPause();
    Object.keys(entries).forEach(function (owner) {
      var e = entries[owner];
      var on = ownerActive(e.scopes);
      if (on) {
        // 刚恢复：立刻补一次，不等下一个周期。
        if (!e.timer && !e.running) schedule(owner, 0);
      } else {
        clearEntry(owner);
      }
    });
    Object.keys(watchers).forEach(function (owner) {
      var w = watchers[owner];
      var on = ownerActive(w.scopes);
      if (on === w.active) return;
      w.active = on;
      try { w.fn(on); } catch (err) {}
    });
  }

  function bindRoute() {
    if (routeBound) return;
    routeBound = true;
    // 路由函数在 app.js 里是全局函数声明，这里包一层通知，不改动原实现。
    ['tabTo', 'openSubPage', 'closeSubPage'].forEach(function (name) {
      var original = window[name];
      if (typeof original !== 'function') return;
      window[name] = function () {
        var out = original.apply(this, arguments);
        // closeSubPage 有 310ms 收起动画，路由状态稍后才稳定。
        setTimeout(evaluate, name === 'closeSubPage' ? 340 : 0);
        return out;
      };
    });
  }

  window.yyPollDefine = function (owner, opts) {
    opts = opts || {};
    clearEntry(owner);
    entries[owner] = {
      scopes: opts.scopes || null,
      ms: Math.max(1000, Number(opts.ms) || 30000),
      fn: opts.fn,
      timer: null,
      running: false
    };
    if (ownerActive(entries[owner].scopes)) schedule(owner, opts.immediate === false ? entries[owner].ms : 0);
  };

  window.yyPollStop = function (owner) {
    clearEntry(owner);
    delete entries[owner];
  };

  window.yyPollSetInterval = function (owner, ms) {
    var e = entries[owner];
    if (!e) return;
    e.ms = Math.max(1000, Number(ms) || e.ms);
  };

  window.yyPollKick = function (owner) {
    var e = entries[owner];
    if (!e || !ownerActive(e.scopes)) return false;
    run(owner);
    return true;
  };

  window.yyPollActive = function (owner) {
    var e = entries[owner] || watchers[owner];
    return !!e && ownerActive(e.scopes);
  };

  // 只关心「该不该跑」的场景（例如 EventSource 自己管自己的定时器）。
  window.yyPollWatch = function (owner, scopes, fn) {
    watchers[owner] = { scopes: scopes || null, fn: fn, active: ownerActive(scopes) };
    try { fn(watchers[owner].active); } catch (err) {}
  };

  window.yyPollEvaluate = evaluate;

  // 排查用：当前守卫状态与在册定时器数量。
  window.yyPollGuardStats = function () {
    return { paused: guardPaused, timers: Object.keys(guardTimers).length };
  };

  document.addEventListener('visibilitychange', evaluate);
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', function () { bindRoute(); evaluate(); });
  else { bindRoute(); evaluate(); }
  // app.js 在本脚本之后加载，补一次绑定。
  setTimeout(function () { bindRoute(); evaluate(); }, 0);
  setTimeout(function () { bindRoute(); evaluate(); }, 1200);

  // ---- 本地请求计数（判断额度消耗用，不新增后端接口）----
  // 包一层 window.fetch：所有 /api/* 请求（含 sse.js 直接 fetch 的事件长轮询）都计数，
  // api.js 里不再重复计数。
  var HIT_KEY = 'yy-api-hits-v1';
  function today() {
    var d = new Date();
    return d.getFullYear() + '-' + (d.getMonth() + 1) + '-' + d.getDate();
  }
  function readHits() {
    try {
      var raw = sessionStorage.getItem(HIT_KEY);
      if (!raw) return { day: today(), n: 0 };
      var o = JSON.parse(raw);
      if (!o || o.day !== today()) return { day: today(), n: 0 };
      return { day: o.day, n: Number(o.n) || 0 };
    } catch (e) { return { day: today(), n: 0 }; }
  }
  window.yyApiHit = function () {
    var o = readHits();
    o.n += 1;
    try { sessionStorage.setItem(HIT_KEY, JSON.stringify(o)); } catch (e) {}
    if (typeof window.yyApiHitRender === 'function') {
      try { window.yyApiHitRender(o.n); } catch (e) {}
    }
    return o.n;
  };
  window.yyApiHitCount = function () { return readHits().n; };

  var nativeFetch = typeof window.fetch === 'function' ? window.fetch : null;
  if (nativeFetch) {
    window.fetch = function (input, init) {
      try {
        var url = typeof input === 'string' ? input : (input && input.url) || '';
        if (url.indexOf('/api/') >= 0) window.yyApiHit();
      } catch (e) {}
      return nativeFetch(input, init);
    };
  }
})();
