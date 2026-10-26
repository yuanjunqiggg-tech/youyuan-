/* Account console: existing registration/authentication flows and IDs stay live. */
let authRows=[],authSelected=new Set(),authPage='accounts',authViewer=null,authDialogRow=null,authRefreshEpoch=0,authDialogEpoch=0,authExpandedAccounts=new Set();
const authText=(tag,text,cls)=>{const n=document.createElement(tag);n.textContent=text;if(cls)n.className=cls;return n};
function authButton(text,fn,cls='secondary'){const b=authText('button',text,cls);b.type='button';b.onclick=async()=>{b.disabled=true;try{await fn()}catch(e){el('result').textContent=e.message}finally{b.disabled=false}};return b}
function authSection(title,icon='·'){const wrap=document.createElement('div');wrap.className='auth-section';const head=authText('button','','auth-section-head');head.type='button';head.setAttribute('aria-expanded','false');const ic=authText('span',icon,'auth-section-icon');ic.setAttribute('aria-hidden','true');head.append(ic,authText('span',title,'auth-section-title'),authText('span','','auth-chevron'));const body=document.createElement('div');body.className='auth-section-body';const inner=document.createElement('div');inner.className='auth-section-inner';body.append(inner);head.onclick=()=>{const open=head.getAttribute('aria-expanded')!=='true';head.setAttribute('aria-expanded',String(open));wrap.classList.toggle('open',open)};wrap.append(head,body);const append=wrap.append.bind(wrap);wrap.append=(...nodes)=>{inner.append(...nodes);return wrap};return wrap}
function authBoot(){
 const workspace=el('workspace'),grid=workspace.querySelector('.grid'),sections=[...workspace.children].filter(x=>x.tagName==='SECTION'),add=grid.children[0],tokens=grid.children[1];
 const shell=document.createElement('div');shell.className='auth-shell';const nav=document.createElement('nav');nav.className='auth-nav';nav.setAttribute('aria-label','账号管理导航');const main=document.createElement('div');main.className='auth-main';
 for(const [id,label]of [['accounts','我的账号'],['tokens','游戏令牌'],['services','服务中心'],['add','添加与注册'],['pool','项目账号池']]){const b=authButton(label,()=>authNavigate(id));b.id='auth-nav-'+id;b.setAttribute('role','tab');b.setAttribute('aria-selected',id==='accounts'?'true':'false');if(id==='pool')b.hidden=true;nav.append(b);const page=document.createElement('div');page.id='auth-page-'+id;page.className='auth-page';page.hidden=id!=='accounts';main.append(page)}
 const list=sections.find(s=>s.querySelector('#accounts'));
 const top=document.createElement('div');top.className='auth-top';top.append(authText('h2','我的游戏账号'),authButton('添加账号',()=>authNavigate('add')));main.querySelector('#auth-page-accounts').append(top);
 const overview=document.createElement('div');overview.className='auth-metrics';overview.id='auth-overview';main.querySelector('#auth-page-accounts').append(overview);
 const toolbar=document.createElement('div');toolbar.className='auth-toolbar';const search=document.createElement('input');search.type='search';search.id='auth-search';search.placeholder='搜索昵称、游戏ID或账号';search.setAttribute('aria-label','搜索游戏账号');search.oninput=authRender;const filter=document.createElement('select');filter.id='auth-filter';filter.setAttribute('aria-label','账号状态筛选');for(const [value,label]of [['all','全部状态'],['ready','凭据可用'],['error','需要处理'],['refresh','定时刷新已开启'],['paused','定时刷新已关闭']]){const o=authText('option',label);o.value=value;filter.append(o)}filter.onchange=authRender;toolbar.append(search,filter,authButton('刷新列表',refresh));main.querySelector('#auth-page-accounts').append(toolbar);
 const selection=document.createElement('div');selection.className='auth-selection';const all=document.createElement('input');all.type='checkbox';all.id='auth-select-all';all.onchange=()=>{for(const r of authVisibleRows())all.checked?authSelected.add(r.id):authSelected.delete(r.id);authRender()};const label=authText('label','全选当前结果');label.prepend(all);selection.append(label,authText('span','已选 0 个'));selection.lastChild.id='auth-selected-count';for(const [text,action]of [['刷新所选','refresh'],['开启定时刷新','on'],['关闭定时刷新','off']])selection.append(authButton(text,()=>authBatch(action)));main.querySelector('#auth-page-accounts').append(selection,list.querySelector('#accounts'));
 const pageAdd=main.querySelector('#auth-page-add');pageAdd.append(add,authText('p','游戏账号接入或注册成功后，在“我的账号”原位展开资料。','auth-notice'));for(const s of sections){if(s===list||s.id==='project-admin')continue;const title=s.querySelector('h2')?.textContent||'任务';if(s.id==='pending-section'){main.querySelector('#auth-page-accounts').append(s);continue}const details=authSection(title);details.append(s);pageAdd.append(details)}
 main.querySelector('#auth-page-tokens').append(tokens);tokens.prepend(authText('p','选择账号后取用它的独立游戏令牌。查看不会重新签发；只有明确点击重签才撤销旧令牌。','auth-notice'));const picker=document.createElement('select');picker.id='auth-token-account';picker.setAttribute('aria-label','选择游戏令牌账号');picker.onchange=()=>{const id=picker.value;el('auth-token-name').textContent=id?'已选账号；点“查看并复制令牌”才会读取':'尚未选择账号'};tokens.insertBefore(picker,tokens.querySelector('label'));const selectedName=authText('p','尚未选择账号');selectedName.id='auth-token-name';tokens.insertBefore(selectedName,picker);
 // 2026-10-26 第 31 条：账号标识就是令牌，不能“选一下账号就明文+自动复制”。
 // 只有明确点这个按钮才读取；切页/刷新后明文随卡片重建消失。
 const tokenView=authButton('查看并复制令牌',()=>authUseToken(picker.value));picker.after(tokenView);
 const services=main.querySelector('#auth-page-services');services.append(authText('h2','服务中心'));const cards=document.createElement('div');cards.className='auth-services';for(const [title,desc,url]of [['云端工具箱','打开Linux工作台、插件、群服互通与文件管理。',authReturnURL],['账号与机器人池','将自己的游戏令牌接入工具箱，选择服务器与多机器人任务。',authReturnURL],['注册接入教程','平台账号、游戏账号和令牌用途分开说明。',null]]){const a=document.createElement(url?'a':'button');if(url){a.href=url;a.target='_blank';a.rel='noopener noreferrer'}else a.onclick=()=>{authNavigate('add');el('registration-guide').open=true};a.append(authText('h3',title),authText('p',desc));cards.append(a)}services.append(cards);const status=authSection('平台服务状态与凭据维护');const statusText=authText('p','点击检查服务状态');status.append(statusText,authButton('检查认证服务',async()=>{const r=await api('/health',null,session||key);statusText.textContent='认证服务 '+(r.version||r.auth_version||'在线')+' · '+(r.ok?'正常':'请检查')}));services.append(status);
 const admin=el('project-admin');main.querySelector('#auth-page-pool').append(admin);workspace.append(shell);shell.append(nav,main);grid.remove();list.remove();
 const dialog=document.createElement('dialog');dialog.id='auth-detail';dialog.className='auth-dialog';dialog.addEventListener('close',()=>{authViewer?.dispose();authViewer=null});document.body.append(dialog);el('result').setAttribute('role','status');document.body.append(el('result'));
 const oldReady=ready;ready=function(){oldReady();if(!key){authRows=[];authSelected.clear();authExpandedAccounts.clear()}else authNavigate(authPage)};
 const oldAdmin=adminInit;adminInit=async function(){await oldAdmin();el('auth-nav-pool').hidden=el('project-admin').hidden};
 // 置位后 ready() 才会揭掉首屏骨架、显示重排好的控制台
 window.__yyConsoleBooted=true;
 ready();if(key){refresh();adminInit()}
}
function authNavigate(page){if(!el('auth-page-'+page))return;authPage=page;for(const n of document.querySelectorAll('.auth-page'))n.hidden=n.id!=='auth-page-'+page;for(const n of document.querySelectorAll('.auth-nav button'))n.setAttribute('aria-selected',n.id==='auth-nav-'+page?'true':'false')}
function authVisibleRows(){const q=(el('auth-search')?.value||'').trim().toLowerCase(),filter=el('auth-filter')?.value||'all';return authRows.filter(r=>([r.name,r.nickname,r.game_id].join(' ').toLowerCase().includes(q))&&(filter==='all'||filter==='ready'&&r.status==='ready'||filter==='error'&&(r.status!=='ready'||r.last_error)||filter==='refresh'&&r.keep_alive||filter==='paused'&&!r.keep_alive)).sort((a,b)=>Number(authSelected.has(b.id))-Number(authSelected.has(a.id))||b.created-a.created)}
refresh=async function(){const epoch=++authRefreshEpoch;try{const r=await api('/api/accounts');if(epoch!==authRefreshEpoch)return;
  // 皮肤缓存整体清掉：应用新皮肤、换设备改皮肤等变更都靠这里兜底刷新，
  // 顺带救回"上次 skin_preview 网络抽风"导致的占位头像
  authSkinCache.clear();
  authRows=r.accounts||[];authSelected=new Set([...authSelected].filter(id=>authRows.some(r=>r.id===id)));renderPending(r.pending||[]);renderBatches(r.batches||[]);authRender()}catch(e){el('result').textContent=e.message;
  // 首次加载失败时骨架必须撤掉，否则看起来像永远在转圈
  if(!authRows.length){const h=el('accounts');if(h&&h.querySelector('.yy-skel')){h.textContent='';h.append(authText('p','账号列表没能加载，点上方“刷新列表”重试。'))}}}};
