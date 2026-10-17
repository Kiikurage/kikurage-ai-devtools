---
name: js-stack
description: JavaScript / TypeScript のプロジェクトで使うツールの決まり（ランタイムとパッケージ管理は bun、テストは bun:test、ビルドと開発サーバーは Vite、言語は TypeScript）。JS/TS のプロジェクトを新しく作る、パッケージを入れる、スクリプト・テスト・開発サーバーを実行する、テストを書く、ビルドの設定をするときに、npm・npx・node・jest・webpack などを使う前に読む。
---

# JS/TS のツールの決まり

JS/TS の作業では、次のものを使う。既存のプロジェクトが別のツールで組まれているときは、勝手に移行せず、そのプロジェクトのやり方に従う。移行したほうがよいと思ったら提案する。

| 用途 | 使う | 使わない |
|---|---|---|
| ランタイム | `bun` | `node` |
| パッケージ管理 | `bun install` / `bun add` / `bun remove` | `npm` / `yarn` / `pnpm` |
| パッケージのコマンドを一度だけ実行 | `bunx` | `npx` |
| スクリプトの実行 | `bun run <script>`、`bun <file>.ts` | `npm run`、`node`、`ts-node`、`tsx` |
| テスト | `bun test`（`bun:test`） | jest、vitest、mocha |
| ビルド・開発サーバー（ブラウザ向け） | Vite | webpack、Create React App、Parcel |
| 言語 | TypeScript | JavaScript |

## bun

- ロックファイルは `bun.lock`。`package-lock.json` などは作らない
- 依存の追加は `bun add <pkg>`、開発用は `bun add -d <pkg>`
- `.ts` / `.tsx` は、トランスパイルの準備なしに `bun <file>.ts` でそのまま実行できる
- 型の検査は bun ではされない。`bunx tsc --noEmit` で別に行う

## テスト（bun:test）

```ts
import { describe, expect, test } from "bun:test";

describe("sum", () => {
  test("足し算", () => {
    expect(sum(1, 2)).toBe(3);
  });
});
```

- 実行は `bun test`。`*.test.ts` や `*.spec.ts` などが自動で見つかる
- jest に近い API（`describe` / `test` / `expect` / `mock` / `beforeEach` など）が `bun:test` から使える。jest の型定義やパッケージは入れない
- `bun test` は Vite の設定を読まない。Vite のプラグインに頼った変換が要るコードは、テストでは別に扱う必要がある

## Vite

- 新しく作るときは `bun create vite <名前>` で、テンプレートは TypeScript のもの（`react-ts`、`vanilla-ts` など）を選ぶ
- 開発サーバーは `bun run dev`、ビルドは `bun run build`
- 設定ファイルは `vite.config.ts`

## TypeScript

- 新しいファイルは `.ts` / `.tsx` で書く
- `tsconfig.json` は `strict: true` にする
