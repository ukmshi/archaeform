package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ukms/archaeform/pkg/terraform"
)

func TestHclGenerator_GenerateByType_WithRelations(t *testing.T) {
	dir := t.TempDir()

	subnet := terraform.Resource{
		ID:       "aws:aws_subnet:subnet-1234",
		Provider: "aws",
		Type:     "aws_subnet",
		Name:     "subnet_public_a",
		Attributes: map[string]any{
			"id":         "subnet-1234",
			"cidr_block": "10.0.1.0/24",
		},
	}
	instance := terraform.Resource{
		ID:       "aws:aws_instance:i-1234",
		Provider: "aws",
		Type:     "aws_instance",
		Name:     "web_1",
		Attributes: map[string]any{
			"id":                 "i-1234",
			"ami":                "ami-aaaa",
			"instance_type":      "t3.micro",
			"subnet_id":         "subnet-1234",
			"vpc_security_group_ids": []string{"sg-1"},
		},
	}

	resources := []terraform.Resource{subnet, instance}
	relations := []terraform.Relation{
		{
			From: instance.ID,
			To:   subnet.ID,
			Kind: terraform.RelationNetwork,
		},
	}

	gen := NewHclGenerator()
	cfg := HclGenerationConfig{
		TfDir:         dir,
		SplitStrategy: SplitByType,
	}

	result, err := gen.Generate(resources, relations, cfg)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if len(result.GeneratedFiles) == 0 {
		t.Fatalf("expected some generated files")
	}

	// aws_instance.tf が生成されていることを確認
	instanceFile := filepath.Join(result.OutputDir, "aws_instance.tf")
	data, err := os.ReadFile(instanceFile)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	text := string(data)

	// subnet_id がリソース参照に変換されていることを確認
	if !strings.Contains(text, `subnet_id = aws_subnet.subnet_public_a.id`) {
		t.Fatalf("expected subnet_id reference in HCL, got:\n%s", text)
	}
}


