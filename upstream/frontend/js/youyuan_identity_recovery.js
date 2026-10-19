/* Recover platform sign-in separately from game/device connectivity. */
(function(){
 function recovered(user){
  if(!yyIsCloud||!user?.cloud_allowed)return;
  window.yyCloudBlocked=false;
  if(window.yyLinuxTransportStatus){window.yyLinuxTransportStatus.auth_required=false;if(window.yyLinuxTransportStatus.last_error==='登录状态已失效，请重新登录')window.yyLinuxTransportStatus.last_error=''}
 }
 const base=yyIdentityReady;
 yyIdentityReady=function(user){recovered(user);return base.apply(this,arguments)};
 document.addEventListener('yy-identity-ready',()=>recovered(yyPlatformUser));
 window.addEventListener('yy-cloud-auth-expired',()=>{
  if(!yyIsCloud)return;
  let panel=document.getElementById('yy-identity-gate');
  if(!panel||!document.getElementById('yy-login-name')){yyShowIdentity();panel=document.getElementById('yy-identity-gate')}
  // Keep a file/export dialog intact underneath the sign-in gate.
  if(panel){panel.style.zIndex='2147483000';panel.setAttribute('aria-label','重新登录游园')}
  const result=document.getElementById('yy-login-result');
  if(result&&!result.textContent)result.textContent='登录状态已失效，请重新登录。设备上的机器人、文件与任务会保留。';
 });
})();
