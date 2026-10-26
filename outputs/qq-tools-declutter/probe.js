const { chromium } = require("playwright-core");
(async () => {
  const b = await chromium.launch({ executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe", headless: true, args:["--no-sandbox"] });
  const p = await b.newPage({ viewport: { width: 1440, height: 900 } });
  await p.goto("http://127.0.0.1:19429/", { waitUntil: "load" });
  await p.waitForTimeout(2000);
  const r = await p.evaluate(() => {
    const hits = [];
    document.querySelectorAll("body *").forEach((e) => {
      const t = (e.textContent || "").trim();
      if (t.startsWith("登录游园") && e.children.length < 6) hits.push({ tag: e.tagName, id: e.id, cls: e.className });
    });
    // 找最上层固定定位遮罩
    const overlays = [];
    document.querySelectorAll("body > *").forEach((e) => {
      const cs = getComputedStyle(e);
      overlays.push({ tag: e.tagName, id: e.id, cls: String(e.className).slice(0,60), pos: cs.position, z: cs.zIndex, display: cs.display });
    });
    return { hits, overlays };
  });
  console.log(JSON.stringify(r, null, 2));
  await b.close();
})();
