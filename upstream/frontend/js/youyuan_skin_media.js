/* 皮肤与头像图片的取图通道（2026-10-24）
 *
 * 为什么需要它：/api/skin/* 和 /api/qq/player-card 都要 Authorization，
 * 而 <img src> 和 new Image() 发不出这个头 —— 浏览器里所有头像、所有立体皮肤
 * 都会 401，然后退到占位图，看起来就是「皮肤半天加载不出来」「形象生成不了」。
 *
 * 做法：用带鉴权的 fetch 取成 blob，再换成 blob: URL 交给 <img> / 3D 预览。
 *   - 同一次会话里同一张图只取一次（Map 缓存）；
 *   - 失败记进 failed，避免每次重渲染都重试打满请求；
 *   - 全局 MutationObserver 自动接管任何新插入的 img[data-skin-media]，
 *     现有几十处渲染函数都不用改。
 */
(function () {
  "use strict";
  if (window.YYSkinMediaURL) return;

  var objects = new Map();   // url -> Promise<blobURL>
  // 失败过的 url：值是失败时间。以前用 Set 永久钉死——「机器人刚添加、皮肤还在
  // 注册即落库的路上」时第一次取到 JSON 错误，之后就算纹理落了盘，本轮会话里
  // 头像/皮肤也永远停在占位图。现在改成冷却重试：冷却期内直接拒绝（不刷请求），
  // 冷却一过就放行重取，失败态自愈。
  var failed = new Map();    // 网络失败 / HTTP 错误 -> 失败时间
  var dead = new Map();      // 明确不是图片（后端回 JSON 错误）-> 失败时间
  var retryAfter = 12000;    // 冷却 12 秒：落盘预热一般 1~3 秒，12 秒足够且不频繁

  function pinned(set, key) {
    var at = set.get(key);
    if (at == null) return false;
    if (Date.now() - at > retryAfter) {
      set.delete(key);
      return false;
    }
    return true;
  }

  function reasonFor(text, status) {
    var message = String(text || "").trim();
    if (message) {
      try {
        var data = JSON.parse(message);
        if (data && data.error) return data.error;
      } catch (error) {
        // 不是 JSON，就当纯文本用
      }
      return message.slice(0, 80);
    }
    return "HTTP " + status;
  }

  // 清掉某个地址的失败/无效记忆，让下次 YYSkinMediaURL 重新取一次。
  // 用于「纹理还没落盘就先渲染」的瞬态失败：重试前 reset，避免永久钉在失败态。
  window.YYSkinMediaReset = function (url) {
    var key = String(url || "");
    if (!key) return;
    failed.delete(key);
    dead.delete(key);
  };

  window.YYSkinMediaURL = function (url) {
    var key = String(url || "");
    if (!key) return Promise.reject(new Error("没有图片地址"));
    if (/^(data:|blob:)/.test(key)) return Promise.resolve(key);
    if (pinned(dead, key)) return Promise.reject(new Error("这张图后端没有数据"));
    if (pinned(failed, key)) return Promise.reject(new Error("上次没取到"));
    if (objects.has(key)) return objects.get(key);
    var task = fetch(key, { credentials: "same-origin" }).then(function (response) {
      var type = response.headers.get("content-type") || "";
      if (!response.ok) {
        return response.text().then(function (text) {
          throw new Error(reasonFor(text, response.status));
        });
      }
      if (type.indexOf("image/") !== 0) {
        return response.text().then(function (text) {
          dead.set(key, Date.now());
          throw new Error(reasonFor(text, response.status));
        });
      }
      return response.blob();
    }).then(function (blob) {
      return URL.createObjectURL(blob);
    }).catch(function (error) {
      failed.set(key, Date.now());
      objects.delete(key);
      throw error;
    });
    objects.set(key, task);
    return task;
  };

  // 把 <img data-skin-media="URL"> 换成真图；失败时走 data-fallback 或原有 onerror。
  function fill(image) {
    if (!image || image.dataset.skinMediaState) return;
    var url = image.getAttribute("data-skin-media");
    if (!url) return;
    image.dataset.skinMediaState = "loading";
    window.YYSkinMediaURL(url).then(function (objectURL) {
      image.dataset.skinMediaState = "ready";
      image.setAttribute("src", objectURL);
    }).catch(function (error) {
      image.dataset.skinMediaState = "failed";
      image.dataset.skinMediaError = error && error.message ? error.message : "取图失败";
      var fallback = image.getAttribute("data-fallback");
      if (fallback) {
        image.setAttribute("src", fallback);
        image.dataset.fallbackTried = "1";
      }
      if (typeof image.onerror === "function") image.onerror();
    });
  }

  window.YYSkinMediaMount = function (root) {
    var scope = root && root.querySelectorAll ? root : document;
    var list = scope.querySelectorAll ? scope.querySelectorAll("img[data-skin-media]") : [];
    for (var i = 0; i < list.length; i++) fill(list[i]);
    if (scope.matches && scope.matches("img[data-skin-media]")) fill(scope);
  };

  function startObserver() {
    if (typeof MutationObserver !== "function") return;
    var observer = new MutationObserver(function (records) {
      for (var i = 0; i < records.length; i++) {
        var added = records[i].addedNodes || [];
        for (var j = 0; j < added.length; j++) {
          var node = added[j];
          if (node.nodeType !== 1) continue;
          if (node.tagName === "IMG" && node.hasAttribute("data-skin-media")) fill(node);
          else if (node.querySelectorAll) window.YYSkinMediaMount(node);
        }
      }
    });
    observer.observe(document.documentElement, { childList: true, subtree: true });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", startObserver, { once: true });
  } else {
    startObserver();
  }
})();
