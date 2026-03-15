# Extensions Integration Smoke Baseline

更新时间：2026-03-15

本文档定义扩展域前后端联调最小基线，覆盖安装、升级、启停、健康检查、事件分页。

## 1. 测试前提

- Server 启动并可访问 `/api/v1/*`
- 至少存在一个可安装扩展（建议 `official.external-platform`）
- 测试账号具备扩展读写权限
- Dashboard 已接入 contracts/adapters/error-mapper

## 2. 用例清单

1. Catalog 列表可读
- 请求：`GET /api/v1/extensions/catalog?page=1&page_size=20`
- 断言：返回 `items[]`，字段包含 `extension_id/installed`

2. 安装扩展
- 请求：`POST /api/v1/extensions/install`
- 断言：返回 `installation_id`，`status` 非空

3. 重复安装冲突
- 同 scope/target 重复安装同扩展
- 断言：错误 `details.code=extension_already_installed`

4. 启停流程
- 请求：`POST /api/v1/extensions/:id/disable` -> `enable`
- 断言：`enabled` 状态正确切换

5. 升级流程
- 请求：`POST /api/v1/extensions/:id/upgrade`
- 断言：`release_version` 更新为目标版本

6. 健康检查
- 请求：`POST /api/v1/extensions/:id/health-check`
- 断言：成功响应，事件流有健康检查事件

7. 事件分页
- 请求：`GET /api/v1/extensions/:id/events?page=1&page_size=10`
- 断言：返回 `items/total`，分页参数生效

8. 卸载依赖阻塞
- 被依赖时卸载
- 断言：错误 `details.code=dependency_blocked` 且 `details.blockers` 非空

## 3. 前端断言点

- 错误提示由 `mapExtensionError` 统一映射
- 页面不直接解析后端 raw 字段
- 升级/启停后列表状态与详情一致

## 4. 通过标准

- 8 条用例全部通过
- 无 P0/P1 阻塞问题
- 同一套用例在 CI 可重复执行
