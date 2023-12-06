package main

import (
	"fmt"
	"strconv"
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
		err = pr.createApexRecordPointingAtCdnResourceID()
		if err != nil {
			return err
		}
		// create CNAME 'cdnverify.tld.com' record
		cdnVerify := "cdnverify"
		cdnVerifyHostname := pr.webCdnEp.HostName.ApplyT(func(h string) (r string) {
			r = cdnVerify + "." + h
			return
		}).(pulumi.StringOutput)
		err = pr.createCNAMERecordPointingAtCdnEndpoint(cdnVerifyHostname, pr.cfgKeys.siteKey+cdnVerify)
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
		err = pr.createCNAMERecordPointingAtCdnEndpoint(pr.webCdnEp.HostName, pr.cfgKeys.envKey)
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

func (pr *projectResources) createCNAMERecordPointingAtCdnEndpoint(ep pulumi.StringOutput, name string) (err error) {
	ttl, err := strconv.Atoi(pr.cfgKeys.dnsRecordTTL)
	if err != nil {
		fmt.Printf("ERROR: dnsRecordTTL provided cannot be converted from string to int\n")
		return err
	}
	// create new CNAME record in zone for non-prod env that will be used by CDN endpoint
	dnsRecordArgs := dns.CNameRecordArgs{
		ZoneName:          pulumi.String(pr.webDnsZone.Name),
		ResourceGroupName: pulumi.String(pr.cfgKeys.dnsResourceGrp),
		Ttl:               pulumi.Int(ttl),
		Name:              pulumi.String(name),
		Record:            ep,
	}

	pr.dnsRecords.cname, err = dns.NewCNameRecord(pr.pulumiCtx, name, &dnsRecordArgs)
	if err != nil {
		fmt.Printf("ERROR: creating CNAME record in RG %s failed\n",
			pr.cfgKeys.dnsResourceGrp)
		return err
	}
	return
}

func (pr *projectResources) createApexRecordPointingAtCdnResourceID() (err error) {
	ttl, err := strconv.Atoi(pr.cfgKeys.dnsRecordTTL)
	if err != nil {
		fmt.Printf("ERROR: dnsRecordTTL provided cannot be converted from string to int\n")
		return err
	}
	rootRecordName := "@"
	dnsRecordArgs := dns.ARecordArgs{
		Name:              pulumi.String(rootRecordName),
		ZoneName:          pulumi.String(pr.webDnsZone.Name),
		ResourceGroupName: pulumi.String(pr.cfgKeys.dnsResourceGrp),
		Ttl:               pulumi.Int(ttl),
		TargetResourceId:  pulumi.StringOutput(pr.webCdnEp.ID()),
	}
	name := pr.cfgKeys.siteKey + pr.cfgKeys.envKey
	pr.dnsRecords.a, err = dns.NewARecord(pr.pulumiCtx, name, &dnsRecordArgs)
	if err != nil {
		fmt.Printf("ERROR: creating A record in RG %s failed\n",
			pr.cfgKeys.dnsResourceGrp)
		return err
	}
	return
}
