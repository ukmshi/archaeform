# Archaeform VPC Import 詳細設計書 - F-02 内部抽象リソースモデルへのマッピング

## 1. 概要

- **対象機能**: F-02 内部抽象リソースモデルへのマッピング  
- **目的**: F-01 で取得した AWS リソース情報を、クラウド共通で利用可能な `Resource` / `Relation` モデルへ変換し、後続の HCL 生成・競合検出・import スクリプト生成で再利用可能な形にする。  
- **タスク優先度**: 2

## 2. 対象コンポーネント

- `pkg/terraform`（共通モデル）
  - `type Resource struct`
  - `type Relation struct`
- `pkg/importer` または `pkg/terraform/mapping`（マッピングロジック）
  - `type AwsToResourceMapper struct`

## 3. データモデル再掲

```go
type Resource struct {
    ID         string
    Provider   string
    Type       string
    Name       string
    Labels     map[string]string
    Attributes map[string]any
    Origin     string
}

type Relation struct {
    From string
    To   string
    Kind string
}
```

- `Origin` は VPC import 由来であることを示すため `"cloud"` を設定する。

## 4. マッピングインターフェース

```go
type CloudResourceMapper interface {
    MapFromAws(raw interface{}) ([]Resource, []Relation, error)
}
```

AWS 専用の具体実装:

```go
type AwsToResourceMapper struct {
    nameGenerator NameGenerator
}

func (m *AwsToResourceMapper) MapSubnet(subnets []rawSubnet) ([]Resource, []Relation, error)
func (m *AwsToResourceMapper) MapInstance(instances []rawInstance) ([]Resource, []Relation, error)
// ... 他リソース種別ごとに追加
```

## 5. マッピングルール

### 5.1 共通ルール

- `Provider`: 常に `"aws"`。
- `ID`:
  - フォーマット: `<provider>:<type>:<cloud-unique-id>`
  - 例: `aws:aws_instance:i-0123456789abcdef0`
- `Name`:
  - Terraform の論理名候補。`Name` タグ、もしくはリソース ID の一部から生成する。
  - 例: `web_1`, `vpc_main`, `subnet_public_a` など、HCL で利用可能な識別子に正規化する。
- `Labels`:
  - AWS タグ (`Tags`) をそのまま `Labels` にコピー（`Name` タグも含む）。
  - 追加メタデータとして `{"aws_region": "...", "vpc_id": "..."}` などを付加。
- `Attributes`:
  - HCL 生成に必要となるフィールドのみを抽出し格納。
  - 例: `{"cidr_block": "...", "availability_zone": "..."}` など。

### 5.2 リソース種別ごとの例

#### Subnet

- `Type`: `"aws_subnet"`
- 主な `Attributes`:
  - `vpc_id`, `cidr_block`, `availability_zone`, `map_public_ip_on_launch`, `tags`
- `Relation`:
  - `from`: サブネット `Resource.ID`
  - `to`: VPC `Resource.ID`
  - `kind`: `"network"`

#### EC2 Instance

- `Type`: `"aws_instance"`
- 主な `Attributes`:
  - `ami`, `instance_type`, `subnet_id`, `vpc_security_group_ids`, `tags` など。
- `Relation`:
  - インスタンス -> サブネット: `kind: "network"`
  - インスタンス -> セキュリティグループ: `kind: "security"`

## 6. 名前生成戦略 (`NameGenerator`)

Terraform のリソース論理名は HCL 上の可読性に大きく影響するため、専用の名前生成コンポーネントを設ける。

```go
type NameGenerator interface {
    Generate(resourceType string, labels map[string]string, id string) string
}
```

- 生成規約（例）:
  1. `Name` タグ値があれば、記号類を `_` に変換し小文字に変換したものをベースにする。
  2. 同一論理名が衝突する場合は `_1`, `_2` などのサフィックスを付与。
  3. `Name` タグがない場合は、`<type>_<short-id>` の形式（例: `instance_i0123abcd`）。

## 7. 処理フロー

