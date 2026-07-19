package function

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	logicfunction "github.com/cuihairu/croupier/internal/logic/function"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/policy"
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
		PaginationOptions: model.NewPagination(1, 10000),
		GameID:            strings.TrimSpace(req.GameId),
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
			CreatedAt:   logicResp.Function.CreatedAt,
			UpdatedAt:   logicResp.Function.UpdatedAt,
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
	admin, roles, err := utils.LoadCurrentAdmin(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	_, err = utils.PermissionIDsFromRoles(ctx, svcCtx, roles)
	if err != nil {
		return nil, err
	}

	var functionPolicy *policy.Policy
	// Apply function policy checks
	if svcCtx.PolicyManager != nil {
		roleNames := utils.RoleNamesFromModels(roles)
		functionPolicy, err = enforceFunctionPolicy(ctx, svcCtx, req.ID, roleNames)
		if err != nil {
			return nil, err
		}
	}

	payload, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, err
	}

	// Check if approval is required
	if functionPolicy != nil && functionPolicy.RequireApproval && svcCtx.ApprovalsStore != nil {
		// Create approval request instead of executing directly
		approvalID, err := createFunctionApproval(ctx, svcCtx, req.ID, payload, req.Mode, admin, functionPolicy)
		if err != nil {
			return nil, fmt.Errorf("failed to create approval request: %w", err)
		}

		// Log approval creation if audit is enabled
		if svcCtx.AuditService != nil && functionPolicy.RequireAudit {
			auditApprovalCreated(ctx, svcCtx, req.ID, admin, utils.RoleNamesFromModels(roles), approvalID, functionPolicy)
		}

		// Return approval response
		return &FunctionInvokeResponse{
			TaskId:           "",
			ApprovalID:       approvalID,
			ApprovalRequired: true,
			ApprovalWorkflow: functionPolicy.ApprovalWorkflow,
			Result:           nil,
		}, nil
	}

	metadata := map[string]string{
		"async": "false",
	}
	if req.Mode == "async" {
		metadata["async"] = "true"
	}

	// 记录操作者
	if admin != nil {
		metadata["actor"] = admin.Username
	}

	var result *FunctionInvokeResponse
	var invokeErr error

	if req.Mode == "async" {
		taskResp, err := svcCtx.Dispatcher.StartTaskRequest(ctx, utils.BuildInvokeRequest(req.ID, payload, metadata))
		if err != nil {
			invokeErr = err
		} else {
			result = &FunctionInvokeResponse{
				TaskId: taskResp.GetTaskId(),
				TaskID: taskResp.GetTaskId(),
				Result: nil,
			}
		}
	} else if strings.EqualFold(strings.TrimSpace(req.Route), "broadcast") {
		broadcast, err := svcCtx.Dispatcher.InvokeBroadcast(ctx, utils.BuildInvokeRequest(req.ID, payload, metadata))
		if err != nil {
			invokeErr = err
		} else {
			result = buildBroadcastResponse(broadcast)
		}
	} else {
		resp, err := svcCtx.Dispatcher.InvokeRequest(ctx, utils.BuildInvokeRequest(req.ID, payload, metadata))
		if err != nil {
			invokeErr = err
		} else {
			result = &FunctionInvokeResponse{}
			if resp != nil && len(resp.GetPayload()) > 0 {
				var v map[string]interface{}
				if err := json.Unmarshal(resp.GetPayload(), &v); err == nil {
					result.Result = v
				}
			}
		}
	}

	// Audit logging: log function invocation if policy requires audit
	if svcCtx.AuditService != nil && functionPolicy != nil && functionPolicy.RequireAudit {
		auditFunctionInvoke(ctx, svcCtx, req.ID, admin, utils.RoleNamesFromModels(roles), functionPolicy, invokeErr)
	}

	if invokeErr != nil {
		return &FunctionInvokeResponse{
			TaskId: "",
			Result: nil,
		}, invokeErr
	}

	return result, nil
}

