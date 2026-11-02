---
design: 2
issue: 3
title: Validate the model with its schema, and declare the components
---

# Design 0002: Validate the model with its schema, and declare the components

## Problem

The JSON schema of the model and the Go validator stated the same structural rules twice, and the two had drifted apart.
The schema rejected builtin symbols that the code accepts, and the Go code did not check several lists for unique items.
The model of the checker also declared no components, so the checker did not check package ownership on itself.

## Design

The new `schema` layer holds the JSON schema of the model and validates each model against it.
It uses a JSON Schema 2020-12 validator that generates no code, with an English message printer from `golang.org/x/text`.
The `model` layer uses the `schema` layer, and its Go code checks only the rules that the schema cannot express.

Each package of the checker is now a component, with the dependencies and modules that its imports need.
The `record` layer keeps the YAML module only to encode model structures, because the front matter now uses the strict decoder of the `model` package.

## Alternatives

- Keep the schema as documentation only. The schema and the code then drift apart again.
- Put the validator in the `model` layer. The embedding then needs the schema file in the `model` directory.
