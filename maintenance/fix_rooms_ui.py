#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""修复房间发现 UI 三个问题：
1) 点"重新查询"自动连接 —— discover 增加 listOnly 模式，单候选不再自动 choose
2) 封面只显示一瞬间 —— choose 成功后不再清空列表，改为高亮选中卡片 + 顶部状态条
3) 服主/人数/规模展示弱 —— 卡片右上角加人数徽标(在线 n/40)、左上角版本徽标，房主单独一行
用法：python3 fix_rooms_ui.py
"""
import io, sys

PATH = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/upstream/frontend/js/youyuan_room_discovery.js"

src = io.open(PATH, encoding="utf-8").read()
orig = src

def rep(old, new, count=1):
    global src
    n = src.count(old)
    if n != count:
        print("ANCHOR-FAIL(%d!=%d): %r" % (n, count, old[:70])); sys.exit(1)
    src = src.replace(old, new, count)

# 1) choose 成功后：不清空列表，改为标记选中 + 顶部状态条（封面保留）
rep(
    "      renderChoice(result.selected, original);\n      await start(result.selected, original);",
    "      markSelected(result.selected);\n      await start(result.selected, original);",
)

# 2) renderChoice 的"重新查询"按钮：listOnly 模式（只列候选，不自动连接）
rep(
    "    change.onclick = function() {\n      discover(true);\n    };",
    "    change.onclick = function() {\n      discover(true, true);\n    };",
)

# 3) render() 卡片：绑定 data-id、人数徽标、版本徽标
rep(
    '        var card = el("article", null, "yy-room-card");\n        var cover = image(v.cover_url, "yy-room-cover");\n        if (cover) card.append(cover);',
    '        var card = el("article", null, "yy-room-card");\n        card.setAttribute("data-id", v.id);\n'
    '        var verBadge = el("span", v.version || "", "yy-room-ver");\n'
    '        if (v.version) card.append(verBadge);\n'
    '        var cntBadge = el("span", "", "yy-room-badge");\n'
    '        if (v.players != null) cntBadge.textContent = "\\u5728\\u7EBF " + v.players + (v.max_players != null ? "/" + v.max_players : "");\n'
    '        else if (v.max_players != null) cntBadge.textContent = "\\u5BB9\\u91CF " + v.max_players;\n'
    '        if (cntBadge.textContent) card.append(cntBadge);\n'
    '        var cover = image(v.cover_url, "yy-room-cover");\n        if (cover) card.append(cover);',
)

# 4) discover 签名与分支：listOnly 时 selected 不 start、单候选不自动 choose
rep(
    "  async function discover(force) {",
    "  async function discover(force, listOnly) {",
)
rep(
    "      if (result.selected) {\n        renderChoice(result.selected, original);\n        await start(result.selected, original);\n        return true;\n      }",
    "      if (result.selected) {\n        if (!listOnly) {\n          renderChoice(result.selected, original);\n          await start(result.selected, original);\n          return true;\n        }\n        if (!(result.candidates || []).length) { renderChoice(result.selected, original); return true; }\n      }",
)
rep(
    "      if (result.complete && (result.candidates || []).length === 1) await choose(result.candidates[0], original);",
    "      if (!listOnly && result.complete && (result.candidates || []).length === 1) await choose(result.candidates[0], original);",
)

# 5) 新增 markSelected：保留列表，仅高亮选中卡片 + 顶部连接状态条
mark = (
    "  function markSelected(candidate) {\n"
    "    var box = document.getElementById(\"yy-room-results\");\n"
    "    if (box) {\n"
    "      var old = document.getElementById(\"yy-room-selected-bar\");\n"
    "      if (old) old.remove();\n"
    "      var bar = el(\"p\", \"\\u5DF2\\u9009\\u62E9 \" + labels[candidate.type] + \" \\u00B7 \" + (candidate.name || candidate.number || candidate.room_id) + \" \\u2014 \\u6B63\\u5728\\u8FDE\\u63A5\\u2026\", \"yy-room-selected-bar\");\n"
    "      bar.id = \"yy-room-selected-bar\";\n"
    "      box.prepend(bar);\n"
    "      var card = box.querySelector('.yy-room-card[data-id=\"' + candidate.id + '\"]');\n"
    "      if (card) {\n"
    "        card.classList.add(\"yy-room-card-selected\");\n"
    "        var btn = card.querySelector(\"button\");\n"
    "        if (btn) { btn.disabled = true; btn.textContent = \"\\u5DF2\\u9009\\u62E9\\uFF0C\\u8FDE\\u63A5\\u4E2D\\u2026\"; }\n"
    "        card.scrollIntoView({ block: \"nearest\" });\n"
    "      }\n"
    "    }\n"
    "  }\n"
    "  function renderChoice(candidate, original) {"
)
rep("  function renderChoice(candidate, original) {", mark)

assert src != orig
io.open(PATH, "w", encoding="utf-8", newline="\n").write(src)
print("room_discovery.js patched OK")
