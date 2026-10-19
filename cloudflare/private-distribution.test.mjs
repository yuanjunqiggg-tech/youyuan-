import test from 'node:test';
import assert from 'node:assert/strict';
import {isPrivateDistributionPath as blocked} from './private-distribution.mjs';
test('public source paths are blocked including case and encoded names',()=>{for(const p of ['/source.zip','/SOURCE.ZIP','/%73ource.zip','/source-v0.10.29.zip','/plugin-sdk.zip','/skill-library.zip','/kernel.wasm','/wasm_exec.js','/.git/config','/backend/main.go','/js/app.js.map'])assert.equal(blocked(p),true,p)});
test('UI, APK updates and authenticated user project routes remain usable',()=>{for(const p of ['/','/js/youyuan_accounts.js','/updates/android/latest.apk','/api/youyuan/projects/file/main.go','/api/plugin/export','/mcp/oauth/linux-server'])assert.equal(blocked(p),false,p)});

test('private handoff archive names cannot accidentally become public assets',()=>{
 for(const p of ['/游园源码.zip','/youyuan-source.zip','/插件开发资料.7z','/youyuan-SDK.tar.gz','/交付/源代码备份.zip'])assert.equal(blocked(p),true,p);
 for(const p of ['/api/library/download?id=file','/api/youyuan/plugin-outputs/download'])assert.equal(blocked(p),false,p);
});
