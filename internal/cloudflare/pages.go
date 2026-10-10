package cloudflare

import (
	"fmt"

	cf "github.com/pulumi/pulumi-cloudflare/sdk/v6/go/cloudflare"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"elyclover.com-infra/internal/config"
)

const pagesProjectLogicalName = "pages-project"

// domainSpec describes one custom domain of the Pages project.
type domainSpec struct {
	key    string // "apex" or the preview subdomain; suffix of the logical names
	fqdn   string // the custom domain
	target string // the *.pages.dev host its CNAME points at
}

// customDomainSpecs lists the apex plus one domain per preview subdomain, or
// nothing when custom domains are disabled. A preview subdomain is also the
// Pages branch alias, so its target is <sub>.<project>.pages.dev.
func customDomainSpecs(cfg *config.Config) []domainSpec {
	if !cfg.EnableCustomDomains {
		return nil
	}
	pagesHost := cfg.PagesProjectName + ".pages.dev"
	specs := []domainSpec{{key: "apex", fqdn: cfg.Domain, target: pagesHost}}
	for _, sub := range cfg.PagesPreviewSubdomains {
		specs = append(specs, domainSpec{key: sub, fqdn: sub + "." + cfg.Domain, target: sub + "." + pagesHost})
	}
	return specs
}

// NewPagesProject creates the Direct Upload Pages project (no git source).
func NewPagesProject(ctx *pulumi.Context, cfg *config.Config) (*cf.PagesProject, error) {
	project, err := cf.NewPagesProject(ctx, pagesProjectLogicalName, &cf.PagesProjectArgs{
		AccountId:        pulumi.String(cfg.CloudflareAccountID),
		Name:             pulumi.String(cfg.PagesProjectName),
		ProductionBranch: pulumi.String(cfg.PagesProductionBranch),
	})
	if err != nil {
		return nil, fmt.Errorf("creating pages project %q: %w", cfg.PagesProjectName, err)
	}
	return project, nil
}

// newPagesDomains attaches each custom domain to the project. The result is
// parallel to specs.
func newPagesDomains(ctx *pulumi.Context, cfg *config.Config, project *cf.PagesProject, specs []domainSpec) ([]*cf.PagesDomain, error) {
	domains := make([]*cf.PagesDomain, 0, len(specs))
	for _, s := range specs {
		d, err := cf.NewPagesDomain(ctx, "pages-domain-"+s.key, &cf.PagesDomainArgs{
			AccountId:   pulumi.String(cfg.CloudflareAccountID),
			ProjectName: pulumi.String(cfg.PagesProjectName),
			Name:        pulumi.String(s.fqdn),
		}, pulumi.DependsOn([]pulumi.Resource{project}))
		if err != nil {
			return nil, fmt.Errorf("creating pages domain %s: %w", s.fqdn, err)
		}
		domains = append(domains, d)
	}
	return domains, nil
}

// NewCustomDomains creates the PagesDomains and their DNS CNAMEs, but only when
// enableCustomDomains is true. It returns the custom domain names (empty when
// disabled).
func NewCustomDomains(ctx *pulumi.Context, cfg *config.Config, zone *cf.Zone, project *cf.PagesProject) ([]string, error) {
	specs := customDomainSpecs(cfg)
	domains, err := newPagesDomains(ctx, cfg, project, specs)
	if err != nil {
		return nil, err
	}
	if err := newPagesCNAMEs(ctx, zone, specs, domains); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(specs))
	for _, s := range specs {
		names = append(names, s.fqdn)
	}
	return names, nil
}
