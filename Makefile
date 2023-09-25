# elyclover.com-infra

# targets to handle SOPS encryption/decryption using Azure keyvault
decrypt:
	@./scripts/sops.sh $@
encrypt:
	@./scripts/sops.sh $@
