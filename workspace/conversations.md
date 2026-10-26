# 历史对话：怎么查、查到了什么、决定了什么

> 完整路径表在 `skills.md` §3；本文只讲**检索策略**与**必须记住的结论**。
> 隐私边界（硬规则）：只检索本项目自己的会话目录与项目文档；**不扫描其他客户端/其他 AI 应用的私有目录、会话、Token、Cookie**（`协作交接/维护入口.md` 2026-10-20 段明确禁止）。

---

## 1. 先查哪一层（按成本从低到高）

| 顺序 | 数据源 | 适合查什么 | 成本 |
|---|---|---|---|
| ① | `输出蒸馏`：`outputs/_user_messages_readable.txt`（266 条用户原话 / 2074 行） | **用户到底怎么说的**、原始需求、抱怨点 | 秒级 |
| ② | `协作交接/维护入口.md` 顶部 | 当前状态、最近一次发布/事故 | 秒级 |
| ③ | `outputs/*.md`（按日期） | 某次交付的证据、根因、未完成项 | 秒级 |
| ④ | `需求逐项总清单.json` / `项目待办.md` | 需求编号、状态、验证层 | 秒级 |
| ⑤ | WorkBuddy `.jsonl` | 当时的完整推理、命令、报错原文 | 分钟级 |
| ⑥ | Codex `rollout-*.jsonl` | 最早期的需求与架构讨论 | 分钟级 |
| ⑦ | `git log -p` | 代码到底改了什么 | 分钟级 |

**纪律**：先用 ①–④ 把问题收敛成「哪一天、哪个编号、哪个文件」，再去 ⑤–⑦ 找原文。直接全量 grep 几百 MB 的 jsonl 只会把上下文撑爆。

---

## 2. 检索命令

### 2.1 最快：搜用户原话

```bash
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"
grep -n -B2 -A8 "种子" outputs/_user_messages_readable.txt | head -100
grep -n "部署\|服务器\|skill\|会话" outputs/_user_messages_readable.txt | head -50
```

文件格式：`===== #序号 | ISO时间 | 来源 rollout 路径 =====` 后接正文，可以直接定位到某次会话。

### 2.2 搜某一天的交接结论

```bash
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"
grep -n "^>" 协作交接/维护入口.md | head -40      # 每条都是「时间 + 结论 + 链接」
grep -rn "2026-10-2[0-6]" outputs/*.md | head -30
```

### 2.3 搜 WorkBuddy 原始会话（只在本项目目录内）

```bash
DIR="C:/Users/Lenovo/.workbuddy-ai/projects/c-Users-Lenovo-Documents-Codex-2026-09-28-apk-project"
grep -rl "结构显示\|一滩\|种子查询" "$DIR" --include="*.jsonl"
```

按角色过滤（Node，无需 jq）：

```bash
NODE="C:/Users/Lenovo/.workbuddy-ai/binaries/node/versions/22.22.2-6/node.exe"
"$NODE" -e '
const fs=require("fs"),path=require("path");
const dir=process.argv[1], kw=process.argv[2];
for(const f of fs.readdirSync(dir).filter(x=>x.endsWith(".jsonl"))){
  const txt=fs.readFileSync(path.join(dir,f),"utf8").split(/\r?\n/);
  for(const l of txt){ if(!l.trim())continue; let o; try{o=JSON.parse(l)}catch(e){continue}
    if(o.type!=="message"||o.role!=="user")continue;
    const t=(o.content||[]).filter(c=>c.type==="input_text").map(c=>c.text).join(" ");
    if(t.includes(kw)) console.log("\n["+f+"] "+new Date(o.timestamp).toISOString()+"\n"+t.slice(0,600));
  }
}' "$DIR" "种子"
```

### 2.4 搜 Codex 早期讨论

```bash
cd "C:/Users/Lenovo/.codex/sessions"
grep -rl "工具箱\|种子\|构建" --include="*.jsonl" . | tail -10
grep -o '.\{0,150\}种子.\{0,250\}' 2026/10/19/*.jsonl | head -20
```

