# 二进制加固（MCORT）

范围：两条产物 —— Linux ELF（`build/youyuan-linux-x86_64`，部署在 box 上）与 Android APK。
原则：**只加防护层与构建参数，不改业务逻辑**；所有命令都在本机真实跑过。
配套脚本：`build/_harden/build_linux_hardened.ps1`（用法见 5.1 节）。

先说结论，避免读完全文还在等一个"全都做了"的答案：

- **已经真正生效的**只有 15 项（下面 1 节），全部是"构建参数 + 自校验 + 签名白名单"这一档；
- **能靠开源/自研再抬高的**约 10 项，代价是重编、真机冒烟或一点运行开销；
- **剩下 40 多项（VMP、DEX 加壳、指令抽取、Java2C、反内存 Dump）必须买商业壳**。谁说自己能自研，等于说自己能重写一个加固厂。
- 最关键的一句：**本地混淆只抬高"strings + grep"这条最低成本路径**。真正让"拿到也拿不到"的只有 [通用 1] 服务端收权。

---

## 1. 当前已生效的防护（逐条带证据）

| # | 防护项 | 落地位置 | 说明 |
|---|---|---|---|
| 1 | 符号与调试信息剥离 | `build.ps1:42` `-s -w` | `.symtab`/DWARF 全无 |
| 2 | 路径裁剪 | `build.ps1:65/78/61` `-trimpath` | 二进制里不出现 `C:\Users\...` 本机路径 |
| 3 | 去 build id | `build.ps1:42` `-buildid=` | 两份产物无法靠内部指纹互相对齐 |
| 4 | 构建指纹注入 | `-X main.buildStamp=<时间戳-随机>` | 线上崩溃/安全日志能对回构建批次 |
| 5 | 内嵌前端摘要钉死 | `build.ps1:17-40` ↔ `integrity.go:169` | 前端整棵树 `go:embed` + 目录级 sha256，改一个 JS 重签即对不上 |
| 6 | 前端摘要强制注入 | `build.ps1:42` + `integrity.go:229` | 有 stamp 无 hash 直接判失败，堵住"漏传参数静默通过" |
| 7 | APK 签名证书白名单 | `integrity.go:16-20`（3 张历史证书） | 重签名必然换证书 → 不在白名单 |
| 8 | 包名校验 | `integrity.go:25`、`:130` | 必须 `org.youyuan.toolbox` |
| 9 | 权威签名读取 | `apk_signing.go:12` 解析 APK Sig Block | Go 直接读 APK 文件的 v2 块证书，**不信 Java 上报**，改 Java 伪造绕不过 |
| 10 | 启动即校验 + HTTP 闸门 | `integrity_gate.go:191` ← `main.go:4991`；闸门 `main.go:5257` | 监听端口前同步跑；判定篡改则 `/api/*` 403 JSON、静态资源也回拦截页 |
| 11 | 自身封印（落盘完整性） | `protect_selfcheck.go:52/90` + `maintenance/seal_binary.py` | 槽位清零后整文件 sha256 回填；**构建后写回，之后改动任何字节都 mismatch** |
| 12 | TracerPid 调试检测 | `protect_selfcheck.go:176` | `/proc/self/status` 非 0 即告警；**只记日志不退出** |
| 13 | 敏感常量收敛（10 段） | `upstream/protect_secrets.go` | 域名/更新地址/两段系统提示词改 XOR+hex 存储，运行时还原 |
| 14 | R8 + 资源压缩 | `android/app/build.gradle:14-16` + `app/proguard-rules.pro` | `minifyEnabled true` / `shrinkResources true`；自身类 `-keepnames`，第三方库整体改名 |
| 15 | 加固后重签名 + 验证 | `build.ps1:96-101` | `zipalign -p 4` → `apksigner sign` → `verify --print-certs` |

补充事实（避免误判）：

- Linux 上完整性判定是 **unknown → 放行**（`integrity_gate.go:230`）。Linux 没有 APK，这是刻意设计，不是漏做。
- `apk_signing.go` 只认 **v2 块（ID `0x7109871a`）**。只用 v1 或只出 v3 的签名包会解析失败 → 判篡改。当前 `apksigner` 默认 v1+v2+v3，安全。
- `usesCleartextTraffic="true"`（`AndroidManifest.xml:24`）与 `extractNativeLibs="true"`（`:26`）是**反向配置**，见 6.5 / 6.6。

---

## 2. Android APK 54 项对照表

状态口径：**已做** = 代码/脚本里真实生效；**部分** = 做了一半或只覆盖一部分面；**可做** = 本仓库能自己做，给出命令/位置；**需商业** = 自研不现实，列厂商；**不适用** = 本工程形态用不上。

