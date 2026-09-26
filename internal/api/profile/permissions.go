package profile

import (
	"context"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/rbac"
)

// 权限解析：把「用户持有的 permission id 列表」解析成前端可渲染的资源 → 操作
// 结构，并给出访问级别。
//
// 背景（docs/BUGS.md BUG-018 / BUG-019）：此前两个接口都在编造数据——
//   - GET /profile/games 把 `Permissions` 硬编码成 `[]string{}`，对所有用户、
//     所有游戏恒为空（不是查询过滤不到，是字面量），admin 的「游戏访问权限」
//     永远是一块空白；
//   - GET /profile/permissions 的 `permissions[]` 是「一个角色一条、resource
//     恒为字面量 "role"、把角色名塞进 actions」，完全不是权限列表。
//
// 而真正的权限 id 其实**已经查到了**（`permissionIDs`），只是到那里就停了；
// 更糟的是 `permissionIDs` 里混进了角色名（`appendPermission(role)`），
// 前端拿去和权限目录比对时会把 "admin" 当成一条不存在的权限。
// 这里把真实数据接通，并把角色名与权限 id 彻底分开。
//
// 口径说明：RBAC 挂在角色上、**不按游戏维度切分**。单个用户在所有游戏上的权限
// 集合是同一个；真正按游戏切分的是「可见游戏 / 可见环境」（admin_game_env_scopes，
// GetUserGames 已用它过滤）。因此 per-game 权限要么是「全部权限」（持通配），
// 要么就是那一份通用权限集，不存在「只在这个游戏有某个操作」的语义。

// accessFull / accessScoped / accessNone 是访问级别。
const (
	// accessFull 持有通配权限（* / admin:all / admin 角色），等价于全部权限。
	accessFull = "full"
	// accessScoped 持有显式权限集（可能为空集：只有角色名、没挂任何权限）。
	accessScoped = "scoped"
	// accessNoAccess 无任何显式权限且无通配。
	accessNoAccess = "none"
)

// accessScopeRole 是权限的授权维度说明，随响应下发给前端展示，避免用户
// 误以为「每个游戏能单独授权」。
const accessScopeRole = "role"

// resolvedPermissions 是解析结果。
type resolvedPermissions struct {
	// IDs 用户实际持有的**权限 id**（已去空白、去重、排序）。不含角色名。
	IDs []string
	// Groups resource → actions，按字典序稳定排序。
	Groups []ProfilePermission
	// FullAccess 表示用户持有通配权限（* 或 admin:all）。
	FullAccess bool
	// AccessLevel 是 accessFull / accessScoped / accessNone 之一。
	AccessLevel string
}

// resolvePermissions 把持有的权限 id 解析成资源 → 操作分组。
//
// 资源轴取自**权限 id 的前缀**（`pages:write` → resource=pages），而不是
// permissions 表的 `resource` 列：那一列存的是 module（如 dashboard），
// 38 条目录会塌缩成 7 个值，渲染出来的树只剩 7 个节点，且与 id 对不上。
func resolvePermissions(owned []string) resolvedPermissions {
	ids := dedupeSorted(owned)

	fullAccess := false
	actionsByResource := make(map[string]map[string]struct{})
	for _, id := range ids {
		resource, action := rbac.SplitLogicalPermission(id)
		// 判定「全局全量」必须**两个维度都通配**。
		//
		// 只看其中之一是错的：`user:*` 只是「user 这个资源的全部操作」，
		// 无权访问其它资源；把它算成 fullAccess 会让前端把所有条目都渲染成
		// 「已授权」，又变成一次假状态（这正是本条要消灭的东西）。
		// 真正等价于全部权限的只有 resource 与 action 同时为 `*`
		// （对应 id 为 "" / "*" / "admin:all"）。
		if resource == "*" && action == "*" {
			fullAccess = true
		}
		if resource == "" || resource == "*" {
			// 全局通配项不落到具体资源分组上——否则会渲染出一个假的 "*" 资源节点。
			continue
		}
		set := actionsByResource[resource]
		if set == nil {
			set = make(map[string]struct{})
			actionsByResource[resource] = set
		}
		set[action] = struct{}{}
	}

	groups := make([]ProfilePermission, 0, len(actionsByResource))
	for res, set := range actionsByResource {
		actions := make([]string, 0, len(set))
		for a := range set {
			actions = append(actions, a)
		}
		sort.Strings(actions)
		groups = append(groups, ProfilePermission{Resource: res, Actions: actions})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Resource < groups[j].Resource })

	level := accessNoAccess
	switch {
	case fullAccess:
		level = accessFull
	case len(ids) > 0:
		level = accessScoped
	}

	return resolvedPermissions{IDs: ids, Groups: groups, FullAccess: fullAccess, AccessLevel: level}
}

