# Archaeform VPC Import 詳細設計書 - F-09 Import ユースケース・オーケストレーション

## 1. 概要

- **対象機能**: VPC import 全体フローのオーケストレーション  
- **目的**: 既に実装済みの各コンポーネント（`pkg/aws`, `pkg/terraform`, `pkg/importer`）を **1 本のユースケースとして結線し、CLI から実行〜終了コードまでの流れを明確化** する。  
- **位置付け**:
  - `archae-vpc-import`（`cmd/vpc-importer/main.go`）から呼び出されるアプリケーション層ユースケース。
  - F-01〜F-08 の機能を「いつ・どの順番で・どの条件で」実行するかを定義する。

---

## 2. 対象範囲とコンポーネント

- **対象 CLI**
  - `archae-vpc-import` (`cmd/vpc-importer/main.go`)

- **ユースケース層（新規追加）**
  - 追加想定ファイル: `pkg/importer/usecase.go`（案）

- **利用する既存コンポーネント**
  - `pkg/aws`
    - `CloudDiscovery` / `AwsVpcDiscoveryService`（F-01）
    - `Logger` インターフェース
  - `pkg/terraform`
    - `Resource`, `Relation`（共通リソースモデル / F-02）
    - `DiscoveryScope`, `ResourceFilter`, `MatchResource`（F-01, F-08）
    - `TerraformExecutor`（terraform CLI ラッパ / F-06）
  - `pkg/importer`
    - `ExistingConfigAnalyzer`（F-04）
    - `HclGenerator`（F-03）
    - `ImportCommandGenerator`（F-05）
    - `ImportSummary`（F-07）

---

## 3. インターフェース設計

### 3.1 設定モデル (`ImportConfig`)

- **役割**: CLI 層（`cmd/vpc-importer`）でバリデーション済みの入力値を 1 つの構造体にまとめ、ユースケース層へ渡すためのコンフィグ。
- **配置案**: `pkg/importer/usecase.go`

```go
// pkg/importer/usecase.go（案）
package importer

import "github.com/ukms/archaeform/pkg/terraform"

// ImportConfig は VPC import ユースケース全体の設定を表す。
type ImportConfig struct {
    VpcID   string
    Region  string
    Profile string

    TfDir     string // Terraform 構成ディレクトリ（--tf-dir）
    OutputDir string // HCL 出力ディレクトリ（空なら TfDir/generated）

    Apply           bool                     // --apply
    ResourceFilters []terraform.ResourceFilter // --resource-filters
}
```

### 3.2 依存コンポーネント (`ImportDependencies`)

- **目的**: テスト容易性と疎結合のため、ユースケース層は具体実装に直接依存せず、抽象インターフェース経由で各コンポーネントを利用する。
- **CLI からの注入**: `cmd/vpc-importer/main.go` で具体実装を生成し、`ImportDependencies` に詰めて渡す。

```go
// Logger はユースケース層で利用する簡易ロガーインターフェース。
// cmd/vpc-importer の stdLogger を満たす最小限のメソッドのみ定義する。
type Logger interface {
    Infof(format string, args ...any)
    Warnf(format string, args ...any)
    Errorf(format string, args ...any)
}

// CloudDiscovery は F-01 の CloudDiscovery（AWS VPC ディスカバリ）を抽象化したもの。
type CloudDiscovery interface {
    ListResources(scope terraform.DiscoveryScope) ([]terraform.Resource, []terraform.Relation, error)
}

// ImportDependencies は ImportVpcResources が利用する依存コンポーネント群。
type ImportDependencies struct {
    Discovery        CloudDiscovery
    HclGen           *HclGenerator
    ExistingAnalyzer *ExistingConfigAnalyzer
    ImportCmdGen     *ImportCommandGenerator
    TfExecutor       terraform.TerraformExecutor

    Logger Logger
}
```

#### 3.2.1 CLI 側での依存解決イメージ

```go
// cmd/vpc-importer/main.go（イメージ）

logger := &stdLogger{}

var ec2 aws.Ec2API   // TODO: 実装差し込み
var elb aws.ElbAPI   // TODO: 実装差し込み
var rds aws.RdsAPI   // TODO: 実装差し込み

discovery := aws.NewAwsVpcDiscoveryService(ec2, elb, rds, logger)

deps := importer.ImportDependencies{
    Discovery:        discovery,
    HclGen:           importer.NewHclGenerator(),
    ExistingAnalyzer: importer.NewExistingConfigAnalyzer(),
    ImportCmdGen:     importer.NewImportCommandGenerator(),
    TfExecutor:       terraform.NewDefaultTerraformExecutor(),
    Logger:           logger,
}
```

