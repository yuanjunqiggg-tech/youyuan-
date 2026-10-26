'use strict';
const { chromium } = require('playwright');
(async()=>{
 const b=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
 const ctx=await b.newContext({viewport:{width:1200,height:860}});
 const p=await ctx.newPage();
 let block=true;
 await p.route('**/api/seed/v2/manifest', route=>{
   if(block) return route.fulfill({status:503, contentType:'application/json', body:JSON.stringify({ok:false,error:'生物群系表未装载'})});
   return route.continue();
 });
 await p.goto('http://127.0.0.1:8787/index.html',{waitUntil:'domcontentloaded'});
 await p.addStyleTag({content:'#yy-identity-gate,#yy-intro-overlay,video{display:none!important}'});
 await p.waitForTimeout(900);
 await p.evaluate(()=>window.yyOpenModule('seedmap2'));
 await p.waitForTimeout(1500);
 console.log('清单失败', await p.evaluate(()=>({
   empty: !document.querySelector('#sm2-empty').hidden,
   title: document.querySelector('#sm2-empty-title').textContent,
   desc: document.querySelector('#sm2-empty-desc').textContent.slice(0,60),
   btn: document.querySelector('#sm2-empty-btn').textContent,
   white: document.querySelector('.sm2').innerText.length < 20
 })));
 await p.screenshot({path:'desktop-1440-manifest-fail.png'});
 block=false;
 await p.evaluate(()=>document.querySelector('#sm2-empty-btn').click());
 await p.waitForTimeout(2500);
 console.log('重试后', await p.evaluate(()=>({
   empty: !document.querySelector('#sm2-empty').hidden,
   tiles: Array.from(document.querySelectorAll('.sm2-tile')).filter(i=>i.naturalWidth>0).length,
   chips: document.querySelectorAll('.sm2-chip').length,
   versions: document.querySelectorAll('#sm2-version option').length
 })));
 await b.close();
})();
