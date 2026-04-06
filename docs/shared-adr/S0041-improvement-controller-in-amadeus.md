# S0041. Improvement Controller Remains in Amadeus

**Date:** 2026-04-06
**Status:** Accepted

## Context

Track D3/F の設計判断: improvement controller (Weave feedback の取り込み、normalized signal の保存、corrective policy の生成) をどこに置くか。

選択肢:
1. **amadeus 内に留める** (現状維持)
2. **phonewave に寄せる** (courier と routing を統合)
3. **新しい別コンポーネントとして分離**

## Decision

**amadeus 内に留める。**

### 理由

1. amadeus が唯一の corrective routing authority (`DetermineCorrectionDecision`)
2. 既に `improvement_collector.go` (488行) + `improvement_signal_store.go` (453行) + `improvement_weave_source.go` (233行) が amadeus session に実装済み
3. 分離すると amadeus の SQLite (`improvement-ingestion.db`) へのアクセスがプロセス間通信に変わり、WAL cooperative model の複雑さが増す
4. 将来的に分離が必要になった場合、既存の port interface (`ImprovementFeedbackSource`, `SQLiteImprovementCollectorStore`) 経由で抽出可能

### phonewave に寄せない理由

phonewave は courier (transport) に特化すべき。routing decision と improvement signal の管理は scorer/verifier の責務であり、amadeus の domain。

### 別コンポーネントにしない理由

現時点では improvement controller の input/output が amadeus の check cycle に密結合しており、分離のメリットがコストに見合わない。MVP の段階では amadeus 内で十分。

## Consequences

### Positive
- 実装変更なし (既存コードが正)
- SQLite アクセスがプロセス内で完結
- corrective routing と improvement signal が同一 aggregate 内で一貫

### Negative
- amadeus の責務が大きくなる (scorer + verifier + improvement controller)
- 将来的にスケーラビリティの問題が出る可能性 (signal volume が増えた場合)

### Neutral
- Track F の F2 (ports) / F3 (policy store) は amadeus 内で段階的に実装可能
- 分離判断は signal volume が問題になった時点で再検討
