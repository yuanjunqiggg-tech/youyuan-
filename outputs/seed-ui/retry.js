'use strict';
const { chromium } = require('playwright');
(async()=>{
 await fetch('http://127.0.0.1:8787/__tilefail?on=1');
 const b=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
 const ctx=await b.newContext({viewport:{width:1200,height:860}});
 const p=await ctx.newPage();
 await p.goto('http://127.0.0.1:8787/index.html',{waitUntil:'domcontentloaded'});
 await p.addStyleTag({content:'#yy-identity-gate,#yy-intro-overlay,video{display:none!important}'});
 await p.waitForTimeout(900);
 await p.evaluate(()=>window.yyOpenModule('seedmap2'));
 await p.waitForTimeout(2000);
 console.log('故障时', await p.evaluate(()=>({
   empty: !document.querySelector('#sm2-empty').hidden,
   failTiles: document.querySelectorAll('.sm2-tile.fail').length,
   list: document.querySelectorAll('#sm2-list .sm2-row').length
 })));
 await fetch('http://127.0.0.1:8787/__tilefail?on=0');
 await p.evaluate(()=>document.querySelector('#sm2-empty-btn').click());
 await p.waitForTimeout(2200);
 console.log('重试后', await p.evaluate(()=>({
   empty: !document.querySelector('#sm2-empty').hidden,
   okTiles: Array.from(document.querySelectorAll('.sm2-tile')).filter(i=>i.naturalWidth>0).length,
   failTiles: document.querySelectorAll('.sm2-tile.fail').length
 })));
 await p.screenshot({path:'desktop-1440-retry.png'});
 await b.close();
})();
