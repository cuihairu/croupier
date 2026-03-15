# Extensions UI Regression Gate

更新时间：2026-03-15

## 1. 目标

为扩展域页面建立固定的发布门禁，避免 UI 与契约漂移。

## 2. 门禁范围

- Store 列表页
- Installations 列表页
- 安装详情页（events/health/config）
- 关键动作：install/upgrade/enable/disable/uninstall

## 3. 门禁项

1. Contract sanity
- OpenAPI 契约文件存在且通过基础校验
- 前端错误码映射包含基线错误码

2. Smoke E2E
- 覆盖 `install -> disable -> enable -> health-check -> events page`

3. Visual regression
- 关键页面截图比对：
  - store
  - installations
  - installation detail

4. Artifact upload
- 失败时上传截图与 trace 便于定位

## 4. 失败策略

- PR 阶段：任一门禁失败即阻断合并
- main 阶段：失败后自动通知并保留 artifacts

## 5. 建议工作流模板

参考：

- `docs/contracts/templates/dashboard/.github/workflows/extensions-regression.yml`
