# Phase 5 — Types Struct Tag 迁移

状态: ⬜ 待开始

## 目标

将 `internal/types/types.go` 中的 go-zero 专有 struct tag 替换为 Gin 兼容的标准 tag。

## 统计

- `path:"..."` tag: 109 处 → 替换为 `uri:"..."`
- `json:"...,optional"` tag: 400 处 → 移除 `,optional` 后缀

## Tag 对应关系

| go-zero tag | Gin tag | 说明 |
|-------------|---------|------|
| `path:"id"` | `uri:"id"` | URL 路径参数，配合 `ShouldBindUri` |
| `json:"name,optional"` | `json:"name,omitempty"` | 可选字段（注意语义差异） |
| `form:"page,optional"` | `form:"page"` | Query 参数，直接移除 optional |

> **注意**：go-zero 的 `optional` 表示"解析时不报错"，Gin 的 `omitempty` 表示"序列化时忽略零值"。
> 对于 Request struct，直接移除 `,optional` 即可（Gin 不会因为字段缺失而报错）。
> 对于 Response struct，根据需要决定是否加 `omitempty`。

## 操作命令

```bash
# 在 types.go 中批量替换
# 1. path: → uri:
sed -i 's/`path:"/`uri:"/g' internal/types/types.go

# 2. 移除 json tag 中的 ,optional
sed -i 's/,optional"/"/g' internal/types/types.go

# 3. 移除 form tag 中的 ,optional
sed -i 's/form:"\([^"]*\),optional"/form:"\1"/g' internal/types/types.go
```

> 执行后用 `git diff internal/types/types.go` 检查变更是否符合预期。

## 检查清单

- [ ] 5.1 备份 types.go（`git stash` 或确认在 feature 分支）
- [ ] 5.2 执行 `path:` → `uri:` 替换（109 处）
- [ ] 5.3 执行移除 `,optional` 替换（400 处）
- [ ] 5.4 检查 `AdminsListRequest` 等带 `Status int` 的 struct（`-1` 默认值逻辑需保留）
- [ ] 5.5 运行 `go build ./...` 确认无编译错误
- [ ] 5.6 运行相关测试

## 特殊情况

### Status 字段默认值

`AdminsListRequest.Status` 使用 `-1` 表示"不过滤"，这是业务逻辑，不是 tag 问题，无需修改。

### 混合绑定 struct

有些 Request struct 同时包含 `uri:` 和 `json:` / `form:` tag，例如：

```go
type AdminUpdateRequest struct {
    ID   string `uri:"id"`           // 来自 URL 路径
    Name string `json:"name"`        // 来自 JSON body
}
```

对应 handler 需要分两步绑定：
```go
var req types.AdminUpdateRequest
if err := c.ShouldBindUri(&req); err != nil { ... }
if err := c.ShouldBindJSON(&req); err != nil { ... }
```
