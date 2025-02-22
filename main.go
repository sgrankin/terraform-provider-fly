package main

import (
	"context"
	"flag"
	"log"

	"github.com/fly-apps/terraform-provider-fly/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// Generate graphql client.
//go:generate go run github.com/Khan/genqlient

// Format the terraform examples.
//go:generate go run github.com/hashicorp/terraform fmt -recursive ./examples/

// Run the docs generation tool. Check its repository for more information on how it works and how docs can be customized.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

var (
	// these will be set by the goreleaser configuration
	// to appropriate values for the compiled binary
	version string = "dev"

	// goreleaser can also pass the specific commit if you want
	// commit  string = ""
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/fly-apps/fly",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
