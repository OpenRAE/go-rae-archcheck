---
design: 3
issue: 3
title: Allow testscript in tests
---

# Design 0003: Allow testscript in tests

## Problem

The command contract tests use testscript, which lives in `github.com/rogpeppe/go-internal`, and no test module allowed it.

## Design

The test modules add `github.com/rogpeppe/go-internal`, so any test file can import testscript.
No production package can import it.

## Alternatives

- Keep the command tests in Go code only. They call the run function, not the command with real files.
