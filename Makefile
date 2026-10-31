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

# MOCKERY_VERSION pins the mock generator, run through the pinned toolchain:
# a mockery binary built with an older Go cannot read the pinned toolchain's
# export data ("package X without types") and fails mid-run after deleting
# the mocks. `go run` at a tag keeps `make mock` reproducible like the lint
# tools above.
MOCKERY_VERSION := v3.8.1

# GOTOOLCHAIN_VERSION pins the toolchain used to BUILD and RUN the pinned lint
# tools. Some tool modules (golangci-lint) declare a lower go directive in
# their own go.mod (latest-1 by their policy), so module-aware commands build
# them with whatever local toolchain satisfies that lower bar — producing
# binaries too old to lint this module when the local Go lags go.mod. The
# `go run pkg@version` invocations below also bypass PATH entirely, so stale
# tool binaries elsewhere on the system cannot shadow the pinned versions.
# Keep the version in sync with the `toolchain` directive in go.mod.
GOTOOLCHAIN_VERSION := go1.27.3

# SUBMODULES lists every nested Go module covered by the test/lint gates
# alongside the root module. Add each new modules/<name> here on creation.
# `make release` derives the per-release tag list from it as well.
SUBMODULES := kernel modules/posttypes-basic modules/cart modules/catalog modules/orders modules/payments modules/media modules/comments modules/seo modules/theme-default modules/theme-storefront modules/site-ssr modules/textgen modules/importers modules/adminpanel modules/users modules/auth

lint: modernize depguard stylecheck
	GOTOOLCHAIN=$(GOTOOLCHAIN_VERSION) go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run
	for m in $(SUBMODULES); do (cd $$m && GOTOOLCHAIN=$(GOTOOLCHAIN_VERSION) go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run) || exit 1; done

# Fails when a nested module (including its tests) depends on the main
# module's internal tree. The import-direction rule (modules import kernel
# only) is structural for kernel but conventional for feature modules —
# this gate makes it mechanical. It uses the compiler's own dep graph, so
# test-only imports are covered too.

# Fails (exit non-zero) when the modernize analyzer finds old Go syntax.
# Each nested Go module is checked from its own directory.
modernize:
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@$(MODERNIZE_VERSION) -test $(MODERNIZE_PACKAGES)
	for m in $(SUBMODULES); do (cd $$m && go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@$(MODERNIZE_VERSION) -test ./...) || exit 1; done

# Applies the modernize rewrites in place.
modernize-apply:
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@$(MODERNIZE_VERSION) -fix -test $(MODERNIZE_PACKAGES)

mock:
	find . -type d -name "mocks" -prune -exec rm -rf {} +
	GOTOOLCHAIN=$(GOTOOLCHAIN_VERSION) go run github.com/vektra/mockery/v3@$(MOCKERY_VERSION)

test:
	go test ./...
	for m in $(SUBMODULES); do (cd $$m && go test ./...) || exit 1; done
# NOTE: `go test -tags nosqlite` fails by design — the repository and
# database suites are sqlite-backed. Test the default build; verify the
# server edition with `go build -tags nosqlite ./...` instead.

test-race:
	go test ./... -race -cover
	for m in $(SUBMODULES); do (cd $$m && go test ./... -race -cover) || exit 1; done

test-verbose:
	go test ./... -v

# GO-2026-5932 (golang.org/x/crypto/openpgp) is an accepted, unfixable advisory:
# it affects every x/crypto version and has no upstream patch. This module is
# required only for x/crypto/argon2 (internal/auth); openpgp is never imported.
# govulncheck reports it as a module-level finding only, which does not affect
# the exit code, so the gate stays green without suppressing it.
vulncheck:
	GOTOOLCHAIN=$(GOTOOLCHAIN_VERSION) go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	for m in $(SUBMODULES); do (cd $$m && GOTOOLCHAIN=$(GOTOOLCHAIN_VERSION) go run golang.org/x/vuln/cmd/govulncheck@latest ./...) || exit 1; done

depguard:
	@for m in $(SUBMODULES); do \
		if (cd $$m && go list -deps ./... | grep -q "lesstruct/internal"); then \
			echo "forbidden internal import in $$m:"; \
			(cd $$m && go list -deps ./... | grep "lesstruct/internal"); \
			exit 1; \
		fi; \
	done

# Fails when the file-layout contract drifts: methods must stay with their
# struct, constructors close their type, and package-level helpers precede
# public declarations. Tests and generated files are skipped; the walk covers
# the nested modules on disk, so no per-module loop is needed.
stylecheck:
	go run ./tools/stylecheck/cmd/stylecheck .

build-admin:
	cd web/admin && npm ci && npm run build-only

build-cli:
	mkdir -p bin && go build -ldflags "$(CLI_LDFLAGS)" -o bin/lesstruct-cli ./cmd/lesstruct-cli

# build-server compiles the server edition: the root package with the
# nosqlite build tag — postgres/mysql only, no embedded sqlite.
build-server:
	mkdir -p bin && go build -tags nosqlite -o bin/lesstruct-server .

# build-core compiles the Core edition: the headless engine (identity,
# content, media, plugins) without the SSR/theme/comment/textgen/importer
# modules. Install path: ./cmd/lesstruct-core.
build-core:
	mkdir -p bin && go build -o bin/lesstruct-core ./cmd/lesstruct-core

