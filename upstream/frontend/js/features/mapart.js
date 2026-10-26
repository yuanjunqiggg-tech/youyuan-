var maPath = "", maClarity = 1, maCrop = { cx: 0, cy: 0, cw: 0, ch: 0 };
function RmaHTML() {
  return '<div class="card"><div class="pick" onclick="doMaPick()"><i class="pi">' + ICONS.upload + '</i><div id="ma-pick-txt">\u70B9\u51FB\u9009\u62E9\u56FE\u7247</div></div><div id="ma-info" class="gone"></div><div id="ma-params" class="gone"><label>\u653E\u7F6E\u5750\u6807</label><div class="row"><input id="ma-x" type="number" placeholder="X"><input id="ma-y" type="number" placeholder="Y"><input id="ma-z" type="number" placeholder="Z"></div><button class="btn-s" id="ma-align-btn" onclick="alignMaCoords()" style="width:100%;margin:2px 0 6px;font-size:11px;display:none">\u5BF9\u9F50 128 \u7F51\u683C</button><label>\u81EA\u5B9A\u4E49\u5C3A\u5BF8</label><div class="row"><input id="ma-w" type="number" placeholder="\u5BBD\uFF08\u81EA\u52A8\uFF09"><input id="ma-h" type="number" placeholder="\u9AD8\uFF08\u81EA\u52A8\uFF09"></div><label>\u6446\u653E\u65B9\u5411</label><select id="ma-ori" onchange="toggleMaRelief()"><option value="horizontal">\u6C34\u5E73\uFF08\u5730\u9762\uFF09</option><option value="vertical">\u5782\u76F4\uFF08\u5899\u58C1\uFF09</option></select><div id="ma-relief-opts" class="qopt"><label class="lbl-cb"><input type="checkbox" id="ma-relief" onchange="toggleMaRelief()"><span class="tgl"></span> \u6D6E\u96D5\u6A21\u5F0F</label><div class="dim" style="font-size:10px">\u6839\u636E\u989C\u8272\u4EAE\u5EA6\u8C03\u6574\u65B9\u5757Y\u8F74\u9AD8\u5EA6\uFF0C\u4EA7\u751F\u7ACB\u4F53\u611F</div></div><div id="ma-gravity-opts" class="qopt"><label class="lbl-cb"><input type="checkbox" id="ma-gravity" onchange="toggleGravitySub()"><span class="tgl"></span> \u91CD\u529B\u65B9\u5757\u6A21\u5F0F</label><div id="ma-gravity-sub" class="gone qopt-sub"><label class="lbl-cb"><input type="checkbox" id="ma-platform"><span class="tgl"></span> \u653E\u7F6E\u73BB\u7483\u5E73\u53F0</label><div class="dim" style="font-size:10px">\u6C34\u5E73\u5BFC\u5165\u65F6\u5728\u4E0B\u65B9\u94FA\u4E00\u5C42\u73BB\u7483</div></div></div><label>\u901F\u5EA6\uFF08\u65B9\u5757/\u79D2\uFF09</label><input id="ma-spd" type="number" value="' + cfgSpeed + '" style="margin-bottom:6px"><label>\u8272\u5F69\u7A7A\u95F4</label><div class="row" style="gap:6px;margin:2px 0 8px"><button class="btn-s" id="ma-cs-btn" onclick="showColorSpacePicker()" style="flex:1;text-align:left;font-size:12px;padding:10px 12px">LAB\uFF08\u63A8\u8350\uFF09</button></div><label>\u6296\u52A8\u7B97\u6CD5</label><div class="row" style="gap:6px;margin:2px 0 0"><button class="btn-s" id="ma-dither-btn" onclick="showDitherPicker()" style="flex:1;text-align:left;font-size:12px;padding:10px 12px">Floyd-Steinberg\uFF08\u63A8\u8350\uFF09</button></div><div class="row" style="margin-top:8px"><button class="btn gone" id="md-go" onclick="doMa()">\u5F00\u59CB\u751F\u6210</button><button class="btn-d gone" id="md-stop" onclick="doMaStop()">\u505C\u6B62</button></div><div class="pg gone" id="md-pg"><div id="md-bar"></div></div><div class="dim" id="ma-stat"></div></div><div class="card gone" id="ma-log-card"><div class="log" id="ma-log"></div></div>';
}
async function doMaPick() {
  var p = useBuiltinPicker ? await fpicker("image") : await pick("image");
  if (!p) return;
  showCropUI(p, function(croppedPath, cw, ch, cx, cy, clarity) {
    maPath = croppedPath;
    maCrop = { cx, cy, cw, ch };
    maClarity = clarity;
    showMaPreview(croppedPath, cw, ch, cx, cy, clarity);
  });
}
async function showMaPreview(path, tw, th, cx, cy, clarity) {
  var _a;
  document.getElementById("ma-pick-txt").innerHTML = '<span class="spin"></span> \u751F\u6210\u9884\u89C8...';
  var r = await A("POST", "/api/mapart/preview", {
    path,
    targetW: tw,
    targetH: th,
    orientation: document.getElementById("ma-ori").value,
    use_relief: ((_a = document.getElementById("ma-relief")) == null ? void 0 : _a.checked) || false,
    color_space: maCs,
    dither: maDither
  });
  if (!r.ok) {
    T("\u9884\u89C8\u5931\u8D25: " + r.error, "e");
    return;
  }
  document.getElementById("ma-pick-txt").textContent = "\u5DF2\u9009\u62E9: " + path.split("/").pop();
  document.getElementById("ma-info").classList.remove("gone");
  document.getElementById("ma-info").innerHTML = '<div class="stats"><div class="s"><div class="n">' + r.blocks + '</div><div class="l">\u65B9\u5757\u6570</div></div><div class="s"><div class="n">' + tw * clarity + "\xD7" + th * clarity + '</div><div class="l">\u5C3A\u5BF8</div></div></div>';
  var wEl = document.getElementById("ma-w"), hEl = document.getElementById("ma-h");
  if (wEl) wEl.value = tw * clarity;
  if (hEl) hEl.value = th * clarity;
  document.getElementById("ma-params").classList.remove("gone");
  document.getElementById("md-go").classList.remove("gone");
  document.getElementById("ma-align-btn").style.display = "";
  document.getElementById("md-go").textContent = "\u5F00\u59CB\u751F\u6210";
  document.getElementById("md-go").disabled = false;
  if (r.voxel && !r.voxel.too_large) {
    var v = document.createElement("div");
    v.className = "viewer";
    v.id = "viewer-mapart";
    document.getElementById("ma-info").appendChild(v);
    setTimeout(function() {
      if (typeof THREE !== "undefined") show3D(v, r.voxel);
    }, 100);
  }
  var refreshTimer;
  function scheduleRefresh() {
    clearTimeout(refreshTimer);
    refreshTimer = setTimeout(refreshMaPreview, 300);
  }
  if (wEl) wEl.addEventListener("input", scheduleRefresh);
  if (hEl) hEl.addEventListener("input", scheduleRefresh);
  document.getElementById("ma-ori").addEventListener("change", function() {
    toggleMaRelief();
    scheduleRefresh();
  });
  document.getElementById("ma-relief").addEventListener("change", function() {
    toggleMaRelief();
    scheduleRefresh();
  });
}
async function refreshMaPreview() {
  var _a;
  var w = parseInt(document.getElementById("ma-w").value) || maCrop.cw;
  var h = parseInt(document.getElementById("ma-h").value) || maCrop.ch;
  var path = maPath;
  if (!path) return;
  document.getElementById("ma-pick-txt").innerHTML = '<span class="spin"></span> \u91CD\u65B0\u89E3\u6790...';
  var r = await A("POST", "/api/mapart/preview", {
    path,
    targetW: w,
    targetH: h,
    orientation: document.getElementById("ma-ori").value,
    use_relief: ((_a = document.getElementById("ma-relief")) == null ? void 0 : _a.checked) || false,
    color_space: maCs,
    dither: maDither
  });
  if (!r.ok) {
    T("\u89E3\u6790\u5931\u8D25: " + r.error, "e");
    document.getElementById("ma-pick-txt").textContent = "\u5DF2\u9009\u62E9: " + path.split("/").pop();
    return;
  }
  document.getElementById("ma-pick-txt").textContent = "\u5DF2\u9009\u62E9: " + path.split("/").pop();
  document.getElementById("ma-info").innerHTML = '<div class="stats"><div class="s"><div class="n">' + r.blocks + '</div><div class="l">\u65B9\u5757\u6570</div></div><div class="s"><div class="n">' + w + "\xD7" + h + '</div><div class="l">\u5C3A\u5BF8</div></div></div>';
  var existing = document.getElementById("viewer-mapart");
  if (r.voxel && !r.voxel.too_large) {
    var v = existing || document.createElement("div");
    if (!existing) {
      v.className = "viewer";
      v.id = "viewer-mapart";
      document.getElementById("ma-info").appendChild(v);
    }
    setTimeout(function() {
      if (typeof THREE !== "undefined") show3D(v, r.voxel);
    }, 100);
  } else if (existing) {
    existing.remove();
  }
}
async function doMa() {
  var _a, _b, _c;
  if (checkUpdateLock()) return;
  var btn = document.getElementById("md-go");
  btn.disabled = true;
  btn.innerHTML = '<span class="spin"></span> \u542F\u52A8\u4E2D...';
  document.getElementById("md-stop").classList.remove("gone");
  document.getElementById("md-pg").classList.remove("gone");
  document.getElementById("ma-log-card").classList.remove("gone");
  var body = {
    type: "mapart",
    params: {
      path: maPath,
      target_w: parseInt(document.getElementById("ma-w").value) || maCrop.cw,
      target_h: parseInt(document.getElementById("ma-h").value) || maCrop.ch,
      crop_x: maCrop.cx,
      crop_y: maCrop.cy,
      clarity: maClarity,
      x: parseInt(document.getElementById("ma-x").value) || 0,
      y: parseInt(document.getElementById("ma-y").value) || 0,
      z: parseInt(document.getElementById("ma-z").value) || 0,
      orientation: document.getElementById("ma-ori").value,
      use_relief: ((_a = document.getElementById("ma-relief")) == null ? void 0 : _a.checked) || false,
      color_space: maCs,
      dither: maDither,
      use_gravity: ((_b = document.getElementById("ma-gravity")) == null ? void 0 : _b.checked) || false,
      gravity_platform: ((_c = document.getElementById("ma-platform")) == null ? void 0 : _c.checked) || false,
      speed: Math.min(Math.max(parseInt(document.getElementById("ma-spd").value) || 9500, 1), 2e4)
    }
  };
  var r = await A("POST", "/api/task/start", body);
  if (r.ok) {
    btn.innerHTML = "\u5F00\u59CB\u751F\u6210";
    btn.disabled = false;
    T("\u4EFB\u52A1\u5DF2\u542F\u52A8", "o");
  } else {
    T("\u542F\u52A8\u5931\u8D25: " + r.error, "e");
    btn.innerHTML = "\u5F00\u59CB\u751F\u6210";
    btn.disabled = false;
    document.getElementById("md-stop").classList.add("gone");
  }
}
function doMaStop() {
  A("POST", "/api/task/stop");
  T("\u5DF2\u505C\u6B62");
}
function alignMaCoords() {
  var xEl = document.getElementById("ma-x"), zEl = document.getElementById("ma-z");
  if (!xEl || !zEl) return;
  var curX = parseInt(xEl.value) || 0, curZ = parseInt(zEl.value) || 0;
  var bw = Math.round((maCrop.cw || 128) * maClarity);
  var bh = Math.round((maCrop.ch || 128) * maClarity);
  function snap(v) {
    var g = 128 << maClarity - 1, offset = 64;
    return Math.round((v + offset) / g) * g - offset;
  }
  xEl.value = snap(curX);
  zEl.value = snap(curZ);
  T("\u5DF2\u5BF9\u9F50\u5230 " + xEl.value + ", " + zEl.value, "o");
}
function toggleGravitySub() {
  var sub = document.getElementById("ma-gravity-sub");
  var grav = document.getElementById("ma-gravity");
  var ori = document.getElementById("ma-ori");
  var show = grav && grav.checked && ori && ori.value === "horizontal";
  if (sub) sub.classList.toggle("gone", !show);
}
function toggleMaRelief() {
  var _a;
  var relief = (_a = document.getElementById("ma-relief")) == null ? void 0 : _a.checked;
  var opts = document.getElementById("ma-gravity-opts");
  var ori = document.getElementById("ma-ori");
  if (opts) opts.style.display = relief || ori && ori.value !== "horizontal" ? "none" : "";
}
var maCs = "lab", maDither = "floyd_steinberg";
var CS_ITEMS = [
  { value: "lab", label: "LAB", desc: "\u6700\u7B26\u5408\u4EBA\u773C\u611F\u77E5\uFF0C\u989C\u8272\u8FC7\u6E21\u81EA\u7136", badge: "\u63A8\u8350" },
  { value: "rgb", label: "RGB", desc: "\u901F\u5EA6\u6700\u5FEB\uFF0C\u7EAF\u8272\u533A\u57DF\u5339\u914D\u597D" },
  { value: "hsv", label: "HSV", desc: "\u5BF9\u8272\u8C03\u66F4\u654F\u611F\uFF0C\u6E10\u53D8\u8272\u8868\u73B0\u597D" }
];
var DITHER_ITEMS = [
  { value: "none", label: "\u65E0", desc: "\u76F4\u63A5\u5339\u914D\uFF0C\u901F\u5EA6\u6700\u5FEB\uFF0C\u8272\u5757\u611F\u5F3A" },
  { value: "floyd_steinberg", label: "Floyd-Steinberg", desc: "\u7ECF\u5178\u7B97\u6CD5\uFF0C\u6548\u679C\u6700\u597D\uFF0C\u63A8\u8350\u5927\u591A\u6570\u573A\u666F", badge: "\u63A8\u8350" },
  { value: "atkinson", label: "Atkinson", desc: "\u8F7B\u91CF\u7248\uFF0C\u6269\u6563\u8303\u56F4\u5C0F\uFF0C\u9002\u5408\u5927\u56FE" },
  { value: "burkes", label: "Burkes", desc: "Floyd \u53D8\u4F53\uFF0C\u66F4\u5E73\u6ED1\u81EA\u7136" },
  { value: "stucki", label: "Stucki", desc: "\u6269\u6563\u8303\u56F4\u5E7F\uFF0C\u8D85\u5927\u56FE\u6548\u679C\u597D" },
  { value: "jarvis", label: "Jarvis", desc: "12\u65B9\u5411\u6269\u6563\uFF0C\u6700\u7EC6\u817B\u7684\u8BEF\u5DEE\u6269\u6563" },
  { value: "bayer_2x2", label: "Bayer 2\xD72", desc: "\u6709\u5E8F\u6296\u52A8\uFF0C\u901F\u5EA6\u5FEB\uFF0C\u6709\u56FA\u5B9A\u7EB9\u7406" },
  { value: "bayer_4x4", label: "Bayer 4\xD74", desc: "\u6BD42\xD72\u66F4\u7EC6\u817B" },
  { value: "bayer_8x8", label: "Bayer 8\xD78", desc: "\u6700\u7EC6\u817B\u7684\u6709\u5E8F\u6296\u52A8" },
  { value: "ordered_3x3", label: "Ordered 3\xD73", desc: "\u53E6\u4E00\u79CD\u6709\u5E8F\u6296\u52A8\u56FE\u6848" }
];
function showColorSpacePicker() {
  showMaPicker("\u8272\u5F69\u7A7A\u95F4", CS_ITEMS, maCs, function(v, item) {
    maCs = v;
    document.getElementById("ma-cs-btn").textContent = item.label + (item.badge ? "\uFF08" + item.badge + "\uFF09" : "");
    if (maPath) refreshMaPreview();
  });
}
function showDitherPicker() {
  showMaPicker("\u6296\u52A8\u7B97\u6CD5", DITHER_ITEMS, maDither, function(v, item) {
    maDither = v;
    document.getElementById("ma-dither-btn").textContent = item.label + (item.badge ? "\uFF08" + item.badge + "\uFF09" : "");
    if (maPath) refreshMaPreview();
  });
}
function showMaPicker(title, items, selected, cb) {
  var overlay = document.createElement("div");
  overlay.className = "bs-overlay";
  var itemsHtml = "";
  for (var i = 0; i < items.length; i++) {
    var item = items[i];
    var sel = item.value === selected;
    itemsHtml += '<div class="bs-option' + (sel ? " active" : "") + '" data-value="' + item.value + '"><div class="bs-dot"></div><span class="bs-label">' + item.label + '</span><span class="dim" style="font-size:11px;text-align:right;max-width:140px">' + item.desc + "</span></div>";
  }
  overlay.innerHTML = '<div class="bs-sheet"><div class="bs-title">' + title + '</div><div class="bs-body">' + itemsHtml + '</div><div class="bs-footer"><button class="bs-btn bs-btn-cancel">\u53D6\u6D88</button><button class="bs-btn bs-btn-confirm">\u786E\u5B9A</button></div></div>';
  document.body.appendChild(overlay);
  var selVal = selected;
  overlay.querySelectorAll(".bs-option").forEach(function(opt) {
    opt.addEventListener("click", function() {
      overlay.querySelectorAll(".bs-option").forEach(function(o) {
        o.classList.remove("active");
      });
      this.classList.add("active");
      selVal = this.getAttribute("data-value");
    });
  });
  overlay.querySelector(".bs-btn-cancel").addEventListener("click", function() {
    closeOverlay(overlay);
  });
  overlay.querySelector(".bs-btn-confirm").addEventListener("click", function() {
    closeOverlay(overlay);
    var found = null;
    for (var i2 = 0; i2 < items.length; i2++) {
      if (items[i2].value === selVal) {
        found = items[i2];
        break;
      }
    }
    if (found && cb) cb(selVal, found);
  });
  overlay.addEventListener("click", function(e) {
    if (!e.target.closest(".bs-sheet")) closeOverlay(overlay);
  });
}
var RATIOS = [[1, 3], [1, 2], [2, 3], [3, 4], [1, 1], [4, 3], [3, 2], [2, 1], [3, 1]];
function showCropUI(path, cb) {
  var ov = document.createElement("div");
  ov.className = "crop-overlay";
  ov.innerHTML = '<canvas id="crop-canvas" style="position:absolute;top:0;left:0;right:0;bottom:88px;width:100%"></canvas><div class="crop-bottombar"><button class="crop-cancel" id="crop-cancel" style="flex-shrink:0;width:40px;height:40px;display:flex;align-items:center;justify-content:center">' + ICONS.close + '</button><div class="crop-label" id="crop-label" style="flex:1;text-align:center;font-size:12px;font-weight:700;color:rgba(255,255,255,.7);padding:0 8px">\u52A0\u8F7D\u4E2D...</div><div style="display:flex;align-items:center;gap:8px;flex-shrink:0;font-size:12px;color:#fff"><span>\u6E05\u6670\u5EA6</span><input type="range" id="crop-clarity" min="1" max="3" value="1" style="width:120px;height:6px;accent-color:var(--phx-primary,#d48a0e)"><span id="crop-clarity-label" style="min-width:18px;font-weight:700;font-size:14px">1</span><button class="crop-done" id="crop-done" style="padding:8px 16px;border-radius:20px;font-size:14px">' + ICONS.check + ' \u786E\u8BA4</button></div></div><div id="crop-err" class="gone" style="position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);color:#ff6b6b;font-size:15px;font-weight:700;text-align:center;z-index:50;pointer-events:none;line-height:1.6"></div>';
  document.body.appendChild(ov);
  var cv = document.getElementById("crop-canvas"), ctx = cv.getContext("2d");
  var errEl = document.getElementById("crop-err");
  var labelEl = document.getElementById("crop-label");
  var vw = ov.clientWidth, vh = ov.clientHeight - 88;
  if (vw < 1) vw = window.innerWidth;
  if (vh < 1) vh = window.innerHeight - 88;
  cv.width = vw;
  cv.height = vh;
  var img = new Image();
  var loadTimer = setTimeout(function() {
    if (!img.complete) {
      img.src = "";
      if (img.onerror) img.onerror();
    }
  }, 3e4);
  img.onerror = function() {
    clearTimeout(loadTimer);
    errEl.classList.remove("gone");
    errEl.innerHTML = '\u26A0 \u56FE\u7247\u52A0\u8F7D\u5931\u8D25<br><span style="font-size:12px;font-weight:400;color:rgba(255,107,107,.7)">\u8BF7\u68C0\u67E5\u7F51\u7EDC\u540E\u91CD\u8BD5</span>';
    labelEl.textContent = "\u52A0\u8F7D\u5931\u8D25";
  };
  img.onload = function() {
    clearTimeout(loadTimer);
    errEl.classList.add("gone");
    labelEl.textContent = "\u88C1\u526A\u56FE\u7247";
    var iw = img.width, ih = img.height;
    ctx.imageSmoothingEnabled = false;
    var clarity = 1;
    var g = snapToStd(iw, ih);
    var cw = g.pw, ch = g.ph;
    var cx = Math.round((iw - cw) / 2), cy = Math.round((ih - ch) / 2);
    var sc = Math.min(vw / iw, vh / ih, 1) * 0.85;
    var ox = (vw - iw * sc) / 2, oy = (vh - ih * sc) / 2;
    var drag = null;
    var dlx = 0, dly = 0;
    var pinchDist = 0;
    var cornerR = 28;
    var edgeR = 38;
    function snapToStd(rcw, rch) {
      var ar = rcw / rch;
      var bi = 0, bs = Infinity;
      for (var i = 0; i < RATIOS.length; i++) {
        var a = RATIOS[i][0], b = RATIOS[i][1], sa = a / b;
        var s = Math.abs(sa - ar);
        if (s < bs) {
          bs = s;
          bi = i;
        }
      }
      var a = RATIOS[bi][0], b = RATIOS[bi][1];
      var k = Math.min(Math.floor(rcw / a), Math.floor(rch / b));
      if (k < 1) k = 1;
      return { pw: a * k, ph: b * k, ai: bi };
    }
    function rd() {
      ctx.clearRect(0, 0, cv.width, cv.height);
      ctx.fillStyle = "#2b2118";
      ctx.fillRect(0, 0, cv.width, cv.height);
      ctx.drawImage(img, ox, oy, iw * sc, ih * sc);
      var bx = ox + cx * sc, by = oy + cy * sc, bw = cw * sc, bh = ch * sc;
      ctx.fillStyle = "rgba(0,0,0,.35)";
      ctx.fillRect(0, 0, cv.width, by);
      ctx.fillRect(0, by + bh, cv.width, cv.height - by - bh);
      ctx.fillRect(0, by, bx, bh);
      ctx.fillRect(bx + bw, by, cv.width - bx - bw, bh);
      ctx.strokeStyle = "#fff";
      ctx.lineWidth = 2.5;
      ctx.strokeRect(bx, by, bw, bh);
      var gcx = ox + (cx + (cw - g.pw) / 2) * sc, gcy = oy + (cy + (ch - g.ph) / 2) * sc;
      var ghw = g.pw * sc, ghh = g.ph * sc;
      ctx.setLineDash([5, 3]);
      ctx.strokeStyle = "rgba(255,179,71,.85)";
      ctx.lineWidth = 2;
      ctx.strokeRect(gcx, gcy, ghw, ghh);
      ctx.setLineDash([]);
      ctx.fillStyle = "rgba(255,179,71,.12)";
      ctx.fillRect(gcx, gcy, ghw, ghh);
      var cr = cornerR;
      [[bx, by], [bx + bw, by], [bx, by + bh], [bx + bw, by + bh]].forEach(function(p) {
        ctx.fillStyle = "#fff";
        ctx.beginPath();
        ctx.arc(p[0], p[1], cr, 0, 6.28);
        ctx.fill();
        ctx.fillStyle = "#333";
        ctx.beginPath();
        ctx.arc(p[0], p[1], cr - 3, 0, 6.28);
        ctx.fill();
      });
      var ri = g.ai, ra = RATIOS[ri][0], rb = RATIOS[ri][1];
      var outW = ra * 128 * clarity, outH = rb * 128 * clarity;
      labelEl.textContent = outW + "\xD7" + outH + " \u65B9\u5757  [" + ra + ":" + rb + "] \xD7" + clarity;
      maCrop = { cx: Math.round(cx + (cw - g.pw) / 2), cy: Math.round(cy + (ch - g.ph) / 2), cw: g.pw, ch: g.ph };
    }
    function hitCorner(px, py) {
      var bx = ox + cx * sc, by = oy + cy * sc, bw = cw * sc, bh = ch * sc;
      var cr = cornerR + 24;
      function d(x, y) {
        return Math.hypot(px - x, py - y) <= cr;
      }
      if (d(bx, by)) return "tl";
      if (d(bx + bw, by)) return "tr";
      if (d(bx, by + bh)) return "bl";
      if (d(bx + bw, by + bh)) return "br";
      var er = edgeR;
      var inL = Math.abs(px - bx) <= er && py > by + cr && py < by + bh - cr;
      var inR = Math.abs(px - (bx + bw)) <= er && py > by + cr && py < by + bh - cr;
      var inT = Math.abs(py - by) <= er && px > bx + cr && px < bx + bw - cr;
      var inB = Math.abs(py - (by + bh)) <= er && px > bx + cr && px < bx + bw - cr;
      if (inT) return "t";
      if (inB) return "b";
      if (inL) return "l";
      if (inR) return "r";
      return null;
    }
    ov.addEventListener("pointerdown", function(e) {
      if (errEl && !errEl.classList.contains("gone")) return;
      if (e.target.closest(".crop-bottombar")) return;
      var p = { x: e.clientX, y: e.clientY };
      var cn = hitCorner(p.x, p.y);
      drag = cn || "move";
      dlx = p.x;
      dly = p.y;
      e.preventDefault();
    });
    ov.addEventListener("pointermove", function(e) {
      if (!drag) return;
      var dx = e.clientX - dlx, dy = e.clientY - dly;
      dlx = e.clientX;
      dly = e.clientY;
      if (drag === "move") {
        ox += dx;
        oy += dy;
        var minOx = cv.width - iw * sc - 20, maxOx = 20;
        var minOy = cv.height - ih * sc - 20, maxOy = 20;
        ox = Math.max(minOx, Math.min(maxOx, ox));
        oy = Math.max(minOy, Math.min(maxOy, oy));
        rd();
        return;
      }
      var ndx = dx / sc, ndy = dy / sc;
      if (drag.indexOf("t") >= 0) {
        cy += ndy;
        ch -= ndy;
      }
      if (drag.indexOf("b") >= 0) {
        ch += ndy;
      }
      if (drag.indexOf("l") >= 0) {
        cx += ndx;
        cw -= ndx;
      }
      if (drag.indexOf("r") >= 0) {
        cw += ndx;
      }
      var minS = 128;
      cw = Math.max(minS, Math.min(iw - cx, cw));
      ch = Math.max(minS, Math.min(ih - cy, ch));
      cx = Math.max(0, Math.min(iw - minS, cx));
      cy = Math.max(0, Math.min(ih - minS, cy));
      g = snapToStd(cw, ch);
      rd();
    });
    ov.addEventListener("pointerup", function() {
      if (!drag) return;
      drag = null;
    });
    ov.addEventListener("pointerleave", function() {
      if (!drag) return;
      drag = null;
    });
    ov.addEventListener("wheel", function(e) {
      if (errEl && !errEl.classList.contains("gone")) return;
      e.preventDefault();
      var oldS = sc;
      sc *= e.deltaY < 0 ? 1.1 : 0.9;
      sc = Math.max(0.1, Math.min(5, sc));
      var r = e.clientX - ox, r2 = e.clientY - oy;
      ox = e.clientX - r / oldS * sc;
      oy = e.clientY - r2 / oldS * sc;
      rd();
    }, { passive: false });
    ov.addEventListener("touchstart", function(e) {
      if (e.touches.length === 2) {
        var t = e.touches;
        pinchDist = Math.hypot(t[0].clientX - t[1].clientX, t[0].clientY - t[1].clientY);
        drag = null;
      }
    }, { passive: true });
    ov.addEventListener("touchmove", function(e) {
      if (e.touches.length === 2 && pinchDist > 0) {
        e.preventDefault();
        var t = e.touches, d2 = Math.hypot(t[0].clientX - t[1].clientX, t[0].clientY - t[1].clientY);
        sc *= d2 / pinchDist;
        sc = Math.max(0.1, Math.min(5, sc));
        pinchDist = d2;
        rd();
      }
    }, { passive: false });
    var cancelBtn = document.getElementById("crop-cancel");
    var clarityInput = document.getElementById("crop-clarity");
    var clarityLabel = document.getElementById("crop-clarity-label");
    var doneBtn = document.getElementById("crop-done");
    cancelBtn.addEventListener("click", function() {
      ov.remove();
    });
    clarityInput.addEventListener("input", function() {
      clarity = parseInt(this.value);
      clarityLabel.textContent = clarity;
      rd();
    });
    doneBtn.addEventListener("click", function() {
      try {
        var ri = g.ai, a = RATIOS[ri][0], b = RATIOS[ri][1];
        var dw = a * 128 * clarity, dh = b * 128 * clarity;
        var srcX = Math.round(cx + (cw - g.pw) / 2);
        var srcY = Math.round(cy + (ch - g.ph) / 2);
        var cc = document.createElement("canvas");
        cc.width = dw;
        cc.height = dh;
        var cctx = cc.getContext("2d");
        cctx.imageSmoothingEnabled = false;
        cctx.drawImage(img, srcX, srcY, g.pw, g.ph, 0, 0, dw, dh);
        cc.toBlob(function(blob) {
          if (!blob) {
            showAlertModal("\u9519\u8BEF", "\u88C1\u526A\u5931\u8D25\uFF0C\u8BF7\u91CD\u8BD5");
            return;
          }
          var fd = new FormData();
          fd.append("file", blob, "crop_" + Date.now() + ".png");
          fetch("/api/file/upload", { method: "POST", body: fd }).then(function(r) {
            return r.json();
          }).then(function(u) {
            ov.remove();
            if (u.ok && u.path) {
              maPath = u.path;
              maClarity = clarity;
              maCrop = { cx: srcX, cy: srcY, cw: dw, ch: dh };
              cb(u.path, dw, dh, srcX, srcY, clarity);
            } else showAlertModal("\u4E0A\u4F20\u5931\u8D25", u.error || "\u670D\u52A1\u5668\u9519\u8BEF");
          }).catch(function(e) {
            showAlertModal("\u4E0A\u4F20\u5931\u8D25", e.message);
          });
        }, "image/png");
      } catch (e) {
        showAlertModal("\u88C1\u526A\u51FA\u9519", e.message);
      }
    });
    g = snapToStd(cw, ch);
    rd();
  };
  img.src = "/uploads/" + encodeURIComponent(path.split("/").pop());
}
