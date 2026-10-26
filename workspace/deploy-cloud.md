# 部署：Cloudflare Worker / 认证站 / box Linux

> 适用对象：本机 `C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project`。
> 三条链路**互相独立**：网页挂了不影响游戏，游戏挂了不影响网页。历史上最大的事故就是把「网页 200」当成「Linux 内核正常」。
> 授权分级（`../agent-hub/AUTHORIZATION.md`）：A 只读/本地构建默认允许；B 本地写需授权；**C 生产写（`wrangler deploy`、重启 systemd、`git push`、APK 上架）逐项确认**。

---

## 0. 前置检查（30 秒）

```bash
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"

git status --short | head -30          # 谁在改？有 dirty 不是垃圾，禁止 checkout -- .
git log --oneline -3                   # 记住当前回滚点
date +%Y%m%d-%H%M%S                   # 后面所有备份名用这个戳

# 工具链
"C:/Program Files/Go/bin/go.exe" version
node -v
D:/YouYuan-Codex/.venv/Scripts/python.exe -c "import paramiko,httpx;print('py ok')"
```

| 用途 | 路径 / 值 |
|---|---|
| Node（构建前端用） | `C:/Users/Lenovo/.workbuddy-ai/binaries/node/versions/22.22.2-6/node.exe` |
| wrangler 4.149.0 | `C:/Users/Lenovo/AppData/Local/npm-cache/_npx/31df52248099582d/node_modules/wrangler/bin/wrangler.js` |
| wrangler 登录态 | `C:/Users/Lenovo/AppData/Roaming/xdg.config/.wrangler/config/default.toml`（OAuth，别打印内容） |
| Python（paramiko/httpx） | `D:/YouYuan-Codex/.venv/Scripts/python.exe -u -X utf8` |
| 本地代理 | `http://127.0.0.1:9674`（GitHub / Cloudflare 直连会被重置） |

> `npx` 缓存目录哈希可能随重装变化；找不到就用 `npx wrangler --version` 让它自己解析，或 `find ~/AppData/Local/npm-cache/_npx -maxdepth 3 -name wrangler -type d`。

---

## 1. 三条链路总览

```
┌─────────────────────────────┐
│ upstream/frontend  (源码)   │
└──────────┬──────────────────┘
           │ build_frontend.py（复制 + esbuild 压缩）
           ▼
┌─────────────────────────────┐        wrangler deploy        ┌──────────────────────────────┐
│ cloudflare/public  (产物)   │ ────────────────────────────► │ Worker youyuan-apk-v030       │
└─────────────────────────────┘                               │ toolbox / updates 两个域名     │
                                                              │ /api/* /mcp 反代到 Linux 中继  │
                                                              └──────────────┬───────────────┘
                                                                             │ WebSocket relay
┌─────────────────────────────┐   build.ps1(GOOS=js/wasm)     ┌──────────────▼───────────────┐
│ auth-service/engine.go      │ ────────────────────────────► │ Worker youyuan-game-auth      │
│ auth-service/public         │        wrangler deploy        │ auth.youyuanqi.dpdns.org      │
└─────────────────────────────┘                               └──────────────────────────────┘

┌─────────────────────────────┐   GOOS=linux go build + seal   ┌──────────────────────────────┐
│ upstream/*.go + frontend/** │ ────────────────────────────► │ 47.102.156.125                │
└─────────────────────────────┘   deploy_linux_bin.py (SFTP)   │ systemd youyuan.service:19429 │
                                                              │ /opt/youyuan/youyuan          │
                                                              │ /var/lib/youyuan  (数据)      │
                                                              └──────────────────────────────┘
```

---

## 2. 链路 A：Cloudflare Worker（网页控制台 + 更新分发 + 中继）

### 2.1 它是什么

