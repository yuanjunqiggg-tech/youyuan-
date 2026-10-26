// review-ui 探查 10：边缘路径——连点、非法输入、空表单
const { chromium } = require('playwright-core');
const BASE = 'https://toolbox.youyuanqi.dpdns.org/';
const EXE = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
const TOKEN = '839649fd95d6d3862b85189d77ad4c219b4cc0b1f5a2d03e652765cc32e3ea07';
const sleep = ms => new Promise(r => setTimeout(r, ms));

(async () => {
  const browser = await chromium.launch({ executablePath: EXE, headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  const errs = [];
  page.on('console', m => { if (m.type() === 'error') errs.push(m.text().slice(0, 140)); });
  page.on('pageerror', e => errs.push('PAGEERROR ' + String(e).slice(0, 140)));

  await page.goto(BASE, { waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.evaluate((t) => { sessionStorage.setItem('yy-cloud-key', t); sessionStorage.setItem('yy-user-control', t); }, TOKEN);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.waitForTimeout(4000);

  const nav = async (name) => {
    await page.evaluate((n) => { const b = [...document.querySelectorAll('button')].find(x => (x.textContent || '').trim() === n && x.offsetParent !== null); if (b) b.click(); }, name);
    await sleep(3500);
  };

  // ===== 1. 终端：空输入连点发送 5 次 + 超长文本 + 特殊字符 =====
  await nav('终端');
  const term = await page.evaluate(() => {
    const inp = [...document.querySelectorAll('input, textarea')].find(x => (x.placeholder || '').includes('指令') || (x.placeholder || '').includes('命令') || (x.placeholder || '').includes('终端'));
    return { found: !!inp, ph: inp ? inp.placeholder : '' };
  });
  // 连点发送 5 次（空输入）
  const sendBtn = await page.$('button:has-text("发送")');
  let clickRes = 'no-btn';
  if (sendBtn) {
    for (let i = 0; i < 5; i++) { await sendBtn.click({ timeout: 2000 }).catch(() => {}); await sleep(150); }
    clickRes = 'clicked-5x';
  }
  await sleep(1500);
  // 超长文本
  const longRes = await page.evaluate(() => {
    const inp = [...document.querySelectorAll('input, textarea')].find(x => (x.placeholder || '').includes('指令') || (x.placeholder || '').includes('命令'));
    if (!inp) return 'no-input';
    const setter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value') || Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value');
    setter.set.call(inp, 'A'.repeat(5000));
    inp.dispatchEvent(new Event('input', { bubbles: true }));
    return { len: inp.value.length, maxlen: inp.maxLength };
  });
  await sleep(800);
  const longAfter = await page.evaluate(() => {
    const inp = [...document.querySelectorAll('input, textarea')].find(x => (x.placeholder || '').includes('指令') || (x.placeholder || '').includes('命令'));
    return inp ? { valLen: inp.value.length } : null;
  });
  console.log('TERM:', JSON.stringify({ term, clickRes, longRes, longAfter }));
  await page.screenshot({ path: 'e1-term.png' });

  // ===== 2. 导出建筑：空文件名 + 特殊字符 =====
  await nav('导出建筑');
  const exp = await page.evaluate(() => {
    const btns = [...document.querySelectorAll('button')].map(b => (b.textContent || '').trim());
    const inputs = [...document.querySelectorAll('input')].map(i => ({ ph: i.placeholder, type: i.type, val: i.value }));
    return { btns: btns.slice(0, 25), inputs };
  });
  // 空文件名点开始导出
  const expClick = await page.evaluate(() => {
    const b = [...document.querySelectorAll('button')].find(x => (x.textContent || '').trim() === '开始导出' && x.offsetParent !== null);
    if (b) { b.click(); return true; } return false;
  });
  await sleep(2000);
  const expAfter = await page.evaluate(() => {
    const toasts = [...document.querySelectorAll('[class*=toast], [class*=Toast], [class*=hint], [class*=error]')].filter(x => x.offsetParent !== null).map(x => (x.textContent || '').trim().slice(0, 80));
    return { toasts, hasError: /必填|不能为空|请输入|文件名/.test(document.body.innerText) };
  });
  console.log('EXPORT:', JSON.stringify({ exp, expClick, expAfter }, null, 1).slice(0, 2500));
  await page.screenshot({ path: 'e2-export.png' });

  // ===== 3. 命令链：新建空名称 =====
  await nav('命令链');
  const chain = await page.evaluate(() => {
    const b = [...document.querySelectorAll('button')].find(x => (x.textContent || '').trim().includes('新建命令链') && x.offsetParent !== null);
    if (b) { b.click(); return true; } return false;
  });
  await sleep(2000);
  const chainModal = await page.evaluate(() => ({
    modals: [...document.querySelectorAll('[class*=modal], [class*=drawer], [class*=dialog]')].filter(x => x.offsetParent !== null).map(m => (m.textContent || '').trim().slice(0, 200)),
  }));
  console.log('CHAIN:', JSON.stringify({ chain, chainModal }, null, 1).slice(0, 1800));
  await page.screenshot({ path: 'e3-chain.png' });

  console.log('ERRS:', JSON.stringify([...new Set(errs)].slice(0, 15)));
  await browser.close();
})().catch(e => { console.error('FATAL', e); process.exit(1); });
