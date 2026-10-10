package cloudflare

import (
	cf "github.com/pulumi/pulumi-cloudflare/sdk/v6/go/cloudflare"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"elyclover.com-infra/internal/config"
)

// Result holds the handles other packages and the exports need.
type Result struct {
	Zone    *cf.Zone
	Project *cf.PagesProject
	Token   *cf.AccountToken
	// CustomDomains lists the custom domain names; empty when disabled.
	CustomDomains []string
}

// Provision creates every Cloudflare resource, in dependency order: zone and
// settings, email records, Pages project, (gated) custom domains and CNAMEs,
// and finally the CI deploy token.
func Provision(ctx *pulumi.Context, cfg *config.Config) (*Result, error) {
	zone, err := NewZone(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := NewEmailRecords(ctx, cfg, zone); err != nil {
		return nil, err
	}
	project, err := NewPagesProject(ctx, cfg)
	if err != nil {
		return nil, err
	}
	domains, err := NewCustomDomains(ctx, cfg, zone, project)
	if err != nil {
		return nil, err
	}
	token, err := NewDeployToken(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Result{Zone: zone, Project: project, Token: token, CustomDomains: domains}, nil
}
