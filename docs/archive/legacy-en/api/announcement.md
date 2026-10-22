---
title: 公告 API
icon: megaphone
order: 23
---

# 公告 API（Announcements）

管理端公告 CRUD 与用户侧活跃公告拉取/关闭。实现以 `internal/api/announcement/` 为准。

- **认证**：均需 JWT（管理端为 admin 面，用户侧走 `protected` 组）。
- **响应**：成功直返业务 JSON；错误统一 `{error, message, details}`。

## 端点

| 方法   | 路径                                | 说明                                                      |
| ------ | ----------------------------------- | --------------------------------------------------------- |
| GET    | `/api/v1/admin/announcements`       | 管理列表（query `gameId` 非空时只返回对该游戏适用的公告） |
| POST   | `/api/v1/admin/announcements`       | 创建                                                      |
| PUT    | `/api/v1/admin/announcements/:id`   | 更新                                                      |
| DELETE | `/api/v1/admin/announcements/:id`   | 删除                                                      |
| GET    | `/api/v1/announcements/active`      | 用户侧当前生效公告                                        |
| POST   | `/api/v1/announcements/:id/dismiss` | 用户关闭（不再弹出）某条公告                              |

## 请求结构

```go
// POST 创建
type CreateRequest struct {
	Title     string     `json:"title" binding:"required"`
	ContentMd string     `json:"contentMd" binding:"required"` // Markdown 正文
	Audience  string     `json:"audience"`                     // all | role
	Role      string     `json:"role"`                         // audience=role 时的目标角色
	Popup     bool       `json:"popup"`                        // 是否弹窗展示
	Active    *bool      `json:"active"`                       // 上下架
	StartAt   *time.Time `json:"startAt"`
	EndAt     *time.Time `json:"endAt"`
	GameIds   []string   `json:"gameIds"`  // 绑定游戏（#45）；空/缺省 = 全服可见
}

// PUT 更新（指针字段：提供 = 更新，省略 = 保持）
type UpdateRequest struct {
	Title     *string    `json:"title"`
	ContentMd *string    `json:"contentMd"`
	Audience  *string    `json:"audience"`
	Role      *string    `json:"role"`
	Popup     *bool      `json:"popup"`
	Active    *bool      `json:"active"`
	StartAt   *time.Time `json:"startAt"`
	EndAt     *time.Time `json:"endAt"`
	GameIds   *[]string  `json:"gameIds"` // 非 nil 全量替换（空数组 = 清空绑定 → 全服可见）
}
```

## 语义注记

- **适用性判定**：公告未绑定游戏 = 全服可见；绑定了 `gameIds` 则仅对该游戏适用
  （管理列表 `?gameId=` 过滤同口径）。
- **用户侧**：`GET /announcements/active` 返回当前时间窗内且未过期的活跃公告；
  `POST /:id/dismiss` 记录关闭，弹窗不再展示。
