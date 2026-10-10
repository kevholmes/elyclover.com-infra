// Package infra holds the Pulumi program: it loads the configuration, creates
// the Cloudflare resources, then the GitHub ones that consume them, and exports
// the values the runbooks need.
package infra

import (
	"fmt"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"elyclover.com-infra/internal/cloudflare"
	"elyclover.com-infra/internal/config"
	"elyclover.com-infra/internal/github"
)

// expiryWarnDays is the threshold below which the RELEASER_TOKEN expiry warning fires.
const expiryWarnDays = 30

// Program is the Pulumi entrypoint.
func Program(ctx *pulumi.Context) error {
	return programWithClock(ctx, time.Now)
}

// programWithClock is Program with an injectable clock, so tests are deterministic.
func programWithClock(ctx *pulumi.Context, now func() time.Time) error {
	exports, err := build(ctx, now)
	if err != nil {
		return err
	}
	for name, value := range exports {
		ctx.Export(name, value)
	}
	return nil
}

// build validates the configuration and registers every resource, then returns
// the stack outputs by name. Nothing is registered when the configuration is
// invalid.
func build(ctx *pulumi.Context, now func() time.Time) (map[string]pulumi.Input, error) {
	cfg, err := config.Load(ctx)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	cf, err := cloudflare.Provision(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := github.Provision(ctx, cfg, cf.Token.Value); err != nil {
		return nil, err
	}

	days, err := config.DaysUntil(cfg.ReleaserTokenExpiresOn, now())
	if err != nil {
		return nil, err
	}
	if msg := expiryWarning(days); msg != "" {
		// A warning that cannot be delivered must not fail the deployment.
		_ = ctx.Log.Warn(msg, nil)
	}

	return map[string]pulumi.Input{
		"zoneId":                cf.Zone.ID().ToStringOutput(),
		"nameServers":           cf.Zone.NameServers,
		"pagesSubdomain":        cf.Project.Subdomain,
		"customDomainsEnabled":  pulumi.Bool(cfg.EnableCustomDomains),
		"releaserTokenDaysLeft": pulumi.Int(days),
		"customDomains":         pulumi.ToStringArray(cf.CustomDomains),
	}, nil
}

// expiryWarning returns the warning to log when RELEASER_TOKEN has fewer than
// 30 days left, or "" when there is nothing to warn about.
func expiryWarning(daysLeft int) string {
	if daysLeft >= expiryWarnDays {
		return ""
	}
	return fmt.Sprintf("RELEASER_TOKEN expires in %d days; see runbook", daysLeft)
}
