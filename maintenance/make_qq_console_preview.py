#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""make_qq_console_preview.py —— 群服互通机器人面板验收页。

模型不能读图，用真实 CSS + 真实 youyuan_qq_console.js + mock 接口拼一个页面，
页内脚本把度量写进 <pre id="audit">，再用 Chrome --dump-dom 读回来。

用法：python3 make_qq_console_preview.py
输出：preview/qq-console.html
"""
import io, os, base64, struct, zlib

ROOT = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"
FE = ROOT + "/upstream/frontend"
OUT_DIR = ROOT + "/preview"

CSS = [
    "css/base.css", "css/components.css", "css/layout.css", "css/style.css",
    "css/youyuan_design.css", "css/youyuan_qq_console.css", "css/youyuan_usability.css",
]


def png(w, h, rgb):
    raw = b"".join(b"\x00" + bytes(rgb) * w for _ in range(h))
    def chunk(tag, data):
        return struct.pack(">I", len(data)) + tag + data + struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF)
    return (b"\x89PNG\r\n\x1a\n"
            + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(raw, 6)) + chunk(b"IEND", b""))


def data_url(w, h, rgb):
    return "data:image/png;base64," + base64.b64encode(png(w, h, rgb)).decode()


def read(rel):
    with io.open(os.path.join(FE, rel), encoding="utf-8") as f:
        return f.read()


CAPS = [
    ("like", "点赞", "👍", "给群友的名片点赞，每人每天次数有限", 0, "成员", 120, 10, 3, True, ".点赞 QQ号 [次数]"),
    ("emoji_like", "表情回应", "😄", "对消息贴上表情回应，可自定义表情 ID", 0, "成员", 0, 3, 60, True, ".表情 [表情ID]"),
    ("face", "收藏表情", "🖼", "读取机器人 QQ 的收藏表情包", 0, "成员", 0, 0, 0, True, ".表情包"),
    ("qzone", "发动态", "📮", "以机器人身份发 QQ 空间动态", 1, "管理员", 300, 30, 6, True, ".动态 内容"),
    ("avatar", "改头像", "🧑", "更换机器人 QQ 头像", 2, "超级管理员", 600, 30, 10, True, ".改头像 图片URL"),
    ("kick", "踢人", "👢", "把指定群成员移出群", 1, "管理员", 10, 5, 20, False, ".踢 QQ号 [原因]"),
]


def caps_json():
    import json
    rows = []
    for kind, label, icon, desc, lv, lvtxt, cool, gap, daily, default, cmd in CAPS:
        rows.append({"kind": kind, "label": label, "icon": icon, "desc": desc, "min_level": lv,
                     "min_level_text": lvtxt, "cool_seconds": cool, "gap_seconds": gap, "daily": daily,
                     "default": default, "onebot_only": True, "command": cmd, "enabled": default})
    return json.dumps(rows, ensure_ascii=False)


def main():
    os.makedirs(OUT_DIR, exist_ok=True)
    css = "\n".join(read(p) for p in CSS)
    js = read("js/youyuan_qq_console.js")
    avatar = data_url(96, 96, (86, 138, 122))
    face = data_url(88, 88, (198, 161, 101))

    html = """<!DOCTYPE html>
