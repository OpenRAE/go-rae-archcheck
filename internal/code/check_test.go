package code

import (
	"slices"
	"strings"
	"testing"

	"github.com/OpenRAE/go-rae-archcheck/internal/finding"
	"github.com/OpenRAE/go-rae-archcheck/internal/model"
)

// testModel is a model with three layers, two components, test support and
// two rules.
func testModel() *model.Model {
	return &model.Model{
		Version: 1,
		Module:  "example.com/shop",
		Layers: []model.Layer{
			{ID: "command", Directory: "cmd/shop", Uses: []string{"core", "adapter"}},
			{ID: "core", Directory: "internal/core", External: []model.External{
				{Path: "example.com/money/...", Reason: "Money."},
				{Path: "example.com/clock", Reason: "Clock."},
			}},
			{ID: "adapter", Directory: "internal/adapter", Uses: []string{"core"}},
		},
		Components: []model.Component{
			{ID: "cart", Packages: []string{"example.com/shop/internal/core/cart"}, Modules: []string{"example.com/money"}},
			{ID: "pay", Packages: []string{"example.com/shop/internal/core/pay"}, Modules: []string{"example.com/money"}},
			{ID: "store", Packages: []string{"example.com/shop/internal/adapter/..."}, DependsOn: []string{"cart"}},
			{ID: "app", Packages: []string{"example.com/shop/cmd/shop"}, DependsOn: []string{"cart", "store"}},
			{ID: "ghost", Packages: []string{"example.com/shop/internal/core/ghost"}},
			{ID: "support", Packages: []string{"example.com/shop/internal/core/coretest"}},
		},
		TestModules: []string{"github.com/google/go-cmp"},
		TestSupport: []string{"example.com/shop/internal/core/coretest"},
		Rules: []model.Rule{
			{ID: "no-env", Description: "No environment.", Fix: "Pass values.", Scope: model.Scope{Layers: []string{"core", "adapter"}}, ForbidSymbols: []string{"os.Getenv"}},
			{ID: "no-exec", Description: "No processes.", Fix: "Use a job.", Scope: model.Scope{Layers: []string{"core"}, TestFiles: true}, ForbidImports: []string{"os/exec"}, ExceptComponents: []string{"pay"}},
		},
	}
}

func imp(path, mod string, line int) Import {
	return Import{Path: path, Module: mod, Line: line}
}

const shop = "example.com/shop"

