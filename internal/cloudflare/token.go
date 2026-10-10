package cloudflare

import (
	"encoding/json"
	"fmt"

	cf "github.com/pulumi/pulumi-cloudflare/sdk/v6/go/cloudflare"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"elyclover.com-infra/internal/config"
)

const (
	deployTokenLogicalName = "ci-deploy-token"
	deployTokenName        = "github-actions-elyclover-pages-deploy"
)

// findPermissionGroup returns the ID of the group with exactly the given name.
func findPermissionGroup(groups []cf.GetAccountApiTokenPermissionGroupsListResult, name string) (string, error) {
	for _, g := range groups {
		if g.Name == name {
			return g.Id, nil
		}
	}
	return "", fmt.Errorf("cloudflare permission group %q not found", name)
}

// accountResources is the JSON policy resource scoping a token to one account.
func accountResources(accountID string) string {
	// Marshalling a map[string]string cannot fail.
	b, _ := json.Marshal(map[string]string{"com.cloudflare.api.account." + accountID: "*"})
	return string(b)
}

// NewDeployToken mints the account API token CI uses to deploy to Pages. It has
// a single allow policy: the configured permission group (Pages Write), on this
// account only. The group is looked up by name and filtered client-side.
func NewDeployToken(ctx *pulumi.Context, cfg *config.Config) (*cf.AccountToken, error) {
	accountID := cfg.CloudflareAccountID
	res, err := cf.LookupAccountApiTokenPermissionGroupsList(ctx, &cf.LookupAccountApiTokenPermissionGroupsListArgs{
		AccountId: &accountID,
	})
	if err != nil {
		return nil, fmt.Errorf("listing cloudflare permission groups: %w", err)
	}
	groupID, err := findPermissionGroup(res.Results, cfg.PagesWritePermissionGroupName)
	if err != nil {
		return nil, err
	}

	token, err := cf.NewAccountToken(ctx, deployTokenLogicalName, &cf.AccountTokenArgs{
		AccountId: pulumi.String(accountID),
		Name:      pulumi.String(deployTokenName),
		Policies: cf.AccountTokenPolicyArray{
			cf.AccountTokenPolicyArgs{
				Effect: pulumi.String("allow"),
				PermissionGroups: cf.AccountTokenPolicyPermissionGroupArray{
					cf.AccountTokenPolicyPermissionGroupArgs{Id: pulumi.String(groupID)},
				},
				Resources: pulumi.String(accountResources(accountID)),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating deploy token: %w", err)
	}
	return token, nil
}
