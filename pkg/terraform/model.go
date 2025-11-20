package terraform

// Origin は Resource がどのソースから来たかを表す。
// 例: AWS API（cloud）、Terraform の .tf（terraform_config）、tfstate（terraform_state）など。
type Origin string

const (
	OriginCloud           Origin = "cloud"
	OriginTerraformConfig Origin = "terraform_config"
	OriginTerraformState  Origin = "terraform_state"
)

// RelationKind はリソース間の関係種別を表す。
type RelationKind string

const (
	RelationDependsOn RelationKind = "depends_on"
	RelationNetwork   RelationKind = "network"
	RelationSecurity  RelationKind = "security"
	RelationContains  RelationKind = "contains"
)

// Resource はクラウド / Terraform 双方で利用する共通リソースモデル。
// system_design.md / vpc_import_basic_design.md に記載のフィールド構成に対応する。
type Resource struct {
	ID         string            `json:"id"`                   // 内部一意 ID (例: "aws:aws_instance:web_server")
	Provider   string            `json:"provider"`             // 例: "aws"
	Type       string            `json:"type"`                 // 例: "aws_instance"
	Name       string            `json:"name"`                 // Terraform 論理名候補
	Labels     map[string]string `json:"labels,omitempty"`     // タグやメタデータ ("Name", "Env" など)
	Attributes map[string]any    `json:"attributes,omitempty"` // 追加属性（初期は汎用マップ）
	Origin     Origin            `json:"origin"`               // 由来 (cloud / terraform_config / terraform_state)
}

// Relation は 2 つの Resource 間の関係を表す。
type Relation struct {
	From string       `json:"from"` // Resource.ID
	To   string       `json:"to"`   // Resource.ID
	Kind RelationKind `json:"kind"`
}

// ResourceFilter は import 対象とするリソースタイプやタグのフィルタ条件を表す。
// vpc_import_basic_design.md の ResourceFilter に対応。
type ResourceFilter struct {
	Type       string            `json:"type,omitempty"`       // 例: "aws_instance"
	TagFilters map[string]string `json:"tagFilters,omitempty"` // 例: {"Env": "prod"}
}

// DiscoveryScope はクラウド側リソース列挙のスコープを表す。
// F-01 詳細設計の DiscoveryScope に対応。
type DiscoveryScope struct {
	VpcID          string           `json:"vpcId"`
	Region         string           `json:"region"`
	Profile        string           `json:"profile,omitempty"`
	ResourceFilters []ResourceFilter `json:"resourceFilters,omitempty"`
}


