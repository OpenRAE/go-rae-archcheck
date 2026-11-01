package model

import (
	"fmt"
	"go/token"
	"go/types"
	"path"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/mod/module"

	"github.com/OpenRAE/go-rae-archcheck/internal/finding"
)

// idPattern matches an identifier: lowercase ASCII words of letters and
// digits, joined by single hyphens, that start with a letter.
var idPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// directoryPattern matches a layer directory: lowercase slash-separated
// segments of letters joined by single hyphens.
var directoryPattern = regexp.MustCompile(`^[a-z]+(?:-[a-z]+)*(?:/[a-z]+(?:-[a-z]+)*)*$`)

// Validate returns the defects of the model. Each finding names location,
// the path of the model file.
func Validate(m *Model, location string) []finding.Finding {
	v := validator{model: m, location: location}
	v.checkHeader()
	v.checkElements()
	v.checkRelationships()
	v.checkLayers()
	v.checkComponents()
	v.checkTestLists()
	v.checkRules()
	return finding.Sort(v.findings)
}

// validator collects the defects of one model.
type validator struct {
	model    *Model
	location string
	findings []finding.Finding
}

func (v *validator) addf(format string, args ...any) {
	v.findings = append(v.findings, finding.Finding{
		Location: v.location,
		Rule:     finding.Model,
		Message:  fmt.Sprintf(format, args...),
		Fix:      "Correct the model in a design step, and record the reason in a design record.",
	})
}

func (v *validator) checkHeader() {
	if v.model.Version != Version {
		v.addf("version is %d, and the only supported version is %d", v.model.Version, Version)
	}
	if err := module.CheckPath(v.model.Module); err != nil {
		v.addf("the module path is not valid: %v", err)
	}
	if v.model.System.Name == "" || v.model.System.Description == "" {
		v.addf("the system needs a name and a description")
	}
}

// checkElements checks the identifiers of the C4 elements, which share one
// namespace, and the required text of each element.
func (v *validator) checkElements() {
	seen := map[string]bool{}
	claim := func(kind, id string) {
		switch {
		case !idPattern.MatchString(id):
			v.addf("%s identifier %q must be lowercase words joined by hyphens", kind, id)
		case seen[id]:
			v.addf("identifier %q is used more than once", id)
		}
		seen[id] = true
	}
	claim("system", v.model.System.ID)
	for _, e := range v.model.People {
		claim("person", e.ID)
		v.needText("person", e.ID, e.Name, e.Description)
	}
	for _, e := range v.model.ExternalSystems {
		claim("external system", e.ID)
		v.needText("external system", e.ID, e.Name, e.Description)
	}
	for _, c := range v.model.Containers {
		claim("container", c.ID)
		v.needText("container", c.ID, c.Name, c.Description)
	}
	for _, c := range v.model.Components {
		claim("component", c.ID)
	}
}

func (v *validator) needText(kind, id, name, description string) {
	if name == "" || description == "" {
		v.addf("%s %q needs a name and a description", kind, id)
	}
}

func (v *validator) checkRelationships() {
	ends := map[string]bool{v.model.System.ID: true}
	for _, e := range slices.Concat(v.model.People, v.model.ExternalSystems) {
		ends[e.ID] = true
	}
	for _, c := range v.model.Containers {
		ends[c.ID] = true
	}
	for _, r := range v.model.Relationships {
		if !ends[r.From] || !ends[r.To] || r.From == r.To {
			v.addf("relationship %q -> %q must connect two different elements or containers", r.From, r.To)
		}
		if r.Description == "" {
			v.addf("relationship %q -> %q needs a description", r.From, r.To)
		}
	}
}

func (v *validator) checkLayers() {
	ids, dirs := map[string]bool{}, map[string]string{}
	for _, l := range v.model.Layers {
		v.checkLayer(l, ids, dirs)
	}
	for _, l := range v.model.Layers {
		for _, used := range l.Uses {
			if !ids[used] || used == l.ID {
				v.addf("layer %q uses %q, which is not another layer", l.ID, used)
			}
		}
	}
	if cycle := v.layerCycle(); cycle != nil {
		v.addf("the layers %s use each other in a cycle", strings.Join(cycle, " -> "))
	}
}

// checkLayer checks one layer against the identifiers and directories of
// the layers before it, and then records its own.
func (v *validator) checkLayer(l Layer, ids map[string]bool, dirs map[string]string) {
	switch {
	case !idPattern.MatchString(l.ID):
		v.addf("layer identifier %q must be lowercase words joined by hyphens", l.ID)
	case ids[l.ID]:
		v.addf("layer identifier %q is used more than once", l.ID)
	}
	ids[l.ID] = true
	if l.Description == "" {
		v.addf("layer %q needs a description", l.ID)
	}
	if !directoryPattern.MatchString(l.Directory) {
		v.addf("layer %q has directory %q, which is not a clean relative path of lowercase words", l.ID, l.Directory)
	}
	for dir, other := range dirs {
		if Within(l.Directory, dir) || Within(dir, l.Directory) {
			v.addf("the directories of layers %q and %q overlap", other, l.ID)
		}
	}
	dirs[l.Directory] = l.ID
	v.checkExternal(l)
}

