package main

import (
	"github.com/pulumi/pulumi-azure-native-sdk/authorization/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/storage/v2"
	"github.com/pulumi/pulumi-azuread/sdk/v5/go/azuread"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ServicePrincipalEnvelope struct {
	ServicePrincipal     *azuread.ServicePrincipal
	ServicePrincipalPass *azuread.ServicePrincipalPassword
}

// create an Azure Service Principal to be used by CI/CD for deploying built code, expiring CDN content
func generateCICDServicePrincipal(ctx *pulumi.Context, sa *storage.StorageAccount) (nsp ServicePrincipalEnvelope, err error) {
	cicd := "cicd-actions"

	azGuid, err := random.NewRandomUuid(ctx, "CI/CD Service Principal GUID", nil)
	if err != nil {
		return nsp, err
	}

	spDesc := pulumi.Sprintf("Service Principal used for CI/CD purposes within %s-%s", ctx.Project(), ctx.Stack())
	nspArgs := azuread.ServicePrincipalArgs{
		ApplicationId: azGuid.Result,
		UseExisting:   pulumi.Bool(false),
		Description:   spDesc,
	}
	nsp.ServicePrincipal, err = azuread.NewServicePrincipal(ctx, cicd+"-serviceprincipal", &nspArgs)
	if err != nil {
		return nsp, err
	}

	// authorize new SP to modify the resources required to deploy code to this project
	roleAssignments := map[string]string{
		cicd + "-storagerole": "/providers/Microsoft.Authorization/roleDefinitions/ba92f5b4-2d11-453d-a403-e96b0029c9fe",
		cicd + "-cdnrole":     "/providers/Microsoft.Authorization/roleDefinitions/426e0c7f-0c7e-4658-b36f-ff54d6c29b45",
	}
	for k, v := range roleAssignments {
		_, err = authorization.NewRoleAssignment(ctx, k, &authorization.RoleAssignmentArgs{
			PrincipalId:      nsp.ServicePrincipal.ID(),
			PrincipalType:    pulumi.String("ServicePrincipal"),
			RoleDefinitionId: pulumi.String(v),
			Scope:            sa.ID(),
		})
	}
	if err != nil {
		return nsp, err
	}

	// generate password / client secret for Service Principal
	nsp.ServicePrincipalPass, err = azuread.NewServicePrincipalPassword(ctx, cicd+"-secret", &azuread.ServicePrincipalPasswordArgs{
		ServicePrincipalId: nsp.ServicePrincipal.ApplicationId,
	})
	if err != nil {
		return nsp, err
	}

	return
}
