package importer

import (
	"bytes"
	"strings"
	"testing"
)

func TestImportSummary_WriteText_WithWarningsAndErrors(t *testing.T) {
	s := &ImportSummary{
		TotalResources:          10,
		ImportableResources:     8,
		ConflictedResources:     2,
		GeneratedHclFiles:       3,
		GeneratedImportCommands: 8,
		ApplyRequested:          true,
		ApplySucceeded:          7,
		ApplyFailed:             1,
		Warnings: []string{
			"RDS リソース取得に失敗したため import 対象外になりました",
		},
		Errors: []string{
			"aws_instance.web_1 の import に失敗しました",
		},
	}

	var buf bytes.Buffer
	if err := s.WriteText(&buf, "vpc-1234", "ap-northeast-1", "./infra/generated", "./infra/import.sh"); err != nil {
		t.Fatalf("WriteText returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Target VPC          : vpc-1234 (ap-northeast-1)") {
		t.Fatalf("expected Target VPC line in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Discovered resources: 10") {
		t.Fatalf("expected discovered resources line, got:\n%s", out)
	}
	if !strings.Contains(out, "Warnings:\n  - RDS リソース取得に失敗したため import 対象外になりました") {
		t.Fatalf("expected warnings section, got:\n%s", out)
	}
	if !strings.Contains(out, "Errors:\n  - aws_instance.web_1 の import に失敗しました") {
		t.Fatalf("expected errors section, got:\n%s", out)
	}
}


