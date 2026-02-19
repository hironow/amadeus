# Linear Integration Design: D-Mail → Linear Issue via Claude Code MCP

**Goal:** Amadeus D-Mails を Linear issue として既存ワークフローに統合する。

**Approach:** Go コードは Linear を直接知らない。Claude Code の Linear MCP ツール経由で issue 作成する。

---

## 1. D-Mail 構造体の拡張

`DMail` struct に `LinearIssueID` フィールドを追加。

```go
LinearIssueID *string `json:"linear_issue_id,omitempty"`
```

- `nil` = 未同期
- 値あり = Linear issue 作成済み
- `omitempty` で既存ファイルとの後方互換を維持

---

## 2. `amadeus sync` コマンド

未同期 D-Mail を JSON で stdout に出力する。

```
$ amadeus sync
{"unsynced":[{"id":"d-043","severity":"HIGH","summary":"ADR-003 violation",...}]}
```

- `LinearIssueID` が `nil` の D-Mail を抽出
- Claude Code がこの出力を読んで Linear MCP で issue 作成

---

## 3. Linear Issue マッピング

| D-Mail フィールド | Linear issue フィールド |
|---|---|
| `summary` | title |
| `detail` + メタ情報 | description（Markdown） |
| `severity` HIGH/MED/LOW | priority 1(Urgent)/3(Normal)/4(Low) |
| — | label: `D-Mail` |
| チーム | MINE（固定） |
| プロジェクト | Amadeus |

- ラベルは `D-Mail` の 1 つのみ
- description に D-Mail ID、チェック日時、target 情報を含める

---

## 4. `amadeus link` コマンド

Linear issue 作成後に D-Mail と紐付ける。

```
amadeus link <dmail-id> <linear-issue-id>
```

- `linear_issue_id` フィールドを更新して保存
- 既にリンク済みならエラー（上書き防止）

---

## 5. ワークフロー

```
amadeus check          → D-Mail 生成（ローカル）
amadeus sync           → 未同期 D-Mail を JSON 出力
Claude Code            → Linear MCP で issue 作成
amadeus link d-043 MY-250  → 紐付け保存
```

Go コードは Linear API に一切依存しない。
