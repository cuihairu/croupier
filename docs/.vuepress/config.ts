import { defineUserConfig } from 'vuepress'
import { defaultTheme } from '@vuepress/theme-default'
import { viteBundler } from '@vuepress/bundler-vite'
import { searchPlugin } from '@vuepress/plugin-search'
import { markdownChartPlugin } from '@vuepress/plugin-markdown-chart'

export default defineUserConfig({
  lang: 'zh-CN',
  title: 'Croupier',
  description: '分布式游戏管理系统文档',
  base: '/croupier/',
  bundler: viteBundler({
    viteOptions: {
      build: {
        chunkSizeWarningLimit: 1200,
      },
    },
  }),
  theme: defaultTheme({
    repo: 'cuihairu/croupier',
    docsDir: 'server/docs',
    docsBranch: 'main',
    editLinkText: '在 GitHub 上编辑此页',
    lastUpdated: true,
    contributors: false,
    navbar: [
      { text: '文档', link: '/' },
      { text: 'GitHub', link: 'https://github.com/cuihairu/croupier' },
    ],
    sidebar: {
      '/': [
        {
          text: '概览',
          children: [
            '/README.md',
            '/directory-structure.md',
            '/deployment.md',
            '/config.md',
            '/配置管理系统使用指南.md',
            '/security.md',
            '/e2e-example.md',
          ],
        },
        {
          text: '架构与设计',
          children: [
            '/ARCHITECTURE.md',
            '/VIRTUAL_OBJECT_DESIGN.md',
            '/VIRTUAL_OBJECT_QUICK_REFERENCE.md',
            '/ui-and-views.md',
            '/wire-and-di.md',
            '/control-capabilities.md',
            '/approvals-storage.md',
          ],
        },
        {
          text: '函数管理',
          children: [
            '/FUNCTION_MANAGEMENT_README.md',
            '/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md',
            '/FUNCTION_MANAGEMENT_EXECUTIVE_SUMMARY.md',
            '/FUNCTION_MANAGEMENT_ARCHITECTURE_ANALYSIS.md',
            '/FUNCTION_MANAGEMENT_COMPARISON.md',
            '/函数管理系统重构实施指南.md',
            '/函数管理系统重构部署清单.md',
            '/函数管理系统重构完成总结.md',
          ],
        },
        {
          text: 'SDK 开发',
          children: [
            '/sdk-development.md',
            '/SDK_VERSION_MANAGEMENT.md',
            '/SDK_HOTRELOAD_SUPPORT.md',
            '/HOT_RELOAD_SOLUTIONS.md',
            '/HOTRELOAD_BEST_PRACTICES.md',
            '/HOTRELOAD_IMPLEMENTATION_SUMMARY.md',
          ],
        },
        {
          text: 'C++ SDK',
          children: [
            '/CPP_SDK_DOCS_INDEX.md',
            '/CPP_SDK_QUICK_REFERENCE.md',
            '/CPP_SDK_ANALYSIS_SUMMARY.md',
            '/CPP_SDK_ANALYSIS.md',
            '/CPP_SDK_DEEP_ANALYSIS.md',
            '/CPP_SDK_DIRECTORY_INDEX.md',
            '/CPP_SDK_BUILD_OPTIMIZATION.md',
            '/VCPKG_OPTIMIZATION.md',
            '/sdks/cpp/README.md',
            '/sdks/cpp/CONFIG_GUIDE.md',
            '/sdks/cpp/PLUGIN_GUIDE.md',
            '/sdks/cpp/VIRTUAL_OBJECT_REGISTRATION.md',
          ],
        },
        {
          text: '生成器与协议',
          children: [
            '/generator.md',
            '/PROTO_OPTIONS_GUIDE.md',
            '/providers-manifest.md',
            '/api.md',
            '/http-adapter.md',
          ],
        },
        {
          text: '运营与分析',
          children: [
            '/analytics/README.md',
            '/analytics/quick-start.md',
            '/analytics/game-metrics-overview.md',
            '/analytics/metrics-dictionary.md',
            '/analytics/api-reference.md',
            '/analytics/sdk-reference.md',
            '/analytics/data-collection-architecture.md',
            '/analytics/opentelemetry-integration.md',
            '/analytics/current-system-analysis.md',
            '/analytics/enhancement-plan.md',
            '/analytics/best-practices.md',
            '/analytics/troubleshooting.md',
            '/analytics/game-type-adaptation.md',
            '/analytics/casual-games.md',
            '/analytics/competitive-games.md',
            '/analytics/rpg-games.md',
            '/analytics/strategy-games.md',
            '/analytics/clickhouse-schema.md',
            '/analytics/instrumentation-spec-cn.md',
            '/analytics/playbooks/card-ccg-cn.md',
            '/analytics/playbooks/board-table-cn.md',
            '/analytics/playbooks/idle-cn.md',
            '/analytics/playbooks/tower-defense-cn.md',
          ],
        },
        {
          text: '运维与监控',
          children: [
            '/ops/remote-access-web.md',
            '/metrics.md',
            '/tracing.md',
            '/ingest-signing.md',
          ],
        },
        {
          text: '运营设计',
          children: [
            '/assignments.md',
            '/game-roles-design.md',
            '/complete-game-roles-design.md',
          ],
        },
      ],
    },
  }),
  plugins: [
    searchPlugin(),
    markdownChartPlugin({
      mermaid: true,
    }),
  ],
})