| # | 项目 | 状态 | 依据 / 命令 / 说明 |
|---|---|---|---|
| 1 | R8/ProGuard 混淆 | 已做 | `app/build.gradle:14`。实测 APK −1.05 MB；`org.youyuan.toolbox` 真实类改名 0 个（仅第三方库与合成类） |
| 2 | 资源压缩 | 已做 | `app/build.gradle:15` `shrinkResources true`；全工程仅 2 处 `getIdentifier`，均查 `"android"`，安全 |
| 3 | 控制流平坦化 | 需商业 | DEX 层无开源等价物；易盾/乐固/KiwiVM |
| 4 | 字符串 XOR 加密 | 部分 | Go 侧 10 段已做（`protect_secrets.go`）；**DEX 字符串未做**，需自研 gradle 插件，不建议 |
| 5 | 类名混淆 | 部分 | 第三方库已改名；自身类因三处名字硬约束保名（`proguard-rules.pro:22` 注释）。放开需按 6.1 分步 |
| 6 | DEX 整体加壳 | 需商业 | 易盾/乐固/360/阿里 |
| 7 | DEX 方法指令抽取加密 | 需商业 | 同上 |
| 8 | DEX 动态加载 | 需商业 | 自研 = 写壳（`InMemoryDexClassLoader`，minSdk 26 可用，但配套解密/修复成本等同买壳） |
| 9 | DEX 防篡改 | 部分 | 有签名级防篡改（#27/#28）；无 DEX 段级 checksum 修复 |
| 10 | Java2C | 需商业 | 乐固/易盾 |
| 11 | Dex2C | 需商业 | 见 #54 |
| 12 | SO 段加密 | 需商业 | Virbox/乐固；自研 `.so` 解密 stub 会与 Go runtime 段布局打架 |
| 13 | SO 符号混淆 | 已做 | `-s -w` 剥 `.symtab`；再叠 garble（3.1）连 Go 函数名一起变 |
| 14 | SO 代码虚拟化 | 需商业 | Virbox Protector / KiwiVM |
| 15 | SO 导入/导出符号保护 | 部分 | `.symtab` 已剥；**`JNI_OnLoad`/`Java_*` 导出符号必须保留**，否则 JVM 找不到，物理上藏不掉 |
| 16 | VMP 虚拟机保护 | 需商业 | Virbox / 易盾 |
| 17 | 自定义指令集转换 | 需商业 | VMP 附带能力 |
| 18 | VMP 2.0 方法抽取 | 需商业 | 同上 |
| 19 | 防调试器（GDB/IDA/lldb） | 部分 | Linux 侧有 TracerPid（#25）；APK 侧无。可加：`android.os.Debug.isDebuggerConnected()` + 读 `/proc/self/status`，位置 `MainActivity.java`（见 6.2） |
| 20 | 防 Frida/Xposed/Substrate | 可做（有限） | 27042 端口、`/proc/self/maps` 含 `frida`、`de.robv.android.xposed` 类探测（6.3）。**易绕过**，强对抗需商业 |
| 21 | 防内存 Dump | 需商业 | Go runtime 堆里全是明文（提示词、token），自研无解 |
| 22 | 防内存篡改 | 部分 | 封印只覆盖**落盘文件**，不覆盖内存；内存校验需商业壳 |
| 23 | 防注入 | 需商业 | 易盾/乐固 |
| 24 | 防 Hook | 需商业 | 静态 Go 无 PLT/GOT，常规 inline-hook 检测无处落脚 |
| 25 | TracerPid 检测 | 已做 | `protect_selfcheck.go:176`，只告警不自杀（systemd 环境误报代价太高） |
| 26 | 多进程互监控 | 可做 | 已有 8 个 `:accountN` 进程 + `KeepService`（`AndroidManifest.xml:40-56`）；互 ping 心跳即可实现。代价：耗电与误杀，建议只上报 |
| 27 | 防二次打包 | 已做 | 签名证书白名单（`integrity.go:16-20`）+ 包名 + 直接读 APK |
| 28 | 签名校验 | 已做 | Java 上报兜底（`MainActivity.java:73/618`）+ Go 权威解析（`apk_signing.go:18`） |
| 29 | 文件完整性校验 | 已做 | 前端树摘要 + 自身封印槽 + APK 证书，三条互补 |
| 30 | ROOT 检测 | 可做 | `su` 路径 / `ro.build.tags=test-keys` / Magisk 特征；**误报率高、绕过也容易**，建议只回传遥测，不硬拦 |
| 31 | 模拟器检测 | 可做 | `ro.hardware=goldfish/ranchu`、`ro.kernel.qemu=1`；同上，只遥测 |
| 32 | 多开器检测 | 可做 | 同包名多进程枚举 + 文件锁；本工程本来就多进程，**必须按 uid/pid 白名单**，否则自杀 |
| 33 | 加速器检测 | 不适用 | 本产品是 bot 工具箱，加速器不构成威胁模型；硬拦只会误伤 |
| 34 | 资源文件混淆 | 可做 | AndResGuard（开源）：`classpath 'com.tencent.mm:AndResGuard-gradle-plugin:1.3.7'`；AGP 8 兼容性差，需实测。当前只有 AAPT2 默认压缩 |
| 35 | Assets 加密 | 部分 | **前端资源已 `go:embed` 进 `libyouyuan.so`**，比 assets 加密更强（assets 里已经没有前端）；仅 Python 资源未加密，改 Chaquopy 不划算 |
| 36 | XML 配置保护 | 部分 | 清单本身被 AAPT2 编译为二进制 XML；防导出需商业 |
| 37 | 数据库加密 | 不适用 | 本工程无 SQLite，配置是 JSON + 文件权限 |
| 38 | 防截屏 | 可做 | `getWindow().setFlags(FLAG_SECURE, FLAG_SECURE)`（`MainActivity.java` 的 `onCreate`）。**副作用：用户也不能截图分享**，建议按页面开 |
| 39 | 防录屏 | 可做 | 同 #38，`FLAG_SECURE` 一并生效 |
| 40 | 防抓包 | 可做 | Manifest `usesCleartextTraffic` 改 `false`（`:24`）+ `network_security_config.xml` 只放行自家域；okhttp 4.12 已在依赖里，可加 `CertificatePinner`（6.6） |
| 41 | APKARMOR 结构异常注入 | 需商业 | 结构层对抗，只有壳厂做 |
| 42 | 伪造压缩方法 | 需商业 | 同上（破坏 apktool 解包） |
| 43 | 修改偏移字段 | 需商业 | 同上 |
| 44 | 脏字节码替换 | 需商业 | 同上 |
| 45 | 网易易盾 | 需商业 | 1–5 万元/年；**必须用"加固签名"或把自有 keystore 交给对方**，否则 `integrity.go` 白名单要重算 |
| 46 | 几维安全 KiwiVM | 需商业 | 主打 SO 虚拟化 |
| 47 | 360 加固保 | 需商业 | 免费档有，功能弱于付费 |
| 48 | 腾讯云乐固 | 需商业 | 与腾讯系工具链顺手 |
| 49 | 阿里云安全加固 | 需商业 | 同上 |
| 50 | XopProtector | 需商业 | 同上（厂商能力需自行询价） |
| 51 | Appxlrth | 需商业 | 名称疑为商业壳产品，采购前须核实其是否仍维护（见 7.1） |
| 52 | Dex Protect | 需商业 | 能力等同于 #6/#7，多为壳厂子功能 |
| 53 | So Protect | 需商业 | 等同于 #12/#14 |
| 54 | Dex2c | 需商业 | 等同 #11 |

