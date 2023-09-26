# elyclover.com-infra

# targets to handle SOPS encryption/decryption using Azure keyvault
decrypt:
	@./scripts/sops.sh $@
encrypt:
	@./scripts/sops.sh $@

# ci
lint: lint-shell lint-go

lint-shell:
	@shellcheck --color=always scripts/*

lint-go: lint-go-setup
	@staticcheck ./...

lint-go-setup:
	@go install honnef.co/go/tools/cmd/staticcheck@latest

build:
	@go build
