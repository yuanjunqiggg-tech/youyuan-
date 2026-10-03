"""Ready-to-run game controller. All game mutations require a connected bot."""
counter = 0
ticks = 0

def on_init(h):
    global counter
    counter = h.state().get('counter', 0)
    game.setConfigDefault({'trigger': '.插件状态'})
    game.registerTrigger([game.getConfig('trigger', '.插件状态')], on_trigger)
    events.onJoin(on_join)
    util.schedule(2, on_tick)
    web.page('control', 'Python 插件控制', [
        {'type': 'text', 'key': 'counter', 'label': '事件计数'},
        {'type': 'button', 'action': 'status', 'label': '读取真实状态'},
    ])
    web.action('control', 'status', on_web)

def on_trigger(player_name, args):
    global counter
    counter += 1
    host.save({'counter': counter})
    game.sendTargeted(player_name, '§aPython 常驻插件已处理 %d 次请求' % counter)

def on_join(player_name):
    game.sendTargeted(player_name, '§a欢迎进入服务器！')

def on_tick():
    global ticks
    ticks += 1

def on_web(request):
    return {'counter': counter, 'ticks': ticks, 'status': host.status()}

def add(a, b): return a+b
def state(): return {'counter': counter, 'ticks': ticks, 'account': host.account}
def sum_cpp(a,b): return api.require('yy_sdk_cpp_demo').add(a,b)
def execute(command): return game.execute(command)
plugin_api = {'add': add, 'state': state, 'sum_cpp': sum_cpp, 'execute': execute}

def on_event(event,h):
    return {'ok':True, 'counter':counter, 'ticks':ticks}

def on_stop(h): h.save({'counter':counter})
