## Archaeform VPC Import 基本設計書

### 1. 対象と目的

- **対象**  
  - 本ドキュメントは、`archae-vpc-import`（仮称）CLI が提供する「VPC 内リソース自動 import 機能」の基本設計を示す。  
  - システム設計書（`system_design.md`）のうち、「2.1 VPC 内リソース自動 import CLI」を対象とする。

- **目的**  
  - 指定された AWS VPC 内の既存クラウドリソースを走査し、Terraform 管理下へ取り込むための HCL ファイルおよび `terraform import` コマンドスクリプトを自動生成できるようにする。  
  - 既存の Terraform 構成ディレクトリがある場合でも、競合を検知しつつ安全に追加できる仕組みを提供する。  
  - **VPC 内に配置されたリソースに「論理的に関連する外部マネージドサービス」も、必要に応じて import 対象とする**ことで、実運用上意味のある単位で Terraform 管理下へ移行できるようにする。  
    - 例: IAM ロール / ポリシー、S3 バケット、CloudWatch Logs ロググループ / CloudWatch アラーム、KMS キー、Secrets Manager シークレット / SSM パラメータ、ECR リポジトリ、SNS トピック / SQS キュー、WAF Web ACL 等  
  - 将来的な他クラウド対応（GCP, Azure 等）を見据え、内部の抽象リソースモデルとインターフェースを共通化する。

### 2. 前提・制約

- **前提**
  - 初期ターゲットは AWS の VPC 環境。  
  - Terraform CLI はあらかじめインストール済みであること（バージョンは 0.13 以降を想定）。  
  - AWS 認証情報は AWS SDK の標準クレデンシャルプロバイダチェーンまたは `--profile` により取得可能であること。  
  - 対象 VPC ID とリージョンが一意に指定可能であること。

- **対応 OS**
  - macOS（優先）、Linux。  
  - Windows は将来対応を想定し、パスや改行コードなど OS 依存箇所は抽象化する。

- **パフォーマンス・スケール**
  - 対象リソース数は数百〜数千程度を想定。  
  - AWS API コール結果や Terraform 解析結果はキャッシュ可能なインターフェースを定義し、将来的なキャッシュ導入を可能とする。

- **非機能的制約**
  - CLI は標準出力／標準エラーへのテキスト出力のみを行う。  
  - ネットワークアクセスは AWS API へのアクセスのみに限定し、生成したファイルやメタデータはローカルディスクのみに保存する。

### 3. 機能一覧

- **F-01: VPC 内リソース列挙**
  - 指定された `--vpc-id`, `--region` をもとに、対象 VPC 内の AWS リソース（サブネット、ルートテーブル、IGW/NATGW、セキュリティグループ、EC2、ALB/NLB、RDS、Lambda（VPC 接続）等）を列挙する。
  - さらに、**VPC 内リソースから参照されている外部マネージドサービス（IAM ロール / ポリシー、S3 バケット等）を「関連リソース」として発見するための起点情報**を収集する。

- **F-02: 内部抽象リソースモデルへのマッピング**
  - 列挙した各 AWS リソースおよび関連マネージドサービスを共通の `Resource` / `Relation` モデルへ変換する。  
  - VPC 内リソースと関連外部リソース（IAM, S3 など）の関係を `Relation` として表現し、後続の HCL 生成で参照解決できるようにする。

- **F-03: Terraform HCL ファイル生成**
  - 内部リソースモデルから Terraform の `*.tf` ファイルを生成し、指定ディレクトリ配下（例: `--tf-dir/generated`）に出力する。  
  - 既存 Terraform ディレクトリがある場合、競合検出結果に基づき、新規定義のみを追加する。

- **F-04: 既存 Terraform 構成の解析・競合検出**
  - `--tf-dir` 配下の既存 `.tf` を解析し、既に定義済みのリソースと import 対象リソースの重複を検知する。  
  - 競合がある場合は当該リソースをスキップし、警告を出力する。

- **F-05: `terraform import` コマンドスクリプト生成**
  - 各リソースについて `terraform import 'aws_instance.example' 'i-xxxx'` 形式のコマンド列を生成し、シェルスクリプト（例: `import.sh`）として出力する。

- **F-06: 自動 import 実行（オプション）**
  - `--apply` オプション指定時に Terraform CLI をサブプロセスとして起動し、生成した import コマンドを自動実行する。  
  - エラー発生時は該当リソースと理由を明示し、残りのリソース処理可否を制御できるようにする（継続／中断ポリシーはオプション化を検討）。