// userPermissionSet 是「某个用户的角色 + 权限」解析结果。
type userPermissionSet struct {
	// RoleGrants 逐角色的权限授予明细（供角色 → 资源 → 操作的树使用）。
	RoleGrants []RolePermissionGrant
	// Roles 角色名（去重、排序）。
	Roles []string
	// PermissionIDs 真正的权限 id（去重、排序），**不含角色名**。
	PermissionIDs []string
	// IsAdmin 命中 admin / super_admin 角色。
	IsAdmin bool
	// Resolved 由 PermissionIDs 解析出的分组结构。
	Resolved resolvedPermissions
}

// collectUserPermissionSet 汇总用户角色与角色所挂权限。
//
// 关键修正（BUG-019）：角色名**不再**混进 PermissionIDs。旧实现对每个角色调
// appendPermission(role)，于是 admin 的 permissionIDs 里同时出现
// ["*", "admin", "user:read", ...]；前端把 permissionIDs 与权限目录做差集来
// 渲染「已授权/未授权」时，"admin" 会变成一条永远查不到的假权限。
func collectUserPermissionSet(ctx context.Context, s *Service, roleModels []model.Role) userPermissionSet {
	roles := make([]string, 0, len(roleModels))
	isAdmin := false
	roleIDs := make([]uint, 0, len(roleModels))
	seenRole := make(map[string]struct{}, len(roleModels))

	for _, role := range roleModels {
		name := strings.TrimSpace(role.Name)
		if name == "" {
			continue
		}
		roleIDs = append(roleIDs, role.ID)
		if _, dup := seenRole[name]; dup {
			continue
		}
		seenRole[name] = struct{}{}
		roles = append(roles, name)
		if isPrivilegedRoleName(name) {
			isAdmin = true
		}
	}
	sort.Strings(roles)

	// 只有真正的权限 id 进这里；通配符是权限 id 的一种（configs/permissions.json
	// 里就有 "*" 与 "admin:all" 两条），因此 admin 角色补上通配是合理的。
	owned := make([]string, 0, len(roleModels)*4+2)
	if isAdmin {
		owned = append(owned, "admin:all", "*")
	}

	// 逐角色明细：回答「这个操作是哪个角色授予的」。角色名与 roleID 一一对应，
	// 因此放在同一层构建，避免再次出现「角色名与权限错配」。
	grants := make([]RolePermissionGrant, 0, len(roleModels))
	if s.roleModel != nil && len(roleIDs) > 0 {
		if rolePermMap, err := s.roleModel.GetRolesPermissionIDs(ctx, roleIDs); err == nil {
			byID := make(map[uint]string, len(roleModels))
			for _, role := range roleModels {
				if n := strings.TrimSpace(role.Name); n != "" {
					byID[role.ID] = n
				}
			}
			seenGrant := make(map[string]struct{}, len(roleModels))
			for _, role := range roleModels {
				name := byID[role.ID]
				if name == "" {
					continue
				}
				if _, dup := seenGrant[name]; dup {
					continue
				}
				seenGrant[name] = struct{}{}
				// 角色没挂权限时也保留条目，否则树上会缺失该角色节点
				grants = append(grants, RolePermissionGrant{
					Role:          name,
					PermissionIDs: dedupeSorted(rolePermMap[role.ID]),
				})
				owned = append(owned, rolePermMap[role.ID]...)
			}
			sort.Slice(grants, func(i, j int) bool { return grants[i].Role < grants[j].Role })
		}
	}

	resolved := resolvePermissions(owned)
	return userPermissionSet{
		RoleGrants:    grants,
		Roles:         roles,
		PermissionIDs: resolved.IDs,
		IsAdmin:       isAdmin,
		Resolved:      resolved,
	}
}

// isPrivilegedRoleName 判定角色是否等价于管理员。
func isPrivilegedRoleName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "admin", "super_admin":
		return true
	default:
		return false
	}
}

// perGamePermissions 给出某个游戏下应当展示的权限标签。
//
// 因为 RBAC 不按游戏切分，同一个用户在所有可见游戏上的权限集完全相同：
//   - 持通配 → 返回 ["*"]，由前端渲染成「全部权限」；
//   - 否则 → 返回真实持有的权限 id（可能为空数组，此时前端必须显式说明
//     「无显式权限」，不能留白）。
//
// 这比旧实现（对所有用户所有游戏恒返回 []string{}）多出的正是 admin 场景：
// 之前 admin 的这一栏永远是空的。
func perGamePermissions(resolved resolvedPermissions) []string {
	if resolved.FullAccess {
		return []string{"*"}
	}
	// 复制一份，避免调用方改动底层切片
	return append([]string(nil), resolved.IDs...)
}

// dedupeSorted 去空白、去重、排序。
func dedupeSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		t := strings.TrimSpace(v)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
