// Return navigation contains no credentials. App returns close the separate
// verification browser instead of navigating its local toolbox into the cloud.
function authToolboxReturn(raw){
 const fallback='https://toolbox.youyuanqi.dpdns.org/';
 try{
  const url=new URL(raw||fallback);
  if(url.username||url.password)return fallback;
  const cloud=url.protocol==='https:'&&url.hostname==='toolbox.youyuanqi.dpdns.org'&&!url.port;
  const local=url.protocol==='http:'&&['127.0.0.1','localhost'].includes(url.hostname)&&url.port==='19429';
  if(!cloud&&!local)return fallback;
  const clean=new URL(url.origin+'/');
  if(cloud&&url.searchParams.get('return_to_app')==='1')clean.searchParams.set('return_to_app','1');
  else if(cloud&&url.searchParams.get('device'))clean.searchParams.set('device',url.searchParams.get('device'));
  return clean.href;
 }catch{return fallback;}
}
var authReturnURL=authToolboxReturn(new URL(location.href).searchParams.get('return_to'));
for(const link of document.querySelectorAll('a[data-toolbox-return]'))link.href=authReturnURL;