// auditFunctionInvoke logs function invocation to audit service
func auditFunctionInvoke(ctx context.Context, svcCtx *svc.ServiceContext, functionID string, admin interface{}, userRoles []string, functionPolicy *policy.Policy, invokeErr error) {
	username := ""
	if admin != nil {
		if u, ok := admin.(interface{ GetUsername() string }); ok {
			username = u.GetUsername()
		}
	}

	outcome := "success"
	errorMsg := ""
	if invokeErr != nil {
		outcome = "failure"
		errorMsg = invokeErr.Error()
	}

	// Build audit details
	details := map[string]interface{}{
		"function_id":   functionID,
		"risk_level":    functionPolicy.DefaultRiskLevel,
		"require_audit": functionPolicy.RequireAudit,
		"is_override":   functionPolicy.IsOverride,
		"policy_source": functionPolicy.Source,
		"user_roles":    userRoles,
		"allowed_roles": functionPolicy.AllowedRoles,
	}

	// Log the audit event
	_, err := svcCtx.AuditService.Log(ctx, audit.EventFunctionInvoke,
		audit.WithActorID(username, "user", username),
		audit.WithResourceID("function", functionID),
		audit.WithDetails(details),
		audit.WithOutcome(outcome, errorMsg),
		audit.WithIPAddress("", ""), // Could extract from request context if needed
	)
	if err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to log audit event: %v\n", err)
	}
}

