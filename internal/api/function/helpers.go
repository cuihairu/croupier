package function

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/internal/common/errorx"
	logicfunction "github.com/cuihairu/croupier/internal/logic/function"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

// Function management implementations

func functionsList(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionsListRequest) (*FunctionsListResponse, error) {
	logicResp, err := logicfunction.NewFunctionsListLogic(ctx, svcCtx).FunctionsList(&logicfunction.FunctionsListRequest{
		Page:     1,
		PageSize: 10000,
		GameId:   req.GameId,
		Category: req.Category,
		Status:   req.Status,
	})
	if err != nil {
		return nil, err
	}

	dbItems, _, err := svcCtx.FunctionModel.List(ctx, model.ListFunctionsOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     1,
			PageSize: 10000,
		},
		GameID: strings.TrimSpace(req.GameId),
	})
	if err != nil {
		return nil, err
	}
	dbIndex := make(map[string]model.Function, len(dbItems))
	for _, item := range dbItems {
		dbIndex[item.FunctionID] = item
	}

	items := make([]Function, 0, len(logicResp.Items))
	for _, fn := range logicResp.Items {
		dbFn, ok := dbIndex[fn.ID]
		category := fn.Category
		version := fn.Version
		specFormat := fn.SpecFormat
		openAPISpec := fn.OpenAPISpec
		description := fn.Description
		createdAt := ""
		updatedAt := ""
		if ok {
			if category == "" {
				category = getStringFromMetadata(dbFn.Metadata, "category")
			}
			if version == "" {
				version = getStringFromMetadata(dbFn.Metadata, "version")
			}
			if specFormat == "" {
				specFormat = getStringFromMetadata(dbFn.Metadata, "spec_format")
			}
			if openAPISpec == nil {
				openAPISpec = getInterfaceFromMetadata(dbFn.Metadata, "openapi_spec")
			}
			if description == "" {
				description = dbFn.Description
			}
			createdAt = utils.FormatTimestamp(dbFn.CreatedAt)
			updatedAt = utils.FormatTimestamp(dbFn.UpdatedAt)
		}
		items = append(items, Function{
			Id:          fn.ID,
			Name:        fn.Name,
			Description: description,
			Category:    category,
			GameId:      fn.GameId,
			Status:      fn.Status,
			Version:     version,
			Instances:   fn.Instances,
			SpecFormat:  specFormat,
			OpenAPISpec: openAPISpec,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		})
	}

	return &FunctionsListResponse{
		Items: items,
		Total: logicResp.Total,
		Page:  logicResp.Page,
		Size:  logicResp.Size,
	}, nil
}

func functionsPending(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionsPendingRequest) (*FunctionsPendingResponse, error) {
	// Implementation would query pending functions
	return &FunctionsPendingResponse{
		Items: []PendingFunction{},
	}, nil
}

func functionDetail(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionDetailRequest) (*FunctionDetailResponse, error) {
	logicResp, err := logicfunction.NewFunctionDetailLogic(ctx, svcCtx).FunctionDetail(&logicfunction.FunctionDetailRequest{
		ID: req.ID,
	})
	if err != nil {
		return nil, err
	}

	return &FunctionDetailResponse{
		Function: Function{
			Id:          logicResp.Function.ID,
			Name:        logicResp.Function.Name,
			Description: logicResp.Function.Description,
			Category:    logicResp.Function.Category,
			GameId:      logicResp.Function.GameId,
			Status:      logicResp.Function.Status,
			Version:     logicResp.Function.Version,
			Instances:   logicResp.Function.Instances,
			SpecFormat:  logicResp.Function.SpecFormat,
			OpenAPISpec: logicResp.Function.OpenAPISpec,
		},
		Descriptor: FunctionDescriptor{
			Input:  logicResp.Descriptor.Input,
			Output: logicResp.Descriptor.Output,
			Schema: logicResp.Descriptor.Schema,
		},
	}, nil
}

func functionAnalytics(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionAnalyticsRequest) (*FunctionAnalyticsResponse, error) {
	// Implementation would query analytics data
	return &FunctionAnalyticsResponse{
		TotalCalls:     0,
		SuccessRate:    0,
		AvgLatency:     0,
		CallsToday:     0,
		CallsThisWeek:  0,
		CallsThisMonth: 0,
	}, nil
}

func functionCopy(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionCopyRequest) (*FunctionCopyResponse, error) {
	// Implementation would copy function to target env
	return &FunctionCopyResponse{
		FunctionId: req.ID,
		NewId:      "",
	}, nil
}

func functionDelete(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionDeleteRequest) error {
	return svcCtx.FunctionModel.DeleteFunction(ctx, req.FunctionId)
}

