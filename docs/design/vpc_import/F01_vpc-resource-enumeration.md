# Archaeform VPC Import 詳細設計書 - F-01 VPC 内リソース列挙

## 1. 概要

- **対象機能**: F-01 VPC 内リソース列挙  
- **目的**: 指定された `--vpc-id`, `--region`（および `--profile`）を用いて、対象 VPC 内の AWS リソースを網羅的かつ効率的に列挙し、内部共通リソースモデルに渡すための生データを取得する。  
- **タスク優先度**: 1（最優先。後続の全機能の前提）

本機能は `pkg/aws.VpcDiscoveryService` を中心とした AWS SDK v2 を用いたリソース走査ロジックで構成される。

## 2. 対象コンポーネント

- `pkg/aws`
  - `type VpcDiscoveryService struct`
  - `type CloudDiscovery interface`
  - AWS SDK v2 用クライアントラッパ群（`Ec2Client`, `ElbClient`, `RdsClient` など）

## 3. インターフェース詳細

### 3.1 CloudDiscovery インターフェース

```go
type CloudDiscovery interface {
    ListResources(scope DiscoveryScope) ([]Resource, []Relation, error)
}
```

- **責務**: 任意クラウド上のスコープ（VPC など）に含まれるリソース一覧を返す抽象インターフェース。
- **本機能では** `AwsVpcDiscoveryService` がこれを実装する。

### 3.2 AwsVpcDiscoveryService インターフェース案

```go
type AwsVpcDiscoveryService interface {
    // VPC 自体およびネットワーク構成
    ListVpcs() ([]Resource, []Relation, error)
    ListSubnets(vpcID string) ([]Resource, []Relation, error)
    ListRouteTables(vpcID string) ([]Resource, []Relation, error)
    ListSecurityGroups(vpcID string) ([]Resource, []Relation, error)
    ListInternetGateways(vpcID string) ([]Resource, []Relation, error)
    ListNatGateways(vpcID string) ([]Resource, []Relation, error)

    // 代表的な常駐ワークロード
    ListInstances(vpcID string) ([]Resource, []Relation, error)
    ListLoadBalancers(vpcID string) ([]Resource, []Relation, error)
    ListRdsInstances(vpcID string) ([]Resource, []Relation, error)

    // VPC 内に配置されるマネージドサービス（初期フェーズから対象）
    ListEcsClusters(vpcID string) ([]Resource, []Relation, error)
    ListEcsServices(vpcID string) ([]Resource, []Relation, error)
    ListElastiCacheClusters(vpcID string) ([]Resource, []Relation, error)
    ListCodeBuildProjects(vpcID string) ([]Resource, []Relation, error)
    // 以外の VPC 内リソースも、サービスごとに ListXXX を追加していく
}
```

- 実装構造:

```go
type awsVpcDiscoveryService struct {
    ec2Client Ec2API
    elbClient ElbAPI
    rdsClient RdsAPI
    logger    Logger
}
```

## 4. 入出力仕様

### 4.1 入力 (`DiscoveryScope`)

- フィールド:
  - `VpcID: string`（必須）
  - `Region: string`（必須）
  - `Profile: string`（任意）
  - `ResourceFilters: []ResourceFilter`（任意。F-08 と連携）

### 4.2 出力

- `[]Resource`: AWS 生リソースを抽象化した共通モデル（データモデル詳細は別ドキュメント参照）  
- `[]Relation`: リソース間の関係（サブネットと VPC、EC2 とサブネット、セキュリティグループ関連など）  
- `error`: 重大な AWS API エラー／認証エラーなど

## 5. 処理フロー詳細

### 5.1 全体フロー（VPC 内列挙）

