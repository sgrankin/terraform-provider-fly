package provider

import (
	"context"
	"errors"
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
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccApp_secrets(t *testing.T) {
	ctx := context.Background()
	name := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)

	h := http.Client{Timeout: 60 * time.Second, Transport: &utils.Transport{UnderlyingTransport: http.DefaultTransport, Token: os.Getenv("FLY_API_TOKEN"), Ctx: ctx}}
	client := graphql.NewClient("https://api.fly.io/graphql", &h)

	var lastDigest, newDigest string

	testDigestEqualInApi := func(digest string) error {
		r, err := providerGraphql.GetSecrets(ctx, client, name)
		if err != nil {
			t.Fatal(err)
		}
		apiDigest := r.App.Secrets[0].Digest
		if digest != apiDigest {
			return fmt.Errorf("Digest %s in resource differs from digest %s from API", digest, apiDigest)
		}
		return nil
	}

	testDigestChanged := func(digest string) error {
		if digest == lastDigest {
			return fmt.Errorf("digest %s did not change even though we changed the secret's value", digest)
		}
		lastDigest = digest
		return nil
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					%s
					resource "fly_app" "the_app" {
						name = "%s" 
						org = "%s"
					}
					resource "fly_app_secret" "the_secret" {
						app_id = resource.fly_app.the_app.name
						name = "TEST" 
						value = "42"
					}`,
					providerConfig(), name, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_app_secret.the_secret", "value", "42"),
					resource.TestCheckResourceAttrSet("fly_app_secret.the_secret", "digest"),
					resource.TestCheckResourceAttrSet("fly_app_secret.the_secret", "created_at"),
					resource.TestCheckResourceAttrWith("fly_app_secret.the_secret", "digest", testDigestEqualInApi),
					resource.TestCheckResourceAttrWith("fly_app_secret.the_secret", "digest", func(digest string) error {
						lastDigest = digest
						return nil
					}),
				),
			},
			{
				Config: fmt.Sprintf(`
					%s
					resource "fly_app" "the_app" {
						name = "%s" 
						org = "%s"
					}
					resource "fly_app_secret" "the_secret" {
						app_id = resource.fly_app.the_app.name
						name = "TEST" 
						value = "43"
					}`,
					providerConfig(), name, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_app_secret.the_secret", "value", "43"),
					resource.TestCheckResourceAttrSet("fly_app_secret.the_secret", "digest"),
					resource.TestCheckResourceAttrSet("fly_app_secret.the_secret", "created_at"),
					resource.TestCheckResourceAttrWith("fly_app_secret.the_secret", "digest", testDigestChanged),
					resource.TestCheckResourceAttrWith("fly_app_secret.the_secret", "digest", testDigestEqualInApi),
					resource.TestCheckResourceAttrWith("fly_app_secret.the_secret", "digest", func(digest string) error {
						lastDigest = digest
						return nil
					}),
				),
			},

			// Secret drift detection: We change the secret's value using direct API calls outside
			// and verify that the resource is able to detect this and restore the original state
			{
				PreConfig: func() {
					r, err := providerGraphql.GetSecrets(ctx, client, name)
					if err != nil {
						t.Fatal(err)
					}
					oldSecret := r.App.Secrets[0]
					sr, err := providerGraphql.SetSecret(ctx, client, name, "TEST", "44")
					if err != nil {
						t.Fatal(err)
					}
					r, err = providerGraphql.GetSecrets(ctx, client, name)
					if err != nil {
						t.Fatal(err)
					}
					newSecret := r.App.Secrets[0]
					if sr.SetSecrets.App.Secrets[0].Digest != newSecret.Digest {
						t.Fatal("fly API SetSecrets returned different digest than subsequent GetSecrets")
					}
					if newSecret.Digest == oldSecret.Digest {
						t.Fatal("fly API SetSecret had no effect")
					}
					newDigest = newSecret.Digest
				},
				Config: fmt.Sprintf(`
					%s
					resource "fly_app" "the_app" {
						name = "%s" 
						org = "%s"
					}
					resource "fly_app_secret" "the_secret" {
						app_id = resource.fly_app.the_app.name
						name = "TEST" 
						value = "43"
					}`,
					providerConfig(), name, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("fly_app_secret.the_secret", "value", "43"),
					resource.TestCheckResourceAttrSet("fly_app_secret.the_secret", "digest"),
					resource.TestCheckResourceAttrSet("fly_app_secret.the_secret", "created_at"),
					resource.TestCheckResourceAttrWith("fly_app_secret.the_secret", "digest", testDigestEqualInApi),
					func(state *terraform.State) error {
						r, err := providerGraphql.GetSecrets(ctx, client, name)
						if err != nil {
							return err
						}
						if len(r.App.Secrets) != 1 {
							return fmt.Errorf("Unexpected number of secrets %d", len(r.App.Secrets))
						}
						if r.App.Secrets[0].Name != "TEST" {
							return fmt.Errorf("Unexpected secret name %v", r.App.Secrets[0].Name)
						}
						return nil
					},
					resource.TestCheckResourceAttrWith("fly_app_secret.the_secret", "digest", func(value string) error {
						r, err := providerGraphql.GetSecrets(ctx, client, name)
						if err != nil {
							return err
						}
						if r.App.Secrets[0].Digest != value {
							return fmt.Errorf("digest in state (%s) differs from digest from fly API (%s)", value, r.App.Secrets[0].Digest)
						}
						if value != lastDigest {
							return fmt.Errorf("digest in state (%s) differs from digest of same value before drift (%s) -- newDigest %s", value, lastDigest, newDigest)
						}
						return nil
					}),
				),
			},

			// Verify that we don't touch unmanaged secrets
			{
				PreConfig: func() {
					_, err := providerGraphql.SetSecret(ctx, client, name, "unmanaged", "1")
					if err != nil {
						t.Fatal(err)
					}
				},
				Config: fmt.Sprintf(`
					%s
					resource "fly_app" "the_app" {
						name = "%s" 
						org = "%s"
					}
					resource "fly_app_secret" "the_secret" {
						app_id = resource.fly_app.the_app.name
						name = "TEST" 
						value = "43"
					}`,
					providerConfig(), name, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("fly_app_secret.the_secret", "digest"),
					func(state *terraform.State) error {
						r, err := providerGraphql.GetSecrets(ctx, client, name)
						if err != nil {
							return err
						}

						if len(r.App.Secrets) == 1 && r.App.Secrets[0].Name == "TEST" {
							return errors.New("unmanaged secret disappeared")
						} else if len(r.App.Secrets) != 2 {
							return fmt.Errorf("unexpected secrets in API %v", r.App.Secrets)
						}
						return nil
					},
				),
			},

			{
				Config: fmt.Sprintf(`
					%s
					resource "fly_app" "the_app" {
						name = "%s" 
						org = "%s"
					}`,
					providerConfig(), name, getTestOrg()),
				Check: resource.ComposeAggregateTestCheckFunc(
					func(state *terraform.State) error {
						r, err := providerGraphql.GetSecrets(ctx, client, name)
						if err != nil {
							return err
						}

						if len(r.App.Secrets) == 1 && r.App.Secrets[0].Name == "TEST" {
							return errors.New("Secret still there!")
						} else if len(r.App.Secrets) != 1 {
							return fmt.Errorf("unexpected secrets in API %v", r.App.Secrets)
						}
						return nil
					},
				),
			},
		},
	})
}