<html lang="zh-CN" data-theme="light"><head><meta charset="utf-8">
<title>群服互通 · 机器人面板验收</title>
<style>__CSS__</style>
<style>body{margin:0;background:var(--phx-bg);font-family:system-ui,-apple-system,"Segoe UI",sans-serif;color:var(--phx-text)}
#host{max-width:1240px;margin:0 auto;padding:24px 28px}
.yy-page-intro h2{margin:0 0 6px;font-size:24px}.yy-page-intro p{margin:0 0 18px;font-size:12px;color:var(--phx-text-secondary)}
.card{background:var(--phx-bg-card);border:1px solid var(--phx-border);border-radius:16px;padding:20px;margin:0}
.yy-form-head h3{margin:0 0 8px;font-size:15px;display:flex;align-items:center;gap:8px}
.yy-badge{font-size:10px;padding:3px 8px;border-radius:6px;border:1px solid var(--phx-border-light);color:var(--phx-text-secondary)}
.yy-hint{font-size:11px;color:var(--phx-text-secondary);line-height:1.7;margin:6px 0 10px}
.yy-actions{display:flex;gap:8px;flex-wrap:wrap;margin-top:12px}
.btn,.btn-s{min-height:38px;padding:0 14px;border-radius:10px;border:1px solid var(--phx-border);background:var(--phx-bg-secondary);color:var(--phx-text);cursor:pointer}
label{display:block;font-size:12px;margin:10px 0 4px;color:var(--phx-text-secondary)}
input,select,textarea{width:100%;box-sizing:border-box;padding:9px 11px;border-radius:10px;border:1px solid var(--phx-border);background:var(--phx-bg);color:var(--phx-text);font-size:13px}
.yy-inline-check{display:flex;align-items:center;gap:8px;font-size:13px;color:var(--phx-text)}
.yy-inline-check input{width:auto}
</style></head><body>
<div id="host"></div>
<pre id="audit" style="position:fixed;left:-9999px;top:0"></pre>
<script>
window.subPageCache={};
window.T=function(){};
window.yyEscape=function(v){return String(v==null?"":v).replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]})};
window.yyIntro=function(t,d){return '<div class="yy-page-intro"><h2>'+t+'</h2><p>'+d+'</p></div>'};
window.openSubPage=function(title,html){document.getElementById('host').innerHTML=html};
var CAPS=__CAPS__;
window.A=function(method,path,body){
  if(path==='/api/qq/config')return Promise.resolve({ok:true,config:{enabled:true,adapter:'onebot',group_id:'1121432132',
    onebot:{ws_url:'ws://127.0.0.1:3001',bot_id:'3199053371',has_token:true},
    qqofficial:{},assistant:{enabled:true,relay_chat:true,bot_player:'布汲岛',prompt:'你是布汲岛的QQ助手，说话简短、先给结论。',model:''}}});
  if(path==='/api/qq/status')return Promise.resolve({ok:true,state:'open',detail:'已连接 NapCat · 群 1121432132'});
  if(path==='/api/qq/caps')return Promise.resolve({ok:true,caps:CAPS});
  if(path==='/api/qq/actions')return Promise.resolve({ok:true,actions:[
    {time:Date.now()-60000,kind:'like',caller:'3199053371',name:'蓝色大肥鱼',target:'123456',detail:'点赞 3 次',ok:true},
    {time:Date.now()-120000,kind:'emoji_like',caller:'3199053371',name:'蓝色大肥鱼',target:'m1',detail:'emoji=128077',ok:true},
    {time:Date.now()-300000,kind:'kick',caller:'3199053371',name:'蓝色大肥鱼',target:'999',detail:'',ok:false,error:'能力「踢人」已在设置里关闭'}]});
  if(path==='/api/qq/bot/profile')return Promise.resolve({ok:true,state:'open',user_id:'3199053371',nickname:'蓝色大肥鱼',avatar:'__AVATAR__',group_count:3,friend_count:42,bot_player:'布汲岛',group_id:'1121432132'});
  if(path==='/api/qq/faces')return Promise.resolve({ok:true,count:6,faces:[{id:'1',url:'__FACE__'},{id:'2',url:'__FACE__'},{id:'3',url:'__FACE__'},{id:'4',url:'__FACE__'},{id:'5',url:'__FACE__'},{id:'6',url:'__FACE__'}]});
  if(path==='/api/qq/groups/available')return Promise.resolve({ok:true,groups:[{group_id:'1121432132',group_name:'服务器游戏交流群',member_count:33},{group_id:'905472215',group_name:'幻星逐梦服务器',member_count:25}]});
  return Promise.resolve({ok:false,error:'no mock for '+path});
};
</script>
<script>__JS__</script>
<script>
function box(el){var r=el.getBoundingClientRect();return {w:Math.round(r.width),h:Math.round(r.height),y:Math.round(r.top)}}
async function run(){
  await yyQQConsolePage();
  await new Promise(function(r){setTimeout(r,120)});
  await yyQQConsoleLoadFaces();
  await new Promise(function(r){setTimeout(r,120)});
  var hero=document.querySelector('.yy-bot-hero');
  var caps=[].map.call(document.querySelectorAll('.yy-cap-card'),function(n){return {kind:n.getAttribute('data-kind'),w:Math.round(n.getBoundingClientRect().width),on:n.classList.contains('on')}});
  var faces=[].map.call(document.querySelectorAll('.yy-face img'),function(n){return Math.round(n.getBoundingClientRect().width)});
  var act=[].map.call(document.querySelectorAll('.yy-act-list li'),function(n){return Math.round(n.getBoundingClientRect().width)});
  var data={
    hero:hero?box(hero):null,
    avatar:hero&&hero.querySelector('.yy-bot-avatar')?box(hero.querySelector('.yy-bot-avatar')):null,
    factCells:[].map.call(document.querySelectorAll('.yy-bot-facts>div'),function(n){return Math.round(n.getBoundingClientRect().width)}),
    tags:[].map.call(document.querySelectorAll('.yy-bot-tag'),function(n){return n.textContent}),
    heroName:hero?hero.querySelector('strong').textContent:null,
    capCount:caps.length, capWidths:caps.map(function(c){return c.w}), capOn:caps.filter(function(c){return c.on}).length,
    faceCount:faces.length, faceWidths:faces.slice(0,3),
    actCount:act.length, actWidths:act,
    gridCols:getComputedStyle(document.querySelector('.yy-qq-console')).gridTemplateColumns,
    hasPrompt:(document.getElementById('yy-qq-prompt')||{}).value,
    groupInput:(document.getElementById('yy-qq-group')||{}).value,
    docOverflow:document.documentElement.scrollWidth>document.documentElement.clientWidth+1,
    viewport:document.documentElement.clientWidth, docWidth:document.documentElement.scrollWidth
  };
  document.getElementById('audit').textContent=JSON.stringify(data);
}
run();
</script>
</body></html>"""
    html = (html.replace("__CSS__", css).replace("__JS__", js)
                .replace("__CAPS__", caps_json())
                .replace("__AVATAR__", avatar).replace("__FACE__", face))
    path = os.path.join(OUT_DIR, "qq-console.html")
    with io.open(path, "w", encoding="utf-8", newline="") as f:
        f.write(html)
    print("wrote", path, len(html), "bytes")


if __name__ == "__main__":
    main()
