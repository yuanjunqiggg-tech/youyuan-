#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""导入不再死等区块加载：
- waitChunksLoaded 增加 attempts/soft 参数；
- 清除阶段：最多 8 轮探测，超时记警告继续（不 fail 整个任务）；
- 放置阶段：最多 3 轮探测（约 3s），超时记警告继续——区块在 bot TP 到位后 1-2s
  内会流式加载，放置本身有 import_speed 限速，给加载留了时间；
- soft 超时不中断导入，仅记录 chunkWarn 供进度页展示。
用法：python3 fix_import_nowait.py
"""
import io, sys

PATH = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/upstream/building/importer.go"
src = io.open(PATH, encoding="utf-8").read()
orig = src

def rep(old, new, count=1):
    global src
    n = src.count(old)
    if n != count:
        print("ANCHOR-FAIL(%d!=%d): %r" % (n, count, old[:80])); sys.exit(1)
    src = src.replace(old, new, count)

# 1) 签名：加 attempts / soft
rep(
    "func (t *ImportTask) waitChunksLoaded(x1, z1, x2, z2 int32) error {",
    "func (t *ImportTask) waitChunksLoaded(x1, z1, x2, z2 int32, attempts int, soft bool) error {\n"
    "\tif attempts < 1 {\n"
    "\t\tattempts = 1\n"
    "\t}",
)

# 2) 循环上限改为参数
rep(
    "\t// Up to 30 attempts, plus the callback's own bounded command timeout.\n"
    "\t// A confirmed column need not be re-probed while waiting for another one\n"
    "\t// in this same region. There is no cache across teleports or regions.\n"
    "\tfor attempt := 0; attempt < 30; attempt++ {",
    "\t// Limited attempts, plus the callback's own bounded command timeout.\n"
    "\t// A confirmed column need not be re-probed while waiting for another one\n"
    "\t// in this same region. There is no cache across teleports or regions.\n"
    "\tfor attempt := 0; attempt < attempts; attempt++ {",
)

# 3) 超时：soft 模式降级为警告继续，不中断导入
rep(
    '\treturn fmt.Errorf("等待区块加载超时，区域(%d,%d)-(%d,%d)未提交；保留断点后重试", cx1, cz1, cx2, cz2)\n'
    "}",
    '\tif soft {\n'
    '\t\t// 用户已明确：导入/导出不预先等待区块加载。探测超时后继续放置，\n'
    '\t\t// 区块会随 bot 驻留流式加载；放置限速本身给加载留出时间。\n'
    '\t\tt.ChunkWaitWarn = fmt.Sprintf("区块(%d,%d)-(%d,%d)探测超时，已继续放置", cx1, cz1, cx2, cz2)\n'
    '\t\treturn nil\n'
    '\t}\n'
    '\treturn fmt.Errorf("等待区块加载超时，区域(%d,%d)-(%d,%d)未提交；保留断点后重试", cx1, cz1, cx2, cz2)\n'
    "}",
)

# 4) 调用点 1：fillSegments 清除前（8 轮 + soft）
rep(
    "\tif err := t.waitChunksLoaded(x1, z1, x2, z2); err != nil {\n\t\tt.importErr = err\n\t\treturn 0\n\t}",
    "\tif err := t.waitChunksLoaded(x1, z1, x2, z2, 8, true); err != nil {\n\t\tt.importErr = err\n\t\treturn 0\n\t}",
)

# 5) 调用点 2：放置前（3 轮 + soft）
rep(
    "\t\t\tif err := t.waitChunksLoaded(int32(t.BaseX+blockStartX), int32(t.BaseZ+blockStartZ),\n"
    "\t\t\t\tint32(t.BaseX+blockEndX), int32(t.BaseZ+blockEndZ)); err != nil {\n"
    "\t\t\t\treturn err\n"
    "\t\t\t}",
    "\t\t\tif err := t.waitChunksLoaded(int32(t.BaseX+blockStartX), int32(t.BaseZ+blockStartZ),\n"
    "\t\t\t\tint32(t.BaseX+blockEndX), int32(t.BaseZ+blockEndZ), 3, true); err != nil {\n"
    "\t\t\t\treturn err\n"
    "\t\t\t}",
)

# 6) ImportTask 新增 ChunkWaitWarn 字段（超时继续放置的警告展示位）
rep(
    "\t// importErr 记录 StopOnError 模式下的第一个错误\n\timportErr error",
    "\t// importErr 记录 StopOnError 模式下的第一个错误\n\timportErr error\n\n"
    "\t// ChunkWaitWarn 记录最近一次区块探测超时但仍继续放置的警告（soft 模式）\n"
    "\tChunkWaitWarn string",
)

assert src != orig
io.open(PATH, "w", encoding="utf-8", newline="\n").write(src)
print("importer.go patched OK")
