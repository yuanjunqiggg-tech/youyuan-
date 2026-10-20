function previewCheckPermission() {
  return new Promise(function(resolve) {
    if (typeof android === "undefined") {
      resolve(true);
      return;
    }
    try {
      if (typeof android.checkStoragePermission === "function") {
        if (android.checkStoragePermission()) {
          resolve(true);
          return;
        }
      }
    } catch (e) {
      console.error("previewCheckPermission error:", e);
    }
    var overlay = document.createElement("div");
    overlay.className = "modal-overlay";
    overlay.innerHTML = '<div class="modal-box" style="text-align:center;max-width:340px"><div style="margin:0 auto 16px"><svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--phx-warning)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg></div><div style="font-size:15px;font-weight:800;margin-bottom:8px">\u9700\u8981\u7BA1\u7406\u6240\u6709\u6587\u4EF6\u6743\u9650</div><div style="font-size:12px;color:var(--phx-text-secondary);line-height:1.6;margin-bottom:16px">\u6D4F\u89C8\u548C\u9009\u62E9\u5B58\u50A8\u4E2D\u7684\u5EFA\u7B51\u6587\u4EF6\u9700\u8981\u6B64\u6743\u9650</div><div class="row"><button class="btn-s" onclick="var r=window.__previewPermissionResolve__;if(r){r(false)};closeModalOverlay(this)">\u53D6\u6D88</button><button class="btn" onclick="closeModalOverlay(this);requestPreviewPermission()">\u524D\u5F80\u8BBE\u7F6E</button></div></div>';
    document.body.appendChild(overlay);
    window.__previewPermissionResolve__ = resolve;
  });
}
function requestPreviewPermission() {
  if (typeof android !== "undefined" && android.requestStoragePermission) {
    android.requestStoragePermission();
  }
  var orig = window.__onStoragePermissionResult__;
  window.__onStoragePermissionResult__ = function(granted) {
    if (orig) orig(granted);
    if (window.__previewPermissionResolve__) {
      window.__previewPermissionResolve__(granted);
      window.__previewPermissionResolve__ = null;
    }
  };
}
var previewState = {
  step: 1,
  folder: "",
  recursive: false,
  files: [],
  filters: {},
  angles: [],
  output: {
    format: "png",
    width: 1024,
    height: 1024,
    mode: "folder",
    dir: "/sdcard/Download/\u6E38\u56ED\u9884\u89C8/",
    gif: false,
    saveConfig: false
  },
  generateTaskId: null
};
function webpSupported() {
  try {
    var c = document.createElement("canvas");
    c.width = 1;
    c.height = 1;
    var ctx = c.getContext("2d");
    return c.toDataURL("image/webp").indexOf("data:image/webp") === 0;
  } catch (e) {
    return false;
  }
}
function generatePreviewsForContribute(structPath, angles, opts, onComplete) {
  if (typeof onComplete !== "function") return;
  if (!webpSupported()) {
    onComplete({ error: "\u5F53\u524D\u8BBE\u5907\u4E0D\u652F\u6301 WebP \u7F16\u7801" });
    return;
  }
  opts = opts || {};
  var W = opts.width || 1024, H = opts.height || 1024;
  var quality = opts.quality == null ? 0.8 : opts.quality;
  var name = (structPath.split("/").pop() || "\u5EFA\u7B51").replace(/\.[^.]+$/, "") || "\u5EFA\u7B51";
  A("POST", "/api/building/voxel", { path: structPath }).then(function(r) {
    if (!r.ok || !r.voxel) {
      onComplete({ error: r.error || "\u4F53\u7D20\u52A0\u8F7D\u5931\u8D25" });
      return;
    }
    var voxel = r.voxel;
    var s = voxel.step || 1, sx = voxel.size_x || 10, sy = voxel.size_y || 10, sz = voxel.size_z || 10;
    var cx = sx / 2, cy = sy / 2, cz = sz / 2, m = Math.max(sx, sy, sz);
    var webps = [], nextAngle = 0;
    function renderNext() {
      if (nextAngle >= angles.length) {
        if (opts.gif) renderGif();
        else onComplete({ webps, gifFrames: null });
        return;
      }
      var angle = angles[nextAngle++];
      var scene = previewBuildScene(voxel, s, sx, sy, sz, cx, cy, cz, m);
      var renderer = new THREE.WebGLRenderer({ antialias: true, preserveDrawingBuffer: true });
      renderer.setSize(W, H);
      renderer.setPixelRatio(1);
      var yaw = 45, pitch = 30, dist = m * 1.8;
      if (angle.type === "custom") {
        yaw = angle.yaw || 45;
        pitch = angle.pitch || 30;
        dist = (angle.dist || 200) / 100 * m;
      } else {
        var am = {
          front: [0, 0],
          back: [180, 0],
          left: [270, 0],
          right: [90, 0],
          northeast: [45, 30],
          southeast: [135, 30],
          northwest: [315, 30],
          southwest: [225, 30],
          top: [0, 89],
          "0": [0, 30],
          "45": [45, 30],
          "90": [90, 30],
          "135": [135, 30],
          "180": [180, 30],
          "225": [225, 30],
          "270": [270, 30],
          "315": [315, 30]
        };
        var ap = am[angle.value] || [45, 30];
        yaw = ap[0];
        pitch = ap[1];
      }
      var ry = yaw * Math.PI / 180, rp = pitch * Math.PI / 180;
      var cam = new THREE.PerspectiveCamera(45, W / H, 1, 5e3);
      cam.position.set(cx + dist * Math.sin(ry) * Math.cos(rp), cy + dist * Math.sin(rp), cz + dist * Math.cos(ry) * Math.cos(rp));
      cam.lookAt(cx, cy, cz);
      renderer.render(scene, cam);
      canvasToWebp(renderer.domElement, W, H, quality, function(blob) {
        previewDisposeScene(scene);
        renderer.dispose();
        if (!blob) {
          onComplete({ error: "WebP \u751F\u6210\u5931\u8D25" });
          return;
        }
        webps.push({
          blob,
          width: W,
          height: H,
          name: name + "_" + sanitizeGenName(angle.label || "\u673A\u4F4D" + nextAngle) + ".webp",
          label: angle.label || "",
          angle
        });
        renderNext();
      });
    }
    function renderGif() {
      var gifW = opts.gifWidth || 512, gifH = opts.gifHeight || 512;
      var frames = [];
      var scene = previewBuildScene(voxel, s, sx, sy, sz, cx, cy, cz, m);
      var renderer = new THREE.WebGLRenderer({ antialias: true, preserveDrawingBuffer: true });
      renderer.setSize(gifW, gifH);
      renderer.setPixelRatio(1);
      var cam = new THREE.PerspectiveCamera(45, gifW / gifH, 1, 5e3);
      var canvas = renderer.domElement, dist = m * 1.8;
      for (var i = 0; i < 36; i++) {
        var yaw = i * 10, pitch = 30;
        var ry = yaw * Math.PI / 180, rp = pitch * Math.PI / 180;
        cam.position.set(cx + dist * Math.sin(ry) * Math.cos(rp), cy + dist * Math.sin(rp), cz + dist * Math.cos(ry) * Math.cos(rp));
        cam.lookAt(cx, cy, cz);
        renderer.render(scene, cam);
        frames.push(canvas.toDataURL("image/png"));
      }
      previewDisposeScene(scene);
      renderer.dispose();
      onComplete({ webps, gifFrames: frames });
    }
    renderNext();
  });
}
function canvasToWebp(canvas, W, H, quality, cb) {
  var c = document.createElement("canvas");
  c.width = W;
  c.height = H;
  c.getContext("2d").drawImage(canvas, 0, 0, W, H);
  c.toBlob(function(blob) {
    if (!blob || blob.type.indexOf("webp") === -1) {
      cb(null);
      return;
    }
    cb(blob);
  }, "image/webp", quality);
}
function RpreviewHTML() {
  setTimeout(function() {
    if (previewState._generating && previewGenState) {
      previewResumeGenerate();
      return;
    }
    var step = previewState.step || 1;
    renderPreviewStep(step);
  }, 50);
  return '<div id="preview-wrap" style="width:100%"><div id="preview-body"></div></div>';
}
function previewResumeGenerate() {
  var _a, _b;
  var body = document.getElementById("preview-body");
  if (!body) return;
  body.innerHTML = '<div class="wz-page active" style="position:relative;opacity:1;pointer-events:auto;transform:none;padding:0;align-items:stretch;width:100%"><div class="wz-section-title"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>\u6B63\u5728\u751F\u6210\u9884\u89C8\u56FE</div><div class="card" style="margin-top:8px"><div style="display:flex;justify-content:space-between;margin-bottom:6px"><span style="font-size:12px;font-weight:700" id="pv-progress-text">0%</span><span class="dim" style="font-size:10px" id="pv-progress-count">0 / 0</span></div><div class="pg"><div id="pv-progress-bar" style="width:0%"></div></div></div><div class="card" style="margin-top:8px"><div style="font-size:12px;font-weight:700;margin-bottom:6px">\u5F53\u524D\u5EFA\u7B51</div><div style="font-size:13px" id="pv-current-building">\u7B49\u5F85\u4E2D...</div><div class="dim" style="font-size:10px;margin-top:2px" id="pv-current-angle"></div></div><div class="card" style="margin-top:8px;padding:10px;display:none" id="pv-gif-status"><div style="font-size:10px;font-weight:700;color:var(--phx-text-secondary);margin-bottom:4px">GIF \u540E\u53F0\u5408\u6210</div><div id="pv-gif-status-text" style="font-size:10px;color:var(--phx-text-secondary);line-height:1.6"></div></div><div class="card" style="margin-top:8px;max-height:200px;overflow-y:auto;background:var(--phx-bg)"><div style="font-size:11px;font-weight:700;color:var(--phx-text-secondary);margin-bottom:4px">\u5904\u7406\u65E5\u5FD7</div><div id="pv-log" style="font-size:10px;font-family:monospace;line-height:1.6"></div></div><div class="row" style="margin-top:16px"><button class="btn-d" id="pv-cancel-btn" onclick="previewCancelGenerate()">\u53D6\u6D88</button></div></div>';
  updateProgressGen();
  if (previewGenState) {
    updateProgressBuilding(((_b = (_a = previewGenState.files[previewGenState.currentFileIdx]) == null ? void 0 : _a.name) == null ? void 0 : _b.replace(/\.[^.]+$/, "")) || "");
  }
}
async function previewOpen() {
  try {
    var ok = await previewCheckPermission();
    if (!ok) return;
    delete subPageCache["\u751F\u6210\u9884\u89C8\u56FE"];
    openSubPage("\u751F\u6210\u9884\u89C8\u56FE", RpreviewHTML());
  } catch (e) {
    console.error("previewOpen error:", e);
    T("\u6253\u5F00\u9884\u89C8\u5931\u8D25: " + e.message, "e");
  }
}
function renderPreviewStep(step) {
  previewState.step = step;
  var body = document.getElementById("preview-body");
  if (!body) return;
  switch (step) {
    case 1:
      renderStep1();
      break;
    case 2:
      renderStep2();
      break;
    case 3:
      renderStep3();
      break;
    case 4:
      renderStep4();
      break;
  }
}
function renderStep1() {
  var body = document.getElementById("preview-body");
  body.innerHTML = '<div class="wz-page active" style="position:relative;opacity:1;pointer-events:auto;transform:none;padding:0;align-items:stretch;width:100%"><div class="wz-section-title"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>\u9009\u62E9\u6587\u4EF6\u5939</div><div class="dim" style="margin-bottom:12px">\u9009\u62E9\u5305\u542B\u5EFA\u7B51\u6587\u4EF6\u7684\u6587\u4EF6\u5939\uFF0C\u7CFB\u7EDF\u5C06\u81EA\u52A8\u626B\u63CF</div><div class="pick" onclick="previewPickFolder()" style="margin:0"><div class="pi"><svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg></div><div id="pv-folder-txt">\u70B9\u51FB\u9009\u62E9\u6587\u4EF6\u5939</div></div><div id="pv-folder-info" class="gone" style="margin-top:8px"><div class="stats" style="grid-template-columns:1fr"><div class="s"><div class="n" id="pv-folder-path"></div><div class="l">\u5DF2\u9009\u8DEF\u5F84</div></div></div></div><div class="wz-dot-bar" style="padding:8px 0 12px"><span class="wz-dot active"></span><span class="wz-dot"></span><span class="wz-dot"></span><span class="wz-dot"></span></div><div class="row"><button class="btn-s" onclick="closeSubPage()">\u53D6\u6D88</button><button class="btn" id="pv-step1-next" onclick="previewStep1Next()" disabled>\u4E0B\u4E00\u6B65 \u2192</button></div></div>';
}
function previewPickFolder() {
  pickBySetting("building", function(path) {
    previewState.folder = path;
    updateFolderDisplay();
  }, "folder");
}
function updateFolderDisplay() {
  var txt = document.getElementById("pv-folder-txt");
  var info = document.getElementById("pv-folder-info");
  var path = document.getElementById("pv-folder-path");
  var next = document.getElementById("pv-step1-next");
  if (txt) txt.textContent = "\u5DF2\u9009\u62E9: " + previewState.folder.split("/").pop();
  if (info) info.classList.remove("gone");
  if (path) path.textContent = previewState.folder;
  if (next) next.disabled = false;
}
function previewStep1Next() {
  if (!previewState.folder) return;
  renderPreviewStep(2);
}
function renderStep2() {
  var body = document.getElementById("preview-body");
  body.innerHTML = '<div class="wz-page active" style="position:relative;opacity:1;pointer-events:auto;transform:none;padding:0;align-items:stretch;width:100%"><div class="wz-topbar"><button onclick="renderPreviewStep(1)"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/></svg> \u8FD4\u56DE</button></div><div class="wz-section-title"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/></svg>\u626B\u63CF\u9009\u9879</div><div class="card" style="padding:12px 14px;margin-bottom:10px;font-size:12px;font-weight:700;overflow:hidden;text-overflow:ellipsis;white-space:nowrap"><span style="color:var(--phx-primary)">' + previewState.folder.split("/").pop() + '</span><div class="dim" style="font-size:10px;margin-top:2px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + previewState.folder + '</div></div><div id="pv-filter-list"></div><div id="pv-scan-result" class="card" style="text-align:center;margin-top:12px"><div class="n" style="font-size:18px;font-weight:800;color:var(--phx-primary)" id="pv-file-count">0</div><div class="l" style="font-size:11px;color:var(--phx-text-secondary)">\u4E2A\u5EFA\u7B51\u6587\u4EF6</div></div><div id="pv-file-list" style="margin-top:8px;max-height:300px;overflow-y:auto"></div><div class="wz-dot-bar" style="padding:8px 0 12px"><span class="wz-dot"></span><span class="wz-dot active"></span><span class="wz-dot"></span><span class="wz-dot"></span></div><div class="row"><button class="btn-s" onclick="renderPreviewStep(1)">\u4E0A\u4E00\u6B65</button><button class="btn" onclick="renderPreviewStep(3)">\u4E0B\u4E00\u6B65 \u2192</button></div></div>';
  renderFilterList();
}
var previewFilters = [
  { id: "scanMode", label: "\u626B\u63CF\u65B9\u5F0F", summary: "\u9012\u5F52\u626B\u63CF", type: "single" },
  { id: "nameInclude", label: "\u6587\u4EF6\u540D\u5305\u542B", summary: "\u672A\u8BBE\u7F6E", type: "text" },
  { id: "nameExclude", label: "\u6587\u4EF6\u540D\u4E0D\u5305\u542B", summary: "\u672A\u8BBE\u7F6E", type: "text" },
  { id: "langFilter", label: "\u8BED\u8A00\u8FC7\u6EE4", summary: "\u4E0D\u9650\u5236", type: "single" },
  { id: "sizeRange", label: "\u6587\u4EF6\u5927\u5C0F", summary: "\u4E0D\u9650\u5236", type: "range" },
  { id: "formatFilter", label: "\u6587\u4EF6\u683C\u5F0F", summary: "\u5168\u90E8", type: "multi" }
];
function renderFilterList() {
  var list = document.getElementById("pv-filter-list");
  if (!list) return;
  var html = "";
  for (var i = 0; i < previewFilters.length; i++) {
    var f = previewFilters[i];
    var summary = getFilterSummary(f.id);
    html += `<div class="card" style="cursor:pointer;padding:14px 16px;display:flex;align-items:center;justify-content:space-between;margin-bottom:8px" onclick="previewOpenFilter('` + f.id + `')"><div><div style="font-size:13px;font-weight:700">` + f.label + '</div><div class="dim" style="font-size:10px;margin-top:2px" id="pv-filter-summary-' + f.id + '">' + summary + '</div></div><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--phx-text-disabled)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"/></svg></div>';
  }
  list.innerHTML = html;
  previewDoScan();
}
function getFilterSummary(id) {
  switch (id) {
    case "scanMode":
      return previewState.recursive ? "\u9012\u5F52\u626B\u63CF" : "\u53EA\u626B\u5F53\u524D";
    case "nameInclude": {
      var kw = previewState.filters.name_include || [];
      return kw.length ? kw.join(", ") : "\u672A\u8BBE\u7F6E";
    }
    case "nameExclude": {
      var kw = previewState.filters.name_exclude || [];
      return kw.length ? kw.join(", ") : "\u672A\u8BBE\u7F6E";
    }
    case "langFilter": {
      var labels = { none: "\u4E0D\u9650\u5236", chinese_only: "\u5FC5\u987B\u542B\u4E2D\u6587", no_chinese: "\u5FC5\u987B\u4E0D\u542B\u4E2D\u6587" };
      return labels[previewState.filters.lang_filter] || "\u4E0D\u9650\u5236";
    }
    case "sizeRange": {
      var min = previewState.filters.size_min || 0;
      var max = previewState.filters.size_max || 0;
      if (!min && !max) return "\u4E0D\u9650\u5236";
      var parts = [];
      if (min) parts.push(min / 1024 + (min >= 1048576 ? "MB" : "KB"));
      if (min && max) parts.push("\uFF5E");
      if (max) parts.push(max / 1024 + (max >= 1048576 ? "MB" : "KB"));
      return parts.join(" ") || "\u4E0D\u9650\u5236";
    }
    case "formatFilter": {
      var fmts = previewState.filters.formats || [];
      if (!fmts.length) return "\u5168\u90E8";
      var labels = { ".mcstructure": "MCStructure", ".schematic": "Schematic", ".schem": "Schem", ".mcworld": "MCWorld", ".bdx": "BDX" };
      return fmts.map(function(f) {
        return labels[f] || f;
      }).join(", ");
    }
    default:
      return "\u672A\u8BBE\u7F6E";
  }
}
function previewDoScan() {
  var body = {
    path: previewState.folder,
    recursive: previewState.recursive,
    filters: previewState.filters
  };
  A("POST", "/api/preview/scan", body).then(function(r) {
    if (r.ok) {
      previewState.files = r.files || [];
      var count = document.getElementById("pv-file-count");
      if (count) count.textContent = previewState.files.length;
      renderFileList();
    }
  });
}
function renderFileList() {
  var list = document.getElementById("pv-file-list");
  if (!list) return;
  var files = previewState.files || [];
  if (!files.length) {
    list.innerHTML = '<div class="dim" style="text-align:center;padding:12px">\u65E0\u5339\u914D\u6587\u4EF6</div>';
    return;
  }
  var html = '<div style="font-size:11px;font-weight:700;color:var(--phx-text-secondary);margin-bottom:6px">\u5DF2\u9009\u6587\u4EF6\uFF08\u70B9\u51FB\u53EF\u79FB\u9664\uFF09</div>';
  for (var i = 0; i < files.length; i++) {
    var f = files[i];
    var name = f.name || f.path.split("/").pop() || "";
    var extLabel = { ".mcstructure": "MC", ".schematic": "SC", ".schem": "SC", ".mcworld": "WLD", ".bdx": "BDX" }[f.format] || (f.format || "").replace(".", "").substring(0, 3).toUpperCase();
    html += '<div class="fp-item" style="cursor:pointer;overflow:hidden" data-idx="' + i + '"><div class="fp-icon fp-icon-' + (f.format || "file").replace(".", "") + '" style="font-size:9px;width:24px;height:24px;border-radius:4px;flex-shrink:0">' + extLabel + '</div><div class="fp-info" style="min-width:0;flex:1"><div class="fp-name" style="font-size:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + name + '</div><div class="fp-meta">' + (f.size ? formatSize(f.size) : "") + '</div></div><button class="q-item-del" style="flex-shrink:0" onclick="previewRemoveFile(' + i + ')"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button></div>';
  }
  list.innerHTML = html;
}
function previewRemoveFile(idx) {
  previewState.files.splice(idx, 1);
  var count = document.getElementById("pv-file-count");
  if (count) count.textContent = previewState.files.length;
  renderFileList();
}
function previewOpenFilter(id) {
  switch (id) {
    case "scanMode":
      previewFilterSheetScanMode();
      break;
    case "nameInclude":
      previewFilterSheetText("nameInclude", "\u6587\u4EF6\u540D\u5305\u542B", "\u8F93\u5165\u5173\u952E\u8BCD\uFF0C\u591A\u4E2A\u7528\u9017\u53F7\u5206\u9694", previewState.filters.name_include || []);
      break;
    case "nameExclude":
      previewFilterSheetText("nameExclude", "\u6587\u4EF6\u540D\u4E0D\u5305\u542B", "\u8F93\u5165\u5173\u952E\u8BCD\uFF0C\u591A\u4E2A\u7528\u9017\u53F7\u5206\u9694", previewState.filters.name_exclude || []);
      break;
    case "langFilter":
      previewFilterSheetLang();
      break;
    case "sizeRange":
      previewFilterSheetSize();
      break;
    case "formatFilter":
      previewFilterSheetFormats();
      break;
  }
}
function previewFilterSheetScanMode() {
  var overlay = document.createElement("div");
  overlay.className = "bs-overlay";
  overlay.innerHTML = '<div class="bs-sheet"><div class="bs-title">\u626B\u63CF\u65B9\u5F0F</div><div class="bs-body"><div class="bs-option" data-value="current"><div class="bs-dot"></div><span class="bs-label">\u53EA\u626B\u5F53\u524D\u6587\u4EF6\u5939</span><span class="dim" style="font-size:10px">\u4E0D\u8FDB\u5165\u5B50\u6587\u4EF6\u5939</span></div><div class="bs-option" data-value="recursive"><div class="bs-dot"></div><span class="bs-label">\u9012\u5F52\u626B\u63CF\u5168\u90E8\u5B50\u6587\u4EF6\u5939</span><span class="dim" style="font-size:10px">\u626B\u63CF\u6240\u6709\u5B50\u6587\u4EF6\u5939</span></div></div><div class="bs-footer"><button class="bs-btn bs-btn-cancel" onclick="closeBsOverlay(this)">\u53D6\u6D88</button><button class="bs-btn bs-btn-confirm" onclick="closeBsOverlay(this)">\u786E\u5B9A</button></div></div>';
  document.body.appendChild(overlay);
  var sel = previewState.recursive ? "recursive" : "current";
  overlay.querySelectorAll(".bs-option").forEach(function(o) {
    if (o.dataset.value === sel) o.classList.add("active");
    o.addEventListener("click", function() {
      overlay.querySelectorAll(".bs-option").forEach(function(x) {
        x.classList.remove("active");
      });
      this.classList.add("active");
      previewState.recursive = this.dataset.value === "recursive";
      previewFilterUpdateSummary("scanMode", this.dataset.value === "recursive" ? "\u9012\u5F52\u626B\u63CF" : "\u53EA\u626B\u5F53\u524D");
      previewDoScan();
    });
  });
}
function previewFilterSheetText(id, title, placeholder, current) {
  var overlay = document.createElement("div");
  overlay.className = "bs-overlay";
  overlay.innerHTML = '<div class="bs-sheet"><div class="bs-title">' + title + '</div><div class="bs-body" style="padding:8px 18px"><div class="dim" style="font-size:11px;margin-bottom:8px">' + placeholder + '</div><input id="pv-filter-input-' + id + '" type="text" placeholder="\u5173\u952E\u8BCD1, \u5173\u952E\u8BCD2, ..." style="margin:0" value="' + (current && current.length ? current.join(", ") : "") + `"></div><div class="bs-footer"><button class="bs-btn bs-btn-cancel" onclick="closeBsOverlay(this)">\u53D6\u6D88</button><button class="bs-btn bs-btn-confirm" onclick="previewFilterTextConfirm('` + id + `')">\u786E\u5B9A</button></div></div>`;
  document.body.appendChild(overlay);
}
function previewFilterTextConfirm(id) {
  var input = document.getElementById("pv-filter-input-" + id);
  if (!input) return;
  var val = input.value.trim();
  var keywords = val ? val.split(",").map(function(s) {
    return s.trim();
  }).filter(function(s) {
    return s;
  }) : [];
  if (id === "nameInclude") {
    previewState.filters.name_include = keywords;
  } else if (id === "nameExclude") {
    previewState.filters.name_exclude = keywords;
  }
  previewFilterUpdateSummary(id, keywords.length ? keywords.join(", ") : "\u672A\u8BBE\u7F6E");
  document.querySelectorAll(".bs-overlay").forEach(function(o) {
    closeOverlay(o);
  });
  previewDoScan();
}
function previewFilterSheetLang() {
  var overlay = document.createElement("div");
  overlay.className = "bs-overlay";
  var current = previewState.filters.lang_filter || "none";
  overlay.innerHTML = '<div class="bs-sheet"><div class="bs-title">\u8BED\u8A00\u8FC7\u6EE4</div><div class="bs-body"><div class="bs-option" data-value="none"><div class="bs-dot"></div><span class="bs-label">\u4E0D\u9650\u5236</span></div><div class="bs-option" data-value="chinese_only"><div class="bs-dot"></div><span class="bs-label">\u5FC5\u987B\u542B\u4E2D\u6587</span></div><div class="bs-option" data-value="no_chinese"><div class="bs-dot"></div><span class="bs-label">\u5FC5\u987B\u4E0D\u542B\u4E2D\u6587</span></div></div><div class="bs-footer"><button class="bs-btn bs-btn-cancel" onclick="closeBsOverlay(this)">\u53D6\u6D88</button><button class="bs-btn bs-btn-confirm" onclick="closeBsOverlay(this)">\u786E\u5B9A</button></div></div>';
  document.body.appendChild(overlay);
  overlay.querySelectorAll(".bs-option").forEach(function(o) {
    if (o.dataset.value === current) o.classList.add("active");
    o.addEventListener("click", function() {
      overlay.querySelectorAll(".bs-option").forEach(function(x) {
        x.classList.remove("active");
      });
      this.classList.add("active");
      previewState.filters.lang_filter = this.dataset.value;
      var labels = { none: "\u4E0D\u9650\u5236", chinese_only: "\u5FC5\u987B\u542B\u4E2D\u6587", no_chinese: "\u5FC5\u987B\u4E0D\u542B\u4E2D\u6587" };
      previewFilterUpdateSummary("langFilter", labels[this.dataset.value] || "\u4E0D\u9650\u5236");
      previewDoScan();
    });
  });
}
function previewFilterSheetSize() {
  var overlay = document.createElement("div");
  overlay.className = "bs-overlay";
  var min = previewState.filters.size_min || "";
  var max = previewState.filters.size_max || "";
  overlay.innerHTML = '<div class="bs-sheet"><div class="bs-title">\u6587\u4EF6\u5927\u5C0F\u8303\u56F4</div><div class="bs-body" style="padding:8px 18px"><label>\u6700\u5C0F\u503C</label><div class="row"><input id="pv-size-min" type="number" placeholder="0" value="' + min + '" style="margin:0"><select id="pv-size-min-unit" style="width:80px;flex:none"><option value="KB">KB</option><option value="MB">MB</option></select></div><label>\u6700\u5927\u503C</label><div class="row"><input id="pv-size-max" type="number" placeholder="\u4E0D\u9650" value="' + max + `" style="margin:0"><select id="pv-size-max-unit" style="width:80px;flex:none"><option value="KB">KB</option><option value="MB">MB</option></select></div><label class="lbl-cb" style="margin-top:8px"><input type="checkbox" id="pv-size-unlimited" onchange="var els=document.querySelectorAll('#pv-size-min,#pv-size-max,#pv-size-min-unit,#pv-size-max-unit');for(var i=0;i<els.length;i++){els[i].disabled=this.checked}"` + (!min && !max ? " checked" : "") + '><span class="tgl"></span> \u4E0D\u9650\u5236\u5927\u5C0F</label></div><div class="bs-footer"><button class="bs-btn bs-btn-cancel" onclick="closeBsOverlay(this)">\u53D6\u6D88</button><button class="bs-btn bs-btn-confirm" onclick="previewFilterSizeConfirm()">\u786E\u5B9A</button></div></div>';
  document.body.appendChild(overlay);
}
function previewFilterSizeConfirm() {
  var unlimited = document.getElementById("pv-size-unlimited");
  if (unlimited && unlimited.checked) {
    previewState.filters.size_min = 0;
    previewState.filters.size_max = 0;
    previewFilterUpdateSummary("sizeRange", "\u4E0D\u9650\u5236");
  } else {
    var rawMin = parseInt(document.getElementById("pv-size-min").value) || 0;
    var minUnit = document.getElementById("pv-size-min-unit").value;
    var rawMax = parseInt(document.getElementById("pv-size-max").value) || 0;
    var maxUnit = document.getElementById("pv-size-max-unit").value;
    var min = rawMin, max = rawMax;
    if (minUnit === "MB") min *= 1024;
    if (maxUnit === "MB") max *= 1024;
    previewState.filters.size_min = min * 1024;
    previewState.filters.size_max = max * 1024;
    var summary = "";
    if (rawMin) summary += rawMin + (minUnit === "MB" ? "MB" : "KB");
    if (rawMin && rawMax) summary += " \uFF5E ";
    if (rawMax) summary += rawMax + (maxUnit === "MB" ? "MB" : "KB");
    previewFilterUpdateSummary("sizeRange", summary || "\u4E0D\u9650\u5236");
  }
  document.querySelectorAll(".bs-overlay").forEach(function(o) {
    closeOverlay(o);
  });
  previewDoScan();
}
function previewFilterSheetFormats() {
  var overlay = document.createElement("div");
  overlay.className = "bs-overlay";
  var current = previewState.filters.formats || [];
  var allFormats = [{ v: ".mcstructure", l: "MCStructure \u7ED3\u6784\u6587\u4EF6" }, { v: ".schematic", l: "Schematic \u793A\u610F\u56FE" }, { v: ".schem", l: "\u65E7\u7248 Schematic" }, { v: ".mcworld", l: "MCWorld \u4E16\u754C\u6587\u4EF6" }, { v: ".bdx", l: "BDX \u7ED3\u6784\u6587\u4EF6" }];
  var itemsHtml = "";
  for (var i = 0; i < allFormats.length; i++) {
    var f = allFormats[i];
    var checked = !current.length || current.indexOf(f.v) >= 0 ? "checked" : "";
    itemsHtml += '<label class="lbl-cb" style="padding:10px 18px;margin:0"><input type="checkbox" class="pv-format-cb" value="' + f.v + '" ' + checked + '><span class="tgl"></span> ' + f.l + "</label>";
  }
  overlay.innerHTML = '<div class="bs-sheet"><div class="bs-title">\u6587\u4EF6\u683C\u5F0F</div><div class="bs-body">' + itemsHtml + '</div><div class="bs-footer"><button class="bs-btn bs-btn-cancel" onclick="closeBsOverlay(this)">\u53D6\u6D88</button><button class="bs-btn bs-btn-confirm" onclick="previewFilterFormatsConfirm()">\u786E\u5B9A</button></div></div>';
  document.body.appendChild(overlay);
}
function previewFilterFormatsConfirm() {
  var cbs = document.querySelectorAll(".pv-format-cb:checked");
  var formats = [];
  cbs.forEach(function(cb) {
    formats.push(cb.value);
  });
  previewState.filters.formats = formats;
  var labels = { ".mcstructure": "MCStructure", ".schematic": "Schematic", ".schem": "Schem", ".mcworld": "MCWorld", ".bdx": "BDX" };
  var summary = formats.length ? formats.map(function(f) {
    return labels[f] || f;
  }).join(", ") : "\u5168\u90E8";
  previewFilterUpdateSummary("formatFilter", summary);
  document.querySelectorAll(".bs-overlay").forEach(function(o) {
    closeOverlay(o);
  });
  previewDoScan();
}
function previewFilterUpdateSummary(id, text) {
  var el = document.getElementById("pv-filter-summary-" + id);
  if (el) el.textContent = text;
}
function renderStep3() {
  previewCleanup3D();
  var body = document.getElementById("preview-body");
  body.innerHTML = '<div class="wz-page active" style="position:relative;opacity:1;pointer-events:auto;transform:none;padding:0;align-items:stretch;width:100%"><div class="wz-topbar"><button onclick="renderPreviewStep(2)"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/></svg> \u8FD4\u56DE</button></div><div class="wz-section-title"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg>\u955C\u5934\u914D\u7F6E</div><div id="pv-angle-list" class="card" style="min-height:60px"></div><button class="btn-s" onclick="previewShowAnglePicker()" style="width:100%;margin-top:8px"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="vertical-align:middle;margin-right:4px"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg> \u6DFB\u52A0\u955C\u5934</button><div style="margin-top:12px;padding:12px;background:var(--phx-bg);border-radius:var(--phx-radius-sm)"><label class="lbl-cb"><input type="checkbox" id="pv-gen-gif" onchange="previewState.genGif=this.checked"' + (previewState.genGif ? " checked" : "") + '><span class="tgl"></span> \u540C\u65F6\u751F\u6210 GIF \u73AF\u7ED5\u52A8\u753B</label></div><div class="wz-dot-bar" style="padding:8px 0 12px"><span class="wz-dot"></span><span class="wz-dot"></span><span class="wz-dot active"></span><span class="wz-dot"></span></div><div class="row"><button class="btn-s" onclick="renderPreviewStep(2)">\u4E0A\u4E00\u6B65</button><button class="btn" onclick="renderPreviewStep(4)">\u4E0B\u4E00\u6B65 \u2192</button></div></div>';
  renderAngleList();
}
function renderAngleList() {
  var list = document.getElementById("pv-angle-list");
  if (!list) return;
  if (!previewState.angles.length) {
    list.innerHTML = '<div style="text-align:center;padding:20px;color:var(--phx-text-secondary);font-size:12px">\u5C1A\u672A\u6DFB\u52A0\u955C\u5934\uFF0C\u70B9\u51FB\u4E0B\u65B9\u6309\u94AE\u6DFB\u52A0</div>';
    return;
  }
  var html = '<div style="font-size:11px;color:var(--phx-text-secondary);font-weight:700;margin-bottom:8px">\u5DF2\u9009\u955C\u5934\uFF08' + previewState.angles.length + " \u4E2A\uFF09</div>";
  for (var i = 0; i < previewState.angles.length; i++) {
    var a = previewState.angles[i];
    html += '<div style="display:flex;align-items:center;gap:8px;padding:8px 0;border-bottom:1px solid var(--phx-border-light)" data-idx="' + i + '"><span style="cursor:grab;color:var(--phx-text-disabled)">\u283F</span><span style="flex:1;font-size:13px;font-weight:600">' + a.label + '</span><button class="q-item-del" onclick="previewRemoveAngle(' + i + ')"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button></div>';
  }
  list.innerHTML = html;
}
function previewShowAnglePicker() {
  var overlay = document.createElement("div");
  overlay.className = "bs-overlay";
  overlay.innerHTML = `<div class="bs-sheet"><div class="bs-title">\u9009\u62E9\u955C\u5934\u7C7B\u578B</div><div class="bs-body"><div class="bs-option" onclick="closeBsOverlay(this);previewShowAngleCategory('direction','\u6B63\u65B9\u5411',['\u6B63\u9762','\u80CC\u9762','\u5DE6\u9762','\u53F3\u9762'],['front','back','left','right'])"><div class="bs-dot"></div><span class="bs-label">\u6B63\u65B9\u5411</span><span class="dim">\u524D / \u540E / \u5DE6 / \u53F3</span></div><div class="bs-option" onclick="closeBsOverlay(this);previewShowAngleCategory('corner','\u5BF9\u89D2',['\u4E1C\u5317\u89D2','\u4E1C\u5357\u89D2','\u897F\u5317\u89D2','\u897F\u5357\u89D2'],['northeast','southeast','northwest','southwest'])"><div class="bs-dot"></div><span class="bs-label">\u5BF9\u89D2</span><span class="dim">4 \u4E2A\u7B49\u8F74\u6D4B\u89D2</span></div><div class="bs-option" onclick="closeBsOverlay(this);previewShowAngleCategory('octo','\u516B\u89D2',['0\xB0','45\xB0','90\xB0','135\xB0','180\xB0','225\xB0','270\xB0','315\xB0'],['0','45','90','135','180','225','270','315'])"><div class="bs-dot"></div><span class="bs-label">\u516B\u89D2</span><span class="dim">45\xB0 \u589E\u91CF 8 \u4E2A\u65B9\u5411</span></div><div class="bs-option" onclick="closeBsOverlay(this);previewShowAngleCategory('birdseye','\u9E1F\u77B0',['\u6B63\u4E0A\u65B9\u4FEF\u89C6'],['top'])"><div class="bs-dot"></div><span class="bs-label">\u9E1F\u77B0</span><span class="dim">\u6B63\u4E0A\u65B9\u4FEF\u89C6</span></div><div class="bs-option" onclick="closeBsOverlay(this);previewShowAngleCustom()"><div class="bs-dot"></div><span class="bs-label">\u81EA\u5B9A\u4E49</span><span class="dim">\u81EA\u7531\u62D6\u62FD\u8C03\u8282\u89D2\u5EA6</span></div></div><div class="bs-footer"><button class="bs-btn bs-btn-cancel" onclick="closeBsOverlay(this)">\u53D6\u6D88</button></div></div>`;
  document.body.appendChild(overlay);
}
function previewShowAngleCategory(type, title, labels, values) {
  var overlay = document.createElement("div");
  overlay.className = "bs-overlay";
  var itemsHtml = "";
  for (var i = 0; i < labels.length; i++) {
    itemsHtml += '<label class="lbl-cb" style="padding:12px 18px;margin:0;border-bottom:1px solid var(--phx-border-light)"><input type="checkbox" class="pv-angle-cb" value="' + values[i] + '" data-label="' + labels[i] + '"><span class="tgl"></span> ' + labels[i] + "</label>";
  }
  overlay.innerHTML = '<div class="bs-sheet"><div class="bs-title" style="display:flex;align-items:center;gap:8px"><button onclick="closeBsOverlay(this);previewShowAnglePicker()" style="background:none;border:none;padding:0;cursor:pointer;color:var(--phx-text-secondary)"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/></svg></button>' + title + '</div><div class="bs-body">' + itemsHtml + '</div><div class="bs-footer"><button class="bs-btn bs-btn-cancel" onclick="previewShowAnglePicker()">\u53D6\u6D88</button><button class="bs-btn bs-btn-confirm" onclick="previewAngleCategoryConfirm()">\u786E\u5B9A\u6DFB\u52A0</button></div></div>';
  document.body.appendChild(overlay);
}
function previewAngleCategoryConfirm() {
  var cbs = document.querySelectorAll(".pv-angle-cb:checked");
  cbs.forEach(function(cb) {
    previewState.angles.push({
      type: "preset",
      value: cb.value,
      label: cb.dataset.label
    });
  });
  document.querySelectorAll(".bs-overlay").forEach(function(o) {
    closeOverlay(o);
  });
  renderAngleList();
}
function previewShowAngleCustom() {
  var overlay = document.createElement("div");
  overlay.className = "bs-overlay";
  overlay.innerHTML = `<div class="bs-sheet"><div class="bs-title" style="display:flex;align-items:center;gap:8px"><button onclick="closeBsOverlay(this);previewCleanupAnglePreview();previewShowAnglePicker()" style="background:none;border:none;padding:0;cursor:pointer;color:var(--phx-text-secondary)"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/></svg></button>\u81EA\u5B9A\u4E49\u89D2\u5EA6</div><div class="bs-body" style="padding:8px 18px"><div id="pv-angle-preview-3d" style="width:100%;height:220px;border-radius:12px;border:2px solid var(--phx-border-light);margin-bottom:12px;overflow:hidden"></div><label>\u540D\u79F0</label><input id="pv-custom-angle-name" type="text" placeholder="\u81EA\u5B9A\u4E49\u89D2\u5EA6" value="\u81EA\u5B9A\u4E49" style="margin:0"><label>\u6C34\u5E73\u89D2</label><div class="row"><input id="pv-custom-yaw" type="number" value="45" style="margin:0" onchange="if(window.__previewAngleUpdate)window.__previewAngleUpdate()"><button class="btn-s" style="width:auto;padding:11px 16px;flex:none" onclick="var i=document.getElementById('pv-custom-yaw');i.value=parseInt(i.value||0)-5;if(window.__previewAngleUpdate)window.__previewAngleUpdate()">-5\xB0</button><button class="btn-s" style="width:auto;padding:11px 16px;flex:none" onclick="var i=document.getElementById('pv-custom-yaw');i.value=parseInt(i.value||0)+5;if(window.__previewAngleUpdate)window.__previewAngleUpdate()">+5\xB0</button></div><label>\u4EF0\u89D2</label><div class="row"><input id="pv-custom-pitch" type="number" value="30" style="margin:0" onchange="if(window.__previewAngleUpdate)window.__previewAngleUpdate()"><button class="btn-s" style="width:auto;padding:11px 16px;flex:none" onclick="var i=document.getElementById('pv-custom-pitch');i.value=parseInt(i.value||0)-5;if(window.__previewAngleUpdate)window.__previewAngleUpdate()">-5\xB0</button><button class="btn-s" style="width:auto;padding:11px 16px;flex:none" onclick="var i=document.getElementById('pv-custom-pitch');i.value=parseInt(i.value||0)+5;if(window.__previewAngleUpdate)window.__previewAngleUpdate()">+5\xB0</button></div><label>\u8DDD\u79BB</label><div class="row"><input id="pv-custom-dist" type="number" value="200" style="margin:0" onchange="if(window.__previewAngleUpdate)window.__previewAngleUpdate()"><button class="btn-s" style="width:auto;padding:11px 16px;flex:none" onclick="var i=document.getElementById('pv-custom-dist');i.value=Math.max(50,parseInt(i.value||200)-20);if(window.__previewAngleUpdate)window.__previewAngleUpdate()">-</button><button class="btn-s" style="width:auto;padding:11px 16px;flex:none" onclick="var i=document.getElementById('pv-custom-dist');i.value=Math.min(500,parseInt(i.value||200)+20);if(window.__previewAngleUpdate)window.__previewAngleUpdate()">+</button></div></div><div class="bs-footer"><button class="bs-btn bs-btn-cancel" onclick="closeBsOverlay(this);previewCleanupAnglePreview();previewShowAnglePicker()">\u53D6\u6D88</button><button class="bs-btn bs-btn-confirm" onclick="previewAngleCustomConfirm()">\u6DFB\u52A0\u6B64\u89D2\u5EA6</button></div></div>`;
  document.body.appendChild(overlay);
  setTimeout(function() {
    previewInitAnglePreview("pv-angle-preview-3d");
  }, 100);
}
function previewAngleCustomConfirm() {
  var name = document.getElementById("pv-custom-angle-name").value || "\u81EA\u5B9A\u4E49";
  var yaw = parseInt(document.getElementById("pv-custom-yaw").value) || 45;
  var pitch = parseInt(document.getElementById("pv-custom-pitch").value) || 30;
  var dist = parseInt(document.getElementById("pv-custom-dist").value) || 200;
  previewState.angles.push({
    type: "custom",
    yaw,
    pitch,
    dist,
    label: name + " " + yaw + "\xB0"
  });
  previewCleanupAnglePreview();
  document.querySelectorAll(".bs-overlay").forEach(function(o) {
    closeOverlay(o);
  });
  renderAngleList();
}
function previewRemoveAngle(idx) {
  previewState.angles.splice(idx, 1);
  renderAngleList();
}
function previewCleanup3D() {
  activeRenderers.forEach(function(kill) {
    try {
      kill();
    } catch (e) {
    }
  });
  activeRenderers = [];
}
var previewAngleScene = null;
var previewAngleRenderer = null;
var previewAngleAnimId = null;
function previewInitAnglePreview(containerId, yawId, pitchId, distId) {
  yawId = yawId || "pv-custom-yaw";
  pitchId = pitchId || "pv-custom-pitch";
  distId = distId || "pv-custom-dist";
  var container = document.getElementById(containerId);
  if (!container || typeof THREE === "undefined") return;
  previewCleanupAnglePreview();
  var W = container.clientWidth || 280;
  var H = 200;
  var scene = new THREE.Scene();
  scene.background = new THREE.Color(15789800);
  var camera = new THREE.PerspectiveCamera(40, W / H, 1, 100);
  camera.position.set(8, 6, 8);
  camera.lookAt(0, 0, 0);
  var renderer = new THREE.WebGLRenderer({ antialias: true, preserveDrawingBuffer: true });
  renderer.setSize(W, H);
  renderer.setPixelRatio(1);
  container.innerHTML = "";
  container.appendChild(renderer.domElement);
  scene.add(new THREE.AmbientLight(8947848));
  var dl = new THREE.DirectionalLight(16777215, 0.8);
  dl.position.set(10, 15, 10);
  scene.add(dl);
  var dl2 = new THREE.DirectionalLight(16777215, 0.3);
  dl2.position.set(-5, 5, -5);
  scene.add(dl2);
  var buildingGroup = new THREE.Group();
  var bodyMat = new THREE.MeshPhongMaterial({ color: 13928974, specular: 2236962, shininess: 10 });
  var body = new THREE.Mesh(new THREE.BoxGeometry(3, 2, 2), bodyMat);
  body.position.set(0, 1, 0);
  buildingGroup.add(body);
  var roofMat = new THREE.MeshPhongMaterial({ color: 12089355, specular: 2236962, shininess: 5 });
  var roof = new THREE.Mesh(new THREE.ConeGeometry(2.5, 1.5, 4), roofMat);
  roof.position.set(0, 2.75, 0);
  roof.rotation.y = Math.PI / 4;
  buildingGroup.add(roof);
  var chimneyMat = new THREE.MeshPhongMaterial({ color: 9136404 });
  var chimney = new THREE.Mesh(new THREE.BoxGeometry(0.6, 1.2, 0.6), chimneyMat);
  chimney.position.set(1.2, 2.6, 0.5);
  buildingGroup.add(chimney);
  var doorMat = new THREE.MeshPhongMaterial({ color: 7031850 });
  var door = new THREE.Mesh(new THREE.BoxGeometry(0.8, 1.2, 0.1), doorMat);
  door.position.set(0, 0.6, 1.05);
  buildingGroup.add(door);
  var windowMat = new THREE.MeshPhongMaterial({ color: 8433904, emissive: 2113632, emissiveIntensity: 0.3 });
  var window1 = new THREE.Mesh(new THREE.BoxGeometry(0.6, 0.6, 0.1), windowMat);
  window1.position.set(-0.8, 1.2, 1.05);
  buildingGroup.add(window1);
  var window2 = new THREE.Mesh(new THREE.BoxGeometry(0.6, 0.6, 0.1), windowMat);
  window2.position.set(0.8, 1.2, 1.05);
  buildingGroup.add(window2);
  var blockMat = new THREE.MeshPhongMaterial({ color: 12626040, specular: 2236962, shininess: 5 });
  for (var i = 0; i < 3; i++) {
    var b = new THREE.Mesh(new THREE.BoxGeometry(0.8, 0.5 + i * 0.3, 0.8), blockMat);
    b.position.set(-2.5 + i * 1.2, 0.25 + i * 0.15, -1.5);
    buildingGroup.add(b);
  }
  var gridHelper = new THREE.GridHelper(8, 8, 14538440, 15262422);
  gridHelper.position.y = -0.01;
  scene.add(gridHelper);
  scene.add(buildingGroup);
  var currentYaw = 45;
  var currentPitch = 30;
  var currentDist = 200;
  function updateCamera() {
    var radYaw = currentYaw * Math.PI / 180;
    var radPitch = currentPitch * Math.PI / 180;
    var d = currentDist / 30;
    camera.position.set(
      d * Math.sin(radYaw) * Math.cos(radPitch),
      d * Math.sin(radPitch) + 2,
      d * Math.cos(radYaw) * Math.cos(radPitch)
    );
    camera.lookAt(0, 1, 0);
  }
  updateCamera();
  function syncInputs() {
    var yawEl = document.getElementById(yawId);
    var pitchEl = document.getElementById(pitchId);
    var distEl = document.getElementById(distId);
    if (yawEl) yawEl.value = currentYaw;
    if (pitchEl) pitchEl.value = currentPitch;
    if (distEl) distEl.value = currentDist;
  }
  syncInputs();
  function renderNow() {
    renderer.render(scene, camera);
  }
  renderNow();
  var isDragging = false;
  var lastX = 0, lastY = 0;
  renderer.domElement.style.touchAction = "none";
  renderer.domElement.addEventListener("pointerdown", function(e) {
    isDragging = true;
    lastX = e.clientX;
    lastY = e.clientY;
    renderer.domElement.setPointerCapture(e.pointerId);
  });
  renderer.domElement.addEventListener("pointermove", function(e) {
    if (!isDragging) return;
    currentYaw += (e.clientX - lastX) * 0.5;
    currentPitch = Math.max(5, Math.min(85, currentPitch - (e.clientY - lastY) * 0.5));
    lastX = e.clientX;
    lastY = e.clientY;
    updateCamera();
    syncInputs();
    renderNow();
  });
  renderer.domElement.addEventListener("pointerup", function() {
    isDragging = false;
  });
  renderer.domElement.addEventListener("pointercancel", function() {
    isDragging = false;
  });
  previewAngleScene = { scene, camera, renderer, group: buildingGroup, updateCamera, syncInputs };
  previewAngleRenderer = renderer;
  window.__previewAngleUpdate = function() {
    var yawEl = document.getElementById(yawId);
    var pitchEl = document.getElementById(pitchId);
    var distEl = document.getElementById(distId);
    if (yawEl) currentYaw = parseInt(yawEl.value) || 45;
    if (pitchEl) currentPitch = parseInt(pitchEl.value) || 30;
    if (distEl) currentDist = parseInt(distEl.value) || 200;
    updateCamera();
    renderNow();
  };
}
function previewCleanupAnglePreview() {
  if (previewAngleAnimId) {
    cancelAnimationFrame(previewAngleAnimId);
    previewAngleAnimId = null;
  }
  if (previewAngleRenderer) {
    previewAngleRenderer.dispose();
    previewAngleRenderer = null;
  }
  previewAngleScene = null;
}
function renderStep4() {
  previewCleanupAnglePreview();
  var genGif = previewState.genGif || false;
  var gifFrames = genGif ? 36 : 0;
  var totalImages = (previewState.angles.length + gifFrames || 1) * previewState.files.length;
  var body = document.getElementById("preview-body");
  body.innerHTML = '<div class="wz-page active" style="position:relative;opacity:1;pointer-events:auto;transform:none;padding:0;align-items:stretch;width:100%"><div class="wz-topbar"><button onclick="renderPreviewStep(3)"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/></svg> \u8FD4\u56DE</button></div><div class="wz-section-title"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>\u8F93\u51FA\u8BBE\u7F6E</div><div class="card" style="cursor:pointer;padding:14px 16px;display:flex;align-items:center;justify-content:space-between;margin-bottom:8px" onclick="previewShowFormatPicker()"><div><div style="font-size:13px;font-weight:700">\u56FE\u7247\u683C\u5F0F</div><div class="dim" style="font-size:10px;margin-top:2px" id="pv-format-summary">PNG</div></div><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--phx-text-disabled)" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg></div><div class="card" style="cursor:pointer;padding:14px 16px;display:flex;align-items:center;justify-content:space-between;margin-bottom:8px" onclick="previewShowResolutionPicker()"><div><div style="font-size:13px;font-weight:700">\u5206\u8FA8\u7387</div><div class="dim" style="font-size:10px;margin-top:2px" id="pv-resolution-summary">1024\xD71024</div></div><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--phx-text-disabled)" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg></div><div class="card" style="cursor:pointer;padding:14px 16px;display:flex;align-items:center;justify-content:space-between;margin-bottom:8px" onclick="previewShowOutputModePicker()"><div><div style="font-size:13px;font-weight:700">\u8F93\u51FA\u65B9\u5F0F</div><div class="dim" style="font-size:10px;margin-top:2px" id="pv-mode-summary">\u6309\u5EFA\u7B51\u5206\u6587\u4EF6\u5939</div></div><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--phx-text-disabled)" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg></div><div class="card" style="cursor:pointer;padding:14px 16px;display:flex;align-items:center;justify-content:space-between;margin-bottom:8px" onclick="previewShowDirPicker()"><div><div style="font-size:13px;font-weight:700">\u8F93\u51FA\u76EE\u5F55</div><div class="dim" style="font-size:10px;margin-top:2px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:200px" id="pv-dir-summary">' + previewState.output.dir + '</div></div><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--phx-text-disabled)" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg></div><div class="card" style="text-align:center;margin-top:12px;background:var(--phx-primary-bg);border-color:var(--phx-primary)"><div style="font-size:14px;font-weight:800;color:var(--phx-primary)">\u5171 ' + previewState.files.length + " \u4E2A\u5EFA\u7B51 \xD7 " + (previewState.angles.length + gifFrames || 1) + " \u4E2A\u955C\u5934 = " + totalImages + ' \u5F20\u56FE\u7247</div></div><div class="wz-dot-bar" style="padding:8px 0 12px"><span class="wz-dot"></span><span class="wz-dot"></span><span class="wz-dot"></span><span class="wz-dot active"></span></div><div class="row"><button class="btn-s" onclick="renderPreviewStep(3)">\u4E0A\u4E00\u6B65</button><button class="btn" onclick="previewStartGenerate()" style="background:var(--phx-success)">\u2728 \u5F00\u59CB\u751F\u6210</button></div></div>';
}
function previewShowFormatPicker() {
  var overlay = document.createElement("div");
  overlay.className = "bs-overlay";
  var current = previewState.output.format || "png";
  var formats = [{ v: "png", l: "PNG\uFF08\u65E0\u635F\uFF09", d: "\u9AD8\u4FDD\u771F\uFF0C\u6587\u4EF6\u8F83\u5927" }, { v: "jpg", l: "JPG\uFF08\u6709\u635F\u538B\u7F29\uFF09", d: "\u6587\u4EF6\u5C0F\uFF0C\u9002\u5408\u5206\u4EAB" }, { v: "webp", l: "WebP\uFF08\u73B0\u4EE3\u683C\u5F0F\uFF09", d: "\u5C0F\u4F53\u79EF\uFF0C\u8D28\u91CF\u597D" }];
  var itemsHtml = "";
  for (var i = 0; i < formats.length; i++) {
    itemsHtml += '<div class="bs-option' + (formats[i].v === current ? " active" : "") + '" data-value="' + formats[i].v + '"><div class="bs-dot"></div><span class="bs-label">' + formats[i].l + '</span><span class="dim" style="font-size:10px">' + formats[i].d + "</span></div>";
  }
  overlay.innerHTML = '<div class="bs-sheet"><div class="bs-title">\u56FE\u7247\u683C\u5F0F</div><div class="bs-body">' + itemsHtml + '</div><div class="bs-footer"><button class="bs-btn bs-btn-cancel" onclick="closeBsOverlay(this)">\u53D6\u6D88</button><button class="bs-btn bs-btn-confirm" onclick="previewFormatConfirm()">\u786E\u5B9A</button></div></div>';
  document.body.appendChild(overlay);
  overlay.querySelectorAll(".bs-option").forEach(function(o) {
    o.addEventListener("click", function() {
      overlay.querySelectorAll(".bs-option").forEach(function(x) {
        x.classList.remove("active");
      });
      this.classList.add("active");
    });
  });
}
function previewFormatConfirm() {
  var active = document.querySelector(".bs-sheet .bs-option.active");
  if (active) {
    previewState.output.format = active.dataset.value;
    var labels = { png: "PNG", jpg: "JPG", webp: "WebP" };
    document.getElementById("pv-format-summary").textContent = labels[previewState.output.format] || previewState.output.format;
  }
  document.querySelectorAll(".bs-overlay").forEach(function(o) {
    closeOverlay(o);
  });
}
function previewShowResolutionPicker() {
  var overlay = document.createElement("div");
  overlay.className = "bs-overlay";
  var current = previewState.output.width + "\xD7" + previewState.output.height;
  var resolutions = [{ v: "512", l: "512\xD7512" }, { v: "1024", l: "1024\xD71024" }, { v: "2048", l: "2048\xD72048" }];
  var itemsHtml = "";
  for (var i = 0; i < resolutions.length; i++) {
    itemsHtml += '<div class="bs-option' + (resolutions[i].v === String(previewState.output.width) ? " active" : "") + '" data-value="' + resolutions[i].v + '"><div class="bs-dot"></div><span class="bs-label">' + resolutions[i].l + "</span></div>";
  }
  itemsHtml += '<div style="padding:12px 18px;border-top:1px solid var(--phx-border-light)"><label>\u81EA\u5B9A\u4E49</label><div class="row"><input id="pv-res-w" type="number" value="' + previewState.output.width + '" style="margin:0"><span style="align-self:center;color:var(--phx-text-secondary)">\xD7</span><input id="pv-res-h" type="number" value="' + previewState.output.height + '" style="margin:0"></div></div>';
  overlay.innerHTML = '<div class="bs-sheet"><div class="bs-title">\u5206\u8FA8\u7387</div><div class="bs-body">' + itemsHtml + '</div><div class="bs-footer"><button class="bs-btn bs-btn-cancel" onclick="closeBsOverlay(this)">\u53D6\u6D88</button><button class="bs-btn bs-btn-confirm" onclick="previewResolutionConfirm()">\u786E\u5B9A</button></div></div>';
  document.body.appendChild(overlay);
  overlay.querySelectorAll(".bs-option").forEach(function(o) {
    o.addEventListener("click", function() {
      overlay.querySelectorAll(".bs-option").forEach(function(x) {
        x.classList.remove("active");
      });
      this.classList.add("active");
    });
  });
}
function previewResolutionConfirm() {
  var active = document.querySelector(".bs-sheet .bs-option.active");
  if (active) {
    previewState.output.width = parseInt(active.dataset.value);
    previewState.output.height = parseInt(active.dataset.value);
  } else {
    previewState.output.width = parseInt(document.getElementById("pv-res-w").value) || 1024;
    previewState.output.height = parseInt(document.getElementById("pv-res-h").value) || 1024;
  }
  document.getElementById("pv-resolution-summary").textContent = previewState.output.width + "\xD7" + previewState.output.height;
  document.querySelectorAll(".bs-overlay").forEach(function(o) {
    closeOverlay(o);
  });
}
function previewShowOutputModePicker() {
  var overlay = document.createElement("div");
  overlay.className = "bs-overlay";
  var current = previewState.output.mode || "folder";
  var modes = [{ v: "folder", l: "\u6309\u5EFA\u7B51\u5206\u6587\u4EF6\u5939", d: "\u6BCF\u4E2A\u5EFA\u7B51\u4E00\u4E2A\u6587\u4EF6\u5939\uFF0C\u91CC\u9762\u653E\u5BF9\u5E94\u56FE\u7247" }, { v: "flat", l: "\u76F4\u51FA\u5230\u76EE\u5F55\uFF08\u5E73\u94FA\uFF09", d: "\u6240\u6709\u56FE\u7247\u76F4\u63A5\u653E\u5728\u8F93\u51FA\u76EE\u5F55" }, { v: "zip", l: "\u6253\u5305\u6210 ZIP", d: "\u6240\u6709\u56FE\u7247\u6253\u5305\u6210\u4E00\u4E2A ZIP \u6587\u4EF6" }];
  var itemsHtml = "";
  for (var i = 0; i < modes.length; i++) {
    itemsHtml += '<div class="bs-option' + (modes[i].v === current ? " active" : "") + '" data-value="' + modes[i].v + '"><div class="bs-dot"></div><span class="bs-label">' + modes[i].l + '</span><span class="dim" style="font-size:10px">' + modes[i].d + "</span></div>";
  }
  overlay.innerHTML = '<div class="bs-sheet"><div class="bs-title">\u8F93\u51FA\u65B9\u5F0F</div><div class="bs-body">' + itemsHtml + '</div><div class="bs-footer"><button class="bs-btn bs-btn-cancel" onclick="closeBsOverlay(this)">\u53D6\u6D88</button><button class="bs-btn bs-btn-confirm" onclick="previewOutputModeConfirm()">\u786E\u5B9A</button></div></div>';
  document.body.appendChild(overlay);
  overlay.querySelectorAll(".bs-option").forEach(function(o) {
    o.addEventListener("click", function() {
      overlay.querySelectorAll(".bs-option").forEach(function(x) {
        x.classList.remove("active");
      });
      this.classList.add("active");
    });
  });
}
function previewOutputModeConfirm() {
  var active = document.querySelector(".bs-sheet .bs-option.active");
  if (active) {
    previewState.output.mode = active.dataset.value;
    var labels = { folder: "\u6309\u5EFA\u7B51\u5206\u6587\u4EF6\u5939", flat: "\u76F4\u51FA\u5230\u76EE\u5F55\uFF08\u5E73\u94FA\uFF09", zip: "\u6253\u5305\u6210 ZIP" };
    document.getElementById("pv-mode-summary").textContent = labels[previewState.output.mode] || previewState.output.mode;
  }
  document.querySelectorAll(".bs-overlay").forEach(function(o) {
    closeOverlay(o);
  });
}
function previewShowDirPicker() {
  var overlay = document.createElement("div");
  overlay.className = "bs-overlay";
  overlay.innerHTML = '<div class="bs-sheet"><div class="bs-title">\u8F93\u51FA\u76EE\u5F55</div><div class="bs-body" style="padding:8px 18px"><input id="pv-dir-input" type="text" value="' + previewState.output.dir + '" style="margin:0"><div class="dim" style="font-size:10px;margin-top:6px">\u9ED8\u8BA4\u4E3A /sdcard/Download/\u6E38\u56ED\u9884\u89C8/</div></div><div class="bs-footer"><button class="bs-btn bs-btn-cancel" onclick="closeBsOverlay(this)">\u53D6\u6D88</button><button class="bs-btn bs-btn-confirm" onclick="previewDirConfirm()">\u786E\u5B9A</button></div></div>';
  document.body.appendChild(overlay);
}
function previewDirConfirm() {
  var input = document.getElementById("pv-dir-input");
  if (input) {
    previewState.output.dir = input.value.trim() || "/sdcard/Download/\u6E38\u56ED\u9884\u89C8/";
    document.getElementById("pv-dir-summary").textContent = previewState.output.dir;
  }
  document.querySelectorAll(".bs-overlay").forEach(function(o) {
    closeOverlay(o);
  });
}
function previewStartGenerate() {
  if (!previewState.angles.length && !previewState.genGif) {
    T("\u8BF7\u81F3\u5C11\u6DFB\u52A0\u4E00\u4E2A\u955C\u5934\u6216\u5F00\u542F\u73AF\u7ED5\u52A8\u753B", "w");
    return;
  }
  previewState.output.format = previewState.output.format || "png";
  previewState.output.mode = previewState.output.mode || "folder";
  if (document.getElementById("pv-gen-gif")) previewState.genGif = document.getElementById("pv-gen-gif").checked;
  var genGif = previewState.genGif || false;
  var body = document.getElementById("preview-body");
  body.innerHTML = '<div class="wz-page active" style="position:relative;opacity:1;pointer-events:auto;transform:none;padding:0;align-items:stretch;width:100%"><div class="wz-section-title"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>\u6B63\u5728\u751F\u6210\u9884\u89C8\u56FE</div><div class="card" style="margin-top:8px"><div style="display:flex;justify-content:space-between;margin-bottom:6px"><span style="font-size:12px;font-weight:700" id="pv-progress-text">0%</span><span class="dim" style="font-size:10px" id="pv-progress-count">0 / 0</span></div><div class="pg"><div id="pv-progress-bar" style="width:0%"></div></div></div><div class="card" style="margin-top:8px"><div style="font-size:12px;font-weight:700;margin-bottom:6px">\u5F53\u524D\u5EFA\u7B51</div><div style="font-size:13px" id="pv-current-building">\u7B49\u5F85\u4E2D...</div><div class="dim" style="font-size:10px;margin-top:2px" id="pv-current-angle"></div></div><div class="card" style="margin-top:8px;padding:10px;display:none" id="pv-gif-status"><div style="font-size:10px;font-weight:700;color:var(--phx-text-secondary);margin-bottom:4px">GIF \u540E\u53F0\u5408\u6210</div><div id="pv-gif-status-text" style="font-size:10px;color:var(--phx-text-secondary);line-height:1.6"></div></div><div class="card" style="margin-top:8px;max-height:200px;overflow-y:auto;background:var(--phx-bg)"><div style="font-size:11px;font-weight:700;color:var(--phx-text-secondary);margin-bottom:4px">\u5904\u7406\u65E5\u5FD7</div><div id="pv-log" style="font-size:10px;font-family:monospace;line-height:1.6"></div></div><div class="row" style="margin-top:16px"><button class="btn-d" id="pv-cancel-btn" onclick="previewCancelGenerate()">\u53D6\u6D88</button></div></div>';
  previewState._cancelGen = false;
  previewState._generating = true;
  previewGenerateAll(genGif);
}
var previewGenState = null;
var gifBackendQueue = [];
var gifBackendActive = 0;
function previewGenerateAll(genGif) {
  var files = previewState.files;
  var angles = previewState.angles;
  var gifFrames = genGif ? 36 : 0;
  var totalSteps = files.length * (angles.length + gifFrames);
  if (totalSteps === 0) {
    previewLog("\u6CA1\u6709\u9700\u8981\u751F\u6210\u7684\u5185\u5BB9");
    return;
  }
  previewGenState = {
    files,
    angles,
    genGif,
    gifFrames,
    total: totalSteps,
    done: 0,
    currentFileIdx: 0,
    canceled: false
  };
  updateProgressGen();
  previewGenNextFile();
}
function previewLog(msg) {
  var el = document.getElementById("pv-log");
  if (!el) return;
  el.innerHTML = "<div>" + (/* @__PURE__ */ new Date()).toLocaleTimeString() + " " + msg + "</div>" + el.innerHTML;
  while (el.children.length > 50) {
    el.removeChild(el.lastChild);
  }
}
function previewGenNextFile() {
  if (previewGenState.canceled) return;
  var gs = previewGenState;
  if (gs.currentFileIdx >= gs.files.length) {
    tryComplete();
    return;
  }
  var file = gs.files[gs.currentFileIdx];
  var buildingName = file.name.replace(/\.[^.]+$/, "");
  updateProgressBuilding(buildingName);
  previewLog("\u52A0\u8F7D\u4F53\u7D20: " + buildingName);
  A("POST", "/api/building/voxel", { path: file.path }).then(function(r) {
    if (gs.canceled) return;
    if (!r.ok || !r.voxel) {
      previewLog("\u4F53\u7D20\u52A0\u8F7D\u5931\u8D25: " + buildingName + " - " + (r.error || ""));
      gs.done += (gs.angles.length || 1) + (gs.gifFrames || 0);
      updateProgressGen();
      gs.currentFileIdx++;
      previewGenNextFile();
      return;
    }
    previewGenState._currentVoxel = r.voxel;
    previewGenAngles(file, buildingName, 0, function() {
      if (gs.genGif) {
        previewGenGif(file, buildingName, function() {
          finishBuilding(buildingName);
        });
      } else {
        finishBuilding(buildingName);
      }
    });
  });
}
function finishBuilding(buildingName) {
  var gs = previewGenState;
  if (previewState.output.mode === "zip") {
    var buildingDir = previewState.output.dir + "/" + buildingName;
    previewLog("\u6253\u5305: " + buildingName + ".zip");
    A("POST", "/api/preview/zip-building", {
      dir: buildingDir,
      name: buildingName
    }).then(function(r) {
      if (r.ok) {
        previewLog("\u538B\u7F29\u5305\u5B8C\u6210: " + (r.zip_path || ""));
      } else {
        previewLog("\u6253\u5305\u5931\u8D25: " + (r.error || ""));
      }
      previewGenState._currentVoxel = null;
      gs.currentFileIdx++;
      previewGenNextFile();
    });
  } else {
    previewGenState._currentVoxel = null;
    gs.currentFileIdx++;
    previewGenNextFile();
  }
}
function previewGenAngles(file, buildingName, angleIdx, onDone) {
  if (previewGenState.canceled) {
    onDone();
    return;
  }
  var gs = previewGenState;
  if (angleIdx >= gs.angles.length) {
    onDone();
    return;
  }
  var angle = gs.angles[angleIdx];
  updateProgressAngle(angle.label);
  previewLog("\u622A\u56FE: " + buildingName + " - " + angle.label);
  var voxel = gs._currentVoxel;
  if (!voxel || !voxel.points || !voxel.points.length) {
    previewLog("\u8DF3\u8FC7\uFF08\u65E0\u4F53\u7D20\u6570\u636E\uFF09: " + angle.label);
    gs.done++;
    updateProgressGen();
    previewGenAngles(file, buildingName, angleIdx + 1, onDone);
    return;
  }
  previewRenderAndSave(file, buildingName, angle, function() {
    gs.done++;
    updateProgressGen();
    previewGenAngles(file, buildingName, angleIdx + 1, onDone);
  });
}
function previewRenderAndSave(file, buildingName, angle, onDone) {
  var angleName = sanitizeGenName(angle.label);
  var filename = buildingName + "_" + angleName + "." + (previewState.output.format || "png");
  var buildingDir = previewState.output.dir;
  if (previewState.output.mode === "folder" || previewState.output.mode === "zip") {
    buildingDir = buildingDir + "/" + buildingName;
  }
  var imgPath = buildingDir + "/" + filename;
  var voxel = previewGenState._currentVoxel;
  var s = voxel.step || 1, sx = voxel.size_x || 10, sy = voxel.size_y || 10, sz = voxel.size_z || 10;
  var W = previewState.output.width || 1024;
  var H = previewState.output.height || 1024;
  var cx = sx / 2, cy = sy / 2, cz = sz / 2, m = Math.max(sx, sy, sz);
  var scene = previewBuildScene(voxel, s, sx, sy, sz, cx, cy, cz, m);
  var renderer = new THREE.WebGLRenderer({ antialias: true, preserveDrawingBuffer: true });
  renderer.setSize(W, H);
  renderer.setPixelRatio(1);
  var yaw = 45, pitch = 30, dist = m * 1.8;
  if (angle.type === "custom") {
    yaw = angle.yaw || 45;
    pitch = angle.pitch || 30;
    dist = (angle.dist || 200) / 100 * m;
  } else {
    var angleMap = {
      front: [0, 0],
      back: [180, 0],
      left: [270, 0],
      right: [90, 0],
      northeast: [45, 30],
      southeast: [135, 30],
      northwest: [315, 30],
      southwest: [225, 30],
      top: [0, 89],
      "0": [0, 30],
      "45": [45, 30],
      "90": [90, 30],
      "135": [135, 30],
      "180": [180, 30],
      "225": [225, 30],
      "270": [270, 30],
      "315": [315, 30]
    };
    var ap = angleMap[angle.value] || [45, 30];
    yaw = ap[0];
    pitch = ap[1];
  }
  var radYaw = yaw * Math.PI / 180;
  var radPitch = pitch * Math.PI / 180;
  var cam = new THREE.PerspectiveCamera(45, W / H, 1, 5e3);
  cam.position.set(
    cx + dist * Math.sin(radYaw) * Math.cos(radPitch),
    cy + dist * Math.sin(radPitch),
    cz + dist * Math.cos(radYaw) * Math.cos(radPitch)
  );
  cam.lookAt(cx, cy, cz);
  renderer.render(scene, cam);
  var canvas = renderer.domElement;
  canvas.toBlob(function(blob) {
    var reader = new FileReader();
    reader.onloadend = function() {
      var base64data = reader.result;
      A("POST", "/api/preview/save-image", {
        path: imgPath,
        data: base64data
      }).then(function(r) {
        if (r.ok) {
          previewLog("\u5B8C\u6210: " + filename);
        } else {
          previewLog("\u4FDD\u5B58\u5931\u8D25: " + (r.error || "") + " - " + filename);
        }
        previewDisposeScene(scene);
        renderer.dispose();
        if (onDone) onDone();
      });
    };
    reader.readAsDataURL(blob);
  }, "image/" + (previewState.output.format === "jpg" ? "jpeg" : "png"));
}
function previewBuildScene(voxel, s, sx, sy, sz, cx, cy, cz, m) {
  var scene = new THREE.Scene();
  scene.background = new THREE.Color(16316656);
  if (cx === void 0) cx = sx / 2;
  if (cy === void 0) cy = sy / 2;
  if (cz === void 0) cz = sz / 2;
  if (m === void 0) m = Math.max(sx, sy, sz);
  var filtered = [];
  for (var i = 0; i < voxel.points.length; i++) {
    var p = voxel.points[i];
    if (p.c === "#ff00ff" || p.c === "#e94560") continue;
    filtered.push(p);
  }
  var points = filtered.length ? filtered : voxel.points;
  var groups = {};
  for (var i = 0; i < points.length; i++) {
    var col = points[i].c || "#e94560";
    if (!groups[col]) groups[col] = [];
    groups[col].push(points[i]);
  }
  var gk = Object.keys(groups);
  var geom = new THREE.BoxGeometry(s * 0.85, s * 0.85, s * 0.85);
  for (var gi = 0; gi < gk.length; gi++) {
    var color = gk[gi], arr = groups[color];
    var mat = new THREE.MeshPhongMaterial({ color, specular: 1118481, shininess: 20 });
    var mesh = new THREE.InstancedMesh(geom, mat, arr.length);
    var dummy = new THREE.Object3D();
    for (var j = 0; j < arr.length; j++) {
      dummy.position.set(arr[j].x, arr[j].y, arr[j].z);
      dummy.updateMatrix();
      mesh.setMatrixAt(j, dummy.matrix);
    }
    mesh.instanceMatrix.needsUpdate = true;
    scene.add(mesh);
  }
  scene.add(new THREE.AmbientLight(8947848));
  var dl = new THREE.DirectionalLight(16777215, 0.8);
  dl.position.set(cx + m, cy + m * 2, cz + m);
  scene.add(dl);
  return scene;
}
function previewGen2D(file, buildingName, onDone) {
  if (previewGenState.canceled) {
    onDone();
    return;
  }
  var gs = previewGenState;
  var body = {
    files: [file.path],
    angles: [],
    output: {
      format: previewState.output.format,
      width: previewState.output.width,
      height: previewState.output.height,
      mode: previewState.output.mode || "folder",
      dir: previewState.output.dir
    },
    gen_topdown: gs.genTopDown,
    gen_elevation: gs.genElevation,
    gen_gif: false,
    save_config: false
  };
  A("POST", "/api/preview/generate", body).then(function(r) {
    if (r.ok) {
      var pollTimer = setInterval(function() {
        A("GET", "/api/preview/status?task_id=" + encodeURIComponent(r.task_id)).then(function(sr) {
          if (sr.status === "complete" || sr.status === "cancelled") {
            clearInterval(pollTimer);
            gs.done += (gs.genTopDown ? 1 : 0) + (gs.genElevation ? 4 : 0);
            updateProgressGen();
            onDone();
          }
        });
      }, 500);
    } else {
      previewLog("2D \u6E32\u67D3\u542F\u52A8\u5931\u8D25: " + buildingName);
      onDone();
    }
  });
}
function previewGenGif(file, buildingName, onDone) {
  if (previewGenState.canceled) {
    if (onDone) onDone();
    return;
  }
  var gs = previewGenState;
  previewLog("GIF \u6E32\u67D3: " + buildingName);
  var voxel = gs._currentVoxel;
  if (!voxel || !voxel.points || !voxel.points.length) {
    previewLog("\u8DF3\u8FC7\uFF08\u65E0\u4F53\u7D20\u6570\u636E\uFF09");
    gs.done += gs.gifFrames;
    updateProgressGen();
    if (onDone) onDone();
    tryComplete();
    return;
  }
  var frameCount = 36;
  gs.done += frameCount;
  updateProgressGen();
  var s = voxel.step || 1, sx = voxel.size_x || 10, sy = voxel.size_y || 10, sz = voxel.size_z || 10;
  var W = previewState.output.width || 1024;
  var H = previewState.output.height || 1024;
  var cx = sx / 2, cy = sy / 2, cz = sz / 2, m = Math.max(sx, sy, sz);
  var dist = m * 1.8;
  var frames = [];
  var scene = previewBuildScene(voxel, s, sx, sy, sz, cx, cy, cz, m);
  var renderer = new THREE.WebGLRenderer({ antialias: true, preserveDrawingBuffer: true });
  renderer.setSize(W, H);
  renderer.setPixelRatio(1);
  var cam = new THREE.PerspectiveCamera(45, W / H, 1, 5e3);
  var canvas = renderer.domElement;
  for (var i = 0; i < frameCount; i++) {
    var yaw = i * 10, pitch = 30;
    var radYaw = yaw * Math.PI / 180;
    var radPitch = pitch * Math.PI / 180;
    cam.position.set(
      cx + dist * Math.sin(radYaw) * Math.cos(radPitch),
      cy + dist * Math.sin(radPitch),
      cz + dist * Math.cos(radYaw) * Math.cos(radPitch)
    );
    cam.lookAt(cx, cy, cz);
    renderer.render(scene, cam);
    frames.push(canvas.toDataURL("image/png"));
  }
  updateProgressAngle("\u73AF\u7ED5\u5B8C\u6210");
  previewDisposeScene(scene);
  renderer.dispose();
  var gifFilename = buildingName + "_\u73AF\u7ED5\u52A8\u753B.gif";
  var gifPath = previewState.output.dir;
  if (previewState.output.mode === "folder" || previewState.output.mode === "zip") {
    gifPath = gifPath + "/" + buildingName;
  }
  gifPath = gifPath + "/" + gifFilename;
  gifBackendQueue.push({
    path: gifPath,
    frames,
    filename: gifFilename
  });
  previewLog("GIF \u5408\u6210: " + gifFilename + " (" + frames.length + " \u5E27)");
  processGifBackendQueue();
  if (onDone) onDone();
}
function processGifBackendQueue() {
  while (gifBackendActive < 2 && gifBackendQueue.length > 0) {
    var task = gifBackendQueue.shift();
    gifBackendActive++;
    updateGifStatus();
    A("POST", "/api/preview/generate-gif", {
      path: task.path,
      frames: task.frames,
      delay: 5
    }).then(function(r) {
      gifBackendActive--;
      if (r.ok) {
        previewLog("GIF \u5B8C\u6210: " + task.filename);
      } else {
        previewLog("GIF \u5408\u6210\u5931\u8D25: " + (r.error || ""));
      }
      updateGifStatus();
      tryComplete();
      processGifBackendQueue();
    });
  }
}
function updateGifStatus() {
  var el = document.getElementById("pv-gif-status");
  var txt = document.getElementById("pv-gif-status-text");
  if (!el || !txt) return;
  var active = gifBackendActive || 0;
  var pending = gifBackendQueue.length || 0;
  if (active + pending === 0) {
    el.style.display = "none";
    return;
  }
  el.style.display = "";
  var parts = [];
  if (active > 0) parts.push(active + " \u5408\u6210\u4E2D");
  if (pending > 0) parts.push(pending + " \u7B49\u5F85\u5408\u6210");
  txt.textContent = parts.join("\uFF0C");
}
function tryComplete() {
  var gs = previewGenState;
  if (!gs) return;
  if (gs.currentFileIdx < gs.files.length) return;
  if (gifBackendQueue.length > 0 || gifBackendActive > 0) return;
  previewLog("\u5168\u90E8\u5B8C\u6210\uFF01");
  previewShowGenComplete();
}
function sanitizeGenName(name) {
  return String(name).replace(/[\/\\:*?"<>|°\s]/g, "_").replace(/_+/g, "_");
}
function updateProgressGen() {
  var gs = previewGenState;
  if (!gs || gs.total === 0) return;
  var pct = Math.min(1, gs.done / gs.total);
  var bar = document.getElementById("pv-progress-bar");
  var text = document.getElementById("pv-progress-text");
  var count = document.getElementById("pv-progress-count");
  if (bar) bar.style.width = pct * 100 + "%";
  if (text) text.textContent = Math.round(pct * 100) + "%";
  if (count) count.textContent = gs.done + " / " + gs.total;
}
function updateProgressBuilding(name) {
  var el = document.getElementById("pv-current-building");
  if (el) el.textContent = name;
}
function updateProgressAngle(label) {
  var el = document.getElementById("pv-current-angle");
  if (el) el.textContent = "\u5F53\u524D: " + label;
}
function previewCancelGenerate() {
  previewState._generating = false;
  if (previewState.generateTaskId) {
    A("POST", "/api/preview/cancel?task_id=" + encodeURIComponent(previewState.generateTaskId));
  }
  if (previewState._pollTimer) {
    clearInterval(previewState._pollTimer);
    previewState._pollTimer = null;
  }
  if (previewGenState) {
    previewGenState.canceled = true;
  }
  var btn = document.getElementById("pv-cancel-btn");
  if (btn) {
    btn.textContent = "\u5DF2\u53D6\u6D88";
    btn.disabled = true;
  }
  previewLog("\u5DF2\u53D6\u6D88");
}
function previewDoZip(onDone) {
  A("POST", "/api/preview/zip", { dir: previewState.output.dir }).then(function(r) {
    if (r.ok) {
      previewLog("ZIP \u6253\u5305\u5B8C\u6210: " + (r.zip_path || ""));
    } else {
      previewLog("ZIP \u6253\u5305\u5931\u8D25: " + (r.error || ""));
    }
    if (onDone) onDone();
  });
}
function previewShowGenComplete() {
  previewState._generating = false;
  var body = document.getElementById("preview-body");
  if (!body) return;
  body.innerHTML = '<div class="wz-page active" style="position:relative;opacity:1;pointer-events:auto;transform:none;padding:0;align-items:stretch;width:100%"><div style="text-align:center;padding:40px 20px"><svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="var(--phx-success)" stroke-width="1.5" style="margin-bottom:16px"><polyline points="20 6 9 17 4 12"/></svg><div style="font-size:20px;font-weight:900;margin-bottom:8px">\u751F\u6210\u5B8C\u6210</div><div class="dim" style="margin-bottom:24px">\u5171\u5904\u7406 ' + (previewGenState ? previewGenState.done : 0) + ' \u5F20\u56FE\u7247</div><div class="card" style="text-align:left;font-size:12px;line-height:1.8"><div>\u8F93\u51FA\u76EE\u5F55: <span style="color:var(--phx-primary);font-weight:700">' + previewState.output.dir + '</span></div></div><button class="btn" style="margin-top:20px" onclick="closeSubPage()">\u5B8C\u6210</button></div></div>';
}
function renderPreviewAngle(container, voxel, angle, onDone) {
  if (!voxel || !voxel.points || !voxel.points.length || typeof THREE === "undefined") {
    if (onDone) onDone();
    return;
  }
  var s = voxel.step || 1, sx = voxel.size_x || 10, sy = voxel.size_y || 10, sz = voxel.size_z || 10;
  var W = container.clientWidth || previewState.output.width || 1024;
  var H = container.clientHeight || previewState.output.height || 1024;
  var scene = new THREE.Scene();
  scene.background = new THREE.Color(16316656);
  var cam = new THREE.PerspectiveCamera(45, W / H, 1, 5e3);
  var cx = sx / 2, cy = sy / 2, cz = sz / 2, m = Math.max(sx, sy, sz);
  var yaw = 45, pitch = 30, dist = m * 1.8;
  if (angle.type === "custom") {
    yaw = angle.yaw || 45;
    pitch = angle.pitch || 30;
    dist = (angle.dist || 200) / 100 * m;
  } else {
    var angleMap = {
      front: [0, 0],
      back: [180, 0],
      left: [270, 0],
      right: [90, 0],
      northeast: [45, 30],
      southeast: [135, 30],
      northwest: [315, 30],
      southwest: [225, 30],
      top: [0, 89],
      "0": [0, 30],
      "45": [45, 30],
      "90": [90, 30],
      "135": [135, 30],
      "180": [180, 30],
      "225": [225, 30],
      "270": [270, 30],
      "315": [315, 30]
    };
    var ap = angleMap[angle.value] || [45, 30];
    yaw = ap[0];
    pitch = ap[1];
  }
  var radYaw = yaw * Math.PI / 180;
  var radPitch = pitch * Math.PI / 180;
  cam.position.set(
    cx + dist * Math.sin(radYaw) * Math.cos(radPitch),
    cy + dist * Math.sin(radPitch),
    cz + dist * Math.cos(radYaw) * Math.cos(radPitch)
  );
  cam.lookAt(cx, cy, cz);
  var renderer = new THREE.WebGLRenderer({ antialias: true, preserveDrawingBuffer: true });
  renderer.setSize(W, H);
  renderer.setPixelRatio(1);
  container.innerHTML = "";
  container.appendChild(renderer.domElement);
  scene.add(new THREE.AmbientLight(8947848));
  var dl = new THREE.DirectionalLight(16777215, 0.8);
  dl.position.set(cx + m, cy + m * 2, cz + m);
  scene.add(dl);
  var filtered = [];
  for (var i = 0; i < voxel.points.length; i++) {
    var p = voxel.points[i];
    if (p.c === "#ff00ff" || p.c === "#ff00ff" || p.c === "#e94560") continue;
    filtered.push(p);
  }
  var points = filtered.length ? filtered : voxel.points;
  var groups = {};
  for (var i = 0; i < points.length; i++) {
    var col = points[i].c || "#e94560";
    if (!groups[col]) groups[col] = [];
    groups[col].push(points[i]);
  }
  var gk = Object.keys(groups);
  var geom = new THREE.BoxGeometry(s * 0.85, s * 0.85, s * 0.85);
  for (var gi = 0; gi < gk.length; gi++) {
    var color = gk[gi], arr = groups[color];
    var mat = new THREE.MeshPhongMaterial({ color, specular: 1118481, shininess: 20 });
    var mesh = new THREE.InstancedMesh(geom, mat, arr.length);
    var dummy = new THREE.Object3D();
    for (var j = 0; j < arr.length; j++) {
      dummy.position.set(arr[j].x, arr[j].y, arr[j].z);
      dummy.updateMatrix();
      mesh.setMatrixAt(j, dummy.matrix);
    }
    mesh.instanceMatrix.needsUpdate = true;
    scene.add(mesh);
  }
  renderer.render(scene, cam);
  if (onDone) onDone(scene, renderer);
}
function previewDisposeScene(scene) {
  if (!scene) return;
  scene.traverse(function(obj) {
    if (obj.geometry) {
      obj.geometry.dispose();
    }
    if (obj.material) {
      if (Array.isArray(obj.material)) {
        obj.material.forEach(function(m) {
          disposeMaterial(m);
        });
      } else {
        disposeMaterial(obj.material);
      }
    }
  });
}
function disposeMaterial(mat) {
  if (!mat) return;
  if (mat.map) mat.map.dispose();
  if (mat.lightMap) mat.lightMap.dispose();
  if (mat.emissiveMap) mat.emissiveMap.dispose();
  if (mat.specularMap) mat.specularMap.dispose();
  mat.dispose();
}
var origCloseSubPage = closeSubPage;
closeSubPage = function() {
  previewCleanupAnglePreview();
  activeRenderers.forEach(function(kill) {
    try {
      kill();
    } catch (e) {
    }
  });
  activeRenderers = [];
  origCloseSubPage();
};
