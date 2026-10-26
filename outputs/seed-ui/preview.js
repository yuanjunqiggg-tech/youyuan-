'use strict';
const { chromium } = require('playwright');
(async()=>{
 await fetch('http://127.0.0.1:8787/__slow?ms=1500');
 const b=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
 const ctx=await b.newContext({viewport:{width:1200,height:860}});
 const p=await ctx.newPage();
 await p.goto('http://127.0.0.1:8787/index.html',{waitUntil:'domcontentloaded'});
 await p.addStyleTag({content:'#yy-identity-gate,#yy-intro-overlay,video{display:none!important}'});
 await p.waitForTimeout(900);
 await p.evaluate(()=>window.yyOpenModule('seedmap2'));
 await p.waitForTimeout(2000);
 console.log('粗图阶段', await p.evaluate(()=>({
   preview: document.querySelectorAll('.sm2-preview').length,
   previewLoaded: !!document.querySelector('.sm2-preview') && document.querySelector('.sm2-preview').naturalWidth,
   tiles: document.querySelectorAll('img.sm2-tile').length,
   loading: !document.querySelector('#sm2-loading').hidden
 })));
 await p.screenshot({path:'preview-coarse.png'});
 await p.waitForTimeout(5000);
 console.log('细图阶段', await p.evaluate(()=>({
   preview: document.querySelectorAll('.sm2-preview').length,
   tiles: Array.from(document.querySelectorAll('img.sm2-tile')).filter(i=>i.naturalWidth).length,
   loading: !document.querySelector('#sm2-loading').hidden
 })));
 await p.screenshot({path:'preview-fine.png'});
 await fetch('http://127.0.0.1:8787/__slow?ms=0');
 await b.close();
})();
