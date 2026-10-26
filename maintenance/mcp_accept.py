#!/usr/bin/env python3
"""mcp_accept.py —— MCP 四步验收（在 box 本机跑，走 127.0.0.1:19429/mcp/）。

四步：initialize → tools/list → resources/list → tools/call(health/status)。
判定标准：每步 HTTP 200 且 JSON-RPC 无 error 字段。
"""
from __future__ import annotations

import json
import urllib.request

URL = "http://127.0.0.1:19429/mcp/"
KEY = json.load(open("/var/lib/youyuan/youyuan-remote.json"))["key"]


def call(method: str, params: dict | None = None, rid: int = 1) -> dict:
    body = {"jsonrpc": "2.0", "id": rid, "method": method}
    if params is not None:
        body["params"] = params
    req = urllib.request.Request(
        URL,
        data=json.dumps(body).encode(),
        headers={
            "Content-Type": "application/json",
            "Accept": "application/json, text/event-stream",
            "Authorization": "Bearer " + KEY,
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            raw = r.read().decode("utf-8", "replace")
            code = r.status
    except urllib.error.HTTPError as e:
        return {"_http": e.code, "_err": e.read().decode("utf-8", "replace")[:200]}
    # 可能是 SSE：取 data: 行
    if raw.lstrip().startswith("event:") or raw.lstrip().startswith("data:"):
        for line in raw.splitlines():
            if line.startswith("data:"):
                raw = line[5:].strip()
                break
    try:
        out = json.loads(raw)
    except Exception:
        return {"_http": code, "_raw": raw[:200]}
    out["_http"] = code
    return out


print("=" * 60)
print("第 1 步 initialize")
r1 = call("initialize", {
    "protocolVersion": "2025-06-18",
    "capabilities": {},
    "clientInfo": {"name": "youyuan-accept", "version": "1.0"},
})
si = r1.get("result", {}).get("serverInfo", {})
print(f"  HTTP {r1.get('_http')}  error={r1.get('error')}")
print(f"  serverInfo={si}")
print(f"  protocolVersion={r1.get('result', {}).get('protocolVersion')}")
print(f"  capabilities={list((r1.get('result', {}).get('capabilities') or {}).keys())}")

print("=" * 60)
print("第 2 步 tools/list")
r2 = call("tools/list", {}, 2)
tools = r2.get("result", {}).get("tools", [])
print(f"  HTTP {r2.get('_http')}  error={r2.get('error')}")
print(f"  tools 数量 = {len(tools)}")
names = [t.get("name") for t in tools]
print(f"  前 20 个: {names[:20]}")
has_schema = sum(1 for t in tools if t.get("inputSchema"))
print(f"  带 inputSchema 的 = {has_schema}/{len(tools)}")

print("=" * 60)
print("第 3 步 resources/list")
r3 = call("resources/list", {}, 3)
res = r3.get("result", {}).get("resources", [])
tpl = r3.get("result", {}).get("resourceTemplates", [])
print(f"  HTTP {r3.get('_http')}  error={r3.get('error')}")
print(f"  resources 数量 = {len(res)}  templates = {len(tpl)}")
print(f"  前 10 个 URI: {[x.get('uri') for x in res[:10]]}")

print("=" * 60)
print("第 4 步 tools/call（真实读一次状态）")
targets = [n for n in names if n in ("health", "status", "api_call", "runtime_status")]
print(f"  可用的候选工具: {targets}")
for i, tname in enumerate(targets[:2], start=10):
    args = {"path": "/health"} if tname == "api_call" else {}
    rr = call("tools/call", {"name": tname, "arguments": args}, i)
    res_payload = rr.get("result", {})
    content = res_payload.get("content", [])
    text = ""
    if content and isinstance(content, list):
        text = str(content[0].get("text", ""))[:300]
    print(f"  [{tname}] HTTP {rr.get('_http')} isError={res_payload.get('isError')} error={rr.get('error')}")
    print(f"    返回片段: {text}")

print("=" * 60)
ok = (
    r1.get("_http") == 200 and not r1.get("error")
    and r2.get("_http") == 200 and not r2.get("error") and len(tools) > 0
    and r3.get("_http") == 200 and not r3.get("error") and len(res) > 0
)
print("四步验收结论:", "通过" if ok else "未通过")
