#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""renderChoice 从"一行文字+按钮"升级为"当前服务器信息卡"：
- 设置与账号页回填/选中后，照样展示封面、头像、名称、房主、人数徽标、版本徽标
- 重新查询仍是 listOnly（只列候选，不自动连接）
用法：python3 fix_render_choice_card.py
"""
import io, sys

PATH = r"C:/Users/Lenovo/Documents/Codex/2026-09-28/apk/project/upstream/frontend/js/youyuan_room_discovery.js"
src = io.open(PATH, encoding="utf-8").read()
orig = src

old_block = (
    "  function renderChoice(candidate, original) {\n"
    "    var box = host();\n"
    "    if (!box) return;\n"
    "    box.replaceChildren();\n"
    "    box.append(el(\"p\", \"\\u5DF2\\u9009\\u62E9 \" + labels[candidate.type] + \" \\xB7 \" + (candidate.name || candidate.number || candidate.room_id)));\n"
    "    var change = el(\"button\", \"\\u91CD\\u65B0\\u67E5\\u8BE2\", \"btn-s\");\n"
    "    change.type = \"button\";\n"
    "    change.onclick = function() {\n"
    "      discover(true, true);\n"
    "    };\n"
    "    box.append(change);\n"
    "  }\n"
)

new_block = (
    "  function renderChoice(candidate, original) {\n"
    "    var box = host();\n"
    "    if (!box) return;\n"
    "    box.replaceChildren();\n"
    "    // 当前服务器信息卡：封面/头像/名称/房主/人数与版本徽标齐全，\n"
    "    // 重新查询仅列出候选（listOnly），绝不自动连接。\n"
    "    var card = el(\"article\", null, \"yy-room-card yy-room-card-selected\");\n"
    "    card.setAttribute(\"data-id\", candidate.id || \"\");\n"
    "    var verBadge = el(\"span\", candidate.version || \"\", \"yy-room-ver\");\n"
    "    if (candidate.version) card.append(verBadge);\n"
    "    var cntBadge = el(\"span\", \"\", \"yy-room-badge\");\n"
    "    if (candidate.players != null) cntBadge.textContent = \"\\u5728\\u7EBF \" + candidate.players + (candidate.max_players != null ? \"/\" + candidate.max_players : \"\");\n"
    "    else if (candidate.max_players != null) cntBadge.textContent = \"\\u5BB9\\u91CF \" + candidate.max_players;\n"
    "    if (cntBadge.textContent) card.append(cntBadge);\n"
    "    var cover = image(candidate.cover_url, \"yy-room-cover\");\n"
    "    if (cover) card.append(cover);\n"
    "    var content = el(\"div\", null, \"yy-room-content\");\n"
    "    var avatar = image(candidate.avatar_url, \"yy-room-avatar\");\n"
    "    if (avatar) content.append(avatar);\n"
    "    content.append(el(\"strong\", candidate.name || candidate.number || candidate.room_id));\n"
    "    var details = [labels[candidate.type], candidate.number ? \"\\u53F7\\u7801 \" + candidate.number : \"ID \" + candidate.room_id];\n"
    "    if (candidate.owner) details.push(\"\\u623F\\u4E3B \" + candidate.owner);\n"
    "    else if (candidate.owner_id) details.push(\"\\u623F\\u4E3B UID \" + candidate.owner_id);\n"
    "    if (candidate.version) details.push(candidate.version);\n"
    "    content.append(el(\"p\", details.join(\" \\xB7 \")));\n"
    "    var change = el(\"button\", \"\\u91CD\\u65B0\\u67E5\\u8BE2\\uFF08\\u4EC5\\u67E5\\u770B\\u5019\\u9009\\uFF0C\\u4E0D\\u4F1A\\u81EA\\u52A8\\u8FDE\\u63A5\\uFF09\", \"btn-s\");\n"
    "    change.type = \"button\";\n"
    "    change.onclick = function() {\n"
    "      discover(true, true);\n"
    "    };\n"
    "    content.append(change);\n"
    "    card.append(content);\n"
    "    box.append(card);\n"
    "  }\n"
)

if src.count(old_block) != 1:
    print("ANCHOR-FAIL: renderChoice block not found verbatim"); sys.exit(1)
src = src.replace(old_block, new_block)
assert src != orig
io.open(PATH, "w", encoding="utf-8", newline="\n").write(src)
print("renderChoice upgraded OK")
