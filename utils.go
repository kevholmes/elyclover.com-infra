package main

import (
	"strings"

	"github.com/pulumi/pulumi-azure-native-sdk/storage/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func stripWebStorageEndPointUrl(sa *storage.StorageAccount) (url pulumi.StringOutput) {
	webEndptStr := sa.PrimaryEndpoints.Web()
	url = webEndptStr.ApplyT(func(url string) string {
		return strings.ReplaceAll(strings.ReplaceAll(url, "https://", ""), "/", "")
	}).(pulumi.StringOutput)
	return
}
