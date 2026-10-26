(function() {
  var generated = null, busy = false;
  var get = (id) => document.getElementById(id), val = (id) => get(id).value;
  function field(id, label, value, type) {
    return '<label for="' + id + '">' + label + '</label><input id="' + id + '" type="' + (type || "text") + '" value="' + value + '">';
  }
  window.yyConverterPage = function() {
    generated = null;
    delete subPageCache["\u7ED3\u6784\u4E0E\u6307\u4EE4\u8F6C\u6362"];
    openSubPage("\u7ED3\u6784\u4E0E\u6307\u4EE4\u8F6C\u6362", yyIntro("\u7ED3\u6784\u4E0E\u6307\u4EE4\u8F6C\u6362", "\u5206\u6790\u5EFA\u7B51\u6587\u4EF6\u3001\u8F6C\u6362\u683C\u5F0F\uFF0C\u628A\u7ED3\u679C\u9884\u89C8\u540E\u518D\u4EA4\u7ED9\u547D\u4EE4\u94FE\u3002") + `<div class="yy-filters" role="tablist"><button class="yy-filter active" id="cv-tab-structure" role="tab" aria-selected="true" onclick="yyConversionTab('structure')">\u7ED3\u6784\u8F6C\u6362</button><button class="yy-filter" id="cv-tab-commands" role="tab" aria-selected="false" onclick="yyConversionTab('commands')">\u6307\u4EE4\u8F6C\u6362</button></div><div class="yy-form-grid"><div class="yy-form-stack"><section class="card" id="cv-structure"><h3>\u7ED3\u6784\u6587\u4EF6</h3><label for="cv-file">\u9009\u62E9\u672C\u673A\u6587\u4EF6</label><input type="file" id="cv-file" accept=".mcstructure,.bdx,.schem,.schematic,.json"><p class="yy-hint">\u652F\u6301 MCStructure\u3001BDX\u3001Schem\u3001Schematic \u4E0E\u672C\u5DE5\u5177\u5BFC\u51FA\u7684 JSON \u65B9\u5757\u8868\u3002\u65E0\u9700\u8FDE\u63A5\u6E38\u620F\u5373\u53EF\u8F6C\u6362\u3002</p><label for="cv-target">\u8F93\u51FA\u683C\u5F0F</label><select id="cv-target"><option value="txt">TXT \u65B9\u5757\u653E\u7F6E\u6307\u4EE4</option><option value="command_chains">\u63D0\u53D6\u547D\u4EE4\u65B9\u5757 \xB7 \u6309\u671D\u5411\u62C6\u6210\u72EC\u7ACB\u94FE</option><option value="mcstructure">MCStructure \xB7 \u4FDD\u7559\u65B9\u5757\u5B9E\u4F53\u6570\u636E</option><option value="json">JSON \xB7 \u53EF\u7F16\u8F91\u65B9\u5757\u8868</option></select><label for="cv-rotation">\u7ED5 Y \u8F74\u65CB\u8F6C</label><select id="cv-rotation"><option value="0">\u4E0D\u65CB\u8F6C</option><option value="90">90\xB0</option><option value="180">180\xB0</option><option value="270">270\xB0</option></select>` + field("cv-origin", "TXT \u653E\u7F6E\u539F\u70B9 / \u76F8\u5BF9\u504F\u79FB \xB7 x,y,z", "0,80,0") + '<label for="cv-coordinates">TXT \u5750\u6807\u65B9\u5F0F</label><select id="cv-coordinates"><option value="absolute">\u4E16\u754C\u7EDD\u5BF9\u5750\u6807</option><option value="relative">~ \u76F8\u5BF9\u6267\u884C\u4F4D\u7F6E</option></select>' + yySwitch("cv-merge", "\u76F8\u90BB\u540C\u8272\u6A2A\u884C\u5408\u5E76\u4E3A fill", true) + yySwitch("cv-air", "\u751F\u6210\u7A7A\u6C14\u6307\u4EE4\uFF0C\u6E05\u9664\u7ED3\u6784\u5185\u7A7A\u4F4D", false) + '<div class="yy-actions"><button class="btn-s" onclick="yyConvertStructure(true)">\u5206\u6790\u7ED3\u6784</button><button class="btn" onclick="yyConvertStructure(false)">\u751F\u6210\u8F6C\u6362\u7ED3\u679C</button></div><p class="yy-hint">\u65CB\u8F6C\u4F1A\u540C\u6B65\u5185\u6838\u652F\u6301\u7684\u65B9\u5757\u671D\u5411\u5C5E\u6027\u3002TXT \u4E0D\u4FDD\u7559\u7BB1\u5B50\u7269\u54C1\u4E0E\u65B9\u5757\u5B9E\u4F53 NBT\uFF1BMCStructure \u4E0D\u8FD8\u539F\u6E38\u8D70\u5B9E\u4F53\u6216\u7B2C\u4E8C\u6C34\u5C42\u3002\u5355\u6587\u4EF632MiB\u3001\u533A\u57DF131072\u65B9\u5757\uFF0C\u5927\u5EFA\u7B51\u8BF7\u7528\u5EFA\u7B51\u5BFC\u5165\u3002\u7A7A\u6C14\u5F00\u542F\u4F1A\u8986\u76D6\u7A7A\u4F4D\uFF0C\u8BF7\u6838\u5BF9\u9009\u533A\u3002</p></section><section class="card" id="cv-commands-panel" hidden><h3>\u6307\u4EE4\u6587\u4EF6</h3><label for="cv-command-file">\u5BFC\u5165 TXT / mcfunction</label><input id="cv-command-file" type="file" accept=".txt,.mcfunction" onchange="yyReadCommandFile(this)"><label for="cv-commands">\u6E90\u6307\u4EE4 \xB7 \u6BCF\u884C\u4E00\u6761</label><textarea id="cv-commands" rows="10" placeholder="say \u5F00\u59CB\nsetblock ~ ~ ~ stone"></textarea><label for="cv-mode">\u8F6C\u6362\u65B9\u5F0F</label><select id="cv-mode" onchange="yyConvertMode()"><option value="direct">\u76F4\u63A5\u547D\u4EE4\u94FE \xB7 \u4FDD\u7559\u94FE\u8BED\u6CD5</option><option value="execute">\u6307\u5B9A\u6267\u884C\u8005 \xB7 execute as \u2026 at @s</option><option value="scoreboard">\u5B9E\u4F53\u8BA1\u5206\u677F\u6761\u4EF6\u5305\u88C5</option></select><div id="cv-execute" hidden>' + field("cv-selector", "Bedrock \u6267\u884C\u8005\u9009\u62E9\u5668", "@a") + '</div><div id="cv-scoreboard" hidden>' + field("cv-objective", "\u8BA1\u5206\u677F\u540D", "n") + field("cv-entity", "\u6267\u884C\u5B9E\u4F53\u540D", "C") + field("cv-start", "\u8D77\u59CB\u5206\u6570", "0", "number") + '<p class="yy-hint">\u8F93\u51FA\u5E26\u5206\u6570\u6761\u4EF6\u7684 execute\uFF1B\u4F60\u9700\u8981\u53E6\u884C\u521B\u5EFA\u8BA1\u5206\u677F\u3001\u5B9E\u4F53\u5E76\u63A8\u8FDB\u5206\u6570\u3002\u4E0D\u662F\u81EA\u52A8\u8FD0\u884C\u65F6\u949F\u3002</p></div><label for="cv-output">\u8F93\u51FA\u7C7B\u578B</label><select id="cv-output"><option value="txt">TXT \xB7 \u6761\u4EF6\u94FE\u4E0E\u5468\u671F\u6807\u8BB0</option><option value="mcfunction">mcfunction \xB7 \u7EAF\u6307\u4EE4\u6587\u4EF6</option></select><div class="yy-actions"><button class="btn" onclick="yyConvertCommands()">\u8F6C\u6362\u5E76\u68C0\u67E5\u8BED\u6CD5</button></div><p class="yy-hint">\u76F4\u63A5\u65B9\u5F0F\u652F\u6301\u4E09\u4F4D\u5C5E\u6027\u3001\u5206\u94FE\u4E0E\u5468\u671F\u8BED\u6CD5\u3002\u6267\u884C\u8005\u548C\u8BA1\u5206\u677F\u5305\u88C5\u53EA\u63A5\u53D7\u7EAF\u6307\u4EE4\u3002\u7EAF mcfunction \u65E0\u6CD5\u4FDD\u5B58\u6761\u4EF6\u94FE\u4E0E\u5468\u671F\u5C5E\u6027\u3002</p></section></div><div class="yy-form-stack"><section class="card"><h3>\u7ED3\u679C\u4E0E\u9884\u89C8</h3><pre id="cv-result" class="yy-readout" aria-live="polite">\u9009\u62E9\u7ED3\u6784\u5206\u6790\u6216\u751F\u6210\u7ED3\u679C\u3002\u751F\u6210\u8FC7\u7A0B\u4E0D\u4F1A\u4FEE\u6539\u6E38\u620F\u4E16\u754C\u3002</pre><label for="cv-preview">\u6587\u672C\u9884\u89C8</label><textarea id="cv-preview" rows="12" readonly placeholder="TXT / JSON \u7ED3\u679C\u4F1A\u5728\u8FD9\u91CC\u663E\u793A"></textarea><div class="yy-actions"><button class="btn-s" id="cv-copy" disabled onclick="yyCopyConversion()">\u590D\u5236\u5B8C\u6574\u6587\u672C</button><button class="btn" id="cv-download" disabled onclick="yyDownloadConversion()">\u4E0B\u8F7D\u5230\u672C\u673A</button></div></section><section class="card"><h3>\u5BFC\u5165\u547D\u4EE4\u94FE</h3>' + field("cv-name", "\u8F93\u51FA\u6587\u4EF6\u540D", "\u8F6C\u6362\u7ED3\u679C.txt") + '<label for="cv-channel">\u9ED8\u8BA4\u6267\u884C\u901A\u9053</label><select id="cv-channel"><option value="ai">\u9B54\u6CD5\u6307\u4EE4 \xB7 \u81EA\u52A8\u5316\u6765\u6E90</option><option value="player">\u73A9\u5BB6\u6765\u6E90</option><option value="console">\u63A7\u5236\u53F0 \xB7 \u81EA\u52A8\u5316\u6765\u6E90</option></select>' + field("cv-delay", "\u9010\u6761\u5EF6\u8FDF \xB7 \u6BEB\u79D2\uFF080\u4E3A\u8FDE\u7EED\u63D0\u4EA4\uFF09", "0", "number") + field("cv-count", "\u8FD0\u884C\u8F6E\u6570 \xB7 0 \u4E3A\u6301\u7EED", "1", "number") + '<div class="yy-actions"><button class="btn" id="cv-import" disabled onclick="yyImportConversion()">\u4FDD\u5B58\u4E3A\u672A\u542F\u52A8\u7684\u547D\u4EE4\u94FE</button><button class="btn-s" onclick="yyChainsPage()">\u7BA1\u7406\u547D\u4EE4\u94FE</button></div><p class="yy-hint">\u5DF2\u6709\u540C\u540D\u6587\u4EF6\u4F1A\u62D2\u7EDD\u8986\u76D6\uFF1B\u4FDD\u5B58\u540E\u5728\u547D\u4EE4\u94FE\u4E2D\u4FEE\u6539\u3001\u542F\u52A8\u6216\u505C\u6B62\u3002\u76F8\u5BF9\u5750\u6807\u968F\u6267\u884C\u4F4D\u7F6E\u53D8\u5316\uFF0C\u56FA\u5B9A\u5EFA\u7B51\u5EFA\u8BAE\u7528\u7EDD\u5BF9\u5750\u6807\u3002</p></section></div></div>');
  };
  window.yyConversionTab = function(tab) {
    ["structure", "commands"].forEach((t) => {
      get(t === "structure" ? "cv-structure" : "cv-commands-panel").hidden = t !== tab;
      get("cv-tab-" + t).classList.toggle("active", t === tab);
      get("cv-tab-" + t).setAttribute("aria-selected", String(t === tab));
    });
  };
  window.yyConvertMode = function() {
    get("cv-execute").hidden = val("cv-mode") !== "execute";
    get("cv-scoreboard").hidden = val("cv-mode") !== "scoreboard";
  };
  function output(r) {
    generated = r;
    get("cv-result").textContent = (r.size ? "\u5C3A\u5BF8 " + r.size.join(" \xD7 ") + " \xB7 " + r.blocks + " \u4E2A\u975E\u7A7A\u6C14\u65B9\u5757\n\u8C03\u8272\u677F " + r.palette + " \u79CD \xB7 \u65B9\u5757\u5B9E\u4F53 " + r.block_entities + " \u4E2A\n" : "") + (r.commands !== void 0 ? r.commands + " \u6761\u547D\u4EE4\u5DF2\u9884\u89E3\u6790\n" : "") + (r.note || "\u8F6C\u6362\u5B8C\u6210\uFF0C\u5C1A\u672A\u542F\u52A8");
    var text = r.content || "";
    get("cv-preview").value = text.slice(0, 1e5);
    if (text.length > 1e5) get("cv-result").textContent += "\n\u9884\u89C8\u663E\u793A\u524D10\u4E07\u5B57\u7B26\uFF0C\u590D\u5236\u4E0E\u5BFC\u51FA\u4ECD\u4E3A\u5B8C\u6574\u6587\u4EF6\u3002";
    get("cv-name").value = r.filename || "\u8F6C\u6362\u7ED3\u679C.txt";
    get("cv-copy").disabled = !text;
    get("cv-download").disabled = !r.filename;
    get("cv-import").disabled = !r.files && (!text || !["txt", "mcfunction"].includes(r.filename.split(".").pop()));
    if (r.files) get("cv-result").textContent += "\n\u62C6\u5206 " + r.files.length + " \u4E2A\u72EC\u7ACB\u6587\u4EF6\uFF1B\u9ED8\u8BA4\u505C\u7528\u3002\n" + (r.warnings || []).join("\n");
  }
  async function work(fn) {
    if (busy) return;
    busy = true;
    try {
      get("cv-result").textContent = "\u6B63\u5728\u8F6C\u6362\u2026";
      await fn();
    } catch (e) {
      get("cv-result").textContent = "\u8F6C\u6362\u5931\u8D25\uFF1A" + e.message;
      T(e.message, "e");
    } finally {
      busy = false;
    }
  }
  window.yyReadCommandFile = async function(input) {
    try {
      var f = input.files[0];
      if (!f) return;
      if (f.size > 2 * 1024 * 1024) throw Error("\u6307\u4EE4\u6587\u4EF6\u8D85\u8FC72MiB");
      get("cv-commands").value = await f.text();
    } catch (e) {
      T(e.message, "e");
    }
  };
  window.yyConvertStructure = function(inspect) {
    return work(async () => {
      var f = get("cv-file").files[0];
      if (!f) throw Error("\u8BF7\u9009\u62E9\u7ED3\u6784\u6587\u4EF6");
      if (f.size > 32 * 1024 * 1024) throw Error("\u6587\u4EF6\u8D85\u8FC732MiB");
      var origin = val("cv-origin").split(/[ ,，]+/).map(Number);
      if (origin.length !== 3 || origin.some((n) => !Number.isSafeInteger(n) || Math.abs(n) > 3e7)) throw Error("\u539F\u70B9\u9700\u8981\u4E09\u4E2A\u6709\u6548\u6574\u6570");
      var data = await new Promise((resolve, reject) => {
        var reader = new FileReader();
        reader.onload = () => resolve(reader.result.split(",")[1]);
        reader.onerror = () => reject(Error("\u8BFB\u53D6\u6587\u4EF6\u5931\u8D25"));
        reader.readAsDataURL(f);
      });
      var r = await A("POST", "/api/youyuan/convert/structure", { filename: f.name, data, target: inspect ? "inspect" : val("cv-target"), origin, rotation: Number(val("cv-rotation")), relative: val("cv-coordinates") === "relative", merge: get("cv-merge").checked, includeAir: get("cv-air").checked });
      if (!r.ok) throw Error(r.error);
      if (inspect) {
        get("cv-result").textContent = "\u5C3A\u5BF8 " + r.size.join(" \xD7 ") + "\n\u975E\u7A7A\u6C14\u65B9\u5757 " + r.blocks + " \xB7 \u8C03\u8272\u677F " + r.palette + " \xB7 \u65B9\u5757\u5B9E\u4F53 " + r.block_entities + "\n" + r.note;
      } else output(r);
    });
  };
  window.yyConvertCommands = function() {
    return work(async () => {
      var r = await A("POST", "/api/youyuan/convert/commands", { content: val("cv-commands"), mode: val("cv-mode"), objective: val("cv-objective"), entity: val("cv-entity"), selector: val("cv-selector"), start: Number(val("cv-start")), output: val("cv-output") });
      if (!r.ok) throw Error(r.error);
      r.filename = "\u6267\u884C\u547D\u4EE4." + val("cv-output");
      output(r);
    });
  };
  window.yyCopyConversion = async function() {
    if (!(generated == null ? void 0 : generated.content)) return;
    try {
      if (window.android && typeof android.copyToClipboard === "function") android.copyToClipboard(generated.content);
      else await navigator.clipboard.writeText(generated.content);
      T("\u5B8C\u6574\u6587\u672C\u5DF2\u590D\u5236", "s");
    } catch (e) {
      T("\u590D\u5236\u5931\u8D25\uFF1A" + e.message, "e");
    }
  };
  window.yyDownloadConversion = async function() {
    if (!generated) return;
    try {
      var blob = generated.data ? new Blob([Uint8Array.from(atob(generated.data), (c) => c.charCodeAt(0))]) : new Blob([generated.content], { type: "text/plain;charset=utf-8" });
      await yyDownloadArtifact(blob, val("cv-name") || generated.filename);
    } catch (e) {
      T(e.message, "e");
    }
  };
  window.yyImportConversion = async function() {
    if (!(generated == null ? void 0 : generated.content)) return;
    try {
      if (generated.files) {
        await yyImportChainBundle();
        return;
      }
      var name = val("cv-name");
      var rows = await A("GET", "/api/youyuan/chains");
      if (!rows.ok) throw Error(rows.error);
      if ((rows.files || []).some((f) => {
        var _a;
        return f.name === name || ((_a = f.config) == null ? void 0 : _a.name) === name;
      })) throw Error("\u540C\u540D\u547D\u4EE4\u94FE\u5DF2\u5B58\u5728\uFF0C\u8BF7\u4FEE\u6539\u6587\u4EF6\u540D");
      var delay = Number(val("cv-delay")), count = Number(val("cv-count"));
      if (!Number.isInteger(delay) || delay < 0 || !Number.isInteger(count) || count < 0) throw Error("\u5EF6\u8FDF\u4E0E\u8F6E\u6570\u9700\u8981\u975E\u8D1F\u6574\u6570");
      var r = await A("POST", "/api/youyuan/chains", { action: "save", name, content: generated.content, config: { enabled: false, mode: "fast", interval_ms: 1e3, delay_ms: delay, run_count: count, channel: val("cv-channel") } });
      if (!r.ok) throw Error(r.error);
      get("cv-result").textContent += "\n\u5DF2\u4FDD\u5B58\u4E3A\u672A\u542F\u52A8\u7684\u547D\u4EE4\u94FE\uFF1A" + name;
      T("\u5DF2\u5BFC\u5165\uFF0C\u5C1A\u672A\u542F\u52A8", "s");
    } catch (e) {
      T(e.message, "e");
    }
  };
  async function yyImportChainBundle() {
    var r = await A("GET", "/api/youyuan/chains");
    if (!r.ok) throw Error(r.error);
    var existing = new Set((r.files || []).map((f2) => {
      var _a;
      return ((_a = f2.config) == null ? void 0 : _a.name) || f2.name;
    }));
    for (var f of generated.files) if (existing.has(f.name)) throw Error("\u540C\u540D\u6587\u4EF6\u5DF2\u5B58\u5728\uFF1A" + f.name + "\uFF1B\u8BF7\u4FEE\u6539\u6E90\u6587\u4EF6\u540D\u540E\u91CD\u65B0\u63D0\u53D6");
    var created = [];
    try {
      for (var f of generated.files) {
        var result = await A("POST", "/api/youyuan/chains", { action: "save", name: f.name, content: f.content, config: { enabled: false, mode: "fast", channel: val("cv-channel"), interval_ms: 50, delay_ms: 0, run_count: 1 } });
        if (!result.ok) throw Error(result.error);
        created.push(f.name);
      }
    } catch (e) {
      var left = [];
      for (var n of created) {
        try {
          var removed = await A("POST", "/api/youyuan/chains", { action: "delete", name: n });
          if (!removed.ok) left.push(n);
        } catch (_) {
          left.push(n);
        }
      }
      throw Error(e.message + (left.length ? "\uFF1B\u672A\u56DE\u6EDA\u6587\u4EF6\uFF1A" + left.join("\u3001") : ""));
    }
    get("cv-result").textContent += "\n\u5DF2\u4FDD\u5B58 " + created.length + " \u6761\u672A\u542F\u52A8\u547D\u4EE4\u94FE\uFF0C\u9ED8\u8BA4\u5355\u6B21\uFF1B\u5BA1\u6838\u7EA2\u77F3\u3001\u52A8\u6001\u63A5\u7EBF\u4E0E\u539F\u751F\u5EF6\u8FDF\u540E\u518D\u542F\u52A8\u3002";
    T("\u5DF2\u4FDD\u5B58\u62C6\u5206\u94FE\uFF0C\u5C1A\u672A\u8FD0\u884C", "s");
  }
})();
