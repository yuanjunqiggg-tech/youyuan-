async function yyHostFilesPage() {
  delete subPageCache["\u6267\u884C\u8BBE\u5907\u6587\u4EF6"];
  openSubPage("\u6267\u884C\u8BBE\u5907\u6587\u4EF6", yyIntro("\u5DE5\u5177\u7BB1\u6587\u4EF6", "\u6587\u4EF6\u4E0A\u4F20\u5230\u5F53\u524D\u9009\u62E9\u7684\u6267\u884C\u8BBE\u5907\u3002\u9009\u62E9 Linux \u65F6\u4FDD\u5B58\u5728\u4E91\u670D\u52A1\u5668\u81EA\u5DF1\u7684\u78C1\u76D8\u3002") + '<section class="card"><p id="yy-host-file-location" class="yy-hint">\u6B63\u5728\u8BFB\u53D6\u6267\u884C\u8BBE\u5907\u2026</p><input type="file" id="yy-host-file-input" multiple><div class="yy-actions"><button class="btn" id="yy-host-file-submit" onclick="yyUploadHostFiles()">\u4E0A\u4F20\u6240\u9009\u6587\u4EF6</button><button class="btn-s" onclick="yyRefreshHostFiles()">\u5237\u65B0\u6587\u4EF6\u5217\u8868</button></div><p class="yy-hint">\u652F\u6301\u6E90\u7801\u3001TXT\u3001JSON\u3001\u65E5\u5FD7\u3001ZIP \u548C\u5176\u4ED6\u6587\u4EF6\u3002\u6309\u5757\u6821\u9A8C\u672C\u6B21\u6587\u4EF6\u5185\u5BB9\uFF1B\u7EED\u4F20\u65F6\u9010\u5757\u6838\u5BF9\u5DF2\u786E\u8BA4\u7684\u4E0A\u4F20\u524D\u7F00\uFF0C\u4E2D\u65AD\u540E\u53EF\u91CD\u65B0\u9009\u62E9\u540C\u4E00\u6587\u4EF6\u7EE7\u7EED\u3002\u5355\u6587\u4EF6\u6700\u591A2GiB\uFF0C\u672A\u5B8C\u6210\u8BB0\u5F55\u4FDD\u755924\u5C0F\u65F6\u3002\u4E0A\u4F20\u4E0D\u4F1A\u81EA\u52A8\u8FD0\u884C\u6216\u8986\u76D6\u5DF2\u6709\u6587\u4EF6\u3002</p><div id="yy-host-file-status" role="status"></div><div id="yy-host-file-list"></div></section>');
  await yyRefreshHostFiles();
}
async function yyRefreshHostFiles() {
  const scope = yyReadScope(), list = document.getElementById("yy-host-file-list"), location = document.getElementById("yy-host-file-location");
  if (!list || !location) return;
  const r = await A("GET", "/api/youyuan/files");
  if (scope !== yyReadScope() || document.getElementById("yy-host-file-list") !== list || document.getElementById("yy-host-file-location") !== location) return;
  if (!r.ok) {
    T(r.error, "e");
    return;
  }
  location.textContent = r.platform + " \xB7 " + r.directory;
  list.replaceChildren();
  for (const f of r.items) {
    const row = document.createElement("div");
    row.className = "yy-lib-row";
    const info = document.createElement("div");
    info.className = "yy-lib-info";
    const title = document.createElement("strong");
    title.textContent = f.name;
    const note = document.createElement("small");
    note.textContent = (f.bytes / 1024).toFixed(1) + " KB \xB7 " + f.modified;
    info.append(title, note);
    const button = document.createElement("button");
    button.className = "yy-secondary";
    button.textContent = "\u4E0B\u8F7D\u5230\u672C\u5730";
    button.onclick = async () => {
      if (scope !== yyReadScope()) {
        T("\u64CD\u4F5C\u8D26\u53F7\u5DF2\u5207\u6362\uFF0C\u8BF7\u5237\u65B0\u6587\u4EF6\u5217\u8868", "e");
        return;
      }
      button.disabled = true;
      try {
        const response = await fetch("/api/youyuan/files/download?name=" + encodeURIComponent(f.name));
        if (scope !== yyReadScope()) return;
        if (!response.ok) {
          T("\u4E0B\u8F7D\u5931\u8D25", "e");
          return;
        }
        const blob = await response.blob();
        if (scope !== yyReadScope()) return;
        const url = URL.createObjectURL(blob), link = document.createElement("a");
        link.href = url;
        link.download = f.name;
        link.click();
        setTimeout(() => URL.revokeObjectURL(url), 3e4);
      } catch (error) {
        if (scope === yyReadScope()) T(error.message, "e");
      } finally {
        button.disabled = false;
      }
    };
    row.append(info, button);
    list.append(row);
  }
  if (!r.items.length) list.textContent = "\u5C1A\u672A\u4E0A\u4F20\u6587\u4EF6\u3002";
}
var yyHostUploadBusy = false;
async function yyUploadHostFiles() {
  if (yyHostUploadBusy) {
    T("\u6240\u9009\u6587\u4EF6\u6B63\u5728\u4E0A\u4F20\uFF0C\u8BF7\u7B49\u5F85\u7ED3\u679C", "w");
    return;
  }
  const input = document.getElementById("yy-host-file-input"), status = document.getElementById("yy-host-file-status"), button = document.getElementById("yy-host-file-submit");
  if (!(input == null ? void 0 : input.files.length)) {
    T("\u8BF7\u9009\u62E9\u6587\u4EF6", "w");
    return;
  }
  const files = Array.from(input.files), scope = yyReadScope();
  const live = () => scope === yyReadScope() && document.getElementById("yy-host-file-status") === status;
  yyHostUploadBusy = true;
  input.disabled = true;
  if (button) button.disabled = true;
  try {
    for (const file of files) {
      if (!live()) throw Error("\u64CD\u4F5C\u8D26\u53F7\u6216\u9875\u9762\u5DF2\u5207\u6362\uFF0C\u5269\u4F59\u4E0A\u4F20\u5DF2\u6682\u505C");
      status.textContent = "\u6B63\u5728\u4E0A\u4F20 " + file.name;
      const r = await yyUploadFile(file, { onProgress: (p) => {
        if (live()) status.textContent = p.name + " \xB7 " + p.message + " \xB7 " + (p.offset / 1048576).toFixed(1) + " / " + (p.total / 1048576).toFixed(1) + " MiB";
      } });
      if (!r.ok) throw Error(r.error || "\u4E0A\u4F20\u672A\u786E\u8BA4");
      if (!live()) return;
      status.textContent = "\u5DF2\u4FDD\u5B58 " + r.name + " \xB7 " + r.bytes + " \u5B57\u8282";
    }
    if (live()) {
      input.value = "";
      await yyRefreshHostFiles();
    }
  } catch (error) {
    if (live()) status.textContent = "\u4E0A\u4F20\u5DF2\u505C\u6B62\uFF1A" + error.message;
  } finally {
    yyHostUploadBusy = false;
    input.disabled = false;
    if (button) button.disabled = false;
  }
}
