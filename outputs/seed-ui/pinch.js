'use strict';
const { chromium } = require('playwright');
(async()=>{
 const b=await chromium.launch({executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
 const ctx=await b.newContext({viewport:{width:390,height:844},hasTouch:true,isMobile:true});
 const p=await ctx.newPage();
 await p.goto('http://127.0.0.1:8787/index.html',{waitUntil:'domcontentloaded'});
 await p.addStyleTag({content:'#yy-identity-gate,#yy-intro-overlay,video{display:none!important}'});
 await p.waitForTimeout(900);
 await p.evaluate(()=>window.yyOpenModule('seedmap2'));
 await p.waitForTimeout(2200);
 const box=await p.locator('#sm2-map').boundingBox();
 const cx=box.x+box.width/2, cy=box.y+box.height/2;
 const before=await p.evaluate(()=>document.querySelector('#sm2-readout').textContent.replace(/\s+/g,' '));
 const cdp=await ctx.newCDPSession(p);
 await cdp.send('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{x:cx-40,y:cy,id:1},{x:cx+40,y:cy,id:2}]});
 for(let i=1;i<=6;i++){
   await cdp.send('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[{x:cx-40-i*14,y:cy,id:1},{x:cx+40+i*14,y:cy,id:2}]});
   await p.waitForTimeout(30);
 }
 await cdp.send('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]});
 await p.waitForTimeout(600);
 const after=await p.evaluate(()=>document.querySelector('#sm2-readout').textContent.replace(/\s+/g,' '));
 console.log('双指缩放前', before);
 console.log('双指缩放后', after);
 await p.screenshot({path:'phone-390-pinch.png'});
 await b.close();
})();
