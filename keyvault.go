package main

import (
	//legacykeyvault "github.com/pulumi/pulumi-azure/sdk/v5/go/azure/keyvault"
	"github.com/pulumi/pulumi-azure-native-sdk/keyvault/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func getSecretByName(ctx *pulumi.Context, kv string, rg string, name string) (secret *keyvault.LookupSecretResult, err error) {
	secret, err = keyvault.LookupSecret(ctx, &keyvault.LookupSecretArgs{
		VaultName:         kv,
		SecretName:        name,
		ResourceGroupName: rg,
	})
	if err != nil {
		return secret, err
	}

	return
}
