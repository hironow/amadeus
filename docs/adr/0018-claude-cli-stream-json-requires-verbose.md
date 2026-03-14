# 0018. Claude CLI stream-json Requires --verbose

**Date:** 2026-03-13
**Status:** Accepted

## Context

Doctor の `claude-inference` チェックが `exit status 1` で常に失敗していた。エラーヒントは API key/quota/model access や CLAUDECODE env var リークを示唆していたが、実際の原因はそのどちらでもなかった。

手動実行で判明したエラー:

```
Error: When using --print, --output-format=stream-json requires --verbose
```

Claude CLI 2.1.x で `--print --output-format stream-json` の組み合わせに `--verbose` フラグが必須になった。doctor の inference チェックでこのフラグが欠落していた。

## Decision

Claude CLI を `--output-format stream-json` で呼び出す全ての箇所に `--verbose` を付与する。再発防止として semgrep ルール (s0032-stream-json-without-verbose) を `.semgrep/shared-adr.yaml` に追加し、3つのコードパターンを網羅する:

1. 関数呼び出し引数 (`newShellCmd(ctx, name, ..., "--output-format", "stream-json", ...)`)
2. `append` パターン (`args = append(args, "--output-format", "stream-json")`)
3. `[]string` リテラル (`[]string{..., "--output-format", "stream-json", ...}`)

## Consequences

### Positive

- Doctor の `claude-inference` チェックが正常動作するようになった
- `context-budget` チェックも inference 成功に依存するため連鎖的に復活
- semgrep ルールにより同じ問題の再発を静的に防止

### Negative

- Claude CLI のフラグ仕様変更に対する脆弱性が露呈した（CLI のバージョンアップで壊れうる）

### Neutral

- `--verbose` は stream-json 出力に init メッセージ等のメタデータを含めるためのフラグであり、出力量が若干増える
