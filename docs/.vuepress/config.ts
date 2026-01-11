import { defineUserConfig } from 'vuepress'
import { defaultTheme } from '@vuepress/theme-default'
import { viteBundler } from '@vuepress/bundler-vite'
import { searchPlugin } from '@vuepress/plugin-search'
import { markdownChartPlugin } from '@vuepress/plugin-markdown-chart'
import { markdownMathPlugin } from '@vuepress/plugin-markdown-math'

export default defineUserConfig({
  lang: 'zh-CN',
  title: 'Croupier',
  description: '分布式游戏管理系统 - 统一的游戏运营控制面',
  head: [
    ['meta', { name: 'viewport', content: 'width=device-width,initial-scale=1' }],
    ['meta', { name: 'keywords', content: 'croupier,游戏管理,gm系统,分布式系统,gRPC' }],
    ['meta', { name: 'theme-color', content: '#3eaf7c' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:locale', content: 'zh-CN' }],
    ['meta', { property: 'og:title', content: 'Croupier | 分布式游戏管理系统' }],
    ['meta', { property: 'og:site_name', content: 'Croupier' }],
  ],
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
    repoLabel: 'GitHub',
    docsDir: 'server/docs',
    docsBranch: 'main',
    docsRepo: 'https://github.com/cuihairu/croupier',
    editLinkText: '在 GitHub 上编辑此页',
    lastUpdated: true,
    lastUpdatedText: '最后更新',
    contributors: false,
    logo: '/logo.png',

    // 主导航
    navbar: [
      {
        text: '指南',
        link: '/guide/',
      },
      {
        text: '架构',
        link: '/architecture/',
      },
      {
        text: 'API 参考',
        link: '/api/',
      },
      {
        text: 'SDK',
        children: [
          {
            text: 'C++ SDK',
            link: 'https://cuihairu.github.io/croupier-sdk-cpp/',
          },
          {
            text: 'Go SDK',
            link: 'https://cuihairu.github.io/croupier-sdk-go/',
          },
          {
            text: 'Java SDK',
            link: 'https://cuihairu.github.io/croupier-sdk-java/',
          },
          {
            text: 'JavaScript SDK',
            link: 'https://cuihairu.github.io/croupier-sdk-js/',
          },
          {
            text: 'Python SDK',
            link: 'https://cuihairu.github.io/croupier-sdk-python/',
          },
          {
            text: 'C# SDK',
            link: 'https://cuihairu.github.io/croupier-sdk-csharp/',
          },
          {
            text: 'Lua SDK',
            link: 'https://github.com/cuihairu/croupier-sdk-cpp/blob/main/skynet/service/croupier_service.lua',
          },
        ],
      },
      {
        text: '分析',
        link: '/analytics/',
      },
    ],

    // 侧边栏
    sidebar: {
      '/guide/': [
        {
          text: '入门指南',
          collapsable: false,
          children: [
            '/guide/README.md',
            '/guide/tutorial.md',
            '/guide/quick-start.md',
            '/guide/installation.md',
            '/guide/configuration.md',
            '/guide/deployment.md',
            '/guide/faq.md',
          ],
        },
        {
          text: '核心概念',
          collapsable: true,
          children: [
            '/guide/concepts/overview.md',
            '/guide/concepts/virtual-objects.md',
            '/guide/concepts/function-management.md',
            '/guide/concepts/permissions.md',
          ],
        },
        {
          text: '第三方集成',
          collapsable: true,
          children: [
            '/guide/integrations/third-party-platforms.md',
            '/guide/integrations/openapi-registration.md',
          ],
        },
        {
          text: '运维指南',
          collapsable: true,
          children: [
            '/guide/operations/monitoring.md',
            '/guide/operations/security.md',
            '/guide/operations/troubleshooting.md',
          ],
        },
      ],

      '/architecture/': [
        {
          text: '系统架构',
          children: [
            '/architecture/README.md',
            '/architecture/layers.md',
            '/architecture/data-flow.md',
          ],
        },
      ],

      '/api/': [
        {
          text: 'API 参考',
          collapsable: false,
          children: [
            '/api/README.md',
            '/api/grpc.md',
            '/api/rest.md',
          ],
        },
        {
          text: '运维管理',
          collapsable: true,
          children: [
            '/api/ops.md',
            '/api/ops_core.md',
            '/api/ops-simple.md',
            '/api/admin.md',
            '/api/backup.md',
            '/api/config.md',
            '/api/migrate.md',
            '/api/monitoring.md',
            '/api/node.md',
          ],
        },
        {
          text: 'Agent & 函数',
          collapsable: true,
          children: [
            '/api/agent.md',
            '/api/function.md',
            '/api/job.md',
            '/api/pack.md',
            '/api/schema.md',
          ],
        },
        {
          text: '认证与权限',
          collapsable: true,
          children: [
            '/api/auth.md',
            '/api/approval.md',
            '/api/audit.md',
            '/api/rate_limit.md',
          ],
        },
        {
          text: '游戏管理',
          collapsable: true,
          children: [
            '/api/game.md',
            '/api/player.md',
            '/api/entity.md',
            '/api/component.md',
            '/api/registry.md',
          ],
        },
        {
          text: '消息通知',
          collapsable: true,
          children: [
            '/api/message.md',
            '/api/alert.md',
            '/api/support.md',
            '/api/ticket.md',
            '/api/feedback.md',
          ],
        },
        {
          text: '平台与集成',
          collapsable: true,
          children: [
            '/api/platform.md',
            '/api/provider.md',
            '/api/certificate.md',
            '/api/storage.md',
          ],
        },
        {
          text: '数据分析',
          collapsable: true,
          children: [
            '/api/analytics.md',
            '/api/analytics_behavior.md',
            '/api/analytics_overview.md',
            '/api/analytics_payments.md',
            '/api/analytics_retention.md',
          ],
        },
        {
          text: '其他',
          collapsable: true,
          children: [
            '/api/assignment.md',
            '/api/faq.md',
            '/api/meta.md',
            '/api/profile.md',
            '/api/xrender.md',
          ],
        },
      ],

      '/sdk/cpp/': [
        '/sdk/cpp/README.md',
      ],

      '/analytics/': [
        {
          text: '分析系统',
          children: [
            '/analytics/README.md',
            '/analytics/quick-start.md',
          ],
        },
      ],

      // 兼容旧文档路径
      '/': [
        {
          text: '概览',
          collapsible: false,
          children: [
            '/README.md',
            '/directory-structure.md',
            '/deployment.md',
            '/config.md',
            '/security.md',
          ],
        },
        {
          text: '架构设计',
          collapsible: false,
          children: [
            '/ARCHITECTURE.md',
            '/VIRTUAL_OBJECT_DESIGN.md',
            '/VIRTUAL_OBJECT_QUICK_REFERENCE.md',
          ],
        },
        {
          text: '函数管理',
          collapsible: false,
          children: [
            '/FUNCTION_MANAGEMENT_README.md',
            '/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md',
            '/FUNCTION_MANAGEMENT_COMPARISON.md',
          ],
        },
        {
          text: 'SDK 文档',
          collapsible: false,
          children: [
            '/sdk-development.md',
            '/CPP_SDK_DOCS_INDEX.md',
            '/CPP_SDK_QUICK_REFERENCE.md',
          ],
        },
        {
          text: '生成器与协议',
          collapsible: false,
          children: [
            '/generator.md',
            '/PROTO_OPTIONS_GUIDE.md',
            '/api.md',
          ],
        },
        {
          text: '分析系统',
          collapsible: false,
          children: [
            '/analytics/README.md',
            '/analytics/quick-start.md',
          ],
        },
      ],
    },

    // 主题级配置
    themePlugins: {
      git: true,
      gitContributors: false,
      prismHighlighter: true,
      nprogress: true,
      backToTop: true,
    },
  }),

  plugins: [
    searchPlugin({
      locales: {
        '/': {
          placeholder: '搜索文档',
          hotKeys: ['k', '/'],
        },
      },
      hotSearchOnlyFocus: true,
      maxSuggestions: 10,
    }),
    markdownChartPlugin({
      mermaid: true,
    }),
    markdownMathPlugin({
      type: 'katex',
    }),
  ],

  // 开发工具配置
  onInitialized: (app) => {
    console.log(`VuePress app initialized at: ${app.options.base}`)
  },

  onWatched: (app, watchers, restart) => {
    console.log('VuePress is watching for file changes...')
  },
})
