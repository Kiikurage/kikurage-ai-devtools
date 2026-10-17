#!/usr/bin/env python3
"""Claude の返答が英語だったら「日本語で返答して」と伝えるフック。

1 つのスクリプトで 2 つのイベントを扱う。
    PostToolUse（ターン途中）
        ツール実行のたびにバックグラウンドで動き、直前の assistant の発言を判定する。
        async + asyncRewake で登録するので、エージェントの実行は止めない。
        英語でなければ何も出力しない（コンテキストを消費しない）。
        英語なら exit 2 + stderr で Claude を起こし、システムリマインダーとして注意を伝える。
    Stop（ターン終了時）
        最後の返答が英語なら差し戻して日本語で書き直させる。

判定方法
    コードブロック・インラインコード・URL・ファイルパス・引用行・表の行を除いてから、
    日本語の文字（ひらがな・カタカナ・漢字）と英単語の数を数える。
    「日本語がほぼ無く、英単語が一定数以上ある」ときだけ英語と判定する。

ループ・重複防止
    - 注意は 1 ユーザーターンにつき PostToolUse・Stop それぞれ最大 1 回。
      状態は STATE_DIR にセッションごとに保存する。
      意図的に英語を出す場合は、1 回注意されたあとはそのまま英語で続けられる。
    - Stop は stop_hook_active が true（このフックの差し戻しで続行した）なら何もしない。
    - 直近のユーザー発言に日本語が含まれないとき（英語で質問された等）は何もしない。

セットアップ
    このプラグインの hooks/hooks.json で PostToolUse（async + asyncRewake）と Stop に登録している。
"""

import json
import os
import re
import sys
import tempfile

# 英語と判定する閾値
MIN_ENGLISH_WORDS = 8  # 英単語がこれ未満なら短すぎるので判定しない
MAX_JAPANESE_RATIO = 0.05  # 日本語文字 / (日本語文字 + 英字) がこれ以下なら英語扱い

STATE_DIR = os.path.join(tempfile.gettempdir(), "claude-japanese-reply-guard")

JAPANESE_RE = re.compile(r"[぀-ヿ㐀-䶿一-鿿ｦ-ﾟ]")
ENGLISH_WORD_RE = re.compile(r"\b[A-Za-z]{2,}\b")

REASON = (
    "直前の返答が英語でした。日本語で返答し直してください。"
    "ただしユーザーが英語を求めている場合や、英文そのものが成果物である場合は"
    "そのままで構いません（このフックは再度止めません）。"
)

MID_TURN_NOTICE = (
    "直前の発言が英語でした。以降のユーザー向けの発言は日本語で書いてください。"
    "意図的に英語を出している場合はこの注意を無視して構いません（このターンでは再度注意しません）。"
)

# 注意文の末尾に付ける。この注意はユーザーには見えていない
HIDDEN_NOTE = (
    "この注意はユーザーには表示されていません。返答でこの注意やフックの存在に触れず、"
    "自分で気づいたものとして、そのまま日本語で続けてください（注意を受けたことへの返答だけのメッセージも書かない）。"
)
REASON += HIDDEN_NOTE
MID_TURN_NOTICE += HIDDEN_NOTE


def strip_non_prose(text: str) -> str:
    """コードや識別子など、自然言語でない部分を取り除く。"""
    text = re.sub(r"```.*?(```|$)", " ", text, flags=re.S)  # フェンスドコード
    text = re.sub(r"~~~.*?(~~~|$)", " ", text, flags=re.S)
    text = re.sub(r"`[^`\n]*`", " ", text)  # インラインコード
    text = re.sub(r"https?://\S+", " ", text)  # URL
    text = re.sub(r"(?:~|\.{0,2})?/[\w.\-/]+", " ", text)  # パス
    text = re.sub(r"\b[\w.\-]+\.[A-Za-z0-9]{1,5}(:\d+)?\b", " ", text)  # file.py:12
    lines = []
    for line in text.splitlines():
        s = line.strip()
        if s.startswith(">") or s.startswith("|"):  # 引用・表
            continue
        if line.startswith("    ") or line.startswith("\t"):  # インデントコード
            continue
        lines.append(line)
    return "\n".join(lines)


def is_english(text: str) -> bool:
    prose = strip_non_prose(text)
    ja = len(JAPANESE_RE.findall(prose))
    words = ENGLISH_WORD_RE.findall(prose)
    if len(words) < MIN_ENGLISH_WORDS:
        return False
    en_chars = sum(len(w) for w in words)
    return ja / (ja + en_chars) <= MAX_JAPANESE_RATIO


def text_of(content) -> str:
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        return "\n".join(
            b.get("text", "") for b in content if isinstance(b, dict) and b.get("type") == "text"
        )
    return ""


def is_real_user_prompt(entry: dict) -> bool:
    """tool_result ではない、人間の発言か。"""
    if entry.get("type") != "user" or entry.get("isMeta"):
        return False
    content = entry.get("message", {}).get("content")
    if isinstance(content, list):
        return any(isinstance(b, dict) and b.get("type") == "text" for b in content)
    return isinstance(content, str)


def load_last_turn(transcript_path: str):
    """(最後のユーザー発言, その uuid, 最後の assistant テキスト) を返す。"""
    entries = []
    with open(transcript_path, encoding="utf-8") as f:
        for line in f:
            try:
                entries.append(json.loads(line))
            except json.JSONDecodeError:
                pass

    last_user, user_id, last_assistant = "", "", ""
    for entry in reversed(entries):
        if entry.get("type") == "assistant" and not last_assistant:
            t = text_of(entry.get("message", {}).get("content"))
            if t.strip():
                last_assistant = t
        elif is_real_user_prompt(entry):
            last_user = text_of(entry.get("message", {}).get("content"))
            user_id = entry.get("uuid", "")
            break
    return last_user, user_id, last_assistant


def already_warned(session_id: str, event: str, user_id: str) -> bool:
    """このユーザーターンで既に注意したか。未注意なら記録して False を返す。"""
    os.makedirs(STATE_DIR, exist_ok=True)
    path = os.path.join(STATE_DIR, f"{session_id or 'unknown'}.json")
    try:
        with open(path, encoding="utf-8") as f:
            state = json.load(f)
    except (OSError, json.JSONDecodeError):
        state = {}
    if state.get(event) == user_id:
        return True
    state[event] = user_id
    with open(path, "w", encoding="utf-8") as f:
        json.dump(state, f)
    return False


def main() -> int:
    try:
        data = json.load(sys.stdin)
    except json.JSONDecodeError:
        return 0

    event = data.get("hook_event_name", "Stop")
    if event == "Stop" and data.get("stop_hook_active"):
        return 0

    path = data.get("transcript_path")
    if not path:
        return 0
    try:
        user, user_id, assistant = load_last_turn(path)
    except OSError:
        return 0

    if not assistant:
        return 0
    if user and not JAPANESE_RE.search(strip_non_prose(user)):
        return 0  # ユーザーが英語で話しかけている
    if not is_english(assistant):
        return 0
    if already_warned(data.get("session_id", ""), event, user_id):
        return 0

    if event == "Stop":
        print(json.dumps({"decision": "block", "reason": REASON}, ensure_ascii=False))
        return 0
    # PostToolUse（asyncRewake）: exit 2 で Claude を起こし、stderr をリマインダーとして渡す
    print(MID_TURN_NOTICE, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main())
