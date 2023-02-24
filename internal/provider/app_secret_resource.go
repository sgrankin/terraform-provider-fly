package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fly-apps/terraform-provider-fly/graphql"
	"github.com/fly-apps/terraform-provider-fly/internal/provider/modifiers"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type appSecretResourceData struct {
	AppID     types.String `tfsdk:"app_id"`
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Value     types.String `tfsdk:"value"`
	Digest    types.String `tfsdk:"digest"`
	CreatedAt types.String `tfsdk:"created_at"`
}

type appSecretResource struct {
	flyResource
}

var _ resource.ResourceWithConfigure = (*appSecretResource)(nil)

func newAppSecretResource() resource.Resource {
	return &appSecretResource{}
}

func (r *appSecretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_secret"
}

func (r *appSecretResource) Schema(ctx context.Context, req resource.SchemaRequest, rep *resource.SchemaResponse) {
	secretValueUnchanged := func(ctx context.Context, req planmodifier.StringRequest) (bool, diag.Diagnostics) {
		valuePath := req.Path.ParentPath().AtName("value")
		var stateValue, configValue types.String
		var diags diag.Diagnostics
		diags.Append(req.State.GetAttribute(ctx, valuePath, &stateValue)...)
		diags.Append(req.Config.GetAttribute(ctx, valuePath, &configValue)...)
		return stateValue.Equal(configValue), diags
	}

	rep.Schema = schema.Schema{
		MarkdownDescription: "Fly app resource",

		Attributes: map[string]schema.Attribute{
			"app_id": schema.StringAttribute{
				Required:    true,
				Description: "App ID",
			},
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					modifiers.UseStateForUnknownIf(secretValueUnchanged),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"value": schema.StringAttribute{
				Sensitive: true,
				Required:  true,
			},
			"digest": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					modifiers.UseStateForUnknownIf(secretValueUnchanged),
				},
			},
			"created_at": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					modifiers.UseStateForUnknownIf(secretValueUnchanged),
				},
			},
		},
	}
}

// Create applies the plan and returns the new state.
func (r *appSecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data appSecretResourceData
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.setSecret(ctx, &data, resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

const _ = `# @genqlient
	mutation SetSecret($appID: ID!, $key: String!, $value: String!) {
	    setSecrets(input: {appId: $appID, secrets: [{key: $key, value: $value}]}) {
	        app {
	            secrets {
	            	id
	                name
	                digest
	                createdAt
	            }
	        }
	    }
	}
`

// setSecrets calls the `SetSecrets` mutation and then sets the returned secret properties in `data“.
func (r *appSecretResource) setSecret(ctx context.Context, data *appSecretResourceData, diags diag.Diagnostics) {
	resp, err := graphql.SetSecret(ctx, r.gqlClient, data.AppID.ValueString(), data.Name.ValueString(), data.Value.ValueString())
	if err != nil {
		diags.AddError("SetSecrets failed", err.Error())
		return
	}

	for _, sec := range resp.SetSecrets.App.Secrets {
		if sec.Name == data.Name.ValueString() {
			data.ID = types.StringValue(sec.Id)
			data.CreatedAt = types.StringValue(sec.CreatedAt.Format(time.RFC3339))
			data.Digest = types.StringValue(sec.Digest)
			return
		}
	}
	diags.AddError("Secret was not found after setting it", fmt.Sprintf("Secret named %q was not found in response: %q", data.Name.ValueString(), resp))
}

// Read refreshes the state.
func (r *appSecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data appSecretResourceData
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.refreshSecret(ctx, &data)
	var errList gqlerror.List
	if errors.As(err, &errList) && len(errList) == 1 && errList[1].Extensions["code"] == "NOT_FOUND" {
		// (App) resource is missing; remove the secrets as they no longer exist.
		resp.State.RemoveResource(ctx)
		return
	} else if err != nil {
		resp.Diagnostics.AddError("Refreshing failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

const _ = `# @genqlient
	query GetSecrets($name: String!) {
		app(name: $name) {
			secrets {
				id
				name
				digest
				createdAt
			}
		}
	}
`

func (r *appSecretResource) refreshSecret(ctx context.Context, data *appSecretResourceData) error {
	rep, err := graphql.GetSecrets(ctx, r.gqlClient, data.AppID.ValueString())
	if err != nil {
		return err
	}
	for _, sec := range rep.App.Secrets {
		if sec.Name == data.Name.ValueString() {
			id := types.StringValue(sec.Id)
			createdAt := types.StringValue(sec.CreatedAt.Format(time.RFC3339))
			digest := types.StringValue(sec.Digest)

			if data.ID != id || data.CreatedAt != createdAt || data.Digest != digest {
				data.Value = types.StringUnknown()
			}

			data.ID = id
			data.CreatedAt = createdAt
			data.Digest = digest
		}
	}
	return nil
}

// Update applies the plan for an existing resource.
func (r *appSecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data appSecretResourceData
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.setSecret(ctx, &data, resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

const _ = `# @genqlient
	mutation UnsetSecret($appId: ID!, $key: String!) {
		unsetSecrets(input: {appId: $appId, keys: [$key]}) {
			release {
				id
			}
		}
	}
`

func (r *appSecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data appSecretResourceData
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := graphql.UnsetSecret(ctx, r.gqlClient, data.AppID.ValueString(), data.Name.ValueString())

	var errList gqlerror.List
	if errors.As(err, &errList) {
		for _, err := range errList {
			resp.Diagnostics.AddError(err.Message, err.Path.String())
		}
	} else if err != nil {
		resp.Diagnostics.AddError("Delete app failed", err.Error())
	}
	resp.State.RemoveResource(ctx)
}
