(function() {
  "use strict";
  // 超级管理员认领码 · 二维码
  //
  // 用户原话：「生成超级管理员认领码的话 可以变成用QQ去扫一张二维码」。
  // 面板原本只显示一串 24 位文本码，得手打/复制到群里。这里在文本码下面补一张
  // 二维码：手机 QQ「扫一扫」得到 `.认领 <码>` 原文，复制粘进群即可。
  // 文本码仍然照旧显示——二维码接口不可用（旧版后端、断网）时它就是兜底。
  //
  // 依赖：youyuan_qq_assistant.js 里的全局 A(...) 和 window.yyQQCreateClaim。
  // 本文件在它之后加载（见 index.html）；若加载顺序反了，DOMContentLoaded 里补一次覆盖。

  var API = "/api/qq/claim/qr";

  function qrHolder() {
    var text = document.getElementById("yy-qq-claim");
    if (!text) return null;
    var box = document.getElementById("yy-qq-claim-qr");
    if (!box) {
      box = document.createElement("div");
      box.id = "yy-qq-claim-qr";
      box.style.cssText = "display:flex;gap:12px;align-items:flex-start;flex-wrap:wrap;margin:8px 0";
      if (text.parentNode) text.parentNode.insertBefore(box, text.nextSibling);
    }
    return box;
  }

  // 后端 instruction 形如「在目标群 @机器人发送 .认领 ABC」，二维码只装真正的指令。
  function claimCommand(r) {
    var raw = String((r && r.instruction) || "").trim();
    var m = raw.match(/发送\s*([\s\S]+)$/);
    return (m ? m[1] : raw).trim();
  }

  function renderQR(box, cmd) {
    if (!box) return;
    if (!cmd) {
      box.innerHTML = "";
      return;
    }
    box.innerHTML = "";
    var img = document.createElement("img");
    img.alt = "认领码二维码";
    img.width = 180;
    img.height = 180;
    img.style.cssText = "width:180px;height:180px;image-rendering:pixelated;background:#fff;border-radius:8px;display:block";
    img.onerror = function() {
      // 二维码生成失败不打断流程：文本码还在上面，照样能用。
      box.innerHTML = '<span style="font-size:12px;opacity:.7">二维码生成失败，请用上面的文本码</span>';
    };
    img.src = API + "?scale=6&text=" + encodeURIComponent(cmd);
    var tip = document.createElement("span");
    tip.style.cssText = "font-size:12px;line-height:1.6;opacity:.75;max-width:220px";
    tip.textContent = "手机 QQ → 扫一扫 → 复制识别到的文字 → 粘到群里发送。二维码打不开时以上面的文本码为准。";
    box.appendChild(img);
    box.appendChild(tip);
  }

  var prevCreateClaim = window.yyQQCreateClaim;

  window.yyQQCreateClaim = async function() {
    // 请求层没加载（api.js 缺失）时退回原实现，别把面板弄坏。
    if (typeof A !== "function") {
      if (typeof prevCreateClaim === "function") return prevCreateClaim();
      return null;
    }
    var box = qrHolder();
    if (box) box.innerHTML = '<span style="font-size:12px;opacity:.7">生成中…</span>';
    var r;
    try {
      r = await A("POST", "/api/qq/assistant", { action: "claim" });
    } catch (e) {
      r = { ok: false, error: String((e && e.message) || e) };
    }
    var text = document.getElementById("yy-qq-claim");
    if (!text) return r;
    if (!r || !r.ok) {
      text.textContent = (r && r.error) || "生成失败";
      if (box) box.innerHTML = "";
      return r;
    }
    var cmd = claimCommand(r);
    text.textContent = r.instruction + "\n10 分钟有效，一次使用；也可以用手机 QQ 扫下面的二维码。";
    renderQR(box, cmd);
    return r;
  };
})();
