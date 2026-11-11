# Contribute to go-rae-archcheck

go-rae-archcheck holds the architecture of the projects that use it, so its behavior must stay exact and predictable.
Contributions are welcome when they keep the checker generic, tested and documented.

## Choose the route

- For a small documentation or test fix, open a pull request.
- For a change of behavior, of the model format or of the command, open an issue first.
- Keep unrelated work in separate pull requests.
- Base each pull request on `dev`. The `main` branch holds the releases.

## Set up the repository

1. Install [mise](https://mise.jdx.dev/getting-started.html) 2026.10.0 or newer.
2. Run `make tools`.
3. Run `mise exec -- uv run --locked --no-build pre-commit install` to install the hooks.

## Check a change

`make ci-check` runs every check that CI runs: lint, security, tests, fuzzing, regression proof, the self-check and prose.
`make hygiene` runs the file hygiene, workflow and secret hooks on every file.
Run `make help` for the individual checks.

## Follow the standards

- Write Go in the Google Go style. golangci-lint and the OpenRÆ `raesvet` analyzers enforce it.
- Keep statement coverage at or above 90% for each package.
- Test the command contract with the testscript files in `cmd/archcheck/testdata/script`, and give each new parser a fuzz target.
- Name a regression test `TestRegression...` in a `*_regression_test.go` file. It must fail on the base revision.
- Add a design record in `architecture/designs` for each structural change of `architecture/model.yaml`.
- Use a standard or widely used library before custom code, and record the reason for custom code.
- Write documentation in ASD-STE100 Simplified Technical English. Vale checks it.
- Write each pull request title as a Conventional Commit, for example `fix: report a missing design record`.

## Open a pull request

Fill in each section of the pull request template.
CI must pass before a merge.

Release Please makes each release from the Conventional Commit history on `main`.
It writes the version and `CHANGELOG.md`, so do not edit them in a pull request.

## Use Ground Control

The optional Ground Control workflow uses `.ground-control.yaml` for this repository.
It targets `dev` and uses the existing Make checks and pre-commit hooks.
The `.mcp.json` file starts the installed `grndctl mcp` server.

1. Install Ground Control through your approved setup.
2. Set up the local `.env` file with the credentials for this repository.
3. Restart your agent session to load the MCP server.
4. Run `/implement` with the GitHub issue number.

Keep `.env` and the local `.gc` artifacts out of Git.
The issue thread holds the plan and review records.
You review and merge the pull request after the checks pass.

This repository has no automatic Ground Control completion workflow.
Run `/implement` with the same issue number after the merge to record completion.