func functionDisable(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionDisableRequest) error {
	status := 0
	return svcCtx.FunctionModel.Update(ctx, 0, map[string]interface{}{"status": status})
}

func functionEnable(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionEnableRequest) error {
	status := 1
	return svcCtx.FunctionModel.Update(ctx, 0, map[string]interface{}{"status": status})
}

func functionHistory(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionHistoryRequest) (*FunctionHistoryResponse, error) {
	// Implementation would query function history
	return &FunctionHistoryResponse{
		Items: []FunctionHistoryItem{},
	}, nil
}

func functionInvoke(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInvokeRequest) (*FunctionInvokeResponse, error) {
	_, roles, err := utils.LoadCurrentAdmin(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	_, err = utils.PermissionIDsFromRoles(ctx, svcCtx, roles)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, err
	}

	metadata := map[string]string{
		"async": "false",
	}
	if req.Mode == "async" {
		metadata["async"] = "true"
	}

	if req.Mode == "async" {
		jobResp, err := svcCtx.Dispatcher.StartJobRequest(ctx, utils.BuildInvokeRequest(req.ID, payload, metadata))
		if err != nil {
			return nil, err
		}
		return &FunctionInvokeResponse{
			JobId:  jobResp.GetJobId(),
			JobID:  jobResp.GetJobId(),
			Result: nil,
		}, nil
	}

	resp, err := svcCtx.Dispatcher.InvokeRequest(ctx, utils.BuildInvokeRequest(req.ID, payload, metadata))
	if err != nil {
		return &FunctionInvokeResponse{
			JobId:  "",
			Result: nil,
		}, err
	}

	out := &FunctionInvokeResponse{}
	if resp != nil && len(resp.GetPayload()) > 0 {
		var v map[string]interface{}
		if err := json.Unmarshal(resp.GetPayload(), &v); err == nil {
			out.Result = v
		}
	}
	return out, nil
}

func functionPublish(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionPublishRequest) (*FunctionPublishResponse, error) {
	// Implementation would publish function to approval queue
	return &FunctionPublishResponse{
		ApprovalId: "",
		Published:  true,
	}, nil
}

func functionRoute(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionRouteRequest) (*FunctionRouteResponse, error) {
	logicResp, err := logicfunction.NewFunctionRouteLogic(ctx, svcCtx).FunctionRoute(&logicfunction.FunctionRouteRequest{
		ID: req.ID,
	})
	if err != nil {
		return nil, err
	}

	return &FunctionRouteResponse{
		Menu: FunctionRouteConfig{
			Nodes:  logicResp.Menu.(logicfunction.FunctionRouteConfig).Nodes,
			Path:   logicResp.Menu.(logicfunction.FunctionRouteConfig).Path,
			Order:  logicResp.Menu.(logicfunction.FunctionRouteConfig).Order,
			Hidden: logicResp.Menu.(logicfunction.FunctionRouteConfig).Hidden,
		},
		Source: logicResp.Source,
	}, nil
}

func functionRouteUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionRouteUpdateRequest) (*FunctionRouteResponse, error) {
	logicResp, err := logicfunction.NewFunctionRouteUpdateLogic(ctx, svcCtx).FunctionRouteUpdate(&logicfunction.FunctionRouteUpdateRequest{
		ID:     req.ID,
		Nodes:  req.Nodes,
		Path:   req.Path,
		Order:  req.Order,
		Hidden: req.Hidden,
	})
	if err != nil {
		return nil, err
	}

	return &FunctionRouteResponse{
		Menu: FunctionRouteConfig{
			Nodes:  logicResp.Menu.(logicfunction.FunctionRouteConfig).Nodes,
			Path:   logicResp.Menu.(logicfunction.FunctionRouteConfig).Path,
			Order:  logicResp.Menu.(logicfunction.FunctionRouteConfig).Order,
			Hidden: logicResp.Menu.(logicfunction.FunctionRouteConfig).Hidden,
		},
		Source: logicResp.Source,
	}, nil
}

// Instance management implementations

func functionInstances(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInstancesRequest) (*FunctionInstancesResponse, error) {
	store := svcCtx.RegistryStore
	if store == nil {
		return &FunctionInstancesResponse{Items: []FunctionInstance{}}, nil
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	instances := []FunctionInstance{}
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		if _, ok := sess.Functions[req.ID]; ok {
			instances = append(instances, FunctionInstance{
				AgentId:   sess.AgentID,
				AgentName: sess.AgentID, // Use AgentID as name since AgentName doesn't exist
				Status:    "active",
				UpdatedAt: utils.FormatTimestamp(sess.LastSeen),
			})
		}
	}

	return &FunctionInstancesResponse{
		Items: instances,
	}, nil
}

