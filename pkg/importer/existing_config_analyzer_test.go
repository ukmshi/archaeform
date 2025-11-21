package importer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ukms/archaeform/pkg/terraform"
)

func TestExistingConfigAnalyzer_AnalyzeExistingConfigsAndFilterConflicted(t *testing.T) {
	dir := t.TempDir()

	tf := `
resource "aws_instance" "web_1" {
}

resource "aws_subnet" "subnet_public_a" {
}
`
	tfPath := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(tfPath, []byte(tf), 0o644); err != nil {
		t.Fatalf("failed to write tf file: %v", err)
	}

	analyzer := NewExistingConfigAnalyzer()
	index, err := analyzer.AnalyzeExistingConfigs(dir)
	if err != nil {
		t.Fatalf("AnalyzeExistingConfigs returned error: %v", err)
	}
	if len(index.Resources) != 2 {
		t.Fatalf("expected 2 indexed resources, got %d", len(index.Resources))
	}

	// import 対象リソースを準備
	r1 := terraform.Resource{Provider: "aws", Type: "aws_instance", Name: "web_1"}
	r2 := terraform.Resource{Provider: "aws", Type: "aws_subnet", Name: "subnet_public_a"}
	r3 := terraform.Resource{Provider: "aws", Type: "aws_security_group", Name: "sg_web"}

	importable, conflicted := analyzer.FilterConflicted([]terraform.Resource{r1, r2, r3}, index)

	if len(conflicted) != 2 {
		t.Fatalf("expected 2 conflicted resources, got %d", len(conflicted))
	}
	if len(importable) != 1 || importable[0].Name != "sg_web" {
		t.Fatalf("expected 1 importable sg_web, got %#v", importable)
	}
}


