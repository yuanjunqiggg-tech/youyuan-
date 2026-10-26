#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""backup_restore.py —— 从专用备份库回滚

安全设计（三道闸）：
  1. **默认 dry-run**：不加 --apply 只打印将要覆盖的文件，不动磁盘
  2. **回滚前自动建快照**：把当前状态先存一份，回滚错了还能再回来
  3. **逐文件 sha256 校验**：只覆盖内容确实不同的文件，相同的跳过（避免无意义改写 mtime）

用法：
    python backup_restore.py --list                       # 列出可回滚快照
    python backup_restore.py 20261026-1606                # 预览（dry-run）
    python backup_restore.py 20261026-1606 --apply        # 真回滚
    python backup_restore.py 20261026-1606 --apply --only src   # 只回滚源码
    python backup_restore.py 20261026-1606 --apply --file src/upstream/frontend/index.html
"""
from __future__ import annotations

import os
import sys
import json
import hashlib
import datetime

PROJECT = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project"
SKILLS = r"C:/Users/Lenovo/.workbuddy-ai/skills"
MEMORY = r"C:/Users/Lenovo/WorkBuddy AI/2026-10-23-01-34-18/.workbuddy-ai/memory"
WS = r"C:/Users/Lenovo/WorkBuddy AI/2026-10-19-23-00-18"
DEST = r"D:/游园/备份库"
SNAPS = os.path.join(DEST, "snapshots")

# 快照内前缀 → 还原目标根
ROOTS = {
    "src": PROJECT,
    "skills": SKILLS,
    "memory": MEMORY,
    "ledger": WS,
    "config": r"C:/Users/Lenovo/.workbuddy-ai",
}


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def do_list():
    if not os.path.isdir(SNAPS):
        print("备份库为空")
        return
    latest = ""
    lp = os.path.join(DEST, "LATEST")
    if os.path.isfile(lp):
        latest = open(lp, encoding="utf-8").read().strip()
    for n in sorted(d for d in os.listdir(SNAPS) if os.path.isdir(os.path.join(SNAPS, d))):
        mf = os.path.join(SNAPS, n, "manifest.json")
        if not os.path.isfile(mf):
            print("  %s (无 manifest)" % n)
            continue
        m = json.load(open(mf, encoding="utf-8"))["meta"]
        print("  %s  %4d 文件  %7.1f MB  %s%s"
              % (n, m["file_count"], m["total_bytes"] / 1048576.0,
                 m["note"] or "-", "  ← LATEST" if n == latest else ""))


def auto_snapshot():
    """回滚前先把当前状态存一份"""
    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
    try:
        import backup_snapshot
    except Exception as e:  # noqa: BLE001
        print("  ! 无法自动快照（%s），继续回滚但失去二次回退能力" % e)
        return None
    note = "回滚前自动快照 %s" % datetime.datetime.now().strftime("%H:%M")
    return backup_snapshot.build(note)


def main():
    a = sys.argv[1:]
    if not a or a[0] in ("-h", "--help"):
        print(__doc__)
        return
    if a[0] == "--list":
        do_list()
        return

    name = a[0]
    apply_ = "--apply" in a
    only = None
    single = None
    if "--only" in a:
        only = a[a.index("--only") + 1]
    if "--file" in a:
        single = a[a.index("--file") + 1].replace("\\", "/")

    snap = os.path.join(SNAPS, name)
    mf = os.path.join(snap, "manifest.json")
    if not os.path.isfile(mf):
        print("找不到快照 %s" % name)
        do_list()
        return

    data = json.load(open(mf, encoding="utf-8"))
    files = data["files"]
    if only:
        files = [m for m in files if m["path"].split("/")[0] == only]
    if single:
        files = [m for m in files if m["path"] == single]
    if not files:
        print("筛选后没有文件可回滚")
        return

    plan = []
    for m in files:
        rel = m["path"]
        top = rel.split("/")[0]
        root = ROOTS.get(top)
        if not root:
            continue
        target = os.path.join(root, rel.replace("/", os.sep)[len(top) + 1:])
        src = os.path.join(snap, rel.replace("/", os.sep))
        if not os.path.isfile(src):
            print("  ! 快照内缺文件 %s" % rel)
            continue
        same = os.path.isfile(target) and sha256(target) == m["sha256"]
        plan.append((src, target, rel, same, m["sha256"]))

    diff = [p for p in plan if not p[3]]
    print("快照 %s：%d 个文件，需改写 %d 个，内容相同跳过 %d 个"
          % (name, len(plan), len(diff), len(plan) - len(diff)))
    for _s, t, rel, same, _h in diff[:40]:
        print("  %s %s" % ("~" if os.path.exists(t) else "+", t))
    if len(diff) > 40:
        print("  … 另有 %d 个" % (len(diff) - 40))

    if not apply_:
        print("\n这是 dry-run。确认无误后加 --apply 真正回滚。")
        return
    if not diff:
        print("已与快照一致，无需回滚。")
        return

    print("\n[1/3] 回滚前自动建快照 …")
    auto_snapshot()

    print("[2/3] 写入 %d 个文件 …" % len(diff))
    ok = bad = 0
    for src, target, rel, _same, want in diff:
        try:
            os.makedirs(os.path.dirname(target), exist_ok=True)
            with open(src, "rb") as f:
                blob = f.read()
            with open(target, "wb") as f:
                f.write(blob)
            if sha256(target) == want:
                ok += 1
            else:
                print("  ! 写入后校验不符 %s" % rel)
                bad += 1
        except Exception as e:  # noqa: BLE001
            print("  ! 写入失败 %s (%s)" % (rel, e))
            bad += 1

    print("[3/3] 完成：成功 %d，失败 %d" % (ok, bad))
    if ok:
        print("\n提醒：源码回滚后必须重新构建并部署，否则线上还是旧的。")
        print("  Worker：python maintenance/build_frontend.py && wrangler deploy")
        print("  box   ：go build + python maintenance/deploy_linux_bin.py")


if __name__ == "__main__":
    main()
