# Architecture model, version 1

The model is a YAML file, by default `architecture/model.yaml`, that describes one Go module.
[`schema/model.schema.json`](../schema/model.schema.json) defines its format, and the checker adds the semantic rules on this page.

## Contents

| Element | Content |
| --- | --- |
| `version` and `module` | The format version, which is 1, and the path of the Go module |
| `system`, `people` and `external_systems` | The C4 context. Each element has an identifier, a name and a description. |
| `relationships` | Who uses what, between elements and containers, with a description and a technology |
| `containers` | The deployable and runtime units, each with a technology |
| `layers` | A directory, the other layers that its packages can use, its external entries with reasons, and an isolation flag |
| `components` | The containers that run a component, the packages that it owns, the components that it depends on and its external modules |
| `test_modules` | The external modules that any test file can import |
| `test_support` | The packages of shared fakes and conformance suites, which only tests import |
| `rules` | Forbidden imports and symbols, their scope of layers, test files and excepted components, and a fix message |

## Matching

- A forbidden import, a forbidden symbol and an external entry each match the path itself.
- A pattern that ends in `/...` also matches every package below it.
- An external pattern never crosses into another module, so each nested module or `/vN` version needs its own entry.
- A match never extends a path segment, so `k8s.io/api` does not match `k8s.io/apiserver`.

## Model checks

The model must be one YAML document that meets the JSON schema.
The schema states the fields, their types and patterns, the required text, the unique list items and the forbidden lists of each rule.
The checker reports each value that breaks the schema at its JSON pointer, and then it checks these rules:

- Each identifier is unique, and each reference names a known element, container, layer or component.
- No layer directory contains another, and the layer uses and the component dependencies have no cycle.
- The module path, the package patterns and the forbidden imports are valid Go paths.
- External entries name packages outside the standard library.
- The packages of a component sit in one layer, and no two components own the same package.
- A component in an isolated layer depends on no other component of that layer.
- Each forbidden symbol is a builtin function or an exported name.

## Code checks

The checker loads every package of the module with its test files, through `golang.org/x/tools/go/packages`. It then checks these rules:

- Each package sits in the directory of one layer.
- An import of another layer needs that layer in the uses of the importing layer.
- An import from another module needs a matching external entry of the layer.
- A test file can also import the test modules.
- Only test files and test support packages import test support packages.
- Once the model has components, each package has one component, and each component package exists.
- An import of another component needs a dependency on that component.
- An import from another module needs that module in the component.
- Each rule holds in its scope.

The checker reports a Go file that it cannot read, such as a file that a build constraint excludes.

## Design records

Design records are Markdown files, by default in `architecture/designs`, named `NNNN-slug.md`.
Each one starts with front matter between two `---` lines that states `design`, `issue` and `title`, and the design number matches the file name.

Every change to the model other than its descriptive text is structural.
Descriptive text is the names, descriptions, technologies, reasons and fix messages.
With `-base-model` and `-base-designs`, the checker reports a structural change that adds no new design record.

## Command

```sh
archcheck [-model path] [-designs dir] [-base-model path -base-designs path] [-code=false]
```

The command runs from the module root.
`-base-designs` names a file that lists the record names of the base revision, as `git ls-tree --name-only` prints them.
`-code=false` checks only the model and the records.

The exit status is 0 with no findings, 1 with findings or an error, and 2 for a usage error.
