package main

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-azure-native-sdk/authorization/v2"
	nativecdn "github.com/pulumi/pulumi-azure-native-sdk/cdn/v2"
	legacycdn "github.com/pulumi/pulumi-azure/sdk/v5/go/azure/cdn"
	"github.com/pulumi/pulumi-azuread/sdk/v5/go/azuread"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type epCfgs struct {
	userManaged legacycdn.EndpointCustomDomainUserManagedHttpsArgs
	cdnManaged  legacycdn.EndpointCustomDomainCdnManagedHttpsArgs
	domainArgs  legacycdn.EndpointCustomDomainArgs
}

func (pr *projectResources) createCdnProfile() (err error) {
	var cdnProfileArgs = nativecdn.ProfileArgs{
		Location:          pulumi.String("global"),
		ResourceGroupName: pr.webResourceGrp.Name,
		Sku: &nativecdn.SkuArgs{
			Name: pulumi.String("Standard_Microsoft"),
		},
	}
	cdnName := pr.cfgKeys.siteKey + pr.cfgKeys.envKey
	pr.webCdnProfile, err = nativecdn.NewProfile(pr.pulumiCtx, cdnName, &cdnProfileArgs)
	if err != nil {
		fmt.Printf("ERROR: creating cdnProfile %s failed\n", cdnName)
		return err
	}
	return
}

func (pr *projectResources) createCdnEndpoint() (err error) {
	// Create CDN Endpoint using newly created CDN Profile
	originsArgs := nativecdn.DeepCreatedOriginArray{
		nativecdn.DeepCreatedOriginArgs{
			Enabled:  pulumi.Bool(true),
			HostName: pr.webStaticEp,
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
		ProfileName:                pr.webCdnProfile.Name,
		ResourceGroupName:          pr.webResourceGrp.Name,
		OriginHostHeader:           pr.webStaticEp,
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
	epName := pr.cfgKeys.siteKey + pr.cfgKeys.envKey
	pr.webCdnEp, err = nativecdn.NewEndpoint(pr.pulumiCtx, epName, &cdnEndPointArgs)
	if err != nil {
		fmt.Printf("ERROR: creating endpoint %s failed\n", epName)
		return err
	}
	return
}

func (pr *projectResources) newEndpointCustomDomain() (err error) {
	// Utilize the azure legacy provider since it supports setting up auto-TLS for CDN custom domains
	// azure-native provider strangely lacks support for CDN-managed TLS on custom domains...
	epCfg := epCfgs{
		userManaged: legacycdn.EndpointCustomDomainUserManagedHttpsArgs{
			TlsVersion: pulumi.String("TLS12"),
		},
		cdnManaged: legacycdn.EndpointCustomDomainCdnManagedHttpsArgs{
			CertificateType: pulumi.String("Dedicated"),
			ProtocolType:    pulumi.String("ServerNameIndication"),
			TlsVersion:      pulumi.String("TLS12"),
		},
		domainArgs: legacycdn.EndpointCustomDomainArgs{
			CdnEndpointId: pr.webCdnEp.ID(),
			HostName:      pr.webFqdn,
		},
	}

	switch pr.cfgKeys.envKey {
	// prod is byo certificate (self-managed)
	// all subdomains are ACME and managed by Azure (mostly)
	case PROD:
		// import pfx certificate stored at rest in source control to Azure Key Vault
		certSec, err := importPfxToKeyVault(pr.pulumiCtx, pr.cfg)
		if err != nil {
			return err
		}
		epCfg.userManaged.KeyVaultSecretId = certSec.Properties.SecretUri()
		epCfg.domainArgs.UserManagedHttps = epCfg.userManaged
	// azure cdn-managed certificate (auto-gen/rotated)
	default:
		epCfg.domainArgs.CdnManagedHttps = epCfg.cdnManaged
	}
	// create new CDN endpoint custom domain depending on prod/nonprod settings configured above
	// we ignore changes here to "cdnEndpointId" due to spurious "diffs" caused by a capital "G" in resourceGroup URIs
	// returned from Azure provider. I've found some bug reports in Terraform and Azure providers about this but I have
	// a feeling it's due to mix/match of azure and azure-native provider for our CDN work. By adding it we
	// avoid a constant cycle of Pulumi trying to destroy and re-create the Custom Domain which causes other
	// issues due to the reliance on the CNAME record which the provider does not (appear?) pick up on, unfortunately.
	// https://github.com/kevholmes/elyclover.com-infra/issues/76
	_, err = legacycdn.NewEndpointCustomDomain(
		pr.pulumiCtx, pr.cfgKeys.siteKey+pr.cfgKeys.envKey, &epCfg.domainArgs,
		pulumi.IgnoreChanges([]string{"cdnEndpointId"}))
	if err != nil {
		fmt.Println("ERROR: creating custom domain for CDN endpoint failed")
		return err
	}

	return
}

// Non-prod environments use sub-domains (eg dev.tld.com) and Azure CDN (classic) will auto-generate and set up the TLS for us.
// Production uses an apex domain (eg tld.com) which Azure doesn't support free TLS certs + rotation on (:shrug:)
// so we'll need to set up a Service Principal registered under the Azure CDN App profile and give it RBAC access to
// an external Azure KeyVault resource that contains our pfx certificate needed for the prod tld.com domain.
func (pr *projectResources) setupTlsTermination() (err error) {
	if pr.cfgKeys.envKey == PROD {
		// Register Azure CDN Application as Service Principal in AD/Entra tenant so it can fetch TLS pfx data in external Keystore
		nsp, err := azuread.NewServicePrincipal(pr.pulumiCtx, pr.cfgKeys.cdnAzureId, &azuread.ServicePrincipalArgs{
			ApplicationId: pulumi.String(pr.cfgKeys.cdnAzureId),
			UseExisting:   pulumi.Bool(false),
			Description:   pulumi.String("Service Principal tied to built-in Azure CDN/FD Application ID/product"),
		})
		if err != nil {
			return err
		}

		// assign predefined "Key Vault Secret User" RoleDefinitionId to Service Principal we just created
		// https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#key-vault-secrets-user
		// This allows Azure CDN to access the pfx keys we purchase/generate external to pulumi (quite a bit cheaper than asking Azure to do it.)
		keyVaultScope := pulumi.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.KeyVault/vaults/%s",
			pr.cfgKeys.kvAzureSubscription, pr.cfgKeys.kvAzureResourceGrp, pr.cfgKeys.kvAzureName)

		_, err = authorization.NewRoleAssignment(pr.pulumiCtx, "AzureFDCDNreadKVCerts", &authorization.RoleAssignmentArgs{
			PrincipalId:      nsp.ID(),
			PrincipalType:    pulumi.String("ServicePrincipal"),
			RoleDefinitionId: pulumi.String("/providers/Microsoft.Authorization/roleDefinitions/4633458b-17de-408a-b874-0445c86b69e6"),
			Scope:            keyVaultScope,
		})
		if err != nil {
			return err
		}
	}

	// Add Custom Domain to CDN for prod or non-prod
	err = pr.newEndpointCustomDomain()
	if err != nil {
		return err
	}

	return
}
