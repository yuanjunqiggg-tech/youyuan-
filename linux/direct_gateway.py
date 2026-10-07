"""Linux direct transport. Same owner identity; no unauthenticated control API."""
import hashlib, hmac, json, os, threading, time, urllib.request, urllib.error
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
CONFIG=json.loads(Path(os.environ.get('YY_GATEWAY_CONFIG','/etc/youyuan/direct-gateway.json')).read_text())
CORE='http://127.0.0.1:19429'; CLOUD='https://toolbox.youyuanqi.dpdns.org'
ALLOWED_ORIGINS={'https://toolbox.youyuanqi.dpdns.org','https://youyuan-apk-v030.yuanjunqiggg.workers.dev'}
cache={};auth_locks={};lock=threading.Lock();metrics={'requests':0,'cloud_auth_checks':0,'denied':0}
def authenticate(token):
 if not token:return False
 if hmac.compare_digest(token,CONFIG['console_token']):return True
 digest=hashlib.sha256(token.encode()).hexdigest()
 with lock:
  if len(auth_locks)>1024:auth_locks.clear()
  guard=auth_locks.setdefault(digest,threading.Lock())
 with guard:
  with lock:
   record=cache.get(digest)
   if record and record[0]>time.monotonic():return record[1]
  try:
   req=urllib.request.Request(CLOUD+'/api/identity/me',headers={'Authorization':'Bearer '+token,'User-Agent':'YouYuan-LinuxGateway/0.10.14','Accept':'application/json'})
   with lock:metrics['cloud_auth_checks']+=1
   with urllib.request.urlopen(req,timeout=15)as response:data=json.load(response)
   user=data.get('user',{});ok=data.get('ok') and user.get('id')==CONFIG['owner_id'] and user.get('cloud_allowed')
  except Exception:ok=False
  with lock:
   if len(cache)>1024:cache.clear()
   cache[digest]=(time.monotonic()+(60 if ok else 3),bool(ok))
  return bool(ok)
class Server(ThreadingHTTPServer):
 daemon_threads=True; slots=threading.BoundedSemaphore(32)
 def process_request(self,request,address):
  if not self.slots.acquire(False):
   try:request.sendall(b'HTTP/1.1 429 Too Many Requests\r\nContent-Length: 0\r\nConnection: close\r\n\r\n')
   finally:self.shutdown_request(request)
   return
  try:super().process_request(request,address)
  except Exception:self.slots.release();raise
 def process_request_thread(self,*args):
  try:super().process_request_thread(*args)
  finally:self.slots.release()
class Handler(BaseHTTPRequestHandler):
 protocol_version='HTTP/1.1'
 def log_message(self,*args):pass
 def cors(self):
  origin=self.headers.get('Origin','')
  if origin in ALLOWED_ORIGINS:self.send_header('Access-Control-Allow-Origin',origin);self.send_header('Vary','Origin')
 def respond(self,status,data):
  raw=json.dumps(data,ensure_ascii=False).encode();self.send_response(status);self.cors();self.send_header('Content-Type','application/json');self.send_header('Cache-Control','no-store');self.send_header('Content-Length',str(len(raw)));self.end_headers();self.wfile.write(raw)
 def do_OPTIONS(self):
  if self.headers.get('Origin')not in ALLOWED_ORIGINS:self.respond(403,{'ok':False});return
  self.send_response(204);self.cors();self.send_header('Access-Control-Allow-Methods','GET,POST,PUT,DELETE,HEAD,OPTIONS');self.send_header('Access-Control-Allow-Headers','Authorization,Content-Type,X-YouYuan-Device,X-YouYuan-Market-Authorization,MCP-Protocol-Version,Accept');self.send_header('Access-Control-Max-Age','600');self.send_header('Content-Length','0');self.end_headers()
 def do_GET(self):self.proxy()
 def do_HEAD(self):self.proxy()
 def do_POST(self):self.proxy()
 def do_PUT(self):self.proxy()
 def do_DELETE(self):self.proxy()
 def proxy(self):
  with lock:metrics['requests']+=1
  origin=self.headers.get('Origin');token=self.headers.get('Authorization','').removeprefix('Bearer ')
  if origin and origin not in ALLOWED_ORIGINS:self.respond(403,{'ok':False,'error':'来源未授权'});return
  path=self.path
  protected=path.startswith(('/api/','/mcp','/health'))
  login=path.split('?')[0]=='/api/identity/login' and self.command=='POST'
  if protected and not login and not authenticate(token):
   with lock:metrics['denied']+=1
   self.respond(401,{'ok':False,'error':'请以幽怨qi登录；此Linux入口只允许项目所有者'});return
  if path=='/api/cloud/direct-metrics':self.respond(200,{'ok':True,**metrics});return
  if path.startswith('/api/identity/register'):self.respond(403,{'ok':False,'error':'当前为单账号运行，注册请联系管理员'});return
  try:
   size=int(self.headers.get('Content-Length','0'))
   if size<0 or size>32*1024*1024:self.respond(413,{'ok':False,'error':'单次上传超过32MB'});return
   if self.headers.get('Transfer-Encoding'):self.respond(400,{'ok':False,'error':'请使用Content-Length上传'});return
   data=self.rfile.read(size)if size else None
   if login:
    body=json.loads(data or b'{}')
    if body.get('username','').strip().lower()!=CONFIG['owner_username']:self.respond(403,{'ok':False,'error':'仅保留幽怨qi账号'});return
   target=(CLOUD if path.startswith('/api/identity/')else CORE)+path
   headers={name:self.headers[name]for name in ['Content-Type','Accept','Authorization','MCP-Protocol-Version','X-YouYuan-Market-Authorization']if name in self.headers};headers['User-Agent']='YouYuan-LinuxGateway/0.10.14'
   if not target.startswith(CLOUD)and path.startswith('/mcp'):headers['Authorization']='Bearer '+CONFIG['mcp_key']
   req=urllib.request.Request(target,data=data,method=self.command,headers=headers)
   try:response=urllib.request.urlopen(req,timeout=155)
   except urllib.error.HTTPError as error:response=error
   with response:
    self.send_response(response.status);self.cors()
    for key in ['Content-Type','Content-Disposition','Content-Length','X-Content-SHA256','Retry-After']:
     if key in response.headers:self.send_header(key,response.headers[key])
    self.send_header('Cache-Control','no-store');self.send_header('Connection','close');self.end_headers();self.close_connection=True
    if self.command!='HEAD':
     while True:
      chunk=response.read(65536)
      if not chunk:break
      self.wfile.write(chunk)
   if path.startswith('/api/identity/logout'):
    with lock:cache.pop(hashlib.sha256(token.encode()).hexdigest(),None)
  except (BrokenPipeError,ConnectionResetError):pass
  except Exception:self.respond(502,{'ok':False,'error':'Linux本机接口响应失败'})
if __name__=='__main__':Server(('127.0.0.1',19430),Handler).serve_forever()
