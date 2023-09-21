package main

import (
	"encoding/json"

	"github.com/pulumi/pulumi-azure-native-sdk/cdn/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/storage/v2"
	"github.com/pulumi/pulumi-github/sdk/v5/go/github"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

/*
Example JSON object that Azure Login Action expects to find in secrets.AZURE_CREDENTIALS

	{
	  "clientId": "b5b2faf0-****-****-****-abbc84fa2e54",
	  "clientSecret": "cFv8Q~Q**********************~ggid7avq",
	  "subscriptionId": "df058eb7-****-****-****-937439336e86",
	  "tenantId": "6088cb1f-****-****-****-4946a45e93d1",
	  "activeDirectoryEndpointUrl": "https://login.microsoftonline.com",
	  "resourceManagerEndpointUrl": "https://management.azure.com/",
	  "activeDirectoryGraphResourceId": "https://graph.windows.net/",
	  "sqlManagementEndpointUrl": "https://management.core.windows.net:8443/",
	  "galleryEndpointUrl": "https://gallery.azure.com/",
	  "managementEndpointUrl": "https://management.core.windows.net/"
	}
*/
type AzureCredentials struct {
	ClientID             pulumi.StringInput `json:"clientId"`
	ClientSecret         pulumi.StringInput `json:"clientSecret"`
	SubscriptionId       pulumi.StringInput `json:"subscriptionId"`
	TenantId             pulumi.StringInput `json:"tenantId"`
	ActiveDirectoryEpUrl pulumi.StringInput `json:"activeDirectoryEndpointUrl"`
	ResourceMgrEpUrl     pulumi.StringInput `json:"resourceManagerEndpointUrl"`
	AdGraphResourceId    pulumi.StringInput `json:"activeDirectoryGraphResourceId"`
	SqlMgmtEpUrl         pulumi.StringInput `json:"sqlManagementEndpointUrl"`
	GalleryEpUrl         pulumi.StringInput `json:"galleryEndpointUrl"`
	MgmtEpUrl            pulumi.StringInput `json:"managementEndpointUrl"`
}

func exportDeployEnvDataToGitHubRepo(ctx *pulumi.Context, cfg *config.Config, sp *ServicePrincipalEnvelope, rg *resources.ResourceGroup, sa *storage.StorageAccount, cdnprof *cdn.Profile, ep *cdn.Endpoint) (err error) {
	// validate repo
	repoPath := cfg.Require("ghAppSrcProjectPath")
	repo, err := github.LookupRepository(ctx, &github.LookupRepositoryArgs{
		FullName: &repoPath,
	})
	if err != nil {
		return err
	}

	// create deployment environment using stack name (env)
	env := ctx.Stack()
	repoEnv, err := github.NewRepositoryEnvironment(ctx, env, &github.RepositoryEnvironmentArgs{
		Repository:  pulumi.String(repo.Name),
		Environment: pulumi.String(env),
	})
	if err != nil {
		return err
	}

	// populate struct with credential data and then marshall into json obj to store in Actions Secrets
	// https://github.com/Azure/login#configure-a-service-principal-with-a-secret
	azCreds := &AzureCredentials{
		ClientID:             sp.ServicePrincipal.ID(),
		ClientSecret:         sp.ServicePrincipalPass.Value,
		SubscriptionId:       pulumi.String(cfg.Require("AzSubScriptionIdWeb")),
		TenantId:             pulumi.String(cfg.Require("AzTenantId")),
		ActiveDirectoryEpUrl: pulumi.String("https://login.microsoftonline.com"),
		ResourceMgrEpUrl:     pulumi.String("https://management.azure.com/"),
		AdGraphResourceId:    pulumi.String("https://graph.windows.net/"),
		SqlMgmtEpUrl:         pulumi.String("https://management.core.windows.net:8443/"),
		GalleryEpUrl:         pulumi.String("https://gallery.azure.com/"),
		MgmtEpUrl:            pulumi.String("https://management.core.windows.net/"),
	}
	azCredsStr, err := json.Marshal(azCreds)
	if err != nil {
		return err
	}

	// create Actions Deployment Environment Secret for Azure SP that will be deploying via Actions workflows
	_, err = github.NewActionsEnvironmentSecret(ctx, "AZURE_CREDENTIALS", &github.ActionsEnvironmentSecretArgs{
		Repository:     pulumi.String(repo.Name),
		SecretName:     pulumi.String("AZURE_CREDENTIALS"),
		Environment:    repoEnv.Environment,
		PlaintextValue: pulumi.Sprintf("%s", azCredsStr),
	})
	if err != nil {
		return err
	}
	// create Actions Deployment Environment Variables to be used in Actions CI/CD workflows
	actionsVars := map[string]pulumi.StringInput{
		"AZ_CDN_ENDPOINT":     ep.Name,
		"AZ_CDN_PROFILE_NAME": cdnprof.Name,
		"AZ_RESOURCE_GROUP":   rg.Name,
		"AZ_STORAGE_ACCT":     sa.Name,
	}
	for k, v := range actionsVars {
		_, err = github.NewActionsEnvironmentVariable(ctx, k, &github.ActionsEnvironmentVariableArgs{
			Environment:  repoEnv.Environment,
			Repository:   pulumi.String(repo.Name),
			VariableName: pulumi.String(k),
			Value:        v,
		})
		if err != nil {
			return err
		}
	}

	return
}