- 脚本名：`youyuan-apk-v030`（`cloudflare/wrangler.jsonc:2`）
- 入口：`cloudflare/worker.mjs`
- 静态资源：`cloudflare/public/`（`assets.directory`，绑定 `ASSETS`）
- 路由：`toolbox.youyuanqi.dpdns.org`、`updates.youyuanqi.dpdns.org`（自定义域，自动证书）
- 反向代理：`/api/*`、`/mcp`、`/device/connect`、`/qq/webhook`、`/health` → Linux 中继
- 依赖服务绑定：`AUTH_SERVICE` → `youyuan-game-auth`（`wrangler.jsonc` 的 `services`）
- `run_worker_first` 列表决定哪些路径先过 Worker 再回落到静态资源；改路由必须同步这里

### 2.2 构建前端（必做，不能跳）

```bash
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"

# 1) 先看会改哪些文件（不落盘）
D:/YouYuan-Codex/.venv/Scripts/python.exe maintenance/build_frontend.py --dry-run

# 2) 正式构建：会先 rmtree/copytree 覆盖 cloudflare/public，再 esbuild --minify
D:/YouYuan-Codex/.venv/Scripts/python.exe maintenance/build_frontend.py
```

它做四件事（`maintenance/build_frontend.py:1-16`）：

1. `upstream/frontend/` → `cloudflare/public/`（**整目录覆盖**，`public/` 里手放的东西会被删）
2. 所有 `public/js/**/*.js` 用 esbuild `--minify --target=es2019 --legal-comments=none`
3. 所有 `public/css/*.css` 同样压缩
4. 删除任何 `.map`（铁律：线上不能有 sourcemap）

输出末尾有逐文件压缩率清单。看到 `DONE — 可以 wrangler deploy` 才继续。

### 2.3 部署

```bash
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/cloudflare"

# 推荐写法：显式 node + wrangler.js，不依赖 PATH
"C:/Users/Lenovo/.workbuddy-ai/binaries/node/versions/22.22.2-6/node.exe" \
  "C:/Users/Lenovo/AppData/Local/npm-cache/_npx/31df52248099582d/node_modules/wrangler/bin/wrangler.js" \
  deploy 2>&1 | tee /tmp/wrangler-deploy-$(date +%H%M%S).log
```

要点：

- **必须在 `cloudflare/` 目录里执行**，`wrangler.jsonc` 按 cwd 读取。
- 记下输出里的 `Version ID`（形如 `e42572e2-…`），回滚要用。
- 看到 `Uploaded N assets` + `Deployed` 才算成功；只有 `Uploaded` 没有 `Deployed` = 中断，线上没变。
- Windows 下如果 `tee` 不可用，直接看终端输出并复制 Version ID 到交接记录。

### 2.4 验证（四项，缺一不算部署成功）

```bash
# ① 主站可达 + 返回的是新首页
curl -sS -o /dev/null -w "%{http_code} %{size_download}\n" https://toolbox.youyuanqi.dpdns.org/index.html
curl -sS https://toolbox.youyuanqi.dpdns.org/index.html | grep -o "youyuan_seedmap_v2.js?v=[0-9]*"

# ② 接口层活着（Worker→Linux 中继）。/health 属于受保护路径，必须带控制令牌；
#    不带令牌返回 401 {code:"AUTH_REQUIRED"} 是正常的，不代表服务挂了。
TOKEN=$(D:/YouYuan-Codex/.venv/Scripts/python.exe -c "import json;print(json.load(open(r'D:/YouYuan-APK/private/cloud-keys.json'))['CONSOLE_TOKEN'])")
curl -sS -H "Authorization: Bearer $TOKEN" https://toolbox.youyuanqi.dpdns.org/health
#   期望：{"ready":true,"version":"0.10.7x",...}；502 说明 Linux 中继/设备离线，去链路 C 查

# ③ 更新清单与线上 APK 一致
curl -sS -A "Mozilla/5.0" https://updates.youyuanqi.dpdns.org/updates/android/latest.json | head -30
#   核对 version / version_code / sha256 / parts 之和 == size

# ④ 静态资源回源正确（MIME 与缓存）
curl -sSI https://toolbox.youyuanqi.dpdns.org/js/youyuan_seedmap_v2.js | sed -n '1,12p'
```

再打开浏览器硬刷新（Ctrl+F5）确认：控制台能开、插件页有数据、`/mcp` 的 `initialize` 不 401。

