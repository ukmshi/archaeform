# Archaeform VPC Import 詳細設計書 - F-04 既存 Terraform 構成の解析・競合検出

## 1. 概要

- **対象機能**: F-04 既存 Terraform 構成の解析・競合検出  
- **目的**: `--tf-dir` 配下の既存 `.tf` ファイルを解析し、既に Terraform 管理下にあるリソースと、今回 import 対象のリソースとの重複（競合）を検出する。  
- **タスク優先度**: 4

## 2. 対象コンポーネント

- `pkg/importer.ExistingConfigAnalyzer`
  - `AnalyzeExistingConfigs(tfDir string) (ExistingConfigIndex, error)`
  - `FilterConflicted(resources []Resource, index ExistingConfigIndex) (importable []Resource, conflicted []ConflictedResource)`
- `pkg/terraform`
  - HCL パーサ（`hcl/v2` or `terraform-config-inspect` 相当のロジック）

## 3. データ構造

```go
type ExistingConfigIndex struct {
    Resources map[ResourceKey]ExistingResourceMeta
}

type ResourceKey struct {
    Provider string // "aws"
    Type     string // "aws_instance"
    Name     string // "web_1"
}

type ExistingResourceMeta struct {
    FilePath string
    Line     int
}

type ConflictedResource struct {
    Imported Resource
    Existing ExistingResourceMeta
}
```

## 4. 処理フロー

### 4.1 既存 `.tf` のインデックス作成

1. `tfDir` 以下を再帰的に走査し、拡張子 `.tf` のファイルを列挙。
   - `generated/` 配下も解析対象とする（差分管理のため）。
2. 各 `.tf` ファイルを HCL パーサで読み込み、`resource` ブロックを抽出。
3. 各 `resource "<type>" "<name>"` について `ResourceKey` を生成し、`ExistingConfigIndex.Resources` に登録。

### 4.2 競合検出

1. F-02 の `Resource` リストを入力として受け取る。
2. 各 `Resource` について、
   - `key := ResourceKey{Provider: r.Provider, Type: r.Type, Name: r.Name}` を生成。
   - `index.Resources[key]` の存在をチェック。
3. 存在する場合:
   - `ConflictedResource{Imported: r, Existing: meta}` を `conflicted` スライスに追加。
4. 存在しない場合:
   - `importable` スライスに追加。

## 5. 出力仕様

- `importable []Resource`:
  - HCL 生成・import スクリプト生成の対象となるリソース。
- `conflicted []ConflictedResource`:
  - 競合検出されたリソース。F-07 のサマリ出力、WARN ログの根拠として利用。

## 6. エラーハンドリング

- `.tf` パースエラー:
  - 単一ファイルのパースに失敗した場合、そのファイルのみスキップし WARN ログを出力。
  - 構成全体でほとんどのファイルがパース不能な場合は致命的エラーとして扱う。
- ディレクトリ読み取りエラー:
  - `tfDir` 不存在・権限エラー時は直ちにエラーを返し、終了コード 1 相当として扱う。

## 7. ログ出力

- `[INFO]`:
  - 解析対象ファイル数、検出された既存リソース数。
- `[WARN]`:
  - パース不能なファイル。
  - 競合リソースの概要（例: 「aws_instance.web_1 が既存構成に存在するため import 対象から除外」）。

## 8. テスト観点

- 単体テスト:
  - 小さな `.tf` サンプルから期待どおりの `ExistingConfigIndex` が構築されること。
  - `FilterConflicted` による importable/conflicted の分類が正しいこと。
- 結合テスト:
  - 実際のプロジェクト相当の `.tf` 構成を用いて、誤検知・見逃しがないか確認。


