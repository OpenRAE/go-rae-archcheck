# 0003: Four small mechanisms stay custom

Date: 2026-11-01.

## Context

The checker uses a standard or widely used library before custom code.
Four small mechanisms have no maintained library that fits them.

## Decision

These mechanisms stay custom, each with tests:

| Mechanism | Reason |
| --- | --- |
| Front matter of design records | `github.com/adrg/frontmatter` depends on `gopkg.in/yaml.v2`, and the upstream repository of that module is archived. The checker cuts the block between the two `---` lines and decodes it with the maintained `go.yaml.in/yaml/v3` module. |
| Cycle detection in the layer and component graphs | `gonum.org/v1/gonum/graph` detects cycles, but it is a large numeric library for one depth-first search. |
| Matching of `/...` patterns | The go command keeps its pattern matching in internal packages, which other modules cannot import. |
| The standard-library test | The checker applies the rule of the go command: the first element of a standard-library path has no dot. The go command keeps that function internal. |

## Consequences

When a maintained library that fits one of these mechanisms appears, a new record replaces this one, and the library replaces the custom code.
