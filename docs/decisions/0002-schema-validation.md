# 0002: The JSON schema validates the structure of each model

Date: 2026-11-01.

## Context

The repository publishes a JSON schema of the model for editors and for other tools.
The Go validator also stated the same structural rules, and the two drifted apart.
The schema rejected builtin symbols that the code accepts, and the code did not check several lists for unique items.

## Decision

The checker validates the structure of each model against the published schema with `github.com/santhosh-tekuri/jsonschema/v6`.
That validator supports JSON Schema 2020-12 and generates no code.

The Go code checks only the rules that the schema cannot express.
These rules are unique identifiers across lists, references, overlaps, cycles, Go paths and symbols, and the layers of components.

## Alternatives

| Option | Reason it was not selected |
| --- | --- |
| Go validation only | The rules then live in two places, as the drift showed. |
| Go code generated from the schema | The generated validation is a second copy of the rules, and it needs its own generation check. |

## Consequences

The schema is the only statement of the structure of the model.
Each value that breaks the schema produces one finding at its JSON pointer.
