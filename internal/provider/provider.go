package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Khan/genqlient/graphql"
	providerGraphql "github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/fly-apps/terraform-provider-fly/internal/utils"
	"github.com/fly-apps/terraform-provider-fly/internal/wg"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	hreq "github.com/imroc/req/v3"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	tfsdkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ tfsdkprovider.Provider = &provider{}

type gqlClient graphql.Client

type provider struct {
	version string
	token   string
}

type providerClients struct {
	httpEndpoint string
	gqlClient    gqlClient
	httpClient   hreq.Client
}

func (c *providerClients) configure(providerData any, diags *diag.Diagnostics) {
	if providerData == nil {
		return
	}

	if p, ok := providerData.(*providerClients); ok {
		*c = *p
	} else {
		diags.AddError(
			"Unexpected Provider Instance Type",
			fmt.Sprintf("While creating the data source or resource, an unexpected clients type (%T) was received. This is always a bug in the clients code and should be reported to the clients developers.", p),
		)
	}
}

type providerData struct {
	FlyToken             types.String `tfsdk:"fly_api_token"`
	FlyHttpEndpoint      types.String `tfsdk:"fly_http_endpoint"`
	UseInternalTunnel    types.Bool   `tfsdk:"useinternaltunnel"`
	InternalTunnelOrg    types.String `tfsdk:"internaltunnelorg"`
	InternalTunnelRegion types.String `tfsdk:"internaltunnelregion"`
}

func (p *provider) Configure(ctx context.Context, req tfsdkprovider.ConfigureRequest, resp *tfsdkprovider.ConfigureResponse) {
	var data providerData
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	var token string
	if data.FlyToken.IsUnknown() {
		resp.Diagnostics.AddWarning(
			"Unable to create gqlClient",
			"Cannot use unknown value as token",
		)
		return
	}
	if data.FlyToken.IsNull() || data.FlyToken.IsUnknown() {
		token = os.Getenv("FLY_API_TOKEN")
	} else {
		token = data.FlyToken.ValueString()
	}
	if token == "" {
		resp.Diagnostics.AddError(
			"Unable to find token",
			"token cannot be an empty string",
		)
		return
	}

	p.token = token

	endpoint, exists := os.LookupEnv("FLY_HTTP_ENDPOINT")
	httpEndpoint := "127.0.0.1:4280"
	if !data.FlyHttpEndpoint.IsNull() && !data.FlyHttpEndpoint.IsUnknown() {
		httpEndpoint = data.FlyHttpEndpoint.ValueString()
	} else if exists {
		httpEndpoint = endpoint
	}

	var clients providerClients
	clients.httpEndpoint = httpEndpoint

	enableTracing := false
	_, ok := os.LookupEnv("DEBUG")
	if ok {
		enableTracing = true
		resp.Diagnostics.AddWarning("Debug mode enabled", "Debug mode enabled, this will add the Fly-Force-Trace header to all graphql requests")
	}

	clients.httpClient = *hreq.C()

	if enableTracing {
		clients.httpClient.SetCommonHeader("Fly-Force-Trace", "true")
		clients.httpClient = *hreq.C().DevMode()
	}

	clients.httpClient.SetCommonHeader("Authorization", "Bearer "+p.token)
	clients.httpClient.SetTimeout(2 * time.Minute)

	// TODO: Make timeout configurable
	h := http.Client{Timeout: 60 * time.Second, Transport: &utils.Transport{UnderlyingTransport: http.DefaultTransport, Token: token, Ctx: ctx, EnableDebugTrace: enableTracing}}
	client := graphql.NewClient("https://api.fly.io/graphql", &h)
	clients.gqlClient = *(*gqlClient)(&client)

	if data.UseInternalTunnel.ValueBool() {
		org, err := providerGraphql.Organization(context.Background(), client, data.InternalTunnelOrg.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Could not resolve organization", err.Error())
			return
		}
		tunnel, err := wg.Establish(ctx, org.Organization.Id, data.InternalTunnelRegion.ValueString(), token, &client)
		if err != nil {
			resp.Diagnostics.AddError("failed to open internal tunnel", err.Error())
			return
		}
		clients.httpClient.SetDial(tunnel.NetStack().DialContext)
		clients.httpEndpoint = "_api.internal:4280"
	}

	resp.ResourceData = &clients
	resp.DataSourceData = &clients
}

func (p *provider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newAppResource,
		newFlyVolumeResource,
		newFlyIpResource,
		newFlyCertResource,
		newFlyMachineResource,
	}
}

func (p *provider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newAppDataSource,
		newCertDataSource,
		newIpDataSource,
	}
}
func (p *provider) Metadata(_ context.Context, _ tfsdkprovider.MetadataRequest, rep *tfsdkprovider.MetadataResponse) {
	rep.TypeName = "fly"
	rep.Version = p.version
}

func (p *provider) Schema(ctx context.Context, _ tfsdkprovider.SchemaRequest, rep *tfsdkprovider.SchemaResponse) {
	rep.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"fly_api_token": schema.StringAttribute{
				MarkdownDescription: "fly.io api token. If not set checks env for FLY_API_TOKEN",
				Optional:            true,
			},
			"fly_http_endpoint": schema.StringAttribute{
				MarkdownDescription: "Where the clients should look to find the fly http endpoint",
				Optional:            true,
			},
			"useinternaltunnel": schema.BoolAttribute{
				Optional: true,
			},
			"internaltunnelorg": schema.StringAttribute{
				Optional: true,
			},
			"internaltunnelregion": schema.StringAttribute{
				Optional: true,
			},
		},
	}
}

func New(version string) func() tfsdkprovider.Provider {
	return func() tfsdkprovider.Provider {
		return &provider{
			version: version,
		}
	}
}
