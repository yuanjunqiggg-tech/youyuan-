/* The existing workspace, model, skill and MCP contracts remain authoritative. */
(function(){
 'use strict';
 const baseOpen=yyOpenWorkspace,basePane=yyWorkspacePane;
 function button(text,fn,cls){const b=document.createElement('button');b.type='button';b.textContent=text;b.className=cls||'yy-text-btn';b.onclick=fn;return b}
 yyOpenWorkspace=async function(id,pane){await baseOpen(id,pane);if(yyWorkspaceActive?.id!==id||!document.getElementById('yy-workspace'))return;mount();};
 yyWorkspacePane=function(pane){basePane(pane);const page=document.getElementById('sub-page');if(page)page.dataset.studioPane=pane;};
 function mount(){
  const page=document.getElementById('sub-page'),workspace=document.getElementById('yy-workspace'),agent=workspace.querySelector('.yy-agent-pane');
  if(!agent||document.getElementById('yy-agent-studio-shell'))return;
  page.classList.add('yy-agent-studio');page.dataset.studioPane=workspace.dataset.pane;
  const shell=document.createElement('div');shell.id='yy-agent-studio-shell';workspace.before(shell);
  const side=document.createElement('aside');side.className='yy-studio-sessions';side.setAttribute('aria-label','工作区会话');
  const heading=document.createElement('div');heading.className='yy-studio-side-title';heading.textContent='游园 / 创作';side.append(heading);
  const sessions=document.getElementById('yy-open-agent-tabs');if(sessions)side.append(sessions);
  const links=document.createElement('div');links.className='yy-studio-side-links';
  links.append(button('技能与工具',()=>yyAgentPreferences()),button('技能加载库',()=>yyOpenSkillLibrary()),button('MCP 连接',()=>{yyWorkspacePane('context');document.getElementById('yy-mcp-name')?.focus()}),button('运行预算',yyAgentRunBudgetDialog));side.append(links);
  shell.append(side,workspace);
  const context=document.getElementById('yy-workspace-context');shell.append(context);
  const toolbar=agent.querySelector('.yy-model-toolbar'),compose=agent.querySelector('.yy-agent-compose');compose.append(toolbar);
  const select=document.getElementById('yy-agent-model');select.title='本工作区使用的模型';
  toolbar.querySelector('.yy-profile-toolbar')?.remove();
  const settings=document.createElement('details');settings.className='yy-studio-options';const summary=document.createElement('summary');summary.textContent='配置';
  const menu=document.createElement('div');menu.className='yy-studio-options-menu';menu.append(button('模型与服务商',()=>yyAgentModels()),button('提示词 / 技能 / 工具',()=>yyAgentPreferences()),button('MCP 连接',()=>yyWorkspacePane('context')),button('运行预算',yyAgentRunBudgetDialog));settings.append(summary,menu);toolbar.append(settings);
  const legacy=toolbar.querySelector('button');if(legacy)legacy.remove();
  const actions=compose.querySelector('.yy-agent-compose-actions');actions.querySelector('button')?.replaceChildren(document.createTextNode('停止'));
  const text=document.getElementById('yy-agent-message');text.placeholder='描述要完成的目标，或补充当前任务…';
  text.addEventListener('keydown',e=>{if((e.ctrlKey||e.metaKey)&&e.key==='Enter'&&!e.isComposing){e.preventDefault();yyAgentTask('send')}});
  const hint=document.createElement('small');hint.className='yy-studio-input-hint';hint.textContent='Ctrl / ⌘ + Enter 发送 · 停止后可继续，记录自动保存';compose.append(hint);
  mountEditor(workspace);
  // Configuration is folded into separate sections; no second copy of fields.
  context.querySelectorAll('.yy-form-grid > .card').forEach((card,index)=>{const d=document.createElement('details');d.className='yy-studio-config-group';d.open=index===1;const s=document.createElement('summary');s.textContent=['修改历史','MCP 连接与工具','任务记忆与开发说明'][index]||'更多设置';card.before(d);d.append(s,card)});
  const nav=agent.querySelector('.yy-agent-scroll-controls');nav?.querySelector('button')?.remove();
 }
 function drawFiles(){
  const list=document.getElementById('yy-studio-file-list');if(!list)return;
  const files=yyWorkspaceActive?.files||[];
  const signature=JSON.stringify(files.map(file=>file.path));
  if(list.dataset.paths!==signature){list.replaceChildren();for(const file of files){const b=button(file.path,()=>yyOpenProjectFile(file.path),'yy-studio-file');b.title=file.path;b.dataset.path=file.path;list.append(b)}list.dataset.paths=signature}
  list.querySelectorAll('[data-path]').forEach(b=>b.classList.toggle('active',b.dataset.path===yyEditorPath));
  const title=document.getElementById('yy-studio-file-title');if(title)title.textContent=yyEditorPath||'选择项目文件';
 }
 function mountEditor(workspace){
  const pane=workspace.querySelector('.yy-code-pane');if(!pane||pane.querySelector('.yy-editor-layout'))return;
  const layout=document.createElement('div');layout.className='yy-editor-layout';const explorer=document.createElement('aside');explorer.className='yy-studio-explorer';explorer.setAttribute('aria-label','项目文件');const label=document.createElement('strong');label.textContent='项目文件';const list=document.createElement('div');list.id='yy-studio-file-list';explorer.append(label,list);
  const surface=document.createElement('div');surface.className='yy-editor-surface';while(pane.firstChild)surface.append(pane.firstChild);const tab=document.createElement('div');tab.className='yy-studio-file-title';tab.innerHTML='<span aria-hidden="true">◇</span><strong id="yy-studio-file-title"></strong><small>自动保存</small>';surface.querySelector('.yy-editor-toolbar').after(tab);layout.append(explorer,surface);pane.append(layout);drawFiles();
  const search=surface.querySelector('.yy-project-search'),result=surface.querySelector('#yy-project-search-result');if(search&&result){const fold=document.createElement('details');fold.className='yy-editor-search-fold';const summary=document.createElement('summary');summary.textContent='在项目中搜索';search.before(fold);fold.append(summary,search,result)}
 }
 const drawProjectFiles=yyDrawProjectFiles;yyDrawProjectFiles=function(files){drawProjectFiles(files);drawFiles()};
 const openProjectFile=yyOpenProjectFile;yyOpenProjectFile=async function(path){await openProjectFile(path);drawFiles()};
 window.yyAgentRunBudgetDialog=async function(){
  const id=yyWorkspaceActive?.id,ownerScope=yyEditorAccountScope(),epoch=yyWorkspaceOpenEpoch;if(!id)return;
  const live=()=>id===yyWorkspaceActive?.id&&ownerScope===yyEditorAccountScope()&&epoch===yyWorkspaceOpenEpoch;
  const r=await A('GET','/api/youyuan/agent/run-budget?id='+encodeURIComponent(id));if(!live())return;if(!r.ok){T(r.error,'e');return}
  const host=yyAgentProfileModal('运行预算');host.innerHTML='<p class="yy-hint">达到预算会保存进度并等待继续。默认不设轮次和工具次数上限，连续工具失败会交给模型处理。服务商的配额和接口错误仍可能暂停任务。</p><div class="yy-studio-budget-grid"><label>轮次上限（0 不限制）<input id="yy-budget-rounds" type="number" min="0" max="10000"></label><label>工具次数（0 不限制）<input id="yy-budget-tools" type="number" min="0" max="100000"></label><label>单次运行分钟<input id="yy-budget-minutes" type="number" min="1" max="1440"></label></div><p class="yy-hint">保存用于下次开始或继续任务，正在运行的任务沿用启动时预算。</p><div id="yy-budget-result" role="status"></div>';
  host.querySelector('#yy-budget-rounds').value=r.budget.max_rounds;host.querySelector('#yy-budget-tools').value=r.budget.max_tools;host.querySelector('#yy-budget-minutes').value=r.budget.max_minutes;
  const save=button('保存预算',async()=>{if(!live()||!host.isConnected||save.disabled)return;save.disabled=true;try{const budget={max_rounds:Number(host.querySelector('#yy-budget-rounds').value),max_tools:Number(host.querySelector('#yy-budget-tools').value),max_minutes:Number(host.querySelector('#yy-budget-minutes').value)};const result=await A('POST','/api/youyuan/agent/run-budget',{id,budget});if(live()&&host.isConnected)host.querySelector('#yy-budget-result').textContent=result.ok?'预算已保存，下次运行生效。':result.error;}catch(e){if(live()&&host.isConnected)host.querySelector('#yy-budget-result').textContent=e.message}finally{save.disabled=false}},'yy-primary');host.append(save);
 };
 const preferences=yyAgentPreferences;
 yyAgentPreferences=async function(){const id=yyWorkspaceActive?.id;await preferences();if(id!==yyWorkspaceActive?.id)return;const host=document.getElementById('yy-agent-profile-body');if(!host)return;
  host.closest('.modal-box').classList.add('yy-studio-preferences');
  const search=document.createElement('input');search.type='search';search.placeholder='筛选技能或工具名称';search.setAttribute('aria-label','筛选技能或工具');
  search.oninput=()=>host.querySelectorAll('.yy-profile-checks label').forEach(label=>{label.hidden=!label.textContent.toLowerCase().includes(search.value.trim().toLowerCase())});host.prepend(search);
  host.querySelectorAll('.yy-profile-checks').forEach((list,i)=>{const group=document.createElement('details');group.className='yy-studio-config-group';group.open=i===0;const summary=document.createElement('summary');summary.textContent=i===0?'已加载与可用技能':'工具权限';list.before(group);group.append(summary,list)});
 };
})();
