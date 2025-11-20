package terraform

import (
	"fmt"
	"strings"
)

// ParseResourceFilter は 1 つの --resource-filters 式を ResourceFilter に変換する。
// 例: "type=aws_instance,tag:Env=prod"
func ParseResourceFilter(expr string) (ResourceFilter, error) {
	f := ResourceFilter{
		TagFilters: make(map[string]string),
	}
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return f, nil
	}

	parts := strings.Split(expr, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "type=") {
			f.Type = strings.TrimPrefix(p, "type=")
			continue
		}
		if strings.HasPrefix(p, "tag:") {
			rest := strings.TrimPrefix(p, "tag:")
			kv := strings.SplitN(rest, "=", 2)
			if len(kv) != 2 {
				return ResourceFilter{}, fmt.Errorf("invalid tag filter: %q", p)
			}
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			if key == "" {
				return ResourceFilter{}, fmt.Errorf("empty tag key in: %q", p)
			}
			f.TagFilters[key] = val
			continue
		}
		return ResourceFilter{}, fmt.Errorf("unknown filter segment: %q", p)
	}
	return f, nil
}

// MatchResource は与えられた filters のいずれかに Resource がマッチするかを判定する。
// filters が空の場合は常に true を返す。
func MatchResource(filters []ResourceFilter, r Resource) bool {
	if len(filters) == 0 {
		return true
	}
	for _, f := range filters {
		if f.Type != "" && f.Type != r.Type {
			continue
		}
		if !matchTags(f.TagFilters, r.Labels) {
			continue
		}
		return true
	}
	return false
}

func matchTags(want map[string]string, labels map[string]string) bool {
	if len(want) == 0 {
		return true
	}
	if labels == nil {
		return false
	}
	for k, v := range want {
		if labels[k] != v {
			return false
		}
	}
	return true
}