**统计（54 项）**：已做 7 / 部分 8 / 可做 9 / 需商业 28 / 不适用 2。一半以上集中在"必须买壳"的 45–54 与 DEX 加工区。

---

## 3. Linux ELF 30 项对照表

前提必须先讲清楚：本产物是 **`CGO_ENABLED=0` 的纯 Go 静态可执行**（`ET_EXEC`，无 `.dynsym`、无 PLT/GOT、无 C 栈帧）。
清单里一半的"编译器加固项"作用对象是 **C 代码或动态链接结构** —— 在本产物上加了是空操作，不是偷懒。

| # | 项目 | 状态 | 依据 / 命令 / 说明 |
|---|---|---|---|
| 1 | Strip 符号剥离 | 已做 | `-s -w`；节头数量见脚本自检输出 |
| 2 | `-fPIE -pie`（ASLR） | 可做 | `build/_harden/build_linux_hardened.ps1 -Pie`（已实测出 `ET_DYN`）。默认关：会硬依赖 `/lib64/ld-linux-x86-64.so.2`，musl/极小镜像起不来 |
| 3 | `-fstack-protector-strong` | 不适用 | 纯 Go 无 C 栈帧。真要：`CGO_ENABLED=1 go build -buildmode=pie -ldflags "-linkmode=external -extldflags=-fstack-protector-strong"`（代价：动态依赖 + 体积） |
| 4 | `-Wl,-z,relro,-z,now` | 不适用 | 无动态链接即无 GOT/PLT 可保护。同上需 `CGO_ENABLED=1 -linkmode=external` |
| 5 | `-D_FORTIFY_SOURCE=2` | 不适用 | 只影响 libc 调用；本产物静态无 libc |
| 6 | NX | 已做 | `PT_GNU_STACK = RW`（无 X），栈不可执行 |
| 7 | `-fcf-protection`（CET） | 不适用 | Go 编译器不产 `endbr64`；需 C 层 |
| 8 | 控制流平坦化（FLA） | 需商业 | Virbox Protector；Go 无 OLLVM 通道 |
| 9 | 虚假控制流（BCF） | 需商业 | 同上 |
| 10 | 不透明谓词 | 需商业 | 同上 |
| 11 | 指令替换（SUB） | 需商业 | 同上 |
| 12 | 指令重排 | 需商业 | 同上（Go 侧只有编译器自己的调度，不构成对抗） |
| 13 | 等价替换 | 需商业 | 同上 |
| 14 | `.dynsym/.dynstr` 伪造 | 不适用 | 静态产物根本没有这两个节 |
| 15 | GOT/PLT 导入表混淆 | 不适用 | 同上 |
| 16 | 动态符号表破坏 | 不适用 | 同上 |
| 17 | ELF 加密壳 | 需商业 | Virbox；自研 stub 需接管 `_start` + 自解密 + `memfd`，与 Go runtime 冲突面极大 |
| 18 | `.text` 节区加密 | 需商业 | 同上 |
| 19 | 内核级加载解密 | 需商业/不建议 | 需要内核模块或 ptrace 注入，运维与合规风险过高 |
| 20 | 多态/变形壳 | 需商业 | 每次构建变形的壳 |
| 21 | `ptrace(PTRACE_TRACEME)` 反调试 | 可做 | 需 cgo：`prctl(PR_SET_DUMPABLE,0)` / `ptrace(PTRACE_TRACEME)`。**不建议**：会与 systemd 看护、`strace` 运维冲突，且可被 `LD_PRELOAD` 绕过；现有 TracerPid 只读检测更稳 |
| 22 | `/proc/self/status` TracerPid | 已做 | `protect_selfcheck.go:176` |
| 23 | Timing Check（rdtsc 差值） | 可做/不建议 | Go GC 抖动会导致误报，误报=用户机上假告警 |
| 24 | SIGTRAP 自检测 | 可做/不建议 | 需自定义信号处理，与 Go runtime 的信号链冲突 |
| 25 | 内存保护（关键段 RX） | 部分 | Go runtime 已把 text 段设为 `r-x`；额外 `mprotect` 需 cgo，收益低 |
| 26 | 完整性自校验 | 已做 | 封印槽 + `-buildid=` + 前端摘要，三层 |
| 27 | hARMless | 需商业/不适用 | ARM 指令级混淆，本产物 x86_64 |
| 28 | brotliPack | 不做 | 压缩壳会改变文件字节 → **封印必然失效**，且极易被云厂商/杀软判为木马 |
| 29 | bincrypter | 不做 | 同上；另外 `/proc/self/exe` 读到的是解压镜像，自校验读盘逻辑会错乱 |
| 30 | 深盾 Virbox Protector | 需商业 | ELF 加壳 + VMP；Go 静态二进制 + goroutine 栈扫描需实测，CPU 5–15% 开销 |