### 2.5 还原某次改动

```bash
cd "C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"
git log --oneline --all --since="2026-10-19" | head -30
git show --stat <commit>
git log -S "yyV2RenderSampleOff" --oneline -- upstream/
```

---

## 3. 关键决策纪要（按主题，不按时间）

### 3.1 交付与验收

| 决定 | 理由（来自哪次事故） |
|---|---|
| 四层验收：静态核实 → 可构建 → 已运行 → 端到端 | 多次把「本地测试绿」说成「完成」，真机一测就崩 |
| 「网页能开」≠「游戏能用」≠「QQ 真送达」 | Worker 中继正常但 Linux 内核掉线；QQ 接口 200 但群里没消息 |
| 每个「完成」必须带 evidence 文件 | 口头完成导致总账与事实脱节；`agent-hub/DECISIONS.md` D3 直接把「无证据完成任务」设为失败 |
| 总账只增量更新 | 重跑旧 `update/record_acceptance` 会把后来的状态写回旧版本 |

### 3.2 部署纪律

| 决定 | 说明 |
|---|---|
| 生产写（deploy/重启/上架/push）逐项确认 | 曾有一次性授权被误当成长期授权 |
| 部署前先备份 + 时间戳 | Linux 保留 `/var/backups/youyuan/*` 与同目录 `.rollback` |
| 不覆盖并行 Agent 的部署 | 多流程同时发版，远端 sha 可能刚被改；先读交接与远端 sha |
| 不把构建产物入库 | `build/`、`cloudflare/public/`、`private/` 均在 gitignore |
| 签名与 versionCode 不可变/不可降 | 否则老用户无法覆盖安装 |

### 3.3 缓存与状态死锁（三次教训）

1. **抓包机器人不上线**（2026-10-20）：停用账号的残留 slot 指向已死子内核 → 身份判定失败 → `reconcile` 把可增派数归零。修法：先用 `/api/bot/status` 判在线，离线账号不毒化名单；加载时自愈清空停用账号 slot。
2. **注册永久转圈**：客户端无超时 + 服务端 `relay.fetch()` 无超时。修法：客户端 80s AbortController + 可操作提示；服务端仍待补超时。
3. **前端缓存死锁**：`?v=` 不变 → 浏览器永远旧 JS。修法：每次发布全局换版本戳；`youyuan_seedmap_v2.js:145` 的 CSS 版本单独同步。

共性：**「状态 A 导致永远回不到状态 B」比「功能没写」更难查**。排障时先找「有没有一个判断把自己锁死」，再找代码 bug。

### 3.4 box 与 Worker 双链路

见 `skills.md` §5.2。补充两条事实：

- box 的前端是编进二进制的，改前端不重编 = 手机/客户端看不到。
- Linux 手工启动若不设 `YY_DATA_DIR`，会落到 `~/.config/YouYuanToolbox`，表现为「数据全丢」——这是 2026-10 排查过的一次假故障。

### 3.5 组件皮肤 / 主题

- 主题令牌在 `css/youyuan_design.css`（`--phx-*`），深色由 `[data-theme="dark"]` 覆盖；`mcort_theme.css` 只是映射层。
- 历史阻断：深色设置卡白底浅字（1.2:1）、主按钮白字浅青底（1.72:1）、次要文字 3.63:1；已引入 `--phx-on-primary` 并调深 `--phx-text-secondary`。
- 原则：**不写死颜色**、**不造第二套按钮/开关/圆角**（UI 审查点名 20 种圆角、6 套按钮、4 种开关）。
- 游戏皮肤是另一件事：官方头像/皮肤库/中性占位，不伪造。

### 3.6 种子查询 v2 精度校准