/* 网易头像落在 gph.netease.com 这类 CDN 上，直连既慢又可能被墙。
   统一走工具箱的 /api/skin/proxy（服务端按 host 白名单校验 + 7 天缓存），
   与工具箱的头像/皮肤链路保持一致。取不到 base 时直接用原图，不让功能退化。 */
function yyAvatarSrc(url){
 // 2026-10-26 第 31 条：/api/skin/proxy 需要 Authorization，而 <img src> 带不上头，
 // 实测每次都先吃一个 401 再回退直连，首屏头像被拖慢（桌面端甚至久白）。
 // 网易头像本来就在公开 CDN（gph.netease.com），直接给原图；失败仍由 onerror 回退 MC 头。
 return url
}
/* 皮肤预览缓存：同一账号只向 skin_preview 发一次请求。
   列表头像、前 3 个账号的静默预加载、详情立体预览、组件码预览共用这一份，
   点开详情时立体预览秒出；失败结果也缓存，避免每次搜索重打一遍。
   应用新皮肤成功后按 id 精准删除，refresh() 整体清清（兜底恢复）。 */
const authSkinCache=new Map();
function authLoadSkin(row){
 let p=authSkinCache.get(row.id);
 if(!p){
  p=api('/api/accounts',{action:'skin_preview',id:row.id}).catch(()=>null);
  authSkinCache.set(row.id,p);
 }
 return p;
}
/* 用皮肤纹理裁剪玩家自己的 MC 头：经典皮肤头部区域 (8,8) 32×32，
   hat 叠加层在 (40,8)（只有 64×64 及同比例高清格式有；老格式 naturalHeight*2===naturalWidth 没有）。
   叠加层逐像素 alpha 混合，只在有像素处叠加。128×128 等高清皮肤按宽度倍数放大取样区。 */