**统计（30 项）**：已做 4 / 部分 1 / 可做 1 / 需商业 12 / 不适用 7 / 明确不做 2 / 不建议 3。
7 项"不适用"（#3/4/5/7/14/15/16）全部是因为**纯 Go 静态可执行没有 C 栈帧、没有动态符号表**；把它们当成"漏做"是误判格式。

---

## 4. 通用/跨平台 4 项

| # | 项目 | 状态 | 落地 / 下一步 |
|---|---|---|---|
| 1 | 服务端收权（核心逻辑上云） | 部分 | 域名、更新地址、两段系统提示词已收敛到 `protect_secrets.go`（XOR+hex）。**真正的下一步**：把默认提示词、内置指令表（31 条危险前缀）、MCP 工具白名单搬到 Worker，本地只留密文缓存 + 短期票据。这是唯一能实现"拿到二进制也拿不到设置"的路径 |
| 2 | 威胁遥测（异常环境回传） | 部分 | 现只有本地 `<dataDir>/security.log`（0600），无回传。可做：`integrity_gate.go:94` 的 `yyIntegrityReport()` 已去敏（不含 hash），加一次 HTTPS 上报即可，注意**只报 verdict/reason 枚举，不报 hash 与设备指纹** |
| 3 | 数字签名与哈希校验 | 已做 | APK 证书白名单 + 前端摘要 + ELF 封印 + `apksigner verify`。四条链都在 |
| 4 | 最小化发布信息 | 已做 | `-s -w -buildid= -trimpath` + garble + 不发布 `.pdb`/`.symtab`/map 文件 |

---

## 5. 两条产物的加固层级

### 5.1 Linux ELF 构建（新脚本，`build/_harden/build_linux_hardened.ps1`）

分层，从内到外：

