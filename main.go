package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/litebitus/terraform-provider-dashboard/internal/dashboardprovider"
)

var version = "dev"

func main() {
	err := providerserver.Serve(context.Background(), dashboardprovider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/litebitus/dashboard",
	})
	if err != nil {
		log.Fatal(err)
	}
}