1. F-01 から受け取った各種中間構造体（または AWS SDK レスポンス）を入力として受け取る。
2. リソース種別ごとに対応する `MapXXX` 関数を呼び出し、`[]Resource` / `[]Relation` を構築。
3. 全リソース分をマージし、重複しない `ID` を保証する。
4. マッピング中にエラーがあった場合、
   - 個別リソースの変換失敗は WARN ログ出力の上でスキップ。
   - 構造的に致命的な不整合（例: VPC が存在しないのにサブネットだけがある）は全体エラーとして扱う。

## 8. エラーハンドリング・バリデーション

- 入力チェック:
  - `VpcID` 必須確認（F-01 で実施済みだが二重チェックも可）。
  - リソース間の整合性確認（サブネットの `vpc_id` が指定 VPC と一致しない場合は除外するなど）。
- マッピング失敗:
  - 単一フィールドの欠損などは WARN としてスキップ。
  - 型変換に失敗した場合はログ出力し、そのリソースのみ除外。

## 9. テスト観点

- 単体テスト:
  - 代表的な AWS リソースレスポンスから期待される `Resource` / `Relation` が生成されるかを確認。
  - 名前生成ロジックが衝突時にも一意な名前を生成することを検証。
- 結合テスト:
  - F-01 と組み合わせて、LocalStack などで実際の AWS レスポンス相当データから内部モデルを生成し、期待した数・関係性になっているか確認。


*** Add File: docs/design/vpc_import/F03_terraform-hcl-generation.md
# Archaeform VPC Import 詳細設計書 - F-03 Terraform HCL ファイル生成

## 1. 概要

- **対象機能**: F-03 Terraform HCL ファイル生成  
- **目的**: F-02 で生成された内部 `Resource` / `Relation` モデルをもとに、Terraform の `*.tf` ファイルを所定ディレクトリに生成する。  
- **タスク優先度**: 3

## 2. 対象コンポーネント

- `pkg/importer.HclGenerator`
  - `Generate(resources []Resource, relations []Relation, cfg HclGenerationConfig) (HclGenerationResult, error)`
- `pkg/terraform`
  - HCL テンプレート共通ユーティリティ（フォーマット・インデントなど）

## 3. コンフィグ設計

```go
type HclGenerationConfig struct {
    TfDir          string // --tf-dir
    OutputDir      string // --output-dir があれば優先
    SplitStrategy  SplitStrategy
}

type SplitStrategy string

const (
    SplitByType   SplitStrategy = "by_type"   // 初期実装
    SplitByModule SplitStrategy = "by_module" // 将来拡張
    SplitSingle   SplitStrategy = "single"    // 将来拡張
)
```

## 4. 出力仕様

- デフォルト出力先:
  - `outputRoot := cfg.OutputDir` が空なら `filepath.Join(cfg.TfDir, "generated")`
- ファイル命名（SplitByType の場合）:
  - `aws_instance.tf`, `aws_subnet.tf` のように `Type` ごとに 1 ファイル。
- リソースブロック命名:
  - `resource "<type>" "<name>" { ... }`
  - `<name>` は `Resource.Name` を利用。

## 5. 生成ロジック

1. `resources` を `Type` ごとにグルーピング。
2. 各 `Type` ごとに `[]Resource` を走査し、HCL ブロックテキストを生成。
3. 既存ファイルが存在する場合の扱い:
   - 初期実装では `generated/` 配下を「再生成領域」とみなし、毎回上書き（`truncate & write`）。
   - 既存手書き HCL とのマージは行わない（既存 `.tf` は F-04 で解析するのみ）。

### 5.1 HCL ブロック生成

- 例: EC2 インスタンス

```hcl
resource "aws_instance" "web_1" {
  ami           = "ami-0123456789abcdef0"
  instance_type = "t3.micro"

  subnet_id              = aws_subnet.subnet_public_a.id
  vpc_security_group_ids = [aws_security_group.sg_web.id]

  tags = {
    "Name" = "web-1"
    "Env"  = "prod"
  }
}
```

- `Relations` を参照して、他リソースへの参照（`aws_subnet.xxx.id` など）を解決する。
  - 参照名は `Relation.To` に紐づく `Resource.Name` を利用。

### 5.2 依存関係解決

