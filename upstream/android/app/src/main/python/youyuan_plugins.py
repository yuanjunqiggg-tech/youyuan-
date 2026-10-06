"""APK 内置 Python 3.12 常驻模块，每个账号使用独立命名空间和持久数据目录。"""
import importlib.util
import asyncio
import inspect
import hashlib
import json
import os
import sys
import threading
import types
import urllib.request
from pathlib import Path

_instances={}
_lock=threading.RLock()

class Namespace:
    """Original API names with callable callback registration and JSON errors."""
    def __init__(self,host,name): self._host,self._name=host,name
    def __getattr__(self,method):
        if method.startswith('_'): raise AttributeError(method)
        def invoke(*args):
            if self._name=='api' and method=='require':
                if len(args)!=1: raise TypeError('api.require(plugin_id)')
                return RemotePlugin(self._host,args[0])
            encoded=[self._host._callback(value) if callable(value) else value for value in args]
            return self._host.invoke(self._name+'.'+method,*encoded)
        return invoke

class RemotePlugin:
    def __init__(self,host,plugin): self._host,self._plugin=host,plugin
    def __getattr__(self,name):
        if name.startswith('_'): raise AttributeError(name)
        return lambda *args:self._host.invoke('api.call',self._plugin,name,list(args))

class CommandBuffer:
    """Build in Python, then submit one array across the host boundary."""
    def __init__(self, host): self._host=host; self._commands=[]
    def add(self, command):
        if not isinstance(command,str): raise TypeError('command must be text')
        self._commands.append(command); return self
    def clear(self): self._commands.clear(); return self
    def size(self): return len(self._commands)
    def submitFast(self, options=None):
        return self._host.game.sendCommandsFast(self._commands,options or {})