### 2.5 常见坑（都踩过）

| 现象 | 根因 | 处理 |
|---|---|---|
| 部署日志无报错但线上还是旧版 | 资产上传在 22/34 个文件处中断，**中断不会激活部署** | 先核对线上 `latest.json`/版本，再重试；差量上传几秒完成 |
| 页面改了没生效 | 浏览器/CDN 缓存了旧 `?v=` | 改 `index.html` 的版本戳（149 处 `?v=2026102824` + 2 处 `?v=2026102825`），见 `module-update.md` |
| 改了 `cloudflare/public/js/x.js` 一部署就丢 | `build_frontend.py` 整目录覆盖 | 只改 `upstream/frontend/`，`public/` 当只读产物 |
| `/api/*` 404 或落到静态资源 | `run_worker_first` 没包含该前缀 | 在 `wrangler.jsonc` 补路径，重新 deploy |
| `/health` 502 | Worker 正常，Linux 设备离线/中继断开 | 去链路 C 查 systemd 与 19429，不要反复重发 Worker |
| 部署卡在登录 | OAuth token 过期 | `npx wrangler login`（需代理），或换用有权限的账号 |
| `wrangler` 命令找不到 | 没有本地 `node_modules`，也没全局安装 | 用 §0 的绝对路径或 `npx wrangler` |

---

## 3. 链路 B：独立认证站 `youyuan-game-auth`

### 3.1 它是什么

- 独立 Worker，**不是**主站的子路径：`auth.youyuanqi.dpdns.org`
- `auth-service/worker.mjs` + Durable Objects（`AuthVault`、`RELAY`）
- Go 认证内核编成 WASM：`auth-service/engine.go` → `kernel.wasm`
- `RELAY` 的 `script_name` 指向 `youyuan-apk-v030`（跨 Worker DO 绑定，改名会断）
- 主站通过 `AUTH_SERVICE` 服务绑定调用它
- 独立版本号与主程序不同（当前 0.10.70 系列），**不要**跟着 `yyAppVersion` 自动改

### 3.2 构建 wasm

```powershell
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/auth-service"
powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1
```

脚本做三件事（`auth-service/build.ps1:1-13`）：

1. 从 `$(go env GOROOT)/lib/wasm/wasm_exec.js` 复制运行时到本目录
2. 复制 Go LICENSE
3. `GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o kernel.wasm .`

**改了 `engine.go` 必须重编 wasm 再部署**，只 deploy 不改 wasm 等于没改。
**改了 Go 版本必须同时更新 `wasm_exec.js`**：运行时与 wasm 不匹配会报 `Go version mismatch`。

### 3.3 部署

```bash
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/auth-service"
"C:/Users/Lenovo/.workbuddy-ai/binaries/node/versions/22.22.2-6/node.exe" \
  "C:/Users/Lenovo/AppData/Local/npm-cache/_npx/31df52248099582d/node_modules/wrangler/bin/wrangler.js" deploy
```

### 3.4 验证

```bash
curl -sS https://auth.youyuanqi.dpdns.org/health
curl -sS https://auth.youyuanqi.dpdns.org/api/turnstile-config
# 打开 https://auth.youyuanqi.dpdns.org/ 看登录框、Turnstile 是否加载
```

再回主站做一次「注册/登录」链路冒烟（发码接口有真实邮件依赖，别在生产上反复刷）。

### 3.5 坑

| 现象 | 根因 | 处理 |
|---|---|---|
| `Go version mismatch` / wasm 起不来 | `wasm_exec.js` 与 `kernel.wasm` 不是同一次 Go 版本 | 重跑 `build.ps1`，两个文件一起换 |
| 登录 500，日志指向 DO | `AuthVault` 的 SQLite 迁移没应用 | `wrangler.jsonc` 的 `migrations` 只能**新增 tag**，不要改旧 tag 的定义 |
| 主站调认证 404 | 主站 `services.AUTH_SERVICE` 与服务名不一致 | 两边 `wrangler.jsonc` 对齐 `youyuan-game-auth` |
| 中继建连失败 | `RELAY.script_name` 写错或主 Worker 改名 | 改回 `youyuan-apk-v030` |
| 注册卡在「正在注册」 | 客户端无超时（已加 80s）+ 服务端 `relay.fetch()` 仍无超时 | 先看 `/health`；不要把前端转圈当成后端慢 |
| R2 密钥末尾被截 | 历史 `TrimRight` 缺陷已修 | **不得回退**该修复；回滚认证要避开 0.10.70 之前版本 |

