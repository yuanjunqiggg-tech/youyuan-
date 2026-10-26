/* 种子地图 v2 · 本地验收服务（只用于截图，不进交付）
 * 静态代理 upstream/frontend + 伪造 /api/seed/v2/*。
 * 用法：node mock-server.js [port]
 */
'use strict';
const http = require('http');
const fs = require('fs');
const path = require('path');
const zlib = require('zlib');

const ROOT = path.resolve(__dirname, '../../upstream/frontend');
const PORT = Number(process.argv[2] || 8787);
const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.png': 'image/png',
  '.svg': 'image/svg+xml',
  '.json': 'application/json; charset=utf-8',
  '.woff2': 'font/woff2',
  '.ttf': 'font/ttf',
  '.mp3': 'audio/mpeg',
  '.wav': 'audio/wav',
  '.txt': 'text/plain; charset=utf-8'
};

let TILE_FAIL = false;
let TILE_DELAY = 0;

/* ── PNG 编码（无依赖）───────────────────────────────────────────── */
function crc32(buf) {
  let crc = 0xffffffff;
  for (let n = 0; n < buf.length; n++) {
    let c = (crc ^ buf[n]) & 0xff;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    crc = (crc >>> 8) ^ c;
  }
  return (crc ^ 0xffffffff) >>> 0;
}
function chunk(type, data) {
  const len = Buffer.alloc(4); len.writeUInt32BE(data.length);
  const t = Buffer.from(type, 'ascii');
  const crc = Buffer.alloc(4); crc.writeUInt32BE(crc32(Buffer.concat([t, data])));
  return Buffer.concat([len, t, data, crc]);
}
function encodePNG(w, h, rgb) {
  const stride = w * 3;
  const raw = Buffer.alloc((stride + 1) * h);
  for (let y = 0; y < h; y++) {
    raw[y * (stride + 1)] = 0;
    rgb.copy(raw, y * (stride + 1) + 1, y * stride, (y + 1) * stride);
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(w, 0); ihdr.writeUInt32BE(h, 4);
  ihdr[8] = 8; ihdr[9] = 2; ihdr[10] = 0; ihdr[11] = 0; ihdr[12] = 0;
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk('IHDR', ihdr),
    chunk('IDAT', zlib.deflateSync(raw, { level: 6 })),
    chunk('IEND', Buffer.alloc(0))
  ]);
}
function hash(n) {
  const x = Math.sin(n * 12.9898 + 78.233) * 43758.5453;
  return x - Math.floor(x);
}
const PALETTE = [
  [91, 122, 63], [110, 138, 74], [142, 163, 90], [79, 125, 122],
  [169, 168, 106], [63, 95, 99], [198, 187, 132], [74, 104, 84]
];
function makeTile(cx, cz, bpp, size) {
  const w = size || 256, h = size || 256;
  const rgb = Buffer.alloc(w * h * 3);
  const step = 16;
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const gx = Math.floor(x / step), gy = Math.floor(y / step);
      const k = Math.floor(hash(cx * 131 + gx * 7 + cz * 17 + gy * 13 + bpp) * PALETTE.length);
      let [r, g, b] = PALETTE[k];
      if (hash(cx + gx * 3.1 + cz * 2.7 + gy * 5.3) > 0.88) { r += 18; g += 16; b += 10; }
      const grid = (x % 64 === 0 || y % 64 === 0) ? -10 : 0;
      const o = (y * w + x) * 3;
      rgb[o] = Math.max(0, Math.min(255, r + grid));
      rgb[o + 1] = Math.max(0, Math.min(255, g + grid));
      rgb[o + 2] = Math.max(0, Math.min(255, b + grid));
    }
  }
  return encodePNG(w, h, rgb);
}

/* ── 假数据 ──────────────────────────────────────────────────────── */
const JAVA = ['1.21.4', '1.21.3', '1.21.2', '1.21.1', '1.21', '1.20.6', '1.20.5',
  '1.20.4', '1.20.3', '1.20.2', '1.20.1', '1.20', '1.19.4', '1.19.3', '1.19.2',
  '1.19.1', '1.19', '1.18.2', '1.18.1', '1.18', '1.17.1', '1.16.5'];
const BEDROCK = ['1.21.50', '1.21.40', '1.21.30', '1.21.20', '1.21.10', '1.21.0',
  '1.20.80', '1.20.60', '1.20.50', '1.20.40', '1.20.30', '1.20.10'];
