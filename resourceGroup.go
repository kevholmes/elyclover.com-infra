package main

import (
	"fmt"

	"github.com/pulumi/pulumi-azure-native-sdk/resources/v2"
)

/*
func createResourceGroup(ctx *pulumi.Context, name string, args *resources.ResourceGroupArgs) (rg *resources.ResourceGroup, err error) {
	// Create an Azure Resource Group
	rg, err = resources.NewResourceGroup(ctx, name, args)
	if err != nil {
		fmt.Printf("ERROR: creating webResourceGrp failed with %s\n", name)
		return rg, err
	}
	return
}
*/

func (pr *projectResources) createResourceGroup1() (err error) {
	// Create an Azure Resource Group
	name := pr.cfgKeys.projectKey + "-" + pr.cfgKeys.envKey
	pr.webResourceGrp, err = resources.NewResourceGroup(pr.pulumiCtx, name, nil)
	if err != nil {
		fmt.Printf("ERROR: creating webResourceGrp failed with %s\n", name)
		return err
	}
	return
}