- リレーション `Relation{From: instanceID, To: subnetID, Kind: "network"}` がある場合:
  - `attributes["subnet_id"]` を `aws_subnet.<subnet.Name>.id` に置き換える。
- セキュリティグループの `vpc_security_group_ids` も同様に `aws_security_group.<name>.id` の配列へ変換。

## 6. 既存ファイルとの関係

- F-04 の `ExistingConfigAnalyzer` は `--tf-dir` 直下の `.tf` を対象とし、`generated/` 配下も含めて解析対象とする。
- 初回 import 時:
  - `generated/` が存在しない場合はディレクトリ作成。
- 2 回目以降:
  - `generated/` 配下のファイルはすべて再生成（古いリソースが残らないようにする）。

## 7. エラーハンドリング

- 出力ディレクトリ作成失敗:
  - 権限エラー・パス不正時は即座にエラーを返却し、終了コード 1 相当として扱う。
- ファイル書き込み中のエラー:
  - 途中まで生成されたファイルを削除し、`ImportSummary` に「HCL 出力失敗」として記録。
- HCL 生成エラー:
  - 個別リソースの HCL 生成でエラーが発生した場合、そのリソースのみスキップし、WARN ログに詳細を出力。

## 8. テスト観点

- ユニットテスト:
  - 入力 `Resource` の属性から期待どおりの HCL テキストが生成されるか確認。
  - `Relation` に基づくリソース間参照が正しい `address` になっているか検証。
- 結合テスト:
  - F-02 までの流れと接続し、LocalStack で構築した VPC から実際に `*.tf` を出力し、`terraform validate` が成功することを確認。


*** Add File: docs/design/vpc_import/F04_existing-tf-config-analysis.md
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


*** Add File: docs/design/vpc_import/F05_terraform-import-script-generation.md
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


*** Add File: docs/design/vpc_import/F06_auto-import-execution.md
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


*** Add File: docs/design/vpc_import/F07_summary-output.md
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


*** Add File: docs/design/vpc_import/F08_resource-filtering.md
# Archaeform VPC Import 詳細設計書 - F-08 リソースフィルタリング（任意機能）

## 1. 概要

- **対象機能**: F-08 リソースフィルタリング  
- **目的**: `--resource-filters` 等のオプションを用いて、対象リソースタイプやタグ条件に基づき import 対象を絞り込む。  
- **タスク優先度**: 8（任意機能）

## 2. 対象コンポーネント

- CLI 層:
  - `cmd/vpc-importer`: `--resource-filters` フラグの定義・パース。
- ドメイン層:
  - `DiscoveryScope.ResourceFilters []ResourceFilter`
  - `ResourceFilter` 構造体とパーサ。

## 3. 設定モデル

```go
type ResourceFilter struct {
    Type       string            // "aws_instance" など、空なら全タイプ対象
    TagFilters map[string]string // key=value
}
```

## 4. CLI からの入力形式

- 例:
  - `--resource-filters "type=aws_instance,tag:Env=prod" --resource-filters "tag:Owner=team-a"`
- パースルール:
  - `,` 区切りで複数条件。
  - `type=<resource_type>` または `tag:<key>=<value>` フォーマット。

## 5. フィルタ適用タイミング

- F-01 の VPC リソース列挙後、F-02 で `Resource` にマッピングする際、`ResourceFilter` に従って対象を絞り込む。
- 将来的には AWS API レベルでフィルタリング（タグフィルタなど）を行う余地もあるが、初期実装ではアプリケーション側でのフィルタにとどめる。

## 6. フィルタロジック

```go
func MatchResource(filters []ResourceFilter, r Resource) bool {
    if len(filters) == 0 {
        return true
    }
    for _, f := range filters {
        if f.Type != "" && f.Type != r.Type {
            continue
        }
        if !matchTags(f.TagFilters, r.Labels) {
            continue
        }
        return true
    }
    return false
}
```

## 7. サマリとの連携

- フィルタにより除外されたリソース数を `ImportSummary` に記録し、「discovered」と「importable」の差分の一因としてユーザーに伝える。

## 8. テスト観点

- CLI フラグのパーステスト（複数回指定・不正形式の扱い）。
- `MatchResource` の単体テスト（タイプ一致・タグ一致・複合条件など）。


