package code

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/OpenRAE/go-rae-archcheck/internal/finding"
	"github.com/OpenRAE/go-rae-archcheck/internal/model"
)

// Check compares the code with the model. The findings about the model
// itself name location, the path of the model file.
func Check(m *model.Model, location string, g Graph) []finding.Finding {
	c := checker{model: m, graph: g, location: location}
	c.packages = map[string]bool{}
	for _, p := range g.Packages {
		c.packages[p.Path] = true
	}
	c.checkModule()
	for _, name := range g.Unread {
		c.add(finding.Finding{
			Location: name,
			Rule:     finding.UnreadPath,
			Message:  "the check cannot read this path, so the model does not apply to it",
			Fix:      "Remove the build constraint or the cgo code, or make the directory readable.",
		})
	}
	c.checkOwners()
	for _, p := range g.Packages {
		c.checkPackage(p)
	}
	return finding.Sort(c.findings)
}

// checker collects the findings of one comparison of the code with the model.
type checker struct {
	model    *model.Model
	graph    Graph
	location string
	packages map[string]bool
	findings []finding.Finding
}

func (c *checker) add(f finding.Finding) {
	c.findings = append(c.findings, f)
}

func (c *checker) checkModule() {
	if c.graph.Module != c.model.Module {
		c.add(finding.Finding{
			Location: c.location,
			Rule:     finding.ModulePath,
			Message:  fmt.Sprintf("the model describes module %q, but the go.mod file states %q", c.model.Module, c.graph.Module),
			Fix:      "Make the module path of the model and of the go.mod file the same.",
		})
	}
}

// checkOwners reports packages that no component owns, once the model has
// components, and component patterns that match no package.
func (c *checker) checkOwners() {
	if len(c.model.Components) == 0 {
		return
	}
	for _, p := range c.graph.Packages {
		if _, ok := c.owner(p.Path); !ok {
			c.add(finding.Finding{
				Location: c.location,
				Rule:     finding.Owner,
				Message:  fmt.Sprintf("package %q has no component", p.Path),
				Fix:      "Add the package to the packages of a component in a design step.",
			})
		}
	}
	for _, comp := range c.model.Components {
		for _, pattern := range comp.Packages {
			if !slices.ContainsFunc(c.graph.Packages, func(p Package) bool { return model.MatchPattern(pattern, p.Path) }) {
				c.add(finding.Finding{
					Location: c.location,
					Rule:     finding.UnknownPackage,
					Message:  fmt.Sprintf("component %q owns %q, but the code has no such package", comp.ID, pattern),
					Fix:      "Write the package, or remove it from the component in a design step.",
				})
			}
		}
	}
}

func (c *checker) checkPackage(p Package) {
	layer, ok := model.LayerOf(c.model, p.Path)
	if !ok {
		if len(p.Files) > 0 {
			c.add(finding.Finding{
				Location: p.Files[0].Name,
				Rule:     finding.Layer,
				Message:  fmt.Sprintf("package %q sits in no layer directory", p.Path),
				Fix:      "Move the package into the directory of a layer, or add a layer for it in a design step.",
			})
		}
		return
	}
	owner, _ := c.owner(p.Path)
	support := c.testSupport(p.Path)
	for _, f := range p.Files {
		c.checkFile(layer, owner, support, f)
	}
}

// checkFile checks the imports and symbols of one file of a package in layer.
func (c *checker) checkFile(layer model.Layer, owner *model.Component, support bool, f File) {
	rules := c.rules(layer, owner, f.Test || support)
	for _, imp := range f.Imports {
		switch {
		case imp.Module == c.graph.Module:
			c.checkInternal(layer, owner, support, f, imp)
		case imp.Module != "":
			c.checkExternal(layer, owner, f, imp)
		}
		for _, r := range rules {
			if slices.ContainsFunc(r.ForbidImports, func(pattern string) bool { return model.MatchPattern(pattern, imp.Path) }) {
				c.add(ruleFinding(r, f, imp.Line, "import of "+strconv.Quote(imp.Path)))
			}
		}
	}
	for _, s := range f.Symbols {
		for _, r := range rules {
			if slices.Contains(r.ForbidSymbols, s.Name) {
				c.add(ruleFinding(r, f, s.Line, "use of "+s.Name))
			}
		}
	}
}

