## Archaeform システム設計書

### 1. 概要

- **目的**  
  - 指定した VPC 内の既存クラウドリソースを Terraform 管理下へ自動で取り込む CLI を提供する。  
  - Terraform 配下のリソース構成をグラフとして可視化し、詳細情報・関連リソースをローカル GUI で閲覧できる CLI を提供する。  
  - これらを単一モノレポ内で開発・運用し、共通モジュールの再利用性を高める。

- **前提**  
  - 初期ターゲットクラウドは AWS（VPC = AWS VPC）を想定し、将来的に他クラウド（GCP VPC, Azure Virtual Network 等）に拡張可能な設計とする。  
  - Terraform は OSS 版を想定し、0.13 以降の一般的な構成／状態ファイルを対象とする。  
  - VPC import CLI は **Terraform workspace の state が空であること**を前提とし、`terraform state list` などで 1 件以上のリソースが存在する workspace での import はサポートしない（その場合はエラー終了とする）。

### 2. 機能要件

#### 2.1 VPC 内リソース自動 import CLI

- **入力**  
  - 必須: `--vpc-id`, `--region`（`AWS_REGION` 等の環境変数でも可）  
  - 任意: `--profile`, `--tf-dir`, `--plan-output`, `--apply`, `--resource-filters` など
- **主な機能**
  - 対象 VPC 内の AWS リソースを列挙
    - サブネット, ルートテーブル, IGW/NATGW, セキュリティグループ, EC2, ALB/NLB, RDS 等（段階的に拡張）
  - 各リソースを内部の抽象リソースモデルにマッピング
  - Terraform 用の HCL ファイルを生成（`*.tf`）  
    - 基本的には **空の state を持つ新規 workspace 向けに HCL を生成**する（既存 Terraform プロジェクトとの安全なマージは将来拡張とする）
  - `terraform import` コマンド列を生成
    - デフォルトは「コマンドスクリプト生成のみ」  
    - `--apply` 指定時は Terraform CLI をサブプロセスとして起動し、自動 import（このとき state が空でない場合はエラーとして import を実行しない）
  - 生成内容（HCL / import スクリプト）のサマリを出力
- **出力**
  - `--tf-dir` 配下の `.tf` ファイル群
  - `import.sh` などの import コマンドスクリプト（任意）
  - 標準出力でのサマリ（リソース数、スキップされたリソース、警告など）

#### 2.2 Terraform 可視化 GUI CLI

- **入力**
  - 必須: `--tf-dir` または `--state-file`  
  - 任意: `--port`, `--no-open-browser`, `--workspace`, `--graph-scope` など
- **主な機能**
  - Terraform 構成・状態ファイルからリソース情報を取得
    - `.tf` / `.tf.json` ファイル、`.tfstate` ファイル、`terraform show -json` 出力など
  - リソースグラフ（ノード＝リソース、エッジ＝依存関係・接続関係）を構築
  - ローカル HTTP サーバを起動し、フロントエンド SPA を配信
  - ブラウザで以下を閲覧可能：
    - 全リソースのグラフビュー（ズーム・パン・ドラッグ）
    - リソース詳細ビュー（属性、タグ、モジュール、ワークスペース等）
    - 関連リソース一覧（依存リソース、同じ VPC / サブネット内リソースなど）
    - 絞り込み（モジュール単位、リソースタイプ、タグなど）
- **出力**
  - ローカルブラウザでの UI 表示
  - 必要に応じて、グラフを JSON / 画像としてエクスポート（将来拡張）

### 3. 非機能要件

- **実行環境**
  - 対応 OS: macOS（優先）, Linux, 将来的に Windows  
  - 依存: Terraform CLI がインストール済みであることを前提（バージョンは設定で指定可能）
- **配布形態**
  - 各 CLI は単一バイナリとして配布（Go の静的リンクを活用）  
  - フロントエンドはビルド済み静的ファイルを Go バイナリへ埋め込み
- **パフォーマンス**
  - 中規模環境（数百〜数千リソース）で実用的なレスポンスを確保  
  - 重い処理（AWS API コール、Terraform 解析）はキャッシュ可能なインターフェースで設計
- **セキュリティ**
  - クレデンシャルは AWS SDK の標準クレデンシャルプロバイダチェーンを利用（CLI オプションで profile 指定可能）
  - ローカル HTTP サーバは `127.0.0.1` のみで Listen
  - 送受信データはデフォルトで外部送信しない（純ローカル動作）

### 4. 技術選定

#### 4.1 言語・フレームワーク

- **バックエンド / CLI:** Go
  - 理由:
    - Terraform 本体・エコシステム（HCL パーサ、ライブラリ）が Go 製で相性が良い
    - 単一バイナリで配布可能で、クロスコンパイルが容易
    - CLI 開発向けの成熟したライブラリ（`cobra` 等）が存在
    - AWS SDK for Go v2 による API アクセスが安定

