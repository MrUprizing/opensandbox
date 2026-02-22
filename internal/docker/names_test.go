package docker

import (
	"strings"
	"testing"
)

func TestGenerateName_Format(t *testing.T) {
	name := generateName()
	parts := strings.SplitN(name, "-", 2)
	if len(parts) != 2 {
		t.Fatalf("expected adjective-surname, got %q", name)
	}
	if parts[0] == "" || parts[1] == "" {
		t.Fatalf("empty part in %q", name)
	}
}

func TestGenerateName_NoUnderscores(t *testing.T) {
	for range 100 {
		name := generateName()
		if strings.Contains(name, "_") {
			t.Fatalf("name contains underscore: %q", name)
		}
	}
}

func TestGenerateName_NoBoringWozniak(t *testing.T) {
	for range 10000 {
		if generateName() == "boring-wozniak" {
			t.Fatal("generated boring-wozniak")
		}
	}
}

func TestGenerateName_Unique(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		seen[generateName()] = true
	}
	if len(seen) < 500 {
		t.Fatalf("expected high uniqueness, got only %d unique names from 1000", len(seen))
	}
}

func TestGenerateUniqueName_SkipsExisting(t *testing.T) {
	taken := map[string]bool{}
	for range 20 {
		taken[generateName()] = true
	}
	name := generateUniqueName(func(n string) bool { return taken[n] })
	if taken[name] {
		t.Fatalf("returned existing name: %q", name)
	}
}

func TestGenerateUniqueName_FallbackSuffix(t *testing.T) {
	calls := 0
	name := generateUniqueName(func(n string) bool {
		calls++
		return calls <= 10
	})
	// Should have a 4-digit suffix (3+ parts when split by hyphen).
	parts := strings.Split(name, "-")
	if len(parts) < 3 {
		t.Fatalf("expected suffixed name, got %q", name)
	}
}
