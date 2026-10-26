import hashlib, json, time
import paramiko
cfg = json.load(open(r"D:/YouYuan-APK/private/linux-access.json"))
pin = cfg["host_key_sha256"].replace(":", "").lower()
t = paramiko.Transport((cfg["host"], 22))
t.start_client(timeout=30)
hk = t.get_remote_server_key()
assert hashlib.sha256(hk.asbytes()).hexdigest() == pin
t.auth_password(cfg["username"], cfg["password"])
sftp = paramiko.SFTPClient.from_transport(t)
blob = b"\0" * (2 * 1024 * 1024)
st = time.time()
with sftp.open("/tmp/yy_speedtest.bin", "wb") as f:
    f.write(blob)
dt = time.time() - st
print(f"2MB 上传耗时 {dt:.1f}s → {2/dt:.2f} MB/s；52MB 预计 {52/(2/dt)/60:.1f} 分钟")
sftp.remove("/tmp/yy_speedtest.bin")
t.close()
