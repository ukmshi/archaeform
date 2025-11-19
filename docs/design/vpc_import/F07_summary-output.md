# Archaeform VPC Import 詳細設計書 - F-07 サマリ出力

## 1. 概要

- **対象機能**: F-07 サマリ出力  
- **目的**: import 処理全体の結果（対象リソース数、生成ファイル数、import コマンド数、競合・スキップ・エラーなど）をユーザーにわかりやすく提示する。  
- **タスク優先度**: 7

## 2. 対象コンポーネント

- `usecase.ImportVpcResources`
  - `ImportSummary` 構築と出力。

## 3. データ構造

```go
type ImportSummary struct {
    TotalResources          int
    ImportableResources     int
    ConflictedResources     int
    GeneratedHclFiles       int
    GeneratedImportCommands int

    ApplyRequested bool
    ApplySucceeded int
    ApplyFailed    int

    Warnings []string
    Errors   []string
}
```

## 4. 出力フォーマット

標準出力への例:

```text
=== Archaeform VPC Import Summary ===
Target VPC          : vpc-0123456789abcdef0 (ap-northeast-1)
Discovered resources: 120
Importable          : 110
Conflicted          : 10

Generated HCL files : 5 (under ./infra/generated)
Import commands     : 110 (import.sh)

Apply               : disabled

Warnings:
  - RDS リソース取得に失敗したため import 対象外になりました
  - aws_instance.web_1 は既存構成と競合するためスキップしました
```

## 5. 処理フロー

1. 各フェーズ（F-01〜F-06）で必要なメトリクスを `ImportSummary` に加算・設定。
2. `usecase.ImportVpcResources` の最後で `ImportSummary` を標準出力へ整形して表示。
3. 終了コード:
   - 正常終了（警告あり含む）: 0
   - 一部 import 失敗・競合あり: 2

## 6. テスト観点

- 入力メトリクスの組み合わせごとに、期待されるサマリテキストと終了コードが出力されるか確認。


