"""Publish the temporary URL to the core health response on every restart."""
import subprocess,re,json,os,signal
from pathlib import Path
destination=Path('/var/lib/youyuan/linux-direct.json')
destination.write_text(json.dumps({'url':''}))
p=subprocess.Popen(['/opt/youyuan/cloudflared','tunnel','--no-autoupdate','--protocol','http2','--url','http://127.0.0.1:19430'],stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True)
def stop(*args):p.terminate()
signal.signal(signal.SIGTERM,stop);signal.signal(signal.SIGINT,stop)
try:
 for line in p.stdout:
  match=re.search(r'https://[a-z0-9-]+\.trycloudflare\.com',line)
  if match:
   temp=destination.with_suffix('.new');temp.write_text(json.dumps({'url':match[0]}));os.replace(temp,destination);print('Linux direct URL: '+match[0],flush=True)
 p.wait()
finally:
 destination.write_text(json.dumps({'url':''}));p.terminate()
raise SystemExit(p.returncode or 1)
