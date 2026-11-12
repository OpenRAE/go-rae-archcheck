package schema

import (
	"math"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// validDocument returns a small model that meets the schema.
func validDocument() map[string]any {
	return map[string]any{
		"version":          1,
		"module":           "example.com/shop",
		"system":           map[string]any{"id": "shop", "name": "Shop", "description": "Sells things."},
		"people":           []any{},
		"external_systems": []any{},
		"relationships":    []any{},
		"containers": []any{
			map[string]any{"id": "server", "name": "Server", "technology": "Go", "description": "Serves the shop."},
		},
		"layers": []any{
			map[string]any{"id": "core", "directory": "internal/core", "description": "Rules.", "uses": []any{}, "external": []any{}},
		},
		"components":   []any{},
		"test_modules": []any{},
		"rules": []any{
			map[string]any{
				"id":             "no-panic",
				"description":    "No panics.",
				"scope":          map[string]any{"layers": []any{"core"}, "test_files": false},
				"forbid_symbols": []any{"panic", "os.Exit"},
				"fix":            "Return an error.",
			},
		},
	}
}

func TestValidateValid(t *testing.T) {
	violations, err := Validate(validDocument())
	if err != nil || len(violations) != 0 {
		t.Errorf("Validate() = %v, %v, want no violations", violations, err)
	}
}

func TestValidateViolations(t *testing.T) {
	doc := validDocument()
	doc["version"] = 2
	doc["extra"] = true
	doc["layers"] = []any{
		map[string]any{"id": "Core", "directory": "internal/core", "description": "Rules.", "uses": []any{}, "external": []any{}},
	}
	violations, err := Validate(doc)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	var got []string
	for _, v := range violations {
		got = append(got, v.Location)
		if v.Message == "" {
			t.Errorf("violation at %q has no message", v.Location)
		}
	}
	if diff := cmp.Diff([]string{"", "/layers/0/id", "/version"}, got); diff != "" {
		t.Errorf("violation locations (-want +got):\n%s", diff)
	}
}

func TestValidateMissingAndUnknown(t *testing.T) {
	violations, err := Validate(map[string]any{"version": 1, "unknown": true})
	if err != nil || len(violations) == 0 {
		t.Fatalf("Validate() = %v, %v, want violations", violations, err)
	}
	var messages []string
	for _, v := range violations {
		messages = append(messages, v.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{"missing properties", "unknown"} {
		if !strings.Contains(joined, want) {
			t.Errorf("violation messages = %q, want one that contains %q", joined, want)
		}
	}
}

func TestValidateNotJSON(t *testing.T) {
	if _, err := Validate(map[string]any{"version": math.Inf(1)}); err == nil {
		t.Errorf("Validate() of an infinite number: error = nil, want an error")
	}
}

func TestCompileErrors(t *testing.T) {
	for name, data := range map[string]string{
		"not JSON":       "{",
		"invalid schema": `{"type": 5}`,
	} {
		if _, err := compile([]byte(data)); err == nil {
			t.Errorf("compile(%s) error = nil, want an error", name)
		}
	}
}
