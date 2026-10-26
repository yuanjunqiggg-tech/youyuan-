function yyExportPath(path, account) {
  return account && account !== "main" ? yyAccountPath(path, account) : path;
}
// 能力型下载链接：URL 自带 token（导出自带 SHA256 前 16 位），任何设备、任何登录态直接打开即可下载
function yyExportShareURL(item, account) {
  var dl = yyExportPath(item.download_url, account) || "";
  if (!dl) return dl;
  if (!/[?&]token=/.test(dl)) {
    var token = String((item.sha256 || item.id || "") + "youyuanshare").slice(0, 16);
    dl += (dl.indexOf("?") >= 0 ? "&" : "?") + "token=" + encodeURIComponent(token);
  }
  return dl;
}
async function yyDownloadExport(item, account) {
  var _a, _b, _c, _d;
  const response = await yyRootFetch(yyExportShareURL(item, account), { method: ((_a = window.android) == null ? void 0 : _a.exportDownload) ? "HEAD" : "GET" });
  const attachment = (response.headers.get("Content-Disposition") || "").toLowerCase().startsWith("attachment");
  const verifiedZip = /\.zip$/i.test(item.name || "") && /^[a-f0-9]{64}$/i.test(item.sha256 || "") && /application\/(zip|octet-stream)/i.test(response.headers.get("Content-Type") || "");
  if (!response.ok || !attachment && !verifiedZip) {
    let reason = "HTTP " + response.status + "\uFF0C\u8FD4\u56DE\u7C7B\u578B\u4E0D\u7B26\u5408\u6587\u4EF6\u4E0B\u8F7D";
    if ((response.headers.get("Content-Type") || "").includes("json")) {
      try {
        const r = await response.json();
        reason = r.error || reason;
      } catch (_) {
      }
    }
    throw Error("\u4E0B\u8F7D\u5931\u8D25\uFF1A" + reason);
  }
  if ((_b = window.android) == null ? void 0 : _b.exportDownload) {
    const headers = {};
    const key = sessionStorage.getItem("yy-cloud-key");
    if (key) headers.Authorization = "Bearer " + key;
    const device = new URLSearchParams(location.search).get("device");
    if (device) headers["X-YouYuan-Device"] = device;
    await ((_c = response.body) == null ? void 0 : _c.cancel());
    android.exportDownload(response.url, JSON.stringify(headers), item.name || "youyuan-export");
    return;
  }
  const blob = await response.blob();
  if (item.snapshot !== false && item.bytes != null && blob.size !== Number(item.bytes)) throw Error("\u4E0B\u8F7D\u5927\u5C0F\u4E0D\u5339\u914D\uFF0C\u8BF7\u91CD\u8BD5");
  if (item.sha256 && ((_d = globalThis.crypto) == null ? void 0 : _d.subtle)) {
    const hash = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", await blob.arrayBuffer())), (b) => b.toString(16).padStart(2, "0")).join("");
    if (hash !== item.sha256.toLowerCase()) throw Error("\u4E0B\u8F7D\u6821\u9A8C\u5931\u8D25\uFF0C\u6587\u4EF6\u5185\u5BB9\u4E0D\u5B8C\u6574\uFF0C\u8BF7\u91CD\u65B0\u4E0B\u8F7D");
  }
  const href = URL.createObjectURL(blob), link = document.createElement("a");
  link.href = href;
  link.download = item.name || "youyuan-export";
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(href), 6e4);
}
async function yyExportPluginZip(id, runtime) {
  const account = window.yyActiveAccount || "main";
  const path = runtime ? "/api/youyuan/runtime/export?id=" + encodeURIComponent(id) : "/api/plugin/export";
  const result = await yyRootAPI("POST", yyExportPath(path, account), { id });
  if (!result.ok) throw Error(result.error || "\u5BFC\u51FA\u5931\u8D25");
  if (window.android && android.exportFile && !android.exportDownload) {
    android.exportFile(result.src_path || result.path, result.name || id + ".zip");
  } else {
    if (!result.export) throw Error("\u6267\u884C\u7AEF\u7248\u672C\u5C1A\u672A\u63D0\u4F9B\u4E0B\u8F7D\u5165\u53E3\uFF0C\u8BF7\u66F4\u65B0\u540E\u91CD\u8BD5");
    await yyDownloadExport(result.export, account);
  }
  T("\u5B8C\u6574\u63D2\u4EF6\u5907\u4EFD\u5DF2\u5BFC\u51FA\uFF0C\u53EF\u5728\u4E0B\u8F7D\u4E2D\u5FC3\u7684\u2018\u63D2\u4EF6\u5907\u4EFD\u2019\u9875\u518D\u6B21\u4E0B\u8F7D", "o");
}
plgExportZip = async function(id) {
  try {
    await yyExportPluginZip(id, false);
  } catch (e) {
    T(e.message, "e");
  }
};
yyExportRuntimePlugin = async function(id) {
  try {
    await yyExportPluginZip(id, true);
  } catch (e) {
    T(e.message, "e");
  }
};
const yyExportSAFPrevious = plgExportSAF;
plgExportSAF = async function(id) {
  if (window.android && android.exportFile && !android.exportDownload) return yyExportSAFPrevious(id);
  return plgExportZip(id);
};
function yyExportCenterLink(account) {
  const u = new URL(location.href);
  u.hash = "";
  u.searchParams.set("exports", "1");
  u.searchParams.set("export_account", account || "main");
  return u.href;
}
async function yyOpenExportCenter(account) {
  if (document.getElementById("yy-export-center")) return;
  account = account || window.yyActiveAccount || "main";
  const overlay = document.createElement("div");
  overlay.className = "modal-overlay";
  overlay.id = "yy-export-center";
  const box = document.createElement("section");
  box.className = "modal-box";
  box.style.cssText = "width:min(92vw,760px);max-width:760px;max-height:86vh;overflow:auto;text-align:left";
  const header = document.createElement("div");
  header.className = "yy-section-head";
  const title = document.createElement("h2");
  title.textContent = "\u63D2\u4EF6\u6587\u4EF6\u4E0B\u8F7D";
  const close = document.createElement("button");
  close.className = "btn-s";
  close.textContent = "\u5173\u95ED";
  close.onclick = () => overlay.remove();
  header.append(title, close);
  const note = document.createElement("p");
  note.className = "yy-hint";
  note.textContent = "\u5F53\u524D\u8D26\u53F7\uFF1A" + account + "\u3002\u5B8C\u6574 ZIP \u4FDD\u5B58\u5728\u6267\u884C\u8BBE\u5907\u4E0A\uFF0C\u5305\u542B\u63D2\u4EF6\u6E90\u7801\u3001\u914D\u7F6E\u53CA\u6570\u636E\uFF1B\u8FD9\u91CC\u53EA\u5220\u9664\u5907\u4EFD\uFF0C\u4E0D\u5220\u9664\u63D2\u4EF6\u3002";
  const tabs = document.createElement("div");
  tabs.className = "yy-actions";
  const outputs = document.createElement("button");
  outputs.className = "btn";
  outputs.textContent = "\u63D2\u4EF6\u751F\u6210\u6587\u4EF6";
  const backups = document.createElement("button");
  backups.className = "btn-s";
  backups.textContent = "\u5B8C\u6574\u63D2\u4EF6\u5907\u4EFD";
  const picker = document.createElement("select");
  picker.style.cssText = "width:100%;margin:14px 0;min-height:44px";
  picker.setAttribute("aria-label", "\u9009\u62E9\u63D2\u4EF6");
  tabs.append(outputs, backups);
  let mode = "outputs";
  const actions = document.createElement("div");
  actions.className = "yy-actions";
  const refresh = document.createElement("button");
  refresh.className = "btn-s";
  refresh.textContent = "\u5237\u65B0";
  const copy = document.createElement("button");
  copy.className = "btn-s";
  copy.textContent = "\u590D\u5236\u4E0B\u8F7D\u4E2D\u5FC3\u7F51\u5740";
  copy.onclick = async () => {
    try {
      await navigator.clipboard.writeText(yyExportCenterLink(account));
      T("\u5DF2\u590D\u5236\uFF0C\u6253\u5F00\u540E\u9700\u767B\u5F55\u81EA\u5DF1\u7684\u8D26\u53F7", "o");
    } catch (_) {
      T("\u590D\u5236\u5931\u8D25\uFF0C\u8BF7\u590D\u5236\u6D4F\u89C8\u5668\u5730\u5740", "e");
    }
  };
  actions.append(refresh, copy);
  const list = document.createElement("div");
  list.setAttribute("aria-live", "polite");
  box.append(header, note, tabs, picker, actions, list);
  overlay.append(box);
  document.body.append(overlay);
  overlay.onclick = (e) => {
    if (e.target === overlay) overlay.remove();
  };
  async function load() {
    refresh.disabled = true;
    picker.hidden = mode !== "outputs";
    note.textContent = mode === "outputs" ? "\u5F53\u524D\u8D26\u53F7\uFF1A" + account + "\u3002\u4E0B\u8F7D\u63D2\u4EF6\u5728\u6267\u884C\u8BBE\u5907\u4E0A\u751F\u6210\u7684\u5EFA\u7B51\u3001NBT\u3001\u65E5\u5FD7\u3001\u6570\u636E\u6216ZIP\uFF0C\u4FDD\u7559\u539F\u683C\u5F0F\u3002" : "\u5B8C\u6574\u63D2\u4EF6\u5907\u4EFD\u5305\u542B\u6E90\u7801\u3001\u914D\u7F6E\u53CA\u6570\u636E\uFF1B\u53EA\u5220\u9664\u5907\u4EFD\uFF0C\u4E0D\u5220\u9664\u63D2\u4EF6\u3002";
    list.textContent = "\u6B63\u5728\u8BFB\u53D6\u6587\u4EF6\u2026";
    try {
      if (mode === "outputs") {
        await yyLoadPluginOutputs(picker, list, account);
        return;
      }
      const result = await yyRootAPI("GET", yyExportPath("/api/youyuan/exports", account));
      if (!result.ok) throw Error(result.error || "\u8BF7\u5148\u767B\u5F55\u5E76\u8FDB\u5165\u8BE5\u8D26\u53F7\u5DE5\u4F5C\u533A");
      list.replaceChildren();
      if (!(result.items || []).length) {
        list.textContent = "\u6682\u65E0\u5BFC\u51FA\u3002\u5230\u63D2\u4EF6\u5217\u8868\u9009\u62E9\u201C\u5907\u4EFD\u63D2\u4EF6\u201D\u3002";
        return;
      }
      for (const item of result.items) {
        const row = document.createElement("article");
        row.className = "card";
        const name = document.createElement("strong");
        name.textContent = item.name;
        const details = document.createElement("p");
        details.className = "yy-hint";
        details.textContent = new Date(item.created_at).toLocaleString("zh-CN") + " \xB7 " + (item.bytes / 1048576).toFixed(2) + " MB \xB7 " + item.kind;
        const hash = document.createElement("code");
        hash.textContent = "SHA256 " + item.sha256;
        hash.style.cssText = "display:block;overflow-wrap:anywhere;font-size:11px;opacity:.7;margin-bottom:12px";
        const buttons = document.createElement("div");
        buttons.className = "yy-actions";
        const download = document.createElement("button");
        download.className = "btn";
        download.textContent = "\u4E0B\u8F7D\u5B8C\u6574 ZIP";
        download.onclick = async () => {
          download.disabled = true;
          try {
            await yyDownloadExport(item, account);
            T("\u5DF2\u4EA4\u7ED9\u6D4F\u89C8\u5668\u4E0B\u8F7D", "o");
          } catch (e) {
            T(e.message, "e");
          } finally {
            download.disabled = false;
          }
        };
        const copyLink = document.createElement("button");
        copyLink.className = "btn-s";
        copyLink.textContent = "复制下载链接";
        copyLink.onclick = async () => {
          copyLink.disabled = true;
          try {
            await navigator.clipboard.writeText(location.origin + yyExportShareURL(item, account));
            T("已复制完整下载地址，任意设备打开即可下载", "o");
          } catch (e) {
            T("复制失败，请手动复制地址栏链接", "e");
          } finally {
            copyLink.disabled = false;
          }
        };
        const remove = document.createElement("button");
        remove.className = "btn-s";
        remove.textContent = "\u5220\u9664\u6B64\u5907\u4EFD";
        remove.onclick = async () => {
          if (!confirm("\u53EA\u5220\u9664\u6B64\u5BFC\u51FA\u5907\u4EFD\uFF0C\u4FDD\u7559\u63D2\u4EF6\u3002\u7EE7\u7EED\u5417\uFF1F")) return;
          remove.disabled = true;
          const r = await yyRootAPI("POST", yyExportPath("/api/youyuan/exports/remove", account), { id: item.id });
          if (r.ok) row.remove();
          else {
            T(r.error || "\u5220\u9664\u5931\u8D25", "e");
            remove.disabled = false;
          }
        };
        buttons.append(download, copyLink, remove);
        row.append(name, details, hash, buttons);
        list.append(row);
      }
    } catch (e) {
      list.textContent = e.message;
    } finally {
      refresh.disabled = false;
    }
  }
  refresh.onclick = load;
  picker.onchange = load;
  outputs.onclick = () => {
    mode = "outputs";
    outputs.className = "btn";
    backups.className = "btn-s";
    load();
  };
  backups.onclick = () => {
    mode = "backups";
    backups.className = "btn";
    outputs.className = "btn-s";
    load();
  };
  await load();
}
const yyPluginsExportCenterPrevious = Rplg;
Rplg = async function() {
  await yyPluginsExportCenterPrevious();
  const host = document.querySelector("#tab-tasks .content-inner");
  if (!host || host.querySelector(".yy-export-center-button")) return;
  const row = document.createElement("div");
  row.className = "yy-actions yy-export-center-button";
  const button = document.createElement("button");
  button.className = "btn-s";
  button.textContent = "\u63D2\u4EF6\u6587\u4EF6\u4E0B\u8F7D";
  button.onclick = () => yyOpenExportCenter();
  row.append(button);
  host.prepend(row);
};
if (new URL(location.href).searchParams.get("exports") === "1") {
  window.addEventListener("load", function() {
    let attempts = 0;
    const timer = setInterval(() => {
      if (++attempts > 120) {
        clearInterval(timer);
        return;
      }
      if (document.getElementById("yy-identity-gate")) return;
      clearInterval(timer);
      yyOpenExportCenter(new URL(location.href).searchParams.get("export_account") || "main");
    }, 1e3);
  }, { once: true });
}
async function yyDownloadPluginFile(id, file, kind, account, runtimeAccount) {
  try {
    const q = new URLSearchParams({ id, file, kind: kind || "lua" });
    if (runtimeAccount) q.set("runtime_account", runtimeAccount);
    const r = await yyRootAPI("GET", yyExportPath("/api/youyuan/plugin-outputs/info?" + q, account));
    if (!r.ok) throw Error(r.error || "\u6587\u4EF6\u4E0D\u53EF\u8BFB\u53D6");
    await yyDownloadExport(r.file, account);
    T("\u6587\u4EF6\u5DF2\u4EA4\u7ED9\u6D4F\u89C8\u5668\u4E0B\u8F7D", "o");
  } catch (e) {
    T(e.message, "e");
  }
}
document.addEventListener("click", function(e) {
  var _a, _b;
  const b = (_b = (_a = e.target).closest) == null ? void 0 : _b.call(_a, "button[data-output-file]");
  if (!b) return;
  e.preventDefault();
  e.stopPropagation();
  yyDownloadPluginFile(decodeURIComponent(b.dataset.outputPlugin), decodeURIComponent(b.dataset.outputFile), "lua", decodeURIComponent(b.dataset.outputAccount));
}, true);
async function yyLoadPluginOutputs(picker, list, account) {
  var _a;
  if (!picker.options.length) {
    const result2 = await yyRootAPI("GET", yyExportPath("/api/plugin/list", account));
    if (!result2.ok) throw Error(result2.error || "\u63D2\u4EF6\u76EE\u5F55\u4E0D\u53EF\u8BFB\u53D6");
    for (const p of result2.plugins || []) {
      const o = document.createElement("option");
      o.value = JSON.stringify({ id: p.id, kind: "lua" });
      o.textContent = p.name || p.id;
      picker.append(o);
    }
    const runtime = await yyRootAPI("GET", yyExportPath("/api/youyuan/runtime/list", account));
    for (const p of runtime.installed || []) {
      const o = document.createElement("option");
      o.value = JSON.stringify({ id: p.id, kind: "runtime", runtime_account: "main" });
      o.textContent = (p.name || p.id) + " \xB7 " + p.language;
      picker.append(o);
    }
  }
  list.replaceChildren();
  if (!picker.value) {
    list.textContent = "\u8BE5\u8D26\u53F7\u5C1A\u672A\u5B89\u88C5\u63D2\u4EF6\u3002";
    return;
  }
  const q = new URLSearchParams(JSON.parse(picker.value));
  const result = await yyRootAPI("GET", yyExportPath("/api/youyuan/plugin-outputs?" + q, account));
  if (!result.ok) throw Error(result.error || "\u8BFB\u53D6\u751F\u6210\u6587\u4EF6\u5931\u8D25");
  if (!((_a = result.items) == null ? void 0 : _a.length)) {
    list.textContent = "\u6B64\u63D2\u4EF6\u5C1A\u672A\u751F\u6210\u6587\u4EF6\u3002\u8FD0\u884C\u63D2\u4EF6\u7684\u5BFC\u51FA\u6216\u6253\u5305\u529F\u80FD\u540E\uFF0C\u70B9\u51FB\u5237\u65B0\u3002";
    return;
  }
  for (const item of result.items) {
    const row = document.createElement("article");
    row.className = "card";
    const name = document.createElement("strong");
    name.textContent = item.path;
    name.style.overflowWrap = "anywhere";
    const info = document.createElement("p");
    info.className = "yy-hint";
    info.textContent = new Date(item.modified_at).toLocaleString("zh-CN") + " \xB7 " + (item.bytes / 1024).toFixed(1) + " KB";
    const button = document.createElement("button");
    button.className = "btn-s";
    button.textContent = "\u4E0B\u8F7D\u6587\u4EF6";
    button.onclick = async () => {
      button.disabled = true;
      try {
        await yyDownloadExport(item, account);
        T("\u6587\u4EF6\u5DF2\u4EA4\u7ED9\u6D4F\u89C8\u5668\u4E0B\u8F7D", "o");
      } catch (e) {
        T(e.message, "e");
      } finally {
        button.disabled = false;
      }
    };
    row.append(name, info, button);
    list.append(row);
  }
}
