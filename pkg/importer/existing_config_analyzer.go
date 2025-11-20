package importer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ukms/archaeform/pkg/terraform"
)

// ExistingConfigIndex は既存 Terraform 構成中の resource ブロックをインデックスしたもの。
// F-04 詳細設計の ExistingConfigIndex に対応する。
type ExistingConfigIndex struct {
	Resources map[ResourceKey]ExistingResourceMeta
}

// ResourceKey は (provider, type, name) で一意に特定されるリソースキー。
type ResourceKey struct {
	Provider string
	Type     string
	Name     string
}

// ExistingResourceMeta は既存 .tf 内のリソース定義位置を表すメタ情報。
type ExistingResourceMeta struct {
	FilePath string
	Line     int
}

// ConflictedResource は import 対象リソースと既存構成との競合情報。
type ConflictedResource struct {
	Imported terraform.Resource
	Existing ExistingResourceMeta
}

// ExistingConfigAnalyzer は --tf-dir 配下の既存 Terraform 構成を解析し、
// 既存リソースのインデックスを構築するコンポーネント。
type ExistingConfigAnalyzer struct{}

// NewExistingConfigAnalyzer は ExistingConfigAnalyzer を生成する。
func NewExistingConfigAnalyzer() *ExistingConfigAnalyzer {
	return &ExistingConfigAnalyzer{}
}

// AnalyzeExistingConfigs は tfDir 以下の .tf ファイルを走査し、resource ブロックをインデックスする。
// 初期実装では HCL パーサを使わず、resource 行を正規表現で抽出する簡易実装とする。
func (a *ExistingConfigAnalyzer) AnalyzeExistingConfigs(tfDir string) (ExistingConfigIndex, error) {
	info, err := os.Stat(tfDir)
	if err != nil {
		return ExistingConfigIndex{}, fmt.Errorf("failed to stat tfDir %q: %w", tfDir, err)
	}
	if !info.IsDir() {
		return ExistingConfigIndex{}, fmt.Errorf("tfDir %q is not a directory", tfDir)
	}

	index := ExistingConfigIndex{
		Resources: make(map[ResourceKey]ExistingResourceMeta),
	}

	// resource "<type>" "<name>" { ... } を検出する簡易正規表現
	re := regexp.MustCompile(`^\s*resource\s+"([^"]+)"\s+"([^"]+)"`)

	err = filepath.WalkDir(tfDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".tf" {
			return nil
		}

		if err := a.indexFile(path, re, &index); err != nil {
			// 単一ファイルのパースエラーは WARN 相当だが、ここでは致命的ではないため継続する。
			// ログインターフェースはまだないため、エラー内容はまとめて上位に返すようにしてもよい。
			// ひとまずエラーをラップして返し、呼び出し側で扱ってもらう。
			return err
		}
		return nil
	})
	if err != nil {
		return ExistingConfigIndex{}, fmt.Errorf("failed to walk tfDir %q: %w", tfDir, err)
	}

	return index, nil
}

// indexFile は 1 つの .tf ファイルから resource ブロックの先頭行を検出し、インデックスに登録する。
func (a *ExistingConfigAnalyzer) indexFile(path string, re *regexp.Regexp, index *ExistingConfigIndex) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open %q: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		m := re.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}

		resourceType := m[1]
		name := m[2]

		provider := detectProviderFromType(resourceType)

		key := ResourceKey{
			Provider: provider,
			Type:     resourceType,
			Name:     name,
		}
		// 既に存在する場合は最初の定義を優先し、上書きしない。
		if _, exists := index.Resources[key]; !exists {
			index.Resources[key] = ExistingResourceMeta{
				FilePath: path,
				Line:     lineNum,
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan %q: %w", path, err)
	}
	return nil
}

// detectProviderFromType は resource type から provider 名を推測する。
// 例: "aws_instance" -> "aws"。区切り文字がない場合は空文字を返す。
func detectProviderFromType(resourceType string) string {
	for i := 0; i < len(resourceType); i++ {
		if resourceType[i] == '_' {
			return resourceType[:i]
		}
	}
	return ""
}

// FilterConflicted は import 対象 Resource 一覧を、既存構成との競合有無で分類する。
func (a *ExistingConfigAnalyzer) FilterConflicted(resources []terraform.Resource, index ExistingConfigIndex) (importable []terraform.Resource, conflicted []ConflictedResource) {
	for _, r := range resources {
		key := ResourceKey{
			Provider: r.Provider,
			Type:     r.Type,
			Name:     r.Name,
		}

		if meta, ok := index.Resources[key]; ok {
			conflicted = append(conflicted, ConflictedResource{
				Imported: r,
				Existing: meta,
			})
		} else {
			importable = append(importable, r)
		}
	}

	return importable, conflicted
}


