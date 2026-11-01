package model

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// validYAML is a small model that has no defect.
const validYAML = `version: 1
module: example.com/shop
system:
  id: shop
  name: Shop
  description: Sells things.
people:
  - id: buyer
    name: Buyer
    description: Buys things.
external_systems:
  - id: bank
    name: Bank
    description: Moves money.
relationships:
  - from: buyer
    to: server
    description: Uses
    technology: HTTPS
containers:
  - id: server
    name: Server
    technology: Go
    description: Serves the shop.
layers:
  - id: command
    directory: cmd/shop
    description: The main package.
    uses: [core, adapter]
    external: []
  - id: core
    directory: internal/core
    description: Rules of the shop.
    uses: []
    external:
      - path: example.com/money/...
        reason: Money values.
  - id: adapter
    directory: internal/adapter
    description: Outside systems.
    uses: [core]
    isolated: true
    external: []
components:
  - id: cart
    containers: [server]
    description: The cart.
    packages: [example.com/shop/internal/core/cart]
    depends_on: []
    modules: [example.com/money]
test_modules: [github.com/google/go-cmp]
test_support: [example.com/shop/internal/core/coretest]
rules:
  - id: no-exec
    description: No processes.
    scope:
      layers: [core, adapter]
      test_files: false
    forbid_imports: [os/exec]
    forbid_symbols: [os.StartProcess, panic]
    fix: Use an adapter job.
`

func mustParse(t *testing.T, data string) *Model {
	t.Helper()
	m, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return m
}

func TestParse(t *testing.T) {
	m := mustParse(t, validYAML)
	if got, want := m.Module, "example.com/shop"; got != want {
		t.Errorf("Module = %q, want %q", got, want)
	}
	if got, want := len(m.Layers), 3; got != want {
		t.Errorf("len(Layers) = %d, want %d", got, want)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "unknown field", data: "version: 1\nunknown: true\n"},
		{name: "two documents", data: "version: 1\n---\nversion: 1\n"},
		{name: "wrong type", data: "version: one\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.data)); err == nil {
				t.Errorf("Parse(%q) error = nil, want an error", tc.data)
			}
		})
	}
}

func TestStructureDropsText(t *testing.T) {
	a := mustParse(t, validYAML)
	b := mustParse(t, strings.NewReplacer(
		"Sells things.", "Sells goods.",
		"Money values.", "Amounts.",
		"No processes.", "Never start processes.",
		"Use an adapter job.", "Use a job.",
		"technology: HTTPS", "technology: TLS",
		"The cart.", "The basket.",
	).Replace(validYAML))
	if diff := cmp.Diff(a.Structure(), b.Structure()); diff != "" {
		t.Errorf("Structure() differs after text changes (-a +b):\n%s", diff)
	}
	if a.Structure().Layers[1].External[0].Reason != "" {
		t.Errorf("Structure() keeps an external reason")
	}
	c := mustParse(t, strings.Replace(validYAML, "uses: [core]\n    isolated", "uses: []\n    isolated", 1))
	if cmp.Equal(a.Structure(), c.Structure()) {
		t.Errorf("Structure() hides a change of layer uses")
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern, path string
		want          bool
	}{
		{"k8s.io/api", "k8s.io/api", true},
		{"k8s.io/api", "k8s.io/apiserver", false},
		{"k8s.io/api", "k8s.io/api/core", false},
		{"k8s.io/api/...", "k8s.io/api", true},
		{"k8s.io/api/...", "k8s.io/api/core/v1", true},
		{"k8s.io/api/...", "k8s.io/apiserver", false},
	}
	for _, tc := range tests {
		if got := MatchPattern(tc.pattern, tc.path); got != tc.want {
			t.Errorf("MatchPattern(%q, %q) = %t, want %t", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestMatchExternal(t *testing.T) {
	tests := []struct {
		pattern, path, mod string
		want               bool
	}{
		{"connectrpc.com/connect", "connectrpc.com/connect", "connectrpc.com/connect", true},
		{"connectrpc.com/connect", "connectrpc.com/connect/v2", "connectrpc.com/connect/v2", false},
		{"go.temporal.io/sdk/...", "go.temporal.io/sdk/workflow", "go.temporal.io/sdk", true},
		{"go.temporal.io/sdk/...", "go.temporal.io/sdk/contrib/otel", "go.temporal.io/sdk/contrib/otel", false},
		{"example.com/aws/aws/...", "example.com/aws/aws/retry", "example.com/aws", true},
		{"example.com/aws/service/s3/...", "example.com/aws/service/s3/types", "example.com/aws/service/s3", true},
		{"example.com/foo/...", "example.com/foo/v2/bar", "example.com/foo/v2", false},
	}
	for _, tc := range tests {
		if got := MatchExternal(tc.pattern, tc.path, tc.mod); got != tc.want {
			t.Errorf("MatchExternal(%q, %q, %q) = %t, want %t", tc.pattern, tc.path, tc.mod, got, tc.want)
		}
	}
}

func TestIsStandard(t *testing.T) {
	for path, want := range map[string]bool{"os": true, "net/http": true, "example.com/x": false, "golang.org/x/mod": false} {
		if got := IsStandard(path); got != want {
			t.Errorf("IsStandard(%q) = %t, want %t", path, got, want)
		}
	}
}

func TestLayerOf(t *testing.T) {
	m := mustParse(t, validYAML)
	tests := []struct {
		pkg   string
		layer string
		ok    bool
	}{
		{"example.com/shop/internal/core/cart", "core", true},
		{"example.com/shop/internal/core", "core", true},
		{"example.com/shop/cmd/shop", "command", true},
		{"example.com/shop/internal/other", "", false},
		{"example.com/shop", "", false},
		{"example.com/elsewhere/internal/core", "", false},
	}
	for _, tc := range tests {
		l, ok := LayerOf(m, tc.pkg)
		if ok != tc.ok || l.ID != tc.layer {
			t.Errorf("LayerOf(%q) = %q, %t, want %q, %t", tc.pkg, l.ID, ok, tc.layer, tc.ok)
		}
	}
}
