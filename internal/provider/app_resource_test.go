package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	providerGraphql "github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/fly-apps/terraform-provider-fly/internal/utils"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccApp_basic(t *testing.T) {
	appName := "testApp"
	resourceName := fmt.Sprintf("fly_app.%s", appName)
	name := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)

	ctx := context.Background()
	h := http.Client{Timeout: 60 * time.Second, Transport: &utils.Transport{UnderlyingTransport: http.DefaultTransport, Token: os.Getenv("FLY_API_TOKEN"), Ctx: context.Background()}}
	client := graphql.NewClient("https://api.fly.io/graphql", &h)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
%s
resource "fly_app" "%s" {
	name = "%s"
	org = "%s"
}
`, providerConfig(), appName, name, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", name),
					resource.TestCheckResourceAttr(resourceName, "org", getTestOrg()),
					resource.TestCheckResourceAttrSet(resourceName, "orgid"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrWith(resourceName, "id", func(id string) error {
						app, err := providerGraphql.GetApp(ctx, client, id)
						if err != nil {
							t.Fatalf("Error in GetApp for %s: %v", id, err)
						}
						if app == nil {
							t.Fatalf("GetApp for %s returned nil", id)
						}
						return nil
					}),
				),
			},
		},
	})
}