class Host:
    def __init__(self,account,data_dir,port=None,plugin_id=None,module=None):
        self.account=account
        self.port=str(port or os.environ.get("YY_PORT","19429"))
        self.data_dir=Path(data_dir)
        self.data_dir.mkdir(parents=True,exist_ok=True)
        self.plugin_id=plugin_id
        self._module=module
        self._callbacks={}
	
        # One resident loop per Host, including the time between events. Do
        # not create/reset an asyncio.run loop for each callback.
        self._loop=asyncio.new_event_loop()
        self._loop_ready=threading.Event()
        def run_loop():
            asyncio.set_event_loop(self._loop)
            self._loop_ready.set()
            try:self._loop.run_forever()
            finally:
                asyncio.set_event_loop(None)
                self._loop.close()
        self._loop_thread=threading.Thread(target=run_loop,name='yy-async-'+str(plugin_id),daemon=True)
        self._loop_thread.start()
        self._loop_ready.wait()
        for namespace in ('game','system','api','plugin','util','bot','building','media','world','player','events','http','web','chains','packet','storage','accounts'):
            setattr(self,namespace,Namespace(self,namespace))

    def command_buffer(self): return CommandBuffer(self)

    def _callback(self,callback):
        for name,old in self._callbacks.items():
            if old is callback: return name
        name='py_callback_'+str(len(self._callbacks)+1)
        self._callbacks[name]=callback
        return name

    def invoke(self,method,*args):
        result=self.call('/api/youyuan/sdk/call',{'plugin_id':self.plugin_id,'account':self.account,'method':method,'args':list(args)})
        return result.get('result')

    def batch(self,calls):
        """One host request; ordered operations, stop on error, no rollback."""
        encoded=[{'method':method,'args':[self._callback(v) if callable(v) else v for v in args]} for method,args in calls]
        return self.call('/api/youyuan/sdk/batch',{'plugin_id':self.plugin_id,'account':self.account,'calls':encoded})['results']

    def _dispatch(self,event):
        kind=event.get('type')
        if kind in ('sdk_callback','sdk_rpc'):
            name=event.get('handler');args=event.get('args',[])
            if kind=='sdk_rpc':
                exports=getattr(self._module,'plugin_api',{})
                callback=exports.get(name) if isinstance(exports,dict) else None
            else: callback=self._callbacks.get(name) or getattr(self._module,name,None)
            if not callable(callback): raise ValueError('未注册插件函数: '+str(name))
            return {'ok':True,'result':self._resolve(callback(*args))}
        callback=getattr(self._module,'on_event',None)
        return self._resolve(callback(event,self)) if callable(callback) else {'ok':True}

    def _resolve(self,result):
        if not inspect.isawaitable(result):return result
        async def await_result():return await result
        future=asyncio.run_coroutine_threadsafe(await_result(),self._loop)
        try:return future.result(timeout=120)
        except BaseException:
            future.cancel()
            raise

    async def invoke_async(self,method,*args):
        """Offload synchronous host HTTP; keep the plugin loop responsive."""
        return await asyncio.to_thread(self.invoke,method,*args)

    async def call_async(self,path,body=None,method=None):
        return await asyncio.to_thread(self.call,path,body,method)

    def close_async(self):
        if self._loop.is_closed():return
        async def cleanup():
            tasks=[t for t in asyncio.all_tasks() if t is not asyncio.current_task()]
            for task in tasks:task.cancel()
            await asyncio.gather(*tasks,return_exceptions=True)
            await self._loop.shutdown_asyncgens()
            await self._loop.shutdown_default_executor(timeout=5)
        try:asyncio.run_coroutine_threadsafe(cleanup(),self._loop).result(timeout=10)
        finally:
            self._loop.call_soon_threadsafe(self._loop.stop)
            self._loop_thread.join(timeout=10)

    def call(self,path,body=None,method=None):
        if not path.startswith('/api/'):
            raise ValueError('仅允许工具箱 API')
        if body is not None and isinstance(body,dict) and 'account' in body:
            body={**body,'account':self.account}
        if path.startswith('/api/youyuan/runtime/'):
            raise ValueError('插件不能递归调用自己的调度器')
        request=urllib.request.Request('http://127.0.0.1:'+self.port+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'},method=method or ('POST' if body is not None else 'GET'))
        with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(request,timeout=60) as response:
            result=json.load(response)
        if isinstance(result,dict) and result.get('ok') is False:
            raise RuntimeError(result.get('error','游戏调用失败'))
        return result

    def console(self,text):
        return self.call('/api/youyuan/account/action',{'account':self.account,'action':'console','input':text})

    def position(self):
        return self.call('/api/youyuan/account/action',{'account':self.account,'action':'position'})

    def status(self):
        return self.call('/api/youyuan/account/action',{'account':self.account,'action':'status'})

    def state(self):
        path=self.data_dir/'state.json'
        return json.loads(path.read_text(encoding='utf-8')) if path.exists() else {}

    def save(self,value):
        path=self.data_dir/'state.json'
        temp=path.with_suffix('.tmp')
        temp.write_text(json.dumps(value,ensure_ascii=False),encoding='utf-8')
        temp.replace(path)

def dispatch(params_json):
    params=json.loads(params_json)
    key=params['key']
    try:
        if params['action']=='validate':
            try:
                compile(params['content'],params.get('path','main.py'),'exec')
                return json.dumps({'ok':True})
            except SyntaxError as error:
                return json.dumps({'ok':False,'error':'第 %s 行，第 %s 列：%s'%(error.lineno,error.offset,error.msg)},ensure_ascii=False)
        if params['action']=='start':
            with _lock:
                if key in _instances:raise RuntimeError('该实例已经运行')
                package_name='yy_'+hashlib.sha256(key.encode()).hexdigest()[:24]
                package=types.ModuleType(package_name)
                package.__path__=[str(Path(params['entry']).parent)]
                sys.modules[package_name]=package
                spec=importlib.util.spec_from_file_location(package_name+'.main',params['entry'])
                plugin_dir=str(Path(params['entry']).parent)
                if plugin_dir not in sys.path:sys.path.insert(0,plugin_dir)
                module=importlib.util.module_from_spec(spec)
                sys.modules[package_name+'.main']=module
                host=Host(params['account'],params['data_dir'],params.get('port'),params.get('plugin_id'),module)
                # Expose the familiar original API names before on_init runs.
                for namespace in ('game','system','api','plugin','util','bot','building','media','world','player','events','http','web','chains','packet','storage','accounts'):
                    if not hasattr(module,namespace): setattr(module,namespace,getattr(host,namespace))
                module.host=host
                spec.loader.exec_module(module)
                if hasattr(module,'on_init'):host._resolve(module.on_init(host))
                _instances[key]=(module,host,threading.RLock())
            result={'ok':True}
        elif params['action']=='stop':
            with _lock:item=_instances.pop(key,None)
            if item:
                with item[2]:
                    try:
                        if hasattr(item[0],'on_stop'):item[1]._resolve(item[0].on_stop(item[1]))
                    finally:
                        item[1].close_async()
                        prefix=item[0].__package__
                        for name in list(sys.modules):
                            if name==prefix or name.startswith(prefix+'.'):sys.modules.pop(name,None)
            result={'ok':True}
        elif params['action']=='events':
            with _lock:module,host,instance_lock=_instances[key]
            failures=[];last=None
            with instance_lock:
                for index,event in enumerate(params['events']):
                    try:
                        last=host._dispatch(event)
                        if isinstance(last,dict) and last.get('ok') is False:
                            failures.append({'index':index,'error':last.get('error','callback failed')})
                    except Exception as error:failures.append({'index':index,'error':str(error)})
            result={'ok':not failures,'processed':len(params['events']),'errors':failures,'last':last}
        else:
            with _lock:module,host,instance_lock=_instances[key]
            with instance_lock:result=host._dispatch(params['event'])
        return json.dumps(result,ensure_ascii=False)
    except Exception as error:
        if params['action']=='start' and 'package_name' in locals():
            if 'host' in locals():host.close_async()
            for name in list(sys.modules):
                if name==package_name or name.startswith(package_name+'.'):sys.modules.pop(name,None)
        return json.dumps({'ok':False,'error':str(error)},ensure_ascii=False)