func functionInstancesAll(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInstancesAllRequest) (*FunctionInstancesAllResponse, error) {
	store := svcCtx.RegistryStore
	if store == nil {
		return &FunctionInstancesAllResponse{Instances: []map[string]interface{}{}}, nil
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	instances := []map[string]interface{}{}
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		for fid := range sess.Functions {
			instances = append(instances, map[string]interface{}{
				"functionId": fid,
				"agentId":    sess.AgentID,
				"agentName":  sess.AgentID, // Use AgentID as name since AgentName doesn't exist
				"status":     "active",
				"updatedAt":  utils.FormatTimestamp(sess.LastSeen),
			})
		}
	}

	return &FunctionInstancesAllResponse{
		Instances: instances,
	}, nil
}

// Permission management implementations

func functionPermissions(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionPermissionsRequest) (*FunctionPermissionsResponse, error) {
	perms, err := svcCtx.FunctionModel.ListPermissions(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	items := make([]FunctionPermission, 0, len(perms))
	for _, p := range perms {
		var roles []string
		if len(p.Roles) > 0 {
			roles = parseRolesFromJSON(p.Roles)
		}
		var actions []string
		if p.Actions != nil {
			actions = parseActionsFromJSON(p.Actions)
		}
		items = append(items, FunctionPermission{
			Resource: p.Resource,
			Actions:  actions,
			Roles:    roles,
		})
	}

	return &FunctionPermissionsResponse{
		Items: items,
	}, nil
}

func functionPermissionsUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionPermissionsUpdateRequest) error {
	// Convert API permissions to model permissions
	var modelPerms []model.FunctionPermission
	for _, p := range req.Permissions {
		rolesJSON, _ := json.Marshal(p.Roles)
		actionsJSON, _ := json.Marshal(p.Actions)
		modelPerms = append(modelPerms, model.FunctionPermission{
			FunctionID: req.ID,
			Resource:   p.Resource,
			Roles:      rolesJSON,
			Actions:    actionsJSON,
		})
	}
	return svcCtx.FunctionModel.ReplacePermissions(ctx, req.ID, modelPerms)
}

// UI configuration implementations

func functionUI(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionUIRequest) (*FunctionUIResponse, error) {
	logicResp, err := logicfunction.NewFunctionUILogicV2(ctx, svcCtx).FunctionUI(&logicfunction.FunctionUIRequest{
		ID: req.ID,
	})
	if err != nil {
		return nil, err
	}
	return &FunctionUIResponse{
		Schema:         logicResp.Schema,
		Layout:         logicResp.Layout,
		Components:     logicResp.Components,
		Custom:         boolFromAny(logicResp.Custom),
		HasDefault:     logicResp.HasDefault,
		UISource:       logicResp.UISource,
		UISourceDetail: stringFromAny(logicResp.UISourceDetail),
	}, nil
}

func functionUIUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionUIUpdateRequest) error {
	_, err := logicfunction.NewFunctionUIUpdateLogic(ctx, svcCtx).FunctionUIUpdate(&logicfunction.FunctionUIUpdateRequest{
		ID:         req.ID,
		Schema:     req.Schema,
		Layout:     req.Layout,
		Components: req.Components,
	})
	return err
}

func functionUIHistory(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionUIHistoryRequest) (*FunctionUIHistoryResponse, error) {
	// GetUIHistory not implemented - return empty
	return &FunctionUIHistoryResponse{
		Items: []FunctionUIHistoryItem{},
	}, nil
}

func functionUIRollback(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionUIRollbackRequest) error {
	// RollbackUI not implemented
	return nil
}

func functionWarnings(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionWarningsRequest) (*FunctionWarningsResponse, error) {
	return &FunctionWarningsResponse{
		Items: []FunctionWarningItem{},
	}, nil
}

// Descriptors implementations

func descriptors(ctx context.Context, svcCtx *svc.ServiceContext, req *DescriptorsRequest) (*DescriptorsResponse, error) {
	descs, err := svcCtx.FunctionModel.ListDescriptors(ctx, req.GameId)
	if err != nil {
		return nil, err
	}

	items := make([]Descriptor, 0, len(descs))
	for _, d := range descs {
		inputMap := map[string]interface{}(d.Input)
		outputMap := map[string]interface{}(d.Output)
		items = append(items, Descriptor{
			Id:          d.FunctionID,
			Name:        "", // FunctionDescriptor doesn't have Name
			Description: "", // FunctionDescriptor doesn't have Description
			Input:       inputMap,
			Output:      outputMap,
		})
	}

	return &DescriptorsResponse{
		Items: items,
	}, nil
}

// Batch operations implementations

