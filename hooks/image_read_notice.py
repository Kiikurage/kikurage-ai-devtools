#!/usr/bin/env python3
"""画像を Read したあと、細部は zoom-img で拡大して確かめるよう伝えるフック（PostToolUse, matcher Read）。"""

import json
import sys

IMAGE_EXTS = (".png", ".jpg", ".jpeg", ".webp", ".gif")

NOTICE = (
    "画像を読んだ。いま見えているのは縮小された全体図で、細部は潰れている。"
    "線・縁・色・塗り・配置・重なり・大きさの比など細部について判断したり言ったりする前に、"
    "zoom-img スキル（zoom_img）でその部分を目盛り付きで拡大して見る。色や太さは --px で画素を読んで数値で確かめる。"
    "参考画像を真似るときは、要素ごとに拡大して観察した結果を書いてから作り、"
    "出来たものは参考の同じ部分と同じ倍率で並べて比べてから報告する。"
    "指摘を受けたら、合わせて直す前にその点を画像で確かめる。"
)


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except json.JSONDecodeError:
        return
    path = str(data.get("tool_input", {}).get("file_path", ""))
    if not path.lower().endswith(IMAGE_EXTS):
        return
    print(json.dumps({
        "hookSpecificOutput": {"hookEventName": "PostToolUse", "additionalContext": NOTICE}
    }, ensure_ascii=False))


if __name__ == "__main__":
    main()