- **F-07: サマリ出力**
  - 対象リソース数、生成された HCL ファイル数、生成された import コマンド数、スキップされたリソース、警告・エラー概要を標準出力に表示する。

- **F-08: リソースフィルタリング（任意機能）**
  - `--resource-filters` などのオプションにより、対象リソースタイプやタグなどで import 対象を絞り込む。

### 4. CLI インターフェース設計

- **コマンド名**
  - 実行例:  
    - `archae-vpc-import --vpc-id vpc-xxxx --region ap-northeast-1 --tf-dir ./infra`

- **主なオプション**
  - `--vpc-id` (必須): 対象 VPC ID。  
  - `--region` (必須): AWS リージョン。`AWS_REGION` / `AWS_DEFAULT_REGION` があれば省略可能。  
  - `--profile` (任意): AWS プロファイル名。未指定時はデフォルトのクレデンシャルチェーンを利用。  
  - `--tf-dir` (必須): Terraform 構成／生成ファイルを格納するディレクトリ。  
  - `--plan-output` (任意): 将来的な `terraform plan` 出力保存先パス。初期は予約のみ。  
  - `--apply` (任意, bool): import スクリプトを実行し、`terraform import` を自動実行するかどうか。  
  - `--resource-filters` (任意): 対象リソースタイプやタグを指定するためのフィルタ式（例: `type=aws_instance,tag:Env=prod`）。  
  - `--output-dir` (任意): 生成ファイル出力先を `--tf-dir` 配下のサブディレクトリ以外に変更する場合に使用。  
  - `--dry-run` (任意, bool): 実際にはファイルを書き込まず、生成予定の要約のみを表示するオプション（将来拡張候補）。

- **終了コード**
  - `0`: 正常終了（警告はあり得る）。  
  - `1`: 致命的なエラーにより処理継続不可（例: 認証失敗、AWS API 全体の失敗、`--tf-dir` 不正など）。  
  - `2`: 一部リソースで import 失敗や競合が発生したが、処理は可能な範囲で完了した場合（サマリに詳細を出力）。

- **入力情報の優先順位**
  - リージョン: CLI フラグ `--region` > 環境変数 `AWS_REGION` / `AWS_DEFAULT_REGION`。  
  - プロファイル: CLI フラグ `--profile` > 環境変数 `AWS_PROFILE` > デフォルトプロファイル。

### 5. アーキテクチャ・モジュール構成

- **モノレポ構成上の位置付け**
  - `cmd/vpc-importer/`  
    - CLI エントリポイントおよび Runner。  
  - `pkg/aws/`  
    - AWS クライアント・VPC 内リソースディスカバリ。  
  - `pkg/importer/`  
    - Terraform 向け HCL 生成、import コマンド生成、既存構成解析。  
  - `pkg/terraform/`（共通）  
    - 内部 `Resource` / `Relation` モデル、Terraform 構成・状態解析との共通部分。  

- **主なコンポーネント**
  - `cmd/vpc-importer/main.go`  
    - `cobra` を用いた CLI 定義。  
    - フラグ定義、ヘルプテキスト、バージョン情報の表示。  
  - `runner` パッケージ（または `cmd/vpc-importer` 内モジュール）  
    - CLI 引数・環境変数から設定 (`ImportConfig`) を構築。  
    - 依存コンポーネント（`AwsVpcDiscoveryService`, `HclGenerator`, `ImportCommandGenerator`, `ExistingConfigAnalyzer` 等）を組み立て、`usecase.ImportVpcResources` を呼び出す。
  - `usecase.ImportVpcResources`  
    - アプリケーション層のユースケースとして、VPC import 処理全体のオーケストレーションを担当。  
    - エラー・警告を収集し、サマリ結果 (`ImportSummary`) を返す。

### 6. コンポーネント詳細

