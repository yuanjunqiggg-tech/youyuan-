/* 模型品牌官方 logo + 自定义下拉选择器。
 *
 * 背景：#yy-qq-ai-model / #yy-qq-image-model 原来是光秃秃的原生 select，
 * 用户要求「有真实模型的官方图」（DeepSeek 蓝鲸鱼、Grok 黑洞等）。
 * 原生 <option> 不能可靠显示图片，所以把原生 select 升级为
 * 「按钮 + 弹出面板」：原生 select 保留在 DOM 里（ visually hidden ），
 * 现有代码 v("yy-qq-ai-model") / select.value / select.add(...) 全部照旧工作，
 * 自定义面板只负责展示 logo 和点击。
 *
 * logo 为本地静态资源 /assets/brands/*.png|svg（厂商官方图下载而来，
 * 避免外链失效；ernie.svg 为官方标准字复刻，见文件内注释）。
 *
 * 语法保持 ES5：旧 Android WebView / 国产内核上没有箭头函数、const/let。
 */
(function () {
  "use strict";

  var LOGO_BASE = "/assets/brands/";

  /* 品牌 → 官方 logo。hosts 匹配 base_url 主机名，models 匹配 model_id 前缀。
   * 只收录真实存在、且 /api/ai/models 常见配置里会出现的厂商。 */
  var BRANDS = [
    { id: "deepseek", logo: "deepseek.png", hosts: ["deepseek.com"], models: ["deepseek"] },
    { id: "grok", logo: "grok.png", hosts: ["x.ai"], models: ["grok"] },
    { id: "openai", logo: "openai.png", hosts: ["openai.com", "openai.azure.com"], models: ["gpt-", "gpt", "o1-", "o1", "o3-", "o3", "o4-", "chatgpt", "dall-e", "text-embedding", "whisper", "tts-", "omni-", "codex", "sora", "realtime", "moderation"] },
    { id: "claude", logo: "claude.png", hosts: ["anthropic.com"], models: ["claude"] },
    { id: "gemini", logo: "gemini.png", hosts: ["generativelanguage.googleapis.com", "aistudio.google.com", "gemini.google.com"], models: ["gemini", "gemma", "imagen", "veo", "palm"] },
    { id: "qwen", logo: "qwen.png", hosts: ["aliyuncs.com", "dashscope", "qwen.ai", "tongyi"], models: ["qwen", "qwq", "qvq", "tongyi"] },
    { id: "zhipu", logo: "zhipu.png", hosts: ["bigmodel.cn", "zhipu.cn", "openbmind"], models: ["glm-", "glm4", "chatglm", "cogview", "cogvideo", "codegeex"] },
    { id: "doubao", logo: "doubao.png", hosts: ["volces.com", "volcengine", "doubao"], models: ["doubao"] },
    { id: "kimi", logo: "kimi.png", hosts: ["moonshot", "kimi"], models: ["kimi", "moonshot"] },
    { id: "ernie", logo: "ernie.svg", hosts: ["baidu.com", "qianfan", "baidubce"], models: ["ernie", "wenxin", "yiyan"] },
    { id: "hunyuan", logo: "hunyuan.png", hosts: ["hunyuan", "tencent.com"], models: ["hunyuan"] }
  ];

  /* 没匹配到官图时的默认图标：首字母圆角色块（按名字取色，别用丑占位）。 */
  var DEF_COLORS = ["#0ea5e9", "#8b5cf6", "#f59e0b", "#10b981", "#ef4444", "#6366f1", "#14b8a6", "#f43f5e"];

  function hostOf(url) {
    var m = /^https?:\/\/([^/]+)/i.exec(String(url || ""));
    return m ? m[1].toLowerCase() : "";
  }

  /* 按 base_url 主机名优先，再按 model_id 前缀找品牌。 */
  function brandOf(baseUrl, modelId) {
    var host = hostOf(baseUrl);
    var mid = String(modelId || "").toLowerCase();
    var i, j;
    if (host) {
      for (i = 0; i < BRANDS.length; i++) {
        for (j = 0; j < BRANDS[i].hosts.length; j++) {
          if (host.indexOf(BRANDS[i].hosts[j]) !== -1) return BRANDS[i];
        }
      }
    }
    if (mid) {
      for (i = 0; i < BRANDS.length; i++) {
        for (j = 0; j < BRANDS[i].models.length; j++) {
          var k = BRANDS[i].models[j];
          if (mid === k || mid.indexOf(k) === 0 || mid.indexOf("-" + k) !== -1) return BRANDS[i];
        }
      }
    }
    return null;
  }

  function hashColor(text) {
    var h = 0;
    for (var i = 0; i < text.length; i++) h = (h * 31 + text.charCodeAt(i)) & 0xffff;
    return DEF_COLORS[h % DEF_COLORS.length];
  }

  function firstLetter(text) {
    var t = String(text || "").trim();
    return t ? t.charAt(0).toUpperCase() : "?";
  }

  window.yyBrandChipFallback = function (img, text) {
    var chip = img.parentNode;
    if (!chip) return;
    chip.classList.add("yyb-chip-def");
    chip.style.background = hashColor(text || "?");
    chip.textContent = firstLetter(text);
  };

  /* ---------- 自定义选择器 ---------- */

  var OPEN_SEL = null;
  var uid = 0;

  function fireChange(el) {
    var ev = null;
    try { ev = new Event("change", { bubbles: true }); } catch (e) {
      ev = document.createEvent("HTMLEvents");
      ev.initEvent("change", true, false);
    }
    el.dispatchEvent(ev);
  }

  function modelInfoFor(sel, opt) {
    /* 优先 option 上挂的 data-yy-base/data-yy-mid（获取模型列表时写入），
     * 再按 value 去 attach 时传入的 models 数组里找。 */
    var base = opt.getAttribute("data-yy-base") || "";
    var mid = opt.getAttribute("data-yy-mid") || "";
    if (!mid || !base) {
      var list = sel.__yybModels || [];
      for (var i = 0; i < list.length; i++) {
        if (list[i] && list[i].name === opt.value) {
          base = base || list[i].base_url || "";
          mid = mid || list[i].model_id || "";
          break;
        }
      }
    }
    return { base: base, mid: mid || opt.value };
  }

  function buildPanel(sel) {
    var panel = sel.__yybPanel;
    panel.innerHTML = "";
    var opts = sel.options;
    for (var i = 0; i < opts.length; i++) {
      (function (opt) {
        var info = modelInfoFor(sel, opt);
        var brand = brandOf(info.base, info.mid);
        var row = document.createElement("div");
        row.className = "yyb-option";
        row.setAttribute("role", "option");
        row.setAttribute("data-value", opt.value);
        row.setAttribute("aria-selected", opt.selected ? "true" : "false");
        if (opt.selected) row.classList.add("yyb-sel");
        var chip = document.createElement("span");
        chip.className = "yyb-chip";
        if (brand) {
          var img = document.createElement("img");
          img.src = LOGO_BASE + brand.logo;
          img.alt = brand.id;
          img.loading = "lazy";
          img.decoding = "async";
          img.onerror = function () { window.yyBrandChipFallback(img, info.mid || opt.value); };
          chip.appendChild(img);
        } else {
          chip.classList.add("yyb-chip-def");
          chip.style.background = hashColor(info.mid || opt.value || "?");
          chip.textContent = firstLetter(info.mid || opt.value);
        }
        row.appendChild(chip);
        var text = document.createElement("span");
        text.className = "yyb-text";
        var b = document.createElement("b");
        b.textContent = opt.textContent || opt.value || "未配置";
        var small = document.createElement("small");
        small.textContent = info.mid && info.mid !== (opt.textContent || "") ? info.mid : (opt.value ? "" : "");
        text.appendChild(b);
        if (small.textContent) text.appendChild(small);
        row.appendChild(text);
        row.onclick = function (ev) {
          ev.stopPropagation();
          sel.value = opt.value;
          fireChange(sel);
          close(sel);
        };
        panel.appendChild(row);
      })(opts[i]);
    }
  }

  function syncLabel(sel) {
    var trigger = sel.__yybTrigger;
    if (!trigger) return;
    var opt = sel.options[sel.selectedIndex];
    trigger.innerHTML = "";
    var chip = document.createElement("span");
    chip.className = "yyb-chip";
    if (opt && opt.value) {
      var info = modelInfoFor(sel, opt);
      var brand = brandOf(info.base, info.mid);
      if (brand) {
        var img = document.createElement("img");
        img.src = LOGO_BASE + brand.logo;
        img.alt = brand.id;
        img.onerror = function () { window.yyBrandChipFallback(img, info.mid || opt.value); };
        chip.appendChild(img);
      } else {
        chip.classList.add("yyb-chip-def");
        chip.style.background = hashColor(info.mid || opt.value);
        chip.textContent = firstLetter(info.mid || opt.value);
      }
    } else {
      chip.classList.add("yyb-chip-def");
      chip.style.background = "rgba(127,127,127,.25)";
      chip.textContent = "–";
    }
    trigger.appendChild(chip);
    var label = document.createElement("span");
    label.className = "yyb-label";
    label.textContent = opt ? (opt.textContent || opt.value) : "未配置";
    trigger.appendChild(label);
    var caret = document.createElement("span");
    caret.className = "yyb-caret";
    caret.textContent = "▾";
    trigger.appendChild(caret);
  }

  function placePanel(sel) {
    var r = sel.__yybTrigger.getBoundingClientRect();
    var panel = sel.__yybPanel;
    panel.style.left = r.left + "px";
    panel.style.width = r.width + "px";
    var ph = Math.min(panel.scrollHeight + 14, Math.round(window.innerHeight * 0.5));
    var below = window.innerHeight - r.bottom - 8;
    if (below < ph && r.top - 8 > below) {
      panel.style.top = Math.max(8, r.top - 8 - ph) + "px";
      panel.classList.add("yyb-up");
    } else {
      panel.style.top = (r.bottom + 4) + "px";
      panel.classList.remove("yyb-up");
    }
  }

  function close(sel) {
    var panel = sel.__yybPanel;
    if (!panel || panel.hidden) return;
    panel.hidden = true;
    sel.__yybWrap.classList.remove("yyb-open");
    sel.__yybTrigger.setAttribute("aria-expanded", "false");
    if (OPEN_SEL === sel) OPEN_SEL = null;
  }

  function closeAll() {
    if (OPEN_SEL) close(OPEN_SEL);
  }

  function open(sel) {
    closeAll();
    buildPanel(sel);
    var panel = sel.__yybPanel;
    panel.hidden = false;
    sel.__yybWrap.classList.add("yyb-open");
    sel.__yybTrigger.setAttribute("aria-expanded", "true");
    OPEN_SEL = sel;
    placePanel(sel);
    var active = panel.querySelector(".yyb-option.yyb-sel") || panel.querySelector(".yyb-option");
    if (active) {
      active.classList.add("yyb-active");
      active.focus && active.focus();
    }
  }

  function toggle(sel) {
    if (sel.__yybPanel.hidden) open(sel);
    else close(sel);
  }

  /* attach：把原生 select 升级成自定义选择器。幂等，可重复调用（刷新用）。 */
  function attach(sel, models) {
    if (!sel || !sel.tagName || sel.tagName.toUpperCase() !== "SELECT") return;
    if (models) sel.__yybModels = models;
    if (sel.__yyb) { buildPanel(sel); syncLabel(sel); return; }
    sel.__yyb = 1;
    sel.classList.add("yyb-native");
    sel.setAttribute("tabindex", "-1");
    var wrap = document.createElement("div");
    wrap.className = "yyb-select";
    var trigger = document.createElement("button");
    trigger.type = "button";
    trigger.className = "yyb-trigger";
    trigger.id = "yyb-trigger-" + (++uid);
    trigger.setAttribute("aria-haspopup", "listbox");
    trigger.setAttribute("aria-expanded", "false");
    var panel = document.createElement("div");
    panel.className = "yyb-panel";
    panel.id = "yyb-panel-" + uid;
    panel.hidden = true;
    panel.setAttribute("role", "listbox");
    trigger.setAttribute("aria-controls", panel.id);
    sel.parentNode.insertBefore(wrap, sel.nextSibling);
    wrap.appendChild(trigger);
    wrap.appendChild(panel);
    sel.__yybWrap = wrap;
    sel.__yybTrigger = trigger;
    sel.__yybPanel = panel;
    trigger.onclick = function (e) { e.stopPropagation(); toggle(sel); };
    trigger.onkeydown = function (e) {
      if (e.key === "ArrowDown" || e.key === "ArrowUp" || e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        open(sel);
      }
    };
    panel.onkeydown = function (e) {
      var rows = panel.querySelectorAll(".yyb-option");
      var idx = -1;
      for (var i = 0; i < rows.length; i++) if (rows[i].classList.contains("yyb-active")) idx = i;
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        var next = e.key === "ArrowDown" ? Math.min(idx + 1, rows.length - 1) : Math.max(idx - 1, 0);
        if (rows[idx]) rows[idx].classList.remove("yyb-active");
        if (rows[next]) { rows[next].classList.add("yyb-active"); rows[next].focus(); }
      } else if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        if (rows[idx]) rows[idx].onclick({ stopPropagation: function () {} });
      } else if (e.key === "Escape") {
        e.preventDefault();
        close(sel);
        trigger.focus();
      } else if (e.key === "Tab") {
        close(sel);
      }
    };
    sel.addEventListener("change", function () {
      buildPanel(sel);
      syncLabel(sel);
    });
    buildPanel(sel);
    syncLabel(sel);
  }

  function refresh(sel, models) {
    if (!sel || !sel.__yyb) return;
    if (models) sel.__yybModels = models;
    buildPanel(sel);
    syncLabel(sel);
  }

  /* 全局收场：点空白 / Esc 关闭；滚动与缩放时重新定位（fixed 定位防折叠块裁剪）。 */
  document.addEventListener("click", function () { closeAll(); });
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") closeAll();
  });
  window.addEventListener("resize", function () { if (OPEN_SEL) placePanel(OPEN_SEL); });
  window.addEventListener("scroll", function () { if (OPEN_SEL) placePanel(OPEN_SEL); }, true);

  window.yyBrandAttach = attach;
  window.yyBrandRefresh = refresh;
  window.yyBrandOf = brandOf;
})();
