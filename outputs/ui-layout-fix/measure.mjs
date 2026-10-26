import { createRequire } from 'node:module';
import path from 'node:path';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const { chromium } = require('C:/Users/Lenovo/.workbuddy-ai/binaries/node/workspace/node_modules/playwright-core');
const HARNESS = 'file:///' + path.join(__dirname, 'harness.html').replace(/\\/g, '/');
const WIDTHS = process.argv[3] ? process.argv[3].split(',').map(Number) : [320, 360, 390, 430, 600, 800, 1024, 1440];
const THEME = process.argv[2] === 'dark' ? 'dark' : 'light';
const TAG = process.argv[3] ? 'sweep' : THEME;

const browser = await chromium.launch({
  executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
  headless: true,
  args: ['--allow-file-access-from-files', '--font-render-hinting=none']
});

const out = [];
for (const w of WIDTHS) {
  const height = w <= 600 ? 800 : 900;
  const ctx = await browser.newContext({ viewport: { width: w, height }, deviceScaleFactor: 1 });
  const page = await ctx.newPage();
  await page.goto(HARNESS + (THEME === 'dark' ? '?theme=dark' : ''), { waitUntil: 'load' });
  await page.waitForTimeout(400);
  const m = await page.evaluate(() => window.__measure());
  m.width = w;
  m.height = height;
  m.theme = THEME;
  out.push(m);
  await ctx.close();
}
await browser.close();

fs.writeFileSync(path.join(__dirname, `data-${TAG}.json`), JSON.stringify(out, null, 2));

const pad = (v, n) => String(v).padStart(n);
console.log(`theme=${THEME}`);
console.log('W     statusX(6行)                        漂移  bar L/R/W       barT..B  vh   行高 溢  bar可见 模型 T..B (图高) 下方余量');
for (const m of out) {
  const sx = m.statusX.map(v => Math.round(v)).join(',');
  console.log(
    pad(m.width, 5) + ' ' + pad(sx, 38) + ' ' + pad(m.statusDrift, 5) + ' ' +
    pad(Math.round(m.bar.l) + '/' + Math.round(m.bar.r) + '/' + Math.round(m.bar.w), 16) + ' ' +
    pad(Math.round(m.bar.t) + '..' + Math.round(m.bar.b), 9) + ' ' + pad(m.innerHeight, 5) + ' ' +
    pad(m.barRows, 3) + ' ' + pad(m.overflowX, 3) + ' ' + pad(m.barInViewport ? 'Y' : 'N', 5) + '  ' +
    (m.model ? pad(Math.round(m.model.t) + '..' + Math.round(m.model.b), 11) + ' (' + Math.round(m.figure.h) + ')' : '  --         ') + '   ' +
    (m.modelGapBelow === null ? '--' : Math.round(m.modelGapBelow))
  );
}
