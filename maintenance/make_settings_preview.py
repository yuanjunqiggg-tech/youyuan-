#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""make_settings_preview.py —— 设置页「当前服务器 hero」版式验收页。

模型不能读图，所以用真实 CSS + 真实 youyuan_room_discovery.js 拼一个最小页面，
跑完后由页内脚本把度量写进 <pre id="audit">，再用 Chrome --dump-dom 读回来。

用法：python3 make_settings_preview.py
输出：preview/settings-hero.html
"""
import io, os, base64, struct, zlib

ROOT = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"
FE = ROOT + "/upstream/frontend"
OUT_DIR = ROOT + "/preview"

CSS = [
    "css/base.css", "css/components.css", "css/layout.css", "css/style.css",
    "css/youyuan_design.css", "css/youyuan_connection_policy.css",
    "css/youyuan_room_discovery.css", "css/youyuan_navigation.css",
]


def png(w, h, rgb):
    raw = b"".join(b"\x00" + bytes(rgb) * w for _ in range(h))
    def chunk(tag, data):
        c = struct.pack(">I", len(data)) + tag + data
        return c + struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF)
    return (b"\x89PNG\r\n\x1a\n"
            + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(raw, 6))
            + chunk(b"IEND", b""))


def data_url(w, h, rgb):
    return "data:image/png;base64," + base64.b64encode(png(w, h, rgb)).decode()


def read(rel):
    with io.open(os.path.join(FE, rel), encoding="utf-8") as f:
        return f.read()


def main():
    os.makedirs(OUT_DIR, exist_ok=True)
    css = "\n".join(read(p) for p in CSS)
    room_js = read("js/youyuan_room_discovery.js")
    cover = data_url(640, 360, (56, 92, 138))
    avatar = data_url(64, 64, (198, 161, 92))

    html = """<!DOCTYPE html>
<html lang="zh-CN" data-theme="light"><head><meta charset="utf-8">
<title>设置页 · 当前服务器 hero 验收</title>
<style>__CSS__</style>
<style>
/* 仅预览用：模拟应用外壳 */
body{margin:0;background:var(--phx-bg);font-family:system-ui,-apple-system,"Segoe UI",sans-serif;color:var(--phx-text)}
.content-inner{max-width:1440px;margin:0 auto;padding:24px 32px}
.yy-heading{margin:0 0 18px}
.yy-heading h2{margin:0;font-size:26px}
</style>
</head><body>
<div id="tab-settings"><div class="content-inner">
  <div id="yy-player-shortcuts" class="yy-player-shortcuts">
    <button class="yy-secondary">打开终端</button>
    <button class="yy-secondary">角色等级与经验</button>
  </div>
  <div class="yy-heading"><h2>账号与设置</h2></div>
  <section id="yy-connection-server-info" class="yy-server-hero" aria-label="当前服务器">
    <div id="yy-connection-server-card" class="yy-hero-host"></div>
    <div class="yy-server-hero-foot">
      <p id="yy-connection-policy-state" class="yy-hint yy-server-state" role="status">游戏已连接 · 自动重连开启</p>
      <p id="yy-connection-server-note" class="yy-hint yy-server-note" role="status">更新于 23:04:11</p>
      <div class="yy-room-current-actions">
        <button id="yy-resume-reconnect" class="btn-s" hidden>恢复自动重连</button>
        <button type="button" class="yy-text-btn">更新信息</button>
        <label class="yy-room-refresh"><span id="yy-server-refresh-label">在线玩家 · 每 60 秒更新</span>
        <input id="yy-server-refresh-seconds" type="range" min="30" max="60" step="1" value="60"></label>
      </div>
    </div>
  </section>
  <div class="yy-form-grid">
    <div class="yy-form-stack">
      <section class="card"><div class="yy-form-head"><h3>服务器连接</h3><span class="yy-badge">已连接</span></div>
      <label for="bot-server">服务器号 / 分享链接</label><input id="bot-server" value="98014267">
      <div class="yy-actions"><button class="btn">连接服务器</button><button class="btn-s">断开连接</button></div></section>
      <section class="card"><div class="yy-form-head"><h3>多机器人作业</h3></div>
      <div class="yy-actions"><button class="btn-s">跨账号联通</button></div></section>
    </div>
    <div class="yy-form-stack">
      <section class="card"><div class="yy-form-head"><h3>公网远控与 MCP</h3></div>
      <div class="yy-actions"><button class="btn-s">连接配置与密钥</button><button class="btn-s">MCP 专用连接</button></div></section>
      <section class="card"><div class="yy-actions yy-actions-four" style="margin:0">
        <button class="btn-s">工单反馈</button><button class="btn-s">全部 API</button>
        <button class="btn-s">检测更新</button><button class="btn-s">修改平台密码</button></div></section>
    </div>
  </div>
