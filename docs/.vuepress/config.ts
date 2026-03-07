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
        text: '开发',
        link: '/development/',
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
            '/guide/',
            '/guide/tutorial',
            '/guide/quick-start',
            '/guide/installation',
            '/guide/configuration',
            '/guide/deployment',
            '/guide/faq',
          ],
        },
        {
          text: '核心概念',
          collapsable: true,
          children: [
            '/guide/concepts/overview',
            '/guide/concepts/virtual-objects',
            '/guide/concepts/function-management',
            '/guide/concepts/permissions',
          ],
        },
        {
          text: '第三方集成',
          collapsable: true,
          children: [
            '/guide/integrations/third-party-platforms',
            '/guide/integrations/openapi-registration',
          ],
        },
        {
          text: '运维指南',
          collapsable: true,
          children: [
            '/guide/operations/monitoring',
            '/guide/operations/security',
            '/guide/operations/troubleshooting',
            '/guide/operations/operation-guide',
          ],
        },
      ],

      '/architecture/': [
        {
          text: '系统架构',
          children: [
            '/architecture/',
            '/architecture/layers',
            '/architecture/data-flow',
          ],
        },
      ],

      '/api/': [
        {
          text: 'API 参考',
          collapsable: false,
          children: [
            '/api/',
            '/api/grpc',
            '/api/rest',
          ],
        },
        {
          text: '运维管理',
          collapsable: true,
          children: [
            '/api/ops',
            '/api/ops_core',
            '/api/ops-simple',
            '/api/admin',
            '/api/backup',
            '/api/config',
            '/api/migrate',
            '/api/monitoring',
            '/api/node',
          ],
        },
        {
          text: 'Agent & 函数',
          collapsable: true,
          children: [
            '/api/agent',
            '/api/function',
            '/api/job',
            '/api/pack',
            '/api/schema',
          ],
        },
        {
          text: '认证与权限',
          collapsable: true,
          children: [
            '/api/auth',
            '/api/approval',
            '/api/audit',
            '/api/rate_limit',
          ],
        },
        {
          text: '游戏管理',
          collapsable: true,
          children: [
            '/api/game',
            '/api/player',
            '/api/entity',
            '/api/component',
            '/api/registry',
          ],
        },
        {
          text: '消息通知',
          collapsable: true,
          children: [
            '/api/message',
            '/api/alert',
            '/api/support',
            '/api/ticket',
            '/api/feedback',
          ],
        },
        {
          text: '平台与集成',
          collapsable: true,
          children: [
            '/api/platform',
            '/api/provider',
            '/api/certificate',
            '/api/storage',
          ],
        },
        {
          text: '数据分析',
          collapsable: true,
          children: [
            '/api/analytics',
            '/api/analytics_behavior',
            '/api/analytics_overview',
            '/api/analytics_payments',
            '/api/analytics_retention',
          ],
        },
        {
          text: '其他',
          collapsable: true,
          children: [
            '/api/assignment',
            '/api/faq',
            '/api/meta',
            '/api/profile',
            '/api/xrender',
          ],
        },
      ],

      '/sdk/cpp/': [
        '/sdk/cpp/',
      ],

      '/analytics/': [
        {
          text: '分析系统',
          children: [
            '/analytics/',
            '/analytics/quick-start',
          ],
        },
      ],

      // 开发文档
      '/development/': [
        {
          text: '开发指南',
          collapsible: false,
          children: [
            '/development/',
            '/development/vscode-setup',
            '/development/troubleshooting',
            '/development/repository-guidelines',
          ],
        },
      ],

      // 根目录兼容
      '/': [
        {
          text: '概览',
          collapsible: false,
          children: [
            '/',
            '/directory-structure',
            '/deployment',
            '/config',
            '/security',
          ],
        },
        {
          text: '架构设计',
          collapsible: false,
          children: [
            '/ARCHITECTURE',
          ],
        },
        {
          text: '生成器与协议',
          collapsible: false,
          children: [
            '/PROTO_OPTIONS_GUIDE',
            '/api',
          ],
        },
        {
          text: '分析系统',
          collapsible: false,
          children: [
            '/analytics/',
            '/analytics/quick-start',
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
