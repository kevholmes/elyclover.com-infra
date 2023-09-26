package main

import (
	"os"
	"strings"

	"github.com/pulumi/pulumi-azure-native-sdk/storage/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	//"golang.org/x/crypto/pkcs12"
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
