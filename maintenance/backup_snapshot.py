#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""backup_snapshot.py —— 游园专用备份库（部署前必跑）

设计目标：
  1. **部署前跑一次**，出问题能回滚
  2. 只备份**有意义的文本资产**（源码/文档/配置/Skill/记忆/账本），
     跳过构建产物（build 6.7G、cloudflare 5.6G、.skillcheck 1.9G）与二进制
  3. 每个快照带 `manifest.json`（相对路径 + 字节 + sha256），可逐文件校验
  4. `LOG.md` 累积记录「改了什么 / 怎么实现 / 特点」
  5. 只保留最近 KEEP 个快照，自动清理最旧的

用法：
    python backup_snapshot.py                    # 建快照，备注自动填时间
    python backup_snapshot.py "种子地图 CSS 增量"  # 带备注
    python backup_snapshot.py --list             # 列出已有快照
    python backup_snapshot.py --verify <名字>     # 校验某个快照完整性
"""
from __future__ import annotations

import os
import sys
import json
import time
import shutil
import hashlib
import datetime

PROJECT = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"
SKILLS = r"C:/Users/Lenovo/.workbuddy-ai/skills"
MEMORY = r"C:/Users/Lenovo/WorkBuddy AI/2026-10-23-01-34-18/.workbuddy-ai/memory"
WS = r"C:/Users/Lenovo/WorkBuddy AI/2026-10-19-23-00-18"
DEST = r"D:/游园/备份库"
SNAPS = os.path.join(DEST, "snapshots")
# 统一日志写在共用根，三端共看；端名由 ACTOR 决定
ACTOR = os.environ.get("YY_ACTOR", "WorkBuddy")
LOG = os.path.join(os.path.dirname(DEST), "LOG.md")
KEEP = 8

# 备份的文本类型（其余一律跳过）
EXTS = {
    ".go", ".js", ".mjs", ".cjs", ".ts", ".tsx", ".jsx",
    ".css", ".scss", ".html", ".htm",
    ".md", ".txt", ".json", ".jsonl", ".toml", ".yaml", ".yml", ".ini", ".cfg",
    ".py", ".sh", ".ps1", ".bat", ".cmd",
    ".lua", ".sql",
    ".mod", ".sum", ".work",
    ".h", ".hpp", ".cpp", ".c", ".cc",
    ".gitignore", ".env.example",
}

# 整目录跳过（构建产物 / 依赖 / 隐私 / 缓存）
SKIP_DIRS = {
    "build", ".skillcheck", ".git", "node_modules", "private", "preview",
    ".playwright-cli", "dist", ".wrangler", "__pycache__", ".pytest_cache",
    ".idea", ".vscode", "third_party", ".cache", "chrome-prof-ts",
    "chrome-prof-ts2", "cf-browser-profile", "cf-cdp-profile",
}

# 大文件保护：超过这个就跳过（避免把 1.1MB btree.dat 之外的意外大文件拖进来）
MAX_BYTES = 4 * 1024 * 1024


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def walk(src_root, rel_prefix, out, stats):
    for dp, dns, fns in os.walk(src_root):
        dns[:] = [d for d in dns if d.lower() not in SKIP_DIRS]
        for fn in fns:
            p = os.path.join(dp, fn)
            ext = os.path.splitext(fn)[1].lower()
            if ext not in EXTS:
                stats["skip_ext"] += 1
                continue
            try:
                sz = os.path.getsize(p)
            except OSError:
                continue
            if sz > MAX_BYTES:
                stats["skip_big"] += 1
                continue
            rel = os.path.join(rel_prefix, os.path.relpath(p, src_root))
            out.append((p, rel, sz))
            stats["bytes"] += sz
            stats["files"] += 1


def build(note=""):
    stamp = datetime.datetime.now().strftime("%Y%m%d-%H%M")
    name = stamp
    i = 2
    while os.path.isdir(os.path.join(SNAPS, name)):
        name = "%s-%d" % (stamp, i)
        i += 1
    dst = os.path.join(SNAPS, name)
    os.makedirs(dst, exist_ok=True)

    plan = []
    stats = {"files": 0, "bytes": 0, "skip_ext": 0, "skip_big": 0}

    walk(PROJECT, "src", plan, stats)
    walk(SKILLS, "skills", plan, stats)
    walk(MEMORY, "memory", plan, stats)
    # 需求原话与台账（在旧工作区）
    if os.path.isdir(WS):
        for fn in os.listdir(WS):
            if fn.endswith(".md"):
                p = os.path.join(WS, fn)
                try:
                    sz = os.path.getsize(p)
                except OSError:
                    continue
                if sz <= MAX_BYTES:
                    plan.append((p, os.path.join("ledger", fn), sz))
                    stats["files"] += 1
                    stats["bytes"] += sz
    # MCP 配置
    mcp = r"C:/Users/Lenovo/.workbuddy-ai/mcp.json"
    if os.path.isfile(mcp):
        plan.append((mcp, os.path.join("config", "mcp.json"), os.path.getsize(mcp)))
        stats["files"] += 1

    print("快照 %s：计划 %d 个文件 / %.1f MB"
          % (name, stats["files"], stats["bytes"] / 1048576.0))

    manifest = []
    for src, rel, sz in plan:
        target = os.path.join(dst, rel)
        os.makedirs(os.path.dirname(target), exist_ok=True)
        try:
            shutil.copy2(src, target)
        except Exception as e:  # noqa: BLE001
            print("  ! 复制失败 %s (%s)" % (rel, e))
            continue
        manifest.append({"path": rel.replace("\\", "/"), "bytes": sz, "sha256": sha256(target)})

    meta = {
        "name": name,
        "created": datetime.datetime.now().isoformat(timespec="seconds"),
        "note": note,
        "project": PROJECT,
        "file_count": len(manifest),
        "total_bytes": sum(m["bytes"] for m in manifest),
        "skipped_by_ext": stats["skip_ext"],
        "skipped_too_big": stats["skip_big"],
        "excluded_dirs": sorted(SKIP_DIRS),
    }
    with open(os.path.join(dst, "manifest.json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump({"meta": meta, "files": manifest}, f, ensure_ascii=False, indent=1)

    with open(os.path.join(DEST, "LATEST"), "w", encoding="utf-8", newline="\n") as f:
        f.write(name)

    append_log(meta)
    prune()

    print("完成：%s" % dst)
    print("  %d 个文件 / %.1f MB" % (len(manifest), meta["total_bytes"] / 1048576.0))
    print("  跳过：按类型 %d，超 4MB %d" % (stats["skip_ext"], stats["skip_big"]))
    print("  回滚示例：")
    print("    python backup_restore.py %s" % name)
    return name


def append_log(meta):
    new = not os.path.exists(LOG)
    with open(LOG, "a", encoding="utf-8", newline="\n") as f:
        if new:
            f.write("# D:/游园 · 统一日志（三端共用）\n\n")
            f.write("> 只增不改。格式：`## YYYY-MM-DD HH:MM · <谁> · <动作>`\n\n---\n\n")
        f.write("## %s · %s · 建备份快照\n"
                % (meta["created"].replace("T", " ")[:16], ACTOR))
        f.write("- 改了什么：快照 `%s`，备注「%s」\n"
                % (meta["name"], meta["note"] or "（未填）"))
        f.write("- 怎么验证：%d 个文件 / %.1f MB，manifest.json 含逐文件 sha256\n"
                % (meta["file_count"], meta["total_bytes"] / 1048576.0))
        f.write("- 回滚点：`备份库/snapshots/%s`\n\n" % meta["name"])


def prune():
    if not os.path.isdir(SNAPS):
        return
    names = sorted(d for d in os.listdir(SNAPS) if os.path.isdir(os.path.join(SNAPS, d)))
    for old in names[:-KEEP]:
        shutil.rmtree(os.path.join(SNAPS, old), ignore_errors=True)
        print("  清理旧快照 %s" % old)


def do_list():
    if not os.path.isdir(SNAPS):
        print("还没有快照")
        return
    names = sorted(d for d in os.listdir(SNAPS) if os.path.isdir(os.path.join(SNAPS, d)))
    latest = ""
    lp = os.path.join(DEST, "LATEST")
    if os.path.isfile(lp):
        latest = open(lp, encoding="utf-8").read().strip()
    print("备份库 %s（保留最近 %d 个）\n" % (DEST, KEEP))
    for n in names:
        mf = os.path.join(SNAPS, n, "manifest.json")
        if os.path.isfile(mf):
            m = json.load(open(mf, encoding="utf-8"))["meta"]
            tag = " ← LATEST" if n == latest else ""
            print("  %s  %4d 文件  %7.1f MB  %s%s"
                  % (n, m["file_count"], m["total_bytes"] / 1048576.0, m["note"] or "-", tag))
        else:
            print("  %s  (无 manifest)" % n)


def do_verify(name):
    d = os.path.join(SNAPS, name)
    mf = os.path.join(d, "manifest.json")
    if not os.path.isfile(mf):
        print("找不到 %s" % mf)
        return
    data = json.load(open(mf, encoding="utf-8"))
    bad = 0
    for m in data["files"]:
        p = os.path.join(d, m["path"].replace("/", os.sep))
        if not os.path.isfile(p):
            print("  缺失 %s" % m["path"])
            bad += 1
        elif sha256(p) != m["sha256"]:
            print("  校验不符 %s" % m["path"])
            bad += 1
    print("校验 %s：%d 个文件，异常 %d" % (name, len(data["files"]), bad))


def main():
    a = sys.argv[1:]
    if a and a[0] == "--list":
        do_list()
    elif a and a[0] == "--verify":
        do_verify(a[1])
    else:
        note = " ".join(a).strip()
        os.makedirs(SNAPS, exist_ok=True)
        build(note)


if __name__ == "__main__":
    main()