function authMcHeadSrc(texture){
 return new Promise(resolve=>{
  const img=new Image();
  img.onload=()=>{
   try{
    const s=img.naturalWidth/64,out=document.createElement('canvas');
    out.width=32;out.height=32;
    const ctx=out.getContext('2d');
    ctx.imageSmoothingEnabled=false;
    ctx.drawImage(img,8*s,8*s,32*s,32*s,0,0,32,32);
    if(img.naturalWidth===img.naturalHeight){
     const hat=document.createElement('canvas');hat.width=32;hat.height=32;
     const hctx=hat.getContext('2d');hctx.imageSmoothingEnabled=false;
     hctx.drawImage(img,40*s,8*s,32*s,32*s,0,0,32,32);
     const base=ctx.getImageData(0,0,32,32),over=hctx.getImageData(0,0,32,32);
     for(let i=0;i<over.data.length;i+=4){
      const a=over.data[i+3];
      if(!a)continue;
      base.data[i]=(over.data[i]*a+base.data[i]*(255-a))/255;
      base.data[i+1]=(over.data[i+1]*a+base.data[i+1]*(255-a))/255;
      base.data[i+2]=(over.data[i+2]*a+base.data[i+2]*(255-a))/255;
      if(a>base.data[i+3])base.data[i+3]=a;
     }
     ctx.putImageData(base,0,0);
    }
    resolve(out.toDataURL());
   }catch(e){resolve('')}
  };
  img.onerror=()=>resolve('');
  img.src=texture;
 });
}
/* 占位头像：8bit 像素风 MC 头形（路径拼，非照片）。
   上宽下窄的头形轮廓，evenodd 把双眼和嘴掏成透明孔。 */
