(function() {
  var previous = yyQQPage;
  yyQQPage = async function() {
    var _a, _b;
    await previous();
    var r = await A("GET", "/api/qq/config"), c = ((_a = r.config) == null ? void 0 : _a.assistant) || {}, m = await A("GET", "/api/ai/models"), host = document.getElementById("yy-qq-result");
    var field = (id, label, value, type) => '<label for="' + id + '">' + label + '</label><input id="' + id + '" type="' + (type || "text") + '" value="' + yyEscape(value || "") + '">';
    var html = '<section class="yy-card" id="yy-qq-assistant-panel"><h3>\u7FA4\u670D\u4E92\u901A\u52A9\u624B</h3>' + yySwitch("yy-qq-helper-enabled", "\u542F\u7528\u6307\u4EE4\u9762\u677F \xB7 \u53EA\u7528 .\u901A \u8F6C\u53D1\u804A\u5929", c.enabled !== false) + '<p class="yy-hint">\u7FA4\u5185 @\u673A\u5668\u4EBA\u540E\u53D1\u9001 \u5E2E\u52A9\uFF0C\u6240\u6709\u4EBA\u53EF\u67E5\u770B\u6307\u4EE4\uFF1B\u666E\u901A\u6210\u5458\u4E0D\u80FD\u6267\u884C\u547D\u4EE4\u6216\u8BFB\u53D6\u5750\u6807\u3002\u5404\u8D26\u53F7\u7684\u7ED1\u5B9A\u3001\u6743\u9650\u3001AI\u4E0E\u8BB0\u5F55\u76F8\u4E92\u72EC\u7ACB\u3002</p><details><summary>\u9AD8\u7EA7\u6743\u9650\u6807\u8BC6</summary><label for="yy-qq-super">\u8D85\u7EA7\u7BA1\u7406\u5458 \xB7 \u6BCF\u884C\u4E00\u4E2A\u6210\u5458\u6807\u8BC6</label><textarea id="yy-qq-super">' + yyEscape((c.super_admins || []).join("\n")) + '</textarea><label for="yy-qq-admins">\u666E\u901A\u7BA1\u7406\u5458 \xB7 \u6BCF\u884C\u4E00\u4E2A\u6210\u5458\u6807\u8BC6</label><textarea id="yy-qq-admins">' + yyEscape((c.admins || []).join("\n")) + '</textarea><p class="yy-hint">QQ\u5B98\u65B9\u4F7F\u7528\u5185\u90E8 member_openid \u6388\u6743\uFF0C\u4E0D\u80FD\u76F4\u63A5\u586BQQ\u53F7\u3002\u8BF7\u4ECE\u4E0B\u65B9\u771F\u5B9E\u6210\u5458\u5217\u8868\u6388\u6743\uFF1B\u5185\u90E8\u6807\u8BC6\u4EC5\u5728\u8BE6\u60C5\u4E2D\u67E5\u770B\u3002\u4E5F\u53EF\u751F\u6210\u8BA4\u9886\u7801\u6307\u5B9A\u8D85\u7EA7\u7BA1\u7406\u5458\u3002</p></details><button class="btn-s" onclick="yyQQCreateClaim()">\u751F\u6210\u8D85\u7EA7\u7BA1\u7406\u5458\u8BA4\u9886\u7801</button><pre id="yy-qq-claim" style="white-space:pre-wrap;word-break:break-all"></pre>' + yySwitch("yy-qq-death", "\u540C\u6B65\u6B7B\u4EA1\u6D88\u606F", !!c.death) + yySwitch("yy-qq-join", "\u540C\u6B65\u8FDB\u5165\u670D\u52A1\u5668", !!c.join) + yySwitch("yy-qq-leave", "\u540C\u6B65\u9000\u51FA\u670D\u52A1\u5668", !!c.leave) + '<p class="yy-hint">\u5B98\u65B9QQ\u65E0\u4E3B\u52A8\u63A8\u9001\u989D\u5EA6\uFF1B\u901A\u77E5\u9700\u8981\u6709\u6548\u7684\u7FA4\u56DE\u590D\u7A97\u53E3\uFF0C\u5426\u5219\u8BB0\u5F55\u5931\u8D25\u539F\u56E0\u3002OneBot\u4F7F\u7528\u5176\u81EA\u8EAB\u80FD\u529B\u3002</p><label for="yy-qq-public-scores">\u516C\u5F00\u8BA1\u5206\u9879 \xB7 \u6BCF\u884C\u4E00\u4E2A\uFF0C\u7559\u7A7A\u4E0D\u516C\u5F00\u4EFB\u4F55\u5206\u6570</label><textarea id="yy-qq-public-scores">' + yyEscape((c.public_objectives || []).join("\n")) + "</textarea>" + yySwitch("yy-qq-companion", "\u81EA\u7136\u7FA4\u804A \xB7 30\u79D2\u65E0\u65B0\u6D88\u606F\u7ED3\u675F", !!c.companion) + yySwitch("yy-qq-companion-all", "\u6536\u5230\u666E\u901A\u7FA4\u6D88\u606F\u65F6\u4E5F\u53EF\u81EA\u7136\u56DE\u590D", !!c.companion_all_messages) + field("yy-qq-companion-owner", "\u670D\u4E3B\u6635\u79F0 \xB7 \u4ECE\u4E0B\u65B9\u6210\u5458\u5217\u8868\u9009\u62E9", ((_b = c.nicknames) == null ? void 0 : _b[c.companion_owner]) || "\u672A\u9009\u62E9") + '<label for="yy-qq-companion-words">\u89E6\u53D1\u5173\u952E\u8BCD \xB7 \u6BCF\u884C\u4E00\u4E2A</label><textarea id="yy-qq-companion-words">' + yyEscape((c.companion_keywords || ["\u75A3", "\u8150\u7AF9", "\u623F\u4E3B", "\u670D\u4E3B"]).join("\n")) + '</textarea><p class="yy-hint">\u521D\u6B21\u89E6\u53D1\u7B49\u5F855\u79D2\uFF0C\u6536\u5230\u670D\u4E3B\u56DE\u590D\u5373\u505C\u6B62\uFF1B\u5BF9\u8BDD\u53EA\u968F\u6536\u5230\u7684\u6D88\u606F\u7EE7\u7EED\uFF0C30\u79D2\u65E0\u65B0\u6D88\u606F\u7ED3\u675F\u3002\u5B98\u65B9QQ\u901A\u5E38\u53EA\u4E0B\u53D1 @\u673A\u5668\u4EBA \u6D88\u606F\uFF1A\u53EA\u6709 @\u75A3\u3001\u666E\u901A\u804A\u5929\u6216\u670D\u4E3B\u56DE\u590D\u672A\u4E0B\u53D1\u65F6\u65E0\u6CD5\u68C0\u6D4B\u3002OneBot\u53EF\u4F7F\u7528\u5B83\u5B9E\u9645\u6536\u5230\u7684\u666E\u901A\u7FA4\u6D88\u606F\u3002\u7FA4\u804A\u4EC5\u6709\u6210\u5458\u67E5\u8BE2\u6743\u9650\u3002\u6700\u8FD1\u7FA4\u6D88\u606F\u53EF\u4EE5\u5728\u4E0B\u65B9\u67E5\u770B\uFF0C\u6A21\u578B\u53EA\u83B7\u5F97\u672C\u7FA4\u8FD1\u671F12\u6761\u5B9E\u9645\u6536\u5230\u7684\u6D88\u606F\u3002</p><h3>\u7FA4\u804A AI \u4E0E\u5916\u90E8 MCP</h3><label for="yy-qq-ai-model">\u5DF2\u914D\u7F6E\u7684\u6A21\u578B</label><select id="yy-qq-ai-model"><option value="">\u672A\u542F\u7528</option>' + (m.models || []).map((x) => '<option value="' + yyEscape(x.name) + '">' + yyEscape(x.name) + " \xB7 " + yyEscape(x.model_id) + "</option>").join("") + '</select><label for="yy-qq-image-model">\u7ED8\u56FE\u6A21\u578B \xB7 \u652F\u6301 images/generations</label><select id="yy-qq-image-model"><option value="">\u672A\u914D\u7F6E\u7ED8\u56FE</option>' + (m.models || []).map((x) => '<option value="' + yyEscape(x.name) + '">' + yyEscape(x.name) + " \xB7 " + yyEscape(x.model_id) + "</option>").join("") + '</select><p class="yy-hint">\u7ED8\u56FE\u4F7F\u7528\u5355\u72EC\u7684\u56FE\u50CF\u63A5\u53E3\uFF0C\u9700\u8FD4\u56DE b64_json\u3002\u6587\u5B57\u804A\u5929\u6A21\u578B\u4E0D\u4F1A\u81EA\u52A8\u83B7\u5F97\u7ED8\u56FE\u80FD\u529B\uFF1B\u6210\u529F\u4E0A\u4F20QQ\u540E\u624D\u62A5\u544A\u5DF2\u53D1\u9001\u3002</p><details><summary>\u6DFB\u52A0\u6216\u66F4\u65B0\u7ED8\u56FE\u63A5\u53E3</summary>' + field("yy-qq-image-name", "\u7ED8\u56FE\u914D\u7F6E\u540D\u79F0", "\u7FA4\u804A\u7ED8\u56FE") + field("yy-qq-image-base", "\u7ED8\u56FEAPI\u5730\u5740\uFF08\u4F8B\u5982 https://\u4F60\u7684\u670D\u52A1/v1\uFF09", "") + field("yy-qq-image-id", "\u56FE\u50CF\u751F\u6210\u6A21\u578BID", "") + field("yy-qq-image-key", "\u7ED8\u56FEAPI\u5BC6\u94A5 \xB7 \u7559\u7A7A\u4FDD\u7559\u5DF2\u6709\u5BC6\u94A5", "", "password") + '<button class="btn-s" onclick="yyQQSaveImageModel()">\u4FDD\u5B58\u5E76\u9009\u7528\u7ED8\u56FE\u63A5\u53E3</button><p id="yy-qq-image-result" role="status"></p></details><label for="yy-qq-ai-prompt">\u81EA\u5B9A\u4E49\u8EAB\u4EFD\u4E0E\u4EFB\u52A1</label><textarea id="yy-qq-ai-prompt" placeholder="\u4F8B\u5982\uFF1A\u4F60\u662F\u672C\u670D\u52A1\u5668\u7684\u52A9\u624B\uFF0C\u8D1F\u8D23\u89E3\u7B54\u73A9\u6CD5\u4E0E\u7BA1\u7406\u95EE\u9898\u3002">' + yyEscape(c.prompt || "") + "</textarea>" + field("yy-qq-mcp-url", "\u7BA1\u7406\u5458\u4F7F\u7528\u7684\u5916\u90E8 MCP \xB7 Streamable HTTP \u5730\u5740", c.mcp_url) + field("yy-qq-mcp-token", "MCP\u8FDE\u63A5\u5BC6\u94A5 \xB7 \u7559\u7A7A\u4FDD\u7559", "", "password") + '<p class="yy-hint">' + (c.has_mcp_token ? "MCP\u5BC6\u94A5\u5DF2\u4FDD\u5B58\u3002" : "\u5C1A\u672A\u4FDD\u5B58MCP\u5BC6\u94A5\u3002") + "\u6210\u5458AI\u53EA\u83B7\u5F97\u5728\u7EBF\u540D\u5355\u3001\u516C\u5F00\u5206\u6570\u548C\u8BB0\u5206\u677F\u6570\u91CF\u5DE5\u5177\uFF1B\u7BA1\u7406\u5458\u6309\u9700\u8BFB\u53D6Skill\uFF0C\u5E76\u8C03\u7528\u6E38\u620F\u53CA\u8FD9\u4E2AMCP\u3002</p><details><summary>\u6DFB\u52A0\u6216\u66F4\u65B0\u7FA4\u804A\u6A21\u578B\u63A5\u53E3</summary>" + field("yy-qq-model-name", "\u914D\u7F6E\u540D\u79F0", "\u7FA4\u670DAI") + field("yy-qq-model-base", "API\u5730\u5740", "https://api.deepseek.com/v1") + field("yy-qq-model-id", "\u6A21\u578BID", "deepseek-chat") + field("yy-qq-model-key", "API\u5BC6\u94A5 \xB7 \u7559\u7A7A\u4FDD\u7559\u5DF2\u6709\u5BC6\u94A5", "", "password") + '<button class="btn-s" onclick="yyQQSaveModel()">\u4FDD\u5B58\u6A21\u578B\u63A5\u53E3</button><pre id="yy-qq-model-result"></pre></details><button class="btn-s" onclick="yyQQAssistantSave()">\u4FDD\u5B58\u52A9\u624B\u8BBE\u7F6E</button><details><summary>\u6307\u4EE4\u5E2E\u52A9</summary><pre id="yy-qq-helper-help" style="white-space:pre-wrap"></pre></details><section><h3>\u4ECE\u771F\u5B9E\u7FA4\u6210\u5458\u6388\u6743</h3><p class="yy-hint">\u5148\u8BA9\u76EE\u6807\u7528\u6237\u5728\u7FA4\u91CC @\u673A\u5668\u4EBA\u53D1\u9001 .\u8EAB\u4EFD\uFF0C\u518D\u70B9\u51FB\u5237\u65B0\u3002\u9009\u62E9\u5BF9\u5E94\u7684\u6210\u5458\u6807\u8BC6\u6388\u6743\uFF0CQQ\u53F7\u4E0D\u80FD\u8F6C\u6362\u4E3A\u5B98\u65B9\u6807\u8BC6\u3002</p><button class="btn-s" onclick="yyQQRefreshMembers()">\u5237\u65B0\u7FA4\u6210\u5458\u4E0E\u6743\u9650</button><div id="yy-qq-member-cards"></div></section><details><summary>\u7ED1\u5B9A\u7528\u6237\u4E0E\u6700\u8FD1\u6210\u5458\uFF08\u6635\u79F0\uFF09</summary><pre id="yy-qq-helper-members" style="white-space:pre-wrap;word-break:break-all"></pre></details></section>';
    host.insertAdjacentHTML("afterend", html);
    document.getElementById("yy-qq-ai-model").value = c.model || "";
    document.getElementById("yy-qq-image-model").value = c.image_model || "";
    document.getElementById("yy-qq-companion-owner").dataset.user = c.companion_owner || "";
    document.getElementById("yy-qq-companion-owner").readOnly = true;
    var info = await A("GET", "/api/qq/assistant");
    document.getElementById("yy-qq-helper-help").textContent = info.help || "";
    document.getElementById("yy-qq-helper-members").textContent = Object.entries(info.members || {}).map(([id, name]) => {
      var _a2;
      return name + (((_a2 = info.bindings) == null ? void 0 : _a2[id]) ? " \u2192 " + info.bindings[id] : " \xB7 \u672A\u7ED1\u5B9A\u6E38\u620F\u73A9\u5BB6");
    }).join("\n") || "\u5C1A\u672A\u6536\u5230\u6210\u5458\u4E8B\u4EF6";
    await yyQQRefreshMembers();
    var panel = document.getElementById("yy-qq-assistant-panel");
    panel.insertAdjacentHTML("beforeend", '<section class="yy-card"><h3>\u6700\u8FD1\u6536\u5230\u7684\u7FA4\u6D88\u606F</h3><select id="yy-qq-message-group"></select><button class="btn-s" onclick="yyQQRefreshMessages()">\u5237\u65B0\u6D88\u606F</button><div id="yy-qq-group-messages"></div></section>');
    var groups = await A("GET", "/api/qq/groups");
    var select = document.getElementById("yy-qq-message-group");
    (groups.groups || []).forEach((g) => select.add(new Option(g.name || g.number || "\u5DF2\u8FDE\u63A5\u7FA4", g.id)));
    select.onchange = yyQQRefreshMessages;
    await yyQQRefreshMessages();
  };
  window.yyQQRefreshMessages = async function() {
    var select = document.getElementById("yy-qq-message-group"), host = document.getElementById("yy-qq-group-messages");
    if (!host) return;
    var r = await A("GET", "/api/qq/messages?limit=30&group=" + encodeURIComponent((select == null ? void 0 : select.value) || ""));
    if (!r.ok) {
      host.textContent = r.error || "\u8BFB\u53D6\u5931\u8D25";
      return;
    }
    host.innerHTML = (r.messages || []).slice().reverse().map((m) => '<article style="padding:12px;border-bottom:1px solid var(--border);overflow-wrap:anywhere"><strong>' + yyEscape(m.name) + "</strong> <small>" + yyEscape(new Date(m.timestamp).toLocaleString()) + "</small><p>" + yyEscape(m.text) + "</p></article>").join("") || "<p>\u5C1A\u672A\u6536\u5230\u7FA4\u6D88\u606F\uFF1B\u6B64\u5904\u4E0D\u8BFB\u53D6QQ\u5BA2\u6237\u7AEF\u5B8C\u6574\u5386\u53F2\u3002</p>";
  };
  window.yyQQRefreshMembers = async function() {
    var _a;
    var info = await A("GET", "/api/qq/assistant"), config = await A("GET", "/api/qq/config"), c = ((_a = config.config) == null ? void 0 : _a.assistant) || {}, host = document.getElementById("yy-qq-member-cards");
    if (!host) return;
    var ids = Object.keys(info.members || {});
    host.innerHTML = ids.length ? ids.map((id) => {
      var _a2;
      var level = (c.super_admins || []).includes(id) ? "\u8D85\u7EA7\u7BA1\u7406\u5458" : (c.admins || []).includes(id) ? "\u7BA1\u7406\u5458" : "\u6210\u5458";
      return '<section class="yy-card"><strong>' + yyEscape(((_a2 = c.nicknames) == null ? void 0 : _a2[id]) || (info.members[id] === id ? "\u7FA4\u6210\u5458" : info.members[id])) + " \xB7 " + level + '</strong><details><summary>\u5185\u90E8\u8EAB\u4EFD\u8BE6\u60C5</summary><pre style="white-space:pre-wrap;word-break:break-all">' + yyEscape(id) + '</pre></details><button class="btn-s" data-owner="' + yyEscape(id) + '">\u8BBE\u4E3A\u670D\u4E3B</button><button class="btn-s" data-nickname="' + yyEscape(id) + '">\u4FEE\u6539\u663E\u793A\u6635\u79F0</button><button class="btn-s" data-user="' + yyEscape(id) + '" data-role="admin">\u8BBE\u4E3A\u7BA1\u7406\u5458</button><button class="btn-s" data-user="' + yyEscape(id) + '" data-role="super">\u8BBE\u4E3A\u8D85\u7EA7\u7BA1\u7406\u5458</button><button class="btn-s" data-user="' + yyEscape(id) + '" data-role="member">\u64A4\u9500\u6388\u6743</button></section>';
    }).join("") : '<p class="yy-hint">\u5C1A\u672A\u6536\u5230\u6210\u5458\u4E8B\u4EF6\u3002\u8BF7\u5728\u7FA4\u91CC @\u673A\u5668\u4EBA\u53D1\u9001 .\u8EAB\u4EFD\u3002</p>';
    host.querySelectorAll("button[data-owner]").forEach((b) => b.onclick = () => {
      var _a2;
      document.getElementById("yy-qq-companion-owner").dataset.user = b.dataset.owner;
      document.getElementById("yy-qq-companion-owner").value = ((_a2 = c.nicknames) == null ? void 0 : _a2[b.dataset.owner]) || info.members[b.dataset.owner] || "\u7FA4\u6210\u5458";
      T("\u670D\u4E3B\u5DF2\u9009\u62E9\uFF0C\u8BF7\u4FDD\u5B58\u52A9\u624B\u8BBE\u7F6E", "s");
    });
    host.querySelectorAll("button[data-nickname]").forEach((b) => b.onclick = async () => {
      var _a2;
      var n = prompt("\u663E\u793A\u6635\u79F0", ((_a2 = c.nicknames) == null ? void 0 : _a2[b.dataset.nickname]) || "");
      if (n === null) return;
      var aliases = { ...c.nicknames || {} };
      aliases[b.dataset.nickname] = n.trim().slice(0, 64);
      var r = await A("POST", "/api/qq/config", { assistant: { nicknames: aliases } });
      if (r.ok) await yyQQRefreshMembers();
      else T(r.error, "e");
    });
    host.querySelectorAll("button[data-user]").forEach((b) => b.onclick = async () => {
      b.disabled = true;
      try {
        var r = await A("POST", "/api/qq/assistant", { action: "authorize", user: b.dataset.user, role: b.dataset.role });
        if (!r.ok) throw Error(r.error);
        var conf = await A("GET", "/api/qq/config");
        document.getElementById("yy-qq-super").value = (conf.config.assistant.super_admins || []).join("\n");
        document.getElementById("yy-qq-admins").value = (conf.config.assistant.admins || []).join("\n");
        await yyQQRefreshMembers();
        T("\u6210\u5458\u6743\u9650\u5DF2\u4FDD\u5B58", "s");
      } catch (e) {
        T(e.message, "e");
        b.disabled = false;
      }
    });
  };
  window.yyQQAssistantDraft = function() {
    if (!document.getElementById("yy-qq-assistant-panel")) return void 0;
    var v = (id) => document.getElementById(id).value, lines = (id) => v(id).split("\n").map((x) => x.trim()).filter(Boolean), on = (id) => document.getElementById(id).checked;
    return { memory: typeof yyQQMemoryDraft === "function" ? yyQQMemoryDraft() : void 0, enabled: on("yy-qq-helper-enabled"), companion: on("yy-qq-companion"), companion_all_messages: on("yy-qq-companion-all"), companion_owner: document.getElementById("yy-qq-companion-owner").dataset.user || "", image_model: v("yy-qq-image-model"), companion_keywords: lines("yy-qq-companion-words"), super_admins: lines("yy-qq-super"), admins: lines("yy-qq-admins"), death: on("yy-qq-death"), join: on("yy-qq-join"), leave: on("yy-qq-leave"), public_objectives: lines("yy-qq-public-scores"), model: v("yy-qq-ai-model"), prompt: v("yy-qq-ai-prompt"), mcp_url: v("yy-qq-mcp-url").trim(), mcp_token: v("yy-qq-mcp-token").trim() };
  };
  window.yyQQAssistantSave = async function() {
    try {
      var r = await A("POST", "/api/qq/config", { assistant: yyQQAssistantDraft() });
      if (!r.ok) throw Error(r.error);
      document.getElementById("yy-qq-mcp-token").value = "";
      T("\u52A9\u624B\u8BBE\u7F6E\u5DF2\u4FDD\u5B58", "s");
      return true;
    } catch (e) {
      T(e.message, "e");
      return false;
    }
  };
  window.yyQQCreateClaim = async function() {
    var r = await A("POST", "/api/qq/assistant", { action: "claim" });
    document.getElementById("yy-qq-claim").textContent = r.ok ? r.instruction + "\n10\u5206\u949F\u6709\u6548\uFF0C\u4E00\u6B21\u4F7F\u7528\uFF1B\u8BF7\u7531\u4F60\u5E0C\u671B\u6388\u6743\u7684\u4EBA\u53D1\u9001\u3002" : r.error;
  };
  window.yyQQSaveImageModel = async function() {
    var v = (id) => document.getElementById(id).value.trim(), status = document.getElementById("yy-qq-image-result");
    try {
      var name = v("yy-qq-image-name"), base = v("yy-qq-image-base"), model = v("yy-qq-image-id");
      if (!name || !base || !model) throw Error("\u8BF7\u586B\u5199\u914D\u7F6E\u540D\u79F0\u3001API\u5730\u5740\u548C\u56FE\u50CF\u6A21\u578BID");
      var r = await A("POST", "/api/ai/models", { name, provider: "openai", base_url: base, model_id: model, api_key: v("yy-qq-image-key"), thinking: "off", streaming: false });
      if (!r.ok) throw Error(r.error);
      var select = document.getElementById("yy-qq-image-model");
      if (!Array.from(select.options).some((o) => o.value === name)) select.add(new Option(name + " \xB7 " + model, name));
      select.value = name;
      var saved = await A("POST", "/api/qq/config", { assistant: { image_model: name } });
      if (!saved.ok) throw Error(saved.error);
      document.getElementById("yy-qq-image-key").value = "";
      status.textContent = "\u7ED8\u56FE\u63A5\u53E3\u5DF2\u4FDD\u5B58\u5E76\u9009\u7528\u3002\u7FA4\u91CC\u53EF\u7528 .AI \u5E2E\u6211\u753B\u2026\uFF1B\u56FE\u50CF\u5B9E\u9645\u751F\u6210\u4E0E\u4E0A\u4F20\u6210\u529F\u540E\u624D\u786E\u8BA4\u53D1\u9001\u3002";
    } catch (e) {
      status.textContent = "\u4FDD\u5B58\u5931\u8D25\uFF1A" + e.message;
    }
  };
  window.yyQQSaveModel = async function() {
    var v = (id) => document.getElementById(id).value.trim(), result = document.getElementById("yy-qq-model-result");
    try {
      var name = v("yy-qq-model-name"), r = await A("POST", "/api/ai/models", { name, provider: "openai", base_url: v("yy-qq-model-base"), model_id: v("yy-qq-model-id"), api_key: v("yy-qq-model-key"), thinking: "off", streaming: false, max_output_tokens: 1800, context_window: 32e3 });
      if (!r.ok) throw Error(r.error);
      document.getElementById("yy-qq-model-key").value = "";
      var select = document.getElementById("yy-qq-ai-model");
      if (!Array.from(select.options).some((x) => x.value === name)) select.add(new Option(name, name));
      select.value = name;
      if (!await yyQQAssistantSave()) throw Error("\u6A21\u578B\u5DF2\u4FDD\u5B58\uFF0C\u4F46\u7FA4\u52A9\u624B\u9009\u62E9\u672A\u4FDD\u5B58\uFF0C\u8BF7\u91CD\u8BD5\u4FDD\u5B58\u52A9\u624B\u8BBE\u7F6E");
      result.textContent = "\u6A21\u578B\u63A5\u53E3\u5DF2\u4FDD\u5B58\uFF0C\u7FA4\u52A9\u624B\u5DF2\u9009\u7528\u6B64\u914D\u7F6E\u3002";
    } catch (e) {
      result.textContent = "\u4FDD\u5B58\u5931\u8D25\uFF1A" + e.message;
    }
  };
})();
