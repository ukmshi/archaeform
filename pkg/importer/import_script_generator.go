package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ukms/archaeform/pkg/terraform"
)

// ImportScriptConfig は terraform import スクリプト生成の設定。
// F-05 詳細設計の ImportScriptConfig に対応する。
type ImportScriptConfig struct {
	TfDir      string // terraform 実行ディレクトリ (--tf-dir)
	ScriptName string // デフォルト "import.sh"
	Shell      string // "bash" を想定
}

// ImportCommandGenerator は Resource 一覧から terraform import コマンドスクリプトを生成するコンポーネント。
type ImportCommandGenerator struct{}

// NewImportCommandGenerator は ImportCommandGenerator を生成する。
func NewImportCommandGenerator() *ImportCommandGenerator {
	return &ImportCommandGenerator{}
}

// GenerateImportScript は与えられた Resource 一覧から terraform import コマンド列を生成し、
// シェルスクリプトとして出力する。
//
// 戻り値は生成したスクリプトファイルのパス。
func (g *ImportCommandGenerator) GenerateImportScript(resources []terraform.Resource, cfg ImportScriptConfig) (string, error) {
	if cfg.TfDir == "" {
		return "", fmt.Errorf("TfDir is required")
	}
	scriptName := cfg.ScriptName
	if scriptName == "" {
		scriptName = "import.sh"
	}
	shell := cfg.Shell
	if shell == "" {
		shell = "bash"
	}

	scriptPath := filepath.Join(cfg.TfDir, scriptName)

	f, err := os.Create(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to create import script %q: %w", scriptPath, err)
	}
	defer f.Close()

	var b strings.Builder

	// シバンと共通ヘッダ
	fmt.Fprintf(&b, "#!/usr/bin/env %s\n", shell)
	b.WriteString("set -euo pipefail\n\n")

	// --tf-dir へ移動
	// ユーザーが相対パスで指定した場合もそのまま使う。
	fmt.Fprintf(&b, "cd %q\n\n", cfg.TfDir)

	b.WriteString("# Generated terraform import commands\n")

	for _, r := range resources {
		address := fmt.Sprintf("%s.%s", r.Type, r.Name)
		importID, ok := resolveImportID(r)
		if !ok {
			// import ID が取れない場合はスキップ
			continue
		}
		fmt.Fprintf(&b, "terraform import %q %q\n", address, importID)
	}

	if _, err := f.WriteString(b.String()); err != nil {
		return "", fmt.Errorf("failed to write import script %q: %w", scriptPath, err)
	}

	// 実行権限付与
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		// 実行権限付与に失敗しても致命的ではないため、警告相当としてエラーにしない。
	}

	return scriptPath, nil
}

// resolveImportID は Resource から terraform import の ID を解決する。
// 初期実装では Attributes["id"] を最優先で利用し、なければ Labels["aws_id"] を試す。
func resolveImportID(r terraform.Resource) (string, bool) {
	if r.Attributes != nil {
		if v, ok := r.Attributes["id"]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s, true
			}
		}
	}
	if r.Labels != nil {
		if s, ok := r.Labels["aws_id"]; ok && s != "" {
			return s, true
		}
	}
	return "", false
}


