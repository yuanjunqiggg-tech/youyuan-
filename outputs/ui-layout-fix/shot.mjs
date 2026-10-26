import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const { chromium } = require('C:/Users/Lenovo/.workbuddy-ai/binaries/node/workspace/node_modules/playwright-core');

const HARNESS = 'file:///' + path.join(__dirname, 'harness.html').split(path.sep).join('/');
const browser = await chromium.launch({
  executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
  headless: true
});

for (const [w, h] of [[360, 800], [800, 900]]) {
  for (const theme of ['light', 'dark']) {
    const ctx = await browser.newContext({ viewport: { width: w, height: h } });
    const p = await ctx.newPage();
    await p.goto(HARNESS + (theme === 'dark' ? '?theme=dark' : ''), { waitUntil: 'load' });
    await p.waitForTimeout(500);
    await p.evaluate(() => {
      document.querySelectorAll('details.yy-account-card').forEach(d => { d.open = true; });
      const f = document.querySelector('.yy-acct-figure');
      window.scrollTo(0, f.getBoundingClientRect().top + window.scrollY - 8);
    });
    await p.screenshot({ path: path.join(__dirname, `shot-${w}-${theme}.png`) });
    await ctx.close();
  }
}
await browser.close();
console.log('shots done');
