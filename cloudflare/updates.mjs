const HOST='updates.youyuanqi.dpdns.org';
export async function updateRoute(request,env){
 const url=new URL(request.url), dedicated=url.hostname===HOST;
 if(!url.pathname.startsWith('/updates/')&&!dedicated)return null;
 if(!['GET','HEAD'].includes(request.method))return new Response('Method not allowed',{status:405});
 if(dedicated&&url.pathname==='/')return new Response('<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>游园更新</title><main style="font:18px sans-serif;max-width:600px;margin:12vh auto;padding:24px"><h1>游园工具箱</h1><p>在应用设置中点击“检查更新”，下载并覆盖安装，保留账号和插件数据。</p><p><a href="/updates/android/latest.apk">下载最新 APK</a></p><p><a href="/updates/android/latest.json">版本与校验信息</a></p></main>',{headers:{'content-type':'text/html; charset=utf-8'}});
 if(!['/updates/android/latest.json','/updates/android/latest.apk'].includes(url.pathname))return dedicated?new Response('Not found',{status:404}):null;
 const asset=await env.ASSETS.fetch(new Request(new URL('/updates/android/latest.json',url),{method:'GET'}));
 if(!asset.ok)return Response.json({error:'尚未发布更新'},{status:503});
 let manifest;try{manifest=await asset.json()}catch{return Response.json({error:'版本清单损坏'},{status:502})}
 if(url.pathname.endsWith('.json'))return Response.json(manifest,{headers:{'cache-control':'no-cache'}});
 const size=manifest.size;
 if(!Number.isSafeInteger(size)||size<=0||!Array.isArray(manifest.parts)||manifest.parts.reduce((n,p)=>n+p.size,0)!==size||manifest.parts.some(p=>!Number.isSafeInteger(p.size)||p.size<=0||!/^\/updates\/blobs\/[0-9.]+\/part[0-9]{3}$/.test(p.path)))return new Response('Invalid release manifest',{status:502});
 const etag='"'+manifest.sha256+'"';let start=0,end=size-1,status=200;
 const range=request.headers.get('range');
 if(range&&(!request.headers.get('if-range')||request.headers.get('if-range')===etag)){
  const match=range.match(/^bytes=(\d*)-(\d*)$/);
  if(!match||(!match[1]&&!match[2]))return new Response(null,{status:416,headers:{'content-range':'bytes */'+size}});
  if(!match[1]){const suffix=Number(match[2]);if(!Number.isSafeInteger(suffix)||suffix<=0)return new Response(null,{status:416,headers:{'content-range':'bytes */'+size}});start=Math.max(0,size-suffix)}
  else{start=Number(match[1]);end=match[2]?Math.min(size-1,Number(match[2])):size-1}
  if(!Number.isSafeInteger(start)||!Number.isSafeInteger(end)||start>=size||start>end)return new Response(null,{status:416,headers:{'content-range':'bytes */'+size}});
  status=206;
 }
 const headers={'content-type':'application/vnd.android.package-archive','content-length':String(end-start+1),'accept-ranges':'bytes','etag':etag,'cache-control':'no-cache','content-disposition':'attachment; filename="youyuan-v'+manifest.version+'.apk"'};
 if(status===206)headers['content-range']='bytes '+start+'-'+end+'/'+size;
 if(request.method==='HEAD')return new Response(null,{status,headers});
 // Native pipeTo avoids executing a JS reader callback for every network
 // fragment. One asset body is live at a time; pipe cancellation propagates.
 // Workers derives Content-Length from the body, ignoring a manually supplied
 // length on an arbitrary TransformStream. Enforce the complete APK length so
 // an interrupted stream cannot look like a successful shorter download.
 const {readable,writable}=typeof FixedLengthStream==='function'?new FixedLengthStream(end-start+1):new TransformStream();
 (async()=>{
  let offset=0;
  for(const part of manifest.parts){
   const partEnd=offset+part.size-1;
   if(partEnd<start){offset+=part.size;continue}
   if(offset>end)break;
   const first=Math.max(0,start-offset),last=Math.min(part.size-1,end-offset),partial=first!==0||last!==part.size-1;
   const partHeaders=partial?{Range:'bytes '+first+'-'+last}:{};
   const response=await env.ASSETS.fetch(new Request(new URL(part.path,url),{headers:partHeaders}));
   if(!response.ok||!response.body)throw Error('Missing APK part');
   const contentRange=response.headers.get('content-range');
   if(partial&&response.status===206){
    if(contentRange!=='bytes '+first+'-'+last+'/'+part.size)throw Error('Invalid asset range');
    await response.body.pipeTo(writable,{preventClose:true});
   }else if(partial){
    // Some asset adapters ignore Range. Buffer only this one <=8 MiB part,
    // never the full APK, and slice it once rather than fragment callbacks.
    const data=new Uint8Array(await response.arrayBuffer());if(data.length!==part.size)throw Error('Truncated APK part');
    await new Response(data.subarray(first,last+1)).body.pipeTo(writable,{preventClose:true});
   }else{
    const length=response.headers.get('content-length');if(length!==null&&Number(length)!==part.size)throw Error('Invalid APK part length');
    await response.body.pipeTo(writable,{preventClose:true});
   }
   offset+=part.size;
  }
  await writable.close();
 })().catch(error=>writable.abort(error).catch(()=>{}));
 return new Response(readable,{status,headers});
}
