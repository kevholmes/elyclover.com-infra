package main

import (
	"fmt"

	nativecdn "github.com/pulumi/pulumi-azure-native-sdk/cdn/v2"
	legacycdn "github.com/pulumi/pulumi-azure/sdk/v5/go/azure/cdn"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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
	cdnEndPointArgs := nativecdn.EndpointArgs{
		Origins:                    originsArgs,
		ProfileName:                cdnProfile.Name,
		ResourceGroupName:          azureRg,
		OriginHostHeader:           origin,
		IsHttpAllowed:              pulumi.Bool(true),
		IsHttpsAllowed:             pulumi.Bool(true),
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
