package main

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-azure-native-sdk/cdn/v2"
	"github.com/pulumi/pulumi-azure/sdk/v5/go/azure/dns"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func lookupDnsZone(ctx *pulumi.Context, rg string, lz string) (lzResult *dns.LookupZoneResult, err error) {
	dnsLookupZoneArgs := dns.LookupZoneArgs{
		Name:              lz,
		ResourceGroupName: &rg,
	}
	lzResult, err = dns.LookupZone(ctx, &dnsLookupZoneArgs)
	if err != nil {
		fmt.Printf("ERROR: looking up dnsZone in RG %s failed\n", rg)
		return lzResult, err
	}
	return
}

func createDnsRecordByEnv(ctx *pulumi.Context, dnsRG string, dz *dns.LookupZoneResult, ep *cdn.Endpoint, envKey string, siteKey string) (d pulumi.StringOutput, err error) {
	fqdnErr := fmt.Errorf("passed FQDN string didn't include trailing '.' did the Azure API change?")
	switch envKey {
	case "prod": // apex domain for prod eg tld.com requires A record referencing Azure resource
		// create A record pointing at CDN Endpoint resource ID
		dnsRecord, err := createARecordPointingAtCdnResourceID(ctx, dnsRG, dz, pulumi.StringOutput(ep.ID()), envKey, siteKey)
		if err != nil {
			return d, err
		}
		// strip out trailing '.' from CNAME's returned FQDN string within Azure DNS API
		d = dnsRecord.Fqdn.ApplyT(func(fqdn string) (string, error) {
			h, found := strings.CutSuffix(fqdn, ".")
			if !found {
				return h, fqdnErr
			}
			return h, nil
		}).(pulumi.StringOutput)
	default: // everything that's not prod and has a sub-domain eg dev.tld.com
		// create CNAME DNS record to point at CDN endpoint
		dnsRecord, err := createCNAMERecordPointingAtCdnEndpoint(ctx, dnsRG, dz, ep.HostName, envKey, siteKey)
		if err != nil {
			return d, err
		}
		// strip out trailing '.' from CNAME's returned FQDN string within Azure DNS API
		d = dnsRecord.Fqdn.ApplyT(func(fqdn string) (string, error) {
			h, found := strings.CutSuffix(fqdn, ".")
			if !found {
				return h, fqdnErr
			}
			return h, nil
		}).(pulumi.StringOutput)
	}
	return
}

func createCNAMERecordPointingAtCdnEndpoint(ctx *pulumi.Context, dnsRG string, dz *dns.LookupZoneResult, ep pulumi.StringOutput, envKey string, siteKey string) (record *dns.CNameRecord, err error) {
	// create new CNAME record in zone for non-prod env that will be used by CDN endpoint
	dnsRecordArgs := dns.CNameRecordArgs{
		ZoneName:          pulumi.String(dz.Name),
		ResourceGroupName: pulumi.String(dnsRG),
		Ttl:               pulumi.Int(300), // 5 minutes
		Name:              pulumi.String(envKey),
		Record:            ep,
	}
	record, err = dns.NewCNameRecord(ctx, siteKey+envKey, &dnsRecordArgs)
	if err != nil {
		fmt.Printf("ERROR: creating CNAME record in RG %s failed\n",
			dnsRG)
		return record, err
	}
	return
}

func createARecordPointingAtCdnResourceID(ctx *pulumi.Context, dnsRG string, dz *dns.LookupZoneResult, tg pulumi.StringOutput, envKey string, siteKey string) (record *dns.ARecord, err error) {
	dnsRecordArgs := dns.ARecordArgs{
		//Name:            pulumi.String(envKey),
		Name:              pulumi.String("@"),
		ZoneName:          pulumi.String(dz.Name),
		ResourceGroupName: pulumi.String(dnsRG),
		Ttl:               pulumi.Int(300), // 5 minutes
		TargetResourceId:  tg,
	}
	record, err = dns.NewARecord(ctx, siteKey+envKey, &dnsRecordArgs)
	if err != nil {
		fmt.Printf("ERROR: creating A record in RG %s failed\n",
			dnsRG)
		return record, err
	}
	return
}
