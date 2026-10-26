// 一次性审查脚本：判断内联 handler 是否真的挂在 window 上（IIFE 内的函数默认不可达）
const fs = require('fs');
const path = require('path');

function walk(d, out = []) {
  for (const f of fs.readdirSync(d, { withFileTypes: true })) {
    const p = path.join(d, f.name);
    if (f.isDirectory()) {
      if (['vendor', '.playwright-cli', 'assets', 'images', 'sounds', 'licenses', 'blocks'].includes(f.name)) continue;
      walk(p, out);
    } else if (/\.(js|html)$/.test(f.name) && !/\.esb$/.test(f.name)) out.push(p);
  }
  return out;
}

const files = walk('.').filter(f => !f.includes('.audit_scan'));
const handlers = {};
const re = /on(?:click|change|input|submit|keydown|keyup|blur|focus|dblclick|touchstart)\s*=\s*(["'\\`])([\s\S]{0,200}?)\1/g;
for (const f of files) {
  const txt = fs.readFileSync(f, 'utf8');
  let m;
  while ((m = re.exec(txt))) {
    const fm = m[2].match(/^\s*([A-Za-z_$][\w$]*)\s*\(/);
    if (!fm) continue;
    const name = fm[1];
    if (['if', 'for', 'while', 'switch', 'return', 'typeof', 'function', 'catch'].includes(name)) continue;
    const line = txt.slice(0, m.index).split('\n').length;
    (handlers[name] = handlers[name] || []).push(f.replace(/\\/g, '/') + ':' + line);
  }
}

// 全局可达集合：window.X = / window['X'] = / globalThis.X = / window.X ||=
const globalSet = new Set();
// 顶层函数声明集合（只统计“文件不是 IIFE 包裹”的情况）
const topFn = new Set();
const iifeFiles = new Set();
for (const f of files) {
  const txt = fs.readFileSync(f, 'utf8');
  const head = txt.slice(0, 400).replace(/^\/\/.*$/gm, '').trim();
  const isIife = /^[;(]?\s*\(\s*(async\s*)?function/.test(head) || /^[;(]?\s*\(\s*\(?\s*\)?\s*=>/.test(head) || /^[;(]?\s*!function/.test(head);
  if (isIife) iifeFiles.add(f);
  let m;
  const g1 = /(?:window|globalThis)\s*(?:\.\s*([A-Za-z_$][\w$]*)|\[\s*['"]([^'"]+)['"]\s*\])\s*=(?!=)/g;
  while ((m = g1.exec(txt))) globalSet.add(m[1] || m[2]);
  if (!isIife) {
    const g2 = /^(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*\(/gm;
    while ((m = g2.exec(txt))) topFn.add(m[1]);
  }
}

const missing = Object.keys(handlers).filter(n => !globalSet.has(n) && !topFn.has(n)).sort();
console.log('handler 总数:', Object.keys(handlers).length);
console.log('window 赋值暴露:', globalSet.size);
console.log('疑似死入口:', missing.length);
for (const n of missing) {
  const locs = handlers[n];
  console.log('\n## ' + n + '  (' + locs.length + ' 处引用)');
  console.log('   ' + locs.slice(0, 6).join('\n   '));
}
fs.writeFileSync('.audit_dead.json', JSON.stringify({ missing, handlers }, null, 1));
