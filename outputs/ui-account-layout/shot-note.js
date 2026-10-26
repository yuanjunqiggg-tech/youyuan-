// 分页区元素截图：验证「另有 N 个账号未显示」提示在 flex 行里的排布。
const { chromium } = require("playwright-core");
const path = require("path");

const URL = process.argv[2] || "http://127.0.0.1:8791/__h.html";
const TAG = process.argv[3] || "after";
const OUT = __dirname;

(async () => {
  const browser = await chromium.launch({ executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe" });
  for (const vp of [{ name: "desktop", width: 1440, height: 900 }, { name: "mobile", width: 390, height: 844 }]) {
    const ctx = await browser.newContext({ viewport: { width: vp.width, height: vp.height }, deviceScaleFactor: 2, locale: "zh-CN" });
    const page = await ctx.newPage();
    await page.goto(URL, { waitUntil: "networkidle" });
    await page.waitForTimeout(200);
    const el = await page.$(".yy-pagination");
    if (el) await el.screenshot({ path: path.join(OUT, `crop-pagination-${vp.name}-${TAG}.png`) });
    const pool = await page.$(".yy-pool-panel");
    if (pool) await pool.screenshot({ path: path.join(OUT, `crop-pool-${vp.name}-${TAG}.png`) });
    await ctx.close();
  }
  await browser.close();
  console.log("done");
})().catch((e) => { console.error("FAIL", e); process.exit(1); });
