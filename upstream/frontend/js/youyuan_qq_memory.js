(function() {
  let view = null;
  function scope() {
    return typeof yyReadScope === "function" ? yyReadScope() : JSON.stringify([typeof yyActiveAccount === "undefined" ? "main" : yyActiveAccount, typeof yyAccountControllerScope === "function" ? yyAccountControllerScope() : ""]);
  }
  function live(v) {
    return v && view === v && scope() === v.scope && document.getElementById("yy-qq-memory-panel") === v.panel;
  }
  function el(id) {
    return document.getElementById("yy-qq-memory-" + id);
  }
  function enabledGroups(groups) {
    return (groups || []).filter((g) => g.enabled === true && typeof g.id === "string" && g.id !== "");
  }
  function selected(v) {
    var _a;
    const id = (_a = el("group")) == null ? void 0 : _a.value;
    return live(v) ? v.groups.find((g) => g.id === id) : null;
  }
  function feedback(v, text) {
    if (live(v)) el("result").textContent = text;
  }
  function setGroups(v, groups) {
    var _a;
    if (!live(v)) return;
    const select = el("group"), previous = select.value;
    v.groups = enabledGroups(groups);
    select.innerHTML = v.groups.map((g) => '<option value="' + yyEscape(g.id) + '">' + yyEscape(g.name || g.number || g.id) + "</option>").join("");
    select.value = v.groups.some((g) => g.id === previous) ? previous : ((_a = v.groups[0]) == null ? void 0 : _a.id) || "";
    select.disabled = !v.groups.length;
    el("clear").disabled = !v.groups.length;
  }
  function positive(value, fallback, min, max) {
    const n = Number(value);
    return Number.isInteger(n) && n >= min ? Math.min(max, n) : fallback;
  }
  window.yyQQMemoryDraft = function() {
    if (!live(view)) return void 0;
    const ttl = Number(el("ttl").value), budget = Number(el("budget").value);
    if (!Number.isInteger(ttl) || ttl < 1 || ttl > 720) throw Error("\u8BB0\u5FC6\u4FDD\u7559\u65F6\u957F\u9700\u4E3A1\uFF5E720\u5C0F\u65F6\u7684\u6574\u6570");
    if (!Number.isInteger(budget) || budget < 1e3 || budget > 16e3) throw Error("\u4E0A\u4E0B\u6587\u9884\u7B97\u9700\u4E3A1000\uFF5E16000\u5B57\u7B26\u7684\u6574\u6570");
    return { enabled: el("enabled").checked, embedding_model: el("model").value, ttl_hours: ttl, context_chars: budget };
  };
  function statusText(r) {
    if (!r.enabled) return "\u957F\u671F\u68C0\u7D22\u672A\u542F\u7528\uFF1B\u8FD1\u671F\u4E0A\u4E0B\u6587\u4ECD\u53D7\u4FDD\u7559\u65F6\u957F\u4E0E\u9884\u7B97\u9650\u5236\u3002";
    if (!r.embedding_configured) return "\u8BCD\u6CD5\u56DE\u9000\uFF1A\u672A\u914D\u7F6E\u53EF\u7528\u7684\u5D4C\u5165\u6A21\u578B\uFF1B\u5F53\u524D\u4E0D\u662F\u5411\u91CF\u68C0\u7D22\u3002";
    if (String(r.status || "").startsWith("lexical_fallback_")) return "\u8BCD\u6CD5\u56DE\u9000\uFF1A\u5D4C\u5165\u670D\u52A1\u6216\u7D22\u5F15\u6682\u4E0D\u53EF\u7528\u3002";
    if (r.status === "vector_with_lexical_fallback_for_unindexed") return "\u5411\u91CF\u68C0\u7D22\u53EF\u7528\uFF0C\u672A\u7D22\u5F15\u6761\u76EE\u4ECD\u91C7\u7528\u8BCD\u6CD5\u56DE\u9000\u3002";
    if (r.status === "vector_indexed" && Number(r.indexed_entries) > 0) return "\u5DF2\u6709\u771F\u5B9E\u5411\u91CF\u7D22\u5F15\uFF1B\u672A\u7D22\u5F15\u6761\u76EE\u4ECD\u53EF\u80FD\u8BCD\u6CD5\u56DE\u9000\u3002";
    return "\u5DF2\u914D\u7F6E\u5D4C\u5165\u6A21\u578B\uFF0C\u5C1A\u672A\u786E\u8BA4\u53EF\u7528\u5411\u91CF\u7D22\u5F15\u3002";
  }
  window.yyQQMemoryRefreshStatus = async function() {
    var _a, _b;
    const v = view, g = selected(v);
    if (!live(v)) return;
    const epoch = ++v.statusEpoch;
    if (!g) {
      el("status").textContent = "\u6CA1\u6709\u5F53\u524D\u6388\u6743\u4E14\u5DF2\u542F\u7528\u7684\u7FA4\uFF1B\u8BF7\u5148\u5728\u4E92\u901A\u7FA4\u8BBE\u7F6E\u4E2D\u542F\u7528\u7FA4\u3002";
      el("group-id").textContent = "";
      return;
    }
    el("group-id").textContent = "\u7FA4\u6807\u8BC6\uFF1A" + g.id;
    el("status").textContent = "\u6B63\u5728\u8BFB\u53D6\u6B64\u7FA4\u8BB0\u5FC6\u72B6\u6001\u2026";
    try {
      const r = await A("GET", "/api/qq/memory?group=" + encodeURIComponent(g.id));
      if (!live(v) || epoch !== v.statusEpoch || ((_a = selected(v)) == null ? void 0 : _a.id) !== g.id) return;
      if (!r.ok) throw Error(r.error || "\u8BFB\u53D6\u5931\u8D25");
      const count = (n) => Number.isInteger(n) && n >= 0 ? String(n) : "\u672A\u77E5";
      el("status").textContent = statusText(r) + "\n\u4FDD\u5B58\u6761\u76EE\uFF1A" + count(r.entries) + " \xB7 \u5DF2\u7D22\u5F15\u6761\u76EE\uFF1A" + count(r.indexed_entries) + "\n\u6709\u6548\u4FDD\u7559\uFF1A" + count(r.ttl_hours) + " \u5C0F\u65F6 \xB7 \u4E0A\u4E0B\u6587\u9884\u7B97\uFF1A" + count(r.context_chars) + " \u5B57\u7B26" + (r.persistence_error ? "\n\u6301\u4E45\u5316\u5F02\u5E38\uFF1A\u5F53\u524D\u6761\u76EE\u4E0D\u4FDD\u8BC1\u5DF2\u4FDD\u5B58\u5230\u78C1\u76D8\u3002" : "");
    } catch (e) {
      if (live(v) && epoch === v.statusEpoch && ((_b = selected(v)) == null ? void 0 : _b.id) === g.id) el("status").textContent = "\u8BFB\u53D6\u5931\u8D25\uFF1A" + e.message;
    }
  };
  window.yyQQMemoryRefreshGroups = async function() {
    const v = view;
    if (!live(v) || v.refreshing) return;
    v.refreshing = true;
    el("refresh").disabled = true;
    try {
      const r = await A("GET", "/api/qq/groups");
      if (!live(v)) return;
      if (!r.ok) throw Error(r.error || "\u7FA4\u5217\u8868\u8BFB\u53D6\u5931\u8D25");
      setGroups(v, r.groups);
      await yyQQMemoryRefreshStatus();
    } catch (e) {
      feedback(v, "\u5237\u65B0\u5931\u8D25\uFF1A" + e.message);
    } finally {
      if (live(v)) el("refresh").disabled = false;
      v.refreshing = false;
    }
  };
  window.yyQQMemorySave = async function() {
    const v = view;
    if (!live(v) || v.saving) return;
    v.saving = true;
    el("save").disabled = true;
    try {
      const memory = yyQQMemoryDraft(), r = await A("POST", "/api/qq/config", { assistant: { memory } });
      if (!live(v)) return;
      if (!r.ok) throw Error(r.error || "\u4FDD\u5B58\u5931\u8D25");
      feedback(v, "\u8BB0\u5FC6\u7B56\u7565\u5DF2\u4FDD\u5B58\uFF1B\u9002\u7528\u4E8E\u5F53\u524D\u8D26\u53F7\u6240\u6709\u5DF2\u542F\u7528\u7684\u4E92\u901A\u7FA4\u3002");
      await yyQQMemoryRefreshStatus();
    } catch (e) {
      feedback(v, "\u4FDD\u5B58\u5931\u8D25\uFF1A" + e.message);
    } finally {
      if (live(v)) el("save").disabled = false;
      v.saving = false;
    }
  };
  window.yyQQMemoryClear = function() {
    const v = view, g = selected(v);
    if (!live(v) || !g || v.clearing) return;
    const ticket = ++v.clearTicket, label = g.name || g.number || g.id;
    const perform = async () => {
      var _a, _b, _c;
      if (!live(v) || v.clearing || ticket !== v.clearTicket) return;
      if (((_a = selected(v)) == null ? void 0 : _a.id) !== g.id) {
        feedback(v, "\u6240\u9009\u7FA4\u5DF2\u5207\u6362\uFF0C\u8BF7\u91CD\u65B0\u786E\u8BA4\u6E05\u7A7A\u3002");
        return;
      }
      v.clearTicket++;
      v.clearing = true;
      el("clear").disabled = true;
      try {
        const groups = await A("GET", "/api/qq/groups");
        if (!live(v) || ((_b = selected(v)) == null ? void 0 : _b.id) !== g.id) return;
        if (!groups.ok) throw Error(groups.error || "\u7FA4\u6388\u6743\u72B6\u6001\u8BFB\u53D6\u5931\u8D25");
        if (!enabledGroups(groups.groups).some((row) => row.id === g.id)) {
          setGroups(v, groups.groups);
          throw Error("\u8BE5\u7FA4\u5DF2\u505C\u7528\u6216\u4E0D\u518D\u53EF\u7528\uFF0C\u8BF7\u91CD\u65B0\u9009\u62E9");
        }
        const r = await A("DELETE", "/api/qq/memory?group=" + encodeURIComponent(g.id));
        if (!live(v) || ((_c = selected(v)) == null ? void 0 : _c.id) !== g.id) return;
        if (!r.ok) throw Error(r.error || "\u6E05\u7A7A\u5931\u8D25");
        feedback(v, "\u5DF2\u6E05\u7A7A\u6240\u9009\u7FA4\u7684\u8FD1\u671F\u4E0A\u4E0B\u6587\u3001AI\u5386\u53F2\u548C\u6301\u4E45\u68C0\u7D22\u7D22\u5F15\uFF1B\u5176\u4ED6\u7FA4\u4FDD\u7559\u3002");
        await yyQQMemoryRefreshStatus();
      } catch (e) {
        feedback(v, "\u6E05\u7A7A\u672A\u786E\u8BA4\uFF1A" + e.message + "\u3002\u8BF7\u5237\u65B0\u72B6\u6001\u786E\u8BA4\uFF0C\u52FF\u76F2\u76EE\u91CD\u590D\u63D0\u4EA4\u3002");
      } finally {
        v.clearing = false;
        if (live(v)) el("clear").disabled = !selected(v);
      }
    };
    const message = "\u5C06\u6E05\u7A7A\u7FA4\u300C" + label + "\u300D\u7684\u8FD1\u671F\u4E0A\u4E0B\u6587\u3001AI\u5386\u53F2\u548C\u6301\u4E45\u68C0\u7D22\u7D22\u5F15\uFF0C\u5176\u4ED6\u7FA4\u4FDD\u7559\u3002\u6E05\u7A7A\u4E0D\u80FD\u64A4\u56DE\u5DF2\u7ECF\u53D1\u9001\u5230\u7B2C\u4E09\u65B9\u670D\u52A1\u7684\u6570\u636E\u3002";
    if (typeof showConfirm === "function") showConfirm({ title: "\u6E05\u7A7A\u6B64\u7FA4\u7684AI\u8BB0\u5FC6", message: yyEscape(message), confirmText: "\u6E05\u7A7A\u672C\u7FA4\u8BB0\u5FC6", onConfirm: perform });
    else if (typeof confirm === "function" && confirm(message)) return perform();
  };
  window.yyQQMemoryMount = async function(assistant, models, groups) {
    const host = document.getElementById("yy-qq-assistant-panel");
    if (!host || document.getElementById("yy-qq-memory-panel")) return;
    const c = (assistant == null ? void 0 : assistant.memory) || {}, available = (models || []).filter((m) => m.provider === "openai"), saved = c.embedding_model || "";
    let options = '<option value="">\u4E0D\u4F7F\u7528\u5D4C\u5165\u6A21\u578B \xB7 \u8BCD\u6CD5\u68C0\u7D22</option>' + available.map((m) => '<option value="' + yyEscape(m.name) + '">' + yyEscape(m.name) + " \xB7 " + yyEscape(m.model_id || "") + "</option>").join("");
    if (saved && !available.some((m) => m.name === saved)) options += '<option value="' + yyEscape(saved) + '">\u539F\u6A21\u578B\u4E0D\u53EF\u7528\uFF1A' + yyEscape(saved) + "</option>";
    host.insertAdjacentHTML("beforeend", '<details class="yy-card" id="yy-qq-memory-panel" style="min-width:0"><summary>\u7FA4AI\u8BB0\u5FC6\u4E0E\u68C0\u7D22 \xB7 \u53EF\u9009</summary><p class="yy-hint">\u53EA\u4FDD\u7559\u673A\u5668\u4EBA\u5B9E\u9645\u6536\u5230\u7684\u7FA4\u4E8B\u4EF6\uFF0C\u4E0D\u8BFB\u53D6QQ\u5B8C\u6574\u5386\u53F2\u3002\u8EAB\u4EFD\u3001\u5F15\u7528\u4E0E\u5404\u7FA4\u4E0A\u4E0B\u6587\u72EC\u7ACB\uFF1B\u957F\u671F\u68C0\u7D22\u9ED8\u8BA4\u5173\u95ED\u3002</p>' + yySwitch("yy-qq-memory-enabled", "\u542F\u7528\u957F\u671F\u68C0\u7D22\u8BB0\u5FC6\uFF08RAG\uFF09", c.enabled === true) + '<label for="yy-qq-memory-model">\u73B0\u6709\u5D4C\u5165\u6A21\u578B \xB7 \u9700\u8981\u652F\u6301 embeddings \u63A5\u53E3</label><select id="yy-qq-memory-model" style="width:100%;min-width:0;box-sizing:border-box">' + options + '</select><p class="yy-hint">\u590D\u7528AI\u6A21\u578B\u7BA1\u7406\u4E2D\u7684OpenAI\u517C\u5BB9\u914D\u7F6E\uFF0C\u4E0D\u590D\u5236\u5BC6\u94A5\u3002\u672A\u914D\u7F6E\u3001\u670D\u52A1\u5931\u8D25\u6216\u7D22\u5F15\u672A\u5C31\u7EEA\u65F6\u91C7\u7528\u8BCD\u6CD5\u56DE\u9000\uFF0C\u4E0D\u8868\u793A\u5DF2\u542F\u7528\u5411\u91CF\u68C0\u7D22\u3002\u9009\u62E9\u5D4C\u5165\u6A21\u578B\u540E\uFF0C\u76F8\u5173\u7FA4\u6587\u672C\u53EF\u80FD\u53D1\u9001\u5230\u8BE5\u670D\u52A1\u3002</p><div class="yy-form-grid" style="grid-template-columns:repeat(auto-fit,minmax(min(180px,100%),1fr))"><label>\u4FDD\u7559\u65F6\u957F / \u5C0F\u65F6<input id="yy-qq-memory-ttl" type="number" min="1" max="720" step="1" value="' + positive(c.ttl_hours, 24, 1, 720) + '" style="width:100%;min-width:0;box-sizing:border-box"></label><label>\u4E0A\u4E0B\u6587\u9884\u7B97 / \u5B57\u7B26<input id="yy-qq-memory-budget" type="number" min="1000" max="16000" step="1" value="' + positive(c.context_chars, 6e3, 1e3, 16e3) + '" style="width:100%;min-width:0;box-sizing:border-box"></label></div><button class="btn-s" id="yy-qq-memory-save" onclick="yyQQMemorySave()">\u4FDD\u5B58\u8BB0\u5FC6\u7B56\u7565</button><p class="yy-hint">\u7B56\u7565\u9002\u7528\u4E8E\u5F53\u524D\u8D26\u53F7\u6240\u6709\u5DF2\u542F\u7528\u7684\u4E92\u901A\u7FA4\uFF1B\u4E0B\u65B9\u67E5\u770B\u548C\u6E05\u7A7A\u4EC5\u4F5C\u7528\u6240\u9009\u7FA4\u3002</p><label for="yy-qq-memory-group">\u67E5\u770B\u8BB0\u5FC6\u7684\u7FA4</label><select id="yy-qq-memory-group" style="width:100%;min-width:0;box-sizing:border-box"></select><p class="yy-hint" id="yy-qq-memory-group-id" style="overflow-wrap:anywhere"></p><div class="yy-actions" style="flex-wrap:wrap"><button class="btn-s" id="yy-qq-memory-refresh" onclick="yyQQMemoryRefreshGroups()">\u5237\u65B0\u7FA4\u4E0E\u72B6\u6001</button><button class="btn-s" id="yy-qq-memory-clear" onclick="yyQQMemoryClear()">\u6E05\u7A7A\u6240\u9009\u7FA4\u8BB0\u5FC6</button></div><p class="yy-hint" id="yy-qq-memory-status" role="status" style="white-space:pre-line;overflow-wrap:anywhere"></p><p class="yy-hint" id="yy-qq-memory-result" role="status" style="overflow-wrap:anywhere"></p></details>');
    view = { panel: document.getElementById("yy-qq-memory-panel"), scope: scope(), groups: [], statusEpoch: 0, clearTicket: 0 };
    el("model").value = saved;
    setGroups(view, groups);
    el("group").onchange = yyQQMemoryRefreshStatus;
    await yyQQMemoryRefreshStatus();
  };
})();
