package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/fly-apps/terraform-provider-fly/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

var (
	_ resource.ResourceWithConfigure   = &flyAppResource{}
	_ resource.ResourceWithImportState = &flyAppResource{}
)

type flyAppResourceData struct {
	Name   types.String `tfsdk:"name"`
	Org    types.String `tfsdk:"org"`
	OrgId  types.String `tfsdk:"orgid"`
	AppUrl types.String `tfsdk:"appurl"`
	Id     types.String `tfsdk:"id"`
}

func (d *flyAppResourceData) updateFromApi(a graphql.AppFragment) {
	d.Name = types.StringValue(a.Name)
	d.Org = types.StringValue(a.Organization.Slug)
	d.OrgId = types.StringValue(a.Organization.Id)
	d.AppUrl = types.StringValue(a.AppUrl)
	d.Id = types.StringValue(a.Id)
}

func newAppResource() resource.Resource {
	return &flyAppResource{}
}

type flyAppResource struct {
	flyResource
}

func (r flyAppResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_app"
}

func (r flyAppResource) Schema(ctx context.Context, req resource.SchemaRequest, rep *resource.SchemaResponse) {
	rep.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Fly app resource",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of application",
				Required:            true,
			},
			"org": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Optional org slug to operate upon",
			},
			"orgid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "readonly orgid",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "readonly app id",
			},
			"appurl": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "readonly appUrl",
			},
		},
	}
}

func (r flyAppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data flyAppResourceData

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Org.IsUnknown() {
		defaultOrg, err := utils.GetDefaultOrg(r.gqlClient)
		if err != nil {
			resp.Diagnostics.AddError("Could not detect default organization", err.Error())
			return
		}
		data.OrgId = types.StringValue(defaultOrg.Id)
		data.Org = types.StringValue(defaultOrg.Name)
	} else {
		org, err := graphql.Organization(context.Background(), r.gqlClient, data.Org.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Could not resolve organization", err.Error())
			return
		}
		data.OrgId = types.StringValue(org.Organization.Id)
	}
	mresp, err := graphql.CreateAppMutation(context.Background(), r.gqlClient, data.Name.ValueString(), data.OrgId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Create app failed", err.Error())
		return
	}
	data.updateFromApi(mresp.CreateApp.App)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r flyAppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state flyAppResourceData

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	query, err := graphql.GetApp(context.Background(), r.gqlClient, state.Name.ValueString())
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

	state.updateFromApi(query.App)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r flyAppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan flyAppResourceData

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	var state flyAppResourceData
	diags = resp.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	tflog.Info(ctx, fmt.Sprintf("existing: %+v, new: %+v", state, plan))

	if !plan.Org.IsUnknown() && plan.Org.ValueString() != state.Org.ValueString() {
		resp.Diagnostics.AddError("Can't mutate org of existing app", "Can't switch org"+state.Org.ValueString()+" to "+plan.Org.ValueString())
	}
	if !plan.Name.IsNull() && plan.Name.ValueString() != state.Name.ValueString() {
		resp.Diagnostics.AddError("Can't mutate Name of existing app", "Can't switch name "+state.Name.ValueString()+" to "+plan.Name.ValueString())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r flyAppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data flyAppResourceData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	_, err := graphql.DeleteAppMutation(context.Background(), r.gqlClient, data.Name.ValueString())
	var errList gqlerror.List
	if errors.As(err, &errList) {
		for _, err := range errList {
			resp.Diagnostics.AddError(err.Message, err.Path.String())
		}
	} else if err != nil {
		resp.Diagnostics.AddError("Delete app failed", err.Error())
	}

	resp.State.RemoveResource(ctx)

	if resp.Diagnostics.HasError() {
		return
	}
}

func (r flyAppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}
