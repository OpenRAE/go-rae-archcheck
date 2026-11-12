package model

import (
	"strings"
	"testing"

	"github.com/OpenRAE/go-rae-archcheck/internal/finding"
)

func TestValidateValidModel(t *testing.T) {
	if got := Validate(mustParse(t, validYAML), "model.yaml"); len(got) != 0 {
		t.Errorf("Validate() = %v, want no findings", got)
	}
}

func TestValidateDefects(t *testing.T) {
	tests := []struct {
		name    string
		old     string
		new     string
		message string
	}{
		{"module path", "module: example.com/shop", "module: example.com/.shop", "module path is not valid"},
		{"duplicate id", "id: bank", "id: buyer", `identifier "buyer" is used more than once`},
		{"relationship end", "to: server", "to: nowhere", `relationship "buyer" -> "nowhere"`},
		{"layer overlap", "directory: internal/adapter", "directory: internal/core/adapter", "overlap"},
		{"layer uses unknown", "uses: [core, adapter]", "uses: [core, ghost]", `uses "ghost", which is not another layer`},
		{"layer uses itself", "uses: [core]\n    isolated", "uses: [adapter]\n    isolated", `uses "adapter", which is not another layer`},
		{"layer cycle", "    uses: []\n    external:\n      - path: example.com/money", "    uses: [adapter]\n    external:\n      - path: example.com/money", "use each other in a cycle"},
		{"external standard", "path: example.com/money/...", "path: net/http", "outside the standard library"},
		{"component container", "containers: [server]", "containers: [ghost]", `unknown container "ghost"`},
		{"component package outside", "packages: [example.com/shop/internal/core/cart]", "packages: [example.com/other/cart]", "is not an import path in module"},
		{"component two layers", "packages: [example.com/shop/internal/core/cart]", "packages: [example.com/shop/internal/core/cart, example.com/shop/internal/adapter/db]", "exactly one layer"},
		{"component dependency", "depends_on: []", "depends_on: [ghost]", `unknown component "ghost"`},
		{"component self dependency", "depends_on: []", "depends_on: [cart]", "depends on itself"},
		{"component module", "modules: [example.com/money]", "modules: [example.com/.money]", "is not valid"},
		{"test module", "test_modules: [github.com/google/go-cmp]", "test_modules: [example.com/.cmp]", `test module "example.com/.cmp" is not valid`},
		{"test support outside", "test_support: [example.com/shop/internal/core/coretest]", "test_support: [example.com/other/coretest]", "is not an import path in module"},
		{"rule layer", "layers: [core, adapter]", "layers: [core, ghost]", `unknown layer "ghost"`},
		{"rule import", "forbid_imports: [os/exec]", "forbid_imports: [os/exec.]", "invalid import pattern"},
		{"rule symbol", "forbid_symbols: [os.StartProcess, panic]", "forbid_symbols: [os.startProcess]", "invalid symbol"},
		{"rule builtin", "forbid_symbols: [os.StartProcess, panic]", "forbid_symbols: [nothing]", "invalid symbol"},
		{"rule except", "    fix: Use an adapter job.\n", "    fix: Use an adapter job.\n    except_components: [ghost]\n", `unknown component "ghost"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(validYAML, tc.old) {
				t.Fatalf("validYAML does not contain %q", tc.old)
			}
			m := mustParse(t, strings.Replace(validYAML, tc.old, tc.new, 1))
			got := Validate(m, "model.yaml")
			if !hasMessage(got, tc.message) {
				t.Errorf("Validate() = %v, want a finding that contains %q", got, tc.message)
			}
		})
	}
}

func TestValidateComponentGraph(t *testing.T) {
	m := mustParse(t, validYAML)
	m.Components = append(m.Components,
		Component{ID: "db", Containers: []string{"server"}, Description: "The store.", Packages: []string{"example.com/shop/internal/adapter/db"}, DependsOn: []string{"queue"}},
		Component{ID: "queue", Containers: []string{"server"}, Description: "The queue.", Packages: []string{"example.com/shop/internal/adapter/queue/..."}, DependsOn: []string{"db"}},
		Component{ID: "overlap", Containers: []string{"server"}, Description: "Overlaps.", Packages: []string{"example.com/shop/internal/adapter/queue/sub"}},
	)
	got := Validate(m, "model.yaml")
	for _, want := range []string{
		"depend on each other in a cycle",
		`component "db" depends on "queue" in isolated layer "adapter"`,
		"overlap",
	} {
		if !hasMessage(got, want) {
			t.Errorf("Validate() = %v, want a finding that contains %q", got, want)
		}
	}
}

func TestValidateDuplicates(t *testing.T) {
	m := mustParse(t, validYAML)
	m.Layers = append(m.Layers, Layer{ID: "core", Directory: "internal/extra", Description: "Again."})
	m.Layers[1].External = append(m.Layers[1].External, m.Layers[1].External[0])
	m.Rules = append(m.Rules, m.Rules[0])
	got := Validate(m, "model.yaml")
	for _, want := range []string{
		`layer identifier "core" is used more than once`,
		`lists external path "example.com/money/..." more than once`,
		`rule identifier "no-exec" is used more than once`,
	} {
		if !hasMessage(got, want) {
			t.Errorf("Validate() = %v, want a finding that contains %q", got, want)
		}
	}
}

func hasMessage(findings []finding.Finding, part string) bool {
	for _, f := range findings {
		if f.Rule == finding.Model && f.Location == "model.yaml" && strings.Contains(f.Message, part) {
			return true
		}
	}
	return false
}
