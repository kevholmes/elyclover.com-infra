package main

import (
	"fmt"

	"github.com/pulumi/pulumi-azure-native-sdk/cdn/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/storage/v2"
	"github.com/pulumi/pulumi-github/sdk/v5/go/github"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func exportDeployEnvDataToGitHubRepo(ctx *pulumi.Context, cfg *config.Config, sp ServicePrincipalEnvelope, rg *resources.ResourceGroup, sa *storage.StorageAccount, cdnprof *cdn.Profile, ep *cdn.Endpoint) (err error) {
	// validate repo
	githubConfig := config.New(ctx, "github")
	repoPath := fmt.Sprintf("%s/%s", githubConfig.Require("owner"), cfg.Require("ghAppSrcRepo"))
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

	// create Actions Deployment Environment Secret for Azure SP that will be deploying via Actions workflows
	// https://github.com/Azure/login#configure-a-service-principal-with-a-secret
	secretsVars := map[string]pulumi.StringInput{
		"CLIENT_SECRET": sp.ServicePrincipalPass.Value,
	}
	for k, v := range secretsVars {
		_, err = github.NewActionsEnvironmentSecret(ctx, k, &github.ActionsEnvironmentSecretArgs{
			Repository:     pulumi.String(repo.Name),
			SecretName:     pulumi.String(k),
			Environment:    repoEnv.Environment,
			PlaintextValue: v,
		})
		if err != nil {
			return err
		}
	}
	// create Actions Deployment Environment Variables to be used in Actions CI/CD workflows
	actionsVars := map[string]pulumi.StringInput{
		"AZ_CDN_ENDPOINT":     ep.Name,
		"AZ_CDN_PROFILE_NAME": cdnprof.Name,
		"AZ_RESOURCE_GROUP":   rg.Name,
		"AZ_STORAGE_ACCT":     sa.Name,
		"CLIENT_ID":           sp.ServicePrincipal.ApplicationId,
		"SUBSCRIPTION_ID":     pulumi.String(cfg.Require("AzSubScriptionIdWeb")),
		"TENANT_ID":           pulumi.String(cfg.Require("AzTenantId")),
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
