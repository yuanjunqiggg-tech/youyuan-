with open('/root/prism-toolbox/bot-apk/frontend/js/features/others.js', 'r') as f:
    content = f.read()

# Remove the pathfinding control section (uses spaces, not tabs)
old = """
    // 寻路控制
    '<label>目标 X</label>' +
    '<div class="row" style="gap:6px">' +
      '<input id="dbg-goto-x" type="number" step="0.1" placeholder="X" style="flex:1">' +
      '<input id="dbg-goto-y" type="number" step="0.1" placeholder="Y" style="flex:1">' +
      '<input id="dbg-goto-z" type="number" step="0.1" placeholder="Z" style="flex:1">' +
    '</div>' +
    '<div class="row" style="margin-top:6px;gap:6px">' +
      '<button class="btn" onclick="dbgGoto()" style="flex:1;margin:0;font-size:12px">前往坐标</button>' +
      '<button class="btn" onclick="dbgFollow()" style="flex:1;margin:0;font-size:12px">跟随玩家</button>' +
      '<button class="btn-d" onclick="dbgStop()" style="flex:1;margin:0;font-size:12px">停止</button>' +
    '</div>' +

    // 跟随玩家输入
    '<div id="dbg-follow-input" style="display:none;margin-top:8px">' +
      '<label>玩家名称</label>' +
      '<div class="row" style="gap:6px">' +
        '<input id="dbg-follow-name" type="text" placeholder="输入玩家名" style="flex:1">' +
        '<button class="btn-s" onclick="dbgStartFollow()" style="margin:0;font-size:12px">确认跟随</button>' +
      '</div>' +
    '</div>' +

    // 调试日志"""

new = """
    // 调试日志"""

if old in content:
    content = content.replace(old, new)
    with open('/root/prism-toolbox/bot-apk/frontend/js/features/others.js', 'w') as f:
        f.write(content)
    print('OK')
else:
    print('NOT FOUND')