"""Linux C ABI worker. One process and one native instance per plugin.

Only the transport uses Python; business handlers execute in the loaded library.
The SDK allocates/free()s returned strings with the process libc. Keep that ABI.
"""
import ctypes
import json
import os
import sys
import urllib.request

# Native printf/cout must not corrupt our response pipe.
protocol = os.fdopen(os.dup(sys.stdout.fileno()), 'w', buffering=1)
os.dup2(sys.stderr.fileno(), sys.stdout.fileno())
libc = ctypes.CDLL(None)
libc.malloc.argtypes = [ctypes.c_size_t]
libc.malloc.restype = ctypes.c_void_p
libc.free.argtypes = [ctypes.c_void_p]
libc.free.restype = None
HostAPI = ctypes.CFUNCTYPE(ctypes.c_void_p, ctypes.c_char_p, ctypes.c_char_p)
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
library = instance = callback = context = None

def reply(value):
    protocol.write(json.dumps(value, ensure_ascii=False) + '\n')
    protocol.flush()

def host_call(path, body):
    try:
        path = path.decode('utf8')
        if not path.startswith('/api/') or path.startswith('/api/youyuan/runtime/'):
            raise ValueError('插件只能调用工具箱 API，不能递归调用运行器')
        data = json.loads(body) if body else None
        if isinstance(data, dict):
            if 'account' in data:
                data['account'] = context['account']
            if path.startswith('/api/youyuan/sdk/'):
                data['plugin_id'] = context['plugin_id']
        request = urllib.request.Request(
            'http://127.0.0.1:' + str(context['port']) + path,
            data=None if data is None else json.dumps(data).encode('utf8'),
            headers={'Content-Type': 'application/json'})
        with opener.open(request, timeout=60) as response:
            value = json.load(response)
    except Exception as error:
        value = {'ok': False, 'error': str(error)}
    text = json.dumps(value, ensure_ascii=False).encode('utf8') + b'\0'
    pointer = libc.malloc(len(text))
    if pointer:
        ctypes.memmove(pointer, text, len(text))
    return pointer

def dispatch(request):
    global library, instance, callback, context
    action = request['action']
    if action == 'start':
        if instance:
            raise RuntimeError('原生实例已经启动')
        context = request
        library = ctypes.CDLL(request['entry'])
        callback = HostAPI(host_call)
        library.yy_event.argtypes = [ctypes.c_void_p, ctypes.c_char_p]
        library.yy_event.restype = ctypes.c_void_p
        library.yy_destroy.argtypes = [ctypes.c_void_p]
        library.yy_destroy.restype = None
        create = getattr(library, 'yy_create_v2', None)
        if create is not None:
            metadata = json.dumps({'account': request['account'],
                                   'plugin_id': request['plugin_id'],
                                   'sdk_version': '1.15'}).encode('utf8')
        else:
            create = library.yy_create
            metadata = request['account'].encode('utf8')
        create.argtypes = [ctypes.c_char_p, HostAPI]
        create.restype = ctypes.c_void_p
        instance = create(metadata, callback)
        if not instance:
            raise RuntimeError('yy_create 返回空指针，请检查初始化和 SDK 调用错误')
        return {'ok': True}
    if not instance:
        raise RuntimeError('原生实例尚未启动')
    if action == 'event':
        pointer = library.yy_event(instance, request['event'].encode('utf8'))
        if not pointer:
            raise RuntimeError('yy_event 返回空指针')
        try:
            return json.loads(ctypes.string_at(pointer).decode('utf8'))
        finally:
            libc.free(pointer)
    if action == 'stop':
        library.yy_destroy(instance)
        instance = None
        return {'ok': True}
    raise ValueError('未知原生操作: ' + str(action))

for line in sys.stdin:
    try:
        request = json.loads(line)
        reply(dispatch(request))
        if request['action'] == 'stop':
            break
    except Exception as error:
        reply({'ok': False, 'error': str(error)})