- **フロントエンド:** TypeScript + React + Vite
  - 理由:
    - UI 開発の生産性が高く、サードパーティのグラフ可視化ライブラリが豊富
    - Vite により高速な開発サーバ／ビルドが可能

- **グラフ可視化ライブラリ例**（候補）
  - `react-force-graph` / `vis-network` / `Cytoscape.js` などから選定

#### 4.2 Terraform 連携

- **構成ファイル解析**
  - `hashicorp/hcl` 系ライブラリ or `terraform-config-inspect` を利用して `.tf` ファイルから構成情報を取得。
- **状態取得**
  - 既存 `.tfstate` ファイルの JSON を直接パース  
  - あるいは `terraform show -json` をサブプロセスで呼び出し、標準化された JSON 形式を利用。
- **import 実行**
  - 初期は「HCL と import コマンドの生成のみ」がデフォルト。  
  - `--apply` オプション時に `terraform import` を自動実行（エラー時は詳細ログを出力）。

### 5. モノレポ構成

レポジトリ直下（ルート）に Go モジュールとフロントエンドワークスペースをまとめる。

- **ルート**
  - `go.mod`, `go.sum`（Go モジュール定義）
  - `package.json`, `pnpm-lock.yaml` 等（フロントエンドワークスペース、必要に応じて）
  - `cmd/` … 各種 CLI エントリポイント
    - `cmd/vpc-importer/` … `archae-vpc-import`（仮称）
    - `cmd/tf-viz/` … `archae-tf-viz`（仮称）
  - `pkg/` … 共有ライブラリ群
    - `pkg/aws/` … AWS クライアント・VPC 内リソースディスカバリ
    - `pkg/importer/` … Terraform import 向けロジック
    - `pkg/terraform/` … Terraform 構成・状態解析、内部モデル変換
    - `pkg/graph/` … リソースグラフのデータモデルと構築ロジック
    - `pkg/server/` … GUI 用ローカル HTTP サーバ（API & SPA 配信）
  - `ui/`
    - `ui/tf-viz/` … Terraform 可視化用フロントエンド（React + Vite）
  - `docs/`
    - `design/` … 設計ドキュメント（本ファイルなど）

### 6. コンポーネント設計

#### 6.1 内部リソースモデル（共通）

- **Resource**
  - `id`: 内部一意 ID（`provider:type:name` 等）
  - `provider`: 例 `aws`
  - `type`: 例 `aws_instance`, `aws_subnet`
  - `name`: Terraform 論理名候補
  - `labels`: タグ・メタデータ（`key: value`）
  - `attributes`: 抽象化された属性のマップ（構造化が望ましいが、初期は汎用マップ）
  - `origin`: `cloud` / `terraform_config` / `terraform_state` 等

- **Relation**
  - `from`: Resource ID
  - `to`: Resource ID
  - `kind`: `depends_on`, `network`, `security`, `contains` など

このモデルを VPC import と Terraform 可視化の両方で利用し、GUI ではこのグラフを表示する。

#### 6.2 VPC import CLI (`cmd/vpc-importer`)

- **構成**
  - `main.go`: CLI エントリポイント（`cobra` コマンド定義）
  - `runner`: フラグ解析・設定読み込み・ユースケース呼び出し
  - `usecase.ImportVpcResources`
    - AWS クライアント（`pkg/aws`）を呼び出してリソース列挙
    - `pkg/importer` を用いて Terraform 用 HCL & import コマンド生成

- **`pkg/aws`**
  - `VpcDiscoveryService`
    - `ListVpcs`, `ListSubnets(vpcId)`, `ListRouteTables(vpcId)`, `ListSecurityGroups(vpcId)`, `ListInstances(vpcId)` など
    - それぞれ AWS SDK を用いて API をコールし、内部 Resource モデルへ変換
  - 将来のクラウド追加に備え、`CloudDiscovery` インターフェースを定義（`ListResources(scope) ([]Resource, []Relation, error)`）。

- **`pkg/importer`**
  - `HclGenerator`
    - 内部 Resource モデルから Terraform HCL へのマッピング
    - リソースタイプごとにテンプレート or コードで HCL スニペットを生成
    - ファイルへの吐き出し戦略（タイプ別ファイル／VPC 単位ファイルなど）を設定可能
  - `ImportCommandGenerator`
    - `terraform import 'aws_instance.example' 'i-xxxx'` のようなコマンド列を生成
  - `ExistingConfigAnalyzer`
    - 既存 `.tf` を解析し、同一リソースがすでに定義済みかどうかを検知（競合検出）

#### 6.3 Terraform 可視化 GUI CLI (`cmd/tf-viz`)

