var impPath = "";
function showPicker(opts) {
  var overlay = document.createElement("div");
  overlay.className = "bs-overlay";
  var itemsHtml = "";
  for (var i = 0; i < opts.items.length; i++) {
    var item = opts.items[i];
    var sel = item.value === opts.selected;
    itemsHtml += '<div class="bs-option' + (sel ? " active" : "") + '" data-value="' + item.value + '"><div class="bs-dot"></div><span class="bs-label">' + item.label + "</span></div>";
  }
  overlay.innerHTML = '<div class="bs-sheet">' + (opts.title ? '<div class="bs-title">' + opts.title + "</div>" : "") + '<div class="bs-body">' + itemsHtml + '</div><div class="bs-footer"><button class="bs-btn bs-btn-cancel">\u53D6\u6D88</button><button class="bs-btn bs-btn-confirm">\u786E\u5B9A</button></div></div>';
  document.body.appendChild(overlay);
  var selVal = opts.selected;
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
    if (opts.onConfirm) opts.onConfirm(selVal);
  });
  overlay.addEventListener("click", function(e) {
    if (!e.target.closest(".bs-sheet")) closeOverlay(overlay);
  });
}
var ROT_ITEMS = [{ value: "0", label: "0\xB0" }, { value: "90", label: "90\xB0" }, { value: "180", label: "180\xB0" }, { value: "270", label: "270\xB0" }];
var DIM_ITEMS = [{ value: "overworld", label: "\u4E3B\u4E16\u754C" }, { value: "nether", label: "\u4E0B\u754C" }, { value: "the_end", label: "\u672B\u5730" }, { value: "dm", label: "DM" }];
var REGION_ITEMS = [{ value: "1", label: "\u5355\u533A\u5757 16\xD716\uFF08\u5F53\u524D\uFF09" }, { value: "2", label: "2\xD72 \u5C0F\u533A 32\xD732" }, { value: "3", label: "4\xD74 \u5C0F\u533A 64\xD764" }, { value: "4", label: "5×5 区块 · 80×80" }];
var PRECLEAR_ITEMS = [{ value: "0", label: "\u5173\u95ED" }, { value: "1", label: "\u4E00\u683C \u2014 \u6E05\u7A7A\u5EFA\u7B51\u77E9\u5F62\u7A7A\u95F4" }, { value: "2", label: "\u7AD6\u67F1 \u2014 \u57FA\u70B9\u5230\u9AD8\u5EA6\u4E0A\u9650" }, { value: "3", label: "\u533A\u5757 \u2014 \u6E05\u7A7A\u6574\u533A\u5757\u5217" }];
var PRECLEAR_SHORT = { "0": "\u5173\u95ED", "1": "\u4E00\u683C", "2": "\u7AD6\u67F1", "3": "\u533A\u5757" };
var EXP_ITEMS = [{ value: "mcstructure", label: "MCStructure" }, { value: "schematic", label: "Schematic" }, { value: "mcworld", label: "MCWorld \u4E16\u754C\u6587\u4EF6" }];
function showRotPicker() {
  showPicker({
    title: "\u9009\u62E9\u65CB\u8F6C",
    items: ROT_ITEMS,
    selected: document.getElementById("imp-rot-picker").getAttribute("data-value"),
    onConfirm: function(v) {
      var p = document.getElementById("imp-rot-picker");
      p.setAttribute("data-value", v);
      p.querySelector(".picker-label").textContent = ROT_ITEMS.find(function(i) {
        return i.value === v;
      }).label;
    }
  });
}
function showDimPicker() {
  showPicker({
    title: "\u9009\u62E9\u7EF4\u5EA6",
    items: DIM_ITEMS,
    selected: document.getElementById("imp-dim-picker").getAttribute("data-value"),
    onConfirm: function(v) {
      var p = document.getElementById("imp-dim-picker");
      p.setAttribute("data-value", v);
      p.querySelector(".picker-label").textContent = DIM_ITEMS.find(function(i) {
        return i.value === v;
      }).label;
      toggleDimInput();
    }
  });
}
function showRegionPicker() {
  showPicker({
    title: "\u9009\u62E9\u5BFC\u5165\u7C92\u5EA6",
    items: REGION_ITEMS,
    selected: document.getElementById("imp-region-picker").getAttribute("data-value"),
    onConfirm: function(v) {
      var p = document.getElementById("imp-region-picker");
      p.setAttribute("data-value", v);
      p.querySelector(".picker-label").textContent = REGION_ITEMS.find(function(i) {
        return i.value === v;
      }).label;
    }
  });
}
function showPreclearPicker() {
  showPicker({
    title: "\u9009\u62E9\u9884\u6E05\u7A7A\u6A21\u5F0F",
    items: PRECLEAR_ITEMS,
    selected: document.getElementById("imp-preclear-picker").getAttribute("data-value"),
    onConfirm: function(v) {
      var p = document.getElementById("imp-preclear-picker");
      p.setAttribute("data-value", v);
      p.querySelector(".picker-label").textContent = PRECLEAR_SHORT[v] || "\u5173\u95ED";
      if (v !== "0") {
        var k = document.getElementById("imp-keep");
        if (k) k.checked = false;
      }
    }
  });
}
function onImpKeepToggle() {
  var k = document.getElementById("imp-keep");
  if (k && k.checked) {
    var p = document.getElementById("imp-preclear-picker");
    if (p) {
      p.setAttribute("data-value", "0");
      p.querySelector(".picker-label").textContent = "\u5173\u95ED";
    }
  }
}
function onExcludeNBTChange() {
  var cb = document.getElementById("imp-exclude-nbt");
  var opts = document.getElementById("imp-cmd-opts");
  if (!cb || !opts) return;
  opts.classList.toggle("gone", cb.checked);
  if (cb.checked) {
    var c = document.getElementById("imp-cmds");
    if (c) c.checked = false;
  }
}
function showExpFmtPicker() {
  showPicker({
    title: "\u9009\u62E9\u5BFC\u51FA\u683C\u5F0F",
    items: EXP_ITEMS,
    selected: document.getElementById("exp-fmt-picker").getAttribute("data-value"),
    onConfirm: function(v) {
      var p = document.getElementById("exp-fmt-picker");
      p.setAttribute("data-value", v);
      p.querySelector(".picker-label").textContent = EXP_ITEMS.find(function(i) {
        return i.value === v;
      }).label;
      onExpFmtChange();
    }
  });
}
function RimpHTML() {
  var html = '<div class="card"><div class="pick" onclick="doPick()"><i class="pi">' + ICONS.upload + `</i><div id="imp-pick-txt">\u70B9\u51FB\u9009\u62E9\u5EFA\u7B51\u6587\u4EF6</div><div style="font-size:10px;color:var(--phx-text-secondary)">.mcstructure .schematic .schem .mcworld .bdx</div></div><div class="mode-cap gone" id="imp-mode-cap"><div class="mode-cap-thumb" id="imp-mode-thumb"></div><button class="mode-cap-btn active" data-mode="import" onclick="setImpMode('import')">\u5BFC\u5165\u6A21\u5F0F</button><button class="mode-cap-btn" data-mode="multi" onclick="setImpMode('multi')">\u591A\u673A\u5668\u4EBA / \u591A\u533A\u5757</button><button class="mode-cap-btn" data-mode="repair" onclick="setImpMode('repair')">\u4FEE\u8865\u6A21\u5F0F</button></div><div id="imp-info" class="gone"></div><div id="imp-mcworld-params" class="gone"><label>\u8D77\u70B9\u5750\u6807</label><div class="row"><input id="imp-x1" type="number" placeholder="X"><input id="imp-y1" type="number" placeholder="Y"><input id="imp-z1" type="number" placeholder="Z"></div><label>\u7EC8\u70B9\u5750\u6807</label><div class="row"><input id="imp-x2" type="number" placeholder="X"><input id="imp-y2" type="number" placeholder="Y"><input id="imp-z2" type="number" placeholder="Z"></div><button class="btn" id="imp-reanalyze" onclick="doReanalyze()" style="width:auto;padding:8px 14px">\u91CD\u65B0\u89E3\u6790</button></div><div id="imp-params" class="gone"><label>\u653E\u7F6E\u5750\u6807</label><div class="row"><input id="imp-x" type="number" placeholder="X"><input id="imp-y" type="number" placeholder="Y"><input id="imp-z" type="number" placeholder="Z"></div><label>\u65CB\u8F6C</label><div class="picker-btn" id="imp-rot-picker" data-value="0" onclick="showRotPicker()"><span class="picker-label">0\xB0</span><span class="picker-arrow">\u25BE</span></div><div class="qopt" style="margin-top:12px;padding-top:12px;border-top:2px solid var(--phx-border-light)"><label class="lbl-cb"><input type="checkbox" id="imp-keep" onchange="onImpKeepToggle()"><span class="tgl"></span> \u4FDD\u7559\u6A21\u5F0F\uFF08\u4E0D\u7834\u574F\u539F\u6709\u5730\u5F62\uFF09</label><div class="dim" style="font-size:10px;margin-top:4px">setblock/fill \u4EC5\u66FF\u6362\u7A7A\u6C14\u4F4D\uFF0CNBT \u65B9\u5757\u76EE\u6807\u4F4D\u975E\u7A7A\u6C14\u5219\u8DF3\u8FC7\uFF1B\u4E0E\u9884\u6E05\u7A7A\u4E92\u65A5</div></div><div class="qopt" style="margin-top:12px;padding-top:12px;border-top:2px solid var(--phx-border-light)"><label class="lbl-cb"><input type="checkbox" id="imp-exclude-nbt" onchange="onExcludeNBTChange()"><span class="tgl"></span> \u6392\u9664NBT</label><div class="dim" style="font-size:10px;margin-top:4px">\u5BB9\u5668/\u65D7\u5E1C/\u544A\u793A\u724C/\u547D\u4EE4\u65B9\u5757\u7B49\u4E0D\u5199\u5165NBT\u6570\u636E\uFF0C\u4EC5\u653E\u7F6E\u666E\u901A\u65B9\u5757\uFF08\u4EC5\u5355\u673A\u5668\u4EBA\u751F\u6548\uFF09</div></div><div id="imp-cmd-opts" class="gone qopt"><label class="lbl-cb"><input type="checkbox" id="imp-cmds" checked onchange="toggleSub('imp-cmds','imp-cmd-sub')"><span class="tgl"></span> \u5BFC\u5165\u6307\u4EE4</label><div id="imp-cmd-sub" class="qopt-sub"><label class="lbl-cb"><input type="checkbox" id="imp-cmd-disabled"><span class="tgl"></span> \u5173\u95ED\u72B6\u6001\u4E0B\u5BFC\u5165</label><div class="dim" style="font-size:10px">\u5F00\u542F\u540E\uFF0C\u5FAA\u73AF\u547D\u4EE4\u65B9\u5757\u59CB\u7EC8\u4EE5\u7EA2\u77F3\u63A7\u5236\u6A21\u5F0F\u5BFC\u5165</div></div></div><label>\u901F\u5EA6\uFF08\u65B9\u5757/\u79D2\uFF09</label><input id="imp-spd" type="number" value="` + cfgSpeed + `"><div id="imp-adv" class="gone" style="margin-top:12px;padding-top:12px;border-top:2px solid var(--phx-border-light)"><div id="imp-fluid-opts" class="qopt"><label class="lbl-cb"><input type="checkbox" id="imp-exclude-fluids" onchange="toggleSub('imp-exclude-fluids','imp-fluid-sub')"><span class="tgl"></span> \u6392\u9664\u6D41\u4F53\u65B9\u5757</label><div id="imp-fluid-sub" class="gone qopt-sub"><label class="lbl-cb"><input type="checkbox" id="imp-exclude-water" checked><span class="tgl"></span> \u6392\u9664\u6C34</label><label class="lbl-cb"><input type="checkbox" id="imp-exclude-waterlogged" checked><span class="tgl"></span> \u6392\u9664\u542B\u6C34\u65B9\u5757</label><div class="dim" style="font-size:10px;margin-bottom:4px">\u4EC5\u653E\u7F6E\u65B9\u5757\u672C\u8EAB\uFF0C\u4E0D\u653E\u7F6E\u5176\u4E2D\u7684\u6C34</div><label class="lbl-cb"><input type="checkbox" id="imp-exclude-lava" checked><span class="tgl"></span> \u6392\u9664\u5CA9\u6D46</label></div></div><div id="imp-deny-opts" class="qopt"><label class="lbl-cb"><input type="checkbox" id="imp-deny" onchange="toggleSub('imp-deny','imp-deny-sub')"><span class="tgl"></span> \u62D2\u7EDD\u65B9\u5757\u5C42</label><div id="imp-deny-sub" class="gone qopt-sub"><label>Y\u504F\u79FB</label><input id="imp-deny-offset" type="number" value="-1" style="width:80px"><div class="dim" style="font-size:10px">\u5BFC\u5165\u5750\u6807\u4E0B\u65B9\u683C\u6570\uFF08\u8D1F=\u4E0B\u65B9\uFF09</div></div></div><div id="imp-border-opts" class="qopt"><label class="lbl-cb"><input type="checkbox" id="imp-border" onchange="toggleSub('imp-border','imp-border-sub')"><span class="tgl"></span> \u8FB9\u754C\u65B9\u5757</label><div id="imp-border-sub" class="gone qopt-sub"><label>Y\u5750\u6807</label><input id="imp-border-y" type="number" value="0" style="width:80px"></div></div><label>\u7EF4\u5EA6</label><div class="picker-btn" id="imp-dim-picker" data-value="overworld" onclick="showDimPicker()"><span class="picker-label">\u4E3B\u4E16\u754C</span><span class="picker-arrow">\u25BE</span></div><div id="imp-dim-dm" class="gone"><label>DM ID</label><input id="imp-dim-id" type="number" value="3" style="width:80px"></div><label>\u5BFC\u5165\u7C92\u5EA6</label><div class="picker-btn" id="imp-region-picker" data-value="` + cfgRegionMode + '" onclick="showRegionPicker()"><span class="picker-label">' + (cfgRegionMode === "2" ? "2\xD72 \u5C0F\u533A 32\xD732" : cfgRegionMode === "3" ? "4\xD74 \u5C0F\u533A 64\xD764" : "\u5355\u533A\u5757 16\xD716\uFF08\u5F53\u524D\uFF09") + '</span><span class="picker-arrow">\u25BE</span></div><div id="imp-preclear-opts" class="qopt" style="margin-top:12px;padding-top:12px;border-top:2px solid var(--phx-border-light)"><label style="font-weight:700;margin-bottom:4px">\u9884\u6E05\u7A7A</label><div class="picker-btn" id="imp-preclear-picker" data-value="0" onclick="showPreclearPicker()"><span class="picker-label">\u5173\u95ED</span><span class="picker-arrow">\u25BE</span></div><div class="dim" style="font-size:10px;margin-top:4px">\u5BFC\u5165\u524D\u5148\u7528\u7A7A\u6C14\u586B\u5145\u76EE\u6807\u533A\u57DF\uFF0C\u6E05\u7A7A\u5B8C\u6210\u540E\u624D\u5BFC\u5165\u5EFA\u7B51</div></div></div><button class="btn-s" id="imp-adv-btn" onclick="toggleAdvImp()" style="width:100%;margin-top:10px;display:none">\u5C55\u5F00\u9AD8\u7EA7\u9009\u9879</button></div><div id="imp-repair-params" class="gone"><label style="font-weight:700;color:var(--phx-primary);margin-top:8px">\u4FEE\u8865\u6A21\u5F0F</label><div class="dim" style="font-size:10px;margin-bottom:6px">\u8D77\u59CB\u5750\u6807\u5FC5\u987B\u4E0E\u5BFC\u5165\u65F6\u4E00\u81F4</div><label>\u8D77\u59CB\u5750\u6807</label><div class="row"><input id="rep-x" type="number" placeholder="X"><input id="rep-y" type="number" placeholder="Y"><input id="rep-z" type="number" placeholder="Z"></div><label>\u4FEE\u8865\u533A\u57DF</label><div class="picker-btn" id="rep-area-picker" data-value="rect" onclick="showRepairAreaPicker()"><span class="picker-label">\u77E9\u5F62\u8303\u56F4</span><span class="picker-arrow">\u25BE</span></div><div id="rep-rect-params"><label>\u8D77\u70B9</label><div class="row"><input id="rep-x1" type="number" placeholder="X"><input id="rep-y1" type="number" placeholder="Y"><input id="rep-z1" type="number" placeholder="Z"></div><label>\u7EC8\u70B9</label><div class="row"><input id="rep-x2" type="number" placeholder="X"><input id="rep-y2" type="number" placeholder="Y"><input id="rep-z2" type="number" placeholder="Z"></div></div><div id="rep-circle-params" class="gone"><label>\u4E2D\u5FC3\u5750\u6807</label><div class="row"><input id="rep-cx" type="number" placeholder="X"><input id="rep-cz" type="number" placeholder="Z"></div><label>\u534A\u5F84</label><input id="rep-radius" type="number" placeholder="\u534A\u5F84" value="16"><button class="btn-s" onclick="fillBotPos()" style="width:auto;padding:8px 14px;margin:4px 0;font-size:12px">\u4F7F\u7528\u673A\u5668\u4EBA\u5F53\u524D\u4F4D\u7F6E</button></div><label>\u65CB\u8F6C</label><div class="picker-btn" id="rep-rot-picker" data-value="0" onclick="showRepRotPicker()"><span class="picker-label">0\xB0</span><span class="picker-arrow">\u25BE</span></div><label>\u901F\u5EA6\uFF08\u65B9\u5757/\u79D2\uFF09</label><input id="rep-spd" type="number" value="20"><div class="dim" style="font-size:10px">\u4FEE\u8865\u6A21\u5F0F\u9ED8\u8BA4\u901F\u5EA6 20\uFF0C\u5EFA\u8BAE\u4E0D\u8D85\u8FC7 50</div><label>\u7EF4\u5EA6</label><div class="picker-btn" id="rep-dim-picker" data-value="overworld" onclick="showRepDimPicker()"><span class="picker-label">\u4E3B\u4E16\u754C</span><span class="picker-arrow">\u25BE</span></div><div class="qopt" style="margin-top:8px;padding-top:8px;border-top:2px solid var(--phx-border-light)"><label class="lbl-cb"><input type="checkbox" id="rep-nbt-only"><span class="tgl"></span> \u4EC5\u4FEE\u8865NBT</label><div class="dim" style="font-size:10px;margin-bottom:4px">\u53EA\u4FEE\u8865\u547D\u4EE4\u65B9\u5757/\u5BB9\u5668/\u544A\u793A\u724C\u7B49NBT\u6570\u636E\uFF0C\u8DF3\u8FC7\u666E\u901A\u65B9\u5757</div><label class="lbl-cb"><input type="checkbox" id="rep-cmds" checked><span class="tgl"></span> \u5BFC\u5165\u6307\u4EE4</label><label class="lbl-cb"><input type="checkbox" id="rep-cmd-disabled"><span class="tgl"></span> \u5173\u95ED\u72B6\u6001\u4E0B\u5BFC\u5165</label></div></div><div class="row"><button class="btn gone" id="imp-go" onclick="doImp()">\u5F00\u59CB\u5BFC\u5165</button><button class="btn-d gone" id="imp-stop" onclick="doImpStop()">\u505C\u6B62\u5BFC\u5165</button></div><div class="yy-transfer-meta"><span class="yy-transfer-tag">\u6E38\u56ED \xB7 \u5EFA\u7B51\u5199\u5165</span><small>\u5DE5\u5177\u7BB1\u8BBE\u5907\u8BFB\u53D6\u6587\u4EF6 \u2192 \u5F53\u524D\u6E38\u620F\u670D\u52A1\u5668</small></div><div class="pg gone yy-transfer-bar" id="imp-pg"><div id="imp-bar"></div></div><div class="gone" id="imp-multi-pg" style="margin-top:6px"><div class="mp-row"><span class="mp-label">\u65B9\u5757\u5199\u5165</span><div class="mp-bar"><div id="mp-bar-normal" style="width:0%"></div></div><span class="mp-pct" id="mp-pct-normal">\u7B49\u5F85\u5F00\u59CB</span></div><div class="mp-row"><span class="mp-label" style="color:#dc9b40">\u5B9E\u4F53\u6570\u636E</span><div class="mp-bar"><div id="mp-bar-nbt" style="width:0%;background:#dc9b40"></div></div><span class="mp-pct" id="mp-pct-nbt">\u7B49\u5F85\u5F00\u59CB</span></div><div class="mp-row"><span class="mp-label" style="color:#5879d4">\u544A\u793A\u724C</span><div class="mp-bar"><div id="mp-bar-sign" style="width:0%;background:#5879d4"></div></div><span class="mp-pct" id="mp-pct-sign">\u7B49\u5F85\u5F00\u59CB</span></div></div><div class="pg gone" id="clear-pg"><div id="clear-bar" style="background:var(--phx-error)"></div></div><div class="dim gone" id="clear-stat"></div><div class="dim" id="imp-stat"></div></div><div class="card gone" id="imp-log-card"><div class="log" id="imp-log"></div></div>';
  setTimeout(function() {
    document.getElementById("imp-cmds").checked = cfgImportCommands;
    toggleSub("imp-cmds", "imp-cmd-sub");
    document.getElementById("imp-exclude-fluids").checked = cfgExcludeFluids;
    toggleSub("imp-exclude-fluids", "imp-fluid-sub");
    document.getElementById("imp-exclude-water").checked = cfgExcludeWater;
    document.getElementById("imp-exclude-waterlogged").checked = cfgExcludeWaterlogged;
    document.getElementById("imp-exclude-lava").checked = cfgExcludeLava;
    document.getElementById("imp-spd").value = cfgSpeed;
    document.getElementById("imp-cmd-disabled").checked = cfgCmdDisabled;
    var preclearPicker = document.getElementById("imp-preclear-picker");
    if (preclearPicker) {
      preclearPicker.setAttribute("data-value", String(cfgPreClear));
      preclearPicker.querySelector(".picker-label").textContent = PRECLEAR_SHORT[String(cfgPreClear)] || "\u5173\u95ED";
    }
  }, 10);
  return html;
}
function RexpHTML() {
  return '<div class="card"><div style="font-size:16px;font-weight:800;margin-bottom:12px">\u5BFC\u51FA\u533A\u57DF</div><label>\u8D77\u70B9</label><div class="row"><input id="exp-x1" type="number" placeholder="X"><input id="exp-y1" type="number" placeholder="Y"><input id="exp-z1" type="number" placeholder="Z"></div><label>\u7EC8\u70B9</label><div class="row"><input id="exp-x2" type="number" placeholder="X"><input id="exp-y2" type="number" placeholder="Y"><input id="exp-z2" type="number" placeholder="Z"></div><label>\u683C\u5F0F</label><div class="picker-btn" id="exp-fmt-picker" data-value="mcstructure" onclick="showExpFmtPicker()"><span class="picker-label">MCStructure</span><span class="picker-arrow">\u25BE</span></div><label>\u534F\u4F5C\u673A\u5668\u4EBA\u6570\u91CF</label><input id="exp-workers" type="number" min="1" value="1"><div class="dim" style="font-size:12px;margin-bottom:10px">\u4F7F\u7528\u5DF2\u914D\u7F6E\u7684\u540C\u670D\u591A\u673A\u5668\u4EBA\uFF1B\u5355\u8D26\u53F7\u4FDD\u6301\u987A\u5E8F\u8BFB\u53D6\uFF0C\u7F3A\u5757\u65F6\u4E2D\u6B62\u3002</div><label>\u6587\u4EF6\u540D</label><input id="exp-name" value="" placeholder="\u6587\u4EF6\u540D\u79F0" oninput="onExpFmtChange()"><div class="dim" id="exp-name-preview" style="font-size:11px;margin:4px 0;color:var(--phx-text-secondary)"></div><div class="row"><button class="btn" onclick="doExpGo()" id="exp-go">\u5F00\u59CB\u5BFC\u51FA</button><button class="btn-d gone" id="exp-stop" onclick="doExpStop()">\u505C\u6B62\u5BFC\u51FA</button></div><div class="yy-transfer-meta yy-transfer-export"><span class="yy-transfer-tag">\u6E38\u56ED \xB7 \u5EFA\u7B51\u8BFB\u53D6</span><small>\u5F53\u524D\u6E38\u620F\u670D\u52A1\u5668 \u2192 \u5DE5\u5177\u7BB1\u8BBE\u5907\u6587\u4EF6</small></div><div class="yy-transfer-stat" id="exp-stat" aria-live="polite"></div><div class="pg gone yy-transfer-bar yy-transfer-export" id="exp-pg"><div id="exp-bar"></div></div></div><div class="card gone" id="exp-log-card"><div class="log" id="exp-log"></div></div>';
}
function RsearchHTML() {
  return `<div class="card"><div style="display:flex;gap:4px;margin-bottom:12px"><button class="btn-s sc-tab" data-mode="server" onclick="setScMode('server')" style="flex:1;margin:0;font-size:12px;padding:8px;background:var(--phx-primary-bg);color:var(--phx-primary);border-color:var(--phx-primary)">&#9776; \u79DF\u8D41\u670D</button><button class="btn-s sc-tab" data-mode="player" onclick="setScMode('player')" style="flex:1;margin:0;font-size:12px;padding:8px">&#128101; \u73A9\u5BB6</button></div><div id="sc-hint" style="font-size:11px;color:var(--phx-text-secondary);margin-bottom:8px">\u79DF\u8D41\u670D\u641C\u7D22\uFF1A\u6309\u540D\u79F0\u641C\u7D22\u7F51\u6613\u6211\u7684\u4E16\u754C\u79DF\u8D41\u670D</div><div class="row"><input id="sc-query" placeholder="\u641C\u7D22..." style="flex:1" onkeydown="if(event.key==='Enter')doSearch()"><button class="btn" onclick="doSearch()" style="width:auto;margin:0;padding:11px 16px;font-size:13px">\u641C\u7D22</button></div><div id="sc-results" style="margin-top:12px"></div></div>`;
}
function toggleSub(cbId, subId) {
  var sub = document.getElementById(subId);
  var cb = document.getElementById(cbId);
  if (sub) sub.classList.toggle("gone", !cb || !cb.checked);
}
var impMode = "import";
function setImpMode(mode) {
  var _a;
  impMode = mode;
  if (mode === "multi") yyImportWorkerOptions();
  else (_a = document.getElementById("yy-import-workers")) == null ? void 0 : _a.remove();
  var sx = document.getElementById("imp-x"), sy = document.getElementById("imp-y"), sz = document.getElementById("imp-z");
  var rx = document.getElementById("rep-x"), ry = document.getElementById("rep-y"), rz = document.getElementById("rep-z");
  function syncCoord(t, v) {
    if (t && t.value === "") t.value = v;
  }
  if (mode === "repair" && sx) {
    syncCoord(rx, sx.value);
    syncCoord(ry, sy.value);
    syncCoord(rz, sz.value);
  } else if (rx) {
    syncCoord(sx, rx.value);
    syncCoord(sy, ry.value);
    syncCoord(sz, rz.value);
  }
  var thumb = document.getElementById("imp-mode-thumb");
  var btns = document.querySelectorAll("#imp-mode-cap .mode-cap-btn");
  btns.forEach(function(b) {
    b.classList.toggle("active", b.getAttribute("data-mode") === mode);
  });
  if (thumb && btns.length > 0) {
    var idx = 0;
    btns.forEach(function(b, i) {
      if (b.getAttribute("data-mode") === mode) idx = i;
    });
    var pct = 100 / btns.length;
    thumb.style.width = "calc(" + pct + "% - 4px)";
    thumb.style.left = "calc(" + pct * idx + "% + 4px)";
  }
  var impParams = document.getElementById("imp-params");
  var repParams = document.getElementById("imp-repair-params");
  if (impParams) impParams.classList.toggle("gone", mode !== "import" && mode !== "multi");
  if (repParams) repParams.classList.toggle("gone", mode !== "repair");
  var btn = document.getElementById("imp-go");
  if (btn) {
    if (mode === "repair") {
      btn.textContent = "\u5F00\u59CB\u4FEE\u8865";
      btn.onclick = function() {
        doRepair();
      };
    } else {
      btn.textContent = "\u5F00\u59CB\u5BFC\u5165";
      btn.onclick = function() {
        doImp();
      };
    }
  }
}
function showRepairAreaPicker() {
  showPicker({
    title: "\u9009\u62E9\u4FEE\u8865\u533A\u57DF\u7C7B\u578B",
    items: [
      { value: "rect", label: "\u77E9\u5F62\u8303\u56F4 \u2014 \u8F93\u5165\u8D77\u70B9\u7EC8\u70B9" },
      { value: "circle", label: "\u5706\u5F62\u8303\u56F4 \u2014 \u533A\u5757\u5BF9\u9F50" }
    ],
    selected: document.getElementById("rep-area-picker").getAttribute("data-value"),
    onConfirm: function(v) {
      var p = document.getElementById("rep-area-picker");
      p.setAttribute("data-value", v);
      p.querySelector(".picker-label").textContent = v === "circle" ? "\u5706\u5F62\u8303\u56F4\uFF08\u533A\u5757\u5BF9\u9F50\uFF09" : "\u77E9\u5F62\u8303\u56F4";
      document.getElementById("rep-rect-params").classList.toggle("gone", v !== "rect");
      document.getElementById("rep-circle-params").classList.toggle("gone", v !== "circle");
    }
  });
}
function showRepRotPicker() {
  showPicker({
    title: "\u9009\u62E9\u65CB\u8F6C",
    items: ROT_ITEMS,
    selected: document.getElementById("rep-rot-picker").getAttribute("data-value"),
    onConfirm: function(v) {
      var p = document.getElementById("rep-rot-picker");
      p.setAttribute("data-value", v);
      p.querySelector(".picker-label").textContent = ROT_ITEMS.find(function(i) {
        return i.value === v;
      }).label;
    }
  });
}
function showRepDimPicker() {
  showPicker({
    title: "\u9009\u62E9\u7EF4\u5EA6",
    items: DIM_ITEMS,
    selected: document.getElementById("rep-dim-picker").getAttribute("data-value"),
    onConfirm: function(v) {
      var p = document.getElementById("rep-dim-picker");
      p.setAttribute("data-value", v);
      p.querySelector(".picker-label").textContent = DIM_ITEMS.find(function(i) {
        return i.value === v;
      }).label;
    }
  });
}
async function fillBotPos() {
  var r = await A("GET", "/api/fly/position");
  if (!r.ok) {
    T("\u83B7\u53D6\u4F4D\u7F6E\u5931\u8D25: " + (r.error || "\u672A\u77E5"), "e");
    return;
  }
  var cx = document.getElementById("rep-cx");
  var cz = document.getElementById("rep-cz");
  if (cx) cx.value = Math.round(r.x);
  if (cz) cz.value = Math.round(r.z);
  T("\u5DF2\u586B\u5165\u673A\u5668\u4EBA\u5F53\u524D\u4F4D\u7F6E (" + Math.round(r.x) + ", " + Math.round(r.z) + ")", "o");
}
async function doRepair() {
  if (checkUpdateLock()) return;
  var btn = document.getElementById("imp-go");
  if (!btn) return;
  btn.disabled = true;
  btn.innerHTML = '<span class="spin"></span> \u542F\u52A8\u4E2D...';
  document.getElementById("imp-stop").classList.remove("gone");
  document.getElementById("imp-pg").classList.remove("gone");
  document.getElementById("imp-log-card").classList.remove("gone");
  document.getElementById("imp-bar").style.width = "0%";
  var rx = parseInt(document.getElementById("rep-x").value) || 0;
  var ry = parseInt(document.getElementById("rep-y").value) || 0;
  var rz = parseInt(document.getElementById("rep-z").value) || 0;
  if (!rx && !ry && !rz) {
    T("\u8BF7\u586B\u5199\u8D77\u59CB\u5750\u6807", "e");
    btn.innerHTML = "\u5F00\u59CB\u4FEE\u8865";
    btn.disabled = false;
    return;
  }
  var spd = Math.min(200, Math.max(1, parseInt(document.getElementById("rep-spd").value) || 20));
  document.getElementById("rep-spd").value = spd;
  var areaType = document.getElementById("rep-area-picker").getAttribute("data-value");
  var body = {
    path: impPath,
    x: rx,
    y: ry,
    z: rz,
    speed: spd,
    rotation: parseInt(document.getElementById("rep-rot-picker").getAttribute("data-value")) || 0,
    dimension: document.getElementById("rep-dim-picker").getAttribute("data-value"),
    repair_circle: areaType === "circle",
    repair_nbt_only: document.getElementById("rep-nbt-only").checked,
    repair_cx: parseInt(document.getElementById("rep-cx").value) || 0,
    repair_cz: parseInt(document.getElementById("rep-cz").value) || 0,
    repair_radius: parseInt(document.getElementById("rep-radius").value) || 16,
    repair_x1: parseInt(document.getElementById("rep-x1").value) || 0,
    repair_y1: parseInt(document.getElementById("rep-y1").value) || 0,
    repair_z1: parseInt(document.getElementById("rep-z1").value) || 0,
    repair_x2: parseInt(document.getElementById("rep-x2").value) || 0,
    repair_y2: parseInt(document.getElementById("rep-y2").value) || 0,
    repair_z2: parseInt(document.getElementById("rep-z2").value) || 0,
    import_commands: document.getElementById("rep-cmds").checked,
    cmd_disabled: document.getElementById("rep-cmd-disabled").checked
  };
  var r = await A("POST", "/api/repair/start", body);
  if (r.ok) {
    btn.innerHTML = "\u5F00\u59CB\u4FEE\u8865";
    btn.disabled = false;
    T("\u4FEE\u8865\u4EFB\u52A1\u5DF2\u542F\u52A8", "o");
  } else {
    T("\u542F\u52A8\u5931\u8D25: " + r.error, "e");
    btn.innerHTML = "\u5F00\u59CB\u4FEE\u8865";
    btn.disabled = false;
    document.getElementById("imp-stop").classList.add("gone");
  }
}
function toggleAdvImp() {
  var adv = document.getElementById("imp-adv");
  var btn = document.getElementById("imp-adv-btn");
  if (!adv || !btn) return;
  var open = adv.classList.contains("gone");
  adv.classList.toggle("gone");
  btn.textContent = open ? "\u6536\u8D77\u9AD8\u7EA7\u9009\u9879" : "\u5C55\u5F00\u9AD8\u7EA7\u9009\u9879";
}
function toggleDimInput() {
  var el = document.getElementById("imp-dim-dm");
  var picker = document.getElementById("imp-dim-picker");
  if (el) el.classList.toggle("gone", !picker || picker.getAttribute("data-value") != "dm");
}
async function doPick() {
  var p = await fpicker("building");
  if (!p) return;
  impPath = p;
  var isMCWorld = p.toLowerCase().endsWith(".mcworld");
  if (isMCWorld) {
    var m = p.match(/\[(-?\d+)[,\s]+(-?\d+)[,\s]+(-?\d+)\]\s*~\s*\[(-?\d+)[,\s]+(-?\d+)[,\s]+(-?\d+)\]/);
    if (m) {
      document.getElementById("imp-x1").value = m[1];
      document.getElementById("imp-y1").value = m[2];
      document.getElementById("imp-z1").value = m[3];
      document.getElementById("imp-x2").value = m[4];
      document.getElementById("imp-y2").value = m[5];
      document.getElementById("imp-z2").value = m[6];
    }
  }
  await doAnalyze(p, isMCWorld);
}
async function doAnalyze(p, isMCWorld) {
  var _a;
  if (typeof setImpMode === "function") setImpMode("import");
  activeRenderers.forEach(function(kill) {
    try {
      kill();
    } catch (e) {
    }
  });
  activeRenderers = [];
  document.getElementById("imp-pick-txt").innerHTML = '<span class="spin"></span> \u5206\u6790\u4E2D...';
  var body = { path: p };
  if (isMCWorld) {
    body.x1 = I("imp-x1", 0);
    body.y1 = I("imp-y1", -64);
    body.z1 = I("imp-z1", 0);
    body.x2 = I("imp-x2", 256);
    body.y2 = I("imp-y2", 256);
    body.z2 = I("imp-z2", 256);
  }
  var r = await A("POST", "/api/building/analyze", body);
  if (!r.ok) {
    T("\u5206\u6790\u5931\u8D25: " + r.error, "e");
    document.getElementById("imp-pick-txt").innerHTML = "\u70B9\u51FB\u9009\u62E9\u5EFA\u7B51\u6587\u4EF6";
    return;
  }
  var s = r.stats;
  document.getElementById("imp-pick-txt").textContent = "\u5DF2\u9009\u62E9: " + p.split("/").pop();
  document.getElementById("imp-info").classList.remove("gone");
  document.getElementById("imp-mode-cap").classList.remove("gone");
  var nbtSub = [];
  if (s.cmd_blocks) nbtSub.push("\u547D\u4EE4" + s.cmd_blocks);
  if (s.container_items) nbtSub.push("\u5BB9\u5668" + s.container_items);
  var nbtSubHtml = nbtSub.length ? '<div style="font-size:9px;color:var(--phx-text-secondary);font-weight:400">' + nbtSub.join(" ") + "</div>" : "";
  document.getElementById("imp-info").innerHTML = '<div class="stats"><div class="s"><div class="n">' + s.solid_blocks + '</div><div class="l">\u5B9E\u4F53\u65B9\u5757</div></div><div class="s"><div class="n">' + s.nbt_blocks + '</div><div class="l">NBT' + nbtSubHtml + '</div></div><div class="s"><div class="n">' + s.water_blocks + '</div><div class="l">\u542B\u6C34</div></div><div class="s"><div class="n">' + s.size_x + "\xD7" + s.size_y + "\xD7" + s.size_z + '</div><div class="l">\u5C3A\u5BF8</div></div></div>' + (s.block_counts_list ? '<div style="font-size:11px;max-height:100px;overflow-y:auto;margin-top:6px">' + s.block_counts_list.slice(0, 15).map(function(b) {
    return '<div style="display:flex;justify-content:space-between;padding:3px 6px;border-bottom:1px solid var(--phx-border-light)"><span>' + b.name + '</span><span style="color:var(--phx-primary);font-weight:700">' + b.count + "</span></div>";
  }).join("") + "</div>" : "");
  if (isMCWorld) {
    document.getElementById("imp-mcworld-params").classList.remove("gone");
  } else {
    document.getElementById("imp-mcworld-params").classList.add("gone");
  }
  document.getElementById("imp-params").classList.remove("gone");
  document.getElementById("imp-go").classList.remove("gone");
  document.getElementById("imp-go").textContent = "\u5F00\u59CB\u5BFC\u5165";
  document.getElementById("imp-go").disabled = false;
  document.getElementById("imp-adv-btn").style.display = "";
  document.getElementById("imp-stop").classList.add("gone");
  document.getElementById("imp-pg").classList.add("gone");
  document.getElementById("imp-log-card").classList.add("gone");
  var impCmds = document.getElementById("imp-cmds");
  if (impCmds) impCmds.checked = cfgImportCommands;
  toggleSub("imp-cmds", "imp-cmd-sub");
  var exNBT = (_a = document.getElementById("imp-exclude-nbt")) == null ? void 0 : _a.checked;
  if (s.cmd_blocks && s.cmd_blocks > 0 && !exNBT) {
    document.getElementById("imp-cmd-opts").classList.remove("gone");
  } else {
    document.getElementById("imp-cmd-opts").classList.add("gone");
  }
  if (r.voxel_deferred) {
    var v = document.createElement("div");
    v.className = "viewer";
    v.id = "viewer";
    v.style.display = "flex";
    v.style.alignItems = "center";
    v.style.justifyContent = "center";
    v.style.flexDirection = "column";
    v.style.gap = "8px";
    v.innerHTML = '<div style="font-size:12px;color:var(--phx-text-secondary);text-align:center;padding:0 12px">\u65B9\u5757\u8F83\u591A\uFF08' + s.solid_blocks + '\uFF09\uFF0C3D \u9884\u89C8\u5DF2\u6682\u505C</div><button class="btn-s" onclick="doGenVoxel()" id="voxel-gen-btn" style="width:auto;padding:8px 16px;font-size:12px">\u4ECD\u8981\u6709\u9884\u89C8</button>';
    document.getElementById("imp-info").appendChild(v);
  } else if (r.voxel && !r.voxel.too_large) {
    var v = document.createElement("div");
    v.className = "viewer";
    v.id = "viewer";
    document.getElementById("imp-info").appendChild(v);
    setTimeout(function() {
      if (typeof THREE !== "undefined") show3D(v, r.voxel);
    }, 200);
  }
}
async function doGenVoxel() {
  if (!impPath) return;
  var v = document.getElementById("viewer");
  if (!v) return;
  v.innerHTML = '<div style="display:flex;align-items:center;gap:8px;font-size:12px;color:var(--phx-text-secondary)"><span class="spin"></span> \u6B63\u5728\u751F\u6210\u9884\u89C8...</div>';
  var r = await A("POST", "/api/building/voxel", { path: impPath });
  if (!r.ok || r.error) {
    v.innerHTML = '<div style="font-size:12px;color:var(--phx-error)">\u751F\u6210\u5931\u8D25: ' + (r.error || "\u672A\u77E5") + "</div>";
    return;
  }
  if (!r.voxel || r.voxel.too_large || !r.voxel.points || !r.voxel.points.length) {
    v.innerHTML = '<div style="font-size:12px;color:var(--phx-text-secondary)">\u8BE5\u5EFA\u7B51\u4F53\u79EF\u8FC7\u5927\uFF0C\u65E0\u6CD5\u9884\u89C8</div>';
    return;
  }
  v.innerHTML = "";
  if (typeof THREE !== "undefined") show3D(v, r.voxel);
}
async function doReanalyze() {
  if (!impPath) return;
  await doAnalyze(impPath, impPath.toLowerCase().endsWith(".mcworld"));
}
async function doImp() {
  var _a, _b, _c, _d, _e, _f, _g;
  if (checkUpdateLock()) return;
  var btn = document.getElementById("imp-go");
  if (!btn) return;
  var spd = Number(document.getElementById("imp-spd").value);
  if (!Number.isSafeInteger(spd) || spd < 1) {
    T("\u5BFC\u5165\u901F\u5EA6\u8BF7\u586B\u5199\u6B63\u6574\u6570", "e");
    return;
  }
  var isMulti = impMode === "multi";
  var selectedWorkers = [...document.querySelectorAll("#yy-import-workers input:checked")].map((x) => x.value);
  if (isMulti && !selectedWorkers.length) {
    T("\u8BF7\u5148\u9009\u62E9\u53C2\u4E0E\u4F5C\u4E1A\u7684\u673A\u5668\u4EBA", "e");
    return;
  }
  btn.disabled = true;
  btn.innerHTML = '<span class="spin"></span> \u542F\u52A8\u4E2D...';
  document.getElementById("imp-stop").classList.remove("gone");
  document.getElementById("imp-pg").classList.remove("gone");
  document.getElementById("imp-log-card").classList.remove("gone");
  document.getElementById("imp-bar").style.width = "0%";
  document.getElementById("imp-spd").value = spd;
  var cmdOpts = {};
  if (document.getElementById("imp-cmds").checked) {
    cmdOpts.cmd_disabled = document.getElementById("imp-cmd-disabled").checked;
  }
  var body = {
    type: isMulti ? "import_multi" : "import",
    params: {
      ...isMulti ? { bot_names: selectedWorkers } : {},
      path: impPath,
      x: parseInt(document.getElementById("imp-x").value) || 0,
      y: parseInt(document.getElementById("imp-y").value) || 0,
      z: parseInt(document.getElementById("imp-z").value) || 0,
      rotation: parseInt(document.getElementById("imp-rot-picker").getAttribute("data-value")) || 0,
      speed: spd,
      import_commands: document.getElementById("imp-cmds").checked,
      cmd_disabled: document.getElementById("imp-cmd-disabled").checked,
      exclude_fluids: document.getElementById("imp-exclude-fluids").checked,
      exclude_water: document.getElementById("imp-exclude-water").checked,
      exclude_waterlogged: document.getElementById("imp-exclude-waterlogged").checked,
      exclude_lava: document.getElementById("imp-exclude-lava").checked,
      deny_enable: ((_a = document.getElementById("imp-deny")) == null ? void 0 : _a.checked) || false,
      deny_y_offset: parseInt((_b = document.getElementById("imp-deny-offset")) == null ? void 0 : _b.value) || -1,
      border_enable: ((_c = document.getElementById("imp-border")) == null ? void 0 : _c.checked) || false,
      border_y: parseInt((_d = document.getElementById("imp-border-y")) == null ? void 0 : _d.value) || 0,
      border_block: "border_block",
      dimension: document.getElementById("imp-dim-picker").getAttribute("data-value"),
      dm_id: ((_e = document.getElementById("imp-dim-id")) == null ? void 0 : _e.value) || "3",
      region_mode: parseInt(document.getElementById("imp-region-picker").getAttribute("data-value")) || 1,
      keep_mode: ((_f = document.getElementById("imp-keep")) == null ? void 0 : _f.checked) || false,
      exclude_nbt: ((_g = document.getElementById("imp-exclude-nbt")) == null ? void 0 : _g.checked) || false,
      pre_clear_mode: parseInt(document.getElementById("imp-preclear-picker").getAttribute("data-value")) || 0
    }
  };
  if (impPath.toLowerCase().endsWith(".mcworld")) {
    body.params.x1 = I("imp-x1", 0);
    body.params.y1 = I("imp-y1", -64);
    body.params.z1 = I("imp-z1", 0);
    body.params.x2 = I("imp-x2", 256);
    body.params.y2 = I("imp-y2", 256);
    body.params.z2 = I("imp-z2", 256);
  }
  var r = await A("POST", "/api/task/start", body);
  if (r.ok) {
    btn.innerHTML = "\u5F00\u59CB\u5BFC\u5165";
    btn.disabled = false;
    T("\u4EFB\u52A1\u5DF2\u542F\u52A8", "o");
  } else {
    T("\u542F\u52A8\u5931\u8D25: " + r.error, "e");
    btn.innerHTML = "\u5F00\u59CB\u5BFC\u5165";
    btn.disabled = false;
    document.getElementById("imp-stop").classList.add("gone");
  }
}
function doImpStop() {
  A("POST", "/api/task/stop");
  T("\u5DF2\u505C\u6B62");
}
async function doSearch() {
  var inp = document.getElementById("sc-query");
  if (!inp) return;
  var kw = inp.value.trim();
  if (!kw) return;
  var btn = document.getElementById("sc-btn");
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<span class="spin"></span>';
  }
  var res = document.getElementById("sc-results");
  if (res) res.innerHTML = '<div style="text-align:center;padding:16px"><span class="spin" style="color:var(--phx-primary)"></span></div>';
  try {
    if (scMode === "server") {
      var r = await APrism("GET", "/api/server/find?keyword=" + encodeURIComponent(kw));
      if (btn) {
        btn.disabled = false;
        btn.textContent = "\u641C\u7D22";
      }
      if (!r.ok) {
        if (res) res.innerHTML = '<div style="text-align:center;padding:16px;color:var(--phx-error)">' + escHtml(r.error) + "</div>";
        return;
      }
      if (!r.servers || !r.servers.length) {
        if (res) res.innerHTML = '<div style="text-align:center;padding:16px;color:var(--phx-text-secondary);font-size:12px">\u672A\u627E\u5230\u5339\u914D\u7684\u79DF\u8D41\u670D</div>';
        return;
      }
      var h = '<div style="font-size:11px;color:var(--phx-text-secondary);font-weight:700;margin-bottom:6px">\u627E\u5230 ' + r.servers.length + " \u4E2A\u79DF\u8D41\u670D</div>";
      for (var i = 0; i < Math.min(r.servers.length, 20); i++) {
        var sv = r.servers[i];
        h += `<div class="qcard" style="cursor:pointer;margin-bottom:6px" onclick="showServerDetail('` + escHtml(sv.entity_id || "") + "','" + escHtml(sv.server_name || sv.name || "\u672A\u77E5") + `')">` + (sv.image_url ? '<img src="' + sv.image_url + `" style="width:36px;height:36px;border-radius:10px;flex-shrink:0" onerror="this.style.display='none'">` : '<div style="width:36px;height:36px;border-radius:10px;background:var(--phx-border-light);flex-shrink:0;display:flex;align-items:center;justify-content:center;font-size:14px">&#9906;</div>') + '<div class="qc-body"><div class="qc-title">' + escHtml(sv.server_name || sv.name || "\u672A\u77E5") + '</div><div class="qc-sub">\u5728\u7EBF ' + (sv.player_count != null ? sv.player_count : "?") + "/" + (sv.capacity != null ? sv.capacity : "?") + '</div></div><span class="qc-arrow">&#8250;</span></div>';
      }
      if (r.servers.length > 20) h += '<div class="dim" style="text-align:center;font-size:10px">\u4EC5\u663E\u793A\u524D 20 \u6761\u7ED3\u679C</div>';
      if (res) res.innerHTML = h;
    } else {
      var r = await APrism("GET", "/api/social/search?keyword=" + encodeURIComponent(kw));
      if (btn) {
        btn.disabled = false;
        btn.textContent = "\u641C\u7D22";
      }
      if (!r.ok) {
        if (res) res.innerHTML = '<div style="text-align:center;padding:16px;color:var(--phx-error)">' + escHtml(r.error) + "</div>";
        return;
      }
      if (!r.users || !r.users.length) {
        if (res) res.innerHTML = '<div style="text-align:center;padding:16px;color:var(--phx-text-secondary);font-size:12px">\u672A\u627E\u5230\u5339\u914D\u7684\u73A9\u5BB6</div>';
        return;
      }
      var h = '<div style="font-size:11px;color:var(--phx-text-secondary);font-weight:700;margin-bottom:6px">\u627E\u5230 ' + r.users.length + " \u4E2A\u73A9\u5BB6</div>";
      for (var i = 0; i < Math.min(r.users.length, 20); i++) {
        var u = r.users[i];
        h += `<div class="qcard" style="cursor:pointer;margin-bottom:6px" onclick="showPlayerDetail('` + escHtml(u.uid || u.user_id || "") + "','" + escHtml(u.nickname || u.name || "\u672A\u77E5") + `')">` + (u.headImage ? '<img src="' + u.headImage + `" style="width:36px;height:36px;border-radius:10px;flex-shrink:0" onerror="this.style.display='none'">` : '<div style="width:36px;height:36px;border-radius:10px;background:var(--phx-border-light);flex-shrink:0;display:flex;align-items:center;justify-content:center;font-size:14px">&#9787;</div>') + '<div class="qc-body"><div class="qc-title">' + escHtml(u.nickname || u.name || "\u672A\u77E5") + '</div><div class="qc-sub">Lv.' + (u.pe_growth && u.pe_growth.lv != null ? u.pe_growth.lv : "?") + '</div></div><span class="qc-arrow">&#8250;</span></div>';
      }
      if (res) res.innerHTML = h;
    }
  } catch (e) {
    if (btn) {
      btn.disabled = false;
      btn.textContent = "\u641C\u7D22";
    }
    if (res) res.innerHTML = '<div style="text-align:center;padding:16px;color:var(--phx-error)">\u8BF7\u6C42\u5931\u8D25: ' + e.message + "</div>";
  }
}
async function showServerDetail(sid, sname) {
  if (!sid) {
    T("\u7F3A\u5C11\u670D\u52A1\u5668ID", "e");
    return;
  }
  var r = await APrism("GET", "/api/server/detail?server_id=" + encodeURIComponent(sid));
  var sv = r.ok && r.server ? r.server : null;
  var plR = await APrism("GET", "/api/server/players?server_id=" + encodeURIComponent(sid) + "&length=10");
  var m = document.createElement("div");
  m.className = "modal-overlay";
  var h = '<div class="modal-box" style="max-width:380px;max-height:85vh;display:flex;flex-direction:column;padding:18px 20px"><div style="display:flex;align-items:center;gap:8px;margin-bottom:10px;flex-shrink:0">' + (sv && sv.image_url ? '<img src="' + sv.image_url + `" style="width:36px;height:36px;border-radius:10px;flex-shrink:0" onerror="this.style.display='none'">` : "") + '<div style="flex:1;min-width:0"><div style="font-size:15px;font-weight:800">' + escHtml(sname) + "</div>" + (sv ? '<div class="dim" style="font-size:10px">ID: ' + escHtml(sid) + "</div>" : "") + "</div></div>";
  if (sv) {
    h += '<div class="stats" style="grid-template-columns:1fr 1fr 1fr"><div class="s"><div class="n">' + (sv.player_count != null ? sv.player_count : "?") + "/" + (sv.capacity != null ? sv.capacity : "?") + '</div><div class="l">\u5728\u7EBF</div></div><div class="s"><div class="n">' + (sv.mc_version || "?") + '</div><div class="l">\u7248\u672C</div></div><div class="s"><div class="n">' + (sv.like_num || 0) + '</div><div class="l">\u70B9\u8D5E</div></div></div>';
  }
  if (plR.ok && plR.players && plR.players.length) {
    h += '<div style="margin-top:8px;padding-top:8px;border-top:1px solid var(--phx-border-light)"><div class="dim" style="font-size:10px;font-weight:700;margin-bottom:6px">\u5728\u7EBF\u73A9\u5BB6 (' + plR.players.length + ")</div>";
    for (var i = 0; i < Math.min(plR.players.length, 10); i++) {
      var p = plR.players[i];
      h += '<div style="display:flex;align-items:center;gap:8px;padding:5px 0;border-bottom:1px solid var(--phx-border-light);font-size:12px">' + (p.head_image ? '<img src="' + p.head_image + `" style="width:24px;height:24px;border-radius:6px;flex-shrink:0" onerror="this.style.display='none'">` : "") + '<span style="flex:1;font-weight:600">' + escHtml(p.name || "\u672A\u77E5") + '</span><span class="dim">Lv.' + (p.level || "?") + "</span></div>";
    }
    h += "</div>";
  }
  h += '<button class="btn-s" style="width:100%;margin-top:10px;flex-shrink:0" onclick="closeModalOverlay(this)">\u5173\u95ED</button></div>';
  m.innerHTML = h;
  document.body.appendChild(m);
  m.addEventListener("click", function(e) {
    if (e.target === m) closeOverlay(m);
  });
}
async function showPlayerDetail(uid, uname) {
  if (!uid) {
    T("\u7F3A\u5C11\u73A9\u5BB6UID", "e");
    return;
  }
  var r = await APrism("GET", "/api/server/owner?uid=" + encodeURIComponent(uid));
  var u = r.ok && r.user ? r.user : null;
  var m = document.createElement("div");
  m.className = "modal-overlay";
  var h = '<div class="modal-box" style="max-width:360px;padding:18px 20px"><div style="display:flex;align-items:center;gap:10px;margin-bottom:12px">' + (u && u.headImage ? '<img src="' + u.headImage + `" style="width:44px;height:44px;border-radius:12px;flex-shrink:0" onerror="this.style.display='none'">` : '<div style="width:44px;height:44px;border-radius:12px;background:var(--phx-border-light);flex-shrink:0"></div>') + '<div style="flex:1;min-width:0"><div style="font-size:15px;font-weight:800">' + escHtml(uname) + '</div><div class="dim" style="font-size:10px">UID: ' + escHtml(String(uid).substring(0, 15)) + "</div></div></div>";
  if (u) {
    h += '<div class="stats" style="grid-template-columns:1fr 1fr"><div class="s"><div class="n">' + (u.level || u.pe_growth && u.pe_growth.lv || "?") + '</div><div class="l">\u7B49\u7EA7</div></div><div class="s"><div class="n">' + (u.friend_count || "?") + '</div><div class="l">\u597D\u53CB</div></div></div>';
  } else {
    h += '<div style="text-align:center;padding:12px;color:var(--phx-text-secondary);font-size:12px">\u65E0\u6CD5\u83B7\u53D6\u8BE6\u7EC6\u4FE1\u606F</div>';
  }
  h += '<button class="btn-s" style="width:100%;margin-top:10px" onclick="closeModalOverlay(this)">\u5173\u95ED</button></div>';
  m.innerHTML = h;
  document.body.appendChild(m);
  m.addEventListener("click", function(e) {
    if (e.target === m) closeOverlay(m);
  });
}
var expRunning = false;
function doExp() {
  openSubPage("\u5EFA\u7B51\u5BFC\u51FA", RexpHTML());
  setTimeout(onExpFmtChange, 100);
}
function onExpFmtChange() {
  var prev = document.getElementById("exp-name-preview");
  if (prev) prev.textContent = expBuildPath();
}
function expBuildPath() {
  var fmt = document.getElementById("exp-fmt-picker").getAttribute("data-value");
  var name = (document.getElementById("exp-name").value || "").trim();
  if (!name) return "";
  var ext = fmt === "schematic" ? ".schematic" : fmt === "mcworld" ? ".mcworld" : ".mcstructure";
  var x1 = I("exp-x1", 0), y1 = I("exp-y1", 0), z1 = I("exp-z1", 0);
  var x2 = I("exp-x2", 64), y2 = I("exp-y2", 32), z2 = I("exp-z2", 64);
  var coordSuffix = "@[" + x1 + " " + y1 + " " + z1 + "]~[" + x2 + " " + y2 + " " + z2 + "]";
  return "/sdcard/Download/" + name + coordSuffix + ext;
}
async function doExpGo() {
  if (checkUpdateLock()) return;
  var b = document.getElementById("exp-go");
  L(b, true, "\u5BFC\u51FA\u4E2D...");
  var path = expBuildPath();
  if (!path) {
    L(b, false, "\u5F00\u59CB\u5BFC\u51FA");
    T("\u8BF7\u586B\u5199\u6587\u4EF6\u540D", "e");
    return;
  }
  var r = await A("POST", "/api/task/start", { type: "export", params: { workers: Math.max(1, I("exp-workers", 1)), format: document.getElementById("exp-fmt-picker").getAttribute("data-value"), output: path, x1: I("exp-x1", 0), y1: I("exp-y1", 0), z1: I("exp-z1", 0), x2: I("exp-x2", 64), y2: I("exp-y2", 32), z2: I("exp-z2", 64) } });
  L(b, false, "\u5F00\u59CB\u5BFC\u51FA");
  if (!r.ok) {
    T("\u5931\u8D25: " + r.error, "e");
    return;
  }
  expRunning = true;
  document.getElementById("exp-go").classList.add("gone");
  document.getElementById("exp-stop").classList.remove("gone");
  document.getElementById("exp-pg").classList.remove("gone");
  T("\u5BFC\u51FA\u5DF2\u542F\u52A8", "o");
}
async function doExpStop() {
  var b = document.getElementById("exp-stop");
  L(b, true, "\u505C\u6B62\u4E2D...");
  var r = await A("POST", "/api/task/stop");
  L(b, false, "\u505C\u6B62\u5BFC\u51FA");
  expRunning = false;
  document.getElementById("exp-stop").classList.add("gone");
  document.getElementById("exp-go").classList.remove("gone");
  T(r.ok ? "\u5DF2\u505C\u6B62" : "\u505C\u6B62\u5931\u8D25: " + (r.error || "\u672A\u77E5"), r.ok ? "o" : "e");
}
function resetTaskButtons() {
  ["imp-stop", "exp-stop", "md-stop", "skin-stop"].forEach(function(id) {
    var el = document.getElementById(id);
    if (el) el.classList.add("gone");
  });
  ["imp-go", "exp-go", "md-go", "skin-go"].forEach(function(id) {
    var el = document.getElementById(id);
    if (el) {
      el.classList.remove("gone");
      el.disabled = false;
      el.textContent = "\u91CD\u65B0\u5BFC\u5165";
    }
  });
  ["imp-pg", "clear-pg", "exp-pg", "md-pg"].forEach(function(id) {
    var el = document.getElementById(id);
    if (el) el.classList.add("gone");
  });
  expRunning = false;
}
async function yyImportWorkerOptions() {
  var _a, _b;
  (_a = document.getElementById("yy-import-workers")) == null ? void 0 : _a.remove();
  const host = document.getElementById("imp-params");
  if (!host) return;
  const box = document.createElement("section");
  box.id = "yy-import-workers";
  box.className = "card";
  box.innerHTML = '<h3>\u53C2\u4E0E\u6B64\u4F5C\u4E1A\u7684\u673A\u5668\u4EBA</h3><p class="yy-hint">\u53EA\u5217\u51FA\u5F53\u524D\u8D26\u53F7\u5185\u6838\u5DF2\u8FDE\u63A5\u540C\u670D\u7684\u673A\u5668\u4EBA\uFF1B\u53EF\u591A\u9009\u3002\u672A\u9009\u8D26\u53F7\u4E0D\u4F1A\u88AB\u6B64\u4F5C\u4E1A\u4F20\u9001\u3002\u5171\u4EAB\u8D26\u53F7\u987B\u5148\u6295\u653E\u4E3A\u5B50\u8FDE\u63A5\uFF1B\u4E0D\u91CD\u590D\u767B\u5F55\u5176\u4ED6\u72EC\u7ACB\u63A7\u5236\u53F0\u7684\u5728\u7EBF\u8D26\u53F7\u3002</p><div role="status">\u6B63\u5728\u8BFB\u53D6\u2026</div><div id="yy-worker-grid" style="display:grid;grid-template-columns:1fr;gap:10px"></div>';
  host.prepend(box);
  const r = await A("GET", "/api/fleet/status");
  if (!box.isConnected || impMode !== "multi") return;
  box.querySelector("[role=status]").remove();
  if (!r.ok) {
    box.append(document.createTextNode(r.error || "\u8BFB\u53D6\u5931\u8D25"));
    return;
  }
  for (const w of r.workers || []) {
    const label = document.createElement("label"), input = document.createElement("input");
    input.type = "checkbox";
    input.checked = true;
    input.value = w.name;
    label.className = "yy-worker-option";
    label.style.cssText = "display:flex;align-items:center;gap:12px;min-height:52px;padding:12px 16px;box-sizing:border-box;border:1px solid var(--phx-border-light);border-radius:10px;cursor:pointer;transition:border-color .15s,background .15s";
    input.style.width = "18px";
    input.style.height = "18px";
    input.style.flexShrink = "0";
    var info = document.createElement("span");
    info.style.cssText = "display:flex;flex-direction:column;gap:3px;flex:1;min-width:0";
    var wname = document.createElement("strong");
    wname.textContent = w.name;
    wname.style.fontSize = "14px";
    var meta = document.createElement("span");
    meta.textContent = (w.main ? "\u5F53\u524D\u4E3B\u8FDE\u63A5" : "\u5B50\u8FDE\u63A5") + " \u00B7 \u670D\u52A1\u5668 " + w.server;
    meta.style.cssText = "font-size:12px;color:var(--phx-text-secondary)";
    info.append(wname, meta);
    label.append(input, info);
    label.addEventListener("mouseenter", function() { label.style.borderColor = "var(--phx-accent,#888)"; });
    label.addEventListener("mouseleave", function() { label.style.borderColor = "var(--phx-border-light)"; });
    (document.getElementById("yy-worker-grid") || box).append(label);
  }
  if (!((_b = r.workers) == null ? void 0 : _b.length)) box.append(document.createTextNode("\u6CA1\u6709\u5C31\u7EEA\u7684\u673A\u5668\u4EBA\uFF0C\u8BF7\u5148\u5728\u8BBE\u7F6E\u4E2D\u8FDE\u63A5\u5B50\u673A\u5668\u4EBA\u3002"));
}
