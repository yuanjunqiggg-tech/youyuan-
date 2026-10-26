#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
gen_btree.py —— 把参考实现的 src/btree.mjs 转成 Go 内核用的 seedv2_btree.dat

【归属】agent: seed-btree
【输入】D:/种子查询器自研/src/btree.mjs（只读，绝不修改原件）
        5 张表：btree18 / btree192 / btree19 / btree20 / btree21wd
【输出】upstream/seedv2_btree.dat

为什么不用 go:embed 直接塞 mjs、也不写成 Go 字面量：
  1. Go 字面量 4 万多个 uint64 → 编译期巨慢且内存爆炸；
  2. 运行时解析 1.1MB JS 文本 → 需要一个 JS 解释器，客户端没有。

.dat 格式（全部小端，ver 1）：
  magic   [4]byte = "YYBT"
  ver     uint16  = 1
  count   uint16  = 5
  每张表重复：
    nameLen uint8 + name []byte
    order   uint8
    nsteps  uint8  + steps   []int32   （nsteps<=16，与 Go 侧校验一致）
    nparam  uint32 + params  []int32   （2*nparam 个，成对 lo,hi）
    nnodes  uint32 + nodes   []uint64

幂等：同样的输入必然产出逐字节相同的 .dat（不写时间戳、不写主机名）。

用法：
  python gen_btree.py                    # 用默认路径
  python gen_btree.py <btree.mjs> <out.dat>
"""

import os
import re
import struct
import sys

MAGIC = b"YYBT"
VERSION = 1
TABLE_ORDER = ["btree18", "btree192", "btree19", "btree20", "btree21wd"]

DEFAULT_SRC = r"D:/种子查询器自研/src/btree.mjs"
DEFAULT_OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                           "..", "..", "seedv2_btree.dat")


def parse_mjs(path):
    """解析 btree.mjs → [(name, order, steps, params, nodes), ...]"""
    with open(path, "r", encoding="utf-8") as f:
        text = f.read()

    tables = {}
    # 每张表长这样：export const btreeXX = { ... };
    for m in re.finditer(r"export const (\w+)\s*=\s*\{", text):
        name = m.group(1)
        body = _brace_body(text, m.end() - 1)
        tables[name] = _parse_table(name, body)

    missing = [t for t in TABLE_ORDER if t not in tables]
    if missing:
        raise SystemExit("btree.mjs 缺少表: %s" % ", ".join(missing))

    # 输出顺序固定，保证幂等
    return [tables[t] for t in TABLE_ORDER]


def _brace_body(text, start):
    """从 '{' 开始做括号配平，返回对象体（不含外层大括号）。"""
    depth = 0
    i = start
    n = len(text)
    while i < n:
        c = text[i]
        if c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0:
                return text[start + 1:i]
        i += 1
    raise SystemExit("括号不配平：offset %d" % start)


def _parse_table(name, body):
    m = re.search(r"order\s*:\s*(\d+)", body)
    if not m:
        raise SystemExit("%s: 找不到 order" % name)
    order = int(m.group(1))

    m = re.search(r"steps\s*:\s*\[([^\]]*)\]", body)
    if not m:
        raise SystemExit("%s: 找不到 steps" % name)
    steps = [int(x) for x in m.group(1).replace("\n", " ").split(",") if x.strip()]

    # param 段：从 "param: [" 到 "nodes: [" 之前
    i = body.find("param:")
    j = body.find("nodes:")
    if i < 0 or j < 0 or j <= i:
        raise SystemExit("%s: param/nodes 段缺失" % name)
    param_part = body[i:j]
    pairs = re.findall(r"\[\s*(-?\d+)\s*,\s*(-?\d+)\s*\]", param_part)
    params = [(int(lo), int(hi)) for lo, hi in pairs]

    nodes_part = body[j:]
    nodes = []
    for lit in re.findall(r"0x([0-9a-fA-F]+)n|(?<![\w.])(\d+)n", nodes_part):
        hexpart, decpart = lit
        nodes.append(int(hexpart, 16) if hexpart else int(decpart))
    if not nodes:
        raise SystemExit("%s: nodes 为空" % name)

    # 数据合理性自检（与 Go 端校验对齐，生成期就炸，别把坏数据编进二进制）
    if len(steps) > 16:
        raise SystemExit("%s: steps 过多 %d" % (name, len(steps)))
    if len(params) > 100000:
        raise SystemExit("%s: params 过多 %d" % (name, len(params)))
    if len(nodes) > 200000:
        raise SystemExit("%s: nodes 过多 %d" % (name, len(nodes)))
    if order <= 0 or order > 255:
        raise SystemExit("%s: order 越界 %d" % (name, order))

    # 每个节点低 48 位是 6 个字节参数下标，必须都落在 param 范围内
    for k, nd in enumerate(nodes):
        for b in range(6):
            pidx = (nd >> (8 * b)) & 0xFF
            if pidx >= len(params):
                raise SystemExit("%s: node[%d] 参数下标 %d 越界(param=%d)"
                                 % (name, k, pidx, len(params)))

    return {
        "name": name,
        "order": order,
        "steps": steps,
        "params": params,
        "nodes": nodes,
    }


def write_dat(tables, out_path):
    buf = bytearray()
    buf += MAGIC
    buf += struct.pack("<H", VERSION)
    buf += struct.pack("<H", len(tables))

    for t in tables:
        nb = t["name"].encode("utf-8")
        if len(nb) > 255:
            raise SystemExit("表名过长: %s" % t["name"])
        buf += struct.pack("<B", len(nb)) + nb
        buf += struct.pack("<B", t["order"])

        buf += struct.pack("<B", len(t["steps"]))
        for s in t["steps"]:
            buf += struct.pack("<i", s)

        buf += struct.pack("<I", len(t["params"]))
        for lo, hi in t["params"]:
            buf += struct.pack("<ii", lo, hi)

        buf += struct.pack("<I", len(t["nodes"]))
        for nd in t["nodes"]:
            buf += struct.pack("<Q", nd)

    out_path = os.path.normpath(out_path)
    os.makedirs(os.path.dirname(out_path), exist_ok=True)
    with open(out_path, "wb") as f:
        f.write(buf)
    return out_path, len(buf)


def main():
    src = sys.argv[1] if len(sys.argv) > 1 else DEFAULT_SRC
    out = sys.argv[2] if len(sys.argv) > 2 else DEFAULT_OUT

    tables = parse_mjs(src)
    out_path, size = write_dat(tables, out)

    print("=== gen_btree: %s -> %s" % (src, out_path))
    print("%-10s %5s %6s %7s %7s" % ("table", "order", "steps", "params", "nodes"))
    total_nodes = 0
    for t in tables:
        total_nodes += len(t["nodes"])
        print("%-10s %5d %6d %7d %7d"
              % (t["name"], t["order"], len(t["steps"]),
                 len(t["params"]), len(t["nodes"])))
        # 前 10 个节点的原始 uint64，供 Go 测试逐位对拍
        head = " ".join("%016x" % n for n in t["nodes"][:10])
        print("  head10[%s]: %s" % (t["name"], head))
    print("total nodes = %d ; dat size = %d bytes (%.1f KB)"
          % (total_nodes, size, size / 1024.0))


if __name__ == "__main__":
    main()
