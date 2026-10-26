/* 游园终端工作台 · 真终端版。
   能力：①游戏指令（/开头）②内置程序（status/chains/plugins/tasks/accounts/connect…）
   ③输入提示（实时建议 + Tab 补全 + 上下键历史）④触屏点击（建议项/快捷条点按执行）。
   只保留一个原则：任何会写游戏的动作必须显式确认（Enter 或点按发送）。 */
(function(){
  'use strict';
  const states=new Map();
  let owner=scope(),view=null;
  function scope(){return typeof yyReadScope==='function'?yyReadScope():String(typeof yyActiveAccount==='undefined'?'main':yyActiveAccount)}
  function state(){
    const key=scope();
    if(owner!==key){allLogs=[];owner=key;view=null;}
    if(!states.has(key)){
      if(states.size>=8)states.delete(states.keys().next().value);
      states.set(key,{draft:'',search:'',kind:'all',follow:true,manualPause:false,busy:false,unread:0,connection:null,history:[],historyIndex:0,historyDraft:'',suggest:[],suggestIndex:-1});
    }
    return states.get(key);
  }
  function field(id){return document.getElementById(id)}
  function current(){return view&&view.scope===scope()&&field('console-log')===view.log}
  function escape(value){return String(value==null?'':value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
  function kind(row){return row.cl==='e'?'error':row.cl==='cmd'?'command':row.cl==='chat'?'chat':row.cl==='o'?'result':'event'}
  const labels={error:'错误',command:'指令',chat:'聊天',result:'结果',event:'事件'};
  // 日志行统一取文本：事件帧的字段并不统一（msg / message / text / line 都出现过），
  // 取不到文本的行直接不入列表，避免只画出「事件」标签、后面却空着内容。
  function rowText(row){if(!row)return '';const v=row.msg!=null?row.msg:(row.message!=null?row.message:(row.text!=null?row.text:(row.line!=null?row.line:'')));return String(v==null?'':v)}
  function hasText(row){return rowText(row).trim()!==''}
  function visibleCount(){let n=0;for(let i=0;i<allLogs.length;i++)if(hasText(allLogs[i]))n++;return n}
  function matches(rows,s){const query=s.search.trim().toLocaleLowerCase();return rows.filter(row=>hasText(row)&&(s.kind==='all'||kind(row)===s.kind)&&(!query||rowText(row).toLocaleLowerCase().includes(query)))}
  function notice(text){const el=field('yy-terminal-notice');if(el)el.textContent=text}
  function controls(){
    if(!current())return;const s=view.state,connection=s.connection;
    const badge=field('yy-terminal-connection');
    badge.dataset.state=connection===null?'unknown':connection.connected?'online':'offline';
    badge.textContent=connection===null?'连接状态待同步':connection.connected?(connection.op?'游戏已连接 · OP':'游戏已连接 · 权限待确认'):'游戏未连接';
    field('yy-terminal-account').textContent=typeof yyAccountName==='function'?yyAccountName(yyActiveAccount):'当前游戏账号';
    field('yy-terminal-follow').textContent=s.follow?'停止滚动':'跟随最新';
    field('yy-terminal-follow').setAttribute('aria-pressed',String(s.follow));
    const latest=field('yy-terminal-latest');latest.hidden=s.follow;latest.textContent=s.unread?'查看最新 · '+s.unread+' 条新日志':'回到最新';
    field('yy-terminal-count').textContent=view.rows.length+' / '+visibleCount()+' 条';
    field('console-send').disabled=s.busy;field('console-send').textContent=s.busy?'发送中…':'发送';
    field('yy-terminal-input-kind').textContent=yytKindLabel(s.draft);
  }
  function renderRow(row){
    const line=document.createElement('div');line.className='yy-terminal-line';line.dataset.kind=kind(row);
    const label=document.createElement('span');label.className='yy-terminal-level';label.textContent=labels[kind(row)];
    const message=document.createElement('span');message.className='yy-terminal-message';
    // The host renderer already escapes text and preserves Minecraft formatting.
    message.innerHTML=typeof renderLogLine==='function'?renderLogLine(rowText(row)):escape(rowText(row));
    line.append(label,message);return line;
  }
  appendConsoleLogs=function(log){
    if(!current()||log!==view.log)return;
    const s=view.state,rows=matches(allLogs,s),previous=view.rows,scroll=log.scrollTop;
    let drop=rows.length?previous.indexOf(rows[0]):previous.length;
    let compatible=drop>=0&&previous.slice(drop).every((row,i)=>rows[i]===row);
    if(view.source!==allLogs)compatible=false;
    if(!compatible){log.replaceChildren();drop=0;view.rows=[];}
    else if(drop){for(let i=0;i<drop;i++)if(log.firstChild)log.firstChild.remove();view.rows=previous.slice(drop);}
    const start=view.rows.length;
    if(log.querySelector('.yy-terminal-empty'))log.replaceChildren();
    for(let i=start;i<rows.length;i++)log.appendChild(renderRow(rows[i]));
    if(!rows.length){const empty=document.createElement('div');empty.className='yy-terminal-empty';empty.textContent=visibleCount()?'没有匹配的日志。试试其他关键词或类型。':'日志会显示在这里。输入 help 查看全部程序，/ 开头是游戏指令。';log.replaceChildren(empty);}
    if(!s.follow&&compatible)s.unread+=Math.max(0,rows.length-start);
    view.rows=rows;view.source=allLogs;_consoleRendered=allLogs.length;
    if(s.follow){log.scrollTop=log.scrollHeight;s.unread=0;}else log.scrollTop=scroll;
    controls();
  };
  function redraw(){if(current())appendConsoleLogs(view.log)}

  /* ================= 程序表：终端里能运行的全部“程序” ================= */
  function out(text,cls){if(text==null||text==='')return;addConsoleLine(String(text),cls||'o')}
  function outLines(list,cls){if(!list)return;for(let i=0;i<list.length;i++)out(list[i],cls)}
  function pad(text,width){text=String(text==null?'':text);let n=0;for(let i=0;i<text.length;i++)n+=text.charCodeAt(i)>127?2:1;let s=text;while(n<width){s+=' ';n++}return s}
  function yyAccountArg(){return typeof yyActiveAccount==='string'?yyActiveAccount:'main'}

  const YYT_PROGRAMS=[
    {name:'help',args:'[程序]',desc:'列出全部程序；help 程序名 看详细用法',group:'系统'},
    {name:'clear',desc:'清空当前终端画面',group:'系统',alias:['cls']},
    {name:'status',desc:'连接状态、游戏RakNet ACK往返与控制接口耗时',group:'系统'},
    {name:'version',desc:'内核版本与就绪状态',group:'系统'},
    {name:'whoami',desc:'当前游戏账号',group:'系统'},
    {name:'ping',desc:'读取游戏RakNet ACK往返与控制接口耗时',group:'系统'},
    {name:'history',desc:'查看本次会话执行过的命令',group:'系统'},
    {name:'date',desc:'本机时间',group:'系统'},
    {name:'echo',args:'<文本>',desc:'原样输出文本',group:'系统'},
    {name:'connect',args:'<服务器号>',desc:'连接租赁服（等同「连接与设置」里的连接）',group:'连接'},
    {name:'disconnect',desc:'断开游戏连接',group:'连接'},
    {name:'say',args:'<消息>',desc:'以机器人身份发聊天',group:'连接'},
    {name:'chains',args:'[list|start|stop|reload] [名称]',desc:'命令链：列出 / 启停 / 重载',group:'程序'},
    {name:'plugins',args:'[list|run|stop] [ID]',desc:'插件：列出 / 运行 / 停止',group:'程序'},
    {name:'tasks',desc:'任务列表（进行中 / 已完成）',group:'程序'},
    {name:'accounts',desc:'账号列表与当前账号',group:'程序'}
  ];
  function yytFind(name){
    for(let i=0;i<YYT_PROGRAMS.length;i++){
      const p=YYT_PROGRAMS[i];
      if(p.name===name)return p;
      if(p.alias&&p.alias.indexOf(name)>=0)return p;
    }
    return null;
  }
  function yytKindLabel(draft){
    const t=String(draft||'').trim();
    if(!t)return '等待输入';
    if(t.charAt(0)==='/')return '游戏指令';
    const first=t.split(/\s+/)[0];
    if(yytFind(first))return '程序 · '+first;
    return '聊天消息';
  }
  /* 解析：支持双引号包参数，如 say "hello world" */
  function yytParse(raw){
    const text=String(raw||'').trim();
    if(!text)return null;
    if(text.charAt(0)==='/')return {game:true,text:text};
    const argv=[];let buf='',quote=null;
    for(let i=0;i<text.length;i++){
      const ch=text.charAt(i);
      if(quote){if(ch===quote)quote=null;else buf+=ch;continue}
      if(ch==='"'||ch==="'"){quote=ch;continue}
      if(/\s/.test(ch)){if(buf){argv.push(buf);buf=''}continue}
      buf+=ch;
    }
    if(buf)argv.push(buf);
    if(!argv.length)return null;
    return {game:false,name:argv[0].toLowerCase(),argv:argv.slice(1),text:text};
  }
  async function yytRun(cmd){
    if(!cmd)return;
    if(cmd.game){addConsoleLine((cmd.text.charAt(0)==='/'?'[指令] ':'[聊天] ')+cmd.text,cmd.text.charAt(0)==='/'?'cmd':'chat');return yytGameConsole(cmd.text)}
    const p=yytFind(cmd.name);
    if(!p){addConsoleLine('[聊天] '+cmd.text,'chat');return yytGameConsole(cmd.text)}
    addConsoleLine('❯ '+cmd.text,'cmd');
    try{await p.run(cmd.argv||[])}
    catch(e){out('[错误] '+((e&&e.message)||'程序执行失败'),'e')}
    controls();
  }
  async function yytGameConsole(text){
    const key=scope(),s=view?view.state:state();
    if(s.busy)return;
    s.busy=true;controls();
    try{
      const r=await A('POST','/api/bot/console',{input:text});
      if(key!==scope())return;
      if(!r||!r.ok){addConsoleLine('[错误] '+((r&&r.error)||'发送失败'),'e');return}
      if(r.output&&r.output.length){for(const line of r.output)addConsoleLine('▸ '+line,'o')}
      else if(r.success===false)addConsoleLine('[失败] 命令执行失败','e');
      else if(r.note)addConsoleLine('ⓘ '+r.note,'o');
    }catch(e){
      if(key===scope()){
        addConsoleLine('[错误] '+((e&&e.message)||'发送失败'),'e');
        notice('未收到明确结果，不会自动重发。');
      }
    }finally{s.busy=false;if(key===scope()&&current())controls()}
  }

  /* ---------- 各程序实现 ---------- */
  const impl={
    help:async(argv)=>{
      if(argv[0]){
        const p=yytFind(argv[0].toLowerCase());
        if(!p){out('没有这个程序：'+argv[0]+'。输入 help 查看全部。','e');return}
        out(p.name+(p.args?' '+p.args:'')+'  —  '+p.desc);
        if(p.alias)out('  别名：'+p.alias.join(' / '));
        out('  分组：'+p.group);
        return;
      }
      out('游园终端 · 可用程序（'+YYT_PROGRAMS.length+'）');
      out('────────────────────────────────────────');
      const groups={};
      YYT_PROGRAMS.forEach(p=>{(groups[p.group]=groups[p.group]||[]).push(p)});
      Object.keys(groups).forEach(g=>{
        out('【'+g+'】','cmd');
        groups[g].forEach(p=>out('  '+pad(p.name+(p.args?' '+p.args:''),30)+p.desc));
      });
      out('────────────────────────────────────────');
      out('提示：/ 开头 = 游戏指令；↑↓ 历史；Tab 补全；点下方建议直接执行。','cmd');
    },
    clear:async()=>{if(typeof window.yyTerminalClear==='function')window.yyTerminalClear()},
    status:async()=>{
      const t0=Date.now();
      const r=await A('GET','/api/bot/status');
      if(!r||!r.ok){out('[错误] '+((r&&r.error)||'读取失败'),'e');return}
      const gameRTT=r.game_rtt_source==='raknet_ack'&&typeof r.game_rtt_ms==='number'&&Number.isFinite(r.game_rtt_ms)?r.game_rtt_ms.toFixed(2)+' ms':'未取得';
      out('连接状态  '+(r.connected?'已连接':'未连接'));
      out('服务器号  '+(r.server||'—'));
      out('权限      '+(r.is_op?'OP 已授权':'未确认 OP'));
      out('发送队列  '+(r.packet_queue_depth!=null?r.packet_queue_depth+' / '+r.packet_queue_capacity:'—'));
      out('游戏网络往返 (RakNet ACK)  '+gameRTT);
      out('控制接口往返  '+(Date.now()-t0)+' ms');
    },
    version:async()=>{
      const r=await A('GET','/health');
      if(!r){out('[错误] 读取失败','e');return}
      out('版本      '+(r.version||'—'));
      out('内核      '+(r.backend||'—'));
      out('平台      '+(r.platform||'—'));
      out('工具数    '+(r.tools||'—'));
      out('就绪      '+(r.ready?'是':'否'));
    },
    whoami:async()=>{
      out('当前账号  '+(typeof yyAccountName==='function'?yyAccountName(yyActiveAccount):yyAccountArg()));
      const r=await A('GET','/api/youyuan/accounts');
      if(r&&r.ok){
        const cur=(r.accounts||[]).filter(a=>a.id===yyAccountArg()||a.active)[0];
        if(cur)out('账号 ID   '+(cur.id||'—'));
      }
    },
    ping:async()=>{
      const t0=Date.now();
      const r=await A('GET','/api/bot/status');
      if(!r||!r.ok){out('[错误] '+((r&&r.error)||'读取失败'),'e');return}
      const gameRTT=r.game_rtt_source==='raknet_ack'&&typeof r.game_rtt_ms==='number'&&Number.isFinite(r.game_rtt_ms)?r.game_rtt_ms.toFixed(2)+' ms':'未取得';
      out('游戏网络往返 (RakNet ACK)  '+gameRTT);
      out('控制接口往返  '+(Date.now()-t0)+' ms');
    },
    history:async()=>{
      const s=view?view.state:state();
      if(!s.history.length){out('还没有执行过命令。');return}
      s.history.forEach((h,i)=>out('  '+pad(String(i+1),4)+h));
    },
    date:async()=>{out(new Date().toLocaleString('zh-CN'))},
    echo:async(argv)=>{out(argv.join(' '))},
    say:async(argv)=>{
      const text=argv.join(' ').trim();
      if(!text){out('用法：say <消息>','e');return}
      addConsoleLine('[聊天] '+text,'chat');
      await yytGameConsole(text);
    },
    connect:async(argv)=>{
      const server=(argv[0]||'').trim();
      if(!server){out('用法：connect <服务器号>，如 connect 98014267','e');return}
      out('正在连接 '+server+' …','cmd');
      const r=await A('POST','/api/bot/connect',{server:server});
      if(!r||!r.ok){out('[失败] '+((r&&r.error)||'连接请求被拒绝')+'。可在「连接与设置」里查看完整参数。','e');return}
      out('连接请求已提交，正在等待服务器握手…');
      setTimeout(async()=>{
        const st=await A('GET','/api/bot/status');
        if(st&&st.ok&&st.connected)out('已连接 · 服务器 '+st.server+(st.is_op?' · OP':''),'o');
        else out('仍在握手，稍后输入 status 查看结果。','cmd');
      },4000);
    },
    disconnect:async()=>{
      const r=await A('POST','/api/bot/disconnect',{});
      out(r&&r.ok?'已断开连接':'[失败] '+((r&&r.error)||'断开失败'),r&&r.ok?'o':'e');
    },
    chains:async(argv)=>{
      const action=(argv[0]||'list').toLowerCase();
      const name=argv.slice(1).join(' ');
      if(action==='list'||action===''){
        const r=await A('GET','/api/youyuan/chains');
        if(!r||!r.ok){out('[错误] '+((r&&r.error)||'读取失败'),'e');return}
        const files=r.files||[];
        if(!files.length){out('还没有命令链。');return}
        out('命令链（'+files.length+'）');
        out('  '+pad('状态',10)+pad('Hz',7)+pad('命令',6)+pad('名称',24));
        files.forEach(f=>{
          const c=f.config||{};
          out('  '+pad(f.state||'—',10)+pad((f.actual_hz!=null?f.actual_hz.toFixed(1):'0'),7)+pad(f.commands||0,6)+(c.name||''));
        });
        return;
      }
      if(!name){out('用法：chains '+action+' <链名称>（先用 chains 查看名称）','e');return}
      if(action==='start'||action==='stop'||action==='reload'){
        const r=await A('POST','/api/youyuan/chains',{action:action,name:name});
        if(!r||!r.ok){out('[失败] '+((r&&r.error)||action+' 失败'),'e');return}
        const label={start:'已启动',stop:'已停止',reload:'已重载'}[action];
        out(label+' · '+name);
        if(action==='start')out('提示：20Hz 大链建议在「命令链」页设置限速，避免命令风暴。','cmd');
        return;
      }
      out('未知子命令：'+action+'（可用 list / start / stop / reload）','e');
    },
    plugins:async(argv)=>{
      const action=(argv[0]||'list').toLowerCase();
      const id=argv[1]||'';
      if(action==='list'){
        const r=await A('GET','/api/plugin/list');
        if(!r||!r.ok){out('[错误] '+((r&&r.error)||'读取失败'),'e');return}
        const list=r.plugins||[];
        if(!list.length){out('还没有插件。');return}
        out('插件（'+list.length+'）');
        list.forEach(p=>out('  '+pad(p.running?'运行中':'已停止',8)+pad(p.disabled?'已禁用':'',8)+(p.name||p.id||'')));
        return;
      }
      if(action==='run'||action==='stop'){
        if(!id){out('用法：plugins '+action+' <插件ID>','e');return}
        const r=await A('POST','/api/plugin/'+action,{id:id});
        out(r&&r.ok?(action==='run'?'已启动 · ':'已停止 · ')+id:'[失败] '+((r&&r.error)||action+' 失败'),r&&r.ok?'o':'e');
        return;
      }
      out('未知子命令：'+action+'（可用 list / run / stop）','e');
    },
    tasks:async()=>{
      const r=await A('GET','/api/task/list');
      if(!r||!r.ok){out('[错误] '+((r&&r.error)||'读取失败'),'e');return}
      const list=r.tasks||[];
      if(!list.length){out('没有任务。');return}
      out('任务（'+list.length+'）');
      list.slice(-15).forEach(t=>out('  '+pad(t.status||'—',10)+pad(Math.round((t.progress||0)*100)+'%',6)+(t.name||t.type||t.id||'')));
    },
    accounts:async()=>{
      const r=await A('GET','/api/youyuan/accounts');
      if(!r||!r.ok){out('[错误] '+((r&&r.error)||'读取失败'),'e');return}
      const list=r.accounts||[];
      if(!list.length){out('没有账号。');return}
      out('账号（'+list.length+'）');
      list.forEach(a=>out('  '+(a.id===yyAccountArg()?'▸ ':'  ')+pad(a.id||'—',16)+(a.name||'')+(a.active?'  · 当前':'')));
    }
  };
  YYT_PROGRAMS.forEach(p=>{p.run=impl[p.name]||impl.help});

  /* ================= 输入提示 + 触屏 ================= */
  function yytSuggestFor(draft){
    const t=String(draft||'').trim();
    if(!t||t.charAt(0)==='/')return [];
    const parts=t.split(/\s+/);
    const head=parts[0].toLowerCase();
    if(parts.length===1){
      return YYT_PROGRAMS.filter(p=>p.name.indexOf(head)===0||(p.alias||[]).some(a=>a.indexOf(head)===0)).slice(0,6).map(p=>({label:p.name,desc:p.desc,fill:p.name+(p.args?' ':'')}));
    }
    const p=yytFind(head);
    if(!p||!p.args)return [];
    const tail=parts[parts.length-1];
    if(head==='chains'){
      const subs=['list','start','stop','reload'].filter(s=>s.indexOf(tail.toLowerCase())===0);
      return subs.map(s=>({label:s,desc:'chains '+s,fill:parts.slice(0,-1).join(' ')+' '+s+' '}));
    }
    if(head==='plugins'){
      const subs=['list','run','stop'].filter(s=>s.indexOf(tail.toLowerCase())===0);
      return subs.map(s=>({label:s,desc:'plugins '+s,fill:parts.slice(0,-1).join(' ')+' '+s+' '}));
    }
    return [];
  }
  function yytRenderSuggest(){
    const box=field('yy-terminal-suggest');if(!box)return;
    const s=view?view.state:state();
    const list=s.suggest||[];
    if(!list.length){box.hidden=true;box.innerHTML='';return}
    box.hidden=false;
    box.innerHTML=list.map((it,i)=>'<button type="button" class="yy-terminal-suggest-item'+(i===s.suggestIndex?' is-active':'')+'" data-fill="'+escape(it.fill)+'"><code>'+escape(it.label)+'</code><span>'+escape(it.desc||'')+'</span></button>').join('');
  }
  function yytRefreshSuggest(){
    const input=field('console-input');if(!input)return;
    const s=view?view.state:state();
    s.suggest=yytSuggestFor(input.value);
    s.suggestIndex=-1;
    yytRenderSuggest();
  }
  function yytComplete(){
    const input=field('console-input');if(!input)return false;
    const s=view?view.state:state();
    if(!s.suggest.length)return false;
    const idx=s.suggestIndex>=0?(s.suggestIndex+1)%s.suggest.length:0;
    s.suggestIndex=idx;
    input.value=s.suggest[idx].fill;
    s.draft=input.value;
    controls();
    yytRefreshSuggest();
    return true;
  }
  function yytPick(fill){
    const input=field('console-input');if(!input)return;
    input.value=fill||'';
    const s=view?view.state:state();
    s.draft=input.value;
    const parsed=yytParse(input.value);
    // 点按程序名：直接执行无参程序；带参程序把光标留在末尾等用户补参数
    if(parsed&&!parsed.game&&yytFind(parsed.name)&&(!parsed.argv||!parsed.argv.length)){
      const p=yytFind(parsed.name);
      if(!p.args){input.value='';s.draft='';s.suggest=[];yytRenderSuggest();yytRun(parsed);controls();return}
    }
    input.focus();
    try{input.setSelectionRange(input.value.length,input.value.length)}catch(_){}
    controls();
    yytRefreshSuggest();
  }
  function yytBindTouch(box){
    if(!box)return;
    // 触屏优先：touchstart 立即响应，避免 300ms 点击延迟；鼠标走 click
    box.addEventListener('touchstart',function(ev){
      const btn=ev.target&&ev.target.closest?ev.target.closest('.yy-terminal-suggest-item'):null;
      if(!btn)return;
      ev.preventDefault();
      yytPick(btn.getAttribute('data-fill'));
    },{passive:false});
    box.addEventListener('click',function(ev){
      const btn=ev.target&&ev.target.closest?ev.target.closest('.yy-terminal-suggest-item'):null;
      if(!btn)return;
      ev.preventDefault();
      yytPick(btn.getAttribute('data-fill'));
    });
  }

  Rconsole=function(){
    const s=state(),host=field('tab-terminal');if(!host)return;
    const composer='<section class="yy-terminal-composer" aria-labelledby="yy-terminal-compose-title"><label for="console-input" class="yy-terminal-prompt" aria-hidden="true">❯</label><label for="console-input" class="yy-terminal-sr-only">游戏指令、终端程序或聊天消息</label><textarea id="console-input" rows="1" placeholder="输入程序或 / 指令…" spellcheck="false" autocomplete="off" onkeydown="yyTerminalKey(event)"></textarea><div class="yy-terminal-send-row"><button type="button" id="console-send" onclick="sendConsole()">发送</button></div><div class="yy-terminal-status-line"><h3 id="yy-terminal-compose-title" class="yy-terminal-sr-only">游戏指令、终端程序与聊天</h3><span id="yy-terminal-input-kind">等待输入</span><p id="yy-terminal-notice" role="status" aria-live="polite"></p><p class="yy-terminal-hint">Enter 执行 · Shift+Enter 换行 · Tab 补全 · ↑↓ 历史</p></div></section>';
    host.innerHTML='<div class="content-inner yy-terminal-workbench">'+
      '<header class="yy-terminal-heading"><span class="yy-terminal-dot"></span><span class="yy-terminal-dot"></span><span class="yy-terminal-dot"></span><span class="yy-terminal-kicker">youyuan@console</span><span class="yy-terminal-sep">·</span><span id="yy-terminal-account"></span><span id="yy-terminal-connection" role="status"></span></header>'+
      '<section class="yy-terminal-board" aria-label="当前账号运行日志"><div class="yy-terminal-board-head"><span id="yy-terminal-count"></span><div class="yy-terminal-actions"><button type="button" id="yy-terminal-copy">复制</button><button type="button" id="yy-terminal-clear">清空</button></div></div>'+
      '<div class="yy-terminal-filters"><label><span>搜索日志</span><input id="yy-terminal-search" type="search" placeholder="搜索日志…" autocomplete="off"></label><label><span>日志类型</span><select id="yy-terminal-kind"><option value="all">全部类型</option><option value="error">错误</option><option value="command">指令</option><option value="chat">聊天</option><option value="result">结果</option><option value="event">事件</option></select></label><button type="button" id="yy-terminal-follow" aria-pressed="true">停止滚动</button></div>'+
      '<div class="console-log yy-terminal-log" id="console-log" role="region" aria-label="当前账号运行日志" tabindex="0"></div><div class="yy-terminal-log-foot"><span>最近 2,000 条 · 停止滚动时仍接收日志</span><button type="button" id="yy-terminal-latest" hidden>回到最新</button></div></section>'+
      '<div class="yy-terminal-quick" id="yy-terminal-quick" aria-label="常用程序"><button type="button" data-fill="help">help</button><button type="button" data-fill="status">status</button><button type="button" data-fill="chains">chains</button><button type="button" data-fill="plugins">plugins</button><button type="button" data-fill="tasks">tasks</button><button type="button" data-fill="ping">ping</button><button type="button" data-fill="clear">clear</button></div>'+
      '<div class="yy-terminal-suggest" id="yy-terminal-suggest" role="listbox" aria-label="命令建议" hidden></div>'+
      composer+'</div>';
    view={scope:scope(),state:s,log:field('console-log'),rows:[],source:null};
    const input=field('console-input');input.value=s.draft;
    input.oninput=()=>{if(current()){s.draft=input.value;s.historyIndex=s.history.length;s.historyDraft=input.value;controls();yytRefreshSuggest()}};
    field('yy-terminal-search').value=s.search;field('yy-terminal-search').oninput=function(){if(current()){s.search=this.value;view.source=null;redraw()}};
    field('yy-terminal-kind').value=s.kind;field('yy-terminal-kind').onchange=function(){if(current()){s.kind=this.value;view.source=null;redraw()}};
    field('yy-terminal-follow').onclick=()=>yyTerminalFollow(!s.follow);
    field('yy-terminal-latest').onclick=()=>yyTerminalFollow(true);
    field('yy-terminal-clear').onclick=yyTerminalClear;
    field('yy-terminal-copy').onclick=yyTerminalCopy;
    view.log.onscroll=()=>{if(!current()||s.manualPause)return;const near=view.log.scrollHeight-view.log.clientHeight-view.log.scrollTop<64;s.follow=near;if(near)s.unread=0;controls()};
    // 快捷条：点按填入并执行无参程序。
    // 只监听 click：以前在 touchstart 里 preventDefault 会吃掉横向滑动手势，
    // 快捷项一多就滑不到后面的按钮。交给浏览器合成 click 即可，触屏点按同样即时。
    const quick=field('yy-terminal-quick');
    if(quick){
      quick.addEventListener('click',function(ev){
        const btn=ev.target&&ev.target.closest?ev.target.closest('button[data-fill]'):null;
        if(!btn)return;ev.preventDefault();yytPick(btn.getAttribute('data-fill'));
      });
    }
    yytBindTouch(field('yy-terminal-suggest'));
    // 软键盘适配：键盘弹出时把输入区顶到可视区域内
    if(typeof window.visualViewport!=='undefined'&&window.visualViewport){
      const vv=window.visualViewport;
      const adjust=function(){
        const wb=document.querySelector('#tab-terminal .yy-terminal-workbench');
        if(!wb)return;
        const gap=Math.max(0,window.innerHeight-(vv.height+(vv.offsetTop||0)));
        wb.style.paddingBottom=gap>80?Math.min(gap,340)+'px':'';
      };
      vv.addEventListener('resize',adjust);
      vv.addEventListener('scroll',adjust);
    }
    redraw();
  };
  window.yyTerminalFollow=function(on){if(!current())return;const s=view.state;s.follow=!!on;s.manualPause=!on;if(on){s.unread=0;view.log.scrollTop=view.log.scrollHeight;}controls()};
  window.yyTerminalClear=function(){if(!current())return;allLogs=[];view.state.unread=0;view.state.suggest=[];yytRenderSuggest();redraw();notice('已清空当前账号在此页面保留的本地日志。')};
  window.yyTerminalCopy=async function(){
    if(!current())return;const key=scope(),s=view.state,text=matches(allLogs,s).map(row=>'['+labels[kind(row)]+'] '+rowText(row)).join('\n');
    if(!text){notice('没有可复制的筛选结果。');return}
    try{await navigator.clipboard.writeText(text);if(key===scope())notice('已复制筛选结果。')}catch(_){if(key===scope())notice('未能访问剪贴板，请在日志中选择文字复制。')}
  };
  window.yyTerminalKey=function(event){
    if(event.isComposing||event.keyCode===229)return;
    const input=field('console-input');
    if(current()&&input){
      const s=view.state;
      if(event.key==='Tab'){if(yytComplete()){event.preventDefault();return}}
      if((event.key==='ArrowUp'||event.key==='ArrowDown')&&!event.shiftKey&&!event.ctrlKey&&!event.metaKey&&!event.altKey){
        // 有建议时上下键在建议间移动
        if(s.suggest&&s.suggest.length&&!input.value.includes('\n')){
          event.preventDefault();
          const n=s.suggest.length;
          s.suggestIndex=event.key==='ArrowUp'?(s.suggestIndex<=0?n-1:s.suggestIndex-1):(s.suggestIndex>=n-1?0:s.suggestIndex+1);
          yytRenderSuggest();return;
        }
        if(!input.value.includes('\n')&&s.history.length){
          event.preventDefault();
          if(s.historyIndex===s.history.length)s.historyDraft=input.value;
          s.historyIndex=Math.max(0,Math.min(s.history.length,s.historyIndex+(event.key==='ArrowUp'?-1:1)));
          input.value=s.historyIndex===s.history.length?s.historyDraft:s.history[s.historyIndex];
          s.draft=input.value;controls();return;
        }
      }
    }
    if(event.key!=='Enter'||event.shiftKey)return;
    event.preventDefault();sendConsole();
  };
  sendConsole=async function(){
    if(!current())return;
    const s=view.state,input=field('console-input'),key=scope(),text=input.value.trim();
    if(!text||s.busy)return;
    if((s.history[s.history.length-1]!==text))s.history.push(text);
    if(s.history.length>40)s.history.shift();
    s.historyIndex=s.history.length;s.historyDraft='';s.draft='';
    input.value='';s.suggest=[];s.suggestIndex=-1;yytRenderSuggest();
    controls();
    await yytRun(yytParse(text));
    if(key===scope()){s.draft='';controls()}
  };
  // Keep the existing SSE and its bounded shared buffer. Neither navigation nor
  // filtering owns or reconnects the event stream.
  const baseAddConsoleLine=addConsoleLine;
  addConsoleLine=function(message,level){state();return baseAddConsoleLine(message,level)};
  const baseStatus=updateConnStatus;
  updateConnStatus=function(connected,op,server,target){baseStatus(connected,op,server,target);state().connection=connected==null?null:{connected:!!connected,op:!!op};controls()};
  const baseTabTo=tabTo;
  tabTo=function(tab){baseTabTo(tab);if(tab==='terminal'){if(!current())Rconsole();else redraw();}};
  document.addEventListener('yy-identity-ready',()=>{states.clear();allLogs=[];owner=scope();view=null;if(typeof tabInited!=='undefined')tabInited.terminal=false;});
})();
