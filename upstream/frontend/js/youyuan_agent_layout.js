(function(){
 var open=yyOpenWorkspace,queue=Promise.resolve();
 yyOpenWorkspace=function(id,pane){var account=window.yyActiveAccount||'main';queue=queue.catch(()=>{}).then(async()=>{if(account!==(window.yyActiveAccount||'main'))return;await open(id,pane);if(yyWorkspaceActive?.id!==id)return;yyAgentTabNames[id]=yyWorkspaceActive.name;yyRenderOpenAgentTabs();document.getElementById('sub-page')?.classList.add('yy-agent-page');});return queue};
 window.__subPageRestoreHooks=window.__subPageRestoreHooks||[];
 window.__subPageRestoreHooks.push({key:'*',fn:function(){if(!document.getElementById('yy-workspace'))return;var id=yyWorkspaceActive?.id;if(id)yyOpenWorkspace(id,document.getElementById('yy-workspace').dataset.pane)}});
})();
