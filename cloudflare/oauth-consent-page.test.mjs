import assert from 'node:assert/strict';import test from 'node:test';
import {consentPage} from './oauth-consent-page.mjs';
test('form CSP permits only the validated OAuth callback in addition to self',()=>{
 for(const redirect of ['https://chatgpt.com/connector_platform_oauth_redirect','https://chatgpt.com/connector/oauth/abc_123','http://127.0.0.1:65407/callback']){
  const p=consentPage({redirect,name:'ChatGPT',device:'linux-server',scope:'control'},'fixture-tx','/oauth/authorize?fixture');
  assert.ok(p.headers.get('Content-Security-Policy').includes("form-action 'self' "+redirect+';'));
 }
 for(const redirect of ['https://evil.test/callback','https://chatgpt.com.evil.test/connector_platform_oauth_redirect','https://chatgpt.com/other','http://127.0.0.1:65407/other','https://chatgpt.com/connector_platform_oauth_redirect?evil=1',undefined]){
  const p=consentPage({redirect,name:'ChatGPT',device:'linux-server'},'fixture-tx','/oauth/authorize?fixture');
  assert.ok(p.headers.get('Content-Security-Policy').includes("form-action 'self';"));
 }
});
