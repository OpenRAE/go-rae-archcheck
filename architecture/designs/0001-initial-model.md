---
design: 1
issue: 1
title: The initial model of the checker
---

# Design 0001: The initial model of the checker

## Problem

The checker must meet the same architecture rules that it enforces in other repositories.

## Design

Each package directory is a layer: the command, the code checks, the record checks, the model and the finding type.
The command uses every other layer, the checks use the model and the finding type, and the model uses only the finding type.

Each external entry names one package, with its reason.
Two rules keep process globals in the main package and forbid process execution, because the packages loader runs the go command for the checker.

## Alternatives

- One layer for all internal packages. The layers then stop no import between the checks.
