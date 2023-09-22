package main

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-azure-native-sdk/authorization/v2"
	nativecdn "github.com/pulumi/pulumi-azure-native-sdk/cdn/v2"
	legacycdn "github.com/pulumi/pulumi-azure/sdk/v5/go/azure/cdn"
	"github.com/pulumi/pulumi-azuread/sdk/v5/go/azuread"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func createCdnProfile(ctx *pulumi.Context, cdnName string, azureRg pulumi.StringOutput) (profile *nativecdn.Profile, err error) {
	var cdnProfileArgs = nativecdn.ProfileArgs{
		Location:          pulumi.String("global"),
		ResourceGroupName: azureRg,
		Sku: &nativecdn.SkuArgs{
			Name: pulumi.String("Standard_Microsoft"),
		},
	}
	profile, err = nativecdn.NewProfile(ctx, cdnName, &cdnProfileArgs)
	if err != nil {
		fmt.Printf("ERROR: creating cdnProfile %s failed\n", cdnName)
		return nil, err
	}
	return
}

func createCdnEndpoint(ctx *pulumi.Context, epName string, cdnProfile *nativecdn.Profile, azureRg pulumi.StringOutput, origin pulumi.StringOutput) (ep *nativecdn.Endpoint, err error) {
	// Create CDN Endpoint using newly created CDN Profile
	originsArgs := nativecdn.DeepCreatedOriginArray{
		nativecdn.DeepCreatedOriginArgs{
			Enabled:  pulumi.Bool(true),
			HostName: origin,
			Name:     pulumi.String("origin1"),
		}}
	// set up single delivery rule which forwards all HTTP traffic to HTTPS on CDN endpoint
	//lint:ignore S1025 external type doesn't have compatible string method
	https := fmt.Sprintf("%s", nativecdn.DestinationProtocolHttps)
	//lint:ignore S1025 external type doesn't have compatible string method
	http := fmt.Sprintf("%s", nativecdn.DestinationProtocolHttp)
	deliveryRuleName := pulumi.Sprintf("%sTo%s", http, https)
	deliveryRule := nativecdn.DeliveryRuleArgs{
		Name:  deliveryRuleName,
		Order: pulumi.Int(1),
		Conditions: pulumi.All(nativecdn.DeliveryRuleRequestSchemeCondition{
			Name: "RequestScheme",
			Parameters: nativecdn.RequestSchemeMatchConditionParameters{
				MatchValues: []string{strings.ToUpper(http)},
				Operator:    "Equal",
				TypeName:    "DeliveryRuleRequestSchemeConditionParameters",
			},
		}),
		Actions: pulumi.All(nativecdn.UrlRedirectAction{
			Name: "UrlRedirect",
			Parameters: nativecdn.UrlRedirectActionParameters{
				RedirectType:        "Found",
				DestinationProtocol: &https,
				TypeName:            "DeliveryRuleUrlRedirectActionParameters",
			},
		}),
	}
	deliveryPolicy := nativecdn.EndpointPropertiesUpdateParametersDeliveryPolicyArgs{
		Description: pulumi.String("delivery policy that forwards all http to https at CDN-level"),
		Rules:       nativecdn.DeliveryRuleArray{deliveryRule},
	}
	cdnEndPointArgs := nativecdn.EndpointArgs{
		Origins:                    originsArgs,
		ProfileName:                cdnProfile.Name,
		ResourceGroupName:          azureRg,
		OriginHostHeader:           origin,
		IsHttpAllowed:              pulumi.Bool(true),
		IsHttpsAllowed:             pulumi.Bool(true),
		DeliveryPolicy:             deliveryPolicy,
		QueryStringCachingBehavior: nativecdn.QueryStringCachingBehaviorIgnoreQueryString,
		IsCompressionEnabled:       pulumi.Bool(true),
		ContentTypesToCompress: pulumi.StringArray{
			pulumi.String("text/plain"),
			pulumi.String("text/html"),
			pulumi.String("text/css"),
			pulumi.String("text/javascript"),
			pulumi.String("application/x-javascript"),
			pulumi.String("application/javascript"),
			pulumi.String("application/json"),
			pulumi.String("image/svg+xml"),
		},
	}
	ep, err = nativecdn.NewEndpoint(ctx, epName, &cdnEndPointArgs)
	if err != nil {
		fmt.Printf("ERROR: creating endpoint %s failed\n", epName)
		return ep, err
	}
	return
}

func newEndpointCustomDomain(ctx *pulumi.Context, epdName string, endpoint *nativecdn.Endpoint, domain pulumi.StringOutput) (epd *legacycdn.EndpointCustomDomain, err error) {
	// Utilize the azure legacy provider since it supports setting up auto-TLS for CDN custom domains
	// azure-native provider strangely lacks support for CDN-managed TLS on custom domains... pushing front door? $$$
	cdnManagedHttps := legacycdn.EndpointCustomDomainCdnManagedHttpsArgs{
		CertificateType: pulumi.String("Dedicated"),
		ProtocolType:    pulumi.String("ServerNameIndication"),
		TlsVersion:      pulumi.String("TLS12"),
	}
	endpointCustomDomainArgs := legacycdn.EndpointCustomDomainArgs{
		CdnEndpointId:   endpoint.ID(),
		HostName:        domain,
		CdnManagedHttps: &cdnManagedHttps,
	}
	epd, err = legacycdn.NewEndpointCustomDomain(ctx, epdName, &endpointCustomDomainArgs)
	if err != nil {
		fmt.Println("ERROR: creating custom domain for CDN endpoint failed")
		return epd, err
	}
	return
}

// Non-prod environments use sub-domains (eg dev.tld.com) and Azure CDN (classic) will auto-generate and set up the TLS for us
// Production uses an apex domain (eg tld.com) which Azure doesn't support free TLS certs + rotation on (:shrug:)
func setupTlsTermination(ctx *pulumi.Context, cfg *config.Config, ep *nativecdn.Endpoint, fqdn pulumi.StringOutput) (err error) {
	switch ctx.Stack() {
	case "prod":
		// Register Azure CDN Application as Service Principal in AD/Entra tenant so it can fetch TLS pfx data in external Keystore
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
		// This allows Azure CDN to access the pfx keys we purchase/generate external to pulumi (until ACME support is native)
		_, err = authorization.NewRoleAssignment(ctx, "AzureFDCDNreadKVCerts", &authorization.RoleAssignmentArgs{
			PrincipalId:      nsp.ID(),
			PrincipalType:    pulumi.String("ServicePrincipal"),
			RoleDefinitionId: pulumi.String("/providers/Microsoft.Authorization/roleDefinitions/4633458b-17de-408a-b874-0445c86b69e6"),
			Scope:            keyVaultScope,
		})
		if err != nil {
			return err
		}
	default:
		// newEndpointCustomDomain() will need to be refactored to support conditional for auto-tls setup
		// Add Custom Domain to CDN to set up automatic TLS termination/cert rotation
		_, err = newEndpointCustomDomain(ctx, cfg.Require("siteKey")+ctx.Stack(), ep, fqdn)
		if err != nil {
			return err
		}
	}

	return
}
