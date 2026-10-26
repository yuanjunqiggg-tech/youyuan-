const { chromium } = require("playwright-core");
(async () => {
  const b = await chromium.launch({ executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe", headless: true, args:["--no-sandbox"] });
  const p = await b.newPage({ viewport: { width: 1440, height: 900 } });
  await p.goto("http://127.0.0.1:19429/", { waitUntil: "load" });
  await p.waitForTimeout(2000);
  const r = await p.evaluate(() => {
    const h1 = Array.from(document.querySelectorAll("h1")).find((e) => e.textContent.includes("登录游园"));
    if (!h1) return { none: true };
    const chain = [];
    let e = h1;
    for (let i = 0; i < 6 && e; i++) {
      const cs = getComputedStyle(e);
      chain.push({ tag: e.tagName, id: e.id, cls: String(e.className).slice(0,80), pos: cs.position, z: cs.zIndex, w: Math.round(e.getBoundingClientRect().width) });
      e = e.parentElement;
    }
    return { chain };
  });
  console.log(JSON.stringify(r, null, 2));
  await b.close();
})();