const STRUCTS = [
  ['village', '村庄', 'overworld', 'exact', '#2f9e5f', true, 26],
  ['pillagerOutpost', '掠夺者前哨站', 'overworld', 'exact', '#b0432f', true, 9],
  ['desertPyramid', '沙漠神殿', 'overworld', 'exact', '#a8862a', false, 7],
  ['jungleTemple', '丛林神庙', 'overworld', 'exact', '#5f7a3a', false, 6],
  ['witchHut', '沼泽小屋', 'overworld', 'exact', '#6b6fa8', false, 5],
  ['igloo', '雪屋', 'overworld', 'exact', '#7aa8b0', false, 5],
  ['shipwreck', '沉船', 'overworld', 'candidate', '#2f6f9e', false, 11],
  ['oceanMonument', '海底神殿', 'overworld', 'exact', '#2f8f9e', true, 6],
  ['woodlandMansion', '林地府邸', 'overworld', 'exact', '#7a4b9e', true, 4],
  ['ancientCity', '远古城市', 'overworld', 'candidate', '#3c4a6e', true, 8],
  ['trialChamber', '试炼密室', 'overworld', 'candidate', '#9e6f2f', false, 10],
  ['ruinedPortal', '废弃传送门', 'overworld', 'candidate', '#8a5a2b', false, 12],
  ['buriedTreasure', '埋藏的宝藏', 'overworld', 'candidate', '#c2a24a', false, 9],
  ['fortress', '下界要塞', 'nether', 'exact', '#b0432f', true, 8],
  ['bastion', '堡垒遗迹', 'nether', 'exact', '#6f4a2b', true, 7],
  ['ruinedPortalNether', '废弃传送门', 'nether', 'candidate', '#8a5a2b', false, 6],
  ['endCity', '末地城', 'end', 'exact', '#8f6fa8', true, 6],
  ['endShip', '末地船', 'end', 'exact', '#6f5a9e', false, 4]
];

function manifest() {
  return {
    ok: true, engine: 'seedv2', btree_tables: ['btree18', 'btree192', 'btree19', 'btree20', 'btree21wd'],
    biome_engine: true, render_engine: true,
    editions: [
      { id: 'java', name: 'Java 版', versions: JAVA.map(v => ({ label: v, code: 100000 })) },
      { id: 'bedrock', name: '基岩版', versions: BEDROCK.map(v => ({ label: v, code: 100000 })) }
    ],
    dimensions: [
      { id: 'overworld', name: '主世界' },
      { id: 'nether', name: '下界' },
      { id: 'end', name: '末地' }
    ],
    structures: STRUCTS.map(s => ({
      id: s[0], name: s[1], dim: s[2], precision: s[3], color: s[4], default: s[5]
    }))
  };
}

function structurePoints(id, count, r) {
  const seed = 0;
  const arr = [];
  for (let i = 0; i < count * 12; i++) {
    const h1 = hash(i * 7.13 + id.length * 3.7 + seed);
    const h2 = hash(i * 3.31 + id.length * 11.1 + seed);
    const x = Math.round((h1 * 2 - 1) * r);
    const z = Math.round((h2 * 2 - 1) * r);
    arr.push({ x, z });
    if (arr.length >= count) break;
  }
  return arr;
}

function structures(q) {
  const r = Math.min(512, Number(q.r) || 64);
  let wanted = String(q.types || '').split(',').filter(Boolean);
  if (!wanted.length) {
    const dim = q.dim || 'overworld';
    wanted = STRUCTS.filter(s => s[2] === dim && s[5]).map(s => s[0]);
  }
  const out = {}, counts = {};
  let total = 0;
  wanted.forEach(id => {
    const def = STRUCTS.find(s => s[0] === id);
    if (!def) return;
    const scale = Math.max(0.35, r / 64);
    const pts = structurePoints(id, Math.round(def[6] * scale), r)
      .filter(p => Math.abs(p.x) <= r && Math.abs(p.z) <= r);
    out[id] = pts;
    counts[id] = pts.length;
    total += pts.length;
  });
  return { ok: true, seed: Number(q.seed) || 0, structures: out, counts, truncated: total > 900, elapsed_ms: 11 };
}

