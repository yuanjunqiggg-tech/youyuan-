# -*- coding: utf-8 -*-
"""从 需求逐项总清单.json 生成 未解决清单.md 的全量表格部分。

只读总账，不改总账。输出写到 project/未解决清单.md。
手写的分析段落放在 HEAD 常量里，表格由脚本生成，保证与总账逐字一致。
"""
import json
import os
import re
from collections import Counter, OrderedDict

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LEDGER = os.path.join(ROOT, "需求逐项总清单.json")
OUT = os.path.join(ROOT, "未解决清单.md")

CLOSED = {"已完成（验收归档）", "已撤回／不执行"}


def clean(text, limit):
    text = re.sub(r"\s+", " ", str(text or "")).strip()
    if len(text) > limit:
        text = text[: limit - 1] + "…"
    return text.replace("|", "／")


def main():
    data = json.load(open(LEDGER, encoding="utf-8"))
    rows = data["requirements"]
    open_rows = [r for r in rows if r.get("status") not in CLOSED]

    counts = Counter(r.get("status") for r in rows)
    p0 = [r for r in open_rows if r.get("priority") == "P0"]
    p1 = [r for r in open_rows if r.get("priority") == "P1"]

    modules = OrderedDict()
    for r in open_rows:
        modules.setdefault(r.get("module") or "未分类", []).append(r)

    head = open(os.path.join(ROOT, "maintenance", "_unresolved_head.md"), encoding="utf-8").read()

    lines = [head]
    lines.append("## 附：全量未关闭条目（由总账生成，勿手改）\n")
    lines.append(
        "> 本表由 `maintenance/gen_unresolved.py` 从 `需求逐项总清单.json` 生成，"
        "只列 `status` 不属于「已完成（验收归档）／已撤回／不执行」的条目。\n"
        "> 总账口径：%d 项；未关闭 **%d** 项（未完成 %d／部分完成待验收 %d）；"
        "P0 **%d** 项，P1 **%d** 项。\n"
        % (
            len(rows),
            len(open_rows),
            counts.get("未完成", 0),
            counts.get("部分完成／待验收", 0),
            len(p0),
            len(p1),
        )
    )

    for module, items in modules.items():
        lines.append("\n### %s（%d 项）\n" % (module, len(items)))
        lines.append("| 编号 | 优先级 | 要求 | 状态 | 剩余条件 |")
        lines.append("|---|---|---|---|---|")
        for r in sorted(items, key=lambda x: (x.get("priority") != "P0", x.get("id"))):
            lines.append(
                "| %s | %s | %s | %s | %s |"
                % (
                    r.get("id"),
                    r.get("priority"),
                    clean(r.get("request"), 90),
                    clean(r.get("status"), 20),
                    clean(r.get("remaining"), 130),
                )
            )

    open(OUT, "w", encoding="utf-8", newline="\n").write("\n".join(lines) + "\n")
    print("wrote", OUT, "rows", len(open_rows))


if __name__ == "__main__":
    main()
