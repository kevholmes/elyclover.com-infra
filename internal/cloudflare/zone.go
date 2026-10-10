// Package cloudflare provisions the Cloudflare side of the site: the DNS
// zone and its settings, the carried-over email records, the Pages project
// with its (gated) custom domains, and the scoped API token CI uses to deploy.
package cloudflare

import (
	"fmt"

	cf "github.com/pulumi/pulumi-cloudflare/sdk/v6/go/cloudflare"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"elyclover.com-infra/internal/config"
)

// Logical names of the zone and its settings. Tests assert on them.
const zoneName = "zone"

// zoneSettings are applied to the zone, in this order.
var zoneSettings = []struct{ logicalName, settingID, value string }{
	{"zone-setting-ssl", "ssl", "strict"},
	{"zone-setting-always-use-https", "always_use_https", "on"},
	{"zone-setting-min-tls-version", "min_tls_version", "1.2"},
}

// NewZone creates the full-setup zone for the domain, protected against
// accidental deletion, and its three security settings.
func NewZone(ctx *pulumi.Context, cfg *config.Config) (*cf.Zone, error) {
	zone, err := cf.NewZone(ctx, zoneName, &cf.ZoneArgs{
		Account: cf.ZoneAccountArgs{Id: pulumi.String(cfg.CloudflareAccountID)},
		Name:    pulumi.String(cfg.Domain),
		Type:    pulumi.String("full"),
	}, pulumi.Protect(true))
	if err != nil {
		return nil, fmt.Errorf("creating zone %q: %w", cfg.Domain, err)
	}

	for _, s := range zoneSettings {
		if _, err := cf.NewZoneSetting(ctx, s.logicalName, &cf.ZoneSettingArgs{
			ZoneId:    zone.ID().ToStringOutput(),
			SettingId: pulumi.String(s.settingID),
			Value:     pulumi.String(s.value),
		}); err != nil {
			return nil, fmt.Errorf("creating zone setting %q: %w", s.settingID, err)
		}
	}
	return zone, nil
}