func batchCopyFunctions(ctx context.Context, svcCtx *svc.ServiceContext, req *BatchCopyFunctionsRequest) (*BatchCopyFunctionsResponse, error) {
	results := make([]FunctionCopyResponse, 0, len(req.Functions))
	for _, f := range req.Functions {
		result, _ := functionCopy(ctx, svcCtx, &f)
		if result != nil {
			results = append(results, *result)
		}
	}
	return &BatchCopyFunctionsResponse{
		Results: results,
	}, nil
}

func batchDeleteFunctions(ctx context.Context, svcCtx *svc.ServiceContext, req *BatchDeleteFunctionsRequest) (*BatchDeleteFunctionsResponse, error) {
	deleted := []string{}
	failed := []string{}

	for _, id := range req.FunctionIds {
		if err := functionDelete(ctx, svcCtx, &FunctionDeleteRequest{FunctionId: id}); err != nil {
			failed = append(failed, id)
		} else {
			deleted = append(deleted, id)
		}
	}

	return &BatchDeleteFunctionsResponse{
		Deleted: deleted,
		Failed:  failed,
	}, nil
}

func batchUpdateFunctions(ctx context.Context, svcCtx *svc.ServiceContext, req *BatchUpdateFunctionsRequest) (*BatchUpdateFunctionsResponse, error) {
	results := make([]FunctionRouteResponse, 0, len(req.Updates))
	for _, u := range req.Updates {
		result, _ := functionRouteUpdate(ctx, svcCtx, &u)
		if result != nil {
			results = append(results, *result)
		}
	}
	return &BatchUpdateFunctionsResponse{
		Results: results,
	}, nil
}

// Helper functions

func enforceInvokePermission(svcCtx *svc.ServiceContext, roleNames []string, permIDs []string, functionID string, gameID string, env string) error {
	if utils.HasAdminRole(roleNames) {
		return nil
	}

	if svcCtx.FunctionModel == nil {
		return errorx.NewForbidden("无权调用该函数（函数权限模型未初始化）")
	}
	perms, err := svcCtx.FunctionModel.ListPermissions(nil, functionID)
	if err != nil {
		return err
	}
	if allowed, hasRule := utils.FunctionActionAllowed(roleNames, perms, "invoke", gameID, env); hasRule {
		if allowed {
			return nil
		}
		return errorx.NewForbidden("无权调用该函数")
	}

	if utils.HasPermissionID(permIDs, "*") || utils.HasPermissionID(permIDs, "function:invoke") {
		return nil
	}
	return errorx.NewForbidden("无权调用该函数（需要 function:invoke 或配置函数权限）")
}

// getStringFromMetadata gets a string value from metadata map
func getStringFromMetadata(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	if val, ok := metadata[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getIntFromMetadata gets an int value from metadata map
func getIntFromMetadata(metadata map[string]interface{}, key string) int {
	if metadata == nil {
		return 0
	}
	if val, ok := metadata[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			// Try to parse string as int
			var i int
			if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
				return i
			}
		}
	}
	return 0
}

// parseRolesFromJSON parses roles from JSON array or comma-separated string
func parseRolesFromJSON(data datatypes.JSON) []string {
	if len(data) == 0 {
		return []string{}
	}
	// Try to parse as JSON array
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr
	}
	// Try to parse as comma-separated string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str != "" {
			return strings.Split(str, ",")
		}
	}
	return []string{}
}

// parseActionsFromJSON parses actions from JSON array
func parseActionsFromJSON(data datatypes.JSON) []string {
	if len(data) == 0 {
		return []string{}
	}
	// Try to parse as JSON array
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr
	}
	// Try to parse as comma-separated string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str != "" {
			return strings.Split(str, ",")
		}
	}
	return []string{}
}

// getInterfaceFromMetadata gets an interface{} value from metadata map
func getInterfaceFromMetadata(metadata map[string]interface{}, key string) interface{} {
	if metadata == nil {
		return nil
	}
	if val, ok := metadata[key]; ok {
		return val
	}
	return nil
}

// getStringSliceFromMetadata gets a string slice value from metadata map
func getStringSliceFromMetadata(metadata map[string]interface{}, key string) []string {
	if metadata == nil {
		return []string{}
	}
	if val, ok := metadata[key]; ok {
		switch v := val.(type) {
		case []string:
			return v
		case []interface{}:
			result := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return []string{}
}

// getBoolFromMetadata gets a bool value from metadata map
func getBoolFromMetadata(metadata map[string]interface{}, key string) bool {
	if metadata == nil {
		return false
	}
	if val, ok := metadata[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return v == "true" || v == "1"
		case int:
			return v != 0
		case float64:
			return v != 0
		}
	}
	return false
}

func boolFromAny(value interface{}) bool {
	if v, ok := value.(bool); ok {
		return v
	}
	return false
}

func stringFromAny(value interface{}) string {
	if v, ok := value.(string); ok {
		return v
	}
	return ""
}
