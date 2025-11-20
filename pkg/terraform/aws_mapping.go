package terraform

import "fmt"

// RawSubnet は F-01/F-02 間で利用するサブネットの中間構造体。
// F01_vpc-resource-enumeration.md の rawSubnet をベースに、VPC ID などを追加している。
type RawSubnet struct {
	ID        string
	VpcID     string
	CidrBlock string
	Name      string
	Tags      map[string]string
	Az        string
}

// RawInstance は EC2 インスタンス向けの中間構造体。
// F02_internal-resource-model-mapping.md の属性例を参考にしている。
type RawInstance struct {
	ID               string
	Ami              string
	InstanceType     string
	SubnetID         string
	SecurityGroupIDs []string
	Tags             map[string]string
}

// CloudResourceMapper はクラウド固有の生データから共通 Resource/Relation への
// マッピングを行うためのインターフェース。
// F-02 詳細設計の CloudResourceMapper に対応する。
type CloudResourceMapper interface {
	MapFromAws(raw interface{}) ([]Resource, []Relation, error)
}

// AwsToResourceMapper は AWS 固有の生データから共通 Resource/Relation への
// マッピングを行う具象実装。
// 初期実装では Subnet / EC2 Instance を対象とする。
type AwsToResourceMapper struct {
	nameGenerator NameGenerator
}

// NewAwsToResourceMapper は AwsToResourceMapper を生成する。
func NewAwsToResourceMapper(ng NameGenerator) *AwsToResourceMapper {
	if ng == nil {
		ng = NewDefaultNameGenerator()
	}
	return &AwsToResourceMapper{nameGenerator: ng}
}

// MapSubnet は RawSubnet 一覧から Resource / Relation を生成する。
// - Type: aws_subnet
// - Provider: aws
// - Origin: cloud
// - Relation: subnet -> vpc (kind=network)
func (m *AwsToResourceMapper) MapSubnet(subnets []RawSubnet, region string) ([]Resource, []Relation, error) {
	var resources []Resource
	var relations []Relation

	for _, s := range subnets {
		labels := map[string]string{}
		for k, v := range s.Tags {
			if labels == nil {
				labels = make(map[string]string)
			}
			labels[k] = v
		}
		// 追加メタデータ
		if labels == nil {
			labels = make(map[string]string)
		}
		if region != "" {
			labels["aws_region"] = region
		}
		if s.VpcID != "" {
			labels["vpc_id"] = s.VpcID
		}

		id := fmt.Sprintf("aws:aws_subnet:%s", s.ID)
		name := m.nameGenerator.Generate("aws_subnet", labels, s.ID)

		attr := map[string]any{
			"id":                     s.ID,
			"vpc_id":                 s.VpcID,
			"cidr_block":             s.CidrBlock,
			"availability_zone":      s.Az,
			"map_public_ip_on_launch": nil, // F-01/F-02 で必要に応じて拡張
			"tags":                   s.Tags,
		}

		res := Resource{
			ID:         id,
			Provider:   "aws",
			Type:       "aws_subnet",
			Name:       name,
			Labels:     labels,
			Attributes: attr,
			Origin:     OriginCloud,
		}
		resources = append(resources, res)

		// VPC との network 関係
		if s.VpcID != "" {
			vpcID := fmt.Sprintf("aws:aws_vpc:%s", s.VpcID)
			relations = append(relations, Relation{
				From: res.ID,
				To:   vpcID,
				Kind: RelationNetwork,
			})
		}
	}

	return resources, relations, nil
}

// MapInstance は RawInstance 一覧から Resource / Relation を生成する。
// - Type: aws_instance
// - Provider: aws
// - Origin: cloud
// - Relation:
//   - instance -> subnet (network)
//   - instance -> security_group (security) ※ SG 側の Resource.ID とは別途対応が必要
func (m *AwsToResourceMapper) MapInstance(instances []RawInstance, region string) ([]Resource, []Relation, error) {
	var resources []Resource
	var relations []Relation

	for _, inst := range instances {
		labels := map[string]string{}
		for k, v := range inst.Tags {
			if labels == nil {
				labels = make(map[string]string)
			}
			labels[k] = v
		}
		if labels == nil {
			labels = make(map[string]string)
		}
		if region != "" {
			labels["aws_region"] = region
		}

		id := fmt.Sprintf("aws:aws_instance:%s", inst.ID)

		name := m.nameGenerator.Generate("aws_instance", labels, inst.ID)

		attr := map[string]any{
			"id":                 inst.ID,
			"ami":                inst.Ami,
			"instance_type":      inst.InstanceType,
			"subnet_id":         inst.SubnetID,
			"vpc_security_group_ids": inst.SecurityGroupIDs,
			"tags":              inst.Tags,
		}

		res := Resource{
			ID:         id,
			Provider:   "aws",
			Type:       "aws_instance",
			Name:       name,
			Labels:     labels,
			Attributes: attr,
			Origin:     OriginCloud,
		}
		resources = append(resources, res)

		// Subnet との network 関係
		if inst.SubnetID != "" {
			subnetID := fmt.Sprintf("aws:aws_subnet:%s", inst.SubnetID)
			relations = append(relations, Relation{
				From: res.ID,
				To:   subnetID,
				Kind: RelationNetwork,
			})
		}

		// セキュリティグループとの security 関係
		for _, sgID := range inst.SecurityGroupIDs {
			if sgID == "" {
				continue
			}
			sgResID := fmt.Sprintf("aws:aws_security_group:%s", sgID)
			relations = append(relations, Relation{
				From: res.ID,
				To:   sgResID,
				Kind: RelationSecurity,
			})
		}
	}

	return resources, relations, nil
}


