package terraform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// このテストでは、実際の terraform バイナリの代わりに簡易シェルスクリプトを使い、
// DefaultTerraformExecutor が期待どおりのサブコマンドを呼び出しているかを確認する。
func TestDefaultTerraformExecutor_InitAndImport(t *testing.T) {
	dir := t.TempDir()

	// ログファイル
	logPath := filepath.Join(dir, "terraform.log")

	// 疑似 terraform スクリプトを作成
	scriptPath := filepath.Join(dir, "terraform")
	script := "#!/usr/bin/env sh\n" +
		"echo \"$@\" >> " + logPath + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake terraform script: %v", err)
	}

	execDir := t.TempDir()

	exec := &DefaultTerraformExecutor{
		TerraformBin: scriptPath,
	}

	if err := exec.Init(execDir); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if err := exec.Import(execDir, "aws_instance.web_1", "i-1234"); err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	log := string(data)
	if log == "" {
		t.Fatalf("expected log to be non-empty")
	}
	if expected := "init -input=false"; !containsLine(log, expected) {
		t.Fatalf("expected log to contain %q, got:\n%s", expected, log)
	}
	if expected := "import aws_instance.web_1 i-1234"; !containsLine(log, expected) {
		t.Fatalf("expected log to contain %q, got:\n%s", expected, log)
	}
}

func containsLine(all string, sub string) bool {
	lines := strings.Split(all, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == sub {
			return true
		}
	}
	return false
}


