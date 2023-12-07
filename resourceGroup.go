package main

import (
	"fmt"

	"github.com/pulumi/pulumi-azure-native-sdk/resources/v2"
)

func (pr *projectResources) createResourceGroup() (err error) {
	// Create an Azure Resource Group
	name := pr.cfgKeys.projectKey + "-" + pr.cfgKeys.envKey
	pr.webResourceGrp, err = resources.NewResourceGroup(pr.pulumiCtx, name, nil)
	if err != nil {
		fmt.Printf("ERROR: creating webResourceGrp failed with %s\n", name)
		return err
	}
	return
}
