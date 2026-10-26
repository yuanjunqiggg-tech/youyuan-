#!/usr/bin/env python3
"""给 Go 二进制「封印」：把自身 sha256 回填进源码里预留的槽位。

原理见 upstream/protect_selfcheck.go 顶部注释。简版：
  - Go 二进制里有一段常量 "YYSEALSC1" + 64 个 '0' + "YYSEALEND"；
  - 本脚本把槽位清零后对整个文件求 sha256，再把十六进制写回槽位；
  - 运行时运行库重做同一件事并比对，任何字节改动都会导致不一致。

用法：
  python maintenance/seal_binary.py <二进制路径>          # 封印（幂等）
  python maintenance/seal_binary.py --verify <二进制路径>  # 只校验不修改
  python maintenance/seal_binary.py --stats  <二进制路径>  # 打印可读性统计

注意：封印后**不能**再对这个文件做 strip/upx 等任何改写，否则封签必然失效。
"""
from __future__ import annotations

import hashlib
import pathlib
import sys

PREFIX = b"YYSEALSC1"
SUFFIX = b"YYSEALEND"
SLOT = 64


def find_slots(data: bytes) -> list[int]:
    out: list[int] = []
    total = len(PREFIX) + SLOT + len(SUFFIX)
    start = 0
    while True:
        at = data.find(PREFIX, start)
        if at < 0:
            return out
        start = at + 1
        if at + total > len(data):
            continue
        if data[at + total - len(SUFFIX) : at + total] == SUFFIX:
            out.append(at)
    return out


def seal(data: bytes) -> tuple[bytes, str]:
    slots = find_slots(data)
    if len(slots) != 1:
        raise SystemExit(f"封印记录数量异常：期望 1，实际 {len(slots)}（二进制可能不是从当前源码构建的）")
    idx = slots[0]
    slot_start = idx + len(PREFIX)
    buf = bytearray(data)
    buf[slot_start : slot_start + SLOT] = b"0" * SLOT
    digest = hashlib.sha256(bytes(buf)).hexdigest().encode()
    buf[slot_start : slot_start + SLOT] = digest
    return bytes(buf), digest.decode()


def verify(data: bytes) -> tuple[bool, str]:
    slots = find_slots(data)
    if len(slots) != 1:
        return False, f"封印记录数量异常：{len(slots)}"
    idx = slots[0]
    slot_start = idx + len(PREFIX)
    slot = data[slot_start : slot_start + SLOT]
    if slot.strip(b"0") == b"":
        return False, "槽位为空（未封印）"
    buf = bytearray(data)
    buf[slot_start : slot_start + SLOT] = b"0" * SLOT
    actual = hashlib.sha256(bytes(buf)).hexdigest()
    if slot.decode(errors="replace").lower() != actual:
        return False, f"封签不符：槽位={slot.decode(errors='replace')} 实际={actual}"
    return True, actual


def readable_strings(data: bytes) -> None:
    """粗略统计：明文里还能不能直接捞到敏感串。用于自证收敛效果。"""
    probes = [
        "netEase".lower(),
        "netease.com",
        "163.com",
        "126.net",
        "youyuanqi.dpdns.org",
        "updates/android/latest.apk",
        "你是这个群的管理员之一",
        "当前用户是已授权管理员",
    ]
    low = data.lower()
    print("明文命中情况：")
    for p in probes:
        hit = low.count(p.lower().encode())
        print(f"  {p:<32} {'命中 ' + str(hit) + ' 次' if hit else '未出现'}")


def main(argv: list[str]) -> int:
    args = [a for a in argv[1:] if not a.startswith("--")]
    flags = {a for a in argv[1:] if a.startswith("--")}
    if len(args) != 1:
        print(__doc__, file=sys.stderr)
        return 2
    path = pathlib.Path(args[0])
    if not path.is_file():
        print(f"文件不存在：{path}", file=sys.stderr)
        return 2
    data = path.read_bytes()
    print(f"目标：{path}  {len(data)} 字节  sha256={hashlib.sha256(data).hexdigest()}")

    if "--stats" in flags:
        readable_strings(data)
        return 0

    if "--verify" in flags:
        ok, detail = verify(data)
        print(("校验通过：" if ok else "校验失败：") + detail)
        return 0 if ok else 1

    sealed, digest = seal(data)
    tmp = path.with_suffix(path.suffix + ".sealed")
    tmp.write_bytes(sealed)
    tmp.replace(path)
    print(f"已封印：槽位={digest}")
    ok, detail = verify(path.read_bytes())
    print(("复核通过：" if ok else "复核失败：") + detail)
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
