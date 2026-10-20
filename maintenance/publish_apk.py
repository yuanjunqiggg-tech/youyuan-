#!/usr/bin/env python3
"""把已签名的 APK 发布到 Cloudflare 静态目录。

线上 /updates/android/latest.apk 不是静态文件，而是 cloudflare/updates.mjs 按
latest.json 的 parts 列表逐片回源拼出来的。所以发布必须做三件事：
  1. 把 APK 切成固定大小的分片，写进 public/updates/blobs/<version>/partNNN
  2. 生成 latest.json（size 必须等于所有分片之和，否则服务端直接 502）
  3. 由调用方再跑一次 wrangler deploy 把静态资源推上去

签名证书指纹沿用上一版清单，因为用的是同一把 release key；
如果换了密钥，必须显式传 --cert。

用法：
    python maintenance/publish_apk.py build/youyuan-v0.10.76.apk
    python maintenance/publish_apk.py <apk> --version 0.10.76 --code 1076
"""
from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import sys
from pathlib import Path

PART_SIZE = 8 * 1024 * 1024  # 与线上历史分片一致，8 MiB
PACKAGE = "org.youyuan.toolbox"
MIN_SDK = 26


def sha256_of(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def split_apk(apk: Path, target: Path, part_size: int) -> list[dict[str, object]]:
    if target.exists():
        shutil.rmtree(target)
    target.mkdir(parents=True)
    parts: list[dict[str, object]] = []
    with apk.open("rb") as handle:
        index = 1
        while True:
            chunk = handle.read(part_size)
            if not chunk:
                break
            name = f"part{index:03d}"
            (target / name).write_bytes(chunk)
            parts.append({"path": f"/updates/blobs/{target.name}/{name}", "size": len(chunk)})
            index += 1
    if not parts:
        raise SystemExit("APK 是空文件，拒绝发布")
    return parts


def previous_manifest(path: Path) -> dict:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return {}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("apk", type=Path)
    parser.add_argument("--version", default=None)
    parser.add_argument("--code", type=int, default=None)
    parser.add_argument("--cert", default=None)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parent.parent)
    args = parser.parse_args()

    apk: Path = args.apk
    if not apk.is_file():
        print(f"找不到 APK：{apk}", file=sys.stderr)
        return 2

    version = args.version
    if version is None:
        # youyuan-v0.10.76.apk -> 0.10.76
        stem = apk.stem
        version = stem.split("v", 1)[1] if "v" in stem else stem
    code = args.code
    if code is None:
        digits = version.replace(".", "")
        if not digits.isdigit():
            print(f"无法从版本号推出 versionCode：{version}，请用 --code 指定", file=sys.stderr)
            return 2
        code = int(digits)

    updates_dir = args.root / "cloudflare" / "public" / "updates" / "android"
    manifest_path = updates_dir / "latest.json"
    updates_dir.mkdir(parents=True, exist_ok=True)
    old = previous_manifest(manifest_path)

    cert = args.cert or old.get("signing_cert_sha256")
    if not cert:
        print("缺少签名证书指纹：请用 --cert 指定，或先保证已有 latest.json", file=sys.stderr)
        return 2

    blob_dir = args.root / "cloudflare" / "public" / "updates" / "blobs" / version
    parts = split_apk(apk, blob_dir, PART_SIZE)
    total = sum(int(part["size"]) for part in parts)
    digest = sha256_of(apk)
    if total != apk.stat().st_size:
        print("分片总大小与原文件不一致，拒绝发布", file=sys.stderr)
        return 1

    manifest = {
        "version": version,
        "version_code": code,
        "package": PACKAGE,
        "min_sdk": MIN_SDK,
        "size": total,
        "sha256": digest,
        "apk_url": "https://updates.youyuanqi.dpdns.org/updates/android/latest.apk",
        "signing_cert_sha256": cert,
        "parts": parts,
    }
    manifest_path.write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )

    print(f"版本      {version} (code {code})")
    print(f"大小      {total} bytes / {len(parts)} 片")
    print(f"sha256    {digest}")
    print(f"分片目录  {blob_dir}")
    print(f"清单      {manifest_path}")
    print("下一步：cd cloudflare && npx wrangler deploy")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
