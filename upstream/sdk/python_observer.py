"""Server observations, not fabricated health or coordinates. Start manually."""
counts = {'death':0,'health':0,'hurt':0,'move':0,'respawn':0,'block_change':0,'filtered_packet':0}
last = {}
capture_owned = False

def on_init(h):
    events.onDeath(on_death)
    events.onHealthChange(on_health)
    events.onHurt(on_hurt)
    events.onMove(on_move)
    events.onRespawn(on_respawn)
    events.onBlockChange(on_block)
    events.subscribeFiltered('UpdateAttributes', {'every_ms':250}, on_packet)
    web.page('monitor','Python 游戏感知',[
        {'type':'button','action':'scene','label':'读取位置与血量'},
        {'type':'button','action':'capture_start','label':'开始事件抓包'},
        {'type':'button','action':'capture_stop','label':'停止抓包'},
    ])
    web.action('monitor','scene',lambda request: {'scene':game.scene(),'counts':dict(counts),'last':dict(last)})
    web.action('monitor','capture_start',lambda request: start_capture())
    web.action('monitor','capture_stop',lambda request: stop_capture())

def record(kind,event):
    counts[kind]+=1
    last[kind]=event
def on_death(event): record('death',event)
def on_health(event): record('health',event)
def on_hurt(event): record('hurt',event)
def on_move(event): record('move',event)
def on_respawn(event): record('respawn',event)
def on_block(event): record('block_change',event)
def on_packet(packet): record('filtered_packet',packet)
def start_capture():
    global capture_owned
    result=events.captureStart({'capacity':256,'filter':{'every_ms':100}})
    capture_owned=True
    return result
def stop_capture():
    global capture_owned
    result=events.captureStop()
    capture_owned=False
    return result
def metrics(): return {'counts':dict(counts),'last':dict(last)}
def batch_probe(): return host.batch([('util.floor',[3.9]),('bot.self',[])])
def on_stop(h):
    if capture_owned: stop_capture()
plugin_api={'scene':lambda:game.scene(),'metrics':metrics,'batch_probe':batch_probe,'health':lambda target='self':player.health(target),'capture_start':start_capture,'capture_stop':stop_capture}
