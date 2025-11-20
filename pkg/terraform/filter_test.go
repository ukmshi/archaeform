package terraform

import "testing"

func TestParseResourceFilter_TypeAndTag(t *testing.T) {
	f, err := ParseResourceFilter("type=aws_instance,tag:Env=prod,tag:Owner=team-a")
	if err != nil {
		t.Fatalf("ParseResourceFilter returned error: %v", err)
	}
	if f.Type != "aws_instance" {
		t.Fatalf("expected Type=aws_instance, got %q", f.Type)
	}
	if len(f.TagFilters) != 2 || f.TagFilters["Env"] != "prod" || f.TagFilters["Owner"] != "team-a" {
		t.Fatalf("unexpected TagFilters: %#v", f.TagFilters)
	}
}

func TestMatchResource_WithTypeAndTags(t *testing.T) {
	f := ResourceFilter{
		Type: "aws_instance",
		TagFilters: map[string]string{
			"Env": "prod",
		},
	}

	r1 := Resource{
		Type: "aws_instance",
		Labels: map[string]string{
			"Env": "prod",
		},
	}

	r2 := Resource{
		Type: "aws_instance",
		Labels: map[string]string{
			"Env": "stg",
		},
	}

	if !MatchResource([]ResourceFilter{f}, r1) {
		t.Fatalf("expected r1 to match filter")
	}
	if MatchResource([]ResourceFilter{f}, r2) {
		t.Fatalf("expected r2 not to match filter")
	}
}


