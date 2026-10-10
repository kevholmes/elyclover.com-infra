.PHONY: build test coverage-check lint lint-go lint-markdown
COVER_PKGS := $(shell go list ./internal/... | grep -v /testutil | paste -sd, -)

build:
	go build ./...

test:
	go test ./... -race -covermode=atomic -coverpkg=$(COVER_PKGS) -coverprofile=coverage.out

coverage-check: test
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/ {gsub("%","",$$3); print $$3}'); \
	echo "total coverage: $$total%"; \
	awk -v t="$$total" 'BEGIN { exit !(t + 0 >= 80.0) }' || { echo "FAIL: coverage below 80%"; exit 1; }

lint: lint-go lint-markdown

lint-go:
	go vet ./...
	go tool staticcheck ./...

lint-markdown:
	markdownlint '**/*.md' --ignore CHANGELOG.md
