var _currentTask = null;
// 云端额度护栏：事件流以前是「每 3 秒一条 /api/youyuan/events」，一个标签页一天 2.88 万次，
// 而每条 /api/* 在 Workers + DO 上是双扣。现在改成自带节奏的增量通道：
//   · 页面不可见 → 完全不拉
//   · 不在需要实时日志的页面（工作台/插件管理/终端日志）→ 完全不拉，回到这些页立刻补拉
//   · 在需要日志的页面：有任务在跑 3 秒一次，空闲 15 秒一次
// 游标（after=）保证补拉时不丢事件；断线按指数退避，最长 60 秒。
function yyEventsChannel() {
  var self = this;
  self.listeners = {};
  self.closed = false;
  self.cursor = 0;
  self.account = typeof yyActiveAccount === "string" ? yyActiveAccount : "main";
  var opened = false, failures = 0, timer = null, busy = false;
  self.addEventListener = function (name, cb) { (self.listeners[name] || (self.listeners[name] = [])).push(cb); };
  function onRoute() { return typeof yyPollActive === "function" ? yyPollActive("sse") : true; }
  function tempo() {
    if (document.hidden) return 0;
    if (!onRoute()) return _currentTask ? 30000 : 0;
    return _currentTask ? 3000 : 15000;
  }
  function stop() { if (timer) { clearTimeout(timer); timer = null; } }
  function schedule(ms) { stop(); if (self.closed || !ms) return; timer = setTimeout(poll, ms); }
  async function poll() {
    timer = null;
    if (self.closed) return;
    var ms = tempo();
    if (!ms) return;                       // 暂停，等可见性/路由变化唤醒
    if (busy) { schedule(ms); return; }
    if (self.account !== yyActiveAccount) { self.account = yyActiveAccount; self.cursor = 0; opened = false; }
    var account = self.account;
    busy = true;
    try {
      var r = await fetch("/api/youyuan/events?after=" + self.cursor);
      var data = await r.json();
      if (self.closed) return;
      if (account !== yyActiveAccount) { busy = false; schedule(100); return; }
      if (!r.ok || !data.ok) throw Error(data.error || "内核尚未上线");
      failures = 0;
      if (!opened) { opened = true; self.cursor = data.cursor || 0; if (self.onopen) self.onopen({}); }
      else (data.events || []).forEach(function (event) {
        if (self.closed || event.seq <= self.cursor) return;
        self.cursor = event.seq;
        var message = { data: typeof event.data === "string" ? event.data : JSON.stringify(event.data) };
        (self.listeners[event.type] || []).forEach(function (cb) { cb(message); });
      });
    } catch (error) {
      if (self.closed) return;
      failures++;
      if (self.onerror) self.onerror(error);
    }
    busy = false;
    if (self.closed) return;
    var next = tempo();
    if (!next) return;
    schedule(failures ? Math.min(60000, next * Math.pow(2, Math.min(failures, 6))) : next);
  }
  self.close = function () { self.closed = true; stop(); };
  // 唤醒：可见性/路由/任务状态变化时立刻补一次（80ms 后），避免等满一个周期。
  self.wake = function () {
    if (self.closed) return;
    var ms = tempo();
    if (!ms) { stop(); return; }
    schedule(80);
  };
  self.retempo = self.wake;
  if (typeof yyPollWatch === "function") yyPollWatch("sse", ["home", "tasks", "terminal"], function () { self.wake(); });
  if (document.addEventListener) {
    document.addEventListener("visibilitychange", function () {
      if (document.hidden) stop(); else self.wake();
    });
  }
  schedule(100);
}
(function() {
  try {
    let rfshHomeThrottled2 = function() {
      var now = Date.now();
      if (curTab === "home" && !subPageOpen && now - lastHomeRef >= 2e3) {
        lastHomeRef = now;
        refreshHomeTasks();
      }
    }, rfshTasks2 = function() {
      try {
        if (curTab === "tasks" && !subPageOpen) Rplg();
      } catch (e2) {
      }
    }, rfshTasksThrottled2 = function() {
      var now = Date.now();
      if (now - lastTskRef >= 2e3) {
        lastTskRef = now;
        rfshTasks2();
      }
    }, c2 = function() {
      if (e) e.close();
      e = new yyEventsChannel();
      e.onopen = function() {
        A("GET", "/api/bot/status").then(function(s) {
          U(s.ok && s.connected, s.ok && s.is_op);
        });
      };
      e.addEventListener("plugin_update", function(v) {
        var p = JSON.parse(v.data);
        addL("[PLUGIN] " + p.id + " \xB7 " + p.action);
        rfshTasksThrottled2();
      });
      e.onerror = function() {
        clearTimeout(t);
        t = setTimeout(c2, 3e3);
      };
      e.addEventListener("progress", function(v) {
        try {
          var d = JSON.parse(v.data);
          addL(d.msg);
        } catch (e2) {
        }
      });
      // 多机器人连接进度（后端 fleet.go 已在发 fleet_progress），此前没有任何监听，
      // 事件发出来就被丢掉；接进同一条日志流，连子机器人时终端才有过程可见。
      e.addEventListener("fleet_progress", function(v) {
        try {
          var d = JSON.parse(v.data);
          addL(d.msg);
        } catch (e2) {
        }
      });
      e.addEventListener("task_start", function(v) {
        var d = JSON.parse(v.data);
        _currentTask = { type: d.type, progress: 0 };
        addL("[\u542F\u52A8] " + d.type);
        rfshTasks2();
        if (curTab === "home") refreshHomeTasks();
      });
      e.addEventListener("task_paused", function(v) {
        var d = JSON.parse(v.data);
        _currentTask = null;
        var mp = document.getElementById("imp-multi-pg");
        if (mp) mp.classList.add("gone");
        addL("[\u5DF2\u6682\u505C] " + (d.msg || "\u5DF2\u4FDD\u5B58\u65AD\u70B9"), "s");
        T(d.msg || "\u5DF2\u6682\u505C\uFF0C\u53EF\u4ECE\u65AD\u70B9\u7EE7\u7EED", "s");
        resetTaskButtons();
        rfshTasks2();
        if (curTab === "home") refreshHomeTasks();
        if (typeof cbPauseImport === "function") cbPauseImport(d.msg);
      });
      e.addEventListener("task_done", function(v) {
        var d = JSON.parse(v.data);
        _currentTask = null;
        var mp = document.getElementById("imp-multi-pg");
        if (mp) mp.classList.add("gone");
        addL(ICONS.check + " \u5B8C\u6210 " + (d.msg || ""), "o");
        T(d.msg || "\u4EFB\u52A1\u5B8C\u6210", "o");
        resetTaskButtons();
        rfshTasks2();
        if (curTab === "home") refreshHomeTasks();
        cbFinalizeImport(true, d.msg);
      });
      e.addEventListener("task_error", function(v) {
        var d = JSON.parse(v.data);
        _currentTask = null;
        var mp = document.getElementById("imp-multi-pg");
        if (mp) mp.classList.add("gone");
        addL(ICONS.x + " \u9519\u8BEF " + (d.error || ""), "e");
        T(d.error || "\u4EFB\u52A1\u5931\u8D25", "e");
        resetTaskButtons();
        rfshTasks2();
        if (curTab === "home") refreshHomeTasks();
        cbFinalizeImport(false, d.error);
      });
      e.addEventListener("task_progress", function(v) {
        var d = JSON.parse(v.data);
        var importStatus = document.getElementById("imp-start-status");
        if (importStatus && d.phase && _currentTask && String(_currentTask.type).indexOf("导入") !== -1) importStatus.textContent = d.phase;
        if (typeof yyTransferProgress === "function") yyTransferProgress(d, _currentTask);
        rfshTasksThrottled2();
        if (d.progress != null) {
          if (_currentTask) _currentTask.progress = d.progress;
          var b = document.getElementById(_currentTask && _currentTask.type === "export" ? "exp-bar" : "imp-bar") || document.getElementById("md-bar") || document.getElementById("cb-import-bar");
          if (b) b.style.width = d.progress * 100 + "%";
          var cbp = document.getElementById("cb-import-pct");
          if (cbp) cbp.textContent = d.progress >= 1 ? "\u5DF2\u63D0\u4EA4" : "\u6B63\u5728\u5199\u5165";
        }
        ;
        if (d.multi) {
          var mp = document.getElementById("imp-multi-pg");
          if (mp) {
            mp.classList.remove("gone");
            document.getElementById("imp-pg").classList.add("gone");
          }
          ;
          var u = function(id, pct) {
            var el = document.getElementById(id);
            if (el) el.style.width = pct + "%";
          };
          u("mp-bar-normal", (d.multi.normal || 0) * 100);
          u("mp-bar-nbt", (d.multi.nbt || 0) * 100);
          u("mp-bar-sign", (d.multi.sign || 0) * 100);
          ["normal", "nbt", "sign"].forEach(function(k) {
            var el = document.getElementById("mp-pct-" + k);
            if (el) el.textContent = (d.multi[k] || 0) >= 1 ? "\u5DF2\u63D0\u4EA4" : (d.multi[k] || 0) > 0 ? "\u6B63\u5728\u5199\u5165" : "\u7B49\u5F85\u5F00\u59CB";
          });
        }
        ;
        rfshHomeThrottled2();
      });
      e.addEventListener("structure_progress", function(v) {
        var d = JSON.parse(v.data);
        if (typeof yyTransferProgress === "function") yyTransferProgress(d, _currentTask);
      });
      e.addEventListener("clear_progress", function(v) {
        var d = JSON.parse(v.data);
        if (d.progress != null) {
          var b = document.getElementById("clear-bar");
          var p = document.getElementById("clear-pg");
          var s = document.getElementById("clear-stat");
          if (b) b.style.width = d.progress * 100 + "%";
          if (p) p.classList.remove("gone");
          if (s) {
            s.classList.remove("gone");
            s.textContent = "\u6E05\u7A7A\u4E2D " + (d.cleared || 0) + " / " + (d.total || 0) + " \u65B9\u5757";
          }
        }
        ;
        var ipg = document.getElementById("imp-pg");
        if (ipg) ipg.classList.add("gone");
        var ist = document.getElementById("imp-stat");
        if (ist) ist.classList.add("gone");
      });
      e.addEventListener("clear_done", function(v) {
        var b = document.getElementById("clear-bar");
        var p = document.getElementById("clear-pg");
        var s = document.getElementById("clear-stat");
        if (b) b.style.width = "100%";
        if (p) p.classList.add("gone");
        if (s) s.classList.add("gone");
        var ipg = document.getElementById("imp-pg");
        if (ipg) ipg.classList.remove("gone");
        var ist = document.getElementById("imp-stat");
        if (ist) ist.classList.remove("gone");
      });
      e.addEventListener("bot_log", function(v) {
        var d = JSON.parse(v.data);
        var cl = document.getElementById("conn-log");
        if (cl && typeof yyConnAppendLog === "function") {
          // 弹窗内日志：默认遮挡敏感 token、折叠、独立滚动，由 settings.js 统一渲染
          yyConnAppendLog(d.msg);
        } else if (cl) {
          var de = document.createElement("div");
          de.innerHTML = renderLogLine(d.msg);
          cl.appendChild(de);
          cl.scrollTop = cl.scrollHeight;
        } else {
          addL(d.msg);
        }
        if (typeof yyConnEvent === "function") {
          yyConnEvent(d.msg);
          return;
        }
        var s = document.getElementById("conn-stage");
        if (s) {
          var m = d.msg;
          if (m.indexOf("\u5DF2\u8FDE\u63A5\u5230\u670D\u52A1\u5668") >= 0) s.textContent = "\u5DF2\u8FDE\u63A5\uFF0C\u521D\u59CB\u5316\u4E2D...";
        }
      });
      e.addEventListener("conn_ready", function(v) {
        // 弹窗打开时由 yyConnOnReady 接管：显示「已连接」，不强行关窗
        var handled = typeof yyConnOnReady === "function" && yyConnOnReady();
        if (handled) {
          T("\u5DF2\u8FDE\u63A5", "o");
        } else {
          var el = document.getElementById("conn-modal");
          if (el) {
            el.remove();
            T("\u5DF2\u8FDE\u63A5", "o");
          }
        }
        A("GET", "/api/bot/status").then(function(st) {
          U(true, st.is_op);
          if (curTab === "home") Rhome();
          else updateConnStatus(true, st.is_op);
        });
      });
      e.addEventListener("music_start", function(v) {
        var d = JSON.parse(v.data);
        addL("[\u97F3\u4E50] " + d.total_notes + " \u97F3\u7B26, \u7EA6" + (d.title || "") + " " + d.estimated_duration);
      });
      e.addEventListener("music_progress", function(v) {
        var d = JSON.parse(v.data);
        muCurrent = d.current;
        muTotal = d.total;
        var pct = d.total > 0 ? Math.round(d.current / d.total * 100) : 0;
        var bar = document.getElementById("mu-bar");
        if (bar) bar.style.width = pct + "%";
        var stat = document.getElementById("mu-stat");
        if (stat) {
          var name = muPath ? muPath.replace(/\.[^.]+$/, "") : "\u672A\u77E5";
          var extra = "";
          if (muQueueActive && muQueueIndex >= 0 && muQueue[muQueueIndex]) {
            name = muQueue[muQueueIndex].title || name;
            extra = '<span style="margin-left:6px;color:var(--phx-primary);font-size:10px">' + ICONS.musicNote + " " + (muQueueIndex + 1) + "/" + muQueue.length + "</span>";
          }
          ;
          stat.innerHTML = '<div style="font-weight:600;font-size:13px">' + escHtml(name) + '</div><div style="font-size:11px;color:var(--phx-text-secondary);margin-top:2px">' + pct + "%" + extra + "</div>";
        }
      });
      e.addEventListener("music_done", function(v) {
        var _a;
        addL("[\u97F3\u4E50] \u64AD\u653E\u5B8C\u6210", "o");
        if (muQueueActive) {
          muQueueActive = false;
          muQueueIndex = -1;
          renderQueueUI();
          (_a = document.getElementById("mu-queue-controls")) == null ? void 0 : _a.classList.add("gone");
        } else {
          resetMusicUI();
        }
        ;
        T("\u64AD\u653E\u5B8C\u6210", "o");
      });
      e.addEventListener("queue_update", function(v) {
        var d = JSON.parse(v.data);
        if (!d) return;
        muQueue = d.queue || [];
        muQueueIndex = d.current != null ? d.current : -1;
        muQueueActive = d.active || false;
        muLoopMode = d.loop_mode || "none";
        updateLoopBtn();
        renderQueueUI();
        var qc = document.getElementById("mu-queue-controls");
        if (qc) qc.classList.toggle("gone", !muQueue.length);
        var qcard = document.getElementById("mu-queue-card");
        if (qcard) qcard.classList.toggle("gone", !muQueue.length);
      });
      e.addEventListener("track_change", function(v) {
        var d = JSON.parse(v.data);
        if (!d) return;
        muQueueIndex = d.index;
        muPath = d.cache_key;
        muPlaying = true;
        muPaused = false;
        muCurrent = 0;
        renderQueueUI();
        setPlayingUI();
        var stat = document.getElementById("mu-stat");
        if (stat) {
          var extra = '<span style="margin-left:6px;color:var(--phx-primary);font-size:10px">' + ICONS.musicNote + " " + (d.index + 1) + "/" + d.total + "</span>";
          stat.innerHTML = '<div style="font-weight:600;font-size:13px">' + escHtml(d.title || "") + '</div><div style="font-size:11px;color:var(--phx-text-secondary);margin-top:2px">0%' + extra + "</div>";
        }
        ;
        addL("[\u97F3\u4E50] \u64AD\u653E: " + escHtml(d.title || d.cache_key), "o");
      });
      e.addEventListener("version_check", function(v) {
        var d = JSON.parse(v.data);
        handleVersionCheck(d);
      });
      e.addEventListener("announcements", function(v) {
        if (window.wzActive) return;
        var d = JSON.parse(v.data);
        if (d.announcements) showAnnouncements(d.announcements);
      });
      e.addEventListener("notify", function(v) {
        var d = JSON.parse(v.data);
        if (typeof renderNotification === "function") renderNotification(d);
      });
      e.addEventListener("integrity_alert", function(v) {
        var d = JSON.parse(v.data);
        if (!d.passed) showIntegrityAlert(d);
      });
      e.addEventListener("pos", function(v) {
        var d = JSON.parse(v.data);
        flyState.x = d.x;
        flyState.y = d.y;
        flyState.z = d.z;
        flyState.pitch = d.pitch;
        flyState.yaw = d.yaw;
        flyUpdateDisplay();
      });
      e.addEventListener("draw_update", function(v) {
        var d = JSON.parse(v.data);
        if (typeof onDrawUpdate === "function") onDrawUpdate(d);
      });
      e.addEventListener("draw_plane", function(v) {
        var d = JSON.parse(v.data);
        if (typeof onDrawPlane === "function") onDrawPlane(d);
      });
      e.addEventListener("draw_busy", function(v) {
        var d = JSON.parse(v.data);
        if (typeof onDrawBusy === "function") onDrawBusy(d);
      });
      e.addEventListener("draw_error", function(v) {
        var d = JSON.parse(v.data);
        if (typeof onDrawError === "function") onDrawError(d);
      });
      e.addEventListener("draw_sync", function(v) {
        var d = JSON.parse(v.data);
        if (typeof onDrawSync === "function") onDrawSync(d);
      });
    };
    var rfshHomeThrottled = rfshHomeThrottled2, rfshTasks = rfshTasks2, rfshTasksThrottled = rfshTasksThrottled2, c = c2;
    var e = null, t = null;
    var lastTskRef = 0, lastHomeRef = 0;
  } catch (e2) {
  }
  c();
  // 事件帧并不总是带 msg：不同来源的后端帧字段不一致，插件也可能只发进度数字。
  // 以前直接 addL(d.msg) 会把 undefined 存进 allLogs，终端里就成了「事件」标签 + 空白内容。
  // 这里统一取文本字段；取不到文本就丢弃这一帧，不再让空白行留在列表里。
  function yySSELogText(value) {
    if (value == null) return "";
    if (typeof value === "object") {
      var keys = ["msg", "message", "text", "line", "error", "detail"];
      for (var i = 0; i < keys.length; i++) {
        if (value[keys[i]] != null) {
          value = value[keys[i]];
          break;
        }
      }
    }
    if (value == null || typeof value === "object") return "";
    return String(value).trim();
  }
  function addL(m, cl) {
    var text = yySSELogText(m);
    if (!text) return;
    m = text;
    var console = document.getElementById("console-log");
    if (console && console.offsetParent !== null) {
      addConsoleLine(m, cl);
      return;
    }
    allLogs.push({ msg: m, cl });
    if (allLogs.length > 2e3) {
      var removed = allLogs.length - 2e3;
      allLogs.splice(0, removed);
      if (typeof _consoleRendered !== "undefined") _consoleRendered = Math.max(0, _consoleRendered - removed);
    }
    var ids = ["imp-log", "exp-log", "ma-log", "mu-log", "md-log", "conn-log"];
    var l = null;
    for (var i = 0; i < ids.length; i++) {
      var el = document.getElementById(ids[i]);
      if (el && el.offsetParent !== null) {
        l = el;
        break;
      }
    }
    if (!l) return;
    var follow = l.scrollHeight - l.clientHeight - l.scrollTop < 64;
    var d = document.createElement("div");
    d.innerHTML = renderLogLine(m);
    if (cl) d.className = cl === "o" ? "cl-ok" : cl === "e" ? "cl-err" : "";
    l.appendChild(d);
    while (l.children.length > 2e3) l.removeChild(l.firstChild);
    if (follow) l.scrollTop = l.scrollHeight;
  }
})();
