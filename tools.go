//go:build tools

package tools

import (
	_ "github.com/Khan/genqlient"
	_ "github.com/hashicorp/terraform"
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)
