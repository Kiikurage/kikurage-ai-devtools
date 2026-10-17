#!/usr/bin/env python3
"""SessionStart で context/ 以下の Markdown をコンテキストに注入するフック。

プラグインは CLAUDE.md を配れないので、その代わりに使う。
context/*.md をファイル名順に連結し、additionalContext として返す。
"""

import json
import pathlib

CONTEXT_DIR = pathlib.Path(__file__).resolve().parent.parent / "context"


def main() -> None:
    texts = [p.read_text(encoding="utf-8").strip() for p in sorted(CONTEXT_DIR.glob("*.md"))]
    if not texts:
        return
    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "SessionStart",
            "additionalContext": "\n\n".join(texts),
        }
    }, ensure_ascii=False))


if __name__ == "__main__":
    main()
