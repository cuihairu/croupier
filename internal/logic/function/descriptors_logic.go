package function

import (
	"context"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/descriptors"
	"github.com/cuihairu/croupier/internal/dashboard/normalizer"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type DescriptorsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数描述符列表
func NewDescriptorsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DescriptorsLogic {
	return &DescriptorsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DescriptorsV2 returns normalized FunctionSpec list using the shared
// Dashboard descriptor collector and normalizer. This endpoint must not infer
// resource/page semantics differently from Resource API.
func (l *DescriptorsLogic) DescriptorsV2(req *DescriptorsRequest) (*DescriptorsV2Result, error) {
	if err := l.checkReadPermission(); err != nil {
		return nil, err
	}

	category := ""
	if req != nil {
		category = strings.TrimSpace(firstNonEmpty(req.Type, req.Category))
	}

	inputs := descriptors.Collect(l.ctx, l.svcCtx)
	results, _ := normalizer.NormalizeBatch(inputs)

	functions := make([]spec.FunctionSpec, 0, len(results))
	for _, result := range results {
		fn := result.Function
		if fn.ID == "" {
			continue
		}
		if category != "" && fn.Category != category {
			continue
		}
		functions = append(functions, fn)
	}

	sort.Slice(functions, func(i, j int) bool {
		return functions[i].ID < functions[j].ID
	})

	return &DescriptorsV2Result{
		Functions: functions,
	}, nil
}

func (l *DescriptorsLogic) checkReadPermission() error {
	username, _ := utils.CurrentUsername(l.ctx)
	if username == "" {
		return nil
	}

	_, rolesFromDB, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return err
	}
	roleNames := utils.RoleNamesFromModels(rolesFromDB)
	permIDs, err := utils.PermissionIDsFromRoles(l.ctx, l.svcCtx, rolesFromDB)
	if err != nil {
		return err
	}
	if utils.HasAdminRole(roleNames) || utils.HasPermissionID(permIDs, "functions:read") || utils.HasPermissionID(permIDs, "*") {
		return nil
	}
	return errorx.NewForbidden("无权访问函数目录")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}