/* 占位头像：等距三面 MC 头。构图借 mineatar-io/skin-render 的 RenderHead 等距三面，
   光位借 NickAcPT/nmsr-rs（mc-heads.net 同源）的亮面在右前、暗侧在左；
   三档 opacity（0.38/0.68/0.95）做体积分层，单色 currentColor 深浅主题通用；
   五官用 evenodd 从面部路径挖空，任何底色下都不糊。 */
const authHeadSvg='<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path opacity="0.38" d="M13.89 21.50 L13.89 11.65 L3.82 8.13 L3.82 17.98 Z"/><path opacity="0.95" d="M20.18 6.02 L13.89 11.65 L3.82 8.13 L10.11 2.50 Z"/><path fill-rule="evenodd" opacity="0.68" d="M20.18 15.87 L13.89 21.50 L13.89 11.65 L20.18 6.02 Z M19.40 11.65 L17.82 13.06 L17.82 11.82 L19.40 10.42 Z M16.25 14.46 L14.68 15.87 L14.68 14.64 L16.25 13.23 Z M17.82 16.75 L16.25 18.16 L16.25 16.93 L17.82 15.52 Z"/><path opacity="0.95" d="M20.18 6.02 L13.89 11.65 L13.89 14.11 L20.18 8.48 Z"/><path opacity="0.95" d="M20.18 8.48 L18.61 9.89 L18.61 12.35 L20.18 10.94 Z"/><path opacity="0.95" d="M15.46 12.71 L13.89 14.11 L13.89 16.58 L15.46 15.17 Z"/></svg>';
function authAvatar(row,index=0){const b=authButton('',()=>authOpenDetail(row),'auth-avatar');b.setAttribute('aria-label','查看'+(row.nickname||row.name)+'账号详情');const placeholder=authText('span','','auth-avatar-empty');placeholder.setAttribute('aria-label','暂无头像');placeholder.innerHTML=authHeadSvg;b.append(placeholder);authFillAvatar(placeholder,row,index<3);return b}
/* 头像三级链：网易官方头像 → 玩家自己的 MC 头（皮肤纹理裁剪）→ 像素占位。
   任一级成功就显示，全失败才留占位；网易头像代理与直连都失败才继续往下走。 */
