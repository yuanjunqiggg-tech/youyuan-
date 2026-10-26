#!/usr/bin/env python3
"""把 protect_secrets.go 里的 __BLOB_*__ 占位符替换成 XOR+hex 编码后的常量。

一次性工具，不参与运行时。用法：python maintenance/gen_protect_secrets.py
重复执行是幂等的（只替换仍带占位符的常量）。
"""
from __future__ import annotations

import pathlib
import re

ROOT = pathlib.Path(__file__).resolve().parent.parent
SRC = ROOT / "upstream"
KEY = "yy:2026:prism:toolbox:k9"


def enc(s: str) -> str:
    out = bytearray()
    kb = KEY.encode()
    for i, ch in enumerate(s.encode("utf-8")):
        out.append(ch ^ kb[i % len(kb)] ^ ((i * 37 + 11) & 0xFF))
    return out.hex()


def go_unescape(lit: str) -> str:
    """把 Go 源码里的双引号字面量内容还原为真实文本（只处理 \\n \\t \\" \\\\）。"""
    out = []
    i = 0
    while i < len(lit):
        c = lit[i]
        if c == "\\" and i + 1 < len(lit):
            n = lit[i + 1]
            out.append({"n": "\n", "t": "\t", '"': '"', "\\": "\\", "r": "\r"}.get(n, n))
            i += 2
            continue
        out.append(c)
        i += 1
    return "".join(out)


def extract(path: pathlib.Path, needle: str) -> str:
    for line in path.read_text(encoding="utf-8").splitlines():
        if needle in line and "prompt +=" in line:
            m = re.search(r'prompt \+= "((?:[^"\\]|\\.)*)"', line)
            if not m:
                raise SystemExit(f"找不到字符串字面量：{path.name} / {needle}")
            return go_unescape(m.group(1))
    raise SystemExit(f"找不到行：{path.name} / {needle}")


def main() -> int:
    ai = SRC / "youyuan_qq_ai.go"
    blobs = {
        "__BLOB_SKIN_SUFFIXES__": ".netease.com|.163.com|.126.net",
        "__BLOB_SKIN_HOSTS__": "crafatar.com|minotar.net|mc-heads.net",
        "__BLOB_ACCOUNT_SKIN_HOSTS__": "netease.com|.netease.com|163.com|.163.com|.126.net",
        "__BLOB_PLATFORM_ORIGIN__": "https://toolbox.youyuanqi.dpdns.org",
        "__BLOB_AUTH_FALLBACK__": "https://auth.youyuanqi.dpdns.org",
        "__BLOB_UPDATE_MANIFEST__": "https://updates.youyuanqi.dpdns.org/updates/android/latest.json",
        "__BLOB_UPDATE_HOST__": "updates.youyuanqi.dpdns.org",
        "__BLOB_UPDATE_PATH__": "/updates/android/latest.apk",
        "__BLOB_QQ_ADMIN_PERSONA__": extract(ai, "你是这个群的管理员之一"),
        "__BLOB_QQ_ADMIN_TOOLS__": extract(ai, "当前用户是已授权管理员"),
    }

    target = SRC / "protect_secrets.go"
    text = target.read_text(encoding="utf-8")
    changed = 0
    for name, plain in blobs.items():
        if name not in text:
            continue
        text = text.replace(name, enc(plain))
        changed += 1
        print(f"{name}  <- {len(plain.encode('utf-8'))}B 明文 → {len(enc(plain))} hex")
    target.write_text(text, encoding="utf-8", newline="\n")
    print(f"完成：替换 {changed} 个占位符")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