// createFunctionApproval creates an approval request for function invocation
func createFunctionApproval(ctx context.Context, svcCtx *svc.ServiceContext, functionID string, payload []byte, mode string, admin interface{}, functionPolicy *policy.Policy) (string, error) {
	username := "system"
	if admin != nil {
		if u, ok := admin.(interface{ GetUsername() string }); ok {
			username = u.GetUsername()
		}
	}

	// Generate approval ID
	approvalID := fmt.Sprintf("func_%s_%d", functionID, time.Now().UnixNano())

	// Create approval record
	approval := &approvals.Approval{
		ID:         approvalID,
		State:      "pending",
		FunctionID: functionID,
		Actor:      username,
		Mode:       mode,
		Payload:    payload,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Store approval
	_, err := svcCtx.ApprovalsStore.Create(approval)
	if err != nil {
		return "", fmt.Errorf("failed to create approval: %w", err)
	}

	return approvalID, nil
}

// auditApprovalCreated logs approval creation to audit service
func auditApprovalCreated(ctx context.Context, svcCtx *svc.ServiceContext, functionID string, admin interface{}, userRoles []string, approvalID string, functionPolicy *policy.Policy) {
	username := ""
	if admin != nil {
		if u, ok := admin.(interface{ GetUsername() string }); ok {
			username = u.GetUsername()
		}
	}

	// Build audit details
	details := map[string]interface{}{
		"function_id":       functionID,
		"approval_id":       approvalID,
		"approval_workflow": functionPolicy.ApprovalWorkflow,
		"risk_level":        functionPolicy.DefaultRiskLevel,
		"user_roles":        userRoles,
		"allowed_roles":     functionPolicy.AllowedRoles,
	}

	// Log the audit event
	_, err := svcCtx.AuditService.Log(ctx, audit.EventApprovalCreated,
		audit.WithActorID(username, "user", username),
		audit.WithResourceID("approval", approvalID),
		audit.WithDetails(details),
		audit.WithOutcome("success", ""),
	)
	if err != nil {
		fmt.Printf("Failed to log approval audit event: %v\n", err)
	}
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

func functionUIUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionUIUpdateRequest) (*FunctionUIResponse, error) {
	logicResp, err := logicfunction.NewFunctionUIUpdateLogic(ctx, svcCtx).FunctionUIUpdate(&logicfunction.FunctionUIUpdateRequest{
		ID:         req.ID,
		Schema:     req.Schema,
		Layout:     req.Layout,
		Components: req.Components,
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
		UISource:       stringFromAny(logicResp.UISource),
		UISourceDetail: stringFromAny(logicResp.UISourceDetail),
	}, nil
}

func functionUIHistory(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionUIHistoryRequest) (*FunctionUIHistoryResponse, error) {
	logicResp, err := logicfunction.NewFunctionUIHistoryLogic(ctx, svcCtx).FunctionUIHistory(&logicfunction.FunctionUIHistoryRequest{
		ID: req.ID,
	})
	if err != nil {
		return nil, err
	}
	items := make([]FunctionUIHistoryItem, 0, len(logicResp.Items))
	for _, item := range logicResp.Items {
		items = append(items, FunctionUIHistoryItem{
			Version:    item.Version,
			Schema:     item.Schema,
			Layout:     item.Layout,
			Components: item.Components,
			Message:    item.Message,
			CreatedBy:  item.CreatedBy,
			CreatedAt:  item.CreatedAt,
		})
	}
	return &FunctionUIHistoryResponse{Items: items}, nil
}

func functionUIRollback(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionUIRollbackRequest) (*FunctionUIRollbackResponse, error) {
	logicResp, err := logicfunction.NewFunctionUIRollbackLogic(ctx, svcCtx).FunctionUIRollback(&logicfunction.FunctionUIRollbackRequest{
		ID:      req.ID,
		Version: req.Version,
	})
	if err != nil {
		return nil, err
	}
	current := (*FunctionUIResponse)(nil)
	if resp, ok := logicResp.Current.(*logicfunction.FunctionUIResponse); ok && resp != nil {
		current = &FunctionUIResponse{
			Schema:         resp.Schema,
			Layout:         resp.Layout,
			Components:     resp.Components,
			Custom:         boolFromAny(resp.Custom),
			HasDefault:     resp.HasDefault,
			UISource:       stringFromAny(resp.UISource),
			UISourceDetail: stringFromAny(resp.UISourceDetail),
		}
	}
	return &FunctionUIRollbackResponse{
		AppliedVersion: logicResp.AppliedVersion,
		Current:        current,
	}, nil
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

// enforceFunctionPolicy checks if the user's roles are allowed to invoke the function
// based on the effective policy for that function.
// Returns the effective policy for auditing purposes.
func enforceFunctionPolicy(ctx context.Context, svcCtx *svc.ServiceContext, functionID string, userRoles []string) (*policy.Policy, error) {
	// Get function's risk level from registry
	riskLevel := policy.RiskMedium // default
	if svcCtx.RegistryStore != nil {
		if op, err := svcCtx.RegistryStore.GetOpenAPI(functionID); err == nil {
			if riskVal, ok := op.Extensions["x-risk-level"].(string); ok {
				riskLevel = policy.RiskLevel(riskVal)
			}
		}
	}

	// Get effective policy
	functionPolicy, err := svcCtx.PolicyManager.GetPolicy(ctx, functionID, riskLevel)
	if err != nil {
		// Log error but don't block invocation if policy check fails
		return nil, nil
	}

	// Use unified permission check - admin role bypasses all checks
	if err := utils.EnforceFunctionInvokePermission(userRoles, functionPolicy.AllowedRoles); err != nil {
		return nil, err
	}

	return functionPolicy, nil
}

// buildBroadcastResponse aggregates per-agent outcomes from a broadcast
// invocation. The legacy Result field is populated with the first successful
// response so existing clients that don't know about Broadcast keep working.
func buildBroadcastResponse(b *dispatch.BroadcastInvocation) *FunctionInvokeResponse {
	if b == nil {
		return &FunctionInvokeResponse{Broadcast: &BroadcastResult{}}
	}

	out := &FunctionInvokeResponse{
		Broadcast: &BroadcastResult{
			Total:   b.Total,
			Success: len(b.Successes),
			Failure: len(b.Failures),
			Results: make([]BroadcastAgentItem, 0, b.Total),
		},
	}

	for _, s := range b.Successes {
		item := BroadcastAgentItem{AgentID: s.AgentID}
		if s.Response != nil && len(s.Response.GetPayload()) > 0 {
			var v interface{}
			if err := json.Unmarshal(s.Response.GetPayload(), &v); err == nil {
				item.Result = v
				if out.Result == nil {
					out.Result = v
				}
			} else {
				item.Result = string(s.Response.GetPayload())
				if out.Result == nil {
					out.Result = item.Result
				}
			}
		}
		out.Broadcast.Results = append(out.Broadcast.Results, item)
	}

	for _, f := range b.Failures {
		out.Broadcast.Results = append(out.Broadcast.Results, BroadcastAgentItem{
			AgentID: f.AgentID,
			Error:   f.Err.Error(),
		})
	}

	return out
}