function authFillAvatar(host,row,eager){
 let mcStarted=false;
 const loadMc=async()=>{
  if(mcStarted)return;mcStarted=true;
  const preview=await authLoadSkin(row);
  const src=preview&&preview.texture?await authMcHeadSrc(preview.texture):'';
  if(!src||!host.isConnected)return;
  const img=new Image();
  img.alt='玩家自己的游戏头像';img.decoding='async';img.src=src;
  host.querySelectorAll('img').forEach(x=>x.remove());
  host.append(img);
 };
 if(row.avatar_source==='netease'){
  try{
   const url=new URL(row.avatar);
   if(url.protocol==='https:'&&!url.username&&!url.password){
    const img=new Image(),direct=url.href;let fellBack=false;
    img.alt='网易官方头像';img.src=yyAvatarSrc(direct);
    // 代理挂了回退直连，直连也挂了才撤图，改走 MC 头和占位——任何一步都不能留空白块
    img.onerror=()=>{if(!fellBack){fellBack=true;img.src=direct;return}img.remove();loadMc()};
    // 前 3 个头像在首屏内，优先拿；其余滚到视口再拉，避免首屏抢带宽
    img.loading=eager?'eager':'lazy';img.decoding='async';try{img.fetchPriority=eager?'high':'low'}catch{}
    host.append(img);
    return;
   }
  }catch{}
 }
 // 没有网易头像就直接走 MC 头；首屏外的等滚到视口再拉，避免一次打满请求
 if(eager||!('IntersectionObserver' in window)){loadMc();return}
 const io=new IntersectionObserver(entries=>{if(entries.some(e=>e.isIntersecting)){io.disconnect();loadMc()}},{rootMargin:'120px'});
 io.observe(host);
}
function authRender(){
 const host=el('accounts');host.textContent='';const rows=authVisibleRows();
 // 预加载前 3 个账号的皮肤纹理：不渲染 viewer，只温暖缓存，
 // 用户点开详情时立体预览秒出。index<3 限制，不给每个账号都发请求。
 for(let i=0;i<Math.min(3,rows.length);i++)authLoadSkin(rows[i]);
 for(let i=0;i<rows.length;i++){const row=rows[i];
  const card=document.createElement('article');card.className='auth-account-card';card.dataset.accountId=row.id;card.dataset.status=row.status==='ready'?'ready':'error';if(authExpandedAccounts.has(row.id))card.classList.add('expanded');
  const head=document.createElement('div');head.className='auth-account-head';
  const check=document.createElement('input');check.type='checkbox';check.checked=authSelected.has(row.id);check.setAttribute('aria-label','选择'+(row.nickname||row.name));check.onchange=()=>{check.checked?authSelected.add(row.id):authSelected.delete(row.id);authSelection()};
  const text=authButton('',()=>authOpenDetail(row),'auth-identity-button');text.append(authText('strong',row.nickname||row.name),authText('span','UID '+row.game_id));
  const status=authButton(row.status==='ready'?'凭据可用':'需要处理',()=>authOpenDetail(row),'auth-status'+(row.status==='ready'?'':' error'));
  const toggle=authButton(authExpandedAccounts.has(row.id)?'收起':'展开',()=>authOpenDetail(row),'auth-detail-button');
  const avatar=authAvatar(row,i),panel=document.createElement('div');panel.id='auth-account-details-'+row.id;panel.className='auth-account-details';panel.hidden=!authExpandedAccounts.has(row.id);panel.setAttribute('role','region');panel.setAttribute('aria-label',(row.nickname||row.name)+'账号详情');
  for(const b of [avatar,text,status,toggle]){b.dataset.authToggle='1';b.setAttribute('aria-controls',panel.id);b.setAttribute('aria-expanded',String(!panel.hidden))}
  head.append(check,avatar,text,status,toggle);card.append(head,panel);host.append(card);if(!panel.hidden)authBuildDetail(row,panel)
 }if(!rows.length)host.append(authText('p',authRows.length?'没有符合筛选条件的账号。':'还没有游戏账号，点击“添加账号”接入。'));
 const overview=el('auth-overview');overview.textContent='';for(const [n,label]of [[authRows.length,'我的账号'],[authRows.filter(r=>r.keep_alive).length,'定时刷新'],[authRows.filter(r=>r.status!=='ready'||r.last_error).length,'需要处理']]){const a=document.createElement('article');a.append(authText('strong',String(n)),authText('span',label));overview.append(a)}
 const picker=el('auth-token-account'),current=picker.value;picker.textContent='';picker.append(authText('option','选择游戏账号'));picker.firstChild.value='';for(const r of authRows){const o=authText('option',r.nickname||r.name);o.value=r.id;picker.append(o)}picker.value=authRows.some(r=>r.id===current)?current:'';authSelection();
}
function authSelection(){el('auth-selected-count').textContent='已选 '+authSelected.size+' 个';const visible=authVisibleRows();el('auth-select-all').checked=visible.length>0&&visible.every(r=>authSelected.has(r.id));el('auth-select-all').indeterminate=visible.some(r=>authSelected.has(r.id))&&!el('auth-select-all').checked}
async function authBatch(action){const ids=[...authSelected];if(!ids.length)throw Error('先勾选需要处理的账号');for(let i=0;i<ids.length;i++){el('result').textContent='处理中 '+(i+1)+' / '+ids.length;await api('/api/accounts',action==='refresh'?{action:'refresh',id:ids[i]}:{action:'configure',id:ids[i],keep_alive:action==='on'})}await refresh();el('result').textContent='所选账号已处理'}
async function authUseToken(id){if(!id)return;const r=await api('/api/accounts',{action:'token',id});el('token').value=r.login_token;el('auth-origin').value=r.auth_url;el('auth-token-account').value=id;el('auth-token-name').textContent='当前令牌账号：'+(authRows.find(r=>r.id===id)?.nickname||id);
 // 取用即复制、并在账号卡片内就地显示令牌条：
 // 不再把用户送去「游戏令牌」页签手动选中再复制，少两次操作和一次上下文切换。
 // 复制可能因非 https 或权限被拒而失败，此时必须明说，不能显示"已复制"。
 const copied=await copyText(r.login_token);
 const card=authAccountCard(id);if(card)showTokenInline(card,r.login_token,r.auth_url);
 el('result').textContent=copied?'令牌已复制到剪贴板，认证地址已显示在卡片里。':'浏览器不允许自动复制，令牌已显示在卡片里，请手动选中复制。'}
