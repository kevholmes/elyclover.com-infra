package main

import (
	"fmt"

	"github.com/pulumi/pulumi-github/sdk/v5/go/github"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

/*
func exportDeployEnvDataToGitHubRepo(ctx *pulumi.Context, cfg *config.Config, sp ServicePrincipalEnvelope, rg *resources.ResourceGroup, sa *storage.StorageAccount, cdnprof *cdn.Profile, ep *cdn.Endpoint) (err error) {
	// Validate repo
	githubConfig := config.New(ctx, "github")
	repoPath := fmt.Sprintf("%s/%s", githubConfig.Require("owner"), cfg.Require("ghAppSrcRepo"))
	repo, err := github.LookupRepository(ctx, &github.LookupRepositoryArgs{
		FullName: &repoPath,
	})
	if err != nil {
		return err
	}

	// Create deployment environment using stack name (env)
	env := ctx.Stack()
	repoEnv, err := github.NewRepositoryEnvironment(ctx, env, &github.RepositoryEnvironmentArgs{
		Repository:  pulumi.String(repo.Name),
		Environment: pulumi.String(env),
	})
	if err != nil {
		return err
	}

	// Create Actions Deployment Environment Secret for Azure SP that will be deploying via Actions workflows
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
		// This permits PRs submitted by Dependabot (e.g. when bumping project dependencies to newer versions)
		// to have access to the dev stack environment's Azure SP client secret token.
		// This allows the Dependabot PR to be treated just like a user-submitted (chore-like) PR to bump the dependency
		// The outcome here is that the Dependabot submitted PR gets built and deployed just like any other.
		if ctx.Stack() == "dev" {
			_, err = github.NewDependabotSecret(ctx, k, &github.DependabotSecretArgs{
				Repository:     pulumi.String(repo.Name),
				SecretName:     pulumi.String(k),
				PlaintextValue: v,
			})
		}
	}
	// Create Actions Deployment Environment Variables to be used in Actions CI/CD workflows
	actionsVars := map[string]pulumi.StringInput{
		"AZ_CDN_ENDPOINT":     ep.Name,
		"AZ_CDN_PROFILE_NAME": cdnprof.Name,
		"AZ_RESOURCE_GROUP":   rg.Name,
		"AZ_STORAGE_ACCT":     sa.Name,
		"CLIENT_ID":           sp.ServicePrincipal.ClientId,
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
*/

func (pr *projectResources) exportDeployEnvDataToGitHubRepo1() (err error) {
	// Validate repo
	githubConfig := config.New(pr.pulumiCtx, "github")
	repoPath := fmt.Sprintf("%s/%s", githubConfig.Require("owner"), pr.cfgKeys.ghAppSrcRepo)
	repo, err := github.LookupRepository(pr.pulumiCtx, &github.LookupRepositoryArgs{
		FullName: &repoPath,
	})
	if err != nil {
		return err
	}

	// Create deployment environment using stack name (env)
	env := pr.cfgKeys.envKey
	repoEnv, err := github.NewRepositoryEnvironment(pr.pulumiCtx, env, &github.RepositoryEnvironmentArgs{
		Repository:  pulumi.String(repo.Name),
		Environment: pulumi.String(env),
	})
	if err != nil {
		return err
	}

	// Create Actions Deployment Environment Secret for Azure SP that will be deploying via Actions workflows
	// https://github.com/Azure/login#configure-a-service-principal-with-a-secret
	secretsVars := map[string]pulumi.StringInput{
		"CLIENT_SECRET": pr.svcPrincipals.cicd.ServicePrincipalPass.Value,
	}
	for k, v := range secretsVars {
		_, err = github.NewActionsEnvironmentSecret(pr.pulumiCtx, k, &github.ActionsEnvironmentSecretArgs{
			Repository:     pulumi.String(repo.Name),
			SecretName:     pulumi.String(k),
			Environment:    repoEnv.Environment,
			PlaintextValue: v,
		})
		if err != nil {
			return err
		}
		// This permits PRs submitted by Dependabot (e.g. when bumping project dependencies to newer versions)
		// to have access to the dev stack environment's Azure SP client secret token.
		// This allows the Dependabot PR to be treated just like a user-submitted (chore-like) PR to bump the dependency
		// The outcome here is that the Dependabot submitted PR gets built and deployed just like any other.
		if pr.cfgKeys.envKey == DEV {
			_, err = github.NewDependabotSecret(pr.pulumiCtx, k, &github.DependabotSecretArgs{
				Repository:     pulumi.String(repo.Name),
				SecretName:     pulumi.String(k),
				PlaintextValue: v,
			})
		}
	}
	// Create Actions Deployment Environment Variables to be used in Actions CI/CD workflows
	actionsVars := map[string]pulumi.StringInput{
		"AZ_CDN_ENDPOINT":     pr.webCdnEp.Name,
		"AZ_CDN_PROFILE_NAME": pr.webCdnProfile.Name,
		"AZ_RESOURCE_GROUP":   pr.webResourceGrp.Name,
		"AZ_STORAGE_ACCT":     pr.webStorageAccount.Name,
		"CLIENT_ID":           pr.svcPrincipals.cicd.ServicePrincipal.ClientId,
		//"SUBSCRIPTION_ID":   pulumi.String(cfg.Require("AzSubScriptionIdWeb")),
		"SUBSCRIPTION_ID": pulumi.String(pr.thisAzureSubscription.Id),
		"TENANT_ID":       pulumi.String(pr.cfgKeys.thisAzureTenantId),
	}
	for k, v := range actionsVars {
		_, err = github.NewActionsEnvironmentVariable(pr.pulumiCtx, k, &github.ActionsEnvironmentVariableArgs{
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
