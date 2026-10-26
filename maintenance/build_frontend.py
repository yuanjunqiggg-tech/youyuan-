#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""build_frontend.py —— 前端一键构建（源码防暴露）。

铁律第十一条：部署的前端必须压缩混淆、禁 sourcemap，同行不能直接读源码。

流程：
1. frontend/（唯一可读源码）→ cp 到 cloudflare/public/（部署快照根）
2. public/js/**/*.js 用 esbuild minify（--minify --target=es2019，
   自带短变量名/去空白去注释/死代码消除；字符串不阵列化，保证
   DOM 模板与跨文件全局行为不变）
3. public/css/*.css 一并压缩
4. index.html 原样（结构不动）；不产出 .map
5. 输出每个文件的压缩率清单

用法：python3 build_frontend.py [--dry-run]
"""
import io, os, shutil, subprocess, sys

NODE = r"C:/Users/Lenovo/.workbuddy-ai/binaries/node/versions/22.22.2-6/node.exe"

def find_esbuild():
    import glob
    hits = glob.glob("C:/Users/Lenovo/AppData/Local/npm-cache/_npx/*/node_modules/.bin/esbuild")
    if not hits:
        raise SystemExit("esbuild not found in npx cache")
    # glob 在 Windows 返回混合分隔符；先统一再替换
    shim = hits[0].replace("\\", "/")
    # .bin/esbuild 是 shim；真实 JS 入口是平级的 esbuild/bin/esbuild
    real = shim.replace("/.bin/esbuild", "/esbuild/bin/esbuild")
    if not os.path.exists(real):
        raise SystemExit("esbuild JS entry missing: " + real)
    return real

SRC = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/upstream/frontend"
DST = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/cloudflare/public"

DRY = "--dry-run" in sys.argv
changed = []
total_before = total_after = 0

def run_esbuild(file_path):
    entry = find_esbuild()
    out = file_path + ".min.tmp"
    cmd = [NODE, entry, file_path,
           "--minify", "--target=es2019", "--legal-comments=none",
           "--outfile=" + out]
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=120)
    if r.returncode != 0:
        raise SystemExit("esbuild failed on %s:\n%s" % (file_path, r.stderr[:500]))
    before = os.path.getsize(file_path)
    after = os.path.getsize(out)
    if not DRY:
        shutil.move(out, file_path)
    else:
        os.remove(out)
    return before, after

def minify_tree(root, exts, label):
    global total_before, total_after
    n = 0
    for dirpath, _dirs, files in os.walk(root):
        for f in files:
            if not f.endswith(exts):
                continue
            p = os.path.join(dirpath, f)
            before, after = run_esbuild(p)
            total_before += before
            total_after += after
            changed.append((label, os.path.relpath(p, DST), before, after))
            n += 1
    return n

# 1) 同步源码到部署快照
if not DRY:
    for name in os.listdir(SRC):
        s = os.path.join(SRC, name)
        d = os.path.join(DST, name)
        if os.path.isdir(s):
            if os.path.exists(d):
                shutil.rmtree(d)
            shutil.copytree(s, d)
        else:
            shutil.copy2(s, d)
print("[1] frontend -> public 同步完成")

# 2/3) 压缩 js + css
njs = minify_tree(os.path.join(DST, "js"), (".js",), "js")
ncss = minify_tree(os.path.join(DST, "css"), (".css",), "css")
print("[2] minify: js %d 个, css %d 个" % (njs, ncss))

# 4) 清理任何 sourcemap 残留（禁止上线）
for dirpath, _dirs, files in os.walk(DST):
    for f in files:
        if f.endswith(".map"):
            os.remove(os.path.join(dirpath, f))
            print("[4] removed", os.path.join(dirpath, f))
print("[4] 无 .map 残留")

print("\n压缩清单（字节）：")
for label, rel, b, a in changed:
    print("  [%s] %-52s %8d -> %7d  (%.0f%%)" % (label, rel, b, a, 100.0 * a / b if b else 0))
print("\n合计: %d -> %d 字节 (%.1f%%)" % (total_before, total_after,
    100.0 * total_after / total_before if total_before else 0))
print("dry-run" if DRY else "DONE — 可以 wrangler deploy")
