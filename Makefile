SHELL := /bin/bash

# elyclover.com-infra

# targets to handle SOPS encryption/decryption using Azure keyvault
decrypt:
	@./scripts/sops.sh $@
encrypt:
	@./scripts/sops.sh $@

# ci
lint: lint-shell lint-go lint-markdown

lint-shell:
	@shellcheck --color=always scripts/*

lint-go: lint-go-setup
	@staticcheck ./...

lint-go-setup:
	@go install honnef.co/go/tools/cmd/staticcheck@latest

lint-markdown:
	@IFS=$$'\n' && \
	MDFILES=($$(find . -name "*.md")) && \
	unset IFS && \
	for f in "$${MDFILES[@]}"; do \
	  markdownlint "$$f"; \
	done

build:
	@go build
