'use strict';
const path = require('path');
const { chromium } = require('playwright');

const files = [
  ['desktop-1440.png', { x: 288, y: 355, w: 730, h: 567 }],
  ['tablet-800.png', { x: 21, y: 339, w: 758, h: 620 }],
  ['phone-390.png', { x: 19, y: 260, w: 352, h: 473 }],
  ['desktop-1440-slime.png', { x: 288, y: 355, w: 730, h: 567 }],
  ['desktop-1440-engine-down.png', { x: 288, y: 355, w: 730, h: 567 }],
  ['preview-coarse.png', null]
];

(async () => {
  const b = await chromium.launch({ executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true });
  const p = await b.newPage();
  for (const [f, rect] of files) {
    const url = 'file:///' + path.resolve(__dirname, f).split(path.sep).join('/');
    await p.goto(url);
    const stat = await p.evaluate(async (rect) => {
      const img = document.querySelector('img');
      await img.decode();
      const c = document.createElement('canvas');
      c.width = img.naturalWidth;
      c.height = img.naturalHeight;
      const g = c.getContext('2d');
      g.drawImage(img, 0, 0);
      const r = rect || { x: 0, y: 0, w: c.width, h: c.height };
      const d = g.getImageData(r.x, r.y, r.w, r.h).data;
      const set = new Set();
      let sum = 0, n = 0;
      for (let i = 0; i < d.length; i += 4) {
        set.add((d[i] >> 3) + '_' + (d[i + 1] >> 3) + '_' + (d[i + 2] >> 3));
        sum += (d[i] + d[i + 1] + d[i + 2]) / 3;
        n++;
      }
      return { w: r.w, h: r.h, colors: set.size, mean: Math.round(sum / n) };
    }, rect);
    console.log(f, JSON.stringify(stat));
  }
  await b.close();
})();
