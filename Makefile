# Module path (matches go.mod) and CLI build version. VERSION is derived from
# git when available (tag, falling back to the commit SHA, plus a -dirty
# suffix); otherwise it defaults to "dev". It is injected into the CLI binary
# via -ldflags so `lesstruct-cli --version` reports the shipped version.
MODULE := github.com/aristorinjuang/lesstruct
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
CLI_LDFLAGS := -X $(MODULE)/cmd/lesstruct-cli/cmd.version=$(VERSION)

# MODERNIZE_VERSION pins the standalone gopls modernize runner. Keep it in
# sync with the gopls version your editor runs so CI and editor suggestions
# match. /web/ is excluded because it vendors third-party Go code.
MODERNIZE_VERSION := v0.21.1
MODERNIZE_PACKAGES := $(shell go list ./... | grep -v '/web/')

lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4
	golangci-lint run

# Fails (exit non-zero) when the modernize analyzer finds old Go syntax.
modernize:
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@$(MODERNIZE_VERSION) -test $(MODERNIZE_PACKAGES)

# Applies the modernize rewrites in place.
modernize-apply:
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@$(MODERNIZE_VERSION) -fix -test $(MODERNIZE_PACKAGES)

mock:
	find . -type d -name "mocks" -prune -exec rm -rf {} +
	mockery

test:
	go test ./...

test-race:
	go test ./... -race -cover

test-verbose:
	go test ./... -v

# GO-2026-5932 (golang.org/x/crypto/openpgp) is an accepted, unfixable advisory:
# it affects every x/crypto version and has no upstream patch. This module is
# required only for x/crypto/argon2 (internal/auth); openpgp is never imported.
# govulncheck reports it as a module-level finding only, which does not affect
# the exit code, so the gate stays green without suppressing it.
vulncheck:
	go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

build-admin:
	cd web/admin && npm ci && npm run build-only

build-cli:
	mkdir -p bin && go build -ldflags "$(CLI_LDFLAGS)" -o bin/lesstruct-cli ./cmd/lesstruct-cli

install: build-admin build-cli
	go install
	go install -ldflags "$(CLI_LDFLAGS)" ./cmd/lesstruct-cli

clean-bin:
	rm -rf bin

clean-static:
	rm -rf internal/api/static/admin/*

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
