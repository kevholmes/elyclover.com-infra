package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-azure-native-sdk/authorization/v2"
	"github.com/pulumi/pulumi-azuread/sdk/v5/go/azuread"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {

	// Check for DEBUG mode in local execution environment
	isDEBUG := os.Getenv("DEBUG")
	if isDEBUG == "true" {
		fmt.Println("DEBUG: Debug console logging enabled!")
	} else {
		fmt.Println("INFO: Debug console logging disabled!")
	}

	// Begin Pulumi functionality
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Initialize and obtain config values for stack, stack name
		cfg := config.New(ctx, "")
		projectKey := ctx.Project()
		envKey := ctx.Stack()
		fmt.Printf("DEBUG: stack name: %s\n", projectKey)

		// Create an Azure Resource Group
		webResourceGrp, err := createResourceGroup(ctx, projectKey+"-"+envKey, nil)
		if err != nil {
			return err
		}

		// Create an Azure Storage Account to host our site
		siteKey := cfg.Require("siteKey")
		storageAccount, err := newStorageAccount(ctx, siteKey+envKey, webResourceGrp.Name)
		if err != nil {
			return err
		}

		// Enable static web hosting on storage account
		err = enableStaticWebHostOnStorageAccount(ctx, storageAccount.Name, webResourceGrp.Name, siteKey)
		if err != nil {
			return err
		}

		// Strip leading 'https://' and trailing '/' from web endpoint address
		// for the Storage Account's static website URL
		staticEndpoint := stripWebStorageEndPointUrl(storageAccount)

		// Create CDN Profile for usage by our endpoint(s)
		cdnProfile, err := createCdnProfile(ctx, siteKey+envKey, webResourceGrp.Name)
		if err != nil {
			return err
		}

		// Create CDN Endpoint using newly created CDN Profile
		endpoint, err := createCdnEndpoint(ctx, siteKey+envKey, cdnProfile, webResourceGrp.Name, staticEndpoint)
		if err != nil {
			return err
		}

		// Look up DNS zone based on pulumi stack config var for external resource group that houses DNS records
		dnsRG := cfg.Require("dnsResourceGroup")
		dnsLookupZone := cfg.Require("dnsZoneName")
		dnsZone, err := lookupDnsZone(ctx, dnsRG, dnsLookupZone)
		if err != nil {
			return err
		}

		// Set up domains depending on env
		fqdn, err := createDnsRecordByEnv(ctx, dnsRG, dnsZone, endpoint, envKey, siteKey)
		if err != nil {
			return err
		}

		// Set up TLS depending on environment - auto-tls for Azure CDN Classic doesn't support Apex domains :(
		switch envKey {
		case "prod":
			// Register Azure CDN Application as Service Principal in Azure Active Directory tenant so we can grant it access to scoped Azure KV secrets
			cdnId := cfg.Require("Microsoft.AzureFrontDoor-Cdn")
			nsp, err := azuread.NewServicePrincipal(ctx, cdnId, &azuread.ServicePrincipalArgs{
				ApplicationId: pulumi.String(cdnId),
				UseExisting:   pulumi.Bool(false),
				Description:   pulumi.String("Service Principal tied to built-in Azure CDN/FD Application ID/product"),
			})
			if err != nil {
				return err
			}
			keyVaultScope := pulumi.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.KeyVault/vaults/%s",
				cfg.Require("keyVaultAzureSubscription"), cfg.Require("keyvaultResourceGroup"), cfg.Require("keyVaultName"))
			// assign predefined "Key Vault Secret User" RoleDefinitionId to Service Principal we just created
			// https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#key-vault-secrets-user
			_, err = authorization.NewRoleAssignment(ctx, "AzureFDCDNreadKVCerts", &authorization.RoleAssignmentArgs{
				PrincipalId:      nsp.ID(),
				PrincipalType:    pulumi.String("ServicePrincipal"),
				RoleDefinitionId: pulumi.String("/providers/Microsoft.Authorization/roleDefinitions/4633458b-17de-408a-b874-0445c86b69e6"),
				Scope:            keyVaultScope,
			})
			if err != nil {
				return err
			}
			// now create custom domain using user-provided certificate. possibly refactor newEndpointCustomDomain?
		default:
			// newEndpointCustomDomain() will need to be refactored to support conditional for auto-tls setup
			// Add Custom Domain to CDN to set up automatic TLS termination/cert rotation
			_, err = newEndpointCustomDomain(ctx, siteKey+envKey, endpoint, fqdn)
			if err != nil {
				return err
			}
		}

		// create+authorize Service Principal to be used in CI/CD process (uploading new content, invalidating cdn cache)
		cicdSp, err := generateCICDServicePrincipal(ctx, storageAccount)

		// export service principal secret/id, cdn profile/endpoint, resource group, storage acct
		// to GitHub repo Deployment secrets/vars where Actions build and deploy to each environment re: gitops flow
		err = exportDeployEnvDataToGitHubRepo(ctx, cfg, cicdSp, webResourceGrp, storageAccount, cdnProfile, endpoint)
		if err != nil {
			return err
		}

		return nil
	})
}
