export const readTools=new Set(['api_bot_status','api_youyuan_observations','api_youyuan_runtime_list','api_youyuan_docs','api_youyuan_sdk_catalog']);
export function readPacketAllowed(p){
 if(['initialize','ping','tools/list','resources/list','resources/templates/list'].includes(p.method))return true;
 if(p.method==='resources/read'){const u=p.params?.uri||'';return u.startsWith('youyuan://docs/')||['youyuan://skills/catalog','youyuan://runtime/status','youyuan://accounts/status'].includes(u)}
 if(p.method==='tools/call')return readTools.has(p.params?.name)&&(!p.params?.arguments?.method||p.params.arguments.method==='GET');
 return false;
}
