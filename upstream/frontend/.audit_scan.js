// 一次性审查脚本：扫描死入口 / 移动端隐藏 / 空状态文案。用完即删。
const fs = require('fs');
const path = require('path');

function walk(d, out = []) {
  for (const f of fs.readdirSync(d, { withFileTypes: true })) {
    const p = path.join(d, f.name);
    if (f.isDirectory()) {
      if (f.name === 'vendor' || f.name === '.playwright-cli' || f.name === 'assets' || f.name === 'images' || f.name === 'sounds' || f.name === 'licenses' || f.name === 'blocks') continue;
      walk(p, out);
    } else if (/\.(js|html)$/.test(f.name) && !/\.esb$/.test(f.name)) out.push(p);
  }
  return out;
}

const files = walk('.').filter(f => !f.includes('.audit_scan'));
const handlers = {}; // name -> [loc]
const re = /on(?:click|change|input|submit|keydown|keyup|blur|focus|dblclick|touchstart)\s*=\s*(["'\\`])([\s\S]{0,160}?)\1/g;

for (const f of files) {
  const txt = fs.readFileSync(f, 'utf8');
  let m;
  while ((m = re.exec(txt))) {
    const body = m[2];
    const fm = body.match(/^\s*([A-Za-z_$][\w$]*)\s*\(/);
    if (!fm) continue;
    const name = fm[1];
    const line = txt.slice(0, m.index).split('\n').length;
    (handlers[name] = handlers[name] || []).push(f.replace(/\\/g, '/') + ':' + line);
  }
}

// 收集定义： function foo( / window.foo = / foo = function / foo: function
const defined = new Set();
for (const f of files) {
  const txt = fs.readFileSync(f, 'utf8');
  let m;
  const d1 = /function\s+([A-Za-z_$][\w$]*)\s*\(/g;
  while ((m = d1.exec(txt))) defined.add(m[1]);
  const d2 = /(?:window\.|globalThis\.)?([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?(?:function|\()/g;
  while ((m = d2.exec(txt))) defined.add(m[1]);
  const d3 = /(?:window|globalThis)\.([A-Za-z_$][\w$]*)\s*=/g;
  while ((m = d3.exec(txt))) defined.add(m[1]);
  const d4 = /([A-Za-z_$][\w$]*)\s*:\s*(?:async\s*)?function/g;
  while ((m = d4.exec(txt))) defined.add(m[1]);
  const d5 = /(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=/g;
  while ((m = d5.exec(txt))) defined.add(m[1]);
}

const missing = Object.keys(handlers).filter(n => !defined.has(n)).sort();
console.log('=== 内联事件处理器总数(去重) ===', Object.keys(handlers).length);
console.log('=== 找不到定义的 ===', missing.length);
for (const n of missing) {
  console.log('\n### ' + n);
  const locs = handlers[n];
  console.log('  引用次数:', locs.length, '| 前5处:', locs.slice(0, 5).join(', '));
}
fs.writeFileSync('.audit_handlers.json', JSON.stringify({ handlers, missing }, null, 1));
