import io

p = 'youyuan_catalog.go'
data = open(p, 'r', encoding='utf-8', newline='').read()
anchor = '{"api_market_download","/api/market/download"},\r\n'
if anchor not in data:
    anchor = '{"api_market_download","/api/market/download"},\n'
    addition = (
        '{"api_market_plugins","/api/market/plugins"},\n'
        '{"api_market_plugin","/api/market/plugins/"},\n'
        '{"api_market_mine","/api/market/mine"},\n'
    )
else:
    addition = (
        '{"api_market_plugins","/api/market/plugins"},\r\n'
        '{"api_market_plugin","/api/market/plugins/"},\r\n'
        '{"api_market_mine","/api/market/mine"},\r\n'
    )
if '"api_market_plugins"' not in data:
    data = data.replace(anchor, anchor + addition, 1)
    open(p, 'w', encoding='utf-8', newline='').write(data)
    print('catalog updated')
else:
    print('catalog already updated')

p = 'youyuan_mcp_links.go'
data = open(p, 'r', encoding='utf-8', newline='').read()
old = '"/api/youyuan/world/seed": true}'
new = '"/api/youyuan/world/seed": true, "/api/market/plugins": true}'
if old not in data:
    raise SystemExit('whitelist anchor not found')
if '"/api/market/plugins": true' not in data:
    data = data.replace(old, new, 1)
    open(p, 'w', encoding='utf-8', newline='').write(data)
    print('whitelist updated')
else:
    print('whitelist already updated')