function authDialog(title){authDialogEpoch++;authViewer?.dispose();authViewer=null;const d=el('auth-detail');d.textContent='';const h=document.createElement('header');h.append(authText('h2',title),authButton('关闭',()=>d.close()));d.append(h);if(!d.open)d.showModal();return d}
function authInput(label,value,type='text'){const wrapper=document.createElement('label');wrapper.textContent=label;const input=document.createElement('input');input.type=type;input.value=value||'';wrapper.append(input);return {wrapper,input}}
function authAccountCard(id){return [...el('accounts').children].find(card=>card.dataset.accountId===id)}
function authSetExpanded(row,expanded){const card=authAccountCard(row.id);if(!card)return;const panel=card.querySelector('.auth-account-details');panel.hidden=!expanded;card.classList?.toggle('expanded',expanded);if(!expanded){authDisposeViewers(row.id);
 // P0（2026-10-26 第 31 条）：dispose 会把 .yy-skin-model 从台子上 remove，但 panel 子节点还在，
 // 再展开时 panel.children.length 非 0 → 不重建 → 空白皮肤台 + 点不动的姿势按钮。
 // 收起时清空详情，保证下次展开走完整重建（纹理有缓存，秒开）。
 panel.textContent=''}expanded?authExpandedAccounts.add(row.id):authExpandedAccounts.delete(row.id);for(const b of card.querySelectorAll('[data-auth-toggle]'))b.setAttribute('aria-expanded',String(expanded));card.querySelector('.auth-detail-button').textContent=expanded?'收起':'展开';if(expanded&&!panel.children.length)authBuildDetail(row,panel)}
