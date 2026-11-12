// Package finding defines the result of a check: one broken rule at one
// location, with the change that fixes it.
package finding

import (
	"cmp"
	"slices"
)

// Rule identifiers of the checks that the model itself does not define.
// Each rule of a model reports its findings under its own identifier.
const (
	// Model marks a defect inside the model.
	Model = "model"
	// Record marks a defect of a design record or a missing record.
	Record = "design-record"
	// ModulePath marks a module path that differs from the model.
	ModulePath = "module-path"
	// UnreadPath marks a path that the check cannot read.
	UnreadPath = "unread-path"
	// Layer marks a package outside every layer.
	Layer = "layer"
	// LayerUse marks an import of a layer that the importing layer does not use.
	LayerUse = "layer-use"
	// External marks an import that no external entry of the layer allows.
	External = "external-import"
	// TestSupport marks a non-test import of a test support package.
	TestSupport = "test-support"
	// Owner marks a package that no component owns.
	Owner = "package-owner"
	// UnknownPackage marks a component package that the code does not have.
	UnknownPackage = "unknown-package"
	// ComponentDependency marks an import that the component dependencies do not allow.
	ComponentDependency = "component-dependency"
	// ComponentModule marks an import of a module that the component does not list.
	ComponentModule = "component-module"
)

// A Finding is one way in which the model, the design records or the code
// breaks a rule.
type Finding struct {
	// Location is "file:line", or a file or directory when no line applies.
	Location string
	// Rule is the identifier of the broken rule.
	Rule string
	// Message states the problem.
	Message string
	// Fix states the change that corrects the problem.
	Fix string
}

// String formats the finding as two lines: the location, the rule and the
// message, then the fix.
func (f Finding) String() string {
	return f.Location + ": " + f.Rule + ": " + f.Message + "\n\tfix: " + f.Fix
}

// Sort sorts findings by location, rule, message and fix, removes exact
// duplicates and returns the result.
func Sort(findings []Finding) []Finding {
	slices.SortFunc(findings, func(a, b Finding) int {
		return cmp.Or(
			cmp.Compare(a.Location, b.Location),
			cmp.Compare(a.Rule, b.Rule),
			cmp.Compare(a.Message, b.Message),
			cmp.Compare(a.Fix, b.Fix),
		)
	})
	return slices.Compact(findings)
}
