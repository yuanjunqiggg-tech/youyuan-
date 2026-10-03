counter=0
def on_init(host):
    global counter
    counter=host.state().get('counter',0)
def on_event(event,host):
    global counter
    counter+=1
    host.save({'counter':counter})
    if event.get('type')=='chat' and event.get('message','').strip()=='.插件计数':
        host.console('[游园插件] 已处理 %s 次事件'%counter)
    return {'ok':True,'counter':counter,'account':host.account,'type':event.get('type')}