func (v *validator) checkExternal(l Layer) {
	seen := map[string]bool{}
	for _, e := range l.External {
		base := strings.TrimSuffix(e.Path, "/...")
		switch {
		case module.CheckImportPath(base) != nil || IsStandard(base):
			v.addf("layer %q has external path %q, which is not an import path outside the standard library", l.ID, e.Path)
		case seen[e.Path]:
			v.addf("layer %q lists external path %q more than once", l.ID, e.Path)
		case e.Reason == "":
			v.addf("layer %q lists external path %q with no reason", l.ID, e.Path)
		}
		seen[e.Path] = true
	}
}

// layerCycle returns the layers of one cycle in the uses graph, or nil.
func (v *validator) layerCycle() []string {
	uses := map[string][]string{}
	for _, l := range v.model.Layers {
		uses[l.ID] = l.Uses
	}
	return cycle(uses, layerIDs(v.model.Layers))
}

func (v *validator) checkComponents() {
	containers := map[string]bool{}
	for _, c := range v.model.Containers {
		containers[c.ID] = true
	}
	components := map[string]bool{}
	for _, c := range v.model.Components {
		components[c.ID] = true
	}
	var owned []string
	for _, c := range v.model.Components {
		v.checkComponent(c, containers, components)
		for _, p := range c.Packages {
			for _, other := range owned {
				if overlap(p, other) {
					v.addf("package patterns %q and %q overlap", other, p)
				}
			}
			owned = append(owned, p)
		}
	}
	deps := map[string][]string{}
	for _, c := range v.model.Components {
		deps[c.ID] = c.DependsOn
	}
	if cycle := cycle(deps, componentIDs(v.model.Components)); cycle != nil {
		v.addf("the components %s depend on each other in a cycle", strings.Join(cycle, " -> "))
	}
}

// checkComponent checks one component against the known containers and
// components.
func (v *validator) checkComponent(c Component, containers, components map[string]bool) {
	if len(c.Containers) == 0 {
		v.addf("component %q names no container", c.ID)
	}
	for _, id := range c.Containers {
		if !containers[id] {
			v.addf("component %q names unknown container %q", c.ID, id)
		}
	}
	if c.Description == "" {
		v.addf("component %q needs a description", c.ID)
	}
	if len(c.Packages) == 0 {
		v.addf("component %q owns no package", c.ID)
	}
	for _, p := range c.Packages {
		v.checkPackagePattern(fmt.Sprintf("package %q of component %q", p, c.ID), p)
	}
	v.checkLayerOf(c)
	v.checkDependsOn(c, components)
	v.checkModuleList(fmt.Sprintf("module of component %q", c.ID), c.Modules)
}

// checkLayerOf checks that the packages of a component sit in one layer, and
// that a component in an isolated layer depends on no component of that layer.
func (v *validator) checkLayerOf(c Component) {
	layer, ok := v.componentLayer(c)
	if !ok {
		v.addf("the packages of component %q must sit in exactly one layer", c.ID)
		return
	}
	if !layer.Isolated {
		return
	}
	for _, d := range c.DependsOn {
		other, found := v.component(d)
		if !found {
			continue
		}
		if l, ok := v.componentLayer(other); ok && l.ID == layer.ID {
			v.addf("component %q depends on %q in isolated layer %q", c.ID, d, layer.ID)
		}
	}
}

// componentLayer returns the one layer that holds every package of c.
func (v *validator) componentLayer(c Component) (Layer, bool) {
	var found *Layer
	for _, p := range c.Packages {
		l, ok := LayerOf(v.model, strings.TrimSuffix(p, "/..."))
		if !ok || (found != nil && found.ID != l.ID) {
			return Layer{}, false
		}
		found = &l
	}
	if found == nil {
		return Layer{}, false
	}
	return *found, true
}

func (v *validator) component(id string) (Component, bool) {
	i := slices.IndexFunc(v.model.Components, func(c Component) bool { return c.ID == id })
	if i < 0 {
		return Component{}, false
	}
	return v.model.Components[i], true
}

func (v *validator) checkDependsOn(c Component, components map[string]bool) {
	seen := map[string]bool{}
	for _, d := range c.DependsOn {
		switch {
		case !components[d]:
			v.addf("component %q depends on unknown component %q", c.ID, d)
		case d == c.ID:
			v.addf("component %q depends on itself", c.ID)
		case seen[d]:
			v.addf("component %q lists dependency %q more than once", c.ID, d)
		}
		seen[d] = true
	}
}

func (v *validator) checkPackagePattern(what, pattern string) {
	base := strings.TrimSuffix(pattern, "/...")
	if module.CheckImportPath(base) != nil || !Within(base, v.model.Module) {
		v.addf("%s is not an import path in module %q", what, v.model.Module)
	}
}

