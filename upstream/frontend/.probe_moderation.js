// 一次性探针：用最小 DOM 桩按真实加载顺序（console 的假 yyQQRenderAll →
// youyuan_qq_guard.js → youyuan_qq_moderation.js）执行，验证：
//   ① mount 后能包装 yyQQRenderAll 并往 #yyq-root .yyq-fold-grid 注入 #yyq-moderation
//      （注入块里含规则表格和添加表单）
//   ② GET mock 返回 2 条规则后，#yym-body 渲染出 2 行
//   ③ 添加：读全表 → 改 → POST 整表写回（3 条），刷新后 3 行
//   ④ 删除：POST 整表写回（2 条），刷新后 2 行
//   ⑤ 校验：匹配内容为空 / 正则写错时不发请求，红字提示
//   ⑥ console 整体重绘（清空网格）后能从状态重新注入，不打回原形
// 用完即删，不进部署。
'use strict';
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const guardCode = fs.readFileSync(path.resolve(__dirname, 'js/youyuan_qq_guard.js'), 'utf8');
const modCode = fs.readFileSync(path.resolve(__dirname, 'js/youyuan_qq_moderation.js'), 'utf8');

// ---- 假数据（有状态：POST 整表替换后 GET 能看到变化，模拟服务端）----
const store = {
  '/api/qq/moderation': {
    ok: true,
    rules: [
      { pattern: '傻逼', kind: 'substring', action: 'block', reason: '骂人', enabled: true, source: 'manual', added_at: '2026-10-20T10:00:00Z' },
      { pattern: '你好[啊阿]+', kind: 'regex', action: 'recall', reason: '', enabled: false, source: 'natural', added_at: '2026-10-21T09:00:00Z' }
    ],
    taboos: ['不准 玩家Steve 拆家'],
    disk_error: '',
    note: '探针note'
  }
};
const posts = [];
function fakeA(method, url, body) {
  if (url === '/api/qq/moderation') {
    if (method === 'GET') return Promise.resolve(JSON.parse(JSON.stringify(store['/api/qq/moderation'])));
    if (method === 'POST') {
      posts.push(JSON.parse(JSON.stringify(body)));
      store['/api/qq/moderation'].rules = body.rules.map(function (r) {
        return { pattern: r.pattern, kind: r.kind, action: r.action, reason: r.reason, enabled: r.enabled, source: 'manual', added_at: '2026-10-22T08:00:00Z' };
      });
      return Promise.resolve({ ok: true });
    }
  }
  if (url === '/api/qq/guard') return Promise.resolve({ ok: true, rules: [], extra: [], note: '' });
  return Promise.resolve({ ok: true });
}

// ---- DOM 桩 ----
function makeEl(id) {
  return {
    id, innerHTML: '', value: '', textContent: '', checked: false, disabled: false,
    dataset: {}, style: {}, open: false,
    insertAdjacentHTML(pos, html) { this.innerHTML += html; },
    querySelector: () => null, querySelectorAll: () => [],
    addEventListener: () => {}, setAttribute: () => {}, focus: () => {}, closest: () => null
  };
}
const els = {};
const grid = makeEl('grid');
grid.insertAdjacentHTML = function (pos, html) {
  this.innerHTML += html;
  console.log('[grid] insertAdjacentHTML(' + pos + ') 收到 ' + html.length + ' 字符');
};
const root = { querySelector: (sel) => (sel === '#yyq-root .yyq-fold-grid' ? grid : null) };
const sumGuard = makeEl('sum-guard');
const sumMine = makeEl('sum-moderation');

const document = {
  readyState: 'complete',
  getElementById: (id) => {
    if (id === 'yyq-root') return root;
    // guard/moderation 用「网格里有没有这个 id」判断是否已注入（字符串累加器不感知嵌套）
    if (id === 'yyq-guard') return grid.innerHTML.indexOf('yyq-guard') >= 0 ? {} : null;
    if (id === 'yyq-moderation') return grid.innerHTML.indexOf('yyq-moderation') >= 0 ? {} : null;
    if (!els[id]) els[id] = makeEl(id);
    return els[id];
  },
  querySelector: (sel) => {
    if (sel === '#yyq-root .yyq-fold-grid') return grid;
    if (sel === '#yyq-guard .yyq-fold-sum') return sumGuard;
    if (sel === '#yyq-moderation .yyq-fold-sum') return sumMine;
    return null;
  },
  addEventListener: () => {}
};

