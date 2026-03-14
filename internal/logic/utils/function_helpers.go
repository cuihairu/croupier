package utils

import (
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/helper"
	"github.com/cuihairu/croupier/internal/model"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// ValidateFunctionID ensures function ID is provided.
func ValidateFunctionID(id string) (string, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "", errorx.NewBadRequest("函数ID不能为空")
	}
	return trimmed, nil
}

// BuildFunctionDTO maps model.Function to API type.
func BuildFunctionDTO(fn *model.Function) Function {
	return Function{
		Id:          fn.FunctionID,
		Name:        fn.Name,
		Description: fn.Description,
		Category:    fn.Category,
		GameId:      fn.GameID,
		Status:      fn.Status,
		Version:     fn.Version,
		Instances:   fn.Instances,
		SpecFormat:  fn.SpecFormat,
		OpenAPISpec: fn.OpenAPISpec,
		CreatedAt:   helper.FormatTimestamp(fn.CreatedAt),
		UpdatedAt:   helper.FormatTimestamp(fn.UpdatedAt),
	}
}

// BuildFunctionInstances converts instances to API type.
func BuildFunctionInstances(instances []model.FunctionInstance) []FunctionInstance {
	items := make([]FunctionInstance, 0, len(instances))
	for i := range instances {
		instance := instances[i]
		items = append(items, FunctionInstance{
			AgentId:   instance.AgentID,
			AgentName: instance.AgentName,
			Status:    instance.Status,
			UpdatedAt: helper.FormatTimestamp(instance.UpdatedAt),
		})
	}
	return items
}

// BuildFunctionPermissions converts permissions to API type.
func BuildFunctionPermissions(perms []model.FunctionPermission) []FunctionPermission {
	items := make([]FunctionPermission, 0, len(perms))
	for i := range perms {
		perm := perms[i]
		items = append(items, FunctionPermission{
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
func ConvertFunctionPermissions(functionID string, perms []FunctionPermission) ([]model.FunctionPermission, error) {
	result := make([]model.FunctionPermission, 0, len(perms))
	for _, perm := range perms {
		if strings.TrimSpace(perm.Resource) == "" {
			return nil, errorx.NewBadRequest("权限资源名称不能为空")
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

// Local types for backward compatibility within utils package
// These will be removed once all code is migrated to use domain DTOs

type Function struct {
	Id          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	GameId      string      `json:"gameId"`
	Status      int         `json:"status"`
	Version     string      `json:"version"`
	Instances   int         `json:"instances"`
	SpecFormat  string      `json:"specFormat"`
	OpenAPISpec interface{} `json:"openapiSpec"`
	CreatedAt   string      `json:"createdAt"`
	UpdatedAt   string      `json:"updatedAt"`
}

type FunctionInstance struct {
	AgentId   string `json:"agentId"`
	AgentName string `json:"agentName"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updatedAt"`
}

type FunctionPermission struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
	Roles    []string `json:"roles"`
}
