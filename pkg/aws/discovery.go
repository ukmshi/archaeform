package aws

import (
	"context"

	"github.com/ukms/archaeform/pkg/terraform"
)

// CloudDiscovery は、任意クラウド上のスコープ（VPC など）に含まれる
// リソース一覧を返す抽象インターフェース。
// F-01 詳細設計の CloudDiscovery に対応する。
type CloudDiscovery interface {
	ListResources(scope terraform.DiscoveryScope) ([]terraform.Resource, []terraform.Relation, error)
}

// AwsVpcDiscoveryService は AWS VPC 内リソース列挙のためのインターフェース。
// F-01 詳細設計書のメソッド構成に対応する。
type AwsVpcDiscoveryService interface {
	// VPC 自体およびネットワーク構成
	ListVpcs(ctx context.Context) ([]terraform.Resource, []terraform.Relation, error)
	ListSubnets(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error)
	ListRouteTables(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error)
	ListSecurityGroups(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error)
	ListInternetGateways(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error)
	ListNatGateways(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error)

	// 代表的な常駐ワークロード
	ListInstances(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error)
	ListLoadBalancers(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error)
	ListRdsInstances(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error)

	// VPC 内に配置されるマネージドサービス（初期フェーズから対象）
	ListEcsClusters(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error)
	ListEcsServices(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error)
	ListElastiCacheClusters(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error)
	ListCodeBuildProjects(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error)
	ListLambdaFunctions(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error)
}

// Logger は F-01 で想定されている簡易ログインターフェース。
// 初期実装では最小限のメソッドのみ定義する。
type Logger interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// 各 AWS クライアントは AWS SDK v2 のラッパとして定義する想定。
// ここでは F-01 のテスト容易性のためにインターフェースのみ定義し、
// 実装は後続タスクで追加する。
type Ec2API interface {
	// TODO: DescribeVpcs, DescribeSubnets などを必要に応じて追加
}

type ElbAPI interface {
	// TODO: DescribeLoadBalancers などを必要に応じて追加
}

type RdsAPI interface {
	// TODO: DescribeDBInstances などを必要に応じて追加
}

// awsVpcDiscoveryService は AwsVpcDiscoveryService / CloudDiscovery のデフォルト実装。
type awsVpcDiscoveryService struct {
	ec2    Ec2API
	elb    ElbAPI
	rds    RdsAPI
	logger Logger
}

// NewAwsVpcDiscoveryService は AwsVpcDiscoveryService を生成する。
// CloudDiscovery としても利用できる。
func NewAwsVpcDiscoveryService(ec2 Ec2API, elb ElbAPI, rds RdsAPI, logger Logger) *awsVpcDiscoveryService {
	return &awsVpcDiscoveryService{
		ec2:    ec2,
		elb:    elb,
		rds:    rds,
		logger: logger,
	}
}

// ListResources は F-01 で定義された全体フローに従い、
// 各種 ListXXX を順次呼び出して結果を集約する。
// 初期実装では、まだ AWS API 連携を行わず、空の結果を返す。
func (s *awsVpcDiscoveryService) ListResources(scope terraform.DiscoveryScope) ([]terraform.Resource, []terraform.Relation, error) {
	ctx := context.Background()

	s.logger.Infof("Starting VPC discovery: vpc_id=%s region=%s", scope.VpcID, scope.Region)

	var allResources []terraform.Resource
	var allRelations []terraform.Relation

	// 1. VPC 存在確認および VPC リソース
	vpcs, vpcRels, err := s.ListVpcs(ctx)
	if err != nil {
		s.logger.Errorf("failed to list VPCs: %v", err)
		return nil, nil, err
	}
	allResources = append(allResources, vpcs...)
	allRelations = append(allRelations, vpcRels...)

	// 以降の呼び出しは、初期実装では「空実装」を想定。
	// 後続コミットで順次 AWS API 連携を追加していく。

	s.logger.Infof("Finished VPC discovery: resources=%d relations=%d", len(allResources), len(allRelations))

	return allResources, allRelations, nil
}

// ListVpcs は VPC 情報を列挙する。
// 初期実装では空のスライスを返し、後続タスクで AWS API 連携を追加する。
func (s *awsVpcDiscoveryService) ListVpcs(ctx context.Context) ([]terraform.Resource, []terraform.Relation, error) {
	// TODO: AWS EC2 DescribeVpcs を呼び出す実装を追加する。
	return []terraform.Resource{}, []terraform.Relation{}, nil
}

// 以下のメソッドも同様にプレースホルダ実装とし、
// 実際の AWS API 呼び出しは別コミットで行う。

func (s *awsVpcDiscoveryService) ListSubnets(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error) {
	return []terraform.Resource{}, []terraform.Relation{}, nil
}

func (s *awsVpcDiscoveryService) ListRouteTables(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error) {
	return []terraform.Resource{}, []terraform.Relation{}, nil
}

func (s *awsVpcDiscoveryService) ListSecurityGroups(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error) {
	return []terraform.Resource{}, []terraform.Relation{}, nil
}

func (s *awsVpcDiscoveryService) ListInternetGateways(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error) {
	return []terraform.Resource{}, []terraform.Relation{}, nil
}

func (s *awsVpcDiscoveryService) ListNatGateways(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error) {
	return []terraform.Resource{}, []terraform.Relation{}, nil
}

func (s *awsVpcDiscoveryService) ListInstances(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error) {
	return []terraform.Resource{}, []terraform.Relation{}, nil
}

func (s *awsVpcDiscoveryService) ListLoadBalancers(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error) {
	return []terraform.Resource{}, []terraform.Relation{}, nil
}

func (s *awsVpcDiscoveryService) ListRdsInstances(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error) {
	return []terraform.Resource{}, []terraform.Relation{}, nil
}

func (s *awsVpcDiscoveryService) ListEcsClusters(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error) {
	return []terraform.Resource{}, []terraform.Relation{}, nil
}

func (s *awsVpcDiscoveryService) ListEcsServices(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error) {
	return []terraform.Resource{}, []terraform.Relation{}, nil
}

func (s *awsVpcDiscoveryService) ListElastiCacheClusters(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error) {
	return []terraform.Resource{}, []terraform.Relation{}, nil
}

func (s *awsVpcDiscoveryService) ListCodeBuildProjects(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error) {
	return []terraform.Resource{}, []terraform.Relation{}, nil
}

func (s *awsVpcDiscoveryService) ListLambdaFunctions(ctx context.Context, vpcID string) ([]terraform.Resource, []terraform.Relation, error) {
	return []terraform.Resource{}, []terraform.Relation{}, nil
}


