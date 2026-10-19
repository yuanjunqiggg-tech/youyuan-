(function(){
  var open=yyOpenWorkspace;
  yyOpenWorkspace=async function(id,pane){
    await open(id,pane);
    if(!document.getElementById('yy-workspace')||yyWorkspaceActive.language==='mcfunction')return;
    var bar=document.querySelector('.yy-agent-menu');
    if(bar)bar.insertAdjacentHTML('beforeend','<button class="yy-text-btn" onclick="yyPluginCommandSettings()">命令执行参数</button>');
  };
  var usage=yyRenderAgentUsage;
  yyRenderAgentUsage=function(ws){
    usage(ws);var el=document.getElementById('yy-agent-usage'),c=ws&&ws.context;
    if(el&&c&&c.window)el.textContent+=' · '+(c.source==='provider_usage_calibrated_estimate'?'上下文校准估算 ':'上下文估算 ')+c.estimated_tokens+' / '+c.window+' · '+Math.round(100*c.compression_threshold/c.window)+'% 压缩阈值 '+c.compression_threshold;
  };
  window.yyPluginCommandSettings=async function(){
    var id=yyWorkspaceActive.id;
    if(yyEditorDirty&&!await yySaveProjectFile())return;
    var response=await A('GET','/api/youyuan/agent/file?id='+encodeURIComponent(id)+'&path=command-options.json');
    var options={channel:'ai',mode:'ordered',window:8,timeout_ms:3000};
    if(response.ok){try{Object.assign(options,JSON.parse(response.content))}catch(error){T('command-options.json 格式错误，请先在编辑器中修正','e');return}}
    var host=yyAgentProfileModal('插件命令执行参数');
    host.innerHTML='<p class="yy-hint">保存到当前项目 command-options.json，AI 可读取和修改；发布并重新加载插件后生效。每个插件实例独立，源码内 setCommandOptions 可覆盖默认值。</p><label>默认通道</label><select id="yy-exec-channel"><option value="ai">魔法指令 · 自动化</option><option value="console">控制台 · 自动化</option><option value="player">玩家 · 玩家权限</option></select><label>默认批量方式</label><select id="yy-exec-mode"><option value="ordered">逐条等待确认 · 有依赖命令</option><option value="pipeline">流水线 · 仅互不依赖的命令</option><option value="stream">动态滑动窗口 · 按回包背压</option><option value="burst">整批发送 · 最多1024条</option><option value="submit">只发送 · 不确认执行结果</option></select><label>每批条数 · 流水线1—16 / 整批1—1024</label><input id="yy-exec-window" type="number" min="1" max="1024"><label>可选发送上限 · 0 不限速</label><input id="yy-exec-rate" type="number" min="0" max="20000"><label>回包等待时间 · 毫秒</label><input id="yy-exec-timeout" type="number" min="100" max="10000"><p class="yy-hint">魔法指令和控制台目前使用相同自动化来源。流水线不保证服务端完成顺序；只发送不能用于成功条件判断。</p><div class="yy-actions"><button class="btn" id="yy-exec-save">保存参数</button></div><pre class="yy-readout" id="yy-exec-result"></pre>';
    for(var key of ['channel','mode','window'])document.getElementById('yy-exec-'+key).value=options[key];
    document.getElementById('yy-exec-timeout').value=options.timeout_ms;document.getElementById('yy-exec-rate').value=options.rate_cps||0;
    document.getElementById('yy-exec-mode').onchange=function(){document.getElementById('yy-exec-window').value=this.value==='burst'?1024:this.value==='stream'?128:8;document.getElementById('yy-exec-rate').value=0};
    document.getElementById('yy-exec-save').onclick=async function(){
      var value={channel:document.getElementById('yy-exec-channel').value,mode:document.getElementById('yy-exec-mode').value,window:Number(document.getElementById('yy-exec-window').value),timeout_ms:Number(document.getElementById('yy-exec-timeout').value),rate_cps:Number(document.getElementById('yy-exec-rate').value)};
      if(!Number.isInteger(value.window)||value.window<1||value.window>(['burst','stream'].includes(value.mode)?1024:16)||!Number.isInteger(value.rate_cps)||(value.rate_cps!==0&&value.rate_cps<20)||value.rate_cps>20000||!Number.isInteger(value.timeout_ms)||value.timeout_ms<100||value.timeout_ms>10000){document.getElementById('yy-exec-result').textContent='流水线1—16/整批1—1024，发送预算0或20—20000；stream预算0表示无固定CPS，等待时间100—10000毫秒。';return}
      var result=await A('POST','/api/youyuan/agent/file',{id:id,path:'command-options.json',content:JSON.stringify(value,null,2)+'\n',base_hash:response.hash||''});
      document.getElementById('yy-exec-result').textContent=result.ok?'已保存，发布并重新加载插件后生效。':result.error;
      if(result.ok){response.hash=result.hash;await yyPollWorkspace()}
    };
  };
})();