function authOpenDetail(row){authSetExpanded(row,!authExpandedAccounts.has(row.id))}
const authViewers=new Map();
function authDisposeViewers(id){const v=authViewers.get(id);if(v){try{v.dispose()}catch(e){}authViewers.delete(id)}}
function authChip(label,value){const s=document.createElement('span');s.className='auth-chip';s.append(authText('i',label),authText('b',value));return s}
function authBuildDetail(row,d){
 const hero=document.createElement('div');hero.className='auth-detail-hero';
 const stageWrap=document.createElement('div');stageWrap.className='auth-hero-stage';stageWrap.hidden=true;
 const stage=document.createElement('div');stage.className='auth-detail-skin';stageWrap.append(stage);
 const poseBar=document.createElement('div');poseBar.className='auth-pose-bar';poseBar.hidden=true;
 const meta=document.createElement('div');meta.className='auth-hero-meta';
 meta.append(authChip('游戏ID',row.game_id),authChip('渠道',row.channel==='4399'?'4399渠道':'网易账号'),authChip('上次刷新',row.last_refresh?new Date(row.last_refresh).toLocaleString():'未记录'));
 hero.append(stageWrap,meta);d.append(hero);
 (async()=>{try{const r=await authLoadSkin(row);if(d.hidden||!r||!r.texture)return;const viewer=new YYSkinViewer(stage,r.texture,!!(r.skin_info&&r.skin_info.SkinIsSlim));authViewers.set(row.id,viewer);viewer.walk(true);for(const [key,text]of [['walk','走动'],['idle','站立']]){const b=authButton(text,()=>{viewer.walk(key==='walk');for(const o of poseBar.children)o.setAttribute('aria-pressed',o===b?'true':'false')});b.setAttribute('aria-pressed',key==='walk'?'true':'false');poseBar.append(b)}stageWrap.hidden=false;poseBar.hidden=false;stageWrap.append(poseBar)}catch(e){}})();
 const profile=authSection('头像与账号资料','◍');
 // 资料区大头像：与列表头像同一套三级链，展开即有
 const profileAvatar=document.createElement('div');profileAvatar.className='auth-profile-avatar';
 const bigAvatar=authText('span','','auth-avatar-large');
 const bigFill=authText('span','','auth-avatar-empty');bigFill.innerHTML=authHeadSvg;
 bigAvatar.append(bigFill);authFillAvatar(bigFill,row,true);
 profileAvatar.append(bigAvatar,authText('p','头像按“网易官方头像 → 此账号当前玩家自己的 MC 头 → 像素占位”顺序取用；MC 头由真实皮肤纹理裁剪，不是剪影。','auth-hint'));
 profile.append(profileAvatar);
 const name=authInput('控制台备注',row.name),nick=authInput('游戏昵称',row.nickname);
 const fields=document.createElement('div');fields.className='auth-field-grid';fields.append(name.wrapper,nick.wrapper);
 /* 网易没有开放「已拥有头像列表」接口（2026-10-23 对 FunAuth/g79client 等开源实现的全量核实，
    见 outputs/github-netease-interface-research.md）：funauth 50+ 接口只有「读当前头像 / 读当前皮肤 /
    按 item_id 设皮肤 / 按 item_id 查组件」四类能力，没有任何返回拥有集合的路径。
    所以不放一个假禁用按钮——当前官方头像本来就会自动显示在上方，这里如实说明边界。 */
 const avatarNotice=authText('p','网易未开放头像列表与更换接口，官方头像按账号资料自动显示；皮肤可按组件码设置。','auth-notice');
 const profileActions=document.createElement('div');profileActions.className='auth-action-row';
 profileActions.append(authButton('保存备注',async()=>{await api('/api/accounts',{action:'configure',id:row.id,name:name.input.value});await refresh();el('result').textContent='备注已保存'}),authButton('修改游戏昵称',async()=>{await api('/api/accounts',{action:'nickname',id:row.id,nickname:nick.input.value});await refresh();el('result').textContent='游戏昵称已更新'}));
 profile.append(fields,profileActions,avatarNotice);
 d.append(profile);
 const skin=authSection('游戏皮肤与组件码','◈');
 skin.append(authText('p','选择此账号已拥有的皮肤；不会自动购买。应用后需要重新连接游戏。','auth-hint'));
 const component=authInput('皮肤组件码 / 官方分享链接',row.skin_item_id);
 const skinActions=document.createElement('div');skinActions.className='auth-action-row';
 skinActions.append(authButton('应用已有皮肤',async()=>{const raw=component.input.value.trim(),id=/^\d{1,32}$/.test(raw)?raw:(raw.match(/(?:item_id|entity_id|component_id|id)=(\d{1,32})(?:&|$)/)||[])[1];if(!id)throw Error('填写数字组件码或包含item_id的官方分享链接');await api('/api/accounts',{action:'skin',id:row.id,item_id:id});authSkinCache.delete(row.id);await refresh();el('result').textContent='官方皮肤选择成功，下次重新进服应用'}),authButton('打开立体预览',()=>authSkin(row)));
 skin.append(component.wrapper,skinActions);
 /* 组件码预加载区：接口只能读取当前账号已选用皮肤的真实纹理（skin_preview 没有按码取图的能力），
    所以这里如实呈现——先预加载并展示当前账号皮肤的真实头部，组件码逐项列出待应用，
    不凭空生成皮肤图。 */
 const preloadBox=document.createElement('div');preloadBox.className='auth-component-preload';
 const codeBox=authText('textarea');codeBox.rows=3;codeBox.placeholder='每行一个数字组件码';codeBox.setAttribute('aria-label','皮肤组件码列表，每行一个');
 const preloadStatus=authText('p','填入组件码后点“加载预览”。','auth-hint');
 const preloadGrid=document.createElement('div');preloadGrid.className='auth-component-grid';
 const skinTile=(label,sub,src,code)=>{
  const t=document.createElement('div');t.className='auth-component-tile';
  const media=document.createElement(src?'img':'span');
  if(src){media.src=src;media.alt=label+'的真实纹理'}else{media.className='auth-avatar-empty';media.innerHTML=authHeadSvg}
  t.append(media,authText('b',label),authText('span',sub));
  if(code)t.append(authButton('应用',async()=>{
   try{await api('/api/accounts',{action:'skin',id:row.id,item_id:code});authSkinCache.delete(row.id);await refresh();preloadStatus.textContent='组件码 '+code+' 已应用，正在刷新预览…';await loadPreviews();preloadStatus.textContent='组件码 '+code+' 已应用，下次重新进服生效；预览已更新。'}
   catch(e){preloadStatus.textContent=e.message}
  }));
  preloadGrid.append(t);
 };
 const loadPreviews=async()=>{
  const list=[...new Set(codeBox.value.split(/[\s,，、;；]+/).map(x=>x.trim()).filter(x=>/^\d{1,32}$/.test(x)))];
  preloadGrid.textContent='';
  if(!list.length){preloadStatus.textContent='请先按每行一个填写数字组件码';return}
  preloadStatus.textContent='正在读取当前账号皮肤纹理…';
  const preview=await authLoadSkin(row);
  const src=preview&&preview.texture?await authMcHeadSrc(preview.texture):'';
  skinTile('当前账号皮肤',preview&&preview.skin_info&&preview.skin_info.ItemID?'组件码 '+preview.skin_info.ItemID:'未设置组件码',src);
  for(const code of list)skinTile('组件码 '+code,'待应用','',code);
  preloadStatus.textContent='已列出 '+list.length+' 个组件码；应用后当前预览会换成对应皮肤。';
 };
 preloadBox.append(codeBox,authButton('加载预览',loadPreviews),preloadStatus,preloadGrid,authText('p','接口只能读取当前账号已选用皮肤的真实纹理：上方第一个预览是当前账号皮肤的真实头部；组件码逐项应用后预览随之更新，未应用的组件码不显示皮肤图。','auth-notice'));
 skin.append(preloadBox);
 d.append(skin);
 const security=authSection('令牌与凭据维护','◇');
 security.append(authText('p','定时刷新只维护登录凭据；游戏24小时在线由Linux或APK机器人连接负责。','auth-hint'));
 const secActions=document.createElement('div');secActions.className='auth-action-row';
 secActions.append(authButton(row.keep_alive?'关闭定时刷新':'开启定时刷新',async()=>{await api('/api/accounts',{action:'configure',id:row.id,keep_alive:!row.keep_alive});await refresh();authSetExpanded(row,false)}),authButton('使用此账号令牌',async()=>{await authUseToken(row.id);authSetExpanded(row,false)}),authButton('重签游戏令牌',async()=>{if(!confirm('重签会使旧令牌失效，确定继续？'))return;const r=await api('/api/accounts',{action:'reissue',id:row.id});el('token').value=r.login_token;await refresh();authSetExpanded(row,false);authNavigate('tokens')}));
 security.append(secActions);
 d.append(security);
}
async function authSkin(row){const d=authDialog((row.nickname||row.name)+' · 皮肤预览'),epoch=authDialogEpoch,status=authText('p','正在读取账号皮肤…');d.append(status);const r=await authLoadSkin(row);if(!d.open||epoch!==authDialogEpoch)return;if(!r||!r.texture){status.textContent='没能读取这个账号的皮肤纹理：可能没有设置皮肤或官方资源暂不可用，可稍后重试。';return}status.textContent=(r.body_type_known?((r.skin_info&&r.skin_info.SkinIsSlim)?'纤细体型':'经典体型'):'官方体型未知，可手动预览')+(r.custom_geometry?' · 自定义模型暂仅显示标准人形纹理':'');if(r.warning)d.append(authText('p',r.warning));const stage=document.createElement('div');stage.className='skin-stage';stage.setAttribute('aria-label',(row.nickname||row.name)+'的皮肤，拖动转身、双击复位');d.append(stage);authViewer=new YYSkinViewer(stage,r.texture,!!(r.skin_info&&r.skin_info.SkinIsSlim));const controls=document.createElement('div');controls.className='auth-skin-controls';function segment(label,items,value,change){const group=document.createElement('div');group.setAttribute('role','group');group.setAttribute('aria-label',label);for(const [key,text]of items){const b=authButton(text,()=>{change(key);for(const other of group.children)other.setAttribute('aria-pressed',other===b?'true':'false')});b.setAttribute('aria-pressed',key===value?'true':'false');group.append(b)}return group}controls.append(segment('姿势',[['idle','站立'],['walk','走路']],'idle',v=>authViewer?.walk(v==='walk')),segment('手臂',[['classic','经典'],['slim','纤细']],r.skin_info&&r.skin_info.SkinIsSlim?'slim':'classic',v=>authViewer?.model(v==='slim')),authButton('复位视角',()=>authViewer?.reset()));d.append(controls,authText('p','拖动转身，双击复位。此处姿势和体型只用于预览，不修改游戏皮肤。'));const component=authInput('已拥有皮肤的数字组件码',row.skin_item_id);d.append(component.wrapper,authButton('应用此皮肤',async()=>{const id=component.input.value.trim();if(!/^\d{1,32}$/.test(id))throw Error('请输入数字组件码');await api('/api/accounts',{action:'skin',id:row.id,item_id:id});authSkinCache.delete(row.id);await refresh();el('result').textContent='皮肤已设置，重新进服生效';await authSkin(authRows.find(x=>x.id===row.id)||row)}));}

authBoot();
