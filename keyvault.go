package main

import (
	//legacykeyvault "github.com/pulumi/pulumi-azure/sdk/v5/go/azure/keyvault"
	"encoding/base64"

	"github.com/pulumi/pulumi-azure-native-sdk/keyvault/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// look up an AZ Key Vault Secret by name
//
//lint:ignore U1000 external type doesn't have compatible string method
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

// utility functions

// https://learn.microsoft.com/en-us/rest/api/keyvault/certificates/import-certificate/import-certificate?tabs=HTTP
// Why PFX and not PEM: https://learn.microsoft.com/en-us/answers/questions/131459/azure-cdn-key-vault
func importPfxToKeyVault(ctx *pulumi.Context, cfg *config.Config) (sec *keyvault.Secret, err error) {
	pfxData, err := loadFileFromDisk(cfg.Require("prodPfxCertPath"))
	if err != nil {
		return sec, err
	}
	// attempt to validate pfx data (e.g. it is not still in encrypted-at-rest state)
	err = validatePfx(pfxData, "")
	if err != nil {
		return sec, err
	}
	// import pfx to Key Vault
	secretName := cfg.Require("siteKey") + "-" + ctx.Stack() + "-apex-tls"
	sec, err = keyvault.NewSecret(ctx, secretName, &keyvault.SecretArgs{
		ResourceGroupName: pulumi.String(cfg.Require("keyvaultResourceGroup")),
		VaultName:         pulumi.String(cfg.Require("keyVaultName")),
		SecretName:        pulumi.String(secretName),
		Properties: &keyvault.SecretPropertiesArgs{
			ContentType: pulumi.String("application/x-pkcs12"),
			Value:       pulumi.Sprintf("%s", pulumi.ToSecret(base64.StdEncoding.EncodeToString(pfxData))),
		},
	})
	if err != nil {
		return sec, err
	}
	return
}