### 3.3 ユースケース関数シグネチャ

- **責務**: F-01〜F-08 の機能を 1 本のフローとして実行し、`ImportSummary` と CLI の終了コードを決定する。

```go
// ExitCode は CLI に返すべき終了コードを表す。
type ExitCode int

const (
    ExitCodeOK           ExitCode = 0 // 正常終了（警告含む）
    ExitCodeFatal        ExitCode = 1 // 致命的エラーで処理不能
    ExitCodePartialError ExitCode = 2 // 一部 import 失敗・競合など
)

// ImportVpcResources は VPC import 全体を実行するユースケース。
// - cfg: CLI から受け取った設定
// - deps: 依存コンポーネント（DI）
// 戻り値:
// - *ImportSummary: F-07 で定義済みのサマリ結果（可能な限り常に非 nil を返す）
// - ExitCode: CLI が返すべき終了コード
// - error: プログラミングエラー or 想定外の致命的異常のみ
func ImportVpcResources(
    ctx context.Context,
    cfg ImportConfig,
    deps ImportDependencies,
) (*ImportSummary, ExitCode, error)
```

#### 3.3.1 CLI 側での呼び出しイメージ

```go
summary, code, err := importer.ImportVpcResources(ctx, cfg, deps)

// サマリのテキスト出力（F-07）
if summary != nil {
    _ = summary.WriteText(os.Stdout, cfg.VpcID, cfg.Region, hclOutputDir, importScriptPath)
}

if err != nil {
    logger.Errorf("import failed: %v", err)
}

os.Exit(int(code))
```

---

## 4. 処理フロー詳細

### 4.1 全体フロー（正常系）

1. **入力バリデーション**
   - `cfg.VpcID`, `cfg.Region`, `cfg.TfDir` が空でないこと。
   - `cfg.TfDir` が存在しディレクトリであること。
   - NG の場合:
     - `ImportSummary` を生成し、`Errors` に理由（例: `--vpc-id is required`）を 1 件追加。
     - `ExitCodeFatal (1)` と `error` を返す。

2. **Terraform state 空チェック（F-06 仕様の反映）**
   - 本ツールは **空の Terraform state 専用** であるため、`tfDir` に対応する state が空であることを事前確認する。
   - 実装案:
     - `terraform state list` を呼び出し、1 件以上のリソースが存在する場合はエラー。
     - ヘルパー関数例:

       ```go
       func EnsureEmptyState(tfDir string) error
       ```

   - 非空の場合:
     - `summary.Errors` に「このツールは空の state 専用であるため、既存 state がある場合は実行できない」旨を追加。
     - `ExitCodeFatal (1)` で終了（F-06 の設計に準拠）。

3. **DiscoveryScope 構築と VPC リソース列挙（F-01）**

   ```go
   scope := terraform.DiscoveryScope{
       VpcID:           cfg.VpcID,
       Region:          cfg.Region,
       Profile:         cfg.Profile,
       ResourceFilters: cfg.ResourceFilters, // 初期実装では未使用でも可
   }

   resources, relations, err := deps.Discovery.ListResources(scope)
   ```

   - AWS 認証失敗 / VPC 不存在など致命的なエラー時:
     - `summary.Errors` に内容を追加。
     - `ExitCodeFatal (1)` として終了。
   - 成功時:
     - `summary.TotalResources = len(resources)` を設定。

4. **リソースフィルタリング（F-08）**
   - `cfg.ResourceFilters` が空でない場合のみ適用。
   - `terraform.MatchResource` を利用し、`Resource` 単位でフィルタする。

   ```go
   filtered := make([]terraform.Resource, 0, len(resources))
   for _, r := range resources {
       if terraform.MatchResource(cfg.ResourceFilters, r) {
           filtered = append(filtered, r)
       }
   }

   // Relation は「From/To の両方が残った Resource 同士」に限定
   filteredRels := filterRelations(relations, filtered)
   ```

   - `filterRelations` の役割:
     - `relations` のうち、`From` と `To` の両方が `filtered` 内に存在するものだけを残す。
     - Resource.ID → Resource のマップを事前に構築して判定。
   - サマリ更新:
     - フィルタ適用前後の差分をカウントし、必要に応じて
       - `Warnings` に「リソースフィルタにより X 件のリソースが除外されました」などのメッセージを追加。

