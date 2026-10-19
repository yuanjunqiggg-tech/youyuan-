import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {webcrypto,createHash} from 'node:crypto';
import {readFile} from 'node:fs/promises';

async function setup(response){
 const links=[],toasts=[];
 const context=vm.createContext({Blob,URL,URLSearchParams,Uint8Array,crypto:webcrypto,setTimeout:()=>0,location:{href:'https://toolbox.example/'},window:{},Rplg:async()=>{},plgExportSAF:async()=>{},T:m=>toasts.push(m),yyRootFetch:async()=>response,document:{addEventListener(){},body:{append:a=>links.push(a)},createElement:()=>({click(){this.clicked=true},remove(){}})}});
 vm.runInContext(await readFile(new URL('../upstream/frontend/js/youyuan_export_center.js',import.meta.url),'utf8'),context);
 return {context,links,toasts};
}
test('cross-origin backup ZIP downloads when disposition is hidden but metadata and SHA256 agree',async()=>{
 const data=Buffer.from('PK\x03\x04fixture zip bytes'),sha=createHash('sha256').update(data).digest('hex');
 const {context,links}=await setup(new Response(data,{headers:{'Content-Type':'application/zip'}}));
 await context.yyDownloadExport({download_url:'/api/youyuan/exports/download?id=test',name:'plugin.zip',bytes:data.length,sha256:sha},'main');
 assert.equal(links.length,1);assert.equal(links[0].clicked,true);assert.equal(links[0].download,'plugin.zip');
});
test('damaged ZIP fails checksum without downloading',async()=>{
 const data=Buffer.from('PK\x03\x04damaged');const {context,links}=await setup(new Response(data,{headers:{'Content-Type':'application/zip'}}));
 await assert.rejects(()=>context.yyDownloadExport({download_url:'/api/download',name:'p.zip',bytes:data.length,sha256:'a'.repeat(64)}),/校验失败/);assert.equal(links.length,0);
});
test('binary error never exposes file bytes in the toast',async()=>{
 const {context}=await setup(new Response(Buffer.from('PK\x03\x04private-file-content'),{status:400,headers:{'Content-Type':'application/zip'}}));
 await assert.rejects(()=>context.yyDownloadExport({download_url:'/api/download',name:'p.zip'}),e=>e.message.includes('HTTP 400')&&!e.message.includes('private-file-content'));
});
