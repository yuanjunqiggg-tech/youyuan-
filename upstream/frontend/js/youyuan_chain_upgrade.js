(function(){
  var originalEdit=yyChainEdit;
  yyChainEdit=async function(name,content){
    await originalEdit(name,content);
    var row=yyChainRows.find(function(item){return item.config.name===name});
    var cfg=row?row.config:{};
    var account=document.getElementById('yy-chain-account');
    if(!account)return;
    var panel=account.closest('section');
    var interval=document.getElementById('yy-chain-interval');
    var intervalLabel=document.querySelector('label[for="yy-chain-interval"]');
    if(interval){interval.type='hidden';if(intervalLabel)intervalLabel.style.display='none';interval.insertAdjacentHTML('beforebegin','<label for="yy-chain-hz">整条链目标频率 · Hz</label><input id="yy-chain-hz" type="number" min="0.001" max="1000" step="any"><p class="yy-hint">一次执行整个文件为一轮。20 Hz 表示每秒启动20轮；一轮超时则立即继续下一轮，不额外加周期，也不积压轮次。</p>');var hz=document.getElementById('yy-chain-hz');hz.value=1000/(cfg.interval_ms||1000);hz.oninput=function(){if(Number(this.value)>0)interval.value=Math.max(1,Math.round(1000/Number(this.value)))};}

    var hint=panel.querySelector('.yy-hint');
    hint.insertAdjacentHTML('beforebegin','<label for="yy-chain-channel">默认命令通道</label><select id="yy-chain-channel"><option value="ai">魔法指令 · 自动化</option><option value="console">控制台 · 自动化</option><option value="player">玩家 · 玩家身份</option></select><p class="yy-hint">#组 可覆盖通道、速率、前缀与次数；#测试 和 #执行 可展开参数模板。组与链都在保存时预解析，运行时不重复解析文件。</p>');
    document.getElementById('yy-chain-channel').value=cfg.channel||'ai';
    hint.insertAdjacentHTML('beforebegin','<label for="yy-chain-mode">机器人执行方式</label><select id="yy-chain-mode"><option value="fast">快速提交 · 仅真实条件等待结果</option><option value="remote">严格顺序 · 每条确认后继续</option><option value="pipeline">有界流水线 · 无依赖命令批量提交</option><option value="stream">动态滑动窗口 · 回包立即补位</option><option value="burst">整批确认 · 分段提交</option></select><label for="yy-chain-window">确认方式的在途窗口 · 快速模式不使用</label><input id="yy-chain-window" type="number" min="1" max="1024"><p class="yy-hint">快速提交不逐条等回包；魔法通道保留机器人身份与相对坐标。有条件命令及它的前一条用真实回包判断。全部在机器人内运行，不放置命令方块。快速模式不设1024条分段；确认模式窗口不限制TXT总行数；默认无CPS等待。不保证同一服务器Tick，快速模式计数表示提交，游戏结果需核验。</p>');
    document.getElementById('yy-chain-mode').value=['fast','pipeline','burst','stream'].includes(cfg.mode)?cfg.mode:(row?'remote':'fast');
    document.getElementById('yy-chain-window').value=cfg.window||(cfg.mode==='burst'?1024:cfg.mode==='stream'?128:8);document.getElementById('yy-chain-mode').onchange=function(){document.getElementById('yy-chain-window').value=this.value==='burst'?1024:this.value==='stream'?128:8;document.getElementById('yy-chain-window').style.display=this.value==='fast'?'none':'';document.querySelector('label[for="yy-chain-window"]').style.display=this.value==='fast'?'none':''};document.getElementById('yy-chain-window').style.display=document.getElementById('yy-chain-mode').value==='fast'?'none':'';document.querySelector('label[for="yy-chain-window"]').style.display=document.getElementById('yy-chain-mode').value==='fast'?'none':'';
    hint.insertAdjacentHTML('beforebegin','<label for="yy-chain-rate">可选发送上限 · 0 不限速</label><input id="yy-chain-rate" type="number" min="0" max="20000" value="0"><p class="yy-hint">默认0，不加人为CPS等待；只有主动设置正数时限速。窗口只控制未确认请求的数量，不限制整个文件的行数。快速模式整条无依赖片段连续提交；确认模式才使用窗口。未匹配到目标不会中断后续无条件命令。断线、超时停止新发送；有条件、控制点与延迟才等待结果。频率按整轮启动计算，轮次不重叠。</p>');
    var modeSelect=document.getElementById('yy-chain-mode'), oldChange=modeSelect.onchange;modeSelect.onchange=function(){oldChange.call(this);document.getElementById('yy-chain-rate').value=0};
    document.getElementById('yy-chain-rate').value=cfg.rate_cps||0;
    yyChainControlPanel(panel);
    var save=panel.parentElement.querySelector('button[onclick="yyChainSave()"]');
    if(save)save.insertAdjacentHTML('afterend','<button class="btn-s" onclick="yyChainPreview()">预览解析与通道</button>');
    var result=document.getElementById('yy-chain-result');
    result.insertAdjacentHTML('beforebegin','<details class="yy-hint"><summary>函数语法与常驻链示例</summary><pre>#组:每日欢迎 通道=1 速率=200 次数=1\n1,0,0\nsay 开始\n0,1,0\ngive @a stone 1\n.\n#组:巡检 通道=0\n2,0,0 20\ntestfor @a\n!\n#测试:提示 名字:≤玩家≥\nsay 欢迎 ≤名字≥\n#执行:提示 名字=Steve</pre><p>0 连锁、1 脉冲、2 循环；第二位控制条件，第三位保留旧格式，运行时忽略红石门控。. 新建独立链，! 结束周期。玩家通道按玩家权限执行；魔法指令与控制台使用自动化命令来源。</p></details>');
  };
  window.yyChainPreview=async function(){
    var result=document.getElementById('yy-chain-result');
    result.textContent='正在预解析…';
    var config;try{config=yyChainExecutionConfig()}catch(e){result.textContent=e.message;return}
    var response=await A('POST','/api/youyuan/chains',{action:'preview',name:document.getElementById('yy-chain-name').value,content:yyChainEditor.state.doc.toString(),config:config});
    if(!response.ok){result.textContent=response.error;return}
    var plan=response.plan;
    result.textContent='共 '+plan.commands+' 条命令、'+plan.chains.length+' 条独立链、'+plan.periods+' 个周期\n'+plan.chains.map(function(chain,i){return '链 '+(i+1)+' · '+(chain.repeat||1)+' 次 · '+chain.nodes.map(function(node){return '['+(node.group||'默认')+'/'+(node.channel||document.getElementById('yy-chain-channel').value)+'] '+node.command}).join(' → ')}).join('\n');
  };
  yyChainSave=async function(){
    var name=document.getElementById('yy-chain-name').value;
    var row=yyChainRows.find(function(item){return item.config.name===name});
    var config;try{config=yyChainExecutionConfig()}catch(e){document.getElementById('yy-chain-result').textContent=e.message;return}
    config.enabled=!!(row&&row.config.enabled);
    var result=await A('POST','/api/youyuan/chains',{action:'save',name:name,content:yyChainEditor.state.doc.toString(),config:config});
    document.getElementById('yy-chain-result').textContent=result.ok?'文件已保存并预解析，可返回管理页启动。':result.error;
    if(result.ok)await yyChainsRefresh();
  };
  window.yyChainExecutionConfig=function(){
    var mode=document.getElementById('yy-chain-mode').value,windowSize=Number(document.getElementById('yy-chain-window').value);
    if(!Number.isInteger(windowSize)||windowSize<1||windowSize>(['fast','burst','stream'].includes(mode)?1024:16))throw Error('流水线1～16条；整批1～1024条');
    var rate=Number(document.getElementById('yy-chain-rate').value);if(!Number.isInteger(rate)||(rate!==0&&rate<20)||rate>20000)throw Error('发送预算0或20～20000；滑动窗口0表示只按回包背压');
    return {account:document.getElementById('yy-chain-account').value,interval_ms:Number(document.getElementById('yy-chain-interval').value),delay_ms:Number(document.getElementById('yy-chain-delay').value),run_count:Number(document.getElementById('yy-chain-run-count').value),channel:document.getElementById('yy-chain-channel').value,mode:mode,window:windowSize,rate_cps:rate};
  };
})();