5. **既存 Terraform 構成の解析・競合検出（F-04）**

   ```go
   index, err := deps.ExistingAnalyzer.AnalyzeExistingConfigs(cfg.TfDir)
   // 一部ファイルのパースエラーなどは WARN として扱い、致命度に応じて継続可否を判断

   importable, conflicted := deps.ExistingAnalyzer.FilterConflicted(filtered, index)
   ```

   - サマリ更新:
     - `summary.ImportableResources = len(importable)`
     - `summary.ConflictedResources = len(conflicted)`
   - `conflicted` に対して:
     - 例: `aws_instance.web_1` が `main.tf:123` で既に定義されている、といった情報を組み立て、
       - `Warnings` に  
         `aws_instance.web_1 は既存構成 (main.tf:123) と競合するため import 対象から除外しました`  
         のようなメッセージを追加。

6. **HCL 生成（F-03）**

   ```go
   hclCfg := HclGenerationConfig{
       TfDir:         cfg.TfDir,
       OutputDir:     cfg.OutputDir,
       SplitStrategy: SplitByType, // 初期実装
   }

   hclResult, err := deps.HclGen.Generate(importable, filteredRels, hclCfg)
   ```

   - 失敗時:
     - 出力ディレクトリ作成失敗やファイル書き込み不可などは致命的エラーとして扱い、
       - `summary.Errors` に詳細を追加。
       - `ExitCodeFatal (1)` を返す。
   - 成功時:
     - `summary.GeneratedHclFiles = len(hclResult.GeneratedFiles)`
     - CLI 側のサマリ出力で `hclResult.OutputDir` を「HCL 出力先ディレクトリ」として表示。

7. **`terraform import` スクリプト生成（F-05）**

   ```go
   scriptPath, err := deps.ImportCmdGen.GenerateImportScript(importable, ImportScriptConfig{
       TfDir:      cfg.TfDir,
       ScriptName: "import.sh",
       Shell:      "bash",
   })
   ```

   - `GenerateImportScript` 内で `resolveImportID` により import ID が決定できないリソースはスキップされる。
     - ユースケース側では「有効な import ID を持つリソース数」を集計し、
       - `summary.GeneratedImportCommands` に設定。
       - スキップされた件数があれば `Warnings` に「import ID が不明のため X 件のリソースが import スクリプトに含まれていません」を追加。
   - スクリプトファイル作成失敗時:
     - 権限エラー等は致命的として `ExitCodeFatal (1)`。

8. **自動 import 実行（`--apply` 時 / F-06）**

   - `cfg.Apply == false` の場合:
     - `summary.ApplyRequested = false` のまま、以降の Terraform 実行はスキップ。

   - `cfg.Apply == true` の場合:
     1. `summary.ApplyRequested = true` を設定。
     2. `deps.TfExecutor.Init(cfg.TfDir)` を実行。
        - 失敗時は `summary.Errors` に詳細を追加し、`ExitCodeFatal (1)`。
     3. `importable` のうち、`resolveImportID` に成功したリソースについて順次 `Import` を実行。

        ```go
        for _, r := range importable {
            address := fmt.Sprintf("%s.%s", r.Type, r.Name)
            id, ok := resolveImportID(r)
            if !ok {
                continue
            }
            if err := deps.TfExecutor.Import(cfg.TfDir, address, id); err != nil {
                summary.ApplyFailed++
                deps.Logger.Errorf("terraform import failed: address=%s id=%s err=%v", address, id, err)
                summary.Errors = append(summary.Errors,
                    fmt.Sprintf("terraform import failed: %s %s: %v", address, id, err))
                // 初期実装では continue ポリシー（F-06）
                continue
            }
            summary.ApplySucceeded++
        }
        ```

     4. 成否に応じ、`summary.ApplySucceeded` / `summary.ApplyFailed` を更新。

9. **サマリ出力と終了コード決定（F-07）**

   - 終了コード判定ロジック案:

   ```go
   code := ExitCodeOK

   if len(summary.Errors) > 0 {
       // 致命的エラーが 1 件でもあれば ExitCodeFatal
       code = ExitCodeFatal
   } else if summary.ApplyRequested && summary.ApplyFailed > 0 {
       // Apply 実行時に一部リソースの import が失敗した場合
       code = ExitCodePartialError
   } else if summary.ConflictedResources > 0 {
       // 既存構成との競合がある場合も「部分的エラー」として扱う
       code = ExitCodePartialError
   }
   ```

   - `ImportVpcResources` は `summary` と `code` を返却し、F-07 で定義済みの `WriteText` での出力は CLI 側で実施。