---

## 4. 链路 C：box / Linux 生产机

### 4.1 它是什么

- 主机 `47.102.156.125`，root，SSH 22，主机密钥 pin `24bd2547…59bc`（sha256(公钥 blob) 的 hex，**不是** `get_fingerprint()` 的 MD5）
- systemd 单元 `linux/youyuan.service`：
  - `ExecStart=/opt/youyuan/youyuan`
  - `Environment=YY_DATA_DIR=/var/lib/youyuan`
  - `Environment=YY_PORT=19429`
  - `Restart=always`、`RestartSec=10`、`UMask=0077`
- 页面与 API 都由这个二进制提供：前端是 `//go:embed frontend/**` 编进去的
- 数据（账号、插件、命令链、配置、皮肤缓存）全在 `/var/lib/youyuan`

### 4.2 构建

**推荐**：直接跑 `build.ps1`，一次出三平台并保证前端摘要一致：

```powershell
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"
powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1
# 产物：build/youyuan-windows.exe、build/youyuan-linux-x86_64、build/youyuan-v0.10.1.apk（文件名沿用旧模板）
```

**只重编 Linux（明确知道自己在做什么时）**：

```bash
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/upstream"

STAMP=$(date +%Y%m%d-%H%M%S)

# 1) 计算内嵌前端摘要，算法必须与 build.ps1 / integrity.go 逐字节一致
FRONTEND_HASH=$(D:/YouYuan-Codex/.venv/Scripts/python.exe - <<'PY'
import hashlib,os
root=r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/upstream/frontend"
files=[]
for dp,_,fs in os.walk(root):
    for f in fs:
        p=os.path.join(dp,f)
        rel=os.path.relpath(p,root).replace("\\","/")
        files.append(("frontend/"+rel,p))
files.sort(key=lambda x:x[0].encode())
sb="".join(k+"\n"+hashlib.sha256(open(p,"rb").read()).hexdigest()+"\n" for k,p in files)
print(hashlib.sha256(sb.encode()).hexdigest())
PY
)

# 2) 编译（CGO 关闭 = 全静态，任何发行版都能跑）
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  "C:/Program Files/Go/bin/go.exe" build -trimpath \
  -ldflags="-s -w -buildid= -X main.buildStamp=$STAMP -X main.frontendHash=sha256:$FRONTEND_HASH" \
  -o ../build/youyuan-linux-x86_64 .

# 3) 封印（把自身 sha256 回填槽位；之后不能再 strip/upx）
D:/YouYuan-Codex/.venv/Scripts/python.exe ../maintenance/seal_binary.py ../build/youyuan-linux-x86_64
D:/YouYuan-Codex/.venv/Scripts/python.exe ../maintenance/seal_binary.py --verify ../build/youyuan-linux-x86_64
```

> 只写 `go build` 不注入 `frontendHash` 也能编过，但运行时会**跳过**完整性校验（`integrity.go:228-244` 的「两者都空 = 开发构建」分支）。这种包可以自测，**不要当发布产物**。
> 打 PIE 用 `build.ps1 -Pie`；默认全静态 ET_EXEC 是为了兼容 musl/极小镜像。

### 4.3 部署

```bash
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"
D:/YouYuan-Codex/.venv/Scripts/python.exe -u maintenance/deploy_linux_bin.py
```

脚本固定流程（`maintenance/deploy_linux_bin.py:25-80`）：