func (v *validator) checkModuleList(what string, modules []string) {
	seen := map[string]bool{}
	for _, m := range modules {
		if err := module.CheckPath(m); err != nil {
			v.addf("%s %q is not valid: %v", what, m, err)
		}
		if seen[m] {
			v.addf("%s %q is listed more than once", what, m)
		}
		seen[m] = true
	}
}

func (v *validator) checkTestLists() {
	v.checkModuleList("test module", v.model.TestModules)
	seen := map[string]bool{}
	for _, p := range v.model.TestSupport {
		v.checkPackagePattern(fmt.Sprintf("test support package %q", p), p)
		if seen[p] {
			v.addf("test support package %q is listed more than once", p)
		}
		seen[p] = true
	}
}

func (v *validator) checkRules() {
	layers := map[string]bool{}
	for _, l := range v.model.Layers {
		layers[l.ID] = true
	}
	ids := map[string]bool{}
	for _, r := range v.model.Rules {
		if !idPattern.MatchString(r.ID) || ids[r.ID] {
			v.addf("rule identifier %q is not valid or is used more than once", r.ID)
		}
		ids[r.ID] = true
		if r.Description == "" || r.Fix == "" {
			v.addf("rule %q needs a description and a fix", r.ID)
		}
		v.checkScope(r, layers)
		v.checkForbids(r)
	}
}

// checkScope checks the layers and the excepted components of a rule.
func (v *validator) checkScope(r Rule, layers map[string]bool) {
	if len(r.Scope.Layers) == 0 {
		v.addf("rule %q applies to no layer", r.ID)
	}
	for _, id := range r.Scope.Layers {
		if !layers[id] {
			v.addf("rule %q names unknown layer %q", r.ID, id)
		}
	}
	for _, id := range r.ExceptComponents {
		if _, ok := v.component(id); !ok {
			v.addf("rule %q names unknown component %q", r.ID, id)
		}
	}
}

func (v *validator) checkForbids(r Rule) {
	if len(r.ForbidImports) == 0 && len(r.ForbidSymbols) == 0 {
		v.addf("rule %q forbids nothing", r.ID)
	}
	for _, p := range r.ForbidImports {
		if module.CheckImportPath(strings.TrimSuffix(p, "/...")) != nil {
			v.addf("rule %q has invalid import pattern %q", r.ID, p)
		}
	}
	for _, s := range r.ForbidSymbols {
		if !validSymbol(s) {
			v.addf("rule %q has invalid symbol %q", r.ID, s)
		}
	}
}

// LayerOf returns the layer that owns the package with the import path pkg.
func LayerOf(m *Model, pkg string) (Layer, bool) {
	rel, ok := relativeDir(m.Module, pkg)
	if !ok {
		return Layer{}, false
	}
	for _, l := range m.Layers {
		if Within(rel, l.Directory) {
			return l, true
		}
	}
	return Layer{}, false
}

// relativeDir returns the directory of a package in the module, relative to
// the module root, or "." for the root package.
func relativeDir(mod, pkg string) (string, bool) {
	if pkg == mod {
		return ".", true
	}
	rel, ok := strings.CutPrefix(pkg, mod+"/")
	return path.Clean(rel), ok
}

// overlap reports whether two package patterns can match the same package.
func overlap(a, b string) bool {
	baseA, subA := strings.CutSuffix(a, "/...")
	baseB, subB := strings.CutSuffix(b, "/...")
	return baseA == baseB || subA && Within(baseB, baseA) || subB && Within(baseA, baseB)
}

// validSymbol reports whether s is a builtin function, or "importpath.Name"
// with an exported name.
func validSymbol(s string) bool {
	pkg, name, found := strings.CutLast(s, ".")
	if !found {
		_, ok := types.Universe.Lookup(s).(*types.Builtin)
		return ok
	}
	return module.CheckImportPath(pkg) == nil && token.IsIdentifier(name) && token.IsExported(name)
}

// cycle returns the nodes of one cycle in the graph, which maps each node to
// its successors, or nil. It visits the nodes in order.
func cycle(graph map[string][]string, order []string) []string {
	const (
		visiting = iota + 1
		done
	)
	state := map[string]int{}
	var stack []string
	var visit func(node string) []string
	visit = func(node string) []string {
		switch state[node] {
		case visiting:
			return append(slices.Clone(stack[slices.Index(stack, node):]), node)
		case done:
			return nil
		}
		state[node] = visiting
		stack = append(stack, node)
		for _, next := range graph[node] {
			if _, known := graph[next]; !known {
				continue
			}
			if c := visit(next); c != nil {
				return c
			}
		}
		stack = stack[:len(stack)-1]
		state[node] = done
		return nil
	}
	for _, node := range order {
		if c := visit(node); c != nil {
			return c
		}
	}
	return nil
}

func layerIDs(layers []Layer) []string {
	ids := make([]string, len(layers))
	for i, l := range layers {
		ids[i] = l.ID
	}
	return ids
}

func componentIDs(components []Component) []string {
	ids := make([]string, len(components))
	for i, c := range components {
		ids[i] = c.ID
	}
	return ids
}