```
源码
 └ garble（包名/函数名/局部符号随机化，seed 存 private/garble-seed.txt）
    └ go build -trimpath -ldflags "-s -w -buildid= -X main.buildStamp=… -X main.frontendHash=sha256:…"
       └ maintenance/seal_binary.py（回填自身 sha256 到 .rodata 槽位）
          └ sha256 记录到 build/_harden/SHA256SUMS.txt
```

用法：

```powershell
# 默认：只落到 build/_harden/youyuan-linux-x86_64.hardened
powershell -ExecutionPolicy Bypass -File build/_harden/build_linux_hardened.ps1

# PIE + 直接替换正式产物（旧文件自动备份为 build/youyuan-linux-x86_64.rollback）
powershell -ExecutionPolicy Bypass -File build/_harden/build_linux_hardened.ps1 -Pie -Install

# 只对已有产物跑校验（不重编）
powershell -ExecutionPolicy Bypass -File build/_harden/build_linux_hardened.ps1 -SkipBuild
```

脚本做了什么（每步都会打印，失败即 `throw`，不产出半成品）：

1. 没有 garble 就 `go install mvdan.cc/garble@latest`（**不编造工具名**，garble 是 `mvdan.cc/garble`）；
2. seed 从 `private/garble-seed.txt` 读，没有就生成 8 字节随机数的 **base64**（`garble -seed` 只认 base64，写普通随机串会被拒：`error decoding seed: illegal base64 data`）—— **seed 泄露等于混淆白做**（拿源码 + seed 能复现同一套映射），务必随 `.rollback` 一起离线备份；
3. 前端摘要用与 `build.ps1:17` **逐字节相同**的算法算（改算法 = 运行时校验必挂）；
4. `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` + `-trimpath` + `-s -w -buildid=`；
5. **注入回读**：把产物当字节流搜 `buildStamp` 与 `frontendHash`，搜不到直接失败 —— 防"garble 悄悄吞掉 `-X`，线上启动被自己拦死"；
6. 封印 + `--verify` 复核；
7. ELF 自检：本机没有 `readelf` 时用 python 解析 `e_type`/`e_machine`/程序头，输出 `GNU_STACK`/`GNU_RELRO`/`INTERP`/节头数量；
8. `SHA256SUMS.txt` 追加一行（含 stamp，便于把产物对回构建）；
9. `-Install` 时先备份 `.rollback` 再覆盖。

`-tags` 说明：**本工程没有自定义业务 tag**，只有 `js`（wasm 前端）、`android`（c-shared）、`windows/linux`（内置约束）这类。正常 Linux 构建**不需要**传 `-tags`；误传 `android` 会把 JNI 分支编进来导致链接失败（脚本已拦截）。

已知取舍（照抄脚本注释，别当 bug）：

- **不用 `-literals`**：garble 的 literal 加密会把 `.rodata` 里 `YYSEALSC1`+64×`0`+`YYSEALEND` 变成密文，`seal_binary.py` 找不到槽位 → 封印失败（已实测）。字符串保护交给 `protect_secrets.go` 的 XOR+hex。**代价**：字符串字面量仍是明文（`/api/youyuan/` 901 次一次没少，见 5.3），别指望 garble 藏住接口。
- **封印之后不能再 strip/upx**：任何一个字节的改动都会让自检 `mismatch`。这是设计，不是缺陷。
- garble 对 Go 版本敏感：本机 `garble v0.18.0` + `go 1.27.0` 实测可用。**升级 Go 后必须重跑一次本脚本**，否则可能编不过或混淆不彻底。
- **Windows 侧不要发 garble 版**：本机 Defender 会把 garble 产出的 exe 判为可疑并隔离（已实测，见 8 节）。Linux ELF 不受 Defender 影响，但云厂商镜像扫描同样要看一眼。

### 5.2 APK 构建（沿用 `build.ps1`，不新增步骤）

```
go build -buildmode=c-shared -trimpath -ldflags "-s -w -checklinkname=0 -buildid= -X …"（arm64-v8a / x86_64）
 └ gradle assembleRelease（R8 minify + shrinkResources + proguard-rules.pro）
    └ zipalign -p 4 → apksigner sign（自有 keystore）→ apksigner verify --print-certs
```

加固层级现状：**native 层（Go .so）> DEX 层**。APK 95% 体积是 `.so`，R8 管不到 `.so`；
所以"混淆 DEX"对逆向成本的影响远小于把 `.so` 里的明文常量收走（已做）与上商业壳（未做）。

### 5.3 实测对比（2026-10-26，同一份源码，口径见 `build/garble-check.py`）