function yyChainControlPanel(panel){
 var box=document.createElement('details');box.className='card';
 box.innerHTML='<summary>跨链调用、条件分支与等待点</summary><label>分支检测命令</label><input id="yy-branch-command" placeholder="execute if score gate task matches 1"><label>成功时调用的TXT</label><input id="yy-branch-then" placeholder="成功.txt"><label>失败时调用的TXT · 可空</label><input id="yy-branch-else" placeholder="失败.txt"><button class="btn-s" onclick="yyChainInsertBranch()">插入条件分支</button><p class="yy-hint">起止行为目标源文件真实行号，包含两端。父链等待所选片段完成；目标自己的常驻实例不会因此启动。次数0需配时限或持续；持续会停在此处直到停止。</p><label>目标命令链</label><select id="yy-call-name"></select><div class="yy-form-grid"><div><label>从第几行 · 0为开头</label><input id="yy-call-from" type="number" min="0" value="0"></div><div><label>到第几行 · 0为末尾</label><input id="yy-call-to" type="number" min="0" value="0"></div><div><label>本次执行次数</label><input id="yy-call-count" type="number" min="0" max="1000000" value="1"></div><div><label>最长运行毫秒 · 0无限时</label><input id="yy-call-duration" type="number" min="0" max="86400000" value="0"></div></div><label><input id="yy-call-keep" type="checkbox" style="width:18px;height:18px;min-height:0;margin-right:8px;display:inline-block;vertical-align:middle">持续开启所选片段</label><button class="btn-s" onclick="yyChainInsertCall()">插入调用</button><label>下一次启动的条件命令</label><input id="yy-wait-command" placeholder="execute if score gate task matches 1"><label>检测间隔 · 毫秒</label><input id="yy-wait-poll" type="number" min="100" max="60000" value="500"><label><input id="yy-wait-edge" type="checkbox" style="width:18px;height:18px;min-height:0;margin-right:8px;display:inline-block;vertical-align:middle" checked>先观察条件失败，再等待下一次成功</label><div class="yy-actions"><button class="btn-s" onclick="yyChainInsertWait()">插入条件等待</button><button class="btn-s" onclick="yyChainInsertText(\'@pause\')">插入手动等待</button></div><p class="yy-hint">等待点结束当前片段的主动运行；不会把未到期的周期积压起来。手动等待用管理页“触发等待点”或插件 chains.trigger 继续。</p>';
 panel.append(box);
 var select=box.querySelector('#yy-call-name'),account=document.getElementById('yy-chain-account').value||'main';
 yyChainRows.filter(x=>(x.config.account||'main')===account&&x.config.name!==document.getElementById('yy-chain-name').value).forEach(x=>{var o=document.createElement('option');o.value=x.config.name;o.textContent=x.config.name;select.append(o)});
 if(!select.options.length){var o=document.createElement('option');o.value='';o.textContent='先保存同账号的目标链';select.append(o)}
}
function yyChainInsertText(text){var at=yyChainEditor.state.selection.main.head;var before=yyChainEditor.state.doc.sliceString(0,at);var insert=(before&&!before.endsWith('\n')?'\n':'')+text+'\n';yyChainEditor.dispatch({changes:{from:at,insert},selection:{anchor:at+insert.length}});yyChainEditor.focus()}
function yyChainInsertCall(){var n=Number(document.getElementById('yy-call-count').value),keep=document.getElementById('yy-call-keep').checked;var data={name:document.getElementById('yy-call-name').value,from_line:Number(document.getElementById('yy-call-from').value),to_line:Number(document.getElementById('yy-call-to').value),count:keep?0:n,duration_ms:Number(document.getElementById('yy-call-duration').value),keep_open:keep};if(!data.name){T('先保存目标命令链','e');return}if(data.count===0&&!keep&&!data.duration_ms){T('次数0需设置时限或持续开启','e');return}yyChainInsertText('@call '+JSON.stringify(data))}
function yyChainInsertWait(){yyChainInsertText('@wait '+JSON.stringify({command:document.getElementById('yy-wait-command').value,poll_ms:Number(document.getElementById('yy-wait-poll').value),edge:document.getElementById('yy-wait-edge').checked}))}

function yyChainInsertBranch(){var c={command:document.getElementById("yy-branch-command").value,then:document.getElementById("yy-branch-then").value,else:document.getElementById("yy-branch-else").value};yyChainInsertText("@branch "+JSON.stringify(c))}
