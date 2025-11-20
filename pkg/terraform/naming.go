package terraform

import (
	"regexp"
	"strconv"
	"strings"
)

// NameGenerator は Terraform のリソース論理名を生成するためのインターフェース。
// F-02 詳細設計の NameGenerator に対応する。
type NameGenerator interface {
	// Generate はリソースタイプ・ラベル・クラウド固有 ID から
	// HCL 上で利用可能な論理名を生成する。
	Generate(resourceType string, labels map[string]string, id string) string
}

// DefaultNameGenerator はシンプルなデフォルト実装。
// - Name タグがあればそれをベースにする
// - なければ <type>_<short-id> 形式
// - 衝突時は _1, _2 ... のサフィックスを付与
type DefaultNameGenerator struct {
	used map[string]int
}

// NewDefaultNameGenerator は DefaultNameGenerator を生成する。
func NewDefaultNameGenerator() *DefaultNameGenerator {
	return &DefaultNameGenerator{
		used: make(map[string]int),
	}
}

// Generate は Terraform の識別子として利用可能な論理名を生成する。
func (g *DefaultNameGenerator) Generate(resourceType string, labels map[string]string, id string) string {
	base := ""
	if labels != nil {
		if name, ok := labels["Name"]; ok && name != "" {
			base = name
		}
	}

	if base == "" {
		// ID が "i-0123456789abcdef0" のような場合、末尾 8〜12 文字程度を short-id として利用
		shortID := id
		if len(shortID) > 12 {
			shortID = shortID[len(shortID)-12:]
		}
		base = resourceType + "_" + shortID
	}

	sanitized := sanitizeTerraformIdentifier(base)

	// 衝突回避
	if count, ok := g.used[sanitized]; ok {
		count++
		g.used[sanitized] = count
		return sanitized + "_" + itoa(count)
	}

	g.used[sanitized] = 0
	return sanitized
}

// sanitizeTerraformIdentifier は Terraform 論理名として利用可能な識別子へ正規化する。
// - 英数字とアンダースコア以外は '_' に置換
// - 先頭が数字の場合は 'r_' を付加
// - すべて小文字に変換
func sanitizeTerraformIdentifier(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "resource"
	}

	s = strings.ToLower(s)

	// 英数字とアンダースコア以外を '_' にする
	re := regexp.MustCompile(`[^a-z0-9_]+`)
	s = re.ReplaceAllString(s, "_")

	// 先頭が数字の場合は prefix を付与
	if len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
		s = "r_" + s
	}

	// 連続する '_' を 1 つにまとめる
	s = strings.Trim(s, "_")
	if s == "" {
		return "resource"
	}
	s = regexp.MustCompile(`_+`).ReplaceAllString(s, "_")

	return s
}

// itoa は小さな整数を文字列化するヘルパー（strconv.Itoa の薄いラッパ）。
// strconv を直接使ってもよいが、依存をまとめるために分離している。
func itoa(i int) string {
	return strconv.Itoa(i)
}


