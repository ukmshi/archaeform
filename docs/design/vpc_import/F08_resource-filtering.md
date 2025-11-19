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