| 指标 | 常规产物 `build/youyuan-linux-x86_64` | 加固产物 `build/_harden/youyuan-linux-x86_64.hardened` |
|---|---|---|
| 体积 | 58,241,184 B（55.5 MB） | 63,787,132 B（60.8 MB，+9.5%） |
| Go 函数名可见度 | `0001_1.9.0_to_1.10.0.jsoncrypto` 这类**业务语义名**直接可读 | 只剩 `0.A0nIwYaa_u` 这类随机名，调用图失去语义 |
| `/api/youyuan/` 明文 | 901 | **901（未变）** |
| `Authorization` / `Bearer ` | 21 / 17 | 21 / 17（未变） |
| `seedv2` | 14 | 4 |
| `YYSEALSC1` 封印记录 | 1 | 1（garble 没有复制/破坏槽位） |
| 节表 | 16 节，含 `.note.go.buildid` / `.note.gnu.build-id` | 14 节，两个 `.note.*` 消失；两者均**无 `.symtab` / `.debug_*`**（strip 生效） |
| ELF 属性 | ET_EXEC / 静态 / GNU_STACK=RW | 同（`-Pie` 时 ET_DYN） |

**必须读懂的结论**：garble 默认模式**只改标识符，不加密字符串字面量**。
所以 `/api/youyuan/` 这类路由串出现次数一次没少——想靠它"让逆向者看不到接口"是行不通的。
`-literals` 能加密字符串，但会把封印槽 `YYSEALSC1` 一起变成密文导致封印失败（已实测），
所以本项目的字符串收敛只能靠 `protect_secrets.go` 那种"手工 XOR + 运行时还原"，
以及最终的服务端收权。

换句话说：**garble 买到的是"读不懂结构"，不是"看不到内容"**。

本机实跑记录（脚本原样输出摘要，可复现）：

```
[harden] garble  mvdan.cc/garble v0.18.0
[harden] 新建 seed（base64）→ private/garble-seed.txt
[harden] 前端资源摘要 sha256:8b8785b1f9c79a4369995d93e62bf4f9e3962dd4738a77c9ef4fcf0a54ee3255
[harden] 编译完成，耗时 109 秒
[harden] 注入回读：buildStamp 与 frontendHash 均在产物中找到
已封印：槽位=ebf2c5235618b3ab8d66dd5fd69847d9be66fa7c15ee3b09f61c543754bda5d5
[harden] ELF 自检：ET_EXEC / 静态 / GNU_STACK=RW / NX=yes / section headers=14 / .symtab/.debug=stripped
[harden] sha256 20880c8c9c902351d71dad843f7e86262e2c94449d42bd08f1c8db981d618e8c
```

注意封印槽值（`ebf2c5…`）与文件 sha256（`20880c…`）**不相等是正常的**：槽位里存的是
"把槽位当成 64 个 `0` 之后整个文件的 sha256"，而不是最终文件的 sha256。这也是为什么
不能简单用 `sha256sum` 去反推封印——两个值都要记。

---

## 6. APK 侧可执行建议

### 6.1 R8/ProGuard 规则位置与放开类名混淆的分步方案

位置：`upstream/android/app/proguard-rules.pro`，构建开关在 `upstream/android/app/build.gradle:9-17`（已开 `minifyEnabled`/`shrinkResources`）。

当前刻意保名的三处硬约束（改之前必须先看这三条，否则"混淆成功、功能全挂"）：

1. Go 用 `FindClass("org/youyuan/toolbox/FloatingBridge")` 按字符串找类（`libmain.go:31`）；
2. JNI 导出符号 `Java_org_youyuan_toolbox_MainActivity_setDataDir` 等由「声明的类名+方法名」拼出（`libmain.go:104-132`、`:211`）；
3. WebView `addJavascriptInterface` 暴露的对象由 JS 按方法名调用（`MainActivity.java`）。

分步放开（每步都要真机冒烟：启动 → 悬浮窗 → JS 桥 → Python 插件 → 更新）：

1. 把 `-keepnames class org.youyuan.toolbox.**` 收窄成显式白名单：
   `MainActivity`、`FloatingBridge`、`JniFloatingChannel`、`AccountProcessService`（含 `$Slot1..$Slot8`）、`VerificationActivity`、`KeepService`、`BootReceiver`、`NotificationActionReceiver`、`FloatingWindowService`、`PrismDataProvider`、`FloatingChannel`/`WsFloatingChannel`/`FloatingChannelManager`；
2. 其余自家类允许改名，观察 `ClassNotFoundException` / `NoSuchMethodError`（`adb logcat | grep -E "ClassNotFound|NoSuchMethod"`）；
3. 全绿后再考虑成员名混淆（`-keepclassmembers` 已覆盖 `native` 方法与 `@JavascriptInterface`）。

### 6.2 防调试（可做，成本最低的一项）

