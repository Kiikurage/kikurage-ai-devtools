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
| スキル | `skills/js-stack` | JS/TS で使うツールの決まり（bun・bun:test・Vite・TypeScript） |
| コマンド | `bin/zoom_img`、`bin/compare_imgs` | 上の 2 つのスキルが使うコマンド。プラグインが有効な間は Bash ツールの PATH に入る |
| フック（SessionStart） | `hooks/session_context.py` | `context/*.md` をコンテキストに注入する（CLAUDE.md の代わり） |
| フック（PostToolUse: Read） | `hooks/image_read_notice.py` | 画像を読んだら、細部は zoom-img で確かめるよう伝える |
| フック（PostToolUse / Stop） | `hooks/japanese_reply_guard.py` | 返答が英語なら日本語で書き直させる |
| 作業方針 | `context/conventions.md` | ブランチ名とコード品質の規約、AGENTS.md への反映の提案 |
| 作業方針 | `context/guidelines.md` | 進め方、報告は確かめたことだけを書く、ファイルの見せ方 |

作業方針を足すときは、`context/` に Markdown を置く（ファイル名順に連結して注入される）。

## 開発の作法

このマシンには、ローカルのリポジトリをマーケットプレイスとして登録して入れてある。

```sh
claude plugin marketplace add /Users/kikurage/workspace/kikurage-ai-devtools
claude plugin install kikurage-ai-devtools@kikurage-ai-devtools --scope user
```

ローカルのパスで登録したマーケットプレイスのプラグインは、キャッシュにコピーされず、このリポジトリから直接読み込まれる。
リポジトリを編集したら、Claude Code を起動し直すか、起動中のセッションで `/reload-plugins` を実行すれば反映される。`version` を上げる必要はない。

インストールせずに一度だけ試すなら、`claude --plugin-dir /Users/kikurage/workspace/kikurage-ai-devtools` でも読み込める。

### バイナリを作り直す

`skills/*/src`（Go）を直したら、`scripts/build.sh` で全アーキテクチャ分をビルドする。
`libexec/<名前>-<os>-<arch>` が実体で、`bin/<名前>` は OS とアーキテクチャを見て実体を呼ぶラッパー（`scripts/dispatch.sh` のコピー）。
実体を `bin/` に置かないのは、アーキテクチャ名の付いたコマンドまで PATH に出さないため。

compare-imgs は、ヒラギノが無い環境ではラベルに BIZ UDPゴシック Bold（SIL OFL 1.1、`skills/compare-imgs/src/fonts/`）を使う。バイナリに埋め込んである。

### バージョンの付け方

形式は `x.y.z` だが、semver の意味づけには従わない。

- `main` にマージする版は常に `x.0.0`。マージするたびにメジャーを上げて `(x+1).0.0` にする
- フィーチャーブランチで GitHub 経由のインストールを試すときは、マイナーを上げる（`x.1.0`、`x.2.0`、…）。GitHub から入れたプラグインは `version` ごとにキャッシュされるので、上げないと更新されない
- ローカルで試すだけなら上げなくてよい
- パッチ（`z`）は使わない

## 制限

- 同梱のバイナリは macOS arm64・Linux amd64・Linux arm64 の 3 つ。ほかの環境では動かない
- フックは `python3` を使う
- ドキュメントによると、claude.ai と Cowork は `bin/` を持つプラグインをインストールしない