</div></div>
<pre id="audit" style="position:fixed;left:-9999px;top:0"></pre>
<script>
window.A=function(){return Promise.resolve({ok:false})};
window.T=function(){};
window.Rcfg=function(){};
window.refreshCfgAccount=function(){};
window.doConn=function(){};
window.doDisc=function(){};
window.curServerUrl='https://susu.bot.cd';
window.yyActiveAccount='preview';
window.yySettingsData={token:'t',auth_url:'https://susu.bot.cd'};
window.yySettingsViewGuard=function(){return function(){return true}};
window.activeRenderers=[];
</script>
<script>__ROOMJS__</script>
<script>
var cand={id:'rental:1',type:'rental',room_id:'1',target:'98014267',number:'98014267',
  name:'\\u5E03\\u6C72\\u5C9B \\u00B7 \\u6D77\\u5CB8\\u5EFA\\u7B51\\u670D',
  owner:'\\u5E03\\u6C72\\u5C9B',owner_id:'a1b2c3d4e5f6a7b8',
  cover_url:'__COVER__',avatar_url:'__AVATAR__',
  players:7,max_players:40,version:'1.21.0'};
window.yyRoomDiscovery.renderCurrent(cand);
// renderCurrent 的 image() 只接受 https 图；预览用 data URL，这里补齐同样的 DOM 再量版式
(function(){
  var card=document.querySelector('.yy-hero-card');
  if(card&&!card.querySelector('.yy-hero-cover')){
    card.classList.remove('yy-hero-card-plain');
    var img=document.createElement('img');img.className='yy-hero-cover';img.src='__COVER__';
    card.insertBefore(img,card.firstChild);
  }
  var title=document.querySelector('.yy-hero-title');
  if(title&&!title.querySelector('.yy-hero-avatar')){
    var av=document.createElement('img');av.className='yy-hero-avatar';av.src='__AVATAR__';
    title.insertBefore(av,title.firstChild);
  }
})();
function box(el){var r=el.getBoundingClientRect();return {w:Math.round(r.width),h:Math.round(r.height),x:Math.round(r.left),y:Math.round(r.top)}}
function run(){
  var hero=document.getElementById('yy-connection-server-info');
  var card=document.querySelector('.yy-hero-card');
  var cover=document.querySelector('.yy-hero-cover');
  var avatar=document.querySelector('.yy-hero-avatar');
  var facts=document.querySelector('.yy-hero-facts');
  var cells=[].map.call(document.querySelectorAll('.yy-hero-fact'),function(n){return Math.round(n.getBoundingClientRect().width)});
  var four=document.querySelector('.yy-actions-four');
  var btns=[].map.call(four.querySelectorAll('button'),function(n){return Math.round(n.getBoundingClientRect().width)});
  var heroBox=box(hero),cardBox=box(card);
  var foot=document.querySelector('.yy-server-hero-foot');
  var footKids=[].map.call(foot.children,function(n){return {cls:n.className,h:Math.round(n.getBoundingClientRect().height),y:Math.round(n.getBoundingClientRect().top)}});
  var data={
    hero:heroBox,
    card:cardBox,
    cover:cover?box(cover):null,
    avatar:avatar?box(avatar):null,
    factsRowWidth:facts?Math.round(facts.getBoundingClientRect().width):null,
    factCells:cells,
    fourButtons:btns,
    fourColumns:four?getComputedStyle(four).gridTemplateColumns:null,
    heroTexts:[].map.call(document.querySelectorAll('.yy-hero-title-text strong,.yy-hero-tag,.yy-hero-fact dt,.yy-hero-fact dd'),function(n){return n.textContent}),
    coverIsLeftOfContent:cover?cover.getBoundingClientRect().right<=document.querySelector('.yy-hero-main').getBoundingClientRect().left+1:null,
    docOverflow:document.documentElement.scrollWidth>document.documentElement.clientWidth+1,
    foot:{h:Math.round(foot.getBoundingClientRect().height),kids:footKids},
    heroOverflow:hero.scrollWidth>hero.clientWidth+1,
    cardOverflow:card.scrollWidth>card.clientWidth+1,
    viewport:document.documentElement.clientWidth,
    docWidth:document.documentElement.scrollWidth
  };
  document.getElementById('audit').textContent=JSON.stringify(data);
}
setTimeout(run,300);
</script>
</body></html>"""

    html = (html.replace("__CSS__", css)
                .replace("__ROOMJS__", room_js)
                .replace("__COVER__", cover)
                .replace("__AVATAR__", avatar))
    path = os.path.join(OUT_DIR, "settings-hero.html")
    with io.open(path, "w", encoding="utf-8", newline="") as f:
        f.write(html)
    print("wrote", path, len(html), "bytes")


if __name__ == "__main__":
    main()
