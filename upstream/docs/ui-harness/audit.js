// 审查用度量脚本：只读取，不改页面。
export function audit() {
  var de = document.documentElement;
  var out = {
    viewport: innerWidth + "x" + innerHeight,
    scrollWidth: de.scrollWidth,
    clientWidth: de.clientWidth,
    horizontalOverflow: de.scrollWidth > de.clientWidth,
    scrollContainers: [],
    overflowers: [],
    smallTouch: [],
    tinyText: [],
    duplicateIds: [],
    brokenImages: [],
    fontSizes: {},
    fontLevels: 0,
    truncated: [],
    lowContrast: []
  };

  function desc(el) {
    var s = el.tagName.toLowerCase();
    if (el.id) s += "#" + el.id;
    if (el.className && typeof el.className === "string") s += "." + el.className.trim().split(/\s+/).slice(0, 2).join(".");
    return s;
  }
  function text(el) {
    return (el.textContent || "").replace(/\s+/g, " ").trim().slice(0, 26);
  }

  // 0. 所有滚动容器的横向溢出（真实页面滚动发生在 .sp-body 里，document 不会变宽）
  var containers = document.querySelectorAll(".sp-body, #sub-page, .yyq-fold-body, .yyq-caps, .yyq-cap-edit, .yyq-tabs, .yycc, .yycc-list");
  for (var ci = 0; ci < containers.length; ci++) {
    var co = containers[ci];
    if (co.scrollWidth > co.clientWidth + 1) {
      out.scrollContainers.push({ el: desc(co), sw: co.scrollWidth, cw: co.clientWidth });
    }
  }

  // 1. 横向溢出：找出所有右边界超出视口的元素
  var all = document.querySelectorAll("body *");
  for (var i = 0; i < all.length; i++) {
    var el = all[i];
    var r = el.getBoundingClientRect();
    if (!r.width || !r.height) continue;
    if (r.right > de.clientWidth + 0.5 || r.left < -0.5) {
      var cs = getComputedStyle(el);
      if (cs.position === "fixed" || cs.visibility === "hidden" || cs.display === "none") continue;
      out.overflowers.push({ el: desc(el), right: Math.round(r.right), left: Math.round(r.left), w: Math.round(r.width), text: text(el) });
    }
  }

  // 2. 触控区 < 44（只统计真正可点的元素）
  var clickables = document.querySelectorAll("button, a[href], select, input:not([type=hidden]), textarea, summary, [onclick], [role=tab], .yyq-sw");
  for (var j = 0; j < clickables.length; j++) {
    var c = clickables[j];
    var cr = c.getBoundingClientRect();
    if (!cr.width || !cr.height) continue;
    var ccs = getComputedStyle(c);
    if (ccs.visibility === "hidden" || ccs.display === "none") continue;
    if (cr.height < 44 || cr.width < 44) {
      out.smallTouch.push({ el: desc(c), w: Math.round(cr.width), h: Math.round(cr.height), text: text(c) || c.getAttribute("aria-label") || c.placeholder || "" });
    }
  }

  // 3. 字号统计
  var nodes = document.querySelectorAll("body *");
  for (var k = 0; k < nodes.length; k++) {
    var n = nodes[k];
    if (!n.firstChild || n.firstChild.nodeType !== 3) continue;
    var t = (n.textContent || "").trim();
    if (!t) continue;
    var fs = parseFloat(getComputedStyle(n).fontSize);
    var key = fs.toFixed(1);
    out.fontSizes[key] = (out.fontSizes[key] || 0) + 1;
    if (fs < 10) out.tinyText.push({ el: desc(n), size: key, text: t.slice(0, 24) });
  }
  out.fontLevels = Object.keys(out.fontSizes).length;

  // 4. 重复 id
  var ids = {};
  var idNodes = document.querySelectorAll("[id]");
  for (var m = 0; m < idNodes.length; m++) {
    var id = idNodes[m].id;
    ids[id] = (ids[id] || 0) + 1;
  }
  for (var id2 in ids) if (ids[id2] > 1) out.duplicateIds.push(id2 + " x" + ids[id2]);

  // 5. 破图
  var imgs = document.querySelectorAll("img");
  for (var p = 0; p < imgs.length; p++) {
    var img = imgs[p];
    var ir = img.getBoundingClientRect();
    if (!img.complete || img.naturalWidth === 0) {
      out.brokenImages.push({ src: String(img.getAttribute("src")).slice(0, 60), visible: ir.width > 0 && ir.height > 0, hasOnerror: !!img.getAttribute("onerror") });
    }
  }

  // 6. 被省略号截断的文字
  var trunc = document.querySelectorAll(".yycc-name, .yyq-drop-text small, .yysi-hero-txt strong, .yysi-hero-txt small, .yysi-person-txt strong, .yy-compact-identity strong, .yy-compact-identity small");
  for (var q2 = 0; q2 < trunc.length; q2++) {
    var te = trunc[q2];
    if (te.scrollWidth > te.clientWidth + 1) {
      out.truncated.push({ el: desc(te), visible: te.clientWidth, full: te.scrollWidth, text: te.textContent.trim().slice(0, 30) });
    }
  }

  // 7. 对比度（全部叶子文字）
  function lum(rgb) {
    var c = rgb.map(function (v) { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); });
    return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
  }
  function parse(s) {
    var mm = String(s).match(/rgba?\(([^)]+)\)/);
    if (!mm) return null;
    var pp = mm[1].split(",").map(function (x) { return parseFloat(x); });
    return { rgb: pp.slice(0, 3), a: pp.length > 3 ? pp[3] : 1 };
  }
  function bgOf(el) {
    var cur = el;
    while (cur && cur !== document.documentElement) {
      var b = parse(getComputedStyle(cur).backgroundColor);
      if (b && b.a > 0.55) return b.rgb;
      cur = cur.parentElement;
    }
    var bodyBg = parse(getComputedStyle(document.body).backgroundColor);
    return (bodyBg && bodyBg.a > 0.55) ? bodyBg.rgb : [15, 23, 32];
  }
  var leaves = document.querySelectorAll("body *");
  for (var q = 0; q < leaves.length; q++) {
    var s2 = leaves[q];
    if (!s2.firstChild || s2.firstChild.nodeType !== 3) continue;
    var tt = (s2.textContent || "").trim();
    if (!tt) continue;
    var st = getComputedStyle(s2);
    if (st.visibility === "hidden" || st.display === "none") continue;
    var fg = parse(st.color);
    if (!fg) continue;
    var bg = bgOf(s2);
    var l1 = lum(fg.rgb), l2 = lum(bg);
    var ratio = (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
    if (ratio < 4.5) {
      out.lowContrast.push({ el: desc(s2), color: st.color, bg: "rgb(" + bg.map(Math.round).join(",") + ")", ratio: ratio.toFixed(2), size: st.fontSize, text: tt.slice(0, 18) });
    }
  }

  return out;
}
