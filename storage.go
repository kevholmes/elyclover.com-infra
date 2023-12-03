package main

import (
	"fmt"

	"github.com/pulumi/pulumi-azure-native-sdk/storage/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func (pr *projectResources) newStorageAccount1() (err error) {
	storageAccountArgs := storage.StorageAccountArgs{
		ResourceGroupName: pr.webResourceGrp.Name,
		Sku: &storage.SkuArgs{
			Name: pulumi.String("Standard_ZRS"),
		},
		Kind: pulumi.String("StorageV2"),
	}
	name := pr.cfgKeys.siteKey + pr.cfgKeys.envKey
	pr.webStorageAccount, err = storage.NewStorageAccount(pr.pulumiCtx, name, &storageAccountArgs)
	if err != nil {
		fmt.Printf("ERROR: creating storageAccount %s failed\n", name)
		return err
	}
	return
}

/*
func enableStaticWebHostOnStorageAccount(ctx *pulumi.Context, saName pulumi.StringOutput, rg pulumi.StringOutput, siteKey string) (err error) {
	// Enable static website support for the Storage Account
	storageArgs := storage.StorageAccountStaticWebsiteArgs{
		AccountName:       saName,
		ResourceGroupName: rg,
		IndexDocument:     pulumi.String("index.html"),
		Error404Document:  pulumi.String("404.hml"),
	}
	_, err = storage.NewStorageAccountStaticWebsite(ctx, siteKey, &storageArgs)
	if err != nil {
		fmt.Printf("ERROR: creating staticWebsite %s failed\n", siteKey)
		return err
	}
	return
}
*/

func (pr *projectResources) enableStaticWebHostOnStorageAccount1() (err error) {
	// Enable static website support for the Storage Account
	storageArgs := storage.StorageAccountStaticWebsiteArgs{
		AccountName:       pr.webStorageAccount.Name,
		ResourceGroupName: pr.webResourceGrp.Name,
		IndexDocument:     pulumi.String("index.html"),
		Error404Document:  pulumi.String("404.hml"),
	}
	_, err = storage.NewStorageAccountStaticWebsite(pr.pulumiCtx, pr.cfgKeys.siteKey, &storageArgs)
	if err != nil {
		fmt.Printf("ERROR: creating staticWebsite %s failed\n", pr.cfgKeys.siteKey)
		return err
	}
	return
}