1. 读 `D:/YouYuan-APK/private/linux-access.json`，校验主机密钥 pin
2. 本地 sha256
3. SFTP 上传到 `/opt/youyuan/youyuan.new`
4. 远端 `sha256sum` 与本地比对（**双验**，不一致直接 return 4）
5. `chmod +x` / `chown root:root`
6. `cp -f /opt/youyuan/youyuan /opt/youyuan/youyuan.rollback`（回滚点）
7. `systemctl stop` → `mv` 原子替换 → `systemctl start`
8. `systemctl is-active`、`ss -tlnp | grep 19429`、`journalctl -n 6`

相关脚本：
- `maintenance/deploy_linux.py`：只探查（不替换），排障用
- `maintenance/deploy_throttled.py`：限速上传，链路差时用
- `maintenance/box_ssh_exec.py`：单条远端命令
- `maintenance/rebuild_box.py`：**系统重置后**从零重建（会恢复配置/插件/隧道）——不是日常部署

### 4.4 验证

```bash
# 在服务器上（本机 /api/* 目前零鉴权，属已知 H1 问题；不要照这个对外暴露 19429）
curl -sS http://127.0.0.1:19429/health

# 从本机走 Worker 中继（/health 受保护，必须带控制令牌）
TOKEN=$(D:/YouYuan-Codex/.venv/Scripts/python.exe -c "import json;print(json.load(open(r'D:/YouYuan-APK/private/cloud-keys.json'))['CONSOLE_TOKEN'])")
curl -sS -H "Authorization: Bearer $TOKEN" https://toolbox.youyuanqi.dpdns.org/health
```

期望：`ready:true`、`version` 与刚构建的一致。再核：

| 检查 | 命令（远端） | 通过标准 |
|---|---|---|
| 进程 | `systemctl is-active youyuan.service` | `active` |
| 端口 | `ss -tlnp \| grep 19429` | 有监听，pid 与 service 一致 |
| 二进制 | `sha256sum /opt/youyuan/youyuan` | 与本地一致 |
| 启动日志 | `journalctl -u youyuan.service -n 50 --no-pager` | 无 panic / 无 integrity 失败 |
| 数据目录 | `ls -la /var/lib/youyuan` | 账号/插件/链 TXT 仍在，时间戳未被重置 |
| 业务 | `curl -s 127.0.0.1:19429/api/bot/status` | `connected` 与账号状态符合预期 |

### 4.5 坑（每条都真实踩过）

| 坑 | 说明 | 正确做法 |
|---|---|---|
| **数据目录漂移** | 手工 `./youyuan` 且没设 `YY_DATA_DIR` 时，`main_native.go:12-16` 会落到 `~/.config/YouYuanToolbox`，看起来「账号和插件全没了」 | 一律用 systemd 启动；手工排障也必须 `YY_DATA_DIR=/var/lib/youyuan YY_PORT=19429 ./youyuan` |
| **前端没更新** | 前端编进二进制；只 `wrangler deploy` 不会更新 box 页面 | 改前端后重编 Linux 二进制，或确认访问的是 Worker 静态页面 |
| **端口写错** | 历史文档写 19439 | 正确端口 **19429** |
| **pin 校验失败** | 把 `ssh-keyscan` 的 MD5 指纹当 pin | pin 是 `sha256(公钥blob)` 的 hex；脚本自己算 |
| **封印后又被改写** | `strip`/`upx`/二次 `go build` 覆盖会破坏封签 | `seal_binary.py` 最后一步；产物不要再动 |
| **完整性失败** | 改了 `upstream/frontend` 但没重编 | 前端摘要不匹配会拒绝启动（`integrity.go:228-244`） |
| **自动更新定时器** | `youyuan-update.timer` 每 6h 拉更新，但历史记录指出没有 Linux 专用 manifest | 先 `systemctl list-timers --all`、`systemctl cat youyuan-update.timer` 看清再决定是否 disable |
| **版本号漏改** | 只改 `youyuan_product.go` 没改 `build.gradle`（或反之） | 两处一起改；versionCode 只能增不能降 |
| **并行 Agent 覆盖** | 远端可能被另一个流程刚更新 | 部署前先读远端 `sha256sum` 与 `协作交接/维护入口.md` 顶部，确认没人正在发 |
| **`cmd/server` 编译失败** | 结构性问题（`package main` 导入同包），不是你的改动 | 忽略；构建脚本不用它 |
| **race 检测不可用** | 本机无 gcc，`CGO_ENABLED=0` | 不要声称跑过 `-race` |
| **`StartServer` 端口占用** | 旧进程未停就启新的 | 脚本已先 stop；手工时先 `systemctl stop` |