// checkInternal checks an import of another package of the module.
func (c *checker) checkInternal(layer model.Layer, owner *model.Component, support bool, f File, imp Import) {
	target, ok := model.LayerOf(c.model, imp.Path)
	if ok && target.ID != layer.ID && !slices.Contains(layer.Uses, target.ID) {
		c.add(finding.Finding{
			Location: location(f, imp.Line),
			Rule:     finding.LayerUse,
			Message:  fmt.Sprintf("layer %q imports %q of layer %q, which it does not use", layer.ID, imp.Path, target.ID),
			Fix:      fmt.Sprintf("Use a package of a layer that %q uses, or add %q to its uses in a design step.", layer.ID, target.ID),
		})
	}
	if c.testSupport(imp.Path) && !f.Test && !support {
		c.add(finding.Finding{
			Location: location(f, imp.Line),
			Rule:     finding.TestSupport,
			Message:  fmt.Sprintf("a non-test file imports the test support package %q", imp.Path),
			Fix:      "Import test support packages only from test files.",
		})
	}
	// A test file can use any test support package, which exists for the
	// tests of other components.
	if f.Test && c.testSupport(imp.Path) {
		return
	}
	targetOwner, ok := c.owner(imp.Path)
	if owner == nil || !ok || targetOwner.ID == owner.ID || slices.Contains(owner.DependsOn, targetOwner.ID) {
		return
	}
	c.add(finding.Finding{
		Location: location(f, imp.Line),
		Rule:     finding.ComponentDependency,
		Message:  fmt.Sprintf("component %q imports %q of component %q, but the model does not list that dependency", owner.ID, imp.Path, targetOwner.ID),
		Fix:      fmt.Sprintf("Use a component that %q depends on, or add the dependency in a design step.", owner.ID),
	})
}

// checkExternal checks an import of a package of another module.
func (c *checker) checkExternal(layer model.Layer, owner *model.Component, f File, imp Import) {
	testModule := f.Test && slices.Contains(c.model.TestModules, imp.Module)
	allowed := slices.ContainsFunc(layer.External, func(e model.External) bool {
		return model.MatchExternal(e.Path, imp.Path, imp.Module)
	})
	if !allowed && !testModule {
		c.add(finding.Finding{
			Location: location(f, imp.Line),
			Rule:     finding.External,
			Message:  fmt.Sprintf("layer %q imports %q from module %q, which no external entry of the layer allows", layer.ID, imp.Path, imp.Module),
			Fix:      "Add an external entry with a reason to the layer in a design step, or remove the import.",
		})
		return
	}
	if owner == nil || testModule || slices.Contains(owner.Modules, imp.Module) {
		return
	}
	c.add(finding.Finding{
		Location: location(f, imp.Line),
		Rule:     finding.ComponentModule,
		Message:  fmt.Sprintf("component %q imports %q from module %q, which the component does not list", owner.ID, imp.Path, imp.Module),
		Fix:      fmt.Sprintf("Add module %q to the modules of component %q in a design step.", imp.Module, owner.ID),
	})
}

// rules returns the rules that apply to a file of a package in layer. A test
// file, or any file of a test support package, gets only the rules that
// apply to tests.
func (c *checker) rules(layer model.Layer, owner *model.Component, test bool) []model.Rule {
	var rules []model.Rule
	for _, r := range c.model.Rules {
		if !slices.Contains(r.Scope.Layers, layer.ID) || test && !r.Scope.TestFiles {
			continue
		}
		if owner != nil && slices.Contains(r.ExceptComponents, owner.ID) {
			continue
		}
		rules = append(rules, r)
	}
	return rules
}

// owner returns the component whose package patterns match pkg.
func (c *checker) owner(pkg string) (*model.Component, bool) {
	for i := range c.model.Components {
		comp := &c.model.Components[i]
		if slices.ContainsFunc(comp.Packages, func(pattern string) bool { return model.MatchPattern(pattern, pkg) }) {
			return comp, true
		}
	}
	return nil, false
}

func (c *checker) testSupport(pkg string) bool {
	return slices.ContainsFunc(c.model.TestSupport, func(pattern string) bool { return model.MatchPattern(pattern, pkg) })
}

func ruleFinding(r model.Rule, f File, line int, what string) finding.Finding {
	return finding.Finding{
		Location: location(f, line),
		Rule:     r.ID,
		Message:  fmt.Sprintf("%s breaks the rule: %s", what, strings.TrimSpace(r.Description)),
		Fix:      strings.TrimSpace(r.Fix),
	}
}

func location(f File, line int) string {
	return f.Name + ":" + strconv.Itoa(line)
}
