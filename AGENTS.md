# Repository instructions

Ground Control configuration lives in `.ground-control.yaml`.
Repository plan rules live in `.gc/plan_rules.md`.
Run `make ci-check` before you publish a change.

Target ordinary pull requests at `dev`.
The `main` branch is for reviewed promotions from `dev`.

The local `.mcp.json` starts `grndctl mcp`.
Run `grndctl doctor` after you set up the local environment.

Run `mise exec -- uv run --locked --no-build pre-commit install` to install the local hooks.
CI runs the same hooks on the full repository.

`CONTRIBUTING.md` states the standards and the checks of this repository.
