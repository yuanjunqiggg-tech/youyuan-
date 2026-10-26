/* 皮肤库组件：组件码（skin_item_id）→ 真实皮肤图（2026-10-26）
 *
 * 为什么单独一个文件：皮肤库的界面骨架和「换肤」写入链路在 youyuan_accounts.js 里，
 * 但「按组件码取图」是另一件事——网易只提供 item_id → res_url 的接口，
 * 原来的取图链路全按**玩家名/账号当前穿着**走，所以皮肤库里除「机器人正好穿着」
 * 的哪一款之外，所有卡片都只有三位数字方块，用户看到的就是「组件没正常加载」。
 *
 * 这里只做覆盖（override），不改 accounts.js：
 *   - yySkinLibraryFetchTexture：改走 /api/skin/item-texture?item_id=<组件码>，
 *     任意组件码都能出图，不再要求 row.skin_item_id === 组件码；
 *   - yySkinLibrarySyncReal：打开抽屉时把**每张**卡片都补一次真图（并发 3）；
 *   - yySkinLibraryEntries：并入群友分享到本机服务端的组件码；
 *   - yySkinLibraryRender：给卡片加「群友 / 暂无图」标记与失败原因；
 *   - yySkinLibraryOpen：在抽屉里加「分享组件码」区，并把顶部说明改成实际行为。
 * 依赖的全局符号全部来自 accounts.js，替换的是同名全局函数，调用点无需改动。
 */
