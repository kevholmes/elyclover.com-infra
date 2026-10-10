package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"elyclover.com-infra/internal/infra"
)

func main() { pulumi.Run(infra.Program) }