function slime(q) {
  const r = Math.min(96, Number(q.r) || 16);
  const cx = Number(q.cx) || 0, cz = Number(q.cz) || 0;
  const chunks = [];
  for (let i = -r; i <= r; i++) {
    for (let j = -r; j <= r; j++) {
      if (hash((cx + i) * 73856093 + (cz + j) * 19349663) < 0.11) chunks.push({ x: cx + i, z: cz + j });
    }
  }
  return { ok: true, chunks, count: chunks.length };
}

function biome(q) {
  const names = [['平原', '#8db360'], ['森林', '#4f7a35'], ['沙漠', '#e0c860'], ['海洋', '#16418c'], ['雪原', '#eef4f4'], ['沼泽', '#51703d']];
  const pick = names[Math.floor(hash((Number(q.x) || 0) * 31 + (Number(q.z) || 0) * 17) * names.length)];
  return {
    ok: true,
    points: [{
      x: Number(q.x) || 0, z: Number(q.z) || 0, biome: 1,
      key: 'minecraft:' + pick[0], name: pick[0], color: pick[1]
    }]
  };
}

function stronghold(q) {
  const arr = [{ x: -88, z: 64 }, { x: 42, z: -120 }, { x: 160, z: 38 }];
  return { ok: true, strongholds: arr, count: arr.length, precision: 'exact' };
}

/* ── HTTP ────────────────────────────────────────────────────────── */
function json(res, obj, status) {
  const body = Buffer.from(JSON.stringify(obj));
  res.writeHead(status || 200, {
    'Content-Type': 'application/json; charset=utf-8',
    'Content-Length': body.length,
    'Cache-Control': 'no-store'
  });
  res.end(body);
}

const server = http.createServer((req, res) => {
  const u = new URL(req.url, 'http://127.0.0.1');
  const p = decodeURIComponent(u.pathname);
  const q = Object.fromEntries(u.searchParams.entries());

  if (p === '/__tilefail') {
    TILE_FAIL = q.on === '1';
    return json(res, { ok: true, tilefail: TILE_FAIL });
  }
  if (p === '/__slow') {
    TILE_DELAY = Number(q.ms) || 0;
    return json(res, { ok: true, delay: TILE_DELAY });
  }
  if (p === '/api/seed/v2/manifest') return json(res, manifest());
  if (p === '/api/seed/v2/meta') {
    return json(res, { ok: true, version: q.version, edition: q.edition, structures: manifest().structures, unsupported: [] });
  }
  if (p === '/api/seed/v2/structures') return json(res, structures(q));
  if (p === '/api/seed/v2/slime') return json(res, slime(q));
  if (p === '/api/seed/v2/biome') return json(res, biome(q));
  if (p === '/api/seed/v2/stronghold') return json(res, stronghold(q));
  if (p === '/api/seed/v2/tile.png') {
    if (TILE_FAIL) return json(res, { ok: false, error: '生物群系表未装载' }, 503);
    const buf = makeTile(Number(q.cx) || 0, Number(q.cz) || 0, Number(q.bpp) || 4, Math.min(512, Math.max(32, Number(q.w) || 256)));
    const send = () => {
      if (res.destroyed) return;
      res.writeHead(200, {
        'Content-Type': 'image/png',
        'Content-Length': buf.length,
        'Cache-Control': 'public, max-age=60'
      });
      res.end(buf);
    };
    if (TILE_DELAY) setTimeout(send, TILE_DELAY);
    else send();
    return;
  }
  if (p === '/api/seed/v2/icon.svg') {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" width="12" height="12">' +
      '<path d="M6 1.2 10.2 6 6 10.8 1.8 6Z" fill="#0b1118" opacity=".82"/></svg>';
    res.writeHead(200, { 'Content-Type': 'image/svg+xml', 'Cache-Control': 'no-store' });
    return res.end(svg);
  }
  if (p.startsWith('/api/')) {
    return json(res, { ok: false, error: '验收服务没有这个接口' }, 404);
  }

  let file = p === '/' ? '/index.html' : p;
  const full = path.join(ROOT, file);
  if (!full.startsWith(ROOT)) { res.writeHead(403); return res.end('forbidden'); }
  fs.readFile(full, (err, data) => {
    if (err) { res.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' }); return res.end('404 ' + file); }
    res.writeHead(200, {
      'Content-Type': MIME[path.extname(full).toLowerCase()] || 'application/octet-stream',
      'Cache-Control': 'no-store'
    });
    res.end(data);
  });
});

server.listen(PORT, '127.0.0.1', () => {
  console.log('mock server on http://127.0.0.1:' + PORT + ' root=' + ROOT);
});
