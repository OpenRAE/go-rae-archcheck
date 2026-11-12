# 0001: One model states the architecture and configures the checks

Date: 2026-11-01.

## Context

The projects that use the checker need their architecture to hold from the first package.
They need layers, components, external grants with reasons, forbidden imports and symbols, test-only packages and the C4 context of the system.
A structural change of the architecture also needs a record of its reasons.

## Decision

One YAML model is both the C4 architecture model and the configuration of the checks.
The checker reports a structural change of the model that adds no design record.
The checker merged the earlier checkers of OpenRÆ and of the environment packs, so the repositories share one tool.

## Alternatives

The table compares the Go architecture tools as of 2026-11-01.

| Option | Reason it was not selected |
| --- | --- |
| go-arch-lint v1.19.1 | It checks components and third-party imports. It has no forbidden symbols, no reasons, no test-only packages, no C4 model and no design records. |
| arch-go v1.6.2 | It checks dependency, content, function and naming rules, and it has the same gaps as go-arch-lint. |
| depguard and forbidigo in golangci-lint | They check import lists and forbidden symbols by file path. They cannot assign packages to components. |
| A Structurizr workspace beside a lint configuration | Two sources of the same structure drift apart. |

## Consequences

The model format is the public contract of the checker, and `docs/model.md` and the JSON schema state it.
A change of the format needs a new format version.