- Java 侧：`MainActivity.onCreate` 里 `if (android.os.Debug.isDebuggerConnected()) { /* 上报，不退出 */ }`
- Native 侧已经有一半：`protect_selfcheck.go:176` 读 `/proc/self/status`，但**只在启动时读一次**。若要持续检测，需在 Go 里加 ticker（属业务改动，本次不做）。
- 原则：**只上报不自杀**。本产品跑在用户机器/手机上，误判一次就是整台不可用；判断"该不该拒服务"必须交给服务端。

### 6.3 防 Frida / Xposed（可做有限，别指望挡住）

可落地的信号（都只上报、不硬拦）：

- `/proc/self/maps` 含 `frida`、`gadget`、`linjector`；
- 默认端口 27042 有监听；
- `de.robv.android.xposed.XposedBridge` 类可加载；
- `ptrace` 被附加（同 6.2）。

真话：这些特征在 frida-server 改名、`magisk-frida`、非标准端口下全部失效。要真防，只有商业壳的 native 反调试 + 代码虚拟化。

### 6.4 签名校验已实现的部分（不要重复造）

- 白名单：`integrity.go:16-20` 三张历史证书 SHA-256，加证书就加一行；
- 权威路径：`integrity.go:110` → `apk_signing.go:18` 直接解析 APK Sig Block v2 证书（**不信 Java**）；
- 兜底路径：`apkSigHash`/`apkPkgName`（Java 上报，`MainActivity.java:73/618`）；
- 拦截：`integrity_gate.go:117` 包住整个 mux（`main.go:5257`），`/api/*` → 403 JSON，其余 → 内联拦截页（不依赖外部文件，删资源绕不过）；
- 逃生门：`YY_INTEGRITY_ENFORCE=0`（仅本机）。

**注意**：如果上商业加固壳，壳通常要重签（"加固签名"）。要么用壳厂提供的"自有签名"服务，要么上架前把新证书 SHA-256 加进 `officialSigHashes`，否则线上全量 403。

### 6.5 资源与清单

- `shrinkResources` 已开，`res/` 名混淆可上 AndResGuard（#34），但 AGP 8 兼容性需实测；
- 建议改 `AndroidManifest.xml`：
  - `android:allowBackup="false"`（当前未声明 = 默认 `true`，`adb backup` 可拖走数据）；
  - `android:usesCleartextTraffic="false"`（当前 `true`，`:24`）；
  - 需要 HTTP 白名单时加 `android:networkSecurityConfig="@xml/network_security_config"`；
  - `extractNativeLibs="true"`（`:26`）→ 改 `false` + `useLegacyPackaging false` 可让 `.so` 直接从 APK 映射、减少落盘拷贝；**但 Chaquopy/Python 与部分 ROM 有兼容风险，必须真机验证**。

### 6.6 防抓包

- okhttp 4.12 已在依赖里，加 `CertificatePinner` 钉自家域证书（`MainActivity`/`CommandSender` 用到的 OkHttpClient 处）；
- 注意钉证书会在证书轮换时全量失效，必须留"服务端下发新 pin"的窗口；
- 真正的防护是**把敏感字段从接口里删掉**（例如账号标识/令牌不该回给前端，见 `youyuan_identity.go` 的 `control_token` 问题）。

### 6.7 必须上商业壳的（不要在自研上浪费时间）

`#6 DEX 加壳`、`#7 指令抽取`、`#10 Java2C`、`#11/#54 Dex2C`、`#14 SO 虚拟化`、`#16-18 VMP`、`#21 防内存 Dump`、`#23 防注入`、`#24 防 Hook`、`#41-44 结构对抗`、`#45-53 各厂商产品`；
ELF 侧对应 `#8-13 混淆类`、`#17-20 加密壳`、`#30 Virbox`。

---

## 7. 验证方法

### 7.1 Linux ELF

```bash
# 1) 哈希与体积
sha256sum build/youyuan-linux-x86_64.hardened        # 与 build/_harden/SHA256SUMS.txt 对齐

# 2) ELF 属性（目标机上）
readelf -h build/youyuan-linux-x86_64.hardened | grep -E 'Type|Machine'
readelf -l build/youyuan-linux-x86_64.hardened | grep -E 'GNU_STACK|GNU_RELRO|INTERP'
#   期望：Type=EXEC（或 -Pie 时 DYN）、GNU_STACK=RW（无 E）、无 INTERP（静态）

# 3) 封印状态（在目标机上跑，不要在生产 systemd 单元里设这个变量）
YY_SELFTEST=1 ./build/youyuan-linux-x86_64.hardened
#   期望：完整性 : sealed（封签一致）；TracerPid 非 0 时只多一行告警

# 4) 篡改即失效（在副本上动手，别毁掉原件）
cp build/youyuan-linux-x86_64.hardened /tmp/yy_tamper
printf '\x00' | dd of=/tmp/yy_tamper bs=1 seek=1048576 count=1 conv=notrunc
YY_SELFTEST=1 /tmp/yy_tamper                            # 期望：mismatch

# 5) 明文残留对比（Windows 本机即可，纯标准库）
python build/garble-check.py build/youyuan-linux-x86_64 build/_harden/youyuan-linux-x86_64.hardened

# 6) 完整性接口（服务启动后）
curl -s http://127.0.0.1:<port>/api/youyuan/integrity    # linux 上 verdict=unknown 属正常
```

