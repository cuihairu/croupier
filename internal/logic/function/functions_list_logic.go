package function

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionsListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数列表
func NewFunctionsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionsListLogic {
	return &FunctionsListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionsListLogic) FunctionsList(req *FunctionsListRequest) (*FunctionsListResponse, error) {
	// 获取当前用户角色，判断是否为管理员
	admin, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	isAdmin := false
	if err == nil && utils.HasAdminRole(ExtractRoleNames(roles)) {
		isAdmin = true
	}

	// 调试日志：记录当前用户和管理员状态
	if admin != nil {
		slog.InfoContext(l.ctx, "FunctionsList",
			"user", admin.Username,
			"isAdmin", isAdmin,
			"roles", ExtractRoleNames(roles),
			"gameId", req.GameId)
	} else if err != nil && !errors.Is(err, utils.ErrCurrentUserNotFound) {
		slog.InfoContext(l.ctx, "FunctionsList: admin retrieval failed", "error", err)
	}

	opts := model.ListFunctionsOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		Resource: strings.TrimSpace(req.Resource),
	}

	// 只有非管理员才需要按 GameID 过滤
	if !isAdmin && req.GameId != "" {
		opts.GameID = strings.TrimSpace(req.GameId)
	}

	if req.Status != 0 {
		status := req.Status
		opts.Status = &status
	}

	functions, err := l.dbFunctions(opts)
	if err != nil {
		return nil, err
	}

	index := make(map[string]Function, len(functions))
	for i := range functions {
		dto := utils.BuildFunctionDTO(&functions[i])
		converted := convertFromUtilsFunction(dto)
		index[converted.ID] = converted
	}

	for _, rt := range l.runtimeFunctions(req) {
		if existing, ok := index[rt.ID]; ok {
			if rt.Version != "" && rt.Version > existing.Version {
				existing.Version = rt.Version
			}
			if rt.Instances > existing.Instances {
				existing.Instances = rt.Instances
			}
			if existing.GameId == "" && rt.GameId != "" {
				existing.GameId = rt.GameId
			}
			index[rt.ID] = existing
			continue
		}
		index[rt.ID] = rt
	}

	items := make([]Function, 0, len(index))
	for _, v := range index {
		items = append(items, v)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	opts.PaginationOptions.Normalize()
	total := int64(len(items))
	start := opts.Offset()
	if start > len(items) {
		start = len(items)
	}
	end := start + opts.PageSize
	if end > len(items) {
		end = len(items)
	}
	items = items[start:end]

	return &FunctionsListResponse{
		Items: items,
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}

func (l *FunctionsListLogic) runtimeFunctions(req *FunctionsListRequest) []Function {
	store := l.svcCtx.RegistryStore
	if store == nil {
		return nil
	}

	gameIDFilter := strings.TrimSpace(req.GameId)
	index := make(map[string]Function)

	store.Mu().RLock()
	defer store.Mu().RUnlock()
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		if gameIDFilter != "" && !strings.EqualFold(strings.TrimSpace(sess.GameID), gameIDFilter) {
			continue
		}
		for fid, meta := range sess.Functions {
			fid = strings.TrimSpace(fid)
			if fid == "" {
				continue
			}
			item := index[fid]
			if item.ID == "" {
				summaryMap := make(map[string]string)
				if meta.Summary != "" {
					summaryMap["zh"] = meta.Summary
					summaryMap["en"] = meta.Summary
				}
				// Ensure tags is never nil
				tags := meta.Tags
				if tags == nil {
					tags = []string{}
				}
				item = Function{
					ID:       fid,
					Name:     fid,
					GameId:   sess.GameID,
					Status:   1,
					Version:  meta.Version,
					Resource: strings.TrimSpace(meta.Resource),
					Tags:     tags,
					Summary:  summaryMap,
				}
			}
			if req.Status != 0 && item.Status != req.Status {
				continue
			}
			if strings.TrimSpace(req.Resource) != "" && !strings.EqualFold(item.Resource, strings.TrimSpace(req.Resource)) {
				continue
			}
			if meta.Version != "" && meta.Version > item.Version {
				item.Version = meta.Version
			}
			item.Instances++
			index[fid] = item
		}
	}

	out := make([]Function, 0, len(index))
	for _, fn := range index {
		out = append(out, fn)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (l *FunctionsListLogic) dbFunctions(opts model.ListFunctionsOptions) ([]model.Function, error) {
	allOpts := opts
	allOpts.Page = 1
	allOpts.PageSize = 10000
	items, _, err := l.svcCtx.FunctionModel.List(l.ctx, allOpts)
	return items, err
}

// ExtractRoleNames 从角色列表中提取角色名称
func ExtractRoleNames(roles []model.Role) []string {
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		names = append(names, r.Name)
	}
	return names
}
