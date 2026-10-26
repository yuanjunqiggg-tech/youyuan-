import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFile} from 'node:fs/promises';

// Regression for the registration dead-end players hit. The mailbox used to be
// sent through a relay with no timeout, and the toolbox waited on a bare fetch
// with no timeout either, so a stalled relay pinned the form on
// "正在连接游园云端…" with the submit button disabled. The request layer must
// give up on its own, say so, and keep the server-side task usable.
const source = await readFile(new URL('../upstream/frontend/js/youyuan_identity.js', import.meta.url), 'utf8');
// Match on a whitespace-tolerant pattern: the ES2019 pass reformatted declarations
// to `var yyPlatformUser = null`, so a literal `var yyPlatformUser=` lookup missed.
const startMatch = /var\s+yyPlatformUser\s*=/.exec(source);
const endMatch = /function\s+yyShowIdentity\s*\(/.exec(source);
const block = source.slice(
  startMatch ? startMatch.index : 0,
  endMatch ? endMatch.index : source.length
);
if (!/yyIdentityRequest\s*\(/.test(block)) throw new Error('identity request block extraction failed');

function fixture() {
  const calls = [];
  const context = {
    location: {hostname: 'auth.youyuanqi.dpdns.org'},
    JSON, Object, Error, String, Number, Math, Date,
    AbortController,
    setTimeout: (fn, ms) => setTimeout(fn, ms),
    clearTimeout: id => clearTimeout(id),
    yyRootFetch: (path, options) => new Promise((resolve, reject) => {
      calls.push({path, hasSignal: !!options.signal});
      options.signal.addEventListener('abort', () => reject(Object.assign(new Error('The operation was aborted'), {name: 'AbortError'})));
    }),
  };
  vm.createContext(context);
  vm.runInContext(block, context);
  context.YY_IDENTITY_TIMEOUT_MS = 40;
  return {context, calls};
}

test('identity request carries an abort signal so a stalled cloud cannot hold the form forever', () => {
  const f = fixture();
  const pending = f.context.yyIdentityRequest('register', {username: 'qa'}).then(value => ({value, thrown: null}), error => ({value: null, thrown: error}));

  return pending.then(outcome => {
    assert.equal(outcome.thrown, null);
    assert.equal(f.calls.length, 1);
    assert.equal(f.calls[0].path, '/api/identity/register');
    assert.equal(f.calls[0].hasSignal, true);
    assert.equal(outcome.value.ok, false);
    assert.equal(outcome.value.timeout, true);
    assert.match(outcome.value.error, /超时/);
    assert.match(outcome.value.error, /不会重复创建/);
  });
});

test('a network failure is reported instead of thrown, so the caller can re-enable the form', async () => {
  const f = fixture();
  f.context.yyRootFetch = async () => { throw Object.assign(new Error('Failed to fetch'), {name: 'TypeError'}); };
  const answer = await f.context.yyIdentityRequest('login', {username: 'qa'});
  assert.equal(answer.ok, false);
  assert.equal(answer.timeout, undefined);
  assert.match(answer.error, /Failed to fetch/);
});

test('a normal answer is still returned untouched', async () => {
  const f = fixture();
  f.context.yyRootFetch = async () => ({json: async () => ({ok: true, user: {username: 'qa'}})});
  const answer = await f.context.yyIdentityRequest('me');
  assert.deepEqual(answer, {ok: true, user: {username: 'qa'}});
});