### 7.2 APK

```bash
D:/android-build/sdk/build-tools/34.0.0/apksigner.bat verify --verbose --print-certs build/youyuan-v0.10.1.apk
#   证书 SHA-256 必须在 integrity.go:16-20 白名单里

unzip -l build/youyuan-v0.10.1.apk | sort -k1 -n -r | head -15   # 确认 .so 占大头、无 assets/frontend
unzip -o -q build/youyuan-v0.10.1.apk classes.dex -d /tmp/apkcheck
D:/android-build/sdk/build-tools/34.0.0/dexdump.exe -f /tmp/apkcheck/classes.dex | grep -c "org/youyuan/toolbox"
#   期望：关键类还在、第三方库类名已变成 a/b/c 这类短名；想细看直接 jadx-gui 打开 APK
```

真机冒烟（R8 之后必做，缺一项都不算过）：冷启动 → 悬浮窗 → JS 桥（`window.android.*`）→ Python 插件加载 → 检查更新 → 多账号槽位启动 → 断网重连。

---

## 8. 风险与回退

| 风险 | 触发条件 | 处置 |
|---|---|---|
| 产物启动后全量 403 | `frontendHash` 注入失败但 `buildStamp` 成功 | 脚本已在构建后回读字节流，搜不到 hash 直接 `throw`；已上线的临时救：`YY_INTEGRITY_ENFORCE=0` |
| garble 把 JSON 键名改了 | 结构体字段无 json tag 且被序列化 | **上线前必须做接口回归**。可行做法：在 WSL/容器里跑加固产物，`YY_SELFTEST=1` 过自检后启动服务，`curl` 关键接口比对键名。**注意**：用 garble 编 Windows exe 在 Windows 上做冒烟这条路走不通——本机 Defender 直接拦下（见下表杀软误报） |
| 混淆后疑似逻辑异常 | 极少数依赖 `reflect` 字段名的路径 | 处置同上（在 WSL/容器里回归）；确认是混淆导致就退回不混淆构建，别在生产机上赌 |
| PIE 起不来 | 目标机 musl / 极小镜像 | 不加 `-Pie`；PIE 不是安全必需品，静态 `ET_EXEC` 靠封印与混淆兜底 |
| 封印后运维做了 `strip`/`upx` | 手工加工 | 重新构建；**不要**在生产机上 patch 二进制 |
| 自更新把产物换掉 | 更新流程未走封印 | 已在 `protect_selfcheck.go` 注释写明：启用"篡改即拒服务"前，必须让更新流程也走封印，否则一次自更新自锁 |
| seed 丢失 | 只备份了产物 | 复现不出同一份字节，但功能不受影响；`private/garble-seed.txt` 与 `.rollback` 一起离线备份 |
| 商业壳重签 | 上架前加固 | 新证书 SHA-256 加进 `integrity.go` 白名单，否则全量 403 |
| **杀软误报（已实测）** | garble 产物的随机标识符 + 高熵特征会被判可疑 | 2026-10-26 本机构建 Windows 冒烟版时，Defender 直接报 `Operation did not complete successfully because the file contains a virus or potentially unwanted software`，产物被隔离（`build/_harden/*.exe` 不存在）。**结论**：加固产物上线前必须过一遍目标环境的杀软/EDR，必要时提前加白；Windows 侧不建议发 garble 版，Windows 冒烟改在 WSL/容器里做 |

**回退**（Linux）：

```bash
# 脚本 -Install 时自动生成
cp build/youyuan-linux-x86_64.rollback build/youyuan-linux-x86_64
systemctl restart <youyuan 服务名>
```

**回退**（APK）：保留上一版已签名 APK（`build/youyuan-v0.10.<n-1>.apk`），同证书可覆盖安装：
`adb install -r -d build/youyuan-v0.10.<n-1>.apk`。

---

## 9. 不要做的三件事

1. **不要上 UPX / bincrypter / brotliPack**：与封印互斥，且极易被云厂商与杀软判为木马，部署即事故。
2. **不要自研 VMP / DEX 加壳 / 反内存 Dump**：这是加固厂十年积累的工程量，自研版本一定比商业壳更慢、更容易崩、更容易被脱。
3. **不要把"本地混淆"当"安全边界"**：密钥与提示词都在同一个二进制或同一台机器上，铁了心逆向的人一定能还原。让它真正"拿到也拿不到"的只有 [通用 1]。
