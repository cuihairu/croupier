import { defineConfig } from "vitepress";
import { withMermaid } from "vitepress-plugin-mermaid";

const sharedHead = [
  ["meta", { name: "viewport", content: "width=device-width,initial-scale=1" }],
  ["meta", { name: "theme-color", content: "#93394d" }],
];

export default withMermaid(
  defineConfig({
    title: "Croupier",
    srcExclude: [
      "archive/**",
      "guide/**",
      "architecture/**",
      "operations/**",
      "api/**",
      "analytics/**",
      "sdks/**",
      "development/**",
      "design/**",
      "research/**",
      "review/**",
    ],
    head: sharedHead,
    base: "/croupier/",
    locales: {
      root: {
        label: "English",
        lang: "en-US",
        description:
          "Game operations control plane and Agent collaboration platform",
        themeConfig: {
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
                ],
              },
              {
                text: "Concepts",
                items: [
                  {
                    text: "System overview",
                    link: "/guide/concepts/overview",
                  },
                  {
                    text: "Function management",
                    link: "/guide/concepts/function-management",
                  },
                  {
                    text: "Permissions",
                    link: "/guide/concepts/permissions",
                  },
                ],
              },
            ],
            "/architecture/": [
              {
                text: "Architecture",
                items: [
                  { text: "Overview", link: "/architecture/" },
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
                    text: "SDK Wire protocol",
                    link: "/architecture/sdk-wire-protocol",
                  },
                ],
              },
            ],
            "/operations/": [
              {
                text: "Operations",
                items: [{ text: "Overview", link: "/operations/" }],
              },
            ],
            "/api/": [
              {
                text: "API",
                items: [{ text: "Overview", link: "/api/" }],
              },
            ],
            "/analytics/": [
              {
                text: "Analytics",
                items: [{ text: "Overview", link: "/analytics/" }],
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
                ],
              },
            ],
            "/zh/operations/": [
              {
                text: "运维",
                items: [{ text: "概述", link: "/zh/operations/" }],
              },
            ],
            "/zh/api/": [
              {
                text: "API",
                items: [{ text: "概述", link: "/zh/api/" }],
              },
            ],
            "/zh/analytics/": [
              {
                text: "数据分析",
                items: [{ text: "概述", link: "/zh/analytics/" }],
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
