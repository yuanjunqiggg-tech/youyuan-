import json
import re
from pathlib import Path

root=Path(__file__).parent/'upstream'
routes=set()
for source in root.glob('*.go'):
    if source.name.endswith('_test.go'):
        continue
    routes.update(re.findall(r'mux.HandleFunc\("(/api/[^\"]+)"',source.read_text(encoding='utf-8')))
routes.discard('/api/qq/webhook') # 签名回调由 QQ 平台调用，不作为 Agent 操作工具。
routes.discard('/api/youyuan/accounts/bridge/local') # 内部子进程交付，不给外部 MCP 调用。
code='package main\ntype yyAPIRoute struct { Name string `json:"name"`; Path string `json:"path"` }\nvar yyAPIRoutes=[]yyAPIRoute{\n'
for path in sorted(routes):
    name='api_'+path[5:].strip('/').replace('/','_').replace('-','_')
    code+='{'+json.dumps(name)+','+json.dumps(path)+'},\n'
code+='}\n'
(root/'youyuan_catalog.go').write_text(code,encoding='utf-8')
print('原生 API 路由数：',len(routes))
