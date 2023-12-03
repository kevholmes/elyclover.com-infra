package main

import (
	nativecdn "github.com/pulumi/pulumi-azure-native-sdk/cdn/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/storage/v2"
	"github.com/pulumi/pulumi-azure/sdk/v5/go/azure/core"
	"github.com/pulumi/pulumi-azure/sdk/v5/go/azure/dns"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type cfgKeys struct {
	// pulumi/env names
	projectKey string
	envKey     string
	siteKey    string
	// Azure service values we pull in from projects external to this (for now)
	thisAzureTenantId   string
	dnsResourceGrp      string
	dnsLookupZone       string
	cdnAzureId          string
	kvAzureSubscription string // keyvault can live elsewhere
	kvAzureResourceGrp  string
	kvAzureName         string
	// Github service values pulled in from other projects external to this one
	ghAppSrcRepo string
}

type svcPrincipals struct {
	cicd ServicePrincipalEnvelope
}
type projectResources struct {
	pulumiCtx *pulumi.Context
	cfg       *config.Config
	cfgKeys   cfgKeys
	// Azure service values for top-level Subscription
	thisAzureSubscription *core.LookupSubscriptionResult
	svcPrincipals         svcPrincipals
	webResourceGrp        *resources.ResourceGroup
	webStorageAccount     *storage.StorageAccount
	webStaticEp           pulumi.StringOutput
	webCdnProfile         *nativecdn.Profile
	webCdnEp              *nativecdn.Endpoint
	webDnsZone            *dns.LookupZoneResult
	webFqdn               pulumi.StringOutput
}

const PROD = "prod"
const DEV = "dev"

func main() {

	pulumi.Run(func(ctx *pulumi.Context) error {
		// Init common resources
		pr := projectResources{
			pulumiCtx: ctx,
			cfg:       config.New(ctx, ""),
		}
		// Init config keys from Pulumi key:values set per project/env
		err := pr.initConfigKeys()
		if err != nil {
			return err
		}

		// Create an Azure Resource Group
		err = pr.createResourceGroup1()
		//webResourceGrp, err := createResourceGroup(ctx, projectKey+"-"+envKey, nil)
		if err != nil {
			return err
		}

		// Create an Azure Storage Account to host our site
		//siteKey := cfg.Require("siteKey")
		err = pr.newStorageAccount1()
		//storageAccount, err := newStorageAccount(ctx, siteKey+envKey, webResourceGrp.Name)
		if err != nil {
			return err
		}

		// Enable static web hosting on storage account
		//err = enableStaticWebHostOnStorageAccount(ctx, storageAccount.Name, webResourceGrp.Name, siteKey)
		err = pr.enableStaticWebHostOnStorageAccount1()
		if err != nil {
			return err
		}

		// Strip leading 'https://' and trailing '/' from web endpoint address
		// for the Storage Account's static website URL
		pr.webStaticEp = stripWebStorageEndPointUrl(pr.webStorageAccount)

		// Create CDN Profile for usage by our endpoint(s)
		//cdnProfile, err := createCdnProfile(ctx, siteKey+envKey, webResourceGrp.Name)
		err = pr.createCdnProfile1()
		if err != nil {
			return err
		}

		// Create CDN Endpoint using newly created CDN Profile
		//endpoint, err := createCdnEndpoint(ctx, siteKey+envKey, cdnProfile, webResourceGrp.Name, staticEndpoint)
		err = pr.createCdnEndpoint1()
		if err != nil {
			return err
		}

		// Look up DNS zone based on pulumi stack config var for external resource group that houses DNS records
		//dnsRG := cfg.Require("dnsResourceGroup")
		//dnsLookupZone := cfg.Require("dnsZoneName")
		//dnsZone, err := lookupDnsZone(ctx, dnsRG, dnsLookupZone)
		err = pr.lookupDnsZone1()
		if err != nil {
			return err
		}

		// Set up domains depending on env
		//fqdn, err := createDnsRecordByEnv(ctx, dnsRG, dnsZone, endpoint, envKey, siteKey)
		err = pr.createDnsRecordByEnv1()
		if err != nil {
			return err
		}

		// Set up TLS depending on environment and custom domain types
		//err = setupTlsTermination(ctx, cfg, endpoint, fqdn)
		err = pr.setupTlsTermination1()
		if err != nil {
			return err
		}

		// Create+authorize Service Principal to be used in CI/CD process (uploading new content, invalidating cdn cache)
		//cicdSp, err := generateCICDServicePrincipal(ctx, storageAccount, endpoint)
		err = pr.generateCICDServicePrincipal1()
		if err != nil {
			return err
		}

		// Export service principal secret/id, cdn profile/endpoint, resource group, storage acct
		// to GitHub repo Deployment secrets/vars where Actions build and deploy to each environment re: gitops flow
		err = pr.exportDeployEnvDataToGitHubRepo1()
		if err != nil {
			return err
		}

		return nil
	})
}
