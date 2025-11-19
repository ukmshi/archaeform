# Archaeform VPC Import 詳細設計書 - F-05 `terraform import` コマンドスクリプト生成

## 1. 概要

- **対象機能**: F-05 `terraform import` コマンドスクリプト生成  
- **目的**: import 対象となる `Resource` 一覧から、`terraform import` コマンド列を生成し、シェルスクリプトとして出力する。  
- **タスク優先度**: 5

## 2. 対象コンポーネント

- `pkg/importer.ImportCommandGenerator`
  - `GenerateImportScript(resources []Resource, cfg ImportScriptConfig) (string /* scriptPath */, error)`

## 3. 設定オブジェクト

```go
type ImportScriptConfig struct {
    TfDir      string // terraform 実行ディレクトリ (--tf-dir)
    ScriptName string // デフォルト "import.sh"
    Shell      string // "bash" を想定
}
```

## 4. 出力仕様

- 出力先:
  - デフォルト: `filepath.Join(TfDir, "import.sh")`
- スクリプトフォーマット:

```bash
#!/usr/bin/env bash
set -euo pipefail

cd "./infra" # --tf-dir

# 生成された import コマンド
terraform import 'aws_instance.web_1' 'i-0123456789abcdef0'
terraform import 'aws_subnet.subnet_public_a' 'subnet-0123456789abcdef0'
```

## 5. コマンド生成ロジック

1. 各 `Resource` について Terraform アドレスを構築:
   - `address := fmt.Sprintf("%s.%s", r.Type, r.Name)`（`aws_instance.web_1` など）。
2. import ID の決定:
   - AWS の場合、通常はクラウドの物理 ID を利用（例: インスタンス ID、サブネット ID）。
   - これらは `Resource.Attributes["id"]` もしくは `Labels["aws_id"]` 相当から取得する想定とし、F-02 側でセットしておく。
3. 1 リソースごとに 1 行の `terraform import` コマンド文字列を生成。

## 6. エラーハンドリング

- import ID が取得できない `Resource`:
  - WARN ログを出し、そのリソースはスキップ。
- スクリプトファイル書き込みエラー:
  - 権限エラー・ディスクフル時などはエラーを返して処理中断。

## 7. テスト観点

- 単体テスト:
  - 代表的な `Resource` セットから期待どおりのスクリプトテキストが生成されるか検証。
  - `TfDir` が相対パス・絶対パスの場合の `cd` 行が正しく生成されるか確認。
- 結合テスト:
  - 実際に生成された `import.sh` に実行権限を付与し、`bash import.sh` で `terraform import` が成功することを確認（テスト用環境）。


