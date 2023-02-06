package modifiers

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

type UseStateForUnknownIfFunc func(ctx context.Context, req planmodifier.StringRequest) (bool, diag.Diagnostics)

// UseStateForUnknownIf is like UseStateForUnknown, but conditional
func UseStateForUnknownIf(condition UseStateForUnknownIfFunc) planmodifier.String {
	return useStateForUnknownIfModifier{condition}
}

// useStateForUnknownIfModifier implements the UseStateForUnknownIf
// AttributePlanModifier.
type useStateForUnknownIfModifier struct {
	condition UseStateForUnknownIfFunc
}

// Modify copies the attribute's prior state to the attribute plan if the prior
// state value is not null.
func (r useStateForUnknownIfModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// if we have no state value, there's nothing to preserve
	if req.StateValue.IsNull() {
		return
	}

	// if it's not planned to be the unknown value, stick with the concrete plan
	if !resp.PlanValue.IsUnknown() {
		return
	}

	// if the config is the unknown value, use the unknown value otherwise, interpolation gets messed up
	if req.ConfigValue.IsUnknown() {
		return
	}

	ok, diags := r.condition(ctx, req)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !ok {
		return
	}

	resp.PlanValue = req.StateValue
}

func (r useStateForUnknownIfModifier) Description(context.Context) string {
	return "Once set, the value of this attribute in state will not change as long as the given condition holds."
}

func (r useStateForUnknownIfModifier) MarkdownDescription(ctx context.Context) string {
	return r.Description(ctx)
}
