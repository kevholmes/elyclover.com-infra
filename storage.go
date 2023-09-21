package main

import (
	"fmt"

	"github.com/pulumi/pulumi-azure-native-sdk/storage/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func newStorageAccount(ctx *pulumi.Context, name string, rg pulumi.StringOutput) (sa *storage.StorageAccount, err error) {
	storageAccountArgs := storage.StorageAccountArgs{
		ResourceGroupName: rg,
		Sku: &storage.SkuArgs{
			Name: pulumi.String("Standard_ZRS"),
		},
		Kind: pulumi.String("StorageV2"),
	}
	sa, err = storage.NewStorageAccount(ctx, name, &storageAccountArgs)
	if err != nil {
		fmt.Printf("ERROR: creating storageAccount %s failed\n", name)
		return sa, err
	}
	return
}

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
