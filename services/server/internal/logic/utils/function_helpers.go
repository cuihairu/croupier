package utils

import (
	"fmt"
	"strings"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

// ValidateFunctionID ensures function ID is provided.
func ValidateFunctionID(id string) (string, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "", fmt.Errorf("函数ID不能为空")
	}
	return trimmed, nil
}

// BuildFunctionDTO maps model.Function to API type.
func BuildFunctionDTO(fn *model.Function) types.Function {
	return types.Function{
		Id:          fn.FunctionID,
		Name:        fn.Name,
		Description: fn.Description,
		Category:    fn.Category,
		GameId:      fn.GameID,
		Status:      fn.Status,
		Version:     fn.Version,
		Instances:   fn.Instances,
		CreatedAt:   FormatTimestamp(fn.CreatedAt),
		UpdatedAt:   FormatTimestamp(fn.UpdatedAt),
	}
}

// BuildFunctionInstances converts instances to API type.
func BuildFunctionInstances(instances []model.FunctionInstance) []types.FunctionInstance {
	items := make([]types.FunctionInstance, 0, len(instances))
	for i := range instances {
		instance := instances[i]
		items = append(items, types.FunctionInstance{
			AgentId:   instance.AgentID,
			AgentName: instance.AgentName,
			Status:    instance.Status,
			UpdatedAt: FormatTimestamp(instance.UpdatedAt),
		})
	}
	return items
}

// BuildFunctionPermissions converts permissions to API type.
func BuildFunctionPermissions(perms []model.FunctionPermission) []types.FunctionPermission {
	items := make([]types.FunctionPermission, 0, len(perms))
	for i := range perms {
		perm := perms[i]
		items = append(items, types.FunctionPermission{
			Resource: perm.Resource,
			Actions:  DecodeStringSlice(perm.Actions),
			Roles:    DecodeStringSlice(perm.Roles),
		})
	}
	return items
}

func BuildInvokeRequest(functionID string, payload []byte, metadata map[string]string) *sdkv1.InvokeRequest {
	req := &sdkv1.InvokeRequest{
		FunctionId: strings.TrimSpace(functionID),
		Payload:    payload,
	}
	if metadata != nil {
		req.Metadata = metadata
	}
	return req
}

// ConvertFunctionPermissions converts API permissions to model records.
func ConvertFunctionPermissions(functionID string, perms []types.FunctionPermission) ([]model.FunctionPermission, error) {
	result := make([]model.FunctionPermission, 0, len(perms))
	for _, perm := range perms {
		if strings.TrimSpace(perm.Resource) == "" {
			return nil, fmt.Errorf("权限资源名称不能为空")
		}
		result = append(result, model.FunctionPermission{
			FunctionID: functionID,
			Resource:   strings.TrimSpace(perm.Resource),
			Actions:    EncodeStringSlice(perm.Actions),
			Roles:      EncodeStringSlice(perm.Roles),
		})
	}
	return result, nil
}
