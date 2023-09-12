package main

import (
	"fmt"
	"os"
	"strings"

	nativecdn "github.com/pulumi/pulumi-azure-native-sdk/cdn/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/storage/v2"
	legacycdn "github.com/pulumi/pulumi-azure/sdk/v5/go/azure/cdn"
	"github.com/pulumi/pulumi-azure/sdk/v5/go/azure/dns"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {

	// check for DEBUG mode in local execution environment
	isDEBUG := os.Getenv("DEBUG")
	if isDEBUG == "true" {
		fmt.Println("DEBUG: Debug console logging enabled!")
	} else {
		fmt.Println("INFO: Debug console logging disabled!")
	}

	// begin Pulumi functionality
	pulumi.Run(func(ctx *pulumi.Context) error {
		// initialize and obtain config values for stack, stack name
		cfg := config.New(ctx, "")
		projectKey := ctx.Project()
		envKey := ctx.Stack()
		fmt.Printf("DEBUG: stack name: %s\n", projectKey)

		// Create an Azure Resource Group
		webResourceGrp, err := resources.NewResourceGroup(ctx, projectKey+"-"+envKey, nil)
		if err != nil {
			fmt.Printf("ERROR: creating webResourceGrp failed with %s+'-'%s\n", projectKey, envKey)
			return err
		}

		// Create an Azure resource (Storage Account)
		siteKey := cfg.Require("siteKey")
		storageAccountArgs := storage.StorageAccountArgs{
			ResourceGroupName: webResourceGrp.Name,
			Sku: &storage.SkuArgs{
				Name: pulumi.String("Standard_ZRS"),
			},
			Kind: pulumi.String("StorageV2"),
		}
		storageAccount, err := storage.NewStorageAccount(ctx, siteKey+envKey, &storageAccountArgs)
		if err != nil {
			fmt.Printf("ERROR: creating storageAccount %s failed\n",
				siteKey+envKey)
			return err
		}

		// Enable static website support for the Storage Account
		storageArgs := storage.StorageAccountStaticWebsiteArgs{
			AccountName:       storageAccount.Name,
			ResourceGroupName: webResourceGrp.Name,
			IndexDocument:     pulumi.String("index.html"),
			Error404Document:  pulumi.String("404.hml"),
		}
		_, err = storage.NewStorageAccountStaticWebsite(ctx, siteKey, &storageArgs)
		if err != nil {
			fmt.Printf("ERROR: creating staticWebsite %s failed\n",
				siteKey)
			return err
		}

		// Create CDN Profile for usage by our endpoint(s)
		cdnProfileArgs := nativecdn.ProfileArgs{
			Location:          pulumi.String("global"),
			ResourceGroupName: webResourceGrp.Name,
			Sku: &nativecdn.SkuArgs{
				Name: pulumi.String("Standard_Microsoft"),
			},
		}
		cdnProfile, err := nativecdn.NewProfile(ctx, siteKey+envKey, &cdnProfileArgs)
		if err != nil {
			fmt.Printf("ERROR: creating cdnProfile %s failed\n",
				siteKey+envKey)
			return err
		}

		// extract 'https://' and '/' from web endpoint address for the Storage Account's static website
		webEndptStr := storageAccount.PrimaryEndpoints.Web()
		staticEndpoint := webEndptStr.ApplyT(func(url string) string {
			return strings.ReplaceAll(strings.ReplaceAll(url, "https://", ""), "/", "")
		}).(pulumi.StringOutput)

		// Create CDN Endpoint using newly created CDN Profile
		originsArgs := nativecdn.DeepCreatedOriginArray{
			nativecdn.DeepCreatedOriginArgs{
				Enabled:  pulumi.Bool(true),
				HostName: staticEndpoint,
				Name:     pulumi.String("origin1"),
			}}
		cdnEndPointArgs := nativecdn.EndpointArgs{
			Origins:                    originsArgs,
			ProfileName:                cdnProfile.Name,
			ResourceGroupName:          webResourceGrp.Name,
			OriginHostHeader:           staticEndpoint,
			IsHttpAllowed:              pulumi.Bool(false),
			IsHttpsAllowed:             pulumi.Bool(true),
			QueryStringCachingBehavior: nativecdn.QueryStringCachingBehaviorIgnoreQueryString,
			IsCompressionEnabled:       pulumi.Bool(true),
			ContentTypesToCompress: pulumi.StringArray{
				pulumi.String("text/plain"),
				pulumi.String("text/html"),
				pulumi.String("text/css"),
				pulumi.String("application/x-javascript"),
				pulumi.String("text/javascript"),
			},
		}
		endpoint, err := nativecdn.NewEndpoint(ctx, siteKey+envKey, &cdnEndPointArgs)
		if err != nil {
			fmt.Printf("ERROR: creating endpoint %s failed\n",
				siteKey+envKey)
			return err
		}

		// Update DNS CNAME record for envKey.elyclover.com to point at CDN endpoint
		// first look up zone where records are stored in separate Resource Group
		dnsRG := cfg.Require("dnsResourceGroup")
		dnsLookupZoneArgs := dns.LookupZoneArgs{
			Name:              cfg.Require("dnsZoneName"),
			ResourceGroupName: &dnsRG,
		}
		dnsZone, err := dns.LookupZone(ctx, &dnsLookupZoneArgs)
		if err != nil {
			fmt.Printf("ERROR: looking up dnsZone in RG %s failed\n",
				dnsRG)
			return err
		}

		// create new CNAME record in zone for env that will be used by CDN endpoint
		dnsRecordArgs := dns.CNameRecordArgs{
			ZoneName:          pulumi.String(dnsZone.Name),
			ResourceGroupName: pulumi.String(dnsRG),
			Ttl:               pulumi.Int(300), // 5 minutes
			Name:              pulumi.String(envKey),
			Record:            endpoint.HostName,
		}
		cnameRecord, err := dns.NewCNameRecord(ctx, siteKey+envKey, &dnsRecordArgs)
		if err != nil {
			fmt.Printf("ERROR: creating CNAME record in RG %s failed\n",
				dnsRG)
			return err
		}

		// strip out trailing '.' from CNAME's returned FQDN string within Azure DNS API
		cnameFqdnHappy := cnameRecord.Fqdn.ApplyT(func(fqdn string) (string, error) {
			h, found := strings.CutSuffix(fqdn, ".")
			err = fmt.Errorf("passed FQDN string didn't include trailing '.' did the Azure API change?")
			if !found {
				return h, err
			}
			return h, nil
		}).(pulumi.StringOutput)

		// Add Custom Domain to CDN to set up automatic TLS termination/cert rotation
		// Utilize the azure legacy provider since it supports setting up auto-TLS for CDN custom domains
		// azure-native provider strangely lacks support for CDN-managed TLS on custom domains... pushing front door? $$$
		cdnManagedHttps := legacycdn.EndpointCustomDomainCdnManagedHttpsArgs{
			CertificateType: pulumi.String("Dedicated"),
			ProtocolType:    pulumi.String("ServerNameIndication"),
			TlsVersion:      pulumi.String("TLS12"),
		}
		endpointCustomDomainArgs := legacycdn.EndpointCustomDomainArgs{
			CdnEndpointId:   endpoint.ID(),
			HostName:        cnameFqdnHappy,
			CdnManagedHttps: &cdnManagedHttps,
		}
		_, err = legacycdn.NewEndpointCustomDomain(ctx, siteKey+envKey, &endpointCustomDomainArgs)
		if err != nil {
			fmt.Println("ERROR: creating custom domain for CDN endpoint failed")
		}

		return nil
	})
}
