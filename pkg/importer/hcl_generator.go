package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ukms/archaeform/pkg/terraform"
)

// SplitStrategy は HCL ファイルの分割戦略を表す。
// F-03 詳細設計の SplitStrategy に対応する。
type SplitStrategy string

const (
	// SplitByType はリソース Type ごとに 1 ファイルを生成する戦略（初期実装）。
	SplitByType SplitStrategy = "by_type"

	// SplitByModule / SplitSingle は将来拡張用。
	SplitByModule SplitStrategy = "by_module"
	SplitSingle   SplitStrategy = "single"
)

// HclGenerationConfig は HCL 生成の設定。
type HclGenerationConfig struct {
	TfDir         string
	OutputDir     string
	SplitStrategy SplitStrategy
}

// HclGenerationResult は HCL 生成結果のメタ情報。
type HclGenerationResult struct {
	OutputDir     string
	GeneratedFiles []string
	// Type ごとのリソース数を簡易的に保持する。
	ResourceCounts map[string]int
}

// HclGenerator は Resource / Relation から HCL ファイルを生成するコンポーネント。
type HclGenerator struct{}

// NewHclGenerator は HclGenerator を生成する。
func NewHclGenerator() *HclGenerator {
	return &HclGenerator{}
}

// Generate は与えられた Resource / Relation をもとに HCL ファイルを生成する。
func (g *HclGenerator) Generate(resources []terraform.Resource, relations []terraform.Relation, cfg HclGenerationConfig) (HclGenerationResult, error) {
	outputRoot := cfg.OutputDir
	if outputRoot == "" {
		if cfg.TfDir == "" {
			return HclGenerationResult{}, fmt.Errorf("TfDir is required when OutputDir is empty")
		}
		outputRoot = filepath.Join(cfg.TfDir, "generated")
	}

	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return HclGenerationResult{}, fmt.Errorf("failed to create output directory: %w", err)
	}

	if cfg.SplitStrategy == "" {
		cfg.SplitStrategy = SplitByType
	}

	switch cfg.SplitStrategy {
	case SplitByType:
		return g.generateByType(resources, relations, outputRoot)
	default:
		return HclGenerationResult{}, fmt.Errorf("split strategy %q is not supported yet", cfg.SplitStrategy)
	}
}

// generateByType はリソース Type ごとに 1 ファイルを生成する実装。
func (g *HclGenerator) generateByType(resources []terraform.Resource, relations []terraform.Relation, outputRoot string) (HclGenerationResult, error) {
	// Type ごとにグルーピング
	typeGrouped := make(map[string][]terraform.Resource)
	for _, r := range resources {
		typeGrouped[r.Type] = append(typeGrouped[r.Type], r)
	}

	// 安定した出力のため Type 名でソート
	var types []string
	for t := range typeGrouped {
		types = append(types, t)
	}
	sort.Strings(types)

	// Relation と Resource のインデックス
	relsByFrom := groupRelationsByFrom(relations)
	resByID := indexResourcesByID(resources)

	var generatedFiles []string
	resourceCounts := make(map[string]int)

	for _, t := range types {
		rs := typeGrouped[t]
		if len(rs) == 0 {
			continue
		}

		// Name でソートしておくと差分が安定しやすい
		sort.Slice(rs, func(i, j int) bool {
			return rs[i].Name < rs[j].Name
		})

		filename := fmt.Sprintf("%s.tf", t)
		path := filepath.Join(outputRoot, filename)

		f, err := os.Create(path)
		if err != nil {
			return HclGenerationResult{}, fmt.Errorf("failed to create HCL file %s: %w", path, err)
		}

		var b strings.Builder
		for _, r := range rs {
			block := buildResourceBlock(r, relsByFrom, resByID)
			b.WriteString(block)
			b.WriteString("\n\n")
			resourceCounts[t]++
		}

		if _, err := f.WriteString(strings.TrimSpace(b.String()) + "\n"); err != nil {
			_ = f.Close()
			return HclGenerationResult{}, fmt.Errorf("failed to write HCL file %s: %w", path, err)
		}
		if err := f.Close(); err != nil {
			return HclGenerationResult{}, fmt.Errorf("failed to close HCL file %s: %w", path, err)
		}

		generatedFiles = append(generatedFiles, path)
	}

	return HclGenerationResult{
		OutputDir:      outputRoot,
		GeneratedFiles: generatedFiles,
		ResourceCounts: resourceCounts,
	}, nil
}

