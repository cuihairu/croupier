---
title: Dashboard 仓库接入 Runbook
icon: checklist
order: 34
category:
  - 系统架构
tag:
  - dashboard
  - integration
  - runbook
---

# Dashboard 仓库接入 Runbook

更新时间：2026-03-15

本 runbook 用于把当前仓库的 extensions 契约与模板，接入真实 dashboard 仓库。

## 1. 前置条件

1. 可访问 dashboard 仓库本地路径。
2. dashboard 使用 Node.js 18+。
3. dashboard 能执行 `npm ci`。

## 2. 一键 bootstrap

在本仓库执行：

```powershell
powershell -File scripts/contracts/bootstrap-dashboard-contracts.ps1 -DashboardRoot <dashboard_repo_path>
```

执行后 dashboard 仓库将获得：

- `src/services/contracts/extensions.ts`
- `src/services/generated/extensions-client/**`
- `src/services/adapters/extensions.ts`
- `src/services/api/extensions.ts`
- `src/services/errors/codes.ts`
- `src/services/errors/mapper.ts`
- `.github/workflows/extensions-regression.yml`

## 3. 页面接线顺序

1. `Extensions/Store` 页面接 `createExtensionApi().listCatalog/install`。
2. `Extensions/Installations` 页面接 `listInstallations/enable/disable/upgrade/uninstall`。
3. 安装详情页接 `listEvents/health-check/reconcile`。
4. 所有错误统一走 `mapExtensionError`。

## 4. 验证

1. 运行前端单测/构建。
2. 运行 Playwright：
   - `src/tests/smoke/extensions`
   - `src/tests/visual/extensions`
3. PR 中启用 `extensions-regression` 工作流。

## 5. 回滚

如果接入后出现阻断：

1. 保留生成的 `contracts/generated` 文件。
2. 页面层回退到旧调用，但保留 `errors/mapper`。
3. 分批切回 adapter 层，避免一次性全量替换。
