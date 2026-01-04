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
            link: '/sdk/cpp/',
          },
          {
            text: 'Go SDK',
            link: '/sdk/go/',
          },
          {
            text: 'Java SDK',
            link: '/sdk/java/',
          },
          {
            text: 'JavaScript SDK',
            link: '/sdk/js/',
          },
          {
            text: 'Python SDK',
            link: '/sdk/python/',
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
          children: [
            '/guide/README.md',
            '/guide/quick-start.md',
            '/guide/installation.md',
            '/guide/configuration.md',
            '/guide/deployment.md',
          ],
        },
        {
          text: '核心概念',
          children: [
            '/guide/concepts/overview.md',
            '/guide/concepts/virtual-objects.md',
            '/guide/concepts/function-management.md',
            '/guide/concepts/permissions.md',
          ],
        },
        {
          text: '运维指南',
          children: [
            '/guide/operations/monitoring.md',
            '/guide/operations/security.md',
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
          children: [
            '/api/README.md',
            '/api/grpc.md',
            '/api/rest.md',
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
