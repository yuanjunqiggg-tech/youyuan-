'use strict';
const { chromium } = require('playwright');
(async()=>{
 const b=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
 const ctx=await b.newContext({viewport:{width:1200,height:860}});
 const p=await ctx.newPage();
 await p.goto('http://127.0.0.1:8787/index.html',{waitUntil:'domcontentloaded'});
 await p.addStyleTag({content:'#yy-identity-gate,#yy-intro-overlay,video{display:none!important}'});
 await p.waitForTimeout(900);
 await p.evaluate(()=>window.yyOpenModule('seedmap2'));
 await p.waitForTimeout(2200);
 console.log(await p.evaluate(()=>{
   const icons=[...document.querySelectorAll('#sm2-icons .sm2-icon')];
   const first=icons.find(e=>e.style.display!=='none');
   const img=first&&first.querySelector('img');
   const badge=first&&first.querySelector('.sm2-badge');
   return {
     total: icons.length,
     firstSrc: img&&img.getAttribute('src'),
     imgBox: img?{w:Math.round(img.getBoundingClientRect().width),h:Math.round(img.getBoundingClientRect().height),loaded:img.complete&&img.naturalWidth>0}:null,
     badgeBox: badge?{w:Math.round(badge.getBoundingClientRect().width),h:Math.round(badge.getBoundingClientRect().height)}:null,
     fallback: icons.filter(e=>e.querySelector('svg')).length
   };
 }));
 await p.screenshot({path:'desktop-1440-icons.png'});
 await b.close();
})();
