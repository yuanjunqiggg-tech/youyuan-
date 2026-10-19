import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
const source=readFileSync(new URL('../upstream/frontend/js/youyuan_workspace.js',import.meta.url),'utf8');
function setup(){
 const calls=[],notices=[],fields=new Map(['yy-editor-status','yy-code-editor','yy-file-select'].map(id=>[id,{textContent:'',innerHTML:'',value:''}]));let visible=true;
 const c=vm.createContext({yyWorkspaceActive:{id:'one',language:'lua'},yyWorkspaceOpenEpoch:1,yyEditorDirty:false,yyEditorPath:'',yyFileBaseHash:'',yyWorkspaceEditor:null,yyEditorTimer:null,yyActiveAccount:'main',clearTimeout(){},T:msg=>notices.push(msg),document:{getElementById:id=>id==='sub-page'?{classList:{contains:()=>visible}}:fields.get(id)},A:(...args)=>new Promise(resolve=>calls.push({args,resolve})),YYEditor:{create:(host,content,path)=>({path,state:{doc:{toString:()=>content}},destroy(){this.destroyed=true}})}});c.window=c;
 vm.runInContext(source.slice(source.indexOf('function yyEditorAccountScope('),source.indexOf('async function yyNewProjectFile')),c);
 return {c,calls,fields,notices,hide(){visible=false}};
}
test('rapid file selection preserves the newest selected file when replies arrive out of order',async()=>{
 const p=setup(),old=p.c.yyOpenProjectFile('old.lua'),newest=p.c.yyOpenProjectFile('new.lua');assert.equal(p.calls.length,2);assert.match(p.fields.get('yy-editor-status').textContent,/new.lua/);
 p.calls[1].resolve({ok:true,content:'new',hash:'new-hash'});await newest;p.calls[0].resolve({ok:true,content:'old',hash:'old-hash'});await old;
 assert.equal(p.c.yyEditorPath,'new.lua');assert.equal(p.c.yyWorkspaceEditor.state.doc.toString(),'new');assert.equal(p.c.yyFileBaseHash,'new-hash');
});
test('typing into the previous file while the next file loads is saved before replacing that editor',async()=>{
 const p=setup(),editor={state:{doc:{toString:()=>'edited during loading'}},destroy(){this.destroyed=true}};p.c.yyWorkspaceEditor=editor;p.c.yyEditorPath='old.lua';p.c.yyEditorScope='main';
 const pending=p.c.yyOpenProjectFile('new.lua');p.c.yyEditorDirty=true;p.calls[0].resolve({ok:true,content:'new file',hash:'new-hash'});await new Promise(r=>setImmediate(r));
 assert.equal(p.calls.length,2);assert.equal(p.calls[1].args[0],'POST');assert.equal(p.calls[1].args[2].path,'old.lua');assert.equal(p.calls[1].args[2].content,'edited during loading');assert.equal(editor.destroyed,undefined);
 p.calls[1].resolve({ok:true,hash:'old-saved-hash'});await pending;assert.equal(editor.destroyed,true);assert.equal(p.c.yyEditorPath,'new.lua');assert.equal(p.c.yyWorkspaceEditor.state.doc.toString(),'new file');
});
test('a failed save or more typing during the save keeps the original editor and draft',async()=>{
 for(const failure of [true,false]){const p=setup();let contents='first edit';const editor={state:{doc:{toString:()=>contents}},destroy(){throw Error('must retain draft')}};p.c.yyWorkspaceEditor=editor;p.c.yyEditorPath='old.lua';p.c.yyEditorScope='main';
 const pending=p.c.yyOpenProjectFile('new.lua');p.c.yyEditorDirty=true;p.calls[0].resolve({ok:true,content:'new file'});await new Promise(r=>setImmediate(r));if(!failure)contents='newer edit';p.calls[1].resolve(failure?{ok:false,error:'save conflict'}:{ok:true,hash:'saved'});await pending;assert.equal(p.c.yyWorkspaceEditor,editor);assert.equal(p.c.yyEditorPath,'old.lua');assert.equal(p.c.yyEditorDirty,true)}
});
test('saving an old scope draft cannot submit it under another account or login session',async()=>{
 const p=setup();p.c.yyWorkspaceEditor={state:{doc:{toString:()=>'private draft'}}};p.c.yyEditorDirty=true;p.c.yyEditorScope='original-session';p.c.yyReadScope=()=> 'new-session';
 assert.equal(await p.c.yySaveProjectFile(),false);assert.equal(p.calls.length,0);assert.equal(p.c.yyEditorDirty,true);assert.match(p.notices[0],/草稿仍保留/);
});
test('a save reply after editor teardown cannot dereference or repaint the destroyed editor',async()=>{
 const p=setup();p.c.yyWorkspaceEditor={state:{doc:{toString:()=>'draft'}}};p.c.yyEditorScope='main';p.c.yyEditorPath='old.lua';p.c.yyEditorDirty=true;const pending=p.c.yySaveProjectFile();p.c.yyWorkspaceEditor=null;p.fields.get('yy-editor-status').textContent='new page';p.calls[0].resolve({ok:true,hash:'saved'});assert.equal(await pending,true);assert.equal(p.fields.get('yy-editor-status').textContent,'new page');
});
test('opening a different workspace preserves an unsaved old draft when its save is refused',async()=>{
 const p=setup();vm.runInContext(source.slice(source.indexOf('async function yyOpenWorkspace('),source.indexOf('// Durable conversation')),p.c);
 p.c.yyWorkspaceEditor={state:{doc:{toString:()=>'unsaved old workspace'}}};p.c.yyEditorScope='main';p.c.yyEditorPath='old.lua';p.c.yyEditorDirty=true;const pending=p.c.yyOpenWorkspace('two','code');assert.equal(p.calls.length,1);assert.equal(p.calls[0].args[0],'POST');assert.equal(p.calls[0].args[2].id,'one');p.calls[0].resolve({ok:false,error:'conflict'});await pending;assert.equal(p.calls.length,1);assert.equal(p.c.yyWorkspaceActive.id,'one');assert.equal(p.c.yyEditorDirty,true);
});
test('a workspace reply from an old authentication scope cannot mount or start background model reads',async()=>{
 const p=setup();vm.runInContext(source.slice(source.indexOf('async function yyOpenWorkspace('),source.indexOf('// Durable conversation')),p.c);
 p.c.yyReadScope=()=> 'old-session';const pending=p.c.yyOpenWorkspace('two','agent');p.c.yyReadScope=()=> 'new-session';p.calls[0].resolve({ok:true,workspace:{id:'two'}});await pending;assert.equal(p.calls.length,1);assert.equal(p.c.yyWorkspaceActive.id,'one');
});
test('file replies and errors from a previous account, workspace or closed page cannot change the editor',async()=>{
 for(const change of ['account','workspace','closed']){const p=setup(),pending=p.c.yyOpenProjectFile('old.lua');if(change==='account')p.c.yyActiveAccount='other';if(change==='workspace')p.c.yyWorkspaceActive={id:'other'};if(change==='closed')p.hide();p.calls[0].resolve({ok:true,content:'old'});await pending;assert.equal(p.c.yyEditorPath,'');assert.equal(p.c.yyWorkspaceEditor,null);assert.equal(p.notices.length,0)}
});
test('a failed file open keeps the existing unsent file contents and gives an immediate error state',async()=>{
 const p=setup(),editor={state:{doc:{toString:()=>'previous contents'}},destroy(){throw Error('must retain previous editor')}};p.c.yyWorkspaceEditor=editor;p.c.yyEditorPath='previous.lua';const pending=p.c.yyOpenProjectFile('missing.lua');p.calls[0].resolve({ok:false,error:'file missing'});await pending;assert.equal(p.c.yyWorkspaceEditor,editor);assert.equal(p.c.yyEditorPath,'previous.lua');assert.match(p.fields.get('yy-editor-status').textContent,/打开失败/);assert.deepEqual(p.notices,['file missing']);
});
