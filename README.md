# archaeform

Cloud Base infrastructure import to terraform

## archae-vpc-import (WIP)

指定した AWS VPC 内の既存リソースを列挙し、Terraform 管理下へ取り込むための情報を生成する CLI です。

現在は以下のスケルトン実装が含まれます。

- 共通リソースモデル (`pkg/terraform`)
  - `Resource`, `Relation`, `DiscoveryScope`, `ResourceFilter` など
- AWS VPC ディスカバリ (`pkg/aws`)
  - `CloudDiscovery` / `AwsVpcDiscoveryService` インターフェース
  - `awsVpcDiscoveryService` スケルトン実装（AWS SDK 連携は今後追加）
- CLI エントリポイント (`cmd/vpc-importer`)
  - フラグ:
    - `--vpc-id` (必須)
    - `--region` (必須, もしくは `AWS_REGION` / `AWS_DEFAULT_REGION`)
    - `--profile` (任意)
    - `--tf-dir` (必須)
    - `--apply` (任意, bool)
    - `--resource-filters` (任意)
  - 実行例:

    ```bash
    go run ./cmd/vpc-importer --vpc-id vpc-xxxx --region ap-northeast-1 --tf-dir ./infra
    ```

  - 現時点では VPC ディスカバリは空の結果を返し、今後のコミットで AWS SDK v2 連携や Terraform HCL 生成などを追加予定です。