# build-ecommerce compiles the eCommerce edition: blog plus the catalog
# module (pricing, variants, stock). Install path: ./cmd/lesstruct-ecommerce.
build-ecommerce:
	mkdir -p bin && go build -o bin/lesstruct-ecommerce ./cmd/lesstruct-ecommerce

# release rewrites every Lesstruct require across the workspace to VERSION
# (lockstep: root vX.Y.Z, kernel/vX.Y.Z, modules/*/vX.Y.Z share one number),
# verifies the workspace still builds through a throwaway file proxy (the
# tags do not exist yet, so the real proxy cannot resolve them), and prints
# the commit, tag, push, and verify sequence. It never commits or tags by
# itself: review the diff, run the gates, then follow the printed steps.
# Filesystem `replace` directives must never come back — the proxy rejects
# such modules, which is what broke `go install …@latest` before v1.1.1.
# Never run a bare `go work sync` here either: it strips the intra-workspace
# requires this target maintains (and prunes the matching go.sum entries).
# The EDGE map below is the publishable require set; it mirrors the Requires
# column in docs/built-in-modules.md — update both when the graph changes.
release:
	@if [ -z "$(VERSION)" ]; then echo "usage: make release VERSION=vX.Y.Z"; exit 1; fi
	@case "$(VERSION)" in v[0-9]*.[0-9]*.[0-9]*) ;; *) echo "VERSION must look like v1.2.3, got '$(VERSION)'"; exit 1;; esac
	@LP=github.com/aristorinjuang/lesstruct; \
	for m in . $(SUBMODULES); do \
		(cd $$m && go mod edit -json | python3 -c 'import json,sys; d=json.load(sys.stdin); print("\n".join((r.get("Old") or {}).get("Path","") for r in d.get("Replace") or []))' | while IFS= read -r r; do \
			if [ -n "$$r" ]; then go mod edit -dropreplace="$$r"; fi; \
		done); \
	done; \
	(cd . && go mod edit \
		-require=github.com/aristorinjuang/lesstruct/kernel@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/adminpanel@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/auth@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/cart@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/catalog@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/comments@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/importers@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/media@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/orders@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/payments@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/posttypes-basic@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/seo@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/site-ssr@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/textgen@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/theme-default@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/theme-storefront@$(VERSION) \
		-require=github.com/aristorinjuang/lesstruct/modules/users@$(VERSION)); \
	edge() { m="$$1"; shift; a=""; for s in "$$@"; do a="$$a -require=$$LP/$$s@$(VERSION)"; done; (cd "modules/$$m" && go mod edit $$a); }; \
	edge adminpanel kernel; \
	edge auth kernel modules/users; \
	edge cart kernel; \
	edge catalog kernel; \
	edge comments kernel; \
	edge importers kernel modules/media modules/posttypes-basic modules/seo; \
	edge media kernel; \
	edge orders kernel; \
	edge payments kernel; \
	edge posttypes-basic kernel; \
	edge seo kernel; \
	edge site-ssr kernel modules/comments modules/media modules/posttypes-basic modules/theme-default; \
	edge textgen kernel modules/media; \
	edge theme-default kernel; \
	edge theme-storefront kernel; \
	edge users kernel modules/media
	@tmp=$$(mktemp -d); \
	for m in . $(SUBMODULES); do \
		mod=$$(head -1 $$m/go.mod | awk '{print $$2}'); \
		rel=$$(echo "$$mod" | sed 's#^github.com/aristorinjuang/lesstruct##'); \
		if [ -z "$$rel" ]; then d="$$tmp/github.com/aristorinjuang/lesstruct/@v"; \
		else d="$$tmp/github.com/aristorinjuang/lesstruct$$rel/@v"; fi; \
		mkdir -p "$$d"; cp $$m/go.mod "$$d/$(VERSION).mod"; \
		printf '{"Version":"$(VERSION)"}' > "$$d/$(VERSION).info"; \
	done; \
	export GOPROXY="file://$$tmp,https://proxy.golang.org" GOSUMDB=off; \
	go build ./... && for m in $(SUBMODULES); do (cd $$m && go build ./...) || exit 1; done; \
	rm -rf "$$tmp"
	@echo "requires now pin $(VERSION) (builds update go.work.sum by themselves)."
	@echo "next: review (git diff), run make lint test vulncheck, then:"
	@echo "  git commit -m 'build: release $(VERSION)'"
	@echo "  git tag $(VERSION) $$(for m in $(SUBMODULES); do echo $$m/$(VERSION); done | tr '\n' ' ')"
	@echo "  git push origin main --tags"
	@echo "verify once the proxy picks the tags up:"
	@echo "  GOWORK=off go mod tidy (in ./ and every SUBMODULE; expect no go.mod changes)"
	@echo "  go install github.com/aristorinjuang/lesstruct@$(VERSION)"

install: build-admin build-cli
	go install
	go install -ldflags "$(CLI_LDFLAGS)" ./cmd/lesstruct-cli

clean-bin:
	rm -rf bin

clean-static:
	rm -rf modules/adminpanel/spa/dist/*

css:
	go install github.com/tdewolff/minify/v2/cmd/minify@latest
	minify -o modules/theme-default/static/base.css modules/theme-default/static/base.src.css
	minify -o modules/theme-default/static/style.css modules/theme-default/static/style.src.css

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
	rm -f site/static/screenshots/*.png site/static/screenshots/*.webp docs/assets/screenshots/*.png docs/assets/screenshots/*.webp
