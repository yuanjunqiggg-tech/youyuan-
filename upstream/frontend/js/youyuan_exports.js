var yyRemoteWithAccountState = yyRemotePage;
yyRemotePage = async function() {
  var _a, _b;
  await yyRemoteWithAccountState();
  var state = await yyRootAPI("GET", "/api/youyuan/remote");
  var urls = state.public_urls || {};
  var address = (_b = (_a = document.querySelector("#yy-local-key")) == null ? void 0 : _a.closest(".card")) == null ? void 0 : _b.querySelector(".yy-code");
  if (address) address.textContent = urls.mcp || "\u8BF7\u5148\u914D\u7F6E\u517C\u5BB9\u4E2D\u7EE7";
  var grid = document.querySelector(".yy-form-grid");
  if (!grid) return;
  var card = document.createElement("section");
  card.className = "card";
  var title = document.createElement("h3");
  title.textContent = "\u5F53\u524D\u516C\u7F51\u5730\u5740";
  card.appendChild(title);
  var hint = document.createElement("p");
  hint.className = "yy-hint";
  hint.textContent = state.relay_connected ? "\u5F53\u524D\u8BBE\u5907\u5DF2\u63A5\u5165\u4E2D\u7EE7\u3002" : "\u8FD9\u662F\u6839\u636E\u914D\u7F6E\u751F\u6210\u7684\u5019\u9009\u5730\u5740\uFF1B\u8FDE\u63A5\u6210\u529F\u540E\u624D\u80FD\u4F7F\u7528\u3002";
  card.appendChild(hint);
  [["console", "\u63A7\u5236\u53F0"], ["mcp", "MCP"], ["api", "API \u76EE\u5F55"]].forEach(function(item) {
    var label = document.createElement("label");
    label.textContent = item[1];
    var input = document.createElement("input");
    input.readOnly = true;
    input.value = urls[item[0]] || "";
    var copy = document.createElement("button");
    copy.className = "btn-s";
    copy.textContent = "\u590D\u5236";
    copy.onclick = function() {
      navigator.clipboard.writeText(input.value);
    };
    card.append(label, input, copy);
  });
  var note = document.createElement("p");
  note.className = "yy-hint";
  note.textContent = "\u81EA\u5EFA\u4E2D\u7EE7\u9700\u63D0\u4F9B /device/connect\u3001/mcp/ \u4E0E API \u8DEF\u7531\u3002\u4E5F\u53EF\u4F7F\u7528\u4E0B\u65B9 APK \u5185\u7F6E\u7684\u4E34\u65F6 MCP \u96A7\u9053\uFF0C\u5730\u5740\u7531\u7B2C\u4E09\u65B9\u96A7\u9053\u670D\u52A1\u5206\u914D\u3002";
  card.appendChild(note);
  grid.appendChild(card);
};
var yyAccountsWithExport = Raccounts;
Raccounts = async function() {
  await yyAccountsWithExport();
  var heading = document.querySelector(".yy-monitor .yy-section-head");
  if (!heading) return;
  var button = document.createElement("button");
  button.className = "btn-s";
  button.textContent = "\u5BFC\u51FA\u672C\u8D26\u53F7\u8BB0\u5F55";
  button.onclick = yyExportObservations;
  heading.appendChild(button);
};
async function yyExportObservations() {
  var path = yyMonitorID === "main" ? "/api/youyuan/observations" : yyAccountPath("/api/youyuan/observations", yyMonitorID);
  if (window.android && android.exportFile) {
    var result = await yyRootAPI("POST", path + "/export", {});
    if (!result.ok) {
      T(result.error || "\u5BFC\u51FA\u5931\u8D25", "e");
      return;
    }
    android.exportFile(result.src_path, yyMonitorID + "-observations.json");
    return;
  }
  var result = await yyRootAPI("GET", path);
  if (!result.ok) {
    T(result.error || "\u8BFB\u53D6\u8BB0\u5F55\u5931\u8D25", "e");
    return;
  }
  var blob = new Blob([JSON.stringify({ format: "youyuan.observations", version: 1, exported_at: (/* @__PURE__ */ new Date()).toISOString(), chat: result.chat || [], samples: result.samples || [] }, null, 2)], { type: "application/json" });
  var url = URL.createObjectURL(blob);
  var link = document.createElement("a");
  link.href = url;
  link.download = yyMonitorID + "-observations.json";
  link.click();
  setTimeout(function() {
    URL.revokeObjectURL(url);
  }, 6e4);
}
var yyPluginsWithRuntimeExport = Rplg;
Rplg = async function() {
  await yyPluginsWithRuntimeExport();
  var host = document.querySelector("#tab-tasks .content-inner");
  if (!host) return;
  if (host.querySelector(".yy-installed-runtime")) return;
  var result = await A("GET", "/api/youyuan/runtime/list");
  var installed = result.installed || [];
  if (!installed.length) return;
  var card = document.createElement("section");
  card.className = "card";
  var heading = document.createElement("h3");
  heading.textContent = "Python / C++ \u63D2\u4EF6\u76EE\u5F55\u5BFC\u51FA";
  card.appendChild(heading);
  installed.forEach(function(plugin) {
    var row = document.createElement("div");
    row.className = "yy-runtime-row";
    var name = document.createElement("span");
    name.textContent = (plugin.name || plugin.id) + " \xB7 " + plugin.language;
    var button = document.createElement("button");
    button.className = "btn-s";
    button.textContent = "\u5BFC\u51FA\u5B8C\u6574 ZIP";
    button.onclick = function() {
      yyExportRuntimePlugin(plugin.id);
    };
    row.append(name, button);
    card.appendChild(row);
  });
  host.appendChild(card);
};
async function yyExportRuntimePlugin(id) {
  var _a;
  var path = "/api/youyuan/runtime/export?id=" + encodeURIComponent(id);
  if (window.android && android.exportFile) {
    var result = await A("POST", path, {});
    if (!result.ok) {
      T(result.error || "\u5BFC\u51FA\u5931\u8D25", "e");
      return;
    }
    android.exportFile(result.src_path, id + ".zip");
    return;
  }
  var response = await fetch(path);
  if (!response.ok || !((_a = response.headers.get("Content-Type")) == null ? void 0 : _a.includes("application/zip"))) {
    T("\u5BFC\u51FA\u5931\u8D25\uFF1A" + await response.text(), "e");
    return;
  }
  var url = URL.createObjectURL(await response.blob());
  var link = document.createElement("a");
  link.href = url;
  link.download = id + ".zip";
  link.click();
  setTimeout(function() {
    URL.revokeObjectURL(url);
  }, 6e4);
}
