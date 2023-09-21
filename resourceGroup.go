package main

import (
	"fmt"

	"github.com/pulumi/pulumi-azure-native-sdk/resources/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func createResourceGroup(ctx *pulumi.Context, name string, args *resources.ResourceGroupArgs) (rg *resources.ResourceGroup, err error) {
	// Create an Azure Resource Group
	rg, err = resources.NewResourceGroup(ctx, name, args)
	if err != nil {
		fmt.Printf("ERROR: creating webResourceGrp failed with %s\n", name)
		return rg, err
	}
	return
}
