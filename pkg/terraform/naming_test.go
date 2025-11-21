package terraform

import "testing"

func TestDefaultNameGenerator_FromNameTag(t *testing.T) {
	ng := NewDefaultNameGenerator()

	labels := map[string]string{"Name": "Web-Server 01"}
	name := ng.Generate("aws_instance", labels, "i-0123456789abcdef0")

	if name != "web_server_01" {
		t.Fatalf("expected name %q, got %q", "web_server_01", name)
	}
}

func TestDefaultNameGenerator_FallbackToTypeAndShortID(t *testing.T) {
	ng := NewDefaultNameGenerator()

	labels := map[string]string{}
	name := ng.Generate("aws_instance", labels, "i-0123456789abcdef0")

	if name == "" {
		t.Fatalf("expected non-empty name")
	}
	if len(name) == 0 || name[0] < 'a' || name[0] > 'z' {
		t.Fatalf("expected name to start with a letter, got %q", name)
	}
}

func TestDefaultNameGenerator_DuplicateNamesGetSuffix(t *testing.T) {
	ng := NewDefaultNameGenerator()

	labels := map[string]string{"Name": "app"}
	n1 := ng.Generate("aws_instance", labels, "i-1")
	n2 := ng.Generate("aws_instance", labels, "i-2")
	n3 := ng.Generate("aws_instance", labels, "i-3")

	if n1 != "app" {
		t.Fatalf("expected first name to be %q, got %q", "app", n1)
	}
	if n2 != "app_1" || n3 != "app_2" {
		t.Fatalf("expected subsequent names to be app_1, app_2; got %q, %q", n2, n3)
	}
}


