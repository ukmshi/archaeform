# Archaeform VPC Import 詳細設計書 - F-06 自動 import 実行（オプション）

## 1. 概要

- **対象機能**: F-06 自動 import 実行（`--apply` オプション）  
- **目的**: 生成済みの import スクリプト（F-05）を利用して `terraform import` を自動実行し、既存リソースを Terraform state に取り込む。  
- **タスク優先度**: 6（任意機能だが実運用では有用）

## 2. 対象コンポーネント

- `usecase.ImportVpcResources`
  - `--apply` 指定時に Terraform 実行をオーケストレーション。
- 実行ユーティリティ:
  - `pkg/terraform/cli.Executor`（仮）

## 3. Terraform CLI 実行フロー

1. `tfDir` をワーキングディレクトリとして `terraform init` を実行（必要に応じて）。
2. `import.sh` をサブプロセスとして実行、もしくは直接 `terraform import` コマンドを 1 つずつ実行。
3. 各コマンドの標準出力／標準エラーをリアルタイムでログに転送。
4. 失敗したリソースを記録し、継続／中断ポリシーに従って次のコマンド実行可否を判定。

## 4. CLI 実行インターフェース

```go
type TerraformExecutor interface {
    Init(tfDir string) error
    Import(tfDir string, address string, id string) error
}
```

`usecase.ImportVpcResources` 内では、F-05 で import ID を決定済みの `Resource` 一覧を利用して `Import` を順次呼び出す設計も可能。

## 5. 継続／中断ポリシー

- 初期実装:
  - 1 リソースの import 失敗は記録しつつ、他リソースの import は **継続**。
  - 致命的エラー（`terraform` バイナリ未検出、`terraform init` 失敗など）は即時中断。
- 将来拡張:
  - `--on-error=continue|abort` のようなフラグで制御できるようにする余地を残す。

## 6. ログ・サマリ連携

- 成功・失敗・スキップ件数をカウントし、`ImportSummary` に反映。
- 失敗したリソースについては Terraform のエラーメッセージの先頭行を要約として保持し、F-07 のサマリ出力で表示。

## 7. テスト観点

- 小さなサンプル構成で実際に `terraform import` を実行し、state にリソースが登録されることを確認。
- `terraform` バイナリが見つからない場合や、`tfDir` が不正な場合に適切なエラーコード・メッセージが返ることを確認。


