import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

// The plugin detail panel used to put five equally-weighted buttons in one row:
// 重新加载 / 停用 / 备份 / 上传 / 删除. Deleting a plugin therefore looked as
// routine as reloading it, which is the "太低级、太杂太乱" the user called out.
// These tests lock the three-group layout in place.
const source = readFileSync(new URL('../upstream/frontend/js/youyuan_plugin_studio.js', import.meta.url), 'utf8');
const block = source.slice(source.indexOf('function runStateLine'), source.indexOf('function detail(p) {'));

function fixture() {
  const context = {
    esc: value => String(value == null ? '' : value),
    button: (action, text, args, cls) => '<b data-a="' + action + '"' + (cls ? ' class="' + cls + '"' : '') + '>' + text + '</b>',
  };
  vm.createContext(context);
  vm.runInContext(block, context);
  return context;
}

test('maintenance actions are split into run / data / danger groups', () => {
  const html = fixture().maintenanceGroups({id: 'demo', raw: {}});
  assert.match(html, /<h4>运行<\/h4>/);
  assert.match(html, /<h4>数据留存<\/h4>/);
  assert.match(html, /<h4>危险操作<\/h4>/);
  assert.match(html, /yps-danger-zone/);
});

test('deleting a plugin is never presented next to reloading it', () => {
  const html = fixture().maintenanceGroups({id: 'demo', raw: {}});
  const dangerAt = html.indexOf('yps-danger-zone');
  assert.ok(dangerAt > 0, 'danger zone must exist');
  assert.ok(html.indexOf('data-a="delete"') > dangerAt, 'delete must live inside the danger zone');
  assert.ok(html.indexOf('data-a="reload"') < dangerAt, 'reload must stay in the run group');
  // The old flat row put all five in one container; that must be gone.
  assert.doesNotMatch(html, /重新加载[^]{0,400}删除插件[^]{0,40}<\/div><\/div>$/);
});

test('the danger group states that deletion cannot be undone and points at backup', () => {
  const html = fixture().maintenanceGroups({id: 'demo', raw: {}});
  const danger = html.slice(html.indexOf('yps-danger-zone'));
  assert.match(danger, /无法撤销/);
  assert.match(danger, /备份/);
});

test('run state is read from real fields instead of being inferred from button labels', () => {
  const context = fixture();
  assert.match(context.runStateLine({raw: {running: true}}), /正在运行/);
  assert.match(context.runStateLine({raw: {running: true}}), /data-tone="on"/);
  assert.match(context.runStateLine({raw: {disabled: true}}), /已停用/);
  assert.match(context.runStateLine({raw: {disabled: true}}), /不会自动启动/);
  assert.match(context.runStateLine({raw: {}, state: 'error'}), /上次启动失败/);
  assert.match(context.runStateLine({raw: {}, state: 'error'}), /data-tone="error"/);
});

test('a stopped plugin is reported as stopped, not as a healthy one', () => {
  const line = fixture().runStateLine({raw: {running: false, disabled: false}});
  assert.match(line, /未在运行/);
  assert.doesNotMatch(line, /正在运行/);
});

test('stopping autostart offers the reverse action, and only then', () => {
  const context = fixture();
  const enabled = context.maintenanceGroups({id: 'demo', raw: {disabled: false}});
  assert.match(enabled, /data-a="disable"/);
  assert.doesNotMatch(enabled, /data-a="enable"/);
  const disabled = context.maintenanceGroups({id: 'demo', raw: {disabled: true}});
  assert.match(disabled, /data-a="enable"/);
  assert.doesNotMatch(disabled, /data-a="disable"/);
});
