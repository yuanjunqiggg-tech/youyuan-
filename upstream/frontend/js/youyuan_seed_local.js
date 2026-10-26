/* 种子查询 · 本机计算核心（纯前端，不联网）
 *
 * 为什么放在浏览器里：
 *   种子地图是「拖动一次就要重算一片区块」的高频操作。放到服务端意味着每次平移都打一次
 *   HTTP，公网开放后一台机器根本扛不住几千人同时滑动。这里把算法整段搬到本机，服务端只
 *   保留「读当前世界种子 / 单点查询」两个低频接口。
 *
 * 算法来源（逐条对应 upstream/youyuan_seed_api.go，语义一致）：
 *   yySeedNewJavaRandom / next / nextInt / nextDouble  →  JavaRandom / next / nextInt / nextDouble
 *   yySeedIsSlimeChunk                                 →  isSlimeChunk
 *   yySeedSlime                                        →  slimeChunks
 *   yySeedStrongholds / yySeedStrongholdFarEnough      →  strongholds
 *   yySeedStructureCandidate                           →  structureCandidate
 *   yySeedStructureScanEx（逐个区域精确扫描，lod=2 语义） → structures
 *
 * 与 Go 版唯一的差别是「边界」而不是「公式」：
 *   - Go 用 int64 自然溢出（Java 的 long 语义），JS 的 Number 只有 53 位精度，
 *     所以这里全部用 BigInt + BigInt.asIntN(64, …) 显式做 64 位回绕，结果与 Java 原版一致；
 *   - 单次调用有硬上限（maxChunks / maxRegions），超了就返回 truncated=true，
 *     由页面按视口缩小范围，而不是悄悄少算几块。
 *
 * 不伪造：没有实现的算法（基岩版史莱姆/结构、生物群系过滤、出生点）一律返回
 * ok:false + 中文原因，不给近似坐标。
 */
