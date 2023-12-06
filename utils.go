package main

import (
	"os"
	"strings"

	"github.com/pulumi/pulumi-azure-native-sdk/storage/v2"
	"github.com/pulumi/pulumi-azure/sdk/v5/go/azure/core"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

func stripWebStorageEndPointUrl(sa *storage.StorageAccount) (url pulumi.StringOutput) {
	webEndptStr := sa.PrimaryEndpoints.Web()
	url = webEndptStr.ApplyT(func(url string) string {
		return strings.ReplaceAll(strings.ReplaceAll(url, "https://", ""), "/", "")
	}).(pulumi.StringOutput)
	return
}

func validatePfx(pfxData []byte, pass string) (err error) {
	_, _, _, err = pkcs12.DecodeChain(pfxData, pass)
	if err != nil {
		return err
	}
	return
}

func loadFileFromDisk(path string) (data []byte, err error) {
	data, err = os.ReadFile(path)
	if err != nil {
		return data, err
	}
	return
}

// consider iterating over pr.cfgKeys with a k:v map to load these in if list continues to grow
func (pr *projectResources) initConfigKeys() (err error) {
	pr.cfgKeys.projectKey = pr.pulumiCtx.Project()
	pr.cfgKeys.envKey = pr.pulumiCtx.Stack()
	pr.cfgKeys.siteKey = pr.cfg.Require("siteKey")
	pr.cfgKeys.dnsResourceGrp = pr.cfg.Require("dnsResourceGroup")
	pr.cfgKeys.dnsLookupZone = pr.cfg.Require("dnsZoneName")
	pr.cfgKeys.ghAppSrcRepo = pr.cfg.Require("ghAppSrcRepo")
	pr.cfgKeys.thisAzureTenantId = pr.cfg.Require("AzTenantId")
	pr.cfgKeys.dnsRecordTTL = pr.cfg.Require("dnsRecordTTL")
	if pr.cfgKeys.envKey == PROD {
		pr.cfgKeys.cdnAzureId = pr.cfg.Require("Microsoft.AzureFrontDoor-Cdn")
		pr.cfgKeys.kvAzureSubscription = pr.cfg.Require("keyVaultAzureSubscription")
		pr.cfgKeys.kvAzureResourceGrp = pr.cfg.Require("keyvaultResourceGroup")
		pr.cfgKeys.kvAzureName = pr.cfg.Require("keyVaultName")
	}
	pr.thisAzureSubscription, err = core.LookupSubscription(pr.pulumiCtx, nil)

	return
}