### 4.6 已知但不在本文修复范围的问题

- Linux 直连隧道（`linux/youyuan-direct-tunnel.service`）健康检查偶发失败，直连只覆盖 `/api/*` 与 `/health`
- 默认游戏出口是 Linux 系统 UDP，**不是玩家手机出口**（需求 E06 未完成）
- 真实多人峰值、跨维度长跑未验收

---

## 5. 回滚

### 5.1 Worker

```bash
cd cloudflare
node .../wrangler.js deployments list        # 找上一个正常 Version ID
node .../wrangler.js rollback <version-id>
```

回滚后同样跑 §2.4 四项验证。

### 5.2 认证站

```bash
cd auth-service
node .../wrangler.js deployments list
node .../wrangler.js rollback <version-id>
```

注意：**代码可回滚，Durable Object 的 SQLite 迁移不可回滚**。如果新版本只加了字段通常安全；要改/删旧类必须先设计双写或新类。

### 5.3 Linux

```bash
# 方式一：用部署脚本留下的同目录回滚副本
ssh root@47.102.156.125
systemctl stop youyuan.service
cp -f /opt/youyuan/youyuan.rollback /opt/youyuan/youyuan
systemctl start youyuan.service

# 方式二：用带时间戳的备份（更早的版本）
ls -lt /var/backups/youyuan/ | head
```

回滚后核对：`is-active`、`19429`、`/health` 的 version、`/var/lib/youyuan` 未变。

### 5.4 APK

- 已安装用户：`versionCode` 不能降，回滚包对已升级用户**装不回去**；只能发更高 versionCode 的修复版
- 未安装用户：把 `latest.json` + 分片恢复到上一版并 deploy 即可
- 证书 `cd823622…` 永远不变，否则无法覆盖安装

---

## 6. 发布前检查清单（逐项打勾）

- [ ] `git status` 已看，知道哪些改动是自己的
- [ ] 版本号：`upstream/youyuan_product.go` 与 `upstream/android/app/build.gradle` 一致且已递增
- [ ] 前端：`index.html` 版本戳已更新（含 JS 内自注入 CSS 的版本，见 `module-update.md`）
- [ ] `node --check` 改过的每个 JS
- [ ] `go vet .` + 关联 `go test`（至少 `. ./config ./state ./building`）
- [ ] `node --test cloudflare/*.test.mjs auth-service/*.test.mjs`
- [ ] `build_frontend.py --dry-run` 无异常，正式构建输出无 `.map`
- [ ] wrangler 部署前已确认线上基线（`latest.json`、`/health`）
- [ ] Linux 部署前已确认远端当前 sha 与最近一次交接记录
- [ ] 三个域名 + 19429 全部验证通过
- [ ] 证据写进 `outputs/YYYY-MM-DD-*.md`，更新 `协作交接/维护入口.md` 顶部
- [ ] 备份/回滚点名称带时间戳

---

## 7. 出问题先看哪里

| 症状 | 先查 |
|---|---|
| 网页打不开 | `curl -I toolbox…` → 若 5xx 看 wrangler deployments；若 200 但白屏看浏览器 console + `index.html` 版本戳 |
| 网页能开，功能全 401 | 登录态/中继密钥；`/health`；`cloudflare/wrangler.jsonc` 的 `run_worker_first` |
| 游戏连不上 | `/api/bot/status` → Linux `journalctl` → 账号是否被踢 → 出口 UDP |
| 更新下载 502 | `latest.json` 的 `size` 与 parts 之和不一致；`updates.mjs` 按 parts 拼装 |
| 认证注册卡住 | `auth…/health`；中继；客户端 80s 超时提示；服务端 `relay.fetch()` 仍无超时 |
| 部署后数据「没了」 | 十有八九是数据目录漂移（§4.5），不是真丢 |
