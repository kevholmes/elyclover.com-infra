// Package github wires the repositories to the Cloudflare deployment: Actions
// secrets and variables, deployment environments and branch protections. The
// provider owner comes from the github:owner config; the token from the
// GITHUB_TOKEN environment variable.
package github

import (
	"fmt"

	gh "github.com/pulumi/pulumi-github/sdk/v6/go/github"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"elyclover.com-infra/internal/config"
)

// siteEnvironments are the deployment environments of the site repo.
var siteEnvironments = []string{"dev", "stg", "prod"}

// Provision creates every GitHub resource. cloudflareAPIToken is the value of
// the Cloudflare deploy token; it is passed through unchanged so it stays secret.
func Provision(ctx *pulumi.Context, cfg *config.Config, cloudflareAPIToken pulumi.StringInput) error {
	if err := newSecrets(ctx, cfg, cloudflareAPIToken); err != nil {
		return err
	}
	if err := newEnvironments(ctx, cfg); err != nil {
		return err
	}
	if err := newMainProtection(ctx, "site-main-protection", cfg.SiteRepo, cfg.SiteRequiredChecks); err != nil {
		return err
	}
	return newMainProtection(ctx, "infra-main-protection", cfg.InfraRepo, cfg.InfraRequiredChecks)
}

// newSecrets creates the Actions secrets and the account ID variable.
func newSecrets(ctx *pulumi.Context, cfg *config.Config, cloudflareAPIToken pulumi.StringInput) error {
	secrets := []struct {
		logicalName string
		repo        string
		name        string
		value       pulumi.StringInput
	}{
		{"site-secret-cloudflare-api-token", cfg.SiteRepo, "CLOUDFLARE_API_TOKEN", cloudflareAPIToken},
		{"site-secret-releaser-token", cfg.SiteRepo, "RELEASER_TOKEN", cfg.ReleaserToken},
		{"infra-secret-releaser-token", cfg.InfraRepo, "RELEASER_TOKEN", cfg.ReleaserToken},
	}
	for _, s := range secrets {
		if _, err := gh.NewActionsSecret(ctx, s.logicalName, &gh.ActionsSecretArgs{
			Repository: pulumi.String(s.repo),
			SecretName: pulumi.String(s.name),
			Value:      s.value,
		}); err != nil {
			return fmt.Errorf("creating secret %s in %s: %w", s.name, s.repo, err)
		}
	}

	if _, err := gh.NewActionsVariable(ctx, "site-var-cloudflare-account-id", &gh.ActionsVariableArgs{
		Repository:   pulumi.String(cfg.SiteRepo),
		VariableName: pulumi.String("CLOUDFLARE_ACCOUNT_ID"),
		Value:        pulumi.String(cfg.CloudflareAccountID),
	}); err != nil {
		return fmt.Errorf("creating variable CLOUDFLARE_ACCOUNT_ID in %s: %w", cfg.SiteRepo, err)
	}
	return nil
}

// newEnvironments creates the dev, stg and prod environments in the site repo.
func newEnvironments(ctx *pulumi.Context, cfg *config.Config) error {
	for _, env := range siteEnvironments {
		if _, err := gh.NewRepositoryEnvironment(ctx, "site-env-"+env, &gh.RepositoryEnvironmentArgs{
			Repository:  pulumi.String(cfg.SiteRepo),
			Environment: pulumi.String(env),
		}); err != nil {
			return fmt.Errorf("creating environment %s in %s: %w", env, cfg.SiteRepo, err)
		}
	}
	return nil
}

// newMainProtection protects the main branch of repo: PR required (0 approvals),
// up-to-date branch with the given status checks, resolved conversations, no
// force pushes or deletions, and admins not forced (so a solo maintainer is
// never locked out).
func newMainProtection(ctx *pulumi.Context, logicalName, repo string, checks []string) error {
	info, err := gh.LookupRepository(ctx, &gh.LookupRepositoryArgs{Name: &repo})
	if err != nil {
		return fmt.Errorf("looking up repository %s: %w", repo, err)
	}

	if _, err := gh.NewBranchProtection(ctx, logicalName, &gh.BranchProtectionArgs{
		RepositoryId: pulumi.String(info.NodeId),
		Pattern:      pulumi.String("main"),
		RequiredStatusChecks: gh.BranchProtectionRequiredStatusCheckArray{
			gh.BranchProtectionRequiredStatusCheckArgs{
				Strict:   pulumi.Bool(true),
				Contexts: pulumi.ToStringArray(checks),
			},
		},
		RequiredPullRequestReviews: gh.BranchProtectionRequiredPullRequestReviewArray{
			gh.BranchProtectionRequiredPullRequestReviewArgs{
				RequiredApprovingReviewCount: pulumi.Int(0),
			},
		},
		RequireConversationResolution: pulumi.Bool(true),
		EnforceAdmins:                 pulumi.Bool(false),
		AllowsForcePushes:             pulumi.Bool(false),
		AllowsDeletions:               pulumi.Bool(false),
	}); err != nil {
		return fmt.Errorf("protecting main in %s: %w", repo, err)
	}
	return nil
}