(function () {
  "use strict";
  if (window.__yySkinLibraryComponent) return;
  if (typeof window.yySkinLibraryFetchTexture !== "function" || typeof window.yySkinLibraryOpen !== "function") return;
  window.__yySkinLibraryComponent = true;

  var legacyFetch = window.yySkinLibraryFetchTexture;
  var legacyOpen = window.yySkinLibraryOpen;
  var legacyEntries = window.yySkinLibraryEntries;

  var reason = {};    // 组件码 → 取图失败原因（给卡片显示，不静默）
  var fetching = {};  // 组件码 → 进行中的 Promise，避免重复打接口
  var shared = null;  // 服务端共享组件码列表；null 表示还没拉到

  function text(value) {
    return String(value == null ? "" : value);
  }

  function escapeHTML(value) {
    if (typeof window.yyEscape === "function") return window.yyEscape(value);
    return text(value).replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }

  function targetRow() {
    return typeof window.yySkinLibraryTargetRow === "function" ? window.yySkinLibraryTargetRow() : null;
  }

  // 有了换肤目标账号才带上 id：后端优先用它的登录态调网易组件接口；
  // 没有账号时后端会退回 插件数据文件/皮肤组件凭据.json 里配置的登录态。
  function itemURL(code) {
    var query = "?item_id=" + encodeURIComponent(code);
    var row = targetRow();
    if (row && row.id) query += "&id=" + encodeURIComponent(row.id);
    return "/api/skin/item-texture" + query;
  }

  function rememberHead(code, texture) {
    if (typeof window.yySkinCropHead !== "function") return Promise.resolve("");
    return window.yySkinCropHead(texture).then(function (head) {
      if (head) window.yySkinHeadImage[code] = head;
      return head;
    }, function () {
      return "";
    });
  }

  // 组件码 → 纹理。这是本文件的核心：任意组件码都先问后端要图，
  // 拿不到才退回原链路（机器人当前穿着它时，按玩家的缓存仍能出图）。
  window.yySkinLibraryFetchTexture = function (code, force) {
    var key = text(code).trim();
    if (!key) return Promise.resolve("");
    if (!force && window.yySkinRealTexture[key]) return Promise.resolve(window.yySkinRealTexture[key]);
    if (fetching[key]) return fetching[key];

    var url = itemURL(key);
    if (force && typeof window.YYSkinMediaReset === "function") window.YYSkinMediaReset(url);

    var request = typeof window.YYSkinMediaURL === "function"
      ? window.YYSkinMediaURL(url)
      : Promise.reject(new Error("取图通道未加载"));

    var task = request.then(function (objectURL) {
      window.yySkinRealTexture[key] = objectURL;
      delete reason[key];
      return rememberHead(key, objectURL).then(function () {
        return objectURL;
      });
    }).catch(function (error) {
      var why = (error && error.message) || "取不到这张皮肤图";
      // 直接按组件码取失败时退回老链路：机器人正好穿着这一款时仍能出图。
      return Promise.resolve(legacyFetch.call(window, key, force)).then(function (value) {
        if (value) {
          delete reason[key];
          return value;
        }
        reason[key] = why;
        return "";
      }, function () {
        reason[key] = why;
        return "";
      });
    }).then(function (value) {
      delete fetching[key];
      return value;
    });

    fetching[key] = task;
    return task;
  };

  // 打开抽屉时把每张卡片都补一次真图。原实现只补「账号当前穿着的那一款」，
  // 所以其余卡片永远是组件码方块。
  window.yySkinLibrarySyncReal = function () {
    var entries = window.yySkinLibraryEntries();
    var pending = entries.filter(function (entry) {
      return entry && entry.code && !window.yySkinHeadImage[entry.code];
    });
    if (!pending.length) return Promise.resolve();

    var cursor = 0;
    var running = 0;
    var limit = 3;   // 并发 3：够快，又不会一次打满本机接口
    var changed = false;

    return new Promise(function (resolve) {
      function pump() {
        if (cursor >= pending.length && running === 0) {
          if (changed && typeof window.yySkinLibraryRender === "function") window.yySkinLibraryRender();
          resolve();
          return;
        }
        while (running < limit && cursor < pending.length) {
          running += 1;
          window.yySkinLibraryFetchTexture(pending[cursor++].code).then(function (value) {
            if (value) changed = true;
            running -= 1;
            pump();
          }, function () {
            running -= 1;
            pump();
          });
        }
      }
      pump();
    });
  };

  // 并入群友分享的组件码（服务端存的），与本机 localStorage 里的「我的」合并去重。
  window.yySkinLibraryEntries = function () {
    var base = legacyEntries.apply(window, arguments) || [];
    if (!shared) return base;
    var seen = {};
    base.forEach(function (entry) {
      if (entry && entry.code) seen[text(entry.code)] = true;
    });
    var extra = [];
    shared.forEach(function (item) {
      var code = text(item && item.code).trim();
      if (!/^\d{1,32}$/.test(code) || seen[code]) return;
      seen[code] = true;
      extra.push({
        key: "shared-" + code,
        name: text(item.name) || "组件码 " + code,
        code: code,
        shared: true,
        tone: typeof window.yySkinToneFromCode === "function" ? window.yySkinToneFromCode(code) : ""
      });
    });
    return base.concat(extra);
  };

  window.yySkinLibraryRender = function () {
    var grid = document.getElementById("yy-skin-grid");
    if (!grid) return;
    var entries = window.yySkinLibraryEntries();
    var picked = window.yySkinLibraryPickKey;
    if (!entries.some(function (entry) { return entry.key === picked; })) {
      window.yySkinLibraryPickKey = entries.length ? entries[0].key : "";
      picked = window.yySkinLibraryPickKey;
    }
    var target = targetRow();
    var worn = text(target && target.skin_item_id);

    grid.innerHTML = entries.map(function (entry) {
      var code = text(entry.code);
      var selected = entry.key === picked ? " selected" : "";
      var applied = code && worn && code === worn;
      var kind = entry.custom ? "我的" : (entry.shared ? "群友" : "内置");
      var why = code ? reason[code] : "";
      var tag = kind + (applied ? " · 已生效" : (why ? " · 暂无图" : ""));
      var title = entry.name + (why ? "（" + why + "）" : "");
      return '<div class="yy-skin-card' + selected + '" role="button" tabindex="0" data-key="' + escapeHTML(entry.key) + '"' +
        ' onclick="yySkinLibraryPick(\'' + escapeHTML(entry.key) + '\')"' +
        ' onkeydown="yySkinCardKey(event,\'' + escapeHTML(entry.key) + '\')"' +
        ' title="' + escapeHTML(title) + '">' +
        window.yySkinLibraryFaceHTML(entry) +
        '<span class="yy-skin-card-name">' + escapeHTML(entry.name) + "</span>" +
        '<span class="yy-skin-card-tag">' + escapeHTML(tag) + "</span>" +
        (code ? '<button type="button" class="yy-skin-card-apply" onclick="event.stopPropagation();yySkinLibraryUse(\'' + escapeHTML(entry.key) + '\',this)">让机器人用这款</button>' : "") +
        (entry.custom ? '<button type="button" class="yy-skin-card-del" onclick="event.stopPropagation();yySkinLibraryRemoveCustom(\'' + escapeHTML(entry.key) + '\')">移除</button>' : "") +
        "</div>";
    }).join("");

    var current = typeof window.yySkinLibraryCurrent === "function" ? window.yySkinLibraryCurrent() : null;
    var nameEl = document.getElementById("yy-skin-preview-name");
    if (nameEl) nameEl.textContent = current ? current.name : "选一个皮肤";
    var head = current && current.code ? window.yySkinHeadImage[current.code] : "";
    var tagEl = document.getElementById("yy-skin-preview-tag");
    if (tagEl) {
      if (!current || !current.code) tagEl.textContent = "";
      else if (head) tagEl.textContent = "组件码 " + current.code + " · 真实皮肤";
      else if (reason[current.code]) tagEl.textContent = "组件码 " + current.code + " · " + reason[current.code];
      else tagEl.textContent = "组件码 " + current.code + " · 取图中";
    }
    var front = document.getElementById("yy-skin-face-front");
    if (front) front.style.backgroundImage = head ? "url(" + head + ")" : "none";
    var faces = document.querySelectorAll(".yy-skin-turn-face.back,.yy-skin-turn-face.left,.yy-skin-turn-face.right");
    for (var i = 0; i < faces.length; i++) faces[i].style.backgroundImage = head ? "url(" + head + ")" : "none";
    // 没有真实皮肤图时整块换成「暂无预览 + 组件码」，不显示拼出来的假方块。
    var stage = document.getElementById("yy-skin-turn-stage");
    if (stage) stage.classList.toggle("yy-skin-stage-empty", !head);
    var emptyCode = document.getElementById("yy-skin-turn-empty-code");
    if (emptyCode) emptyCode.textContent = current && current.code ? "组件码 " + current.code : "";
    if (typeof window.yySkinTurnPaint === "function") window.yySkinTurnPaint();
  };

  // ---------- 群友分享的组件码 ----------

  function loadShared() {
    return fetch("/api/skin/item-library", { credentials: "same-origin" }).then(function (response) {
      return response.ok ? response.json() : null;
    }).then(function (data) {
      if (data && data.ok && Array.isArray(data.items)) shared = data.items;
      if (typeof window.yySkinLibraryRender === "function") window.yySkinLibraryRender();
      window.yySkinLibrarySyncReal();
    }).catch(function () {
      // 拉不到就只用本机列表，不打断皮肤库
    });
  }

  function shareStatus(message) {
    var out = document.getElementById("yy-skin-share-status");
    if (out) out.textContent = message;
  }

  function submitShare() {
    var codeEl = document.getElementById("yy-skin-share-code");
    var nameEl = document.getElementById("yy-skin-share-name");
    var code = text(codeEl && codeEl.value).trim();
    if (!/^\d{1,32}$/.test(code)) {
      shareStatus("组件码是纯数字，请检查");
      return;
    }
    shareStatus("正在保存…");
    fetch("/api/skin/item-library", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code: code, name: text(nameEl && nameEl.value).trim() })
    }).then(function (response) {
      return response.json();
    }).then(function (data) {
      if (!data || !data.ok) throw new Error((data && data.error) || "保存失败");
      shared = Array.isArray(data.items) ? data.items : shared;
      if (codeEl) codeEl.value = "";
      if (nameEl) nameEl.value = "";
      shareStatus("已加入皮肤库，点上面的卡片即可预览");
      window.yySkinLibraryRender();
      window.yySkinLibrarySyncReal();
    }).catch(function (error) {
      shareStatus((error && error.message) || "保存失败");
    });
  }

  function injectShare(panel) {
    if (!panel || panel.querySelector(".yy-skin-share")) return;
    var section = document.createElement("section");
    section.className = "yy-skin-share";
    section.innerHTML =
      '<h4>把组件码分享给大家</h4>' +
      '<p class="yy-hint">贴一个纯数字组件码，本机会记住它；别人打开皮肤库也能看到，点一下就直接预览这一款。</p>' +
      '<label>组件码<input id="yy-skin-share-code" inputmode="numeric" autocomplete="off" placeholder="纯数字组件码"></label>' +
      '<label>叫法（可选）<input id="yy-skin-share-name" maxlength="24" autocomplete="off" placeholder="例如 群友分享的皮肤"></label>' +
      '<div class="yy-actions"><button type="button" class="btn-s" id="yy-skin-share-submit">分享到皮肤库</button></div>' +
      '<p id="yy-skin-share-status" class="yy-hint" role="status"></p>';
    var custom = panel.querySelector(".yy-skin-custom");
    if (custom && custom.parentNode === panel) panel.insertBefore(section, custom);
    else panel.appendChild(section);
    var button = section.querySelector("#yy-skin-share-submit");
    if (button) button.addEventListener("click", submitShare);
  }

  window.yySkinLibraryOpen = function () {
    legacyOpen.apply(window, arguments);
    var drawer = document.getElementById("yy-skin-drawer");
    var panel = drawer ? drawer.querySelector(".yy-skin-drawer-panel") : null;
    if (!panel) return;
    // 顶部说明原来写「下面 6 款是内置免费皮肤」，现在列表还含群友分享的，改成实际行为。
    for (var i = 0; i < panel.children.length; i++) {
      var child = panel.children[i];
      if (child.tagName === "P" && child.className.indexOf("yy-hint") >= 0) {
        child.textContent = "内置几款和群友分享的组件码都在下面：点卡片预览，点「让机器人用这款」换到选中的账号，换好后重新进服生效。";
        break;
      }
    }
    injectShare(panel);
    loadShared();
  };
})();
