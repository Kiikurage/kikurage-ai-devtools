# kikurage-ai-devtools

kikurage の Claude Code 用スキル・フック・作業方針をまとめたプラグイン。

## インストール

```
/plugin marketplace add Kiikurage/kikurage-ai-devtools
/plugin install kikurage-ai-devtools@kikurage-ai-devtools
```

ローカルで試すときは `claude --plugin-dir <このリポジトリ>`。

## 中身

| 種類 | 場所 | 内容 |
|---|---|---|
| スキル | `skills/zoom-img` | 画像の一部を目盛り付きで拡大し、点の RGB を読む |
| スキル | `skills/compare-imgs` | 複数の画像を番号・ラベル付きで 1 枚に並べる |
| フック（SessionStart） | `hooks/session_context.py` | `context/*.md` をコンテキストに注入する（CLAUDE.md の代わり） |
| フック（PostToolUse: Read） | `hooks/image_read_notice.py` | 画像を読んだら、細部は zoom-img で確かめるよう伝える |
| フック（PostToolUse / Stop） | `hooks/japanese_reply_guard.py` | 返答が英語なら日本語で書き直させる |
| 作業方針 | `context/guidelines.md` | 進め方、報告は確かめたことだけを書く、ファイルの見せ方 |

作業方針を足すときは、`context/` に Markdown を置く（ファイル名順に連結して注入される）。

## 開発の作法

このマシンには、ローカルのリポジトリをマーケットプレイスとして登録して入れてある。

```sh
claude plugin marketplace add /Users/kikurage/workspace/kikurage-ai-devtools
claude plugin install kikurage-ai-devtools@kikurage-ai-devtools --scope user
```

インストールしたプラグインは `~/.claude/plugins/cache/kikurage-ai-devtools/kikurage-ai-devtools/<version>/` にコピーされたものが使われる。
リポジトリを編集しただけでは反映されないので、試すたびに次の手順を踏む。

1. `.claude-plugin/plugin.json` の `version` を上げる（下のバージョンの付け方に従う）
2. 反映する

   ```sh
   claude plugin validate .
   claude plugin marketplace update kikurage-ai-devtools
   claude plugin update kikurage-ai-devtools@kikurage-ai-devtools
   ```

3. Claude Code を起動し直す（起動中のセッションには反映されない）

インストールし直さずに一度だけ試すなら、`claude --plugin-dir /Users/kikurage/workspace/kikurage-ai-devtools` でも読み込める。

### バイナリを作り直す

`skills/*/src`（Go）を直したら、`scripts/build.sh` で全アーキテクチャ分をビルドする。
`skills/*/bin/<名前>-<os>-<arch>` が実体で、`bin/<名前>` は OS とアーキテクチャを見て実体を呼ぶラッパー（`scripts/dispatch.sh` のコピー）。

compare-imgs は、ヒラギノが無い環境ではラベルに BIZ UDPゴシック Bold（SIL OFL 1.1、`skills/compare-imgs/src/fonts/`）を使う。バイナリに埋め込んである。

### バージョンの付け方

形式は `x.y.z` だが、semver の意味づけには従わない。

- マージした版（GitHub に push する版）は常に `x.0.0`
- 開発中は、ローカルで試すたびにマイナーを上げる（`x.1.0`、`x.2.0`、…）
- 次にマージするときは、メジャーを上げて `(x+1).0.0` にする
- パッチ（`z`）は使わない

## 制限

- 同梱のバイナリは macOS arm64・Linux amd64・Linux arm64 の 3 つ。ほかの環境では動かない
- フックは `python3` を使う
