lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4
	golangci-lint run

mock:
	find . -type d -name "mocks" -prune -exec rm -rf {} +
	mockery

test:
	go test ./... -cover -race -v

vulncheck:
	go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

build-admin:
	cd web/admin && npm ci && npm run build-only

build-cli:
	mkdir -p bin && go build -o bin/lesstruct-cli ./cmd/lesstruct-cli

install: build-admin build-cli
	go install
	go install ./cmd/lesstruct-cli

clean-bin:
	rm -rf bin

clean-static:
	rm -rf internal/api/static/admin/*

css:
	go install github.com/tdewolff/minify/v2/cmd/minify@latest
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
