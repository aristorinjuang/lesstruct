# Module path (matches go.mod) and CLI build version. VERSION is derived from
# git when available (tag, falling back to the commit SHA, plus a -dirty
# suffix); otherwise it defaults to "dev". It is injected into the CLI binary
# via -ldflags so `lesstruct-cli --version` reports the shipped version.
# --match "v*" restricts describe to product tags so kernel/vX.Y.Z never
# leaks into the CLI version.
MODULE := github.com/aristorinjuang/lesstruct
VERSION ?= $(shell git describe --tags --match "v*" --always --dirty 2>/dev/null || echo dev)
CLI_LDFLAGS := -X $(MODULE)/cmd/lesstruct-cli/cmd.version=$(VERSION)

# MODERNIZE_VERSION pins the standalone gopls modernize runner. Keep it in
# sync with the gopls version your editor runs so CI and editor suggestions
# match. /web/ is excluded because it vendors third-party Go code.
MODERNIZE_VERSION := v0.21.1
MODERNIZE_PACKAGES := $(shell go list ./... | grep -v '/web/')

# GOTOOLCHAIN_VERSION pins the toolchain used to BUILD and RUN the pinned lint
# tools. Some tool modules (golangci-lint) declare a lower go directive in
# their own go.mod (latest-1 by their policy), so module-aware commands build
# them with whatever local toolchain satisfies that lower bar — producing
# binaries too old to lint this module when the local Go lags go.mod. The
# `go run pkg@version` invocations below also bypass PATH entirely, so stale
# tool binaries elsewhere on the system cannot shadow the pinned versions.
# Keep the version in sync with the `toolchain` directive in go.mod.
GOTOOLCHAIN_VERSION := go1.27.2

# SUBMODULES lists every nested Go module covered by the test/lint gates
# alongside the root module. Add each new modules/<name> here on creation.
SUBMODULES := kernel modules/posttypes-basic modules/media modules/comments modules/seo modules/theme-default modules/site-ssr modules/textgen modules/importers modules/adminpanel modules/users modules/auth

lint: modernize depguard
	GOTOOLCHAIN=$(GOTOOLCHAIN_VERSION) go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run
	for m in $(SUBMODULES); do (cd $$m && GOTOOLCHAIN=$(GOTOOLCHAIN_VERSION) go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run); done

# Fails when a nested module (including its tests) depends on the main
# module's internal tree. The import-direction rule (modules import kernel
# only) is structural for kernel but conventional for feature modules —
# this gate makes it mechanical. It uses the compiler's own dep graph, so
# test-only imports are covered too.

# Fails (exit non-zero) when the modernize analyzer finds old Go syntax.
# Each nested Go module is checked from its own directory.
modernize:
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@$(MODERNIZE_VERSION) -test $(MODERNIZE_PACKAGES)
	for m in $(SUBMODULES); do (cd $$m && go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@$(MODERNIZE_VERSION) -test ./...); done

# Applies the modernize rewrites in place.
modernize-apply:
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@$(MODERNIZE_VERSION) -fix -test $(MODERNIZE_PACKAGES)

mock:
	find . -type d -name "mocks" -prune -exec rm -rf {} +
	mockery

test:
	go test ./...
	for m in $(SUBMODULES); do (cd $$m && go test ./...); done
# NOTE: `go test -tags nosqlite` fails by design — the repository and
# database suites are sqlite-backed. Test the default build; verify the
# server edition with `go build -tags nosqlite ./...` instead.

test-race:
	go test ./... -race -cover
	for m in $(SUBMODULES); do (cd $$m && go test ./... -race -cover); done

test-verbose:
	go test ./... -v

# GO-2026-5932 (golang.org/x/crypto/openpgp) is an accepted, unfixable advisory:
# it affects every x/crypto version and has no upstream patch. This module is
# required only for x/crypto/argon2 (internal/auth); openpgp is never imported.
# govulncheck reports it as a module-level finding only, which does not affect
# the exit code, so the gate stays green without suppressing it.
vulncheck:
	GOTOOLCHAIN=$(GOTOOLCHAIN_VERSION) go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	for m in $(SUBMODULES); do (cd $$m && GOTOOLCHAIN=$(GOTOOLCHAIN_VERSION) go run golang.org/x/vuln/cmd/govulncheck@latest ./...); done

depguard:
	@for m in $(SUBMODULES); do \
		if (cd $$m && go list -deps ./... | grep -q "lesstruct/internal"); then \
			echo "forbidden internal import in $$m:"; \
			(cd $$m && go list -deps ./... | grep "lesstruct/internal"); \
			exit 1; \
		fi; \
	done

build-admin:
	cd web/admin && npm ci && npm run build-only

build-cli:
	mkdir -p bin && go build -ldflags "$(CLI_LDFLAGS)" -o bin/lesstruct-cli ./cmd/lesstruct-cli

# build-server compiles the server edition: the root package with the
# nosqlite build tag — postgres/mysql only, no embedded sqlite.
build-server:
	mkdir -p bin && go build -tags nosqlite -o bin/lesstruct-server .

install: build-admin build-cli
	go install
	go install -ldflags "$(CLI_LDFLAGS)" ./cmd/lesstruct-cli

clean-bin:
	rm -rf bin

clean-static:
	rm -rf modules/adminpanel/spa/dist/*

css:
	go install github.com/tdewolff/minify/v2/cmd/minify@latest
	minify -o internal/api/template/static/base.css internal/api/template/static/base.src.css
	minify -o internal/api/template/static/style.css internal/api/template/static/style.src.css

test-cli:
	go test -tags integration -race -v ./cmd/lesstruct-cli/...

docs-build:
	cd site && hugo --minify

docs-serve:
	cd site && hugo server -D --disableFastRender

docs-clean:
	rm -rf site/public site/resources site/.hugo_build.lock

docs: docs-build

screenshots-install:
	cd scripts/screenshots && npm install && npx playwright install chromium

screenshots:
	cd scripts/screenshots && node capture.mjs

screenshots-clean:
	rm -f site/static/screenshots/*.png docs/assets/screenshots/*.png