var YYSeedLocal = (function () {
  'use strict';

  var MASK48 = (1n << 48n) - 1n;
  var MULT = 0x5DEECE66Dn;
  var ADD = 0xBn;

  /* ── 基础：Java java.util.Random（48 位 LCG）────────────────────── */

  function JavaRandom(seedBig) {
    this.s = BigInt.asIntN(64, seedBig ^ MULT) & MASK48;
  }
  JavaRandom.prototype.next = function (bits) {
    this.s = (this.s * MULT + ADD) & MASK48;
    return Number(this.s >> BigInt(48 - bits));
  };
  // Java 语义：2 的幂走乘法快速路径，否则拒绝采样消除取模偏差。
  JavaRandom.prototype.nextInt = function (bound) {
    if (bound <= 0) return 0;
    if ((bound & -bound) === bound) {
      return Number((BigInt(bound) * BigInt(this.next(31))) >> 31n);
    }
    for (;;) {
      var bits = this.next(31);
      var val = bits % bound;
      if (bits - val + (bound - 1) >= 0) return val;
    }
  };
  JavaRandom.prototype.nextDouble = function () {
    var hi = this.next(26);
    var lo = this.next(27);
    return (hi * 134217728 + lo) / 9007199254740992;
  };

  /* ── 种子解析 ───────────────────────────────────────────────────── */

  var INT64_MIN = -9223372036854775808n;
  var INT64_MAX = 9223372036854775807n;

  // parseSeed 把用户输入的十进制串解析成 BigInt。非整数 / 越界一律报错，不猜。
  function parseSeed(text) {
    var raw = String(text == null ? '' : text).trim();
    if (!raw) return { ok: false, error: '请输入种子（十进制整数）' };
    if (!/^[+-]?\d{1,20}$/.test(raw)) return { ok: false, error: '种子必须是十进制整数，例如 123456789 或 -123456789' };
    var big;
    try {
      big = BigInt(raw);
    } catch (e) {
      return { ok: false, error: '种子不是合法整数' };
    }
    if (big < INT64_MIN || big > INT64_MAX) return { ok: false, error: '种子超出 64 位整数范围' };
    return { ok: true, value: big, text: big.toString() };
  }

  /* ── 史莱姆区块（Java 版，exact）────────────────────────────────── */

  // 与 yySeedIsSlimeChunk 一致：
  //   sum = seed + x*x*0x4c1906 + x*0x5ac0db + z*z*0x4307a7 + z*0x5f24f（int64 回绕）
  //   Random(sum ^ 0x3ad8025f).nextInt(10) == 0
  function isSlimeChunkBig(seedBig, cx, cz) {
    var x = BigInt(cx);
    var z = BigInt(cz);
    var sum = BigInt.asIntN(64, seedBig + x * x * 0x4c1906n + x * 0x5ac0dbn + z * z * 0x4307a7n + z * 0x5f24fn);
    var rnd = new JavaRandom(BigInt.asIntN(64, sum ^ 0x3ad8025fn));
    return rnd.nextInt(10) === 0;
  }

  function isSlimeChunk(seedText, cx, cz) {
    var s = parseSeed(seedText);
    if (!s.ok) return { ok: false, error: s.error };
    return { ok: true, slime: isSlimeChunkBig(s.value, Math.trunc(cx), Math.trunc(cz)) };
  }

  // slimeChecker 把种子只解析一次，返回逐区块判定函数。
  // 地图拖动时每帧只补几十个区块，省掉反复 BigInt 解析；结果与 isSlimeChunk 完全一致。
  function slimeChecker(seedText) {
    var s = parseSeed(seedText);
    if (!s.ok) return null;
    return function (cx, cz) { return isSlimeChunkBig(s.value, Math.trunc(cx), Math.trunc(cz)); };
  }

  /* ── 结构候选（Java 1.13+ 区域网格，candidate）──────────────────── */

  // 与 yySeedStructureSpecs 同一张表（Java 1.13~1.20 主流公开值）。
  // 没有收录的版本/结构不要编：版本支持见 versionSupport()。
  var SPECS = [
    { id: 'village', label: '村庄', spacing: 34, separation: 8, salt: 10387312, dimension: '主世界' },
    { id: 'fortress', label: '下界要塞', spacing: 27, separation: 4, salt: 30084232, dimension: '下界' },
    { id: 'bastion', label: '堡垒遗迹', spacing: 27, separation: 4, salt: 30084232, dimension: '下界' },
    { id: 'monument', label: '海底神殿', spacing: 32, separation: 5, salt: 10387313, dimension: '主世界' },
    { id: 'mansion', label: '林地府邸', spacing: 80, separation: 20, salt: 10387319, dimension: '主世界' },
    { id: 'temple', label: '神庙（沙漠神殿 / 丛林神庙 / 女巫小屋）', spacing: 32, separation: 8, salt: 14357617, dimension: '主世界' }
  ];

  function specById(id) {
    for (var i = 0; i < SPECS.length; i++) if (SPECS[i].id === id) return SPECS[i];
    return null;
  }

  // 与 yySeedStructureCandidate 一致：
  //   mix = seed + regionX*341873128712 + regionZ*132897987541 + salt
  //   rnd = Random(mix)
  //   chunkX = regionX*spacing + rnd.nextInt(spacing - separation)
  function structureCandidate(seedBig, spec, regionX, regionZ) {
    var rx = BigInt(regionX);
    var rz = BigInt(regionZ);
    var mix = BigInt.asIntN(64, seedBig + rx * 341873128712n + rz * 132897987541n + BigInt(spec.salt));
    var rnd = new JavaRandom(mix);
    var span = spec.spacing - spec.separation;
    if (span <= 0) span = 1;
    return [regionX * spec.spacing + rnd.nextInt(span), regionZ * spec.spacing + rnd.nextInt(span)];
  }

  function floorDiv(a, b) {
    if (!b) return 0;
    return Math.floor(a / b);
  }

  /* ── 要塞（Java 1.9+ 三环，approximate）─────────────────────────── */

  function goRound(v) {
    // Go 的 math.Round 是「远离零取整」，JS 的 Math.round 是「向上取整」，负数半值会差 1。
    return v < 0 ? -Math.round(-v) : Math.round(v);
  }

  function strongholdsBig(seedBig, count) {
    count = Math.max(1, Math.min(16, count || 3));
    var rnd = new JavaRandom(seedBig);
    var base = rnd.nextDouble() * 2 * Math.PI;
    var radii = [1408, 4352, 7424];
    var out = [];
    for (var i = 0; i < count; i++) {
      var ring = (i % 3) + 1;
      var radius = radii[ring - 1];
      var start = rnd.nextInt(128);
      var chosen = null;
      for (var attempt = 0; attempt < 128; attempt++) {
        var angle = base + (2 * Math.PI * ((start + attempt) % 128)) / 128;
        var x = goRound(radius * Math.cos(angle));
        var z = goRound(radius * Math.sin(angle));
        var cand = {
          x: x,
          z: z,
          ring: ring,
          chunkX: floorDiv(x, 16),
          chunkZ: floorDiv(z, 16),
          radiusBlocks: radius,
          angle: goRound((((angle * 180) / Math.PI + 360) % 360) * 10) / 10
        };
        if (farEnough(out, cand, 512)) {
          chosen = cand;
          break;
        }
        if (attempt === 127) chosen = cand;
      }
      out.push(chosen);
    }
    return out;
  }

  function farEnough(list, cand, minDist) {
    for (var i = 0; i < list.length; i++) {
      var dx = list[i].x - cand.x;
      var dz = list[i].z - cand.z;
      if (Math.sqrt(dx * dx + dz * dz) < minDist) return false;
    }
    return true;
  }

  /* ── 对外：史莱姆区块矩形扫描 ───────────────────────────────────── */

  var MAX_SLIME_CHUNKS = 40000;

  // slimeChunks(seed, x0, z0, w, h) —— 区块坐标矩形，w/h 为区块数。
  // 返回 chunks（区块坐标）与 blocks（方块坐标，×16），按 x 再 z 排序，与 Go 版一致。
  function slimeChunks(seedText, x0, z0, w, h, opts) {
    var s = parseSeed(seedText);
    if (!s.ok) return { ok: false, error: s.error };
    var max = (opts && opts.maxChunks) || MAX_SLIME_CHUNKS;
    var width = Math.max(0, Math.trunc(w));
    var height = Math.max(0, Math.trunc(h));
    if (!width || !height) return { ok: true, chunks: [], blocks: [], count: 0, scanned: 0, truncated: false };
    if (width * height > max) {
      return { ok: false, error: '范围过大（' + width + '×' + height + ' 区块），请缩小视口或放大后再看史莱姆区块', truncated: true };
    }
    var startX = Math.trunc(x0);
    var startZ = Math.trunc(z0);
    var chunks = [];
    var blocks = [];
    var scanned = 0;
    for (var dx = 0; dx < width; dx++) {
      for (var dz = 0; dz < height; dz++) {
        var cx = startX + dx;
        var cz = startZ + dz;
        scanned++;
        if (!isSlimeChunkBig(s.value, cx, cz)) continue;
        chunks.push([cx, cz]);
        blocks.push([cx * 16, cz * 16]);
      }
    }
    chunks.sort(function (a, b) { return a[0] - b[0] || a[1] - b[1]; });
    blocks.sort(function (a, b) { return a[0] - b[0] || a[1] - b[1]; });
    return { ok: true, seed: s.text, chunks: chunks, blocks: blocks, count: chunks.length, scanned: scanned, truncated: false, precision: 'exact' };
  }

  /* ── 对外：结构候选扫描 ─────────────────────────────────────────── */

  var MAX_STRUCT_REGIONS = 12000;

  // structures(seed, type, region, opts)
  //   region: {fromX, fromZ, toX, toZ} 区块坐标（含端点）
  //   opts:   {maxRegions, limit}
  // 逐区域精确算（等价 Go 的 lod=2 路径），不做抽样；超上限就截断并标 truncated。
  function structures(seedText, type, region, opts) {
    var s = parseSeed(seedText);
    if (!s.ok) return { ok: false, error: s.error };
    var spec = specById(type);
    if (!spec) return { ok: false, error: '不支持的结构类型：' + type };
    var fromX = Math.trunc(region.fromX), toX = Math.trunc(region.toX);
    var fromZ = Math.trunc(region.fromZ), toZ = Math.trunc(region.toZ);
    if (toX < fromX) { var t = fromX; fromX = toX; toX = t; }
    if (toZ < fromZ) { var t2 = fromZ; fromZ = toZ; toZ = t2; }
    var maxRegions = (opts && opts.maxRegions) || MAX_STRUCT_REGIONS;
    var limit = (opts && opts.limit) || 0;
    var rx0 = floorDiv(fromX, spec.spacing), rx1 = floorDiv(toX, spec.spacing);
    var rz0 = floorDiv(fromZ, spec.spacing), rz1 = floorDiv(toZ, spec.spacing);
    var regions = (rx1 - rx0 + 1) * (rz1 - rz0 + 1);
    var chunks = [];
    var scanned = 0;
    var truncated = false;
    for (var rz = rz0; rz <= rz1; rz++) {
      for (var rx = rx0; rx <= rx1; rx++) {
        if (scanned >= maxRegions) { truncated = true; break; }
        scanned++;
        var c = structureCandidate(s.value, spec, rx, rz);
        if (c[0] < fromX || c[0] > toX || c[1] < fromZ || c[1] > toZ) continue;
        chunks.push(c);
        if (limit && chunks.length >= limit) { truncated = true; break; }
      }
      if (truncated) break;
    }
    chunks.sort(function (a, b) { return a[0] - b[0] || a[1] - b[1]; });
    var blocks = chunks.map(function (c) { return [c[0] * 16, c[1] * 16]; });
    return {
      ok: true, seed: s.text, type: spec.id, label: spec.label, dimension: spec.dimension,
      spacing: spec.spacing, separation: spec.separation, salt: spec.salt,
      chunks: chunks, blocks: blocks, count: chunks.length,
      regions: regions, scanned: scanned, truncated: truncated,
      precision: 'candidate',
      note: '候选区块：实际是否生成取决于生物群系与版本，需进游戏实测'
    };
  }

  // 单区域查询（给「点地图看这里有什么」用，成本 O(1)）。
  function structureAt(seedText, type, chunkX, chunkZ) {
    var s = parseSeed(seedText);
    if (!s.ok) return { ok: false, error: s.error };
    var spec = specById(type);
    if (!spec) return { ok: false, error: '不支持的结构类型：' + type };
    var rx = floorDiv(chunkX, spec.spacing), rz = floorDiv(chunkZ, spec.spacing);
    var c = structureCandidate(s.value, spec, rx, rz);
    return { ok: true, chunk: c, block: [c[0] * 16, c[1] * 16], region: [rx, rz], spacing: spec.spacing, label: spec.label };
  }

  /* ── 对外：要塞 ─────────────────────────────────────────────────── */

  function strongholds(seedText, count) {
    var s = parseSeed(seedText);
    if (!s.ok) return { ok: false, error: s.error };
    return { ok: true, seed: s.text, points: strongholdsBig(s.value, count || 3), precision: 'approximate' };
  }

  /* ── 版本与能力表（唯一事实来源）────────────────────────────────── */

  // 说明：这不是「Minecraft 支持哪些版本」，而是「本机算法表覆盖到哪」。
  // 1.21 的结构参数没有核对过，就不写支持；史莱姆区块算法自 Java 1.0 起未变，仍然可用。
  var JAVA_VERSIONS = ['1.16', '1.17', '1.18', '1.19', '1.20', '1.21'];
  var BEDROCK_VERSIONS = ['1.16', '1.17', '1.18', '1.19', '1.20', '1.21'];

  function javaSupport(version) {
    var v = String(version || '1.20');
    if (JAVA_VERSIONS.indexOf(v) < 0) {
      return { slime: false, stronghold: false, structure: false, reason: '未收录该 Java 版本' };
    }
    if (v === '1.21') {
      return {
        slime: true, stronghold: false, structure: false,
        reason: '本机结构参数表只核对到 Java 1.20；1.21 的结构放置参数未收录，因此村庄/神殿/府邸等一律不显示（史莱姆区块算法未变，仍可用）'
      };
    }
    return {
      slime: true, stronghold: true, structure: true,
      reason: '结构为 Java 1.13+ 区域网格候选值（本机取 1.13~1.20 主流公开参数），需实测确认；要塞省略了原版生物群系过滤'
    };
  }

  function bedrockSupport() {
    return {
      slime: false, stronghold: false, structure: false,
      reason: '基岩版的随机数与结构盐值和 Java 不同，本机未实现基岩版算法。可点「按 Java 算法试算」看同一串数字在 Java 下的结果，但那不是这个世界真实的结构位置'
    };
  }

  // capabilities(edition, version, preview) → 当前选择下每种能力是否可用
  function capabilities(edition, version, preview) {
    if (edition === 'bedrock' && !preview) return bedrockSupport();
    return javaSupport(version);
  }

  return {
    VERSIONS: { java: JAVA_VERSIONS.slice(), bedrock: BEDROCK_VERSIONS.slice() },
    SPECS: SPECS,
    specById: specById,
    parseSeed: parseSeed,
    isSlimeChunk: isSlimeChunk,
    slimeChecker: slimeChecker,
    slimeChunks: slimeChunks,
    structures: structures,
    structureAt: structureAt,
    strongholds: strongholds,
    javaSupport: javaSupport,
    bedrockSupport: bedrockSupport,
    capabilities: capabilities,
    limits: { maxSlimeChunks: MAX_SLIME_CHUNKS, maxStructureRegions: MAX_STRUCT_REGIONS }
  };
})();
