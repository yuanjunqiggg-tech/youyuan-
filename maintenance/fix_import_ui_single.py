#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""导入 UI：worker 选项由浓缩多列网格改回单列长条。
每项占满整宽：左 checkbox，中间 名称(粗体)+类型/服务器 两行说明，视觉上是清晰的一条条。
用法：python3 fix_import_ui_single.py
"""
import io, sys

PATH = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/upstream/frontend/js/features/building.js"
src = io.open(PATH, encoding="utf-8").read()
orig = src

def rep(old, new, count=1):
    global src
    n = src.count(old)
    if n != count:
        print("ANCHOR-FAIL(%d!=%d): %r" % (n, count, old[:80])); sys.exit(1)
    src = src.replace(old, new, count)

# 1) 网格容器：单列纵向（每条占满一行）
rep(
    '<div id="yy-worker-grid" style="display:grid;grid-template-columns:repeat(auto-fill,minmax(230px,1fr));gap:8px"></div>',
    '<div id="yy-worker-grid" style="display:grid;grid-template-columns:1fr;gap:10px"></div>',
)

# 2) label：长条化（更高、内边距更大、圆角、hover 反馈、不浓缩）
rep(
    'label.style.cssText = "display:flex;align-items:center;gap:10px;padding:10px;height:100%;box-sizing:border-box;border:1px solid var(--phx-border-light);border-radius:8px";',
    'label.style.cssText = "display:flex;align-items:center;gap:12px;min-height:52px;padding:12px 16px;box-sizing:border-box;border:1px solid var(--phx-border-light);border-radius:10px;cursor:pointer;transition:border-color .15s,background .15s";',
)

# 3) checkbox 尺寸略放大
rep(
    '    input.style.width = "20px";',
    '    input.style.width = "18px";\n    input.style.height = "18px";\n    input.style.flexShrink = "0";',
)

# 4) 内容：名称(粗体) + 类型/服务器 两行说明，长条内容更透气
rep(
    '    label.append(input, document.createTextNode(w.name + (w.main ? " \\xB7 \\u5F53\\u524D\\u4E3B\\u8FDE\\u63A5" : " \\xB7 \\u5B50\\u8FDE\\u63A5") + " \\xB7 " + w.server));',
    '    var info = document.createElement("span");\n'
    '    info.style.cssText = "display:flex;flex-direction:column;gap:3px;flex:1;min-width:0";\n'
    '    var wname = document.createElement("strong");\n'
    '    wname.textContent = w.name;\n'
    '    wname.style.fontSize = "14px";\n'
    '    var meta = document.createElement("span");\n'
    '    meta.textContent = (w.main ? "\\u5F53\\u524D\\u4E3B\\u8FDE\\u63A5" : "\\u5B50\\u8FDE\\u63A5") + " \\u00B7 \\u670D\\u52A1\\u5668 " + w.server;\n'
    '    meta.style.cssText = "font-size:12px;color:var(--phx-text-secondary)";\n'
    '    info.append(wname, meta);\n'
    '    label.append(input, info);\n'
    '    label.addEventListener("mouseenter", function() { label.style.borderColor = "var(--phx-accent,#888)"; });\n'
    '    label.addEventListener("mouseleave", function() { label.style.borderColor = "var(--phx-border-light)"; });',
)

assert src != orig
io.open(PATH, "w", encoding="utf-8", newline="\n").write(src)
print("building.js patched OK")