const sandbox = {
  console, document, setTimeout, clearTimeout,
  A: fakeA,
  yyQQE: (v) => String(v == null ? '' : v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'),
  yyQQIcon: (n) => '<svg class="yy-icon" data-i="' + n + '"></svg>'
};
sandbox.window = sandbox;
sandbox.globalThis = sandbox;

// 假的 console 渲染函数：每次调用清空网格（模拟 yyQQRenderAll 整体重建 #yyq-root）
let renderCalls = 0;
sandbox.yyQQRenderAll = function () { renderCalls++; grid.innerHTML = ''; };

function count(s, sub) { return s.split(sub).length - 1; }
const rowsOf = () => count(els['yym-body'].innerHTML, 'yyMToggle(');
const el = (id) => document.getElementById(id);

const ctx = vm.createContext(sandbox);
for (const [name, code] of [['guard', guardCode], ['moderation', modCode]]) {
  try { vm.runInContext(code, ctx, { filename: name + '.js' }); }
  catch (e) { console.log('[探针] 执行 ' + name + '.js 抛错：' + e.message); process.exit(1); }
}

setTimeout(() => {
  const wrapped = sandbox.yyQQRenderAll.__yyMWrapped === 1;
  const injected = grid.innerHTML.indexOf('yyq-moderation') >= 0;
  const bodyHtml = els['yym-body'] ? els['yym-body'].innerHTML : '';
  const hasTable = bodyHtml.indexOf('<table') >= 0;
  const hasForm = bodyHtml.indexOf('yym-new') >= 0 && bodyHtml.indexOf('yym-action') >= 0;
  const rows = rowsOf();
  console.log('[探针] 包装成功（moderation）：' + (wrapped ? '是' : '否') + '（renderCalls=' + renderCalls + '）');
  console.log('[探针] 注入 #yyq-moderation：' + (injected ? '是' : '否'));
  console.log('[探针] 规则表格：' + (hasTable ? '是' : '否') + '；添加表单：' + (hasForm ? '是' : '否'));
  console.log('[探针] GET 2 条规则 → 渲染行数：' + rows);
  console.log('[探针] summary：' + JSON.stringify(sumMine.textContent));
  console.log('[探针] 长期禁令条目在正文里：' + (bodyHtml.indexOf('不准 玩家Steve 拆家') >= 0 ? '是' : '否'));

  const ok1 = wrapped && injected && hasTable && hasForm && rows === 2 &&
    sumMine.textContent === '生效 1 条 · AI 长期禁令 1 条' &&
    bodyHtml.indexOf('不准 玩家Steve 拆家') >= 0;

  // ③ 添加
  el('yym-new').value = '探针词';
  el('yym-kind').value = 'substring';
  el('yym-action').value = 'recall';
  el('yym-reason').value = '探针理由';
  sandbox.yyMAdd({ disabled: false });
  setTimeout(() => {
    const last = posts[posts.length - 1];
    const okAdd = posts.length === 1 && last && last.rules.length === 3 &&
      last.rules.some((r) => r.pattern === '探针词' && r.action === 'recall' && r.enabled === true) &&
      rowsOf() === 3;
    console.log('[探针] 添加 → POST 整表 ' + (last ? last.rules.length : 0) + ' 条（含探针词）：' + (okAdd ? '是' : '否') + '；刷新后行数：' + rowsOf());

    // ④ 删除第 0 条
    sandbox.yyMDel(0, { disabled: false });
    setTimeout(() => {
      const last2 = posts[posts.length - 1];
      const okDel = posts.length === 2 && last2 && last2.rules.length === 2 && rowsOf() === 2;
      console.log('[探针] 删除 → POST 整表 ' + (last2 ? last2.rules.length : 0) + ' 条：' + (okDel ? '是' : '否') + '；刷新后行数：' + rowsOf());

      // ⑤ 校验：空内容 / 坏正则都不发请求
      const before = posts.length;
      el('yym-new').value = '';
      sandbox.yyMAdd({ disabled: false });
      const emptyBlocked = posts.length === before && els['yym-body'].innerHTML.indexOf('先填写匹配内容') >= 0;
      el('yym-new').value = '([';
      el('yym-kind').value = 'regex';
      sandbox.yyMAdd({ disabled: false });
      const regexBlocked = posts.length === before && els['yym-body'].innerHTML.indexOf('正则写错了') >= 0;
      console.log('[探针] 空内容被拦：' + (emptyBlocked ? '是' : '否') + '；坏正则被拦：' + (regexBlocked ? '是' : '否'));

      // ⑥ console 整体重绘后重新注入（guard 的包装链也仍在：两个块都要回来）
      grid.innerHTML = '';
      sandbox.yyQQRenderAll();
      const reInjected = grid.innerHTML.indexOf('yyq-moderation') >= 0 && rowsOf() === 2;
      const guardBack = grid.innerHTML.indexOf('yyq-guard') >= 0;
      console.log('[探针] 重绘后重新注入且数据还在：' + (reInjected ? '是' : '否') + '；guard 块也回来：' + (guardBack ? '是' : '否'));

      const ok = ok1 && okAdd && okDel && emptyBlocked && regexBlocked && reInjected && guardBack;
      console.log('[探针] 总结：' + (ok ? 'PASS' : 'FAIL'));
      process.exit(ok ? 0 : 2);
    }, 120);
  }, 120);
}, 150);
