package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ukms/archaeform/pkg/terraform"
)

func TestImportCommandGenerator_GenerateImportScript_SkipsWithoutID(t *testing.T) {
	dir := t.TempDir()

	r1 := terraform.Resource{
		Type: "aws_instance",
		Name: "web_1",
		Attributes: map[string]any{
			"id": "i-1234",
		},
	}
	// ID を持たないリソース
	r2 := terraform.Resource{
		Type: "aws_subnet",
		Name: "subnet_public_a",
	}

	gen := NewImportCommandGenerator()
	cfg := ImportScriptConfig{
		TfDir: dir,
	}

	scriptPath, err := gen.GenerateImportScript([]terraform.Resource{r1, r2}, cfg)
	if err != nil {
		t.Fatalf("GenerateImportScript returned error: %v", err)
	}

	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("failed to read script: %v", err)
	}
	text := string(data)

	if !strings.Contains(text, "terraform import \"aws_instance.web_1\" \"i-1234\"") {
		t.Fatalf("expected import command for web_1, got:\n%s", text)
	}
	if strings.Contains(text, "aws_subnet.subnet_public_a") {
		t.Fatalf("expected subnet without ID to be skipped, got:\n%s", text)
	}
}


