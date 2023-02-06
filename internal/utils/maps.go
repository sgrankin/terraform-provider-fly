package utils

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func KVToTfMap(kv map[string]string, elemType attr.Type) types.Map {
	elements := map[string]attr.Value{}
	for key, value := range kv {
		elements[key] = types.StringValue(value)
	}
	return types.MapValueMust(elemType, elements)
}
