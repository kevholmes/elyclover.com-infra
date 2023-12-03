package main

import (
	"github.com/pulumi/pulumi-azure-native-sdk/authorization/v2"
	"github.com/pulumi/pulumi-azuread/sdk/v5/go/azuread"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ServicePrincipalEnvelope struct {
	ServicePrincipal     *azuread.ServicePrincipal
	ServicePrincipalPass *azuread.ServicePrincipalPassword
}

type RoleAssignments struct {
	Name       string
	Definition string
	Scope      pulumi.IDOutput
}

const StorageContributor string = "/providers/Microsoft.Authorization/roleDefinitions/ba92f5b4-2d11-453d-a403-e96b0029c9fe"
const CdnContributor string = "/providers/Microsoft.Authorization/roleDefinitions/426e0c7f-0c7e-4658-b36f-ff54d6c29b45"

// create an Azure Service Principal to be used by CI/CD for deploying built code, expiring CDN content
/*
func generateCICDServicePrincipal(ctx *pulumi.Context, sa *storage.StorageAccount, ep *nativecdn.Endpoint) (nsp ServicePrincipalEnvelope, err error) {
	cicd := "cicd-actions" + "-" + ctx.Project() + "-" + ctx.Stack()

	app, err := azuread.NewApplication(ctx, cicd, &azuread.ApplicationArgs{
		DisplayName: pulumi.String(cicd),
	})
	if err != nil {
		return nsp, err
	}

	spDesc := pulumi.Sprintf("Service Principal used for CI/CD purposes within %s-%s", ctx.Project(), ctx.Stack())
	nspArgs := azuread.ServicePrincipalArgs{
		ClientId:    app.ClientId,
		UseExisting: pulumi.Bool(false),
		Description: spDesc,
	}
	nsp.ServicePrincipal, err = azuread.NewServicePrincipal(ctx, cicd+"-serviceprincipal", &nspArgs)
	if err != nil {
		return nsp, err
	}

	// authorize new SP to modify any resources required to deploy code to this project
	ra := []RoleAssignments{
		{cicd + "-storagerole", StorageContributor, sa.ID()},
		{cicd + "-cdnrole", CdnContributor, ep.ID()},
	}
	for _, v := range ra {
		_, err = authorization.NewRoleAssignment(ctx, v.Name, &authorization.RoleAssignmentArgs{
			PrincipalId:      nsp.ServicePrincipal.ID(),
			PrincipalType:    pulumi.String("ServicePrincipal"),
			RoleDefinitionId: pulumi.String(v.Definition),
			Scope:            v.Scope,
		})
		if err != nil {
			return nsp, err
		}
	}

	// generate password / client secret for Service Principal
	nsp.ServicePrincipalPass, err = azuread.NewServicePrincipalPassword(ctx, cicd+"-secret", &azuread.ServicePrincipalPasswordArgs{
		ServicePrincipalId: nsp.ServicePrincipal.ID(),
	})
	if err != nil {
		return nsp, err
	}

	return
}
*/

// create an Azure Service Principal to be used by CI/CD for deploying built code, expiring CDN content
func (pr *projectResources) generateCICDServicePrincipal1() (err error) {
	cicd := "cicd-actions" + "-" + pr.cfgKeys.projectKey + "-" + pr.cfgKeys.envKey

	app, err := azuread.NewApplication(pr.pulumiCtx, cicd, &azuread.ApplicationArgs{
		DisplayName: pulumi.String(cicd),
	})
	if err != nil {
		return err
	}

	spDesc := pulumi.Sprintf("Service Principal used for CI/CD purposes within %s-%s", pr.cfgKeys.projectKey, pr.cfgKeys.envKey)
	nspArgs := azuread.ServicePrincipalArgs{
		ClientId:    app.ClientId,
		UseExisting: pulumi.Bool(false),
		Description: spDesc,
	}
	pr.svcPrincipals.cicd.ServicePrincipal, err = azuread.NewServicePrincipal(pr.pulumiCtx, cicd+"-serviceprincipal", &nspArgs)
	if err != nil {
		return err
	}

	// authorize new SP to modify any resources required to deploy code to this project
	ra := []RoleAssignments{
		{cicd + "-storagerole", StorageContributor, pr.webStorageAccount.ID()},
		{cicd + "-cdnrole", CdnContributor, pr.webCdnProfile.ID()},
	}
	for _, v := range ra {
		_, err = authorization.NewRoleAssignment(pr.pulumiCtx, v.Name, &authorization.RoleAssignmentArgs{
			PrincipalId:      pr.svcPrincipals.cicd.ServicePrincipal.ID(),
			PrincipalType:    pulumi.String("ServicePrincipal"),
			RoleDefinitionId: pulumi.String(v.Definition),
			Scope:            v.Scope,
		})
		if err != nil {
			return err
		}
	}

	// generate password / client secret for Service Principal
	pr.svcPrincipals.cicd.ServicePrincipalPass, err = azuread.NewServicePrincipalPassword(pr.pulumiCtx, cicd+"-secret", &azuread.ServicePrincipalPasswordArgs{
		ServicePrincipalId: pr.svcPrincipals.cicd.ServicePrincipal.ID(),
	})
	if err != nil {
		return err
	}

	return
}
