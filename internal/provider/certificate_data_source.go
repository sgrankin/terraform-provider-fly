package provider

import (
	"context"
	"errors"

	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

var _ datasource.DataSourceWithConfigure = &certDataSource{}

// Matches getSchema
type certDataSourceOutput struct {
	Id                        types.String `tfsdk:"id"`
	Appid                     types.String `tfsdk:"app"`
	Dnsvalidationinstructions types.String `tfsdk:"dnsvalidationinstructions"`
	Dnsvalidationhostname     types.String `tfsdk:"dnsvalidationhostname"`
	Dnsvalidationtarget       types.String `tfsdk:"dnsvalidationtarget"`
	Hostname                  types.String `tfsdk:"hostname"`
	Check                     types.Bool   `tfsdk:"check"`
}

func (d certDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "fly_cert"
}

func (d certDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, rep *datasource.SchemaResponse) {
	rep.Schema = schema.Schema{
		MarkdownDescription: "Fly certificate data source",
		Attributes: map[string]schema.Attribute{
			"app": schema.StringAttribute{
				MarkdownDescription: "Name of app that is attacjed",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "ID of address",
				Computed:            true,
			},
			"dnsvalidationinstructions": schema.StringAttribute{
				MarkdownDescription: "DnsValidationHostname",
				Computed:            true,
			},
			"dnsvalidationtarget": schema.StringAttribute{
				MarkdownDescription: "DnsValidationTarget",
				Computed:            true,
			},
			"dnsvalidationhostname": schema.StringAttribute{
				MarkdownDescription: "DnsValidationHostname",
				Computed:            true,
			},
			"check": schema.BoolAttribute{
				MarkdownDescription: "check",
				Computed:            true,
			},
			"hostname": schema.StringAttribute{
				MarkdownDescription: "hostname",
				Required:            true,
			},
		},
	}
}

func newCertDataSource() datasource.DataSource {
	return &certDataSource{}
}

func (d certDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data certDataSourceOutput

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	hostname := data.Hostname.ValueString()
	app := data.Appid.ValueString()

	query, err := graphql.GetCertificate(context.Background(), d.gqlClient, app, hostname)
	var errList gqlerror.List
	if errors.As(err, &errList) {
		for _, err := range errList {
			if err.Message == "Could not resolve " {
				return
			}
			resp.Diagnostics.AddError(err.Message, err.Path.String())
		}
	} else if err != nil {
		resp.Diagnostics.AddError("Read: query failed", err.Error())
	}

	data = certDataSourceOutput{
		Id:                        types.StringValue(query.App.Certificate.Id),
		Appid:                     types.StringValue(data.Appid.ValueString()),
		Dnsvalidationinstructions: types.StringValue(query.App.Certificate.DnsValidationInstructions),
		Dnsvalidationhostname:     types.StringValue(query.App.Certificate.DnsValidationHostname),
		Dnsvalidationtarget:       types.StringValue(query.App.Certificate.DnsValidationTarget),
		Hostname:                  types.StringValue(query.App.Certificate.Hostname),
		Check:                     types.BoolValue(query.App.Certificate.Check),
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
