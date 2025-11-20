package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ukms/archaeform/pkg/aws"
	"github.com/ukms/archaeform/pkg/terraform"
)

// stdLogger は pkg/aws.Logger を標準出力向けに実装した簡易ロガー。
type stdLogger struct{}

func (l *stdLogger) Infof(format string, args ...any) {
	fmt.Fprintf(os.Stdout, "[INFO] "+format+"\n", args...)
}

func (l *stdLogger) Warnf(format string, args ...any) {
	fmt.Fprintf(os.Stdout, "[WARN] "+format+"\n", args...)
}

func (l *stdLogger) Errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[ERROR] "+format+"\n", args...)
}

func main() {
	var (
		vpcID     string
		region    string
		profile   string
		tfDir     string
		apply     bool
		resFilter string
	)

	flag.StringVar(&vpcID, "vpc-id", "", "Target VPC ID (required)")
	flag.StringVar(&region, "region", "", "AWS region (required, or from AWS_REGION/AWS_DEFAULT_REGION)")
	flag.StringVar(&profile, "profile", "", "AWS profile name (optional)")
	flag.StringVar(&tfDir, "tf-dir", "", "Terraform configuration directory (required)")
	flag.BoolVar(&apply, "apply", false, "Execute terraform import automatically")
	flag.StringVar(&resFilter, "resource-filters", "", "Resource filter expression (e.g. type=aws_instance,tag:Env=prod)")

	flag.Parse()

	logger := &stdLogger{}

	if vpcID == "" {
		logger.Errorf("--vpc-id is required")
		os.Exit(1)
	}
	if region == "" {
		region = os.Getenv("AWS_REGION")
		if region == "" {
			region = os.Getenv("AWS_DEFAULT_REGION")
		}
	}
	if region == "" {
		logger.Errorf("--region or AWS_REGION/AWS_DEFAULT_REGION is required")
		os.Exit(1)
	}
	if tfDir == "" {
		logger.Errorf("--tf-dir is required")
		os.Exit(1)
	}

	scope := terraform.DiscoveryScope{
		VpcID:   vpcID,
		Region:  region,
		Profile: profile,
	}

	if resFilter != "" {
		f, err := terraform.ParseResourceFilter(resFilter)
		if err != nil {
			logger.Errorf("invalid --resource-filters value: %v", err)
			os.Exit(1)
		}
		scope.ResourceFilters = []terraform.ResourceFilter{f}
	}

	// TODO: 実際の AWS SDK クライアント実装を差し込む。
	var ec2 aws.Ec2API
	var elb aws.ElbAPI
	var rds aws.RdsAPI

	discovery := aws.NewAwsVpcDiscoveryService(ec2, elb, rds, logger)

	resources, relations, err := discovery.ListResources(scope)
	if err != nil {
		logger.Errorf("VPC discovery failed: %v", err)
		os.Exit(1)
	}

	logger.Infof("VPC discovery completed: resources=%d relations=%d", len(resources), len(relations))

	// TODO: F-02 以降の処理:
	//   - ExistingConfigAnalyzer による既存 .tf 解析
	//   - HclGenerator による HCL 生成
	//   - ImportCommandGenerator による import スクリプト生成
	//   - --apply 時の terraform import 実行
	_ = tfDir
	_ = apply
}


