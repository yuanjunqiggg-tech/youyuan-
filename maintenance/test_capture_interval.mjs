import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const source = fs.readFileSync(new URL('../upstream/frontend/js/youyuan_global_capture.js', import.meta.url), 'utf8');
const start = source.indexOf('async function() {', source.indexOf('getElementById("yy-gc-save").onclick'));
const end = source.indexOf('\n  };', start);
assert(start > 0 && end > start);
assert(source.includes('\\u6BCF\\u6B21\\u5207\\u6362 / \\u6BEB\\u79D2</label>'));
assert(source.includes('Number(c.interval_seconds * 1000)'));

for (const ms of [0.1, 0.001, 15]) {
  let saved;
  const context = {
    document: {getElementById: id => ({value: id === 'yy-gc-interval' ? String(ms) : '0', checked: false, checkValidity: () => true})},
    chosen: () => [], yyCaptureFleetStepsRead: () => [], yyGlobalCaptureDraft: null,
    yyGlobalCaptureSave: async config => {saved = JSON.parse(JSON.stringify(config)); return {ok: true, config: saved};},
    T: text => assert(!String(text).includes('请输入')),
  };
  const save = vm.runInNewContext('(' + source.slice(start, end + 4) + ')', context);
  const button = {};
  await save.call(button);
  assert.equal(saved.interval_seconds, ms / 1000);
  assert.equal(saved.interval_seconds * 1000, ms);
  assert.equal(button.disabled, false);
}
console.log('0.1ms, 0.001ms, 15ms: real save handler retains units through JSON');

let attempted = false, error;
const invalid = vm.runInNewContext('(' + source.slice(start, end + 4) + ')', {
  document: {getElementById: () => ({value: '0', checkValidity: () => false})},
  yyGlobalCaptureSave: () => {attempted = true;},
  T: message => {error = message;},
});
await invalid.call({});
assert.equal(attempted, false);
assert.match(error, /0\.001/);
