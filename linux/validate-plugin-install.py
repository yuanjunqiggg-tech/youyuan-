"""Localhost verification only; no game/QQ/model connections are started."""
import argparse,json,hashlib,time,zipfile,urllib.request,urllib.parse
from pathlib import Path

def main():
 p=argparse.ArgumentParser();p.add_argument('--url',default='http://127.0.0.1:19429');p.add_argument('--plugin',required=True);p.add_argument('--report',required=True);a=p.parse_args()
 assert urllib.parse.urlparse(a.url).hostname in {'127.0.0.1','localhost'},'Validation must stay on localhost'
 def api(method,path,body=None):
  raw=json.dumps(body,ensure_ascii=False).encode() if body is not None else None
  req=urllib.request.Request(a.url+path,data=raw,method=method,headers={'Content-Type':'application/json'})
  with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(req,timeout=30) as r:return json.load(r)
 for n in range(60):
  try:
   health=api('GET','/health')
   if health.get('ready'):break
  except (OSError,ValueError):pass
  time.sleep(.5)
 else:raise RuntimeError('Server not ready')
 with zipfile.ZipFile(a.plugin) as z:expected=z.read('main.lua')
 imported=api('POST','/api/plugin/import',{'path':str(Path(a.plugin).resolve())});assert imported.get('ok'),imported
 pid=imported['id'];assert pid=='ds_ai_agent'
 row=next(x for x in api('GET','/api/plugin/list')['plugins'] if x['id']==pid)
 read=api('GET','/api/plugin/file/read?'+urllib.parse.urlencode({'id':pid,'scope':'code','file':'main.lua'}));assert read.get('ok'),read.get('error')
 content=read['content'].encode();assert content==expected
 entry=read['name']
 wid='deployment_ds_ai_agent'
 workspaces=api('GET','/api/youyuan/agent/workspaces')['workspaces']
 if not any(x['id']==wid for x in workspaces):
  created=api('POST','/api/youyuan/agent/workspaces',{'id':wid,'name':'多功能插件部署验收','language':'lua','plugin_id':pid});assert created.get('ok'),created
 read=api('GET','/api/youyuan/agent/file?'+urllib.parse.urlencode({'id':wid,'path':entry}));assert read.get('ok'),read.get('error');assert read['content'].encode()==expected
 def tool(name,args):
  result=api('POST','/api/youyuan/agent/tool',{'id':wid,'tool':name,'args':args});assert result.get('ok'),result.get('error');return result['result']
 partial=tool('project_read',{'path':entry,'start_line':1,'lines':20});assert len(partial)>100
 found=tool('project_search',{'query':'beijing_fmt'});assert entry in found and 'beijing_fmt' in found
 remote=api('GET','/api/youyuan/remote');key=remote['key']
 req=urllib.request.Request(a.url+'/mcp/',data=json.dumps({'jsonrpc':'2.0','id':1,'method':'tools/list'}).encode(),method='POST',headers={'Content-Type':'application/json','Authorization':'Bearer '+key})
 with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(req,timeout=20) as r:mcp=json.load(r)
 report={'version':health['version'],'plugin_id':pid,'source_bytes':len(expected),'source_sha256':hashlib.sha256(expected).hexdigest(),'actual_plugin_import':True,'installed_source_matches_zip':True,'agent_workspace_has_full_source':True,'actual_agent_project_read':True,'actual_agent_project_search':True,'mcp_tools_count':len(mcp['result']['tools']),'plugin_running':bool(row.get('running')),'game_connected':bool(api('GET','/api/bot/status').get('connected')),'game_event_execution_tested':False,'model_provider_tested':False}
 Path(a.report).write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8');print(json.dumps(report,ensure_ascii=True))
if __name__=='__main__':main()
