import { defineConfig } from "vitepress";
import { withMermaid } from "vitepress-plugin-mermaid";

const sharedHead = [
  ["meta", { name: "viewport", content: "width=device-width,initial-scale=1" }],
  ["meta", { name: "theme-color", content: "#93394d" }],
];

export default withMermaid(
  defineConfig({
    title: "Croupier",
    // CR-FIX-17: dead links fail the build again; only links into the
    // srcExclude'd review/ and archive/ trees are whitelisted (those pages
    // are deliberately not published, but stay reachable in the repo). An
    // unknown dead link = red build.
    ignoreDeadLinks: [/\/review\//, /\/archive\//],
    // 2026-10-20 用户令：英文内页必须上线，撤除 srcExclude（review/archive 仍藏）
    srcExclude: ["archive/**", "review/**", "zh/review/**"],
    head: sharedHead,
    base: "/croupier/",
    locales: {
      root: {
        label: "English",
        lang: "en-US",
        description:
          "Game operations control plane and Agent collaboration platform",
        themeConfig: {
          logo: "/logo.png",
          langMenuLabel: "Change language",
          returnToTopLabel: "Return to top",
          sidebarMenuLabel: "Documentation",
          docFooter: { prev: "Previous", next: "Next" },
          outline: { level: [2, 3], label: "On this page" },
          nav: [
            { text: "Guide", link: "/guide/" },
            { text: "Architecture", link: "/architecture/" },
            { text: "API", link: "/api/" },
            { text: "Operations", link: "/operations/" },
            { text: "Analytics", link: "/analytics/" },
            { text: "SDK", link: "/sdks/" },
          ],
          sidebar: {
            "/guide/": [
              {
                text: "Getting started",
                items: [
                  { text: "Introduction", link: "/guide/" },
                  { text: "Quick start", link: "/guide/quick-start" },
                  { text: "Installation", link: "/guide/installation" },
                  {
                    text: "Interface preview",
                    link: "/guide/interface-preview",
                  },
                ],
              },
              {
                text: "Concepts",
                items: [
                  { text: "System overview", link: "/guide/concepts/overview" },
                  {
                    text: "Function management",
                    link: "/guide/concepts/function-management",
                  },
                  { text: "Permissions", link: "/guide/concepts/permissions" },
                ],
              },
            ],
            "/architecture/": [
              {
                text: "Architecture",
                items: [
                  { text: "Overview", link: "/architecture/" },
                  {
                    text: "Runtime and lifecycles",
                    link: "/architecture/runtime-topology",
                  },
                  { text: "Layers", link: "/architecture/layers" },
                  {
                    text: "Terms and layering",
                    link: "/architecture/terms-and-layering",
                  },
                  { text: "Data flow", link: "/architecture/data-flow" },
                  {
                    text: "Session lifecycle",
                    link: "/architecture/session-lifecycle",
                  },
                  {
                    text: "SDK wire protocol",
                    link: "/architecture/sdk-wire-protocol",
                  },
                  {
                    text: "Composition model",
                    link: "/architecture/composition-model",
                  },
                  {
                    text: "Dashboard glossary",
                    link: "/architecture/dashboard-glossary",
                  },
                  {
                    text: "Transport glossary",
                    link: "/architecture/transport-glossary",
                  },
                ],
              },
            ],
            "/operations/": [
              {
                text: "Operations",
                items: [
                  { text: "Overview", link: "/operations/" },
                  {
                    text: "Health monitoring",
                    link: "/operations/health-monitoring",
                  },
                  {
                    text: "External service probing",
                    link: "/operations/external-service-probing",
                  },
                  { text: "Cost control", link: "/operations/cost-control" },
                  { text: "Ops dashboard", link: "/operations/ops-dashboard" },
                  {
                    text: "Config switch effects",
                    link: "/operations/config-effects",
                  },
                  { text: "Risk control", link: "/operations/risk-control" },
                  {
                    text: "Security configuration",
                    link: "/operations/security",
                  },
                  {
                    text: "Mobile threat model",
                    link: "/operations/mobile-threat-model",
                  },
                ],
              },
            ],
            "/development/": [
              {
                text: "Development",
                items: [
                  {
                    text: "Development lifecycle",
                    link: "/development/lifecycle",
                  },
                  {
                    text: "Versioning and dependency policy",
                    link: "/development/versioning",
                  },
                  {
                    text: "Frontend style boundaries",
                    link: "/development/frontend-styles",
                  },
                ],
              },
            ],
            "/api/": [
              {
                text: "API",
                items: [
                  { text: "Overview", link: "/api/" },
                  { text: "Authentication API", link: "/api/auth" },
                  { text: "Function API", link: "/api/function" },
                  { text: "Resource API", link: "/api/resource" },
                ],
              },
            ],
            "/analytics/": [
              {
                text: "Analytics",
                items: [
                  { text: "Overview", link: "/analytics/" },
                  {
                    text: "Core and external analytics",
                    link: "/analytics/core-and-external",
                  },
                  {
                    text: "Game metrics panorama",
                    link: "/analytics/game-metrics-overview",
                  },
                ],
              },
            ],
            "/sdks/": [
              {
                text: "SDKs",
                items: [{ text: "Overview", link: "/sdks/" }],
              },
            ],
          },
        },
      },
      zh: {
        label: "中文",
        lang: "zh-CN",
        description: "分布式游戏运营控制面与 Agent 协同平台",
        themeConfig: {
          logo: "/logo.png",
          langMenuLabel: "切换语言",
          returnToTopLabel: "回到顶部",
          sidebarMenuLabel: "文档",
          docFooter: { prev: "上一页", next: "下一页" },
          outline: { level: [2, 3], label: "本页内容" },
          nav: [
            { text: "指南", link: "/zh/guide/" },
            { text: "架构", link: "/zh/architecture/" },
            { text: "API", link: "/zh/api/" },
            { text: "运维", link: "/zh/operations/" },
            { text: "数据分析", link: "/zh/analytics/" },
            { text: "SDK", link: "/zh/sdks/" },
          ],
          sidebar: {
            "/zh/guide/": [
              {
                text: "入门",
                items: [
                  { text: "简介", link: "/zh/guide/" },
                  { text: "快速开始", link: "/zh/guide/quick-start" },
                  { text: "安装", link: "/zh/guide/installation" },
                  { text: "界面预览", link: "/zh/guide/interface-preview" },
                ],
              },
              {
                text: "核心概念",
                items: [
                  { text: "系统概述", link: "/zh/guide/concepts/overview" },
                  {
                    text: "函数管理",
                    link: "/zh/guide/concepts/function-management",
                  },
                  { text: "权限控制", link: "/zh/guide/concepts/permissions" },
                ],
              },
            ],
            "/zh/architecture/": [
              {
                text: "架构",
                items: [
                  { text: "概述", link: "/zh/architecture/" },
                  {
                    text: "运行架构与生命周期",
                    link: "/zh/architecture/runtime-topology",
                  },
                  { text: "分层", link: "/zh/architecture/layers" },
                  {
                    text: "术语与分层",
                    link: "/zh/architecture/terms-and-layering",
                  },
                  { text: "数据流", link: "/zh/architecture/data-flow" },
                  {
                    text: "会话生命周期",
                    link: "/zh/architecture/session-lifecycle",
                  },
                  {
                    text: "SDK Wire 协议",
                    link: "/zh/architecture/sdk-wire-protocol",
                  },
                  {
                    text: "组合模型",
                    link: "/zh/architecture/composition-model",
                  },
                  {
                    text: "看板术语表",
                    link: "/zh/architecture/dashboard-glossary",
                  },
                  {
                    text: "传输术语表",
                    link: "/zh/architecture/transport-glossary",
                  },
                  {
                    text: "数据库迁移策略",
                    link: "/zh/architecture/database-migration-strategy",
                  },
                  {
                    text: "函数流水线阻塞项",
                    link: "/zh/architecture/function-pipeline-blockers",
                  },
                  {
                    text: "LocalizedText 契约",
                    link: "/zh/architecture/localized-text-contract",
                  },
                  {
                    text: "会话运行时全景",
                    link: "/zh/architecture/session-runtime-landscape",
                  },
                ],
              },
            ],
            "/zh/development/": [
              {
                text: "开发",
                items: [
                  {
                    text: "开发周期与发布说明",
                    link: "/zh/development/lifecycle",
                  },
                  {
                    text: "版本与依赖策略",
                    link: "/zh/development/versioning",
                  },
                  {
                    text: "前端样式系统边界",
                    link: "/zh/development/frontend-styles",
                  },
                ],
              },
            ],
            "/zh/operations/": [
              {
                text: "运维",
                items: [
                  { text: "概述", link: "/zh/operations/" },
                  { text: "DevOps 说明", link: "/zh/operations/devops" },
                  {
                    text: "权限控制说明",
                    link: "/zh/operations/access-control",
                  },
                  {
                    text: "运维健康监控",
                    link: "/zh/operations/health-monitoring",
                  },
                  {
                    text: "外部服务探测",
                    link: "/zh/operations/external-service-probing",
                  },
                  { text: "成本控制", link: "/zh/operations/cost-control" },
                  { text: "运维大屏", link: "/zh/operations/ops-dashboard" },
                  {
                    text: "配置开关效果",
                    link: "/zh/operations/config-effects",
                  },
                  { text: "风控说明", link: "/zh/operations/risk-control" },
                  { text: "安全配置", link: "/zh/operations/security" },
                  { text: "移动端威胁模型", link: "/zh/operations/mobile-threat-model" },
                ],
              },
            ],
            "/zh/api/": [
              {
                text: "API",
                items: [
                  { text: "概述", link: "/zh/api/" },
                  { text: "认证 API", link: "/zh/api/auth" },
                  { text: "函数 API", link: "/zh/api/function" },
                  { text: "资源 API", link: "/zh/api/resource" },
                ],
              },
            ],
            "/zh/analytics/": [
              {
                text: "数据分析",
                items: [
                  { text: "概述", link: "/zh/analytics/" },
                  {
                    text: "核心指标与外部分析",
                    link: "/zh/analytics/core-and-external",
                  },
                  {
                    text: "游戏指标全景",
                    link: "/zh/analytics/game-metrics-overview",
                  },
                ],
              },
            ],
            "/zh/sdks/": [
              {
                text: "SDK",
                items: [{ text: "概览", link: "/zh/sdks/" }],
              },
            ],
          },
        },
      },
    },
  }),
);
