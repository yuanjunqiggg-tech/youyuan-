const { chromium } = require('playwright-core');
(async () => {
  const b = await chromium.launch({ executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true });
  const c = await b.newContext({ viewport: { width: 1440, height: 900 } });
  const p = await c.newPage();
  const reqs=[]; p.on('response', r=>{ if(r.url().includes('/api/')) reqs.push(r.status()+' '+r.request().method()+' '+r.url().replace('https://toolbox.youyuanqi.dpdns.org','')); });
  await p.goto('https://toolbox.youyuanqi.dpdns.org/', {waitUntil:'domcontentloaded', timeout:60000});
  await p.waitForTimeout(5000);
  // remove gate
  await p.evaluate(()=>{const g=document.getElementById('yy-identity-gate'); if(g) g.remove();});
  await p.waitForTimeout(800);
  console.log('--- API responses ---\n'+reqs.join('\n'));
  // probe each nav item
  const navs = await p.evaluate(()=>Array.from(document.querySelectorAll('.yy-side-nav a, .yy-side-nav button, [data-tab]')).map(e=>({tag:e.tagName, cls:e.className, txt:e.innerText.trim(), tab:e.getAttribute('data-tab')})).filter(x=>x.txt));
  console.log(JSON.stringify(navs,null,1));
  await p.screenshot({path:'p2-guest-home.png', fullPage:true});
  await b.close();
})();
