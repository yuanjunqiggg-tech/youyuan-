var curTab = "home", tabInited = {}, tabHistory = ["home"];
var TAB_ORDER = ["home", "tasks", "terminal", "settings"];
var activeRenderers = [];
var _savedFormState = {};
var _voxelData = {};
var cfgSpeed = 9500, cfgNewProtocol = false, cfgImportCommands = true, cfgCmdDisabled = false, cfgExcludeFluids = false, cfgExcludeWater = true, cfgExcludeWaterlogged = true, cfgExcludeLava = true, cfgGravity = false, cfgPlatform = false;
var cfgToken = "";
var cfgRegionMode = "1";
var cfgBotTokens = [];
var subPageOpen = false;
var subPageCache = {};
var subPageStack = [];
// 子页初始化函数表：页面 HTML 只是骨架，真正的内容靠这些函数填充。
// 缓存命中、以及"返回上一页"复用的都是骨架 HTML，所以必须再跑一次初始化，
// 否则用户看到的是一张只有标题的空壳（历史上的"点进去没内容"就是这个）。
var subPageInits = {};
function subPageInit(title, fn) {
  if (title && typeof fn === "function") subPageInits[title] = fn;
}
// 把当前子页的实时 DOM 写回缓存：缓存里存的应该是"渲染后"的页面，不是骨架。
// .viewer 是 three.js 容器，克隆出来没有 WebGL 上下文，必须剔除；
// canvas 要保留——cloneNode 本来就不复制像素，而各页面的初始化函数
// （flyInitCanvases / drawRestore 等）恢复后会照着元素重新绘制。
// 按 class 清理而不是按 id：各场景的容器 id 已经分开
// （viewer-app / viewer-building / viewer-mapart），只删第一个会留下旧画布。
function yySnapshotSubPage(key) {
  if (!key) return;
  var sp = document.getElementById("sub-page");
  if (!sp) return;
  var header = sp.querySelector(".sp-header");
  var body = sp.querySelector(".sp-body");
  if (!header || !body) return;
  var clone = body.cloneNode(true);
  clone.querySelectorAll(".viewer").forEach(function(el) {
    el.remove();
  });
  subPageCache[key] = header.outerHTML + clone.outerHTML;
}
// 只在这一页仍然显示在栈顶时补跑初始化，避免迟到的定时器写到别的页面里。
function yyRunSubPageInit(key) {
  var fn = subPageInits[key];
  if (!fn) return;
  setTimeout(function() {
    if (!subPageOpen) return;
    if (subPageStack[subPageStack.length - 1] !== key) return;
    try {
      fn(key);
    } catch (e) {
      console.warn("\u6E38\u56ED\uFF1A\u5B50\u9875\u521D\u59CB\u5316\u5931\u8D25", key, e);
    }
  }, 30);
}
var yySubPageEpoch = 0;
function yyRestoreSubPageLater(key, epoch) {
  function live() {
    return epoch === yySubPageEpoch && subPageOpen && subPageStack[subPageStack.length - 1] === key;
  }
  setTimeout(function() {
    if (live()) restoreSubPageState(key);
  }, 20);
  setTimeout(function() {
    if (live()) runSubPageRestore(key);
  }, 60);
}
function runSubPageRestore(key) {
  (window.__subPageRestoreHooks || []).forEach(function(h) {
    if (h.key === "*" || h.key === key) {
      try {
        h.fn(key);
      } catch (e) {
      }
    }
  });
  if (window.onSubPageRestored) {
    try {
      window.onSubPageRestored(key);
    } catch (e) {
    }
  }
}
var cfgPreClear = 0;
(async function preloadCfg() {
  try {
    var c = await A("GET", "/api/config");
    if (c.ok && c.config) {
      cfgToken = c.config.token || "";
      cfgSpeed = c.config.import_speed || 9500;
      cfgNewProtocol = c.config.use_new_protocol || false;
      cfgImportCommands = c.config.import_commands !== false;
      cfgCmdDisabled = c.config.cmd_disabled || false;
      cfgExcludeFluids = c.config.exclude_fluids || false;
      cfgExcludeWater = c.config.exclude_water !== false;
      cfgExcludeWaterlogged = c.config.exclude_waterlogged !== false;
      cfgExcludeLava = c.config.exclude_lava !== false;
      cfgGravity = c.config.use_gravity_blocks || false;
      cfgPlatform = c.config.gravity_platform || false;
      cfgPreClear = c.config.pre_clear_mode || 0;
      cfgRegionMode = c.config.region_mode || "1";
      cfgBotTokens = c.config.bot_tokens || [];
    }
  } catch (e) {
  }
})();
setTimeout(function() {
  A("POST", "/api/version/check", {}).then(function(r) {
    if (r.ok) handleVersionCheck(r);
  }).catch(function() {
  });
}, 3e3);
function openSubPage(title, htmlContent) {
  if (typeof yyNavigateEpoch === "number") yyNavigateEpoch++;
  var epoch = ++yySubPageEpoch;
  var cacheKey = title;
  activeRenderers.forEach(function(kill) {
    try {
      kill();
    } catch (e) {
    }
  });
  activeRenderers = [];
  var sp = document.getElementById("sub-page");
  document.body.classList.add("sp-open");
  document.getElementById("bottom-nav").classList.add("gone");
  document.getElementById("conn-bar").classList.add("gone");
  // 切走之前，先把当前这一页的"渲染结果"存回缓存。
  // 以前只在新开页时把骨架写进缓存，导致 A → B → 返回 A 时 A 是空壳。
  var leaving = subPageOpen && subPageStack.length ? subPageStack[subPageStack.length - 1] : "";
  if (leaving && leaving !== cacheKey) yySnapshotSubPage(leaving);
  if (subPageCache[cacheKey]) {
    sp.innerHTML = subPageCache[cacheKey];
    yyRunSubPageInit(cacheKey);
    yyRestoreSubPageLater(cacheKey, epoch);
  } else {
    var headerHtml = '<div class="sp-header"><button class="sp-back" onclick="closeSubPage()">' + ICONS.arrowLeft + '</button><span class="sp-title">' + title + "</span></div>";
    subPageCache[cacheKey] = headerHtml + '<div class="sp-body">' + htmlContent + "</div>";
    sp.innerHTML = subPageCache[cacheKey];
  }
  sp.className = "sub-page show";
  subPageOpen = true;
  if (subPageStack[subPageStack.length - 1] !== cacheKey) subPageStack.push(cacheKey);
}
function closeSubPage() {
  var _a;
  var epoch = ++yySubPageEpoch;
  if (typeof yyWorkspaceOpenEpoch !== "undefined") yyWorkspaceOpenEpoch++;
  (_a = document.getElementById("yy-agent-profile-modal")) == null ? void 0 : _a.remove();
  activeRenderers.forEach(function(kill) {
    try {
      kill();
    } catch (e) {
    }
  });
  activeRenderers = [];
  try {
    var titleEl = document.querySelector(".sp-title");
    if (titleEl) {
      var key = titleEl.textContent;
      var body = document.querySelector("#sub-page .sp-body");
      if (body) {
        _savedFormState[key] = serializeForm(body);
        yySnapshotSubPage(key);
      }
    }
  } catch (error) {
    console.warn("\u6E38\u56ED\uFF1A\u4FDD\u5B58\u9875\u9762\u72B6\u6001\u5931\u8D25\uFF0C\u4ECD\u5141\u8BB8\u8FD4\u56DE", error);
  }
  if (subPageStack.length > 0) subPageStack.pop();
  if (subPageStack.length > 0) {
    var prevKey = subPageStack[subPageStack.length - 1];
    var sp2 = document.getElementById("sub-page");
    if (subPageCache[prevKey]) {
      sp2.innerHTML = subPageCache[prevKey];
      sp2.className = "sub-page show";
      yyRunSubPageInit(prevKey);
      yyRestoreSubPageLater(prevKey, epoch);
      return;
    }
    subPageStack = [];
  }
  var sp = document.getElementById("sub-page");
  sp.classList.remove("show");
  sp.classList.add("sp-closing");
  setTimeout(function() {
    if (epoch !== yySubPageEpoch || sp.classList.contains("show")) return;
    sp.className = "sub-page";
    document.body.classList.remove("sp-open");
    document.getElementById("bottom-nav").classList.remove("gone");
    document.getElementById("conn-bar").classList.remove("gone");
    subPageOpen = false;
    activeRenderers.forEach(function(kill) {
      try {
        kill();
      } catch (e) {
      }
    });
    activeRenderers = [];
    var keys = Object.keys(subPageCache);
    if (keys.length > 10) {
      var keep = keys.slice(-10);
      Object.keys(subPageCache).forEach(function(k) {
        if (keep.indexOf(k) < 0) delete subPageCache[k];
      });
      Object.keys(_savedFormState).forEach(function(k) {
        if (keep.indexOf(k) < 0) delete _savedFormState[k];
      });
      Object.keys(_voxelData).forEach(function(k) {
        if (keep.indexOf(k) < 0) delete _voxelData[k];
      });
    }
    if (curTab === "home") refreshHomeTasks();
  }, 290);
}
function serializeForm(root) {
  var st = {};
  root.querySelectorAll("input,select,textarea").forEach(function(el) {
    if (!el.id) return;
    if (el.type === "checkbox") st[el.id] = el.checked;
    else st[el.id] = el.value;
  });
  return st;
}
function restoreSubPageState(key) {
  var sv = _savedFormState[key];
  if (sv) {
    Object.keys(sv).forEach(function(id) {
      var el = document.getElementById(id);
      if (!el) return;
      if (el.type === "checkbox") el.checked = sv[id];
      else el.value = sv[id];
    });
    try {
      toggleSub("imp-cmds", "imp-cmd-sub");
      toggleSub("imp-exclude-fluids", "imp-fluid-sub");
      toggleDimInput();
      toggleGravitySub();
      toggleMaRelief();
    } catch (e) {
    }
  }
  var vd = _voxelData[key];
  if (vd && typeof show3D === "function") {
    // 只在当前子页的预览区里找容器；id 各不相同，所以按容器范围查询，
    // 不能用 getElementById("viewer") —— 那会命中别的场景残留的旧画布。
    var vw = document.querySelector("#imp-info .viewer, #ma-info .viewer, #skin-preview-area .viewer");
    if (!vw) {
      var info = document.getElementById("imp-info") || document.getElementById("ma-info") || document.getElementById("skin-preview-area");
      if (info) {
        vw = document.createElement("div");
        vw.className = "viewer";
        vw.id = "viewer-app";
        info.appendChild(vw);
      }
    }
    if (vw) {
      vw.innerHTML = "";
      requestAnimationFrame(function() {
        if (typeof THREE !== "undefined") show3D(vw, vd);
      });
    }
  }
  if (window._currentTask) {
    var ct = window._currentTask;
    var t2p = { import: "imp", export: "exp", mapart: "md", skin: "skin" };
    var pr = t2p[ct.type];
    if (pr) {
      var go = document.getElementById(pr + "-go");
      var stop = document.getElementById(pr + "-stop");
      var pg = document.getElementById(pr + "-pg");
      var lc = document.getElementById(pr + "-log-card");
      if (go) go.classList.add("gone");
      if (stop) stop.classList.remove("gone");
      if (pg) pg.classList.remove("gone");
      if (lc) lc.classList.remove("gone");
      if (ct.progress > 0) {
        var bar = document.getElementById(pr + "-bar");
        if (bar) bar.style.width = ct.progress * 100 + "%";
      }
    }
  }
  if (document.getElementById("mq-text") && typeof restoreMarqueeState === "function") {
    restoreMarqueeState();
  }
  if (document.getElementById("mcfn-wrap") && typeof restoreMcfunctionState === "function") {
    restoreMcfunctionState();
  }
}
function refreshHomeTasks() {
  var section = document.querySelector("#tab-home .tsk-section");
  if (!section) return;
  A("GET", "/api/task/list").then(function(t) {
    if (!t.ok || !t.tasks) return;
    var h = '<div class="tsk-header">\u4EFB\u52A1\u5217\u8868</div>';
    for (var ti = 0; ti < t.tasks.length; ti++) {
      var tk = t.tasks[ti];
      var tIcon = { "import": '<svg width="28" height="28" viewBox="0 0 22 22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>', "export": '<svg width="28" height="28" viewBox="0 0 22 22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/><rect x="3" y="3" width="16" height="6" rx="1"/></svg>', "mapart": '<svg width="28" height="28" viewBox="0 0 22 22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="16" height="16" rx="2"/><line x1="3" y1="9" x2="19" y2="9"/><line x1="9" y1="3" x2="9" y2="19"/></svg>', "skin": '<svg width="28" height="28" viewBox="0 0 22 22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>' }[tk.type] || "\xB7";
      var tSt = tk.status || "unknown";
      var tCl = tSt === "running" ? "running" : tSt === "done" ? "done" : tSt === "failed" ? "failed" : "paused";
      var tLb = tSt === "running" ? "\u8FD0\u884C\u4E2D" : tSt === "done" ? "\u5B8C\u6210" : tSt === "failed" ? "\u5931\u8D25" : "\u6682\u505C";
      var tName = tk.name || tk.type || "\u672A\u77E5";
      var tMeta = tk.message || "";
      if (tk.progress != null) tMeta = "\u8FDB\u5EA6: " + (tk.progress * 100).toFixed(0) + "%";
      h += '<div class="tsk-item"><div class="tsk-icon">' + tIcon + '</div><div class="tsk-body"><div class="tsk-name">' + escHtml(tName) + '</div><div class="tsk-meta">' + escHtml(tMeta) + '</div></div><span class="tsk-status ' + tCl + '">' + tLb + "</span>";
      if (tSt === "running" || tSt === "paused") {
        h += `<button class="tsk-act" onclick="resumeTask('` + tk.task_id + `')">\u6062\u590D</button>`;
      }
      h += `<button class="tsk-act" onclick="delHomeTask('` + tk.task_id + `')">\u5220\u9664</button>`;
      h += "</div>";
    }
    var s = document.querySelector("#tab-home .tsk-section");
    if (s) {
      var hdr = s.querySelector(".tsk-header");
      if (hdr) hdr.outerHTML = '<div class="tsk-header">\u4EFB\u52A1\u5217\u8868</div>';
      s.innerHTML = h;
    }
  });
}
function resumeTask(id) {
  A("POST", "/api/task/resume", { task_id: id }).then(function(r) {
    if (r.ok) {
      T("\u4EFB\u52A1\u5DF2\u6062\u590D", "o");
      refreshHomeTasks();
    } else T("\u6062\u590D\u5931\u8D25: " + r.error, "e");
  });
}
async function delHomeTask(id) {
  await A("POST", "/api/task/delete", { task_id: id });
  refreshHomeTasks();
}
var _navDrag = {
  active: false,
  startX: 0,
  lastX: 0,
  startCenter: 0,
  moved: false,
  centers: [],
  vel: 0,
  // 拖拽速度(px/事件)
  sx: 1,
  sy: 1,
  // 当前透镜 scale
  tsx: 1,
  tsy: 1,
  // 目标 scale
  raf: null
};
var _glideBaseScale = 1;
var _glideLens = null;
function tabCenters() {
  var nav = document.getElementById("bottom-nav");
  var cs = [];
  if (nav) nav.querySelectorAll("button").forEach(function(b) {
    cs.push(b.offsetLeft + b.offsetWidth / 2);
  });
  return cs;
}
function syncGlideSize() {
  var glide = document.getElementById("nav-glide");
  var btn = document.querySelector(".nav button.active") || document.querySelector(".nav button");
  if (!glide || !btn) return;
  glide.style.width = btn.offsetWidth + "px";
  glide.style.height = btn.offsetHeight + "px";
}
function positionGlideAt(center, animate) {
  var glide = document.getElementById("nav-glide");
  if (!glide) return;
  if (!animate) glide.classList.add("no-anim");
  else glide.classList.remove("no-anim");
  glide.style.transform = "translateX(" + (center - glide.offsetWidth / 2) + "px)";
}
function updateNavGlide() {
  syncGlideSize();
  var btn = document.querySelector(".nav button.active");
  if (!btn) return;
  positionGlideAt(btn.offsetLeft + btn.offsetWidth / 2, true);
}
function applyGlideScale() {
  if (!_glideLens) return;
  _glideLens.style.transform = "scale(" + _navDrag.sx + "," + _navDrag.sy + ")";
}
function glideTick() {
  var d = _navDrag;
  var k = 0.28;
  d.sx += (d.tsx - d.sx) * k;
  d.sy += (d.tsy - d.sy) * k;
  applyGlideScale();
  if (Math.abs(d.sx - d.tsx) < 2e-3 && Math.abs(d.sy - d.tsy) < 2e-3 && !d.active) {
    d.sx = d.tsx;
    d.sy = d.tsy;
    applyGlideScale();
    d.raf = null;
    return;
  }
  d.raf = requestAnimationFrame(glideTick);
}
function startGlideRaf() {
  if (!_navDrag.raf) _navDrag.raf = requestAnimationFrame(glideTick);
}
function initNavDrag() {
  var nav = document.getElementById("bottom-nav");
  if (!nav || nav.dataset.dragInit) return;
  nav.dataset.dragInit = "1";
  var glide = document.getElementById("nav-glide");
  _glideLens = glide ? glide.querySelector(".nav-glide-lens") : null;
  nav.addEventListener("pointerdown", function(e) {
    _navDrag.active = true;
    _navDrag.moved = false;
    _navDrag.startX = e.clientX;
    _navDrag.lastX = e.clientX;
    _navDrag.vel = 0;
    _navDrag.centers = tabCenters();
    var btn = nav.querySelector("button.active");
    _navDrag.startCenter = btn ? btn.offsetLeft + btn.offsetWidth / 2 : _navDrag.centers[0] || 0;
    _navDrag.tsx = _navDrag.tsy = _glideBaseScale + 0.4;
    startGlideRaf();
  });
  nav.addEventListener("pointermove", function(e) {
    if (!_navDrag.active) return;
    var dx = e.clientX - _navDrag.lastX;
    if (Math.abs(e.clientX - _navDrag.startX) > 8) {
      if (!_navDrag.moved && nav.setPointerCapture) try {
        nav.setPointerCapture(e.pointerId);
      } catch (err) {
      }
      _navDrag.moved = true;
    }
    _navDrag.vel = dx;
    var total = e.clientX - _navDrag.startX;
    positionGlideAt(_navDrag.startCenter + total, false);
    var v = _navDrag.vel / 8;
    v = Math.max(-0.35, Math.min(0.35, v));
    _navDrag.tsx = (_glideBaseScale + 0.4) / (1 - v * 0.75);
    _navDrag.tsy = (_glideBaseScale + 0.4) * (1 - v * 0.25);
    _navDrag.lastX = e.clientX;
    startGlideRaf();
  });
  function endDrag(e) {
    if (!_navDrag.active) return;
    _navDrag.active = false;
    _navDrag.tsx = _navDrag.tsy = 1;
    if (_navDrag.moved) {
      var cs = _navDrag.centers || tabCenters();
      var x = (e.clientX || _navDrag.lastX) - nav.getBoundingClientRect().left;
      var idx = 0, best = 1e9;
      cs.forEach(function(c, i) {
        var d = Math.abs(c - x);
        if (d < best) {
          best = d;
          idx = i;
        }
      });
      var btn = nav.querySelectorAll("button")[idx];
      if (btn) {
        var t = btn.getAttribute("data-tab");
        if (t !== curTab) tabHistory.push(t);
        tabTo(t);
      }
    } else {
      updateNavGlide();
    }
    startGlideRaf();
  }
  nav.addEventListener("pointerup", endDrag);
  nav.addEventListener("pointercancel", endDrag);
}
initNavDrag();
document.querySelectorAll(".nav button").forEach(function(b) {
  b.addEventListener("click", function() {
    if (_navDrag.moved) {
      _navDrag.moved = false;
      return;
    }
    var t = b.getAttribute("data-tab");
    if (t !== curTab) tabHistory.push(t);
    tabTo(t);
  });
});
function tabTo(t) {
  if (typeof yyNavigateEpoch === "number") yyNavigateEpoch++;
  var next = document.getElementById("tab-" + t);
  if (!next) return;
  var forward = TAB_ORDER.indexOf(t) > TAB_ORDER.indexOf(curTab), changed = t !== curTab;
  document.querySelectorAll(".content").forEach(function(panel) {
    panel.classList.remove("show", "tab-in-right", "tab-in-left", "tab-out-right", "tab-out-left");
    panel.setAttribute("aria-hidden", panel === next ? "false" : "true");
    panel.inert = panel !== next;
  });
  curTab = t;
  next.classList.add("show");
  if (changed) {
    void next.offsetWidth;
    next.classList.add(forward ? "tab-in-right" : "tab-in-left");
  }
  document.querySelectorAll(".nav button").forEach(function(b) {
    b.classList.toggle("active", b.getAttribute("data-tab") === t);
  });
  updateNavGlide();
  activeRenderers.forEach(function(kill) {
    try {
      kill();
    } catch (e) {
    }
  });
  activeRenderers = [];
  if (t !== "settings") {
    if (typeof _cfgRefreshTimer !== "undefined") {
      clearInterval(_cfgRefreshTimer);
      _cfgRefreshTimer = null;
    }
    if (typeof _toolboxPollTimer !== "undefined") {
      clearInterval(_toolboxPollTimer);
      _toolboxPollTimer = null;
    }
  }
  if (!tabInited[t]) {
    tabInited[t] = true;
    if (t === "home") Rhome();
    if (t === "terminal") Rconsole();
    if (t === "settings") Rcfg();
  }
  if (t === "tasks") Rplg();
}
window.__onBackPressed__ = function() {
  var modal = document.querySelector(".modal-overlay");
  if (modal) {
    modal.remove();
    return "true";
  }
  if (subPageOpen) {
    closeSubPage();
    return "true";
  }
  if (tabHistory.length > 1) {
    tabHistory.pop();
    tabTo(tabHistory[tabHistory.length - 1]);
    return "true";
  }
  return "exit";
};
document.addEventListener("click", function(e) {
  var el = e.target && e.target.closest ? e.target.closest('button, .qcard, .grid-card, .fp-item, .bs-option, .pick, .mode-cap-btn, .tsk-act, .sp-back, .cb-btn, a[href], [role="button"]') : null;
  if (el && !el.disabled && navigator && navigator.vibrate) {
    try {
      navigator.vibrate(12);
    } catch (err) {
    }
  }
});
window.addEventListener("resize", function() {
  updateNavGlide();
});
