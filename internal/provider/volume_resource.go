package provider

import (
	"context"
	"fmt"

	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.ResourceWithConfigure   = &flyVolumeResource{}
	_ resource.ResourceWithImportState = &flyVolumeResource{}
)

type flyVolumeResource struct {
	flyResource
}

func newFlyVolumeResource() resource.Resource {
	return &flyVolumeResource{}
}

type flyVolumeResourceData struct {
	Id         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Size       types.Int64  `tfsdk:"size"`
	Appid      types.String `tfsdk:"app"`
	Region     types.String `tfsdk:"region"`
	Internalid types.String `tfsdk:"internalid"`
}

func (vr flyVolumeResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_volume"
}

func (vr flyVolumeResource) Schema(_ context.Context, _ resource.SchemaRequest, rep *resource.SchemaResponse) {
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
				Computed:            true,
				Optional:            true,
			},
		},
	}
}

func (vr flyVolumeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data flyVolumeResourceData

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	q, err := graphql.CreateVolume(context.Background(), vr.gqlClient, data.Appid.ValueString(), data.Name.ValueString(), data.Region.ValueString(), int(data.Size.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create volume", err.Error())
	}

	data = flyVolumeResourceData{
		Id:         types.StringValue(q.CreateVolume.Volume.Id),
		Name:       types.StringValue(q.CreateVolume.Volume.Name),
		Size:       types.Int64Value(int64(q.CreateVolume.Volume.SizeGb)),
		Appid:      types.StringValue(data.Appid.ValueString()),
		Region:     types.StringValue(q.CreateVolume.Volume.Region),
		Internalid: types.StringValue(q.CreateVolume.Volume.InternalId),
	}

	tflog.Info(ctx, fmt.Sprintf("%+v", data))

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (vr flyVolumeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data flyVolumeResourceData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	internalId := data.Internalid.ValueString()
	app := data.Appid.ValueString()

	query, err := graphql.VolumeQuery(context.Background(), vr.gqlClient, app, internalId)
	if err != nil {
		resp.Diagnostics.AddError("Read: query failed", err.Error())
	}

	data = flyVolumeResourceData{
		Id:         types.StringValue(query.App.Volume.Id),
		Name:       types.StringValue(query.App.Volume.Name),
		Size:       types.Int64Value(int64(query.App.Volume.SizeGb)),
		Appid:      types.StringValue(data.Appid.ValueString()),
		Region:     types.StringValue(query.App.Volume.Region),
		Internalid: types.StringValue(query.App.Volume.InternalId),
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (vr flyVolumeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("The fly api does not allow updating volumes once created", "Try deleting and then recreating a volume with new options")
	return
}

func (vr flyVolumeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data flyVolumeResourceData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if !data.Id.IsUnknown() && !data.Id.IsNull() && data.Id.ValueString() != "" {
		_, err := graphql.DeleteVolume(context.Background(), vr.gqlClient, data.Id.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Delete volume failed", err.Error())
		}
	}

	resp.State.RemoveResource(ctx)

	if resp.Diagnostics.HasError() {
		return
	}
}

func (vr flyVolumeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
