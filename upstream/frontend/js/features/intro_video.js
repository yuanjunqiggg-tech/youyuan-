// 开场动画：进入程序先播一次用户提供的片头视频，播完（或点一下）再进工作台。
// 只在每次冷启动播一次（sessionStorage），切标签页/刷新不再打扰。
// 自动播放失败（部分 WebView 静音策略）时立刻跳过，绝不卡住入口。

(function () {
  var KEY = 'yy-intro-played';
  try { if (sessionStorage.getItem(KEY) === '1') return; } catch (_) { return; }

  function mount() {
    var ov = document.createElement('div');
    ov.id = 'yy-intro-overlay';
    ov.setAttribute('role', 'dialog');
    ov.setAttribute('aria-label', '开场动画');
    ov.style.cssText = 'position:fixed;inset:0;z-index:99999;background:#000;display:flex;align-items:center;justify-content:center;overflow:hidden;' +
      'padding:env(safe-area-inset-top) env(safe-area-inset-right) env(safe-area-inset-bottom) env(safe-area-inset-left);';
    var v = document.createElement('video');
    v.src = '/assets/intro.mp4';
    v.muted = true;
    v.defaultMuted = true;
    v.playsInline = true;
    v.setAttribute('muted', '');
    v.setAttribute('playsinline', '');
    v.setAttribute('webkit-playsinline', '');
    v.preload = 'auto';
    v.style.cssText = 'width:100%;height:100%;object-fit:contain;background:#000;';
    var skip = document.createElement('button');
    skip.type = 'button';
    skip.textContent = '跳过';
    skip.style.cssText = 'position:absolute;right:14px;bottom:calc(18px + env(safe-area-inset-bottom));padding:8px 16px;border-radius:18px;' +
      'border:1px solid rgba(255,255,255,.35);background:rgba(0,0,0,.45);color:#fff;font:12px/1.4 system-ui,sans-serif;cursor:pointer;';
    var done = false;
    function finish() {
      if (done) return;
      done = true;
      try { sessionStorage.setItem(KEY, '1'); } catch (_) {}
      try { v.pause(); } catch (_) {}
      ov.style.transition = 'opacity .28s ease';
      ov.style.opacity = '0';
      setTimeout(function () { ov.remove(); }, 300);
    }
    v.addEventListener('ended', finish);
    v.addEventListener('error', finish);
    skip.addEventListener('click', function (e) { e.stopPropagation(); finish(); });
    ov.addEventListener('click', finish);
    ov.appendChild(v);
    ov.appendChild(skip);
    document.body.appendChild(ov);
    var p = v.play();
    if (p && typeof p.catch === 'function') p.catch(function () { finish(); });
    // 保险丝：最长 12 秒，防止个别 WebView 既不报错也不结束。
    setTimeout(finish, 12000);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', mount, { once: true });
  } else {
    mount();
  }
})();
