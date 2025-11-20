package importer

import (
	"fmt"
	"io"
)

// ImportSummary は VPC import 全体の結果サマリを表す。
// F-07 詳細設計の ImportSummary に対応する。
type ImportSummary struct {
	TotalResources          int
	ImportableResources     int
	ConflictedResources     int
	GeneratedHclFiles       int
	GeneratedImportCommands int

	ApplyRequested bool
	ApplySucceeded int
	ApplyFailed    int

	Warnings []string
	Errors   []string
}

// WriteText は ImportSummary を人間が読みやすいテキストとして writer に出力する。
// 実際の CLI では os.Stdout に対して呼び出す想定。
func (s *ImportSummary) WriteText(w io.Writer, vpcID string, region string, hclOutputDir string, importScriptPath string) error {
	if w == nil {
		return fmt.Errorf("writer is nil")
	}

	fmt.Fprintln(w, "=== Archaeform VPC Import Summary ===")
	if vpcID != "" || region != "" {
		fmt.Fprintf(w, "Target VPC          : %s (%s)\n", vpcID, region)
	}
	fmt.Fprintf(w, "Discovered resources: %d\n", s.TotalResources)
	fmt.Fprintf(w, "Importable          : %d\n", s.ImportableResources)
	fmt.Fprintf(w, "Conflicted          : %d\n", s.ConflictedResources)
	fmt.Fprintln(w)

	if hclOutputDir != "" {
		fmt.Fprintf(w, "Generated HCL files : %d (under %s)\n", s.GeneratedHclFiles, hclOutputDir)
	} else {
		fmt.Fprintf(w, "Generated HCL files : %d\n", s.GeneratedHclFiles)
	}
	if importScriptPath != "" {
		fmt.Fprintf(w, "Import commands     : %d (%s)\n", s.GeneratedImportCommands, importScriptPath)
	} else {
		fmt.Fprintf(w, "Import commands     : %d\n", s.GeneratedImportCommands)
	}
	fmt.Fprintln(w)

	applyStatus := "disabled"
	if s.ApplyRequested {
		applyStatus = "enabled"
	}
	fmt.Fprintf(w, "Apply               : %s\n", applyStatus)
	if s.ApplyRequested {
		fmt.Fprintf(w, "  Succeeded         : %d\n", s.ApplySucceeded)
		fmt.Fprintf(w, "  Failed            : %d\n", s.ApplyFailed)
	}
	fmt.Fprintln(w)

	if len(s.Warnings) > 0 {
		fmt.Fprintln(w, "Warnings:")
		for _, msg := range s.Warnings {
			fmt.Fprintf(w, "  - %s\n", msg)
		}
	}

	if len(s.Errors) > 0 {
		if len(s.Warnings) > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, "Errors:")
		for _, msg := range s.Errors {
			fmt.Fprintf(w, "  - %s\n", msg)
		}
	}

	return nil
}


