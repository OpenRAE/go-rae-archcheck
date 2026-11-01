package model

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/OpenRAE/go-rae-archcheck/internal/finding"
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
	m, findings, err := Parse([]byte(data), "model.yaml")
	if err != nil || len(findings) != 0 {
		t.Fatalf("Parse() = %v, %v, want a model", findings, err)
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
		{name: "two documents", data: validYAML + "---\nversion: 1\n"},
		{name: "no document", data: ""},
		{name: "syntax", data: "version: [1\n"},
		{name: "value that JSON cannot hold", data: "version: .inf\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := Parse([]byte(tc.data), "model.yaml"); err == nil {
				t.Errorf("Parse(%q) error = nil, want an error", tc.data)
			}
		})
	}
}

// TestParseSchema checks that the schema rejects each structural defect, at
// the JSON pointer of the value.
func TestParseSchema(t *testing.T) {
	tests := []struct {
		name     string
		old      string
		new      string
		location string
	}{
		{"unknown field", "version: 1\n", "version: 1\nunknown: true\n", `""`},
		{"version", "version: 1", "version: 2", `"/version"`},
		{"version type", "version: 1", "version: one", `"/version"`},
		{"module pattern", "module: example.com/shop", "module: Example.com/Shop!", `"/module"`},
		{"system text", "  name: Shop\n", "  name: \"\"\n", `"/system/name"`},
		{"element id", "id: buyer", "id: Buyer", `"/people/0/id"`},
		{"element text", "    name: Bank\n", "    name: \"\"\n", `"/external_systems/0/name"`},
		{"container text", "    name: Server\n", "    name: \"\"\n", `"/containers/0/name"`},
		{"relationship text", "    description: Uses\n", "    description: \"\"\n", `"/relationships/0/description"`},
		{"layer id", "id: core\n", "id: Core\n", `"/layers/1/id"`},
		{"layer text", "    description: Rules of the shop.\n", "    description: \"\"\n", `"/layers/1/description"`},
		{"layer directory", "directory: internal/core", "directory: internal/Core/", `"/layers/1/directory"`},
		{"external reason", "        reason: Money values.\n", "        reason: \"\"\n", `"/layers/1/external/0/reason"`},
		{"component no container", "containers: [server]", "containers: []", `"/components/0/containers"`},
		{"component text", "    description: The cart.\n", "    description: \"\"\n", `"/components/0/description"`},
		{"component no package", "packages: [example.com/shop/internal/core/cart]", "packages: []", `"/components/0/packages"`},
		{"component dependency twice", "depends_on: []", "depends_on: [cart, cart]", `"/components/0/depends_on"`},
		{"test module twice", "test_modules: [github.com/google/go-cmp]", "test_modules: [github.com/google/go-cmp, github.com/google/go-cmp]", `"/test_modules"`},
		{"test support twice", "test_support: [example.com/shop/internal/core/coretest]", "test_support: [example.com/shop/internal/core/coretest, example.com/shop/internal/core/coretest]", `"/test_support"`},
		{"rule id", "id: no-exec", "id: NoExec", `"/rules/0/id"`},
		{"rule text", "    fix: Use an adapter job.\n", "    fix: \"\"\n", `"/rules/0/fix"`},
		{"rule no layer", "layers: [core, adapter]", "layers: []", `"/rules/0/scope/layers"`},
		{"rule symbol", "forbid_symbols: [os.StartProcess, panic]", "forbid_symbols: [\"os.Start Process\"]", `"/rules/0/forbid_symbols/0"`},
		{"rule nothing", "    forbid_imports: [os/exec]\n    forbid_symbols: [os.StartProcess, panic]\n", "", `"/rules/0"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(validYAML, tc.old) {
				t.Fatalf("validYAML does not contain %q", tc.old)
			}
			m, findings, err := Parse([]byte(strings.Replace(validYAML, tc.old, tc.new, 1)), "model.yaml")
			if err != nil || m != nil {
				t.Fatalf("Parse() = %v, %v, want schema findings and no model", m, err)
			}
			want := "the value at " + tc.location + " breaks the schema"
			if !hasMessage(findings, want) {
				t.Errorf("Parse() findings = %v, want one that contains %q", findings, want)
			}
		})
	}
}

func TestParseSchemaFindingFields(t *testing.T) {
	_, findings, err := Parse([]byte(strings.Replace(validYAML, "version: 1", "version: 2", 1)), "arch/model.yaml")
	if err != nil || len(findings) != 1 {
		t.Fatalf("Parse() = %v, %v, want one finding", findings, err)
	}
	if f := findings[0]; f.Location != "arch/model.yaml" || f.Rule != finding.Model || f.Fix == "" {
		t.Errorf("finding = %+v, want the model location, the model rule and a fix", f)
	}
}

func TestDecodeYAML(t *testing.T) {
	var v struct {
		Name string `yaml:"name"`
	}
	if err := DecodeYAML([]byte("name: a\n"), &v); err != nil || v.Name != "a" {
		t.Errorf("DecodeYAML() = %v with %q, want no error and a", err, v.Name)
	}
	for _, data := range []string{"name: a\nother: b\n", "name: a\n---\nname: b\n", ""} {
		if err := DecodeYAML([]byte(data), &v); err == nil {
			t.Errorf("DecodeYAML(%q) error = nil, want an error", data)
		}
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
