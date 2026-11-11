.DEFAULT_GOAL := help
.PHONY: help tools hygiene lint security test fuzz regression arch prose ci-check

MISE ?= mise
GO = $(MISE) exec -- go
GOLANGCI = .cache/bin/golangci-lint
RAESVET = .cache/bin/raesvet
GOVULNCHECK = .cache/bin/govulncheck
COVERAGE = .cache/bin/go-test-coverage
REGRESSIONTEST = .cache/bin/regressiontest

help:
	@echo 'Checks: make lint, make security, make test, make fuzz, make regression, make arch, make prose, make hygiene'
	@echo 'All checks, as CI runs them: make ci-check'

tools:
	$(MISE) install
	$(MISE) exec -- uv sync --locked --no-build

# File hygiene, workflow lint, workflow security and secret detection.
hygiene:
	$(MISE) exec -- uv run --locked --no-build pre-commit run --all-files

# The tools module pins the Go tools, so the checker module never depends on them.
$(GOLANGCI): tools/go.mod tools/go.sum
	$(GO) build -modfile tools/go.mod -o $@ github.com/golangci/golangci-lint/v2/cmd/golangci-lint

$(RAESVET): tools/go.mod tools/go.sum
	$(GO) build -modfile tools/go.mod -o $@ github.com/OpenRAE/rae/implementations/tooling/go/cmd/raesvet

$(GOVULNCHECK): tools/go.mod tools/go.sum
	$(GO) build -modfile tools/go.mod -o $@ golang.org/x/vuln/cmd/govulncheck

$(COVERAGE): tools/go.mod tools/go.sum
	$(GO) build -modfile tools/go.mod -o $@ github.com/vladopajic/go-test-coverage/v2

$(REGRESSIONTEST): tools/go.mod tools/go.sum
	$(GO) build -modfile tools/go.mod -o $@ github.com/OpenRAE/rae/implementations/tooling/go/cmd/regressiontest

# The lint gate: golangci-lint, go vet, the OpenRÆ raesvet analyzers, and tidy and fix checks.
lint: $(GOLANGCI) $(RAESVET)
	$(GO) mod tidy -diff
	cd tools && $(GO) mod tidy -diff
	$(GOLANGCI) run ./...
	$(GO) vet ./...
	$(GO) vet -vettool=$(CURDIR)/$(RAESVET) ./...
	$(GO) fix -diff ./...

# The dependency gate: module verification, known vulnerabilities and the license scan, which
# fails on restricted, forbidden and unknown licenses. The tools module never ships.
security: $(GOVULNCHECK)
	$(GO) mod verify
	$(GO) mod download
	$(MISE) exec -- trivy fs --scanners license --severity UNKNOWN,HIGH,CRITICAL --exit-code 1 --quiet \
		--skip-dirs tools --skip-dirs .venv --skip-dirs .cache --skip-files uv.lock .
	$(GOVULNCHECK) ./...

# The tests, with the race detector, shuffled order and at least 90% coverage for each package.
test: $(COVERAGE)
	mkdir -p .cache
	$(GO) test -race -shuffle=on -covermode=atomic -coverprofile=.cache/coverage.out ./...
	$(COVERAGE) --config .testcoverage.yml

# Each fuzz target of the parsers runs for a short time. A failing input lands in testdata/fuzz,
# and the fix commits it as a regression seed.
fuzz:
	$(GO) test -run='^$$' -fuzz='^FuzzParse$$' -fuzztime=15s ./internal/model
	$(GO) test -run='^$$' -fuzz='^FuzzParseFrontMatter$$' -fuzztime=15s ./internal/record

# BASE_REV is the git revision of the base, for example origin/dev or a commit.
# Each new TestRegression test in a *_regression_test.go file must fail on the base revision.
regression: $(REGRESSIONTEST)
	$(MISE) exec -- $(CURDIR)/$(REGRESSIONTEST) -base "$${BASE_REV:-origin/dev}"

# The checker checks its own model, records and code. With a base revision, a structural change
# of the model needs a new design record.
arch:
	@mkdir -p .cache; if [ -n "$$BASE_REV" ] && git cat-file -e "$$BASE_REV:architecture/model.yaml" 2>/dev/null; then \
		git show "$$BASE_REV:architecture/model.yaml" > .cache/base-model.yaml; \
		git ls-tree --name-only "$$BASE_REV" architecture/designs/ > .cache/base-designs.txt; \
		$(GO) run ./cmd/archcheck -base-model .cache/base-model.yaml -base-designs .cache/base-designs.txt; \
	else $(GO) run ./cmd/archcheck; fi

# The documentation follows the ASD-STE100 rules of the Vale style.
prose:
	$(MISE) exec -- vale $$(git ls-files '*.md' ':!:**/testdata/**')

ci-check: lint security test fuzz regression arch prose
