/**
 * avatar-probe.js —— 头像显示诊断（只读，不改页面状态）
 * 用法：① 先点顶部「账号」标签，等账号卡片出现 ② F12 → Console → 粘贴本文件全部内容 → 回车 → 等 3 秒
 * ③ 把输出复制或截图发回（可重复运行；只发 GET，不写任何数据）
 */
(function () {
  'use strict';

  if (window.__yyAvatarProbeRunning) {
    console.warn('[头像诊断] 上一次还没跑完，本次忽略。');
    return;
  }
  window.__yyAvatarProbeRunning = true;

  var TAG = '[头像诊断]';
  var TOKEN = '';

  function log() {
    var a = Array.prototype.slice.call(arguments);
    a.unshift(TAG);
    console.log.apply(console, a);
  }
  function group(title) {
    try { console.group(TAG + ' ' + title); } catch (e) { log(title); }
  }
  function groupEnd() {
    try { console.groupEnd(); } catch (e) {}
  }
  function cls(el) {
    if (!el) return '(无父元素)';
    var c = el.className;
    if (typeof c === 'string') return c.trim() || '(空)';
    if (c && c.baseVal) return c.baseVal || '(空)';
    return '(空)';
  }
  function hex(u8) {
    var out = [];
    for (var i = 0; i < u8.length; i++) out.push(('0' + u8[i].toString(16)).slice(-2));
    return out.join(' ');
  }

  // ---------- 1. 页面位置 ----------
  function readToken() {
    try {
      var m = document.querySelector('meta[name="yy-console-token"]');
      if (m && m.getAttribute('content')) return m.getAttribute('content').trim();
    } catch (e) {}
    try { if (window.__yyConsoleToken) return String(window.__yyConsoleToken); } catch (e) {}
    return '';
  }

  function step1() {
    group('1. 页面位置');
    log('href   :', location.href);
    log('origin :', location.origin);
    log('hash   :', location.hash || '(空)');
    log('path   :', location.pathname);
    TOKEN = readToken();
    log('console token :', TOKEN ? ('已取到（' + TOKEN.length + ' 字符，不回显）') : '未取到（/api/skin/* 是豁免接口，不影响）');
    groupEnd();
  }

  // ---------- 2. 探测工具 ----------
  function probe(url, withToken) {
    var opt = { method: 'GET', credentials: 'same-origin', cache: 'no-store' };
    if (withToken && TOKEN) opt.headers = { 'X-YouYuan-Console-Token': TOKEN };
    return fetch(url, opt).then(function (r) {
      return r.arrayBuffer().then(function (buf) {
        var ct = r.headers.get('content-type') || '(无)';
        var bytes = buf.byteLength;
        var head;
        if (/^image\//i.test(ct)) {
          head = 'PNG/图片魔数 ' + hex(new Uint8Array(buf.slice(0, 8)));
        } else {
          head = new TextDecoder('utf-8').decode(new Uint8Array(buf.slice(0, 80))).replace(/\s+/g, ' ');
        }
        return { ok: true, status: r.status, ct: ct, bytes: bytes, head: head };
      });
    }).catch(function (e) {
      return { ok: false, error: String((e && e.message) || e) };
    });
  }

  function printProbe(label, res) {
    if (!res.ok) { log(label, '→ 请求失败:', res.error); return; }
    log(label, '→', 'status=' + res.status, '|', 'type=' + res.ct, '|', res.bytes + 'B', '|', res.head);
  }

  // ---------- 3. 账号接口 ----------
  var accounts = [];

  function step3() {
    group('3. /api/youyuan/accounts');
    return probe('/api/youyuan/accounts', true).then(function (res) {
      printProbe('/api/youyuan/accounts', res);
      return fetch('/api/youyuan/accounts', {
        method: 'GET', credentials: 'same-origin', cache: 'no-store',
        headers: TOKEN ? { 'X-YouYuan-Console-Token': TOKEN } : {}
      }).then(function (r) { return r.json(); }).catch(function () { return null; });
    }).then(function (data) {
      if (!data) { log('无法解析 accounts JSON'); groupEnd(); return; }
      accounts = data.accounts || data.items || [];
      log('账号条数:', accounts.length);
      accounts.forEach(function (a, i) {
        var av = a.avatar === null || a.avatar === undefined ? null : String(a.avatar);
        log('  #' + (i + 1),
          'id=' + a.id,
          '| name=' + a.name,
          '| player_name=' + (a.player_name === undefined ? 'undefined' : JSON.stringify(a.player_name)),
          '| uuid=' + (a.uuid === undefined ? 'undefined' : JSON.stringify(a.uuid)),
          '| avatar_source=' + (a.avatar_source === undefined ? 'undefined' : JSON.stringify(a.avatar_source)),
          '| avatar=' + (av === null ? 'null' : (av === '' ? '空字符串' : ('有值(' + av.length + '字符,' + av.slice(0, 22) + ')')))
        );
      });
      groupEnd();
    });
  }

  // ---------- 4. img[data-skin-media] ----------
  var imgNodes = [];
  var cardCount = 0;
  var gridCount = 0;

  function step4() {
    group('4. img[data-skin-media]');
    gridCount = document.querySelectorAll('#yy-accounts-grid').length;
    cardCount = document.querySelectorAll('#yy-accounts-grid > *').length;
    log('账号容器 #yy-accounts-grid:', gridCount, '个 | 容器内账号卡片:', cardCount, '个');
    if (!gridCount) {
      log('【注意】页面里没有 #yy-accounts-grid —— 你不在「账号」页。请先点顶部「账号」标签，等卡片出来再重跑本脚本。');
    } else if (!cardCount) {
      log('【注意】账号容器存在但里面是空的 —— 账号还没渲染出来，稍等几秒重跑。');
    }
    imgNodes = Array.prototype.slice.call(document.querySelectorAll('img[data-skin-media]'));
    log('数量:', imgNodes.length);
    if (!imgNodes.length) {
      log('页面里没有任何 img[data-skin-media] —— 说明头像 markup 根本没生成 img（url 为空），或还没渲染');
    }
    imgNodes.forEach(function (img, i) {
      var want = img.getAttribute('data-skin-media') || '';
      var src = img.getAttribute('src') || '';
      log('  #' + (i + 1),
        '| 期望地址=' + want,
        '| 实际src=' + (src ? (src.slice(0, 40) + (src.length > 40 ? '…' : '')) : '【空】'),
        '| natural=' + img.naturalWidth + 'x' + img.naturalHeight,
        '| complete=' + img.complete,
        '| 父class=' + cls(img.parentElement)
      );
    });
    groupEnd();
  }

  // ---------- 5. 头像容器状态 ----------
  function step5() {
    group('5. 头像容器状态 (.yy-compact-avatar / .yy-avatar-frame)');
    var frames = Array.prototype.slice.call(
      document.querySelectorAll('.yy-compact-avatar, .yy-avatar-frame')
    );
    log('数量:', frames.length);
    var stat = { skeleton: 0, ready: 0, fallback: 0, none: 0, noImg: 0 };
    frames.forEach(function (el, i) {
      var c = el.classList;
      var state = c.contains('yy-avatar-ready') ? 'ready'
        : c.contains('yy-avatar-skeleton') ? 'skeleton'
        : c.contains('yy-avatar-fallback') ? 'fallback' : 'none';
      stat[state] = (stat[state] || 0) + 1;
      var img = el.querySelector('img');
      if (!img) stat.noImg++;
      var initial = el.querySelector('.yy-avatar-initial');
      var initialVisible = false;
      if (initial) {
        var cs = getComputedStyle(initial);
        initialVisible = cs.display !== 'none' && cs.visibility !== 'hidden' && cs.opacity !== '0';
      }
      log('  #' + (i + 1),
        '| class=' + cls(el),
        '| 状态=' + state,
        '| 有img=' + (img ? '是' : '否'),
        '| img.src=' + (img ? ((img.getAttribute('src') || '【空】').slice(0, 36)) : '-'),
        '| 首字母可见=' + initialVisible
      );
    });
    log('统计:', JSON.stringify(stat));
    groupEnd();
    return stat;
  }

  // ---------- 6. 逐个账号探测 account-head ----------
  var probeResults = [];

  function step6() {
    group('6. /api/skin/account-head 逐个账号');
    var ids = [];
    if (accounts.length) {
      accounts.forEach(function (a) { if (a && a.id) ids.push(a.id); });
    } else {
      // 接口拿不到就从 DOM 兜底找
      Array.prototype.slice.call(document.querySelectorAll('[data-account-id],[data-id]')).forEach(function (el) {
        var v = el.getAttribute('data-account-id') || el.getAttribute('data-id');
        if (v && ids.indexOf(v) < 0) ids.push(v);
      });
      if (!ids.length) log('拿不到账号 id（accounts 接口失败且 DOM 无 data-account-id）');
    }
    if (ids.indexOf('main') < 0) ids.push('main');
    log('待探测 id:', ids.join(', '));

    var chain = Promise.resolve();
    ids.forEach(function (id) {
      chain = chain.then(function () {
        var url = '/api/skin/account-head?id=' + encodeURIComponent(id);
        return probe(url, false).then(function (res) {
          probeResults.push({ id: id, res: res });
          printProbe('account-head?id=' + id, res);
        });
      });
    });
    return chain.then(function () { groupEnd(); });
  }

  // ---------- 7. online-head 对照 ----------
  function step7() {
    group('7. /api/skin/online-head 对照');
    var cjk = '';
    for (var i = 0; i < accounts.length; i++) {
      var n = accounts[i] && accounts[i].name;
      if (n && /[\u3400-\u9fff]/.test(n)) { cjk = n; break; }
    }
    if (!cjk) {
      var names = Array.prototype.slice.call(document.querySelectorAll('.yy-compact-identity strong'));
      for (var j = 0; j < names.length; j++) {
        var t = (names[j].textContent || '').trim();
        if (t && /[\u3400-\u9fff]/.test(t)) { cjk = t; break; }
      }
    }
    if (!cjk) { log('页面里没找到中文账号名，跳过'); groupEnd(); return Promise.resolve(); }
    log('用名字:', cjk);
    return probe('/api/skin/online-head?player=' + encodeURIComponent(cjk), false)
      .then(function (res) {
        printProbe('online-head?player=' + cjk, res);
        // 再测一个肯定不存在的名字，看服务端对不存在玩家返回什么
        return probe('/api/skin/online-head?player=' + encodeURIComponent('ZZZ不存在的玩家999'), false);
      })
      .then(function (res) {
        printProbe('online-head?player=ZZZ不存在的玩家999（对照组）', res);
        groupEnd();
      });
  }

  // ---------- 8. 自动结论 ----------
  function step8(stat) {
    console.log(TAG + ' === 诊断结论 ===');
    var lines = [];

    if (!gridCount) {
      lines.push('【页面不对】没找到 #yy-accounts-grid。请先点顶部「账号」标签再重跑，否则看不到头像问题。');
    } else if (!cardCount) {
      lines.push('【账号没渲染】#yy-accounts-grid 存在但是空的，账号列表还没出来。');
    }

    var emptySrc = imgNodes.filter(function (img) {
      return !img.getAttribute('src') && img.getAttribute('data-skin-media');
    });
    if (emptySrc.length) {
      lines.push('【MutationObserver 没接管】有 ' + emptySrc.length + ' 个 img[data-skin-media] 的 src 仍为空 —— youyuan_skin_media.js 的观察器没补上 blob。');
    }

    var brokenImg = imgNodes.filter(function (img) {
      var s = img.getAttribute('src') || '';
      return /^blob:/.test(s) && img.complete && img.naturalWidth === 0;
    });
    if (brokenImg.length) {
      lines.push('【图解码失败】有 ' + brokenImg.length + ' 个 img 的 src 是 blob 但 naturalWidth=0 —— 取到了字节但不是有效图片。');
    }

    if (stat && stat.skeleton && stat.noImg) {
      lines.push('【markup 没生成 img（url 为空）】有 ' + stat.noImg + ' 个头像容器内部没有 img —— yyAcctAvatarURL 返回空串，说明三层全落空。');
    }

    if (stat && stat.fallback) {
      lines.push('【已回落到默认头】有 ' + stat.fallback + ' 个容器是 yy-avatar-fallback —— 加载失败过，用户看到的是等距三面头占位。');
    }

    var okProbe = probeResults.filter(function (x) {
      return x.res.ok && x.res.status === 200 && /^image\//i.test(x.res.ct);
    });
    if (okProbe.length && (emptySrc.length || (stat && stat.fallback))) {
      lines.push('【服务端有图、前端没接线】account-head 成功 ' + okProbe.length + '/' + probeResults.length +
        ' 个（200 + image/*），但页面没显示 —— 该走 account-head 按 id 取，别靠显示名猜 online-head。');
    }

    if (!lines.length) {
      lines.push('未命中任何已知故障模式。若用户仍看不到头像，请把本段输出和截图一起发回。');
    }

    lines.forEach(function (l) { console.log(TAG + ' ' + l); });
    console.log(TAG + ' === 诊断结束 ===');

    // 汇总一张表，方便一次性复制
    try {
      console.table(probeResults.map(function (x) {
        return {
          '接口': 'account-head?id=' + x.id,
          'status': x.res.ok ? x.res.status : 'ERR',
          'type': x.res.ok ? x.res.ct : '-',
          'bytes': x.res.ok ? x.res.bytes : '-'
        };
      }));
    } catch (e) {}
  }

  // ---------- 主流程 ----------
  step1();
  step3()
    .then(function () { step4(); return step5(); })
    .then(function (stat) {
      return step6().then(function () { return step7(); }).then(function () { return stat; });
    })
    .then(function (stat) {
      step8(stat);
      window.__yyAvatarProbeRunning = false;
      log('完成。可重复运行本脚本。');
    })
    .catch(function (e) {
      console.error(TAG, '脚本异常:', e);
      window.__yyAvatarProbeRunning = false;
    });
})();