// groupRelationsByFrom は From ID ごとの Relation 一覧を作る。
func groupRelationsByFrom(relations []terraform.Relation) map[string][]terraform.Relation {
	m := make(map[string][]terraform.Relation)
	for _, rel := range relations {
		m[rel.From] = append(m[rel.From], rel)
	}
	return m
}

// indexResourcesByID は Resource.ID から Resource へのマップを作る。
func indexResourcesByID(resources []terraform.Resource) map[string]terraform.Resource {
	m := make(map[string]terraform.Resource)
	for _, r := range resources {
		m[r.ID] = r
	}
	return m
}

// buildResourceBlock は 1 つの Resource から HCL の resource ブロック文字列を生成する。
func buildResourceBlock(r terraform.Resource, relsByFrom map[string][]terraform.Relation, resByID map[string]terraform.Resource) string {
	// Attributes をコピーしてから Relation に応じた参照解決を行う
	attrs := make(map[string]any, len(r.Attributes))
	for k, v := range r.Attributes {
		attrs[k] = v
	}

	applyRelationsToAttributes(r, attrs, relsByFrom, resByID)

	var b strings.Builder
	fmt.Fprintf(&b, "resource %q %q {\n", r.Type, r.Name)

	// キー順で安定させる
	var keys []string
	for k := range attrs {
		// nil 値は出力しない
		if attrs[k] == nil {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		val := attrs[key]
		// tags は特別扱い（map[string]string）
		if key == "tags" {
			if tags, ok := val.(map[string]string); ok {
				b.WriteString("  tags = {\n")

				// タグキーもソートしておく
				var tkeys []string
				for tk := range tags {
					tkeys = append(tkeys, tk)
				}
				sort.Strings(tkeys)
				for _, tk := range tkeys {
					tv := tags[tk]
					fmt.Fprintf(&b, "    %q = %q\n", tk, tv)
				}
				b.WriteString("  }\n")
				continue
			}
		}

		line := buildHCLAttributeLine(key, val)
		if line != "" {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("}")
	return b.String()
}

// applyRelationsToAttributes は Relation に応じて attributes 内の参照フィールドを
// HCLExpression に置き換える。
func applyRelationsToAttributes(r terraform.Resource, attrs map[string]any, relsByFrom map[string][]terraform.Relation, resByID map[string]terraform.Resource) {
	rels := relsByFrom[r.ID]
	if len(rels) == 0 {
		return
	}

	var sgExprs []terraform.HCLExpression

	for _, rel := range rels {
		target, ok := resByID[rel.To]
		if !ok {
			continue
		}

		switch rel.Kind {
		case terraform.RelationNetwork:
			// インスタンス -> サブネット の network 関係を subnet_id に反映
			if target.Type == "aws_subnet" {
				expr := terraform.HCLExpression(fmt.Sprintf("%s.%s.id", target.Type, target.Name))
				attrs["subnet_id"] = expr
			}
		case terraform.RelationSecurity:
			// インスタンス -> セキュリティグループ の security 関係を vpc_security_group_ids に反映
			if target.Type == "aws_security_group" {
				expr := terraform.HCLExpression(fmt.Sprintf("%s.%s.id", target.Type, target.Name))
				sgExprs = append(sgExprs, expr)
			}
		}
	}

	if len(sgExprs) > 0 {
		attrs["vpc_security_group_ids"] = sgExprs
	}
}

// buildHCLAttributeLine は 1 つの属性から HCL の 1 行を生成する。
func buildHCLAttributeLine(key string, val any) string {
	switch v := val.(type) {
	case string:
		return fmt.Sprintf("%s = %q", key, v)
	case bool:
		if v {
			return fmt.Sprintf("%s = true", key)
		}
		return fmt.Sprintf("%s = false", key)
	case int, int32, int64, float32, float64:
		return fmt.Sprintf("%s = %v", key, v)
	case terraform.HCLExpression:
		return fmt.Sprintf("%s = %s", key, string(v))
	case []string:
		var parts []string
		for _, s := range v {
			parts = append(parts, fmt.Sprintf("%q", s))
		}
		return fmt.Sprintf("%s = [%s]", key, strings.Join(parts, ", "))
	case []terraform.HCLExpression:
		var parts []string
		for _, e := range v {
			parts = append(parts, string(e))
		}
		return fmt.Sprintf("%s = [%s]", key, strings.Join(parts, ", "))
	default:
		// 対応していない型は一旦 fmt で文字列化してコメントとして出力する。
		// 将来的に必要に応じて拡張する。
		return fmt.Sprintf("// %s = %#v", key, v)
	}
}