- **`pkg/aws.VpcDiscoveryService`**
  - 役割: 対象 VPC / リージョンに対して AWS SDK を用いてリソース一覧を取得する。  
  - インターフェース案:  
    - `ListVpcs() ([]Resource, []Relation, error)`  
    - `ListSubnets(vpcId string) ([]Resource, []Relation, error)`  
    - `ListRouteTables(vpcId string) ([]Resource, []Relation, error)`  
    - `ListSecurityGroups(vpcId string) ([]Resource, []Relation, error)`  
    - `ListInstances(vpcId string) ([]Resource, []Relation, error)`  
  - 将来のクラウド拡張に向けて、`CloudDiscovery` インターフェースを定義:  
    - `ListResources(scope DiscoveryScope) ([]Resource, []Relation, error)`  
    - `DiscoveryScope` は VPC ID, リージョン, フィルタ条件を保持。

- **`pkg/importer.HclGenerator`**
  - 役割: 内部 `Resource` モデルから Terraform HCL を生成し、ファイルとして出力する。  
  - 機能:
    - リソースタイプごとにテンプレートもしくはコード生成ロジックを持ち、`resource "aws_instance" "example" { ... }` を構築。  
    - ファイル分割戦略（タイプごと／VPC 単位／単一ファイル）を設定オプションで切り替え可能にする。初期実装ではシンプルな戦略（例: タイプ別ファイル）を採用。  
  - インターフェース案:
    - `Generate(resources []Resource, relations []Relation, cfg HclGenerationConfig) (HclGenerationResult, error)`

- **`pkg/importer.ImportCommandGenerator`**
  - 役割: `terraform import` コマンド列を生成し、シェルスクリプトとして出力する。  
  - インターフェース案:
    - `GenerateImportScript(resources []Resource, cfg ImportScriptConfig) (string /* scriptPath */, error)`  
  - 設定:
    - Terraform ワーキングディレクトリ（`--tf-dir`）、スクリプトファイル名（例: `import.sh`）、シェル種別（初期は POSIX / bash 前提）。

- **`pkg/importer.ExistingConfigAnalyzer`**
  - 役割: 既存 `.tf` ファイルを解析し、import 対象リソースとの重複を検出する。  
  - 機能:
    - 既存リソースの `(provider, type, name)` キーを抽出。  
    - 同一キーを持つリソースが内部モデルに存在する場合は「競合」として扱う。  
  - インターフェース案:
    - `AnalyzeExistingConfigs(tfDir string) (ExistingConfigIndex, error)`  
    - `FilterConflicted(resources []Resource, index ExistingConfigIndex) (importable []Resource, conflicted []ConflictedResource)`

### 7. データモデル設計

- **共通リソースモデル (`Resource`)**
  - フィールド:
    - `id`: 内部一意 ID（例: `aws:aws_instance:example`）。  
    - `provider`: 例 `aws`。  
    - `type`: 例 `aws_instance`, `aws_subnet`。  
    - `name`: Terraform 論理名候補（例: `web_server`）。  
    - `labels`: `map[string]string`（タグやメタデータ, 例: `{"Name": "web-1", "Env": "prod"}`）。  
    - `attributes`: `map[string]any`（初期は汎用マップとして保持）。  
    - `origin`: 例 `cloud`（VPC import 由来）, 将来的に `terraform_config`, `terraform_state` も利用。  

- **リレーションモデル (`Relation`)**
  - フィールド:
    - `from`: `Resource.id`。  
    - `to`: `Resource.id`。  
    - `kind`: `depends_on`, `network`, `security`, `contains` などの関係種別。  

- **VPC import 固有情報**
  - `DiscoveryScope`
    - `VpcID`: string  
    - `Region`: string  
    - `Profile`: string（任意）  
    - `ResourceFilters`: []`ResourceFilter`  
  - `ResourceFilter`
    - `Type`: string（例: `aws_instance`）  
    - `TagFilters`: map[string]string（例: `{"Env": "prod"}`）

### 8. 処理フロー詳細

- **全体フロー (`usecase.ImportVpcResources`)**
  1. CLI から受け取った `ImportConfig` を検証（必須パラメータ、パス存在確認など）。  
  2. `DiscoveryScope` を構築し、`AwsVpcDiscoveryService` を用いて VPC 内リソースを列挙。  
  3. 列挙結果を内部 `Resource` / `Relation` モデルへマッピング。  
  4. `ExistingConfigAnalyzer` により `--tf-dir` 配下の既存 `.tf` を解析し、重複リソースを検出。  
  5. 競合リソースを除外したインポート対象リソース集合を確定。  
  6. `HclGenerator` を用いて HCL ファイルを生成し、指定ディレクトリへ書き出し。  
  7. `ImportCommandGenerator` を用いて import スクリプトを生成。  
  8. `--apply` 指定時は Terraform CLI をサブプロセスとして起動し、import スクリプトを順次実行。  
  9. 成功／失敗／スキップ件数を集計し、`ImportSummary` を構築して標準出力にサマリを表示。  
  10. エラー種別に応じて適切な終了コードを返却。

