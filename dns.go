package main

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-azure/sdk/v5/go/azure/dns"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func (pr *projectResources) lookupDnsZone() (err error) {
	dnsLookupZoneArgs := dns.LookupZoneArgs{
		Name:              pr.cfgKeys.dnsLookupZone,
		ResourceGroupName: &pr.cfgKeys.dnsResourceGrp,
	}
	pr.webDnsZone, err = dns.LookupZone(pr.pulumiCtx, &dnsLookupZoneArgs)
	if err != nil {
		fmt.Printf("ERROR: looking up dnsZone in RG %s failed\n", pr.cfgKeys.dnsResourceGrp)
		return err
	}
	return
}

func (pr *projectResources) createDnsRecordByEnv() (err error) {
	fqdnErr := fmt.Errorf("passed FQDN string didn't include trailing '.' did the Azure API change?")
	switch pr.cfgKeys.envKey {
	case PROD: // apex domain for prod eg tld.com uses A record referencing Azure resource
		// create A record pointing at CDN Endpoint resource ID
		pr.dnsRecords.a, err = pr.createARecordPointingAtCdnResourceID()
		if err != nil {
			return err
		}
		// create CNAME 'cdnverify.tld.com' record
		cdnVerify := "cdnverify"
		cdnVerifyHostname := pr.webCdnEp.HostName.ApplyT(func(h string) (r string) {
			r = cdnVerify + "." + h
			return
		}).(pulumi.StringOutput)
		pr.dnsRecords.cname, err = pr.createCNAMERecordPointingAtCdnEndpoint(cdnVerifyHostname)
		if err != nil {
			return err
		}
		// strip out trailing '.' from A record returned FQDN string within Azure DNS API
		pr.webFqdn = pr.dnsRecords.a.Fqdn.ApplyT(func(fqdn string) (string, error) {
			h, found := strings.CutSuffix(fqdn, ".")
			if !found {
				return h, fqdnErr
			}
			return h, nil
		}).(pulumi.StringOutput)
	default: // everything that's not prod and has a sub-domain eg dev.tld.com
		// create CNAME DNS record to point at CDN endpoint
		pr.dnsRecords.cname, err = pr.createCNAMERecordPointingAtCdnEndpoint(pr.webCdnEp.HostName)
		if err != nil {
			return err
		}
		// strip out trailing '.' from CNAME's returned FQDN string within Azure DNS API
		pr.webFqdn = pr.dnsRecords.cname.Fqdn.ApplyT(func(fqdn string) (string, error) {
			h, found := strings.CutSuffix(fqdn, ".")
			if !found {
				return h, fqdnErr
			}
			return h, nil
		}).(pulumi.StringOutput)
	}
	return
}

func (pr *projectResources) createCNAMERecordPointingAtCdnEndpoint(ep pulumi.StringOutput) (record *dns.CNameRecord, err error) {
	// create new CNAME record in zone for non-prod env that will be used by CDN endpoint
	dnsRecordArgs := dns.CNameRecordArgs{
		ZoneName:          pulumi.String(pr.webDnsZone.Name),
		ResourceGroupName: pulumi.String(pr.cfgKeys.dnsResourceGrp),
		Ttl:               pulumi.Int(300), // 5 minutes
		Name:              pulumi.String(pr.cfgKeys.envKey),
		Record:            ep,
	}
	name := pr.cfgKeys.siteKey + pr.cfgKeys.envKey
	record, err = dns.NewCNameRecord(pr.pulumiCtx, name, &dnsRecordArgs)
	if err != nil {
		fmt.Printf("ERROR: creating CNAME record in RG %s failed\n",
			pr.cfgKeys.dnsResourceGrp)
		return record, err
	}
	return
}

func (pr *projectResources) createARecordPointingAtCdnResourceID() (record *dns.ARecord, err error) {
	dnsRecordArgs := dns.ARecordArgs{
		Name:              pulumi.String("@"),
		ZoneName:          pulumi.String(pr.webDnsZone.Name),
		ResourceGroupName: pulumi.String(pr.cfgKeys.dnsResourceGrp),
		Ttl:               pulumi.Int(300), // 5 minutes
		TargetResourceId:  pulumi.StringOutput(pr.webCdnEp.ID()),
	}
	name := pr.cfgKeys.siteKey + pr.cfgKeys.envKey
	record, err = dns.NewARecord(pr.pulumiCtx, name, &dnsRecordArgs)
	if err != nil {
		fmt.Printf("ERROR: creating A record in RG %s failed\n",
			pr.cfgKeys.dnsResourceGrp)
		return record, err
	}
	return
}