- **構成**
  - `main.go`: CLI エントリポイント
  - `runner`:
    - `--tf-dir` や `--state-file` を受け取り、`TerraformLoader` を呼ぶ
    - `GraphBuilder` で内部リソースグラフを構築
    - `pkg/server` を起動し、GUI SPA を配信

- **`pkg/terraform`**
  - `ConfigLoader`
    - `.tf` / `.tf.json` を読み込み、モジュール・リソース定義を内部モデルへ変換
  - `StateLoader`
    - `.tfstate` または `terraform show -json` 出力をパースし、実リソース情報を内部モデルへ変換
  - `ResourceMapper`
    - Config/State の情報をマージし、1 つの Resource として統合（「定義 + 実体」）

- **`pkg/graph`**
  - `GraphBuilder`
    - `depends_on`, `provider`, `network 経路`, `セキュリティ関連` などのエッジ構築ロジック
  - `GraphQuery`
    - 種類・タグ・モジュール単位での検索／フィルタ API

- **`pkg/server`**
  - ローカル HTTP サーバ（`net/http` or `gin` 等）
  - エンドポイント例:
    - `GET /api/resources`: 全リソース一覧（ページング・フィルタ用クエリパラメータ付き）
    - `GET /api/resources/{id}`: 単一リソース詳細
    - `GET /api/graph`: グラフ全体（ノード＋エッジ）
    - `GET /api/graph/subset`: 部分グラフ（モジュール別など）
  - `/` 以下でフロントエンドのビルド済み静的ファイル（SPA）を配信。

#### 6.4 フロントエンド (`ui/tf-viz`)

- **技術**
  - React + TypeScript + Vite
  - 状態管理: React Query + コンテキスト程度（初期）
  - グラフ描画: 候補ライブラリのいずれか（PoC で選定）

- **主な画面**
  - グラフビュー
    - ノード: Resource
    - エッジ: Relation
    - ノードクリックで右ペインに詳細表示
  - リソース一覧ビュー
    - テーブル形式、検索・フィルタ（タイプ、モジュール、タグ 等）
  - 詳細ビュー
    - 基本情報（type, name, provider, module）
    - 属性（重要なもののみ整形表示）
    - 関連リソース一覧（グラフ上の近傍ノード）

### 7. 主なフロー

#### 7.1 VPC import フロー

1. ユーザーが `archae-vpc-import --vpc-id vpc-xxxx --region ap-northeast-1 --tf-dir ./infra` を実行。  
2. CLI が設定を解析し、`--tf-dir` に対応する Terraform workspace の state を確認する（`terraform state list` など）。  
   - 1 件以上のリソースが存在する場合は、VPC import 処理を行わずエラー終了とし、ユーザーに「空の state 専用ツール」である旨をメッセージとして伝える。  
3. state が空であることを確認できた場合にのみ、`AwsVpcDiscoveryService` を通じて AWS API からリソース一覧を取得。  
4. 取得したリソースを内部 Resource / Relation モデルに変換。  
5. `ExistingConfigAnalyzer` が `./infra` の既存 `.tf` を解析し、重複や競合をチェック（初期バージョンでは主に警告用途）。  
6. `HclGenerator` が新規用の HCL ファイルを生成し、`./infra/generated` 等へ出力。  
7. `ImportCommandGenerator` が import コマンドスクリプトを生成（`--apply` 指定時はその場で実行）。  
8. サマリを出力し、ユーザーにレビューと `terraform plan` 実行を促す。

#### 7.2 Terraform 可視化フロー

1. ユーザーが `archae-tf-viz --tf-dir ./infra` を実行。  
2. CLI が `ConfigLoader` / `StateLoader` を用いて Terraform 構成・状態を読み込む。  
3. `ResourceMapper` が両者を統合し、内部リソースグラフ（`Resource` + `Relation`）を構築。  
4. `pkg/server` がローカル HTTP サーバを起動し、API と SPA を提供。  
5. CLI がブラウザを自動で開き（`--no-open-browser` 時は URL のみ表示）、ユーザーがグラフを操作。  
6. フロントエンドは API を通じてリソース一覧・グラフ・詳細情報を取得し、インタラクティブに表示。

### 8. 拡張・今後の検討事項

- **クラウド対応拡張**
  - 抽象インターフェース（`CloudDiscovery`）に基づき、GCP, Azure 向け実装を追加可能にする。
- **リソースカバレッジ拡大**
  - 対応リソースタイプを段階的に増やし、VPC 内の観点だけでなくアカウント全体の可視化も検討。
- **ドリフト検知**
  - Terraform state と実インフラ（AWS API）を比較し、ドリフトを可視化する機能の追加。
- **コラボレーション機能（将来）**
  - 完全ローカル前提は維持しつつ、エクスポートされた JSON / PNG を共有できる仕組みなどを検討。