---

## 5. ヘルパー関数設計

### 5.1 `filterRelations`

- **目的**: フィルタ後の `Resource` 一覧に対して不要な `Relation` を除去し、HCL 生成時に意味のない参照が出力されないようにする。

```go
func filterRelations(
    relations []terraform.Relation,
    resources []terraform.Resource,
) []terraform.Relation {
    resByID := make(map[string]struct{}, len(resources))
    for _, r := range resources {
        resByID[r.ID] = struct{}{}
    }

    var filtered []terraform.Relation
    for _, rel := range relations {
        if _, ok := resByID[rel.From]; !ok {
            continue
        }
        if _, ok := resByID[rel.To]; !ok {
            continue
        }
        filtered = append(filtered, rel)
    }
    return filtered
}
```

### 5.2 `EnsureEmptyState`（配置は要検討）

- **目的**: F-06 で定義されている「空の Terraform state 専用ツール」である制約を実装レベルで保証する。
- **配置案**:
  - `pkg/terraform/cli.go` に補助メソッドとして追加するか、
  - `pkg/importer` 内に Terraform 実行ユーティリティとして追加するか。

```go
func EnsureEmptyState(tfDir string) error {
    // terraform state list を実行し、出力に 1 行でもあればエラー
    // もしくは state ファイルの存在チェックを行う実装も検討
}
```

---

## 6. ログポリシー（ユースケース視点）

- **`[INFO]`**
  - フロー開始・終了（対象 VPC, リージョン, 検出リソース数など）。
  - 各フェーズの完了ログ（例: `HCL generation completed: files=5`）。

- **`[WARN]`**
  - 既存 `.tf` の一部ファイルがパース不能でスキップされた場合。
  - import ID が不明でスクリプトからスキップされたリソース。
  - `terraform import` で一部リソースのみ失敗した場合。

- **`[ERROR]`**
  - CLI 入力不備（必須フラグ不足など）。
  - Terraform state が空でない場合。
  - AWS 認証エラー・対象 VPC 不存在などの致命的エラー。
  - HCL 出力ディレクトリ作成失敗などの I/O 致命的エラー。
  - `terraform init` 失敗・バイナリ未検出。

---

## 7. テスト方針（ユースケース単位）

### 7.1 ユニットテスト

- `ImportVpcResources` に対して、`ImportDependencies` をモックで差し替えたテストを実施する。

- 主なパターン:
  - **正常系**:
    - Discovery, HCL 生成, import スクリプト生成が全て成功し、`ExitCodeOK` が返る。
  - **競合あり**:
    - `ExistingConfigAnalyzer.FilterConflicted` がいくつかの `conflicted` を返し、`ExitCodePartialError` になる。
  - **state 非空**:
    - `EnsureEmptyState` 相当がエラーを返した場合に `ExitCodeFatal` となり、以降の処理が実行されない。
  - **Apply 時の一部失敗**:
    - `TfExecutor.Import` の一部がエラーを返し、`summary.ApplyFailed > 0` かつ `ExitCodePartialError` になる。

### 7.2 結合テスト

- LocalStack 等を用いて小規模な VPC を構築し、実際に
  - VPC 内リソースが `Resource` / `Relation` として列挙されること（F-01, F-02）。
  - `*.tf` と `import.sh` が生成されること（F-03, F-05）。
  - `--apply` 指定時に `terraform import` が成功し、state にリソースが登録されること（F-06）。
  - 最終的な `ImportSummary` と終了コードが期待通りであること（F-07）。

を確認する。

---

## 8. 今後の拡張余地

- **クラウド追加**:
  - `CloudDiscovery` を GCP / Azure 実装でも共通で利用できるよう、`ImportConfig` / `DiscoveryScope` に provider 情報を追加する余地を残す。
- **エラー制御オプション**:
  - `--on-error=continue|abort` などのフラグを追加し、F-06 で想定している継続／中断ポリシーをユーザー指定可能にする。
- **詳細サマリ拡張**:
  - リソースタイプ別の import 成功／失敗件数を `ImportSummary` に追加し、F-07 の出力に反映する。


