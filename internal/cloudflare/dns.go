package cloudflare

import (
	"fmt"
	"strings"

	cf "github.com/pulumi/pulumi-cloudflare/sdk/v6/go/cloudflare"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"elyclover.com-infra/internal/config"
)

// autoTTL is Cloudflare's "automatic" TTL.
const autoTTL = 1

// emailRecordName returns the logical resource name for the i-th email record.
func emailRecordName(i int, recordType string) string {
	return fmt.Sprintf("dns-email-%d-%s", i, strings.ToLower(recordType))
}

// NewEmailRecords creates one unproxied DNS record per configured email record.
// TXT contents are quoted for Cloudflare; MX records carry their priority.
func NewEmailRecords(ctx *pulumi.Context, cfg *config.Config, zone *cf.Zone) error {
	for i, r := range cfg.EmailRecords {
		args := &cf.DnsRecordArgs{
			ZoneId:  zone.ID().ToStringOutput(),
			Name:    pulumi.String(r.Name),
			Type:    pulumi.String(r.Type),
			Content: pulumi.String(r.Content),
			Ttl:     pulumi.Float64(autoTTL),
			Proxied: pulumi.Bool(false),
		}
		switch r.Type {
		case "TXT":
			args.Content = pulumi.String(config.QuoteTXT(r.Content))
		case "MX":
			if r.Priority == nil {
				return fmt.Errorf("email record %d (MX %s) has no priority", i, r.Name)
			}
			args.Priority = pulumi.Float64Ptr(float64(*r.Priority))
		}
		if _, err := cf.NewDnsRecord(ctx, emailRecordName(i, r.Type), args); err != nil {
			return fmt.Errorf("creating email record %d (%s %s): %w", i, r.Type, r.Name, err)
		}
	}
	return nil
}

// newPagesCNAMEs points each custom domain at the Pages project through a
// proxied CNAME. Each record waits for its matching PagesDomain.
func newPagesCNAMEs(ctx *pulumi.Context, zone *cf.Zone, specs []domainSpec, domains []*cf.PagesDomain) error {
	for i, s := range specs {
		if _, err := cf.NewDnsRecord(ctx, "dns-pages-"+s.key, &cf.DnsRecordArgs{
			ZoneId:  zone.ID().ToStringOutput(),
			Name:    pulumi.String(s.fqdn),
			Type:    pulumi.String("CNAME"),
			Content: pulumi.String(s.target),
			Ttl:     pulumi.Float64(autoTTL),
			Proxied: pulumi.Bool(true),
		}, pulumi.DependsOn([]pulumi.Resource{domains[i]})); err != nil {
			return fmt.Errorf("creating CNAME for %s: %w", s.fqdn, err)
		}
	}
	return nil
}