| 时间 | 结论 |
|---|---|
| 07:18 前 | 群系 72.76%，7/48 tile 全错（整片繁茂洞穴） |
| 07:18 | depth 基准 `1 - 83/160` → `1.0 + off`，参考实现对拍转绿 |
| 07:21 | 新增 `yyV2SurfaceDepthNP`，地表查询钉到参数表地表点 |
| 07:32-07:34 | 采样偏移 +2（`yyV2RenderSampleOff`），99.703%（24503/24576），41.4ms/tile；新增生产路径闸门 `TestSeedV2RenderBiomeAccuracy`（门槛 99%） |
| 后续 | 要塞改 `ring_approx` + 基岩版 501；结构 13 类逐点 100% |

教训：**闸门必须测玩家实际走的那条路径**。之前 98.9% 的闸门直接调 `Axes()+ClimateToBiome`，绕过了 voronoi 与坐标换算，所以 96.26% 也被判绿。

### 3.7 命令链（雪球）事故

- 症状：原雪球主链高频运行时卡服、随后 UDP 拒绝、主断线。
- 处置：停主链、保留原文件与 Hz、不以压测复现代替修复；只改解释器/调度/发送回执/收包分发/队列。
- 现状：多次修复后仍有「多人峰值未验收」；原链停用状态是**用户明确要求**，不要擅自恢复。
- 语义提醒：`executed/submitted/cycles` 都不是服务端成功；`fast` 模式下 `results_confirmed=false`。

---

## 4. 已知矛盾与待裁决（不要自行补脑）

| 矛盾 | 现状 | 处理 |
|---|---|---|
| `PROJECT_STATE.md` vs 总账 | 文件写 120 项 / 0.10.75；总账 125 项 / 源码 0.10.78；HEAD 也不是文档里的 tag | 巡检报告判定「PROJECT_STATE 已不能作为当前状态源」；当前以 `需求逐项总清单.md` + `维护入口.md` 为准，是否重写由用户决定 |
| 发布脚本断档 | `../work` 最高只到 75 期脚本，76/77/78 期缺失 | 本文的部署命令是**按脚本原理重写**的通用链路，不依赖缺失的 7x 脚本 |
| 旧 SSH 桥 | 决策说已删/禁用，但 `维护入口.md:157` 仍在指示使用 | 以决策为准：不要调用 `ssh_https58.py` / `yy_deploy58_tmp` |
| 自动更新定时器 | 每 6h 拉更新，但没有 Linux 专用 manifest | 未确认前不要当它无害；先 `systemctl cat youyuan-update.timer` |
| 「小黄鱼」 | 历史文档 grep 为 0 | 需要用户说明指什么，禁止猜 |
| `cmd/server` 编译失败 | 结构性（`package main` 导入同包） | 预期失败，不是回归；不要为了「全绿」去删它 |
| `internal/playerrelay` UDP 偶发超时 | 连跑 通过/失败/通过 | 与本项目改动无调用关系；用户未裁决前不要动 |

---

## 5. 从对话里恢复上下文的正确姿势

1. 读 `BOOT.md`（10 秒拿到当前状态与结构性坑）。
2. 读 `协作交接/维护入口.md` **最上面 5 条**（最新状态，越靠前越新）。
3. 读 `需求逐项总清单.json` 里与你任务相关的编号（不要读全量 227 KB）。
4. 只在需要原始语气/报错原文时，才去 `outputs/_user_messages_readable.txt` 或 `.jsonl`。
5. 找到结论后，**把结论写回** `workspace/` 或 `outputs/`，不要只留在聊天里——下一次压缩就没了。

---

## 6. 写交接的三条硬要求

1. **带时间**：`YYYY-MM-DDTHH:MM:SS+08:00`，精确到分钟，否则无法与部署日志对齐。
2. **带版本与 sha**：网页 Version ID、原生二进制 sha 前 8 位、认证独立版本，三者分开写。
3. **带「未做」**：明确列出没验的项（真机、多人、跨维度、长跑）。只写「已完成」的交接等于埋雷。
