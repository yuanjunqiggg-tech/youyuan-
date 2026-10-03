async function yyLibraryPage(){
 delete subPageCache['自建云端资料库'];var r=await A('GET','/api/youyuan/library/list');
 var html='<div class="card"><h3>自己的插件、建筑与音乐资料库</h3><p>文件保存到你自己的 Cloudflare Durable Objects。不会连接原作者市场。</p><select id="yy-lib-category"><option value="plugins">插件</option><option value="buildings">建筑</option><option value="music">音乐</option></select><input id="yy-lib-file" type="file"><button class="btn" onclick="yyLibraryUpload()">上传文件</button><pre id="yy-lib-result" style="white-space:pre-wrap">'+yyEscape(r.error||'')+'</pre></div>';
 (r.items||[]).forEach(item=>{html+='<div class="card"><strong>'+yyEscape(item.name)+'</strong><p>'+yyEscape(item.category)+' · '+item.bytes+' 字节</p><button class="btn-s" onclick="yyLibraryDownload(\''+item.id+'\')">下载</button>'+(item.category==='plugins'?'<button class="btn" onclick="yyLibraryInstall(\''+item.id+'\')">导入 Python 插件</button>':'')+'</div>'});
 openSubPage('自建云端资料库',html);
}
// 原有功能菜单现在进入自建资料库，批量预览仍调用 APK 内核。
pmOpenMarket=yyLibraryPage;
bmOpenMarket=yyLibraryPage;
bmOpenContribute=yyLibraryPage;
RbuildingMgmtHTML=function(){return '<div class="card"><h3>建筑资料管理</h3><button class="btn" onclick="yyLibraryPage()">自建云端资料库 · 上传与下载</button><button class="btn" onclick="bmOpenBatch()">本地批量建筑预览</button><p>资料库存储在自己的 Cloudflare，没有订阅与付费购买入口。</p></div>'};
async function yyLibraryUpload(){var input=document.getElementById('yy-lib-file');if(!input.files[0])return;var form=new FormData();form.append('file',input.files[0]);form.append('category',document.getElementById('yy-lib-category').value);try{var r=await(await fetch('/api/youyuan/library/upload',{method:'POST',body:form})).json();if(r.ok)yyLibraryPage();else document.getElementById('yy-lib-result').textContent=r.error}catch(e){document.getElementById('yy-lib-result').textContent=e.message}}
async function yyLibraryInstall(id){try{var response=await fetch('/api/youyuan/library/download?id='+id);if(!response.ok)throw Error(await response.text());var form=new FormData();form.append('file',await response.blob(),'plugin.zip');var r=await(await fetch('/api/youyuan/runtime/import',{method:'POST',body:form})).json();alert(r.ok?'已导入：'+r.id:r.error)}catch(e){alert(e.message)}}
async function yyLibraryDownload(id){
 if(location.hostname==='127.0.0.1'){var link=document.createElement('a');link.href='/api/youyuan/library/download?id='+id;link.download='';link.click();return}
 var response=await fetch('/api/youyuan/library/download?id='+id);if(!response.ok){alert(await response.text());return}var link=document.createElement('a');link.href=URL.createObjectURL(await response.blob());link.download='youyuan-'+id;link.click();setTimeout(()=>URL.revokeObjectURL(link.href),10000);
}