- **Terraform CLI 呼び出しフロー（`--apply` 時）**
  1. `tfDir` をワーキングディレクトリとして `terraform init` 実行（必要であれば、初回のみ）。  
  2. 生成された import スクリプト内のコマンドを順に実行。  
  3. 各コマンドの標準出力／標準エラーをログに出力し、失敗時は対象リソース ID とともに記録。  
  4. 継続／中断ポリシーに従って処理を続行または終了。

### 9. 外部インターフェース設計

- **AWS SDK インターフェース**
  - AWS SDK for Go v2 を利用し、各サービスクライアント（EC2, ELB, RDS 等）を DI 可能なインターフェースとして抽象化する。  
  - 内部ではリトライポリシーやタイムアウトを設定可能とし、将来的なチューニングを容易にする。

- **ファイルシステムインターフェース**
  - HCL ファイル出力先:
    - デフォルト: `--tf-dir/generated/`。  
    - 変更可能: `--output-dir` 指定時はそちらを優先。  
  - import スクリプト出力先:
    - デフォルト: `--tf-dir/import.sh`。  
    - 将来的に OS ごとのバリエーション（`.bat` 等）を追加できる余地を残しておく。

- **Terraform CLI インターフェース**
  - コマンド実行パターン（例）:
    - `terraform init`（必要に応じて）  
    - `terraform import <address> <id>`  
  - 実行方法:
    - Go の `os/exec` を用いてサブプロセスとして実行し、標準出力・標準エラーをストリーミングで取得・表示。

### 10. 設定・拡張性

- **設定オブジェクト (`ImportConfig`)**
  - フィールド例:
    - `VpcID`  
    - `Region`  
    - `Profile`  
    - `TfDir`  
    - `OutputDir`  
    - `Apply` (bool)  
    - `ResourceFilters` ([]`ResourceFilter`)  

- **拡張ポイント**
  - クラウド追加:
    - `CloudDiscovery` インターフェースに基づき、`GcpVpcDiscoveryService`, `AzureVnetDiscoveryService` などの実装を追加することで対応。  
  - リソースタイプ追加:
    - `VpcDiscoveryService` の実装に新たなリソース列挙メソッドを追加し、`HclGenerator` に対応テンプレートを追加する。  
  - 出力形式拡張:
    - 将来的に HCL に加えて JSON 形式やメタデータエクスポート（可視化用）を追加可能なよう、`HclGenerationResult` に内部表現を保持しておく。

### 11. エラーハンドリング・ログ

- **エラーカテゴリ**
  - 設定エラー: 必須パラメータ不足、パス不正、書き込み権限不足。  
  - 認証・認可エラー: AWS クレデンシャル不備、アクセス拒否。  
  - AWS API エラー: 一時的なネットワーク障害、スロットリング、サービスエラー。  
  - Terraform 実行エラー: `terraform import` 失敗、バイナリ未検出など。  
  - 競合エラー: 既存 `.tf` とのリソース重複。

- **ログ出力ポリシー**
  - 標準出力:
    - 進捗メッセージ、サマリ、警告（ユーザーが直接見る情報）。  
  - 標準エラー:
    - 詳細なエラー内容、スタックトレースに近い情報（必要に応じて）。  
  - ログレベル:
    - 初期は INFO / WARN / ERROR 程度の簡易的なレベル分けを文字列で表現（例: `[INFO] ...`）。  
    - 将来的にフラグ（`--log-level`）で制御可能とする。

### 12. テスト方針（概要）

- **ユニットテスト**
  - `VpcDiscoveryService` に対してモック AWS クライアントを用いたテスト。  
  - `HclGenerator`, `ImportCommandGenerator`, `ExistingConfigAnalyzer` の入出力検証。  

- **結合テスト**
  - ローカルのモック AWS（LocalStack 等）利用を検討し、実際に VPC リソースを作成してから import までの一連の流れを検証。  

- **E2E テスト（将来）**
  - 小規模なサンプル VPC を Terraform で構築し、その状態から `archae-vpc-import` を実行して HCL / import スクリプトの妥当性を確認。