1. `ImportConfig` から `DiscoveryScope` を構築。
2. `AwsVpcDiscoveryService` の `ListResources(scope)` を呼び出す。
3. `ListResources` 内で以下の順序で各種列挙関数を実行する（**初期リリースでの対象リソース**）:
   1. VPC 存在確認（`DescribeVpcs`）
   2. サブネット列挙（`DescribeSubnets`）
   3. ルートテーブル列挙（`DescribeRouteTables`）
   4. IGW/NATGW 列挙（`DescribeInternetGateways`, `DescribeNatGateways`）
   5. セキュリティグループ列挙（`DescribeSecurityGroups`）
   6. EC2 インスタンス列挙（`DescribeInstances`）
   7. ALB/NLB 列挙（`DescribeLoadBalancers` 等）
   8. RDS インスタンス列挙（`DescribeDBInstances`）
4. 取得結果を内部 `Resource` / `Relation` に変換して返却。

#### 5.1.1 対象リソースのスコープ

- 初期フェーズのスコープは、**「対象 VPC に論理的に属する AWS リソース全般」** とする。  
  - 例: サブネット / ルートテーブル / IGW / NATGW / セキュリティグループ / ENI  
  - EC2 / ALB / NLB / RDS  
  - ECS / EKS / ElastiCache / CodeBuild / Lambda (VPC 接続) など、ENI を介して VPC 内に配置されるマネージドサービス
- 一方で、CloudFront・S3・Route53 などの **「VPC 外部に存在するグローバル／リージョナルサービス」** は本 CLI の対象外とする。
- 全 AWS サービスの網羅的列挙は現実的ではないため、実装上は以下の方針を取る:
  - VPC 内リソースはサービスごとに `ListXXX` を追加実装しつつ、`Resource` / `Relation` モデルはサービス非依存で扱えるようにする。
  - 新しい VPC 内サービスに対応する際も、`AwsVpcDiscoveryService` のメソッド追加とマッピング・HCL テンプレート追加で拡張できる。

### 5.2 並列化・ページング戦略

- AWS API の `MaxResults` を適切に設定しつつページング対応を行う。
- サブリソース（例: インスタンスタグ、ネットワークインターフェイス）は、可能なら 1 回の API でまとめて取得。
- 初期実装では **VPC 内でのサービス間並列化は行わず逐次実行** とし、パフォーマンス課題が顕在化した場合に `errgroup.Group` 等で並列化を検討する。

## 6. データマッピング方針（F-01 観点）

F-02 で行う本格的な内部モデルマッピングの前段として、F-01 では以下を行う。

- 各 AWS API レスポンスを中間構造体に詰める:

```go
type rawSubnet struct {
    ID        string
    CidrBlock string
    Name      string
    Tags      map[string]string
    Az        string
}
```

- 中間構造体は F-02 に渡しやすいよう「AWS 固有フィールド」を保持しつつも、`Resource` へ変換しやすい形に整理する。
- F-01 自体は `Resource` を直接組み立てず、「AWS -> 中間構造体」までを主責務とする設計もありうるが、初期実装ではコストを下げるため **F-01 内で `Resource` 生成まで行う**。

## 7. エラーハンドリング

- 認証エラー／権限エラー:
  - 最初の API コール（`DescribeVpcs` など）で検知した場合、即座に処理中断し `error` を返す。
- 部分的な API 失敗（特定サービスのみ失敗）:
  - 失敗したサービスのリソースはスキップしつつ WARN ログを出力。
  - `ImportSummary` に「取得失敗リソース種別」として記録できるようエラー情報を返却する。
- リトライ:
  - AWS SDK v2 の標準リトライポリシーに任せつつ、タイムアウトなどのグローバル設定はコンフィグ可能にする。

## 8. ログ出力

- `[INFO]`:
  - 列挙開始・終了（対象 VPC, リージョン, 主なリソース件数）。
- `[WARN]`:
  - 特定サービスの API 失敗（例: 「RDS API へのアクセスに失敗したため、RDS リソースは import 対象外になります」）。
- `[ERROR]`:
  - 認証失敗、VPC 不存在など致命的エラー。

## 9. テスト観点

- ユニットテスト:
  - `Ec2API`, `ElbAPI`, `RdsAPI` をインターフェース化し、モックを差し替えて VPC ごとのリソース数・構成に応じた挙動を検証。
- 結合テスト:
  - LocalStack 等で VPC / サブネット / インスタンスを作成し、期待どおりの `Resource` / `Relation` が列挙されるか確認。


