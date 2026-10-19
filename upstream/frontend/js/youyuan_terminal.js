/* 游园终端工作台。Only the explicit send action writes to the game. */
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
      states.set(key,{draft:'',search:'',kind:'all',follow:true,manualPause:false,busy:false,unread:0,connection:null,history:[],historyIndex:0,historyDraft:''});
    }
    return states.get(key);
  }
  function field(id){return document.getElementById(id)}
  function current(){return view&&view.scope===scope()&&field('console-log')===view.log}
  function escape(value){return String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
  function kind(row){return row.cl==='e'?'error':row.cl==='cmd'?'command':row.cl==='chat'?'chat':row.cl==='o'?'result':'event'}
  const labels={error:'错误',command:'指令',chat:'聊天',result:'结果',event:'事件'};
  function matches(rows,s){const query=s.search.trim().toLocaleLowerCase();return rows.filter(row=>(s.kind==='all'||kind(row)===s.kind)&&(!query||String(row.msg??'').toLocaleLowerCase().includes(query)))}
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
    field('yy-terminal-count').textContent=view.rows.length+' / '+allLogs.length+' 条';
    field('console-send').disabled=s.busy;field('console-send').textContent=s.busy?'发送中…':'发送';
    field('yy-terminal-input-kind').textContent=s.draft.trim().startsWith('/')?'游戏指令':'聊天消息';
  }
  function renderRow(row){
    const line=document.createElement('div');line.className='yy-terminal-line';line.dataset.kind=kind(row);
    const label=document.createElement('span');label.className='yy-terminal-level';label.textContent=labels[kind(row)];
    const message=document.createElement('span');message.className='yy-terminal-message';
    // The host renderer already escapes text and preserves Minecraft formatting.
    message.innerHTML=typeof renderLogLine==='function'?renderLogLine(String(row.msg??'')):escape(row.msg);
    line.append(label,message);return line;
  }
  appendConsoleLogs=function(log){
    if(!current()||log!==view.log)return;
    const s=view.state,rows=matches(allLogs,s),previous=view.rows,scroll=log.scrollTop;
    let drop=rows.length?previous.indexOf(rows[0]):previous.length;
    let compatible=drop>=0&&previous.slice(drop).every((row,i)=>rows[i]===row);
    if(view.source!==allLogs)compatible=false;
    if(!compatible){log.replaceChildren();drop=0;view.rows=[];}
    else if(drop){for(let i=0;i<drop;i++)log.firstChild?.remove();view.rows=previous.slice(drop);}
    const start=view.rows.length;
    if(log.querySelector('.yy-terminal-empty'))log.replaceChildren();
    for(let i=start;i<rows.length;i++)log.appendChild(renderRow(rows[i]));
    if(!rows.length){const empty=document.createElement('div');empty.className='yy-terminal-empty';empty.textContent=allLogs.length?'没有匹配的日志。试试其他关键词或类型。':'日志会显示在这里。可以先查看状态，准备好后再发送消息或指令。';log.replaceChildren(empty);}
    if(!s.follow&&compatible)s.unread+=Math.max(0,rows.length-start);
    view.rows=rows;view.source=allLogs;_consoleRendered=allLogs.length;
    if(s.follow){log.scrollTop=log.scrollHeight;s.unread=0;}else log.scrollTop=scroll;
    controls();
  };
  function redraw(){if(current())appendConsoleLogs(view.log)}
  Rconsole=function(){
    const s=state(),host=field('tab-terminal');if(!host)return;
    const composer='<section class="yy-terminal-composer" aria-labelledby="yy-terminal-compose-title"><div class="yy-terminal-compose-head"><h3 id="yy-terminal-compose-title">游戏指令与聊天</h3><span id="yy-terminal-input-kind">聊天消息</span></div><label for="console-input" class="yy-terminal-prompt" aria-hidden="true">❯</label><label for="console-input" class="yy-terminal-sr-only">消息或斜杠开头的游戏指令</label><textarea id="console-input" rows="1" placeholder="输入 / 指令或聊天消息…" spellcheck="false" autocomplete="off" onkeydown="yyTerminalKey(event)"></textarea><div class="yy-terminal-send-row"><p>Enter 发送 · Shift + Enter 换行 · ↑ ↓ 历史<br>当前账号执行，指令需要相应游戏权限。</p><button type="button" id="console-send" onclick="sendConsole()">发送</button></div><p id="yy-terminal-notice" role="status" aria-live="polite"></p></section>';
    host.innerHTML='<div class="content-inner yy-terminal-workbench">'+
      '<header class="yy-terminal-heading"><div><span class="yy-terminal-kicker">YOUYUAN / CONSOLE</span><h2>终端</h2><p>实时日志与游戏指令，在同一处完成。</p></div><div class="yy-terminal-session"><span id="yy-terminal-account"></span><span id="yy-terminal-connection" role="status"></span></div></header>'+
      '<section class="yy-terminal-board" aria-labelledby="yy-terminal-log-title"><div class="yy-terminal-board-head"><div><h3 id="yy-terminal-log-title">运行日志</h3><span id="yy-terminal-count"></span></div><div class="yy-terminal-actions"><button type="button" id="yy-terminal-copy">复制筛选结果</button><button type="button" id="yy-terminal-clear">清空本地日志</button></div></div>'+
      '<div class="yy-terminal-filters"><label><span>搜索日志</span><input id="yy-terminal-search" type="search" placeholder="输入玩家、事件或关键词" autocomplete="off"></label><label><span>日志类型</span><select id="yy-terminal-kind"><option value="all">全部类型</option><option value="error">错误</option><option value="command">指令</option><option value="chat">聊天</option><option value="result">结果</option><option value="event">事件</option></select></label><button type="button" id="yy-terminal-follow" aria-pressed="true">停止滚动</button></div>'+
      '<div class="console-log yy-terminal-log" id="console-log" role="region" aria-label="当前账号运行日志" tabindex="0"></div><div class="yy-terminal-log-foot"><span>最近 2,000 条 · 停止滚动时仍接收日志</span><button type="button" id="yy-terminal-latest" hidden>回到最新</button></div></section>'+
      composer+'</div>';
    view={scope:scope(),state:s,log:field('console-log'),rows:[],source:null};
    const input=field('console-input');input.value=s.draft;input.oninput=()=>{if(current()){s.draft=input.value;s.historyIndex=s.history.length;s.historyDraft=input.value;controls()}};
    field('yy-terminal-search').value=s.search;field('yy-terminal-search').oninput=function(){if(current()){s.search=this.value;view.source=null;redraw()}};
    field('yy-terminal-kind').value=s.kind;field('yy-terminal-kind').onchange=function(){if(current()){s.kind=this.value;view.source=null;redraw()}};
    field('yy-terminal-follow').onclick=()=>yyTerminalFollow(!s.follow);
    field('yy-terminal-latest').onclick=()=>yyTerminalFollow(true);
    field('yy-terminal-clear').onclick=yyTerminalClear;
    field('yy-terminal-copy').onclick=yyTerminalCopy;
    view.log.onscroll=()=>{if(!current()||s.manualPause)return;const near=view.log.scrollHeight-view.log.clientHeight-view.log.scrollTop<64;s.follow=near;if(near)s.unread=0;controls()};
    redraw();
  };
  window.yyTerminalFollow=function(on){if(!current())return;const s=view.state;s.follow=!!on;s.manualPause=!on;if(on){s.unread=0;view.log.scrollTop=view.log.scrollHeight;}controls()};
  window.yyTerminalClear=function(){if(!current())return;allLogs=[];view.state.unread=0;redraw();notice('已清空当前账号在此页面保留的本地日志。')};
  window.yyTerminalCopy=async function(){
    if(!current())return;const key=scope(),s=view.state,text=matches(allLogs,s).map(row=>'['+labels[kind(row)]+'] '+String(row.msg??'')).join('\n');
    if(!text){notice('没有可复制的筛选结果。');return}
    try{await navigator.clipboard.writeText(text);if(key===scope())notice('已复制筛选结果。')}catch(_){if(key===scope())notice('未能访问剪贴板，请在日志中选择文字复制。')}
  };
  window.yyTerminalKey=function(event){
    if(event.isComposing||event.keyCode===229)return;
    if(current()&&(event.key==='ArrowUp'||event.key==='ArrowDown')&&!event.shiftKey&&!event.ctrlKey&&!event.metaKey&&!event.altKey){const input=field('console-input'),s=view.state;if(!input.value.includes('\n')&&s.history.length){event.preventDefault();if(s.historyIndex===s.history.length)s.historyDraft=input.value;s.historyIndex=Math.max(0,Math.min(s.history.length,s.historyIndex+(event.key==='ArrowUp'?-1:1)));input.value=s.historyIndex===s.history.length?s.historyDraft:s.history[s.historyIndex];s.draft=input.value;controls();return}}
    if(event.key!=='Enter'||event.shiftKey)return;event.preventDefault();sendConsole();
  };
  sendConsole=async function(){
    if(!current())return;const s=view.state,input=field('console-input'),key=scope(),text=input.value.trim();
    if(!text||s.busy)return;if(s.history.at(-1)!==text)s.history.push(text);if(s.history.length>40)s.history.shift();s.historyIndex=s.history.length;s.historyDraft='';s.busy=true;s.draft='';input.value='';controls();notice('正在等待游戏处理…');
    addConsoleLine((text.startsWith('/')?'[指令] ':'[聊天] ')+text,text.startsWith('/')?'cmd':'chat');
    try{
      const result=await A('POST','/api/bot/console',{input:text});
      if(key!==scope())return;
      if(!result?.ok){addConsoleLine('[错误] '+(result?.error||'发送失败'),'e');notice('发送未完成。请查看错误；不会自动重发。');return}
      if(result.output?.length)for(const line of result.output)addConsoleLine('▸ '+line,'o');
      else if(result.success===false)addConsoleLine('[失败] 命令执行失败','e');
      else if(result.note)addConsoleLine('ⓘ '+result.note,'o');
      notice(result.success===false?'游戏返回执行失败，请查看日志。':'已收到处理结果。');
    }catch(error){if(key===scope()){addConsoleLine('[错误] '+(error.message||'发送失败'),'e');notice('未收到明确结果，不会自动重发。')}}
    finally{s.busy=false;if(key===scope())controls();}
  };
  // Keep the existing SSE and its bounded shared buffer. Neither navigation nor
  // filtering owns or reconnects the event stream.
  const baseAddConsoleLine=addConsoleLine;
  addConsoleLine=function(message,level){state();return baseAddConsoleLine(message,level)};
  const baseStatus=updateConnStatus;
  updateConnStatus=function(connected,op,server){baseStatus(connected,op,server);state().connection=connected==null?null:{connected:!!connected,op:!!op};controls()};
  const baseTabTo=tabTo;
  tabTo=function(tab){baseTabTo(tab);if(tab==='terminal'){if(!current())Rconsole();else redraw();}};
  document.addEventListener('yy-identity-ready',()=>{states.clear();allLogs=[];owner=scope();view=null;if(typeof tabInited!=='undefined')tabInited.terminal=false;});
})();
