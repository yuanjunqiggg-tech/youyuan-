(function() {
  var base = yyQQPage;
  yyQQPage = async function() {
    var _a;
    await base();
    var cfg = await A("GET", "/api/qq/config"), c = cfg.config || {}, official = c.qqofficial || {};
    var secret = document.getElementById("yy-qq-secret");
    secret.placeholder = official.has_secret ? "\u5DF2\u4FDD\u5B58 \xB7 \u7559\u7A7A\u4FDD\u7559\u539F\u5BC6\u94A5" : "\u5C1A\u672A\u4FDD\u5B58 \xB7 \u586B\u5165 AppSecret";
    var hint = document.createElement("p");
    hint.id = "yy-qq-secret-state";
    hint.className = "yy-hint";
    hint.textContent = official.has_secret ? "AppSecret \u5DF2\u4FDD\u5B58\u5230\u5F53\u524D\u8D26\u53F7\uFF1B\u91CD\u65B0\u6253\u5F00\u65F6\u9690\u85CF\uFF0C\u4E0D\u9700\u8981\u91CD\u590D\u586B\u5199\u3002" : "\u5F53\u524D\u8D26\u53F7\u8FD8\u6CA1\u6709\u4FDD\u5B58 AppSecret\u3002";
    secret.after(hint);
    document.getElementById("yy-qq-official").insertAdjacentHTML("beforeend", '<label for="yy-qq-transport">\u5B98\u65B9\u6D88\u606F\u63A5\u6536\u65B9\u5F0F</label><select id="yy-qq-transport"><option value="webhook">\u516C\u7F51\u56DE\u8C03 \xB7 \u9700 QQ \u5E73\u53F0\u8BA2\u9605\u4E8B\u4EF6</option><option value="websocket">\u5B98\u65B9\u957F\u8FDE\u63A5 \xB7 APK \u4E3B\u52A8\u8FDE\u63A5\u817E\u8BAF</option></select><p class="yy-hint">\u957F\u8FDE\u63A5\u65E0\u9700\u586B\u5199\u56DE\u8C03\u7F51\u5740\uFF1B\u9700\u8981\u817E\u8BAF\u5141\u8BB8\u8BE5\u673A\u5668\u4EBA\u7684\u7FA4\u4E8B\u4EF6\u6743\u9650\u3002\u7FA4\u5185\u9700 @\u673A\u5668\u4EBA\u5EFA\u7ACB\u56DE\u590D\u7A97\u53E3\u3002</p>');
    document.getElementById("yy-qq-transport").value = official.transport || "webhook";
    document.getElementById("yy-qq-official").insertAdjacentHTML("beforeend", yySwitch("yy-qq-ack", "\u7FA4\u6D88\u606F\u8F6C\u53D1\u540E\u53D1\u9001\u786E\u8BA4\u56DE\u6267", official.acknowledge !== false));
    if (!document.getElementById("yy-qq-users")) document.getElementById("yy-qq-permission").insertAdjacentHTML("afterend", '<label for="yy-qq-users">\u6307\u5B9A\u6210\u5458 \xB7 \u6BCF\u884C\u4E00\u4E2A member_openid\uFF08OneBot \u7528 QQ \u53F7\uFF09</label><textarea id="yy-qq-users">' + yyEscape((((_a = c.command) == null ? void 0 : _a.users) || []).join("\n")) + "</textarea>");
    var group = document.getElementById("yy-qq-group");
    group.value = c.group_id || c.group_number || "";
    group.insertAdjacentHTML("afterend", '<p class="yy-hint">\u5B98\u65B9\u7FA4\u53F7\u4EC5\u4F5C\u5907\u6CE8\uFF1B\u771F\u6B63\u6536\u53D1\u7528 group_openid\uFF0C\u6536\u5230\u9996\u4E2A\u771F\u5B9E\u7FA4\u4E8B\u4EF6\u540E\u81EA\u52A8\u7ED1\u5B9A\u3002\u5DF2\u6709\u7ED1\u5B9A\u4F1A\u4FDD\u7559\u3002</p>');
    var label = document.querySelector('label[for="yy-qq-whitelist"]');
    if (label) label.textContent = "\u5141\u8BB8\u7684\u6307\u4EE4\u524D\u7F00 \xB7 \u6BCF\u884C\u4E00\u6761\uFF0C\u7559\u7A7A\u4E0D\u9650";
    var result = document.getElementById("yy-qq-result");
    result.insertAdjacentHTML("beforebegin", '<div class="yy-actions"><button class="btn-s" onclick="yyQQDiagnose()">\u68C0\u67E5\u4FDD\u5B58\u4E0E\u8FDE\u63A5</button><button class="btn-s" onclick="yyQQCallbackLabel()">\u66F4\u65B0\u672C\u8D26\u53F7\u56DE\u8C03\u5730\u5740</button></div>');
    await yyQQCallbackLabel();
    await yyQQDiagnose(false);
  };
  yyQQSave = async function() {
    var _a, _b, _c, _d;
    var el = (id) => document.getElementById(id), v = (id) => {
      var _a2;
      return ((_a2 = el(id)) == null ? void 0 : _a2.value) || "";
    }, lines = (id) => v(id).split("\n").map((s) => s.trim()).filter(Boolean);
    try {
      var r = await A("POST", "/api/qq/config", { enabled: el("yy-qq-enabled").checked, adapter: v("yy-qq-adapter"), group_id: v("yy-qq-group").trim(), onebot: { ws_url: v("yy-qq-ws").trim(), token: v("yy-qq-token").trim() }, qqofficial: { app_id: v("yy-qq-appid").trim(), app_secret: v("yy-qq-secret").trim(), transport: v("yy-qq-transport") || "webhook", acknowledge: el("yy-qq-ack").checked }, command: { permission: v("yy-qq-permission"), whitelist: lines("yy-qq-whitelist"), users: lines("yy-qq-users") }, assistant: (_a = window.yyQQAssistantDraft) == null ? void 0 : _a.call(window), game_to_group: { format: v("yy-qq-format"), only_prefixes: lines("yy-qq-prefix") } });
      if (!r.ok) throw Error(r.error);
      el("yy-qq-secret").value = "";
      el("yy-qq-token").value = "";
      if (el("yy-qq-mcp-token")) el("yy-qq-mcp-token").value = "";
      var q = ((_b = r.config) == null ? void 0 : _b.qqofficial) || {};
      el("yy-qq-secret").placeholder = q.has_secret ? "\u5DF2\u4FDD\u5B58 \xB7 \u7559\u7A7A\u4FDD\u7559" : "\u5C1A\u672A\u4FDD\u5B58";
      el("yy-qq-secret-state").textContent = q.has_secret ? "AppSecret \u5DF2\u4FDD\u5B58\u5230\u5F53\u524D\u8D26\u53F7\uFF1B\u8F93\u5165\u6846\u5DF2\u9690\u85CF\u5BC6\u94A5\uFF0C\u7559\u7A7A\u4E0D\u4F1A\u5220\u9664\u3002" : "\u5F53\u524D\u8D26\u53F7\u8FD8\u6CA1\u6709\u4FDD\u5B58 AppSecret\u3002";
      el("yy-qq-group").value = ((_c = r.config) == null ? void 0 : _c.group_id) || ((_d = r.config) == null ? void 0 : _d.group_number) || "";
      T("\u914D\u7F6E\u5DF2\u4FDD\u5B58", "s");
      await yyQQDiagnose();
    } catch (e) {
      el("yy-qq-result").textContent = "\u4FDD\u5B58\u5931\u8D25\uFF1A" + e.message;
      T(e.message, "e");
    }
  };
  window.yyQQDiagnose = async function(verify) {
    var result = document.getElementById("yy-qq-result");
    if (!result) return;
    try {
      var [config, status, diagnostic] = await Promise.all([A("GET", "/api/qq/config"), A("GET", "/api/qq/status"), A("GET", "/api/qq/diagnostics")]);
      var c = config.config || {}, d = diagnostic, q = c.qqofficial || {}, lines = ["\u914D\u7F6E\uFF1A" + (q.has_secret ? "AppSecret \u5DF2\u4FDD\u5B58" : "AppSecret \u5C1A\u672A\u4FDD\u5B58"), "\u63A5\u5165\uFF1A" + (q.transport === "websocket" ? "\u5B98\u65B9\u957F\u8FDE\u63A5" : "\u516C\u7F51\u56DE\u8C03"), "\u72B6\u6001\uFF1A" + (status.detail || status.state || "\u672A\u83B7\u53D6\u5230\u72B6\u6001\uFF0C\u8BF7\u70B9\u201C\u68C0\u67E5\u4FDD\u5B58\u4E0E\u8FDE\u63A5\u201D\u91CD\u8BD5"), "\u7FA4\u7ED1\u5B9A\uFF1A" + (d.group_bound ? "\u5DF2\u7ED1\u5B9A " + (c.group_number || "\u5DF2\u4FDD\u5B58\u5B98\u65B9\u7FA4\u6807\u8BC6") : "\u5C1A\u672A\u6536\u5230\u771F\u5B9E\u7FA4\u4E8B\u4EF6"), "\u53D1\u9001\u901A\u9053\uFF1A" + (status.state === "open" && !(d.reply_slots_remaining > 0) ? "\u957F\u8FDE\u63A5\u5728\u7EBF\uFF0C\u7B49\u5F85\u7FA4\u5185 @ \u6D88\u606F\u5F00\u542F\u56DE\u590D\u7A97\u53E3" : status.state === "open" ? "\u56DE\u590D\u7A97\u53E3\u53EF\u7528" : "\u5C1A\u672A\u8FDE\u63A5"), "\u5269\u4F59\u56DE\u590D\u989D\u5EA6\uFF1A" + (d.reply_slots_remaining || 0)];
      if (verify !== false && c.adapter === "qqofficial") {
        result.textContent = lines.join("\n") + "\n\u6B63\u5728\u9A8C\u8BC1\u817E\u8BAF\u51ED\u636E\u2026";
        var test = await A("POST", "/api/qq/verify", {});
        lines.push("\u817E\u8BAF\u8BA4\u8BC1\uFF1A" + (test.ok ? "\u901A\u8FC7" : test.error));
      }
      if (!d.group_bound) lines.push(q.transport === "websocket" ? "\u9274\u6743\u6210\u529F\u540E\uFF0C\u5728\u76EE\u6807\u7FA4 @\u673A\u5668\u4EBA\u53D1\u9001\u6D4B\u8BD5\u6D88\u606F\u3002" : "\u8BF7\u5728 QQ \u5E73\u53F0\u8BA2\u9605\u7FA4\u804A @ \u6D88\u606F\uFF0C\u56DE\u8C03\u4F7F\u7528\u4E0B\u65B9\u672C\u8D26\u53F7\u5730\u5740\uFF0C\u7136\u540E\u5728\u7FA4\u4E2D @\u673A\u5668\u4EBA\u3002");
      result.textContent = lines.join("\n");
    } catch (e) {
      result.textContent = "\u68C0\u67E5\u5931\u8D25\uFF1A" + e.message;
    }
  };
})();
