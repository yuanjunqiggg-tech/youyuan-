// Cloudflare dash 登录（headed Chrome，持久化用户数据目录）。
// 用法：node cf_login.js step1  → 登录并停在选择账户页
//       node cf_login.js probe → 打开 Turnstile 页并打印页面状态
// 用户已明确授权使用其账号（yuanjunqiggg@gmail.com）登录并创建 Turnstile widget。
const {chromium} = require('playwright-core');
const fs = require('fs');
const path = require('path');

const PROFILE = 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/private/cf-browser-profile';
const CHROME = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
const EMAIL = 'yuanjunqiggg@gmail.com';
const PASSWORD = 'YUANJUNQI314';

async function sleep(ms) {return new Promise(r => setTimeout(r, ms))}

async function main() {
  fs.mkdirSync(PROFILE, {recursive: true});
  const ctx = await chromium.launchPersistentContext(PROFILE, {
    executablePath: CHROME, headless: false, viewport: null,
    args: ['--start-maximized'],
  });
  const page = ctx.pages()[0] || await ctx.newPage();
  const mode = process.argv[2] || 'step1';

  if (mode === 'step1') {
    await page.goto('https://dash.cloudflare.com/login', {waitUntil: 'domcontentloaded', timeout: 60000});
    await sleep(4000);
    console.log('URL:', page.url());
    console.log('TITLE:', await page.title());
    // 已登录会跳转 dash
    if (!/login/.test(page.url())) {console.log('ALREADY-LOGGED-IN');}
    else {
      const email = page.locator('input[name="email"], input[type="email"]').first();
      await email.waitFor({timeout: 20000});
      await email.fill(EMAIL);
      console.log('EMAIL-FILLED');
      // 点 Continue / Log in
      const cont = page.locator('button:has-text("Continue"), button:has-text("登录")').first();
      await cont.click();
      await sleep(3000);
      const pwd = page.locator('input[type="password"]').first();
      // 新版登录页：邮箱提交后密码框短暂 disabled，等待启用；必要时点 "Continue with password"
      for (let i = 0; i < 20; i++) {
        await sleep(1000);
        if (await pwd.isEnabled()) break;
        for (const label of ['Continue with password', 'Log in with password', '使用密码继续']) {
          const b = page.locator(`button:has-text("${label}")`).first();
          if (await b.isVisible().catch(() => false)) {await b.click().catch(() => {}); break}
        }
      }
      let pwdReady = false;
      for (let i = 0; i < 30; i++) {if (await pwd.isEnabled().catch(() => false)) {pwdReady = true; break} await sleep(1000)}
      if (!pwdReady) throw new Error('密码输入框始终未启用');
      await pwd.fill(PASSWORD);
      console.log('PASSWORD-FILLED');
      await pwd.press('Enter');
      console.log('SUBMITTED, waiting...');
      await sleep(15000);
      console.log('URL-NOW:', page.url());
    }
    await page.screenshot({path: 'C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/outputs/cf-login-state.png', fullPage: false});
    console.log('SCREENSHOT-SAVED');
    // 不关浏览器，保留进程供下一步？——脚本结束会关掉。改为：step1 结束即关闭，登录态存 profile
    await ctx.close();
  } else if (mode === 'probe') {
    await page.goto('https://dash.cloudflare.com/?to=/:account/turnstile', {waitUntil: 'domcontentloaded', timeout: 60000});
    await sleep(8000);
    console.log('URL:', page.url());
    const text = await page.evaluate(() => document.body.innerText.slice(0, 1500));
    console.log('TEXT:', text);
    await ctx.close();
  }
}
main().catch(e => {console.error('ERR:', e.message); process.exit(1)});
