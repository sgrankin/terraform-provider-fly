package provider

import (
	"context"

	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSourceWithConfigure = &volumeDataSource{}

// Matches getSchema
type volumeDataSourceOutput struct {
	Id         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Size       types.Int64  `tfsdk:"size"`
	Appid      types.String `tfsdk:"app"`
	Region     types.String `tfsdk:"region"`
	Internalid types.String `tfsdk:"internalid"`
}

func (v volumeDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "fly_volume"
}

func (v volumeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, rep *datasource.SchemaResponse) {
	rep.Schema = schema.Schema{
		MarkdownDescription: "Fly volume resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "ID of volume",
				Computed:            true,
				Optional:            true,
			},
			"app": schema.StringAttribute{
				MarkdownDescription: "Name of app to attach",
				Required:            true,
			},
			"size": schema.Int64Attribute{
				MarkdownDescription: "Size of volume in gb",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "name",
				Required:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "region",
				Required:            true,
			},
			"internalid": schema.StringAttribute{
				MarkdownDescription: "Internal ID",
				Required:            true,
			},
		},
	}
}

func NewVolumeDataSource() datasource.DataSource {
	return volumeDataSource{}
}

func (v volumeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data volumeDataSourceOutput

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	internalId := data.Internalid.ValueString()
	app := data.Appid.ValueString()

	query, err := graphql.VolumeQuery(context.Background(), v.gqlClient, app, internalId)
	if err != nil {
		resp.Diagnostics.AddError("Read: query failed", err.Error())
	}

	data = volumeDataSourceOutput{
		Id:         types.StringValue(query.App.Volume.Id),
		Name:       types.StringValue(query.App.Volume.Name),
		Size:       types.Int64Value(int64(query.App.Volume.SizeGb)),
		Appid:      types.StringValue(data.Appid.ValueString()),
		Region:     types.StringValue(query.App.Volume.Region),
		Internalid: types.StringValue(query.App.Volume.InternalId),
	}

	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
