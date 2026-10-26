#!/usr/bin/env python3
"""给 youyuan_game_auth.js 的皮肤预览失败分支补可见占位。

背景：skin_preview 失败时原来只改 status 文案就 return，舞台
#yy-ga-skin-stage 是一块 340px 高的空 div —— 用户看到的就是「皮肤不显示」。
这里在失败时往舞台里放一段可见说明，并告诉他下一步能干什么。

按行号定点替换，保持文件既有的 \\uXXXX 中文转义风格。
"""
import io
import shutil
import sys

P = "upstream/frontend/js/youyuan_game_auth.js"


def js_esc(s: str) -> str:
    """把中文转成 \\uXXXX，保证 JS 字符串里只有 ASCII。"""
    out = []
    for ch in s:
        if ord(ch) < 128:
            out.append(ch)
        else:
            out.append("\\u%04X" % ord(ch))
    return "".join(out)


def main() -> int:
    shutil.copy2(P, P + ".bak-skinfail")
    src = io.open(P, encoding="utf-8").read()
    lines = src.split("\n")

    # ── 定位校验：改错文件或行号漂移就停 ──
    assert lines[182].strip().startswith(
        'const result = await yyGameAuthAPI("POST", { action: "skin_preview"'
    ), "第 183 行不是 skin_preview 调用：%r" % lines[182]
    assert lines[184].strip() == "if (!result.ok) {", "第 185 行不是失败分支：%r" % lines[184]
    assert lines[186].strip() == "return;", "第 187 行不是 return：%r" % lines[186]
    assert lines[187].strip() == "}", "第 188 行不是右花括号：%r" % lines[187]

    title = js_esc("暂时读不到这个账号的皮肤")
    hint = js_esc("可以在下方直接填入「已拥有的皮肤组件码」，再点「应用皮肤」")

    new = [
        '  if (!result.ok) {',
        '    status.textContent = result.error || "\\u672A\\u80FD\\u8BFB\\u53D6\\u76AE\\u80A4\\uFF0C\\u4ECD\\u53EF\\u586B\\u5199\\u5DF2\\u6709\\u7EC4\\u4EF6\\u7801";',
        '    // 失败时舞台会留一块 340px 的空白，用户看到的就是「皮肤不显示」。',
        '    // 这里放可见占位，并说清下一步能做什么。',
        "    const why = result.error ? '<div style=\"font-size:13px;line-height:1.7;opacity:.8\">' + yyEscape(result.error) + '</div>' : '';",
        "    stage.innerHTML =",
        "      '<div style=\"height:100%;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:10px;padding:24px;text-align:center\">' +",
        "      '<div style=\"font-size:15px;font-weight:600\">' + title + '</div>' +",
        "      why +",
        "      '<div style=\"font-size:13px;line-height:1.7;opacity:.65\">' + hint + '</div>' +",
        "      '</div>';",
        '    return;',
        '  }',
    ]

    # title/hint 定义插在 const result 那一行之后
    defs = [
        '  const title = "%s";' % title,
        '  const hint = "%s";' % hint,
    ]

    out = lines[:183] + defs + lines[183:184] + new + lines[188:]
    io.open(P, "w", encoding="utf-8", newline="\n").write("\n".join(out))
    print("替换完成：原 184-188 行 -> %d 行" % (len(defs) + 1 + len(new)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