func TestCheck(t *testing.T) {
	g := Graph{
		Module: shop,
		Unread: []string{"internal/core/cart/linux.go"},
		Packages: []Package{
			{Path: shop + "/cmd/shop", Files: []File{{Name: "cmd/shop/main.go", Imports: []Import{
				imp(shop+"/internal/core/cart", shop, 3),
				imp(shop+"/internal/adapter/db", shop, 4),
				imp("os", "", 5),
			}}}},
			{Path: shop + "/internal/core/cart", Files: []File{
				{Name: "internal/core/cart/cart.go", Imports: []Import{
					imp("example.com/money/units", "example.com/money", 3),
					imp("example.com/money/v2/units", "example.com/money/v2", 4),
					imp("example.com/clock/zone", "example.com/clock", 5),
					imp(shop+"/internal/adapter/db", shop, 6),
					imp(shop+"/internal/core/pay", shop, 7),
					imp(shop+"/internal/core/coretest", shop, 8),
					imp("os/exec", "", 9),
				}, Symbols: []Symbol{{Name: "os.Getenv", Line: 10}}},
				{Name: "internal/core/cart/cart_test.go", Test: true, Imports: []Import{
					imp("github.com/google/go-cmp/cmp", "github.com/google/go-cmp", 3),
					imp(shop+"/internal/core/coretest", shop, 4),
					imp("os/exec", "", 5),
				}, Symbols: []Symbol{{Name: "os.Getenv", Line: 6}}},
			}},
			{Path: shop + "/internal/core/pay", Files: []File{{Name: "internal/core/pay/pay.go", Imports: []Import{
				imp("os/exec", "", 3),
				imp("example.com/other/x", "example.com/other", 4),
			}}}},
			{Path: shop + "/internal/core/coretest", Files: []File{{Name: "internal/core/coretest/coretest.go", Symbols: []Symbol{{Name: "os.Getenv", Line: 3}}}}},
			{Path: shop + "/internal/adapter/db", Files: []File{{Name: "internal/adapter/db/db.go", Imports: []Import{
				imp(shop+"/internal/core/cart", shop, 3),
			}}}},
			{Path: shop + "/internal/orphan", Files: []File{{Name: "internal/orphan/orphan.go"}}},
		},
	}
	got := Check(testModel(), "model.yaml", g)
	want := []struct {
		location, rule, message string
	}{
		{"internal/core/cart/linux.go", finding.UnreadPath, "cannot read this path"},
		{"model.yaml", finding.Owner, `package "example.com/shop/internal/orphan" has no component`},
		{"model.yaml", finding.UnknownPackage, `component "ghost" owns "example.com/shop/internal/core/ghost"`},
		{"internal/orphan/orphan.go", finding.Layer, "sits in no layer directory"},
		{"internal/core/cart/cart.go:4", finding.External, `imports "example.com/money/v2/units" from module "example.com/money/v2"`},
		{"internal/core/cart/cart.go:5", finding.External, `imports "example.com/clock/zone"`},
		{"internal/core/cart/cart.go:6", finding.ComponentDependency, `component "cart" imports "example.com/shop/internal/adapter/db" of component "store"`},
		{"internal/core/cart/cart.go:6", finding.LayerUse, `layer "core" imports "example.com/shop/internal/adapter/db" of layer "adapter"`},
		{"internal/core/cart/cart.go:7", finding.ComponentDependency, `component "cart" imports "example.com/shop/internal/core/pay" of component "pay"`},
		{"internal/core/cart/cart.go:8", finding.TestSupport, "test support package"},
		{"internal/core/cart/cart.go:8", finding.ComponentDependency, `of component "support"`},
		{"internal/core/cart/cart.go:9", "no-exec", `import of "os/exec" breaks the rule: No processes.`},
		{"internal/core/cart/cart.go:10", "no-env", "use of os.Getenv breaks the rule: No environment."},
		{"internal/core/cart/cart_test.go:5", "no-exec", `import of "os/exec"`},
		{"internal/core/pay/pay.go:4", finding.External, `imports "example.com/other/x"`},
	}
	for _, w := range want {
		if !hasFinding(got, w.location, w.rule, w.message) {
			t.Errorf("Check() has no %s finding at %s with %q", w.rule, w.location, w.message)
		}
	}
	if len(got) != len(want) {
		t.Errorf("Check() returned %d findings, want %d:\n%v", len(got), len(want), got)
	}
}

func TestCheckComponentModule(t *testing.T) {
	m := testModel()
	m.Components = m.Components[:4]
	m.Components[0].Modules = nil
	g := Graph{Module: shop, Packages: []Package{
		{Path: shop + "/internal/core/cart", Files: []File{{Name: "internal/core/cart/cart.go", Imports: []Import{
			imp("example.com/money/units", "example.com/money", 3),
		}}}},
		{Path: shop + "/internal/core/pay", Files: []File{{Name: "internal/core/pay/pay.go"}}},
		{Path: shop + "/internal/adapter/db", Files: []File{{Name: "internal/adapter/db/db.go"}}},
		{Path: shop + "/cmd/shop", Files: []File{{Name: "cmd/shop/main.go"}}},
	}}
	got := Check(m, "model.yaml", g)
	if len(got) != 1 || !hasFinding(got, "internal/core/cart/cart.go:3", finding.ComponentModule, `module "example.com/money"`) {
		t.Errorf("Check() = %v, want one component-module finding", got)
	}
}

func TestCheckModulePath(t *testing.T) {
	m := testModel()
	m.Components = nil
	got := Check(m, "model.yaml", Graph{Module: "example.com/other"})
	if len(got) != 1 || got[0].Rule != finding.ModulePath {
		t.Errorf("Check() = %v, want one module-path finding", got)
	}
}

func TestCheckWithoutComponents(t *testing.T) {
	m := testModel()
	m.Components = nil
	g := Graph{Module: shop, Packages: []Package{
		{Path: shop + "/internal/core/cart", Files: []File{{Name: "internal/core/cart/cart.go", Imports: []Import{
			imp("example.com/money/units", "example.com/money", 3),
			imp(shop+"/internal/core/pay", shop, 4),
		}}}},
	}}
	if got := Check(m, "model.yaml", g); len(got) != 0 {
		t.Errorf("Check() = %v, want no findings", got)
	}
}

func hasFinding(findings []finding.Finding, location, rule, message string) bool {
	return slices.ContainsFunc(findings, func(f finding.Finding) bool {
		return f.Location == location && f.Rule == rule && strings.Contains(f.Message, message)
	})
}
