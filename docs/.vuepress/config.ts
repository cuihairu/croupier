import { defineUserConfig } from 'vuepress'
import { defaultTheme } from '@vuepress/theme-default'

export default defineUserConfig({
  // 站点配置
  lang: 'zh-CN',
  title: 'Croupier',
  description: 'Croupier - 分布式游戏管理系统文档',

  // 基础路径配置
  base: '/croupier/',

  // 主题和其他配置
  theme: defaultTheme({
    // 导航栏
    navbar: [
      {
        text: '首页',
        link: '/'
      },
      {
        text: '架构',
        children: [
          '/ARCHITECTURE.md',
          '/directory-structure.md',
          '/VIRTUAL_OBJECT_DESIGN.md'
        ]
      },
      {
        text: 'SDK',
        children: [
          '/CPP_SDK_DOCS_INDEX.md',
          '/CPP_SDK_QUICK_REFERENCE.md',
          '/sdk-development.md'
        ]
      },
      {
        text: '配置',
        children: [
          '/config.md',
          '/deployment.md',
          '/security.md'
        ]
      },
      {
        text: 'API',
        children: [
          '/api.md',
          '/providers-manifest.md',
          '/FUNCTION_MANAGEMENT_README.md'
        ]
      },
      {
        text: 'GitHub',
        link: 'https://github.com/cuihairu/croupier'
      }
    ],

    // 侧边栏
    sidebar: {
      '/': [
        {
          text: '入门指南',
          children: [
            '/',
            '/ARCHITECTURE.md',
            '/directory-structure.md'
          ]
        },
        {
          text: '虚拟对象系统',
          children: [
            '/VIRTUAL_OBJECT_DESIGN.md',
            '/VIRTUAL_OBJECT_QUICK_REFERENCE.md'
          ]
        },
        {
          text: '函数管理',
          children: [
            '/FUNCTION_MANAGEMENT_README.md',
            '/FUNCTION_MANAGEMENT_QUICK_REFERENCE.md',
            '/FUNCTION_MANAGEMENT_ARCHITECTURE_ANALYSIS.md',
            '/FUNCTION_MANAGEMENT_EXECUTIVE_SUMMARY.md'
          ]
        },
        {
          text: 'C++ SDK',
          children: [
            '/CPP_SDK_DOCS_INDEX.md',
            '/CPP_SDK_QUICK_REFERENCE.md',
            '/CPP_SDK_ANALYSIS_SUMMARY.md',
            '/CPP_SDK_BUILD_OPTIMIZATION.md',
            '/VCPKG_OPTIMIZATION.md'
          ]
        },
        {
          text: '热重载',
          children: [
            '/HOT_RELOAD_SOLUTIONS.md',
            '/HOTRELOAD_BEST_PRACTICES.md',
            '/HOTRELOAD_IMPLEMENTATION_SUMMARY.md',
            '/SDK_HOTRELOAD_SUPPORT.md'
          ]
        },
        {
          text: '配置和部署',
          children: [
            '/config.md',
            '/deployment.md',
            '/security.md',
            '/wire-and-di.md'
          ]
        },
        {
          text: 'API 和接口',
          children: [
            '/api.md',
            '/providers-manifest.md',
            '/http-adapter.md',
            '/ui-and-views.md'
          ]
        },
        {
          text: '监控和运维',
          children: [
            '/metrics.md',
            '/tracing.md',
            '/control-capabilities.md'
          ]
        },
        {
          text: '游戏设计',
          children: [
            '/game-roles-design.md',
            '/complete-game-roles-design.md',
            '/assignments.md'
          ]
        },
        {
          text: '开发工具',
          children: [
            '/generator.md',
            '/sdk-development.md',
            '/e2e-example.md'
          ]
        },
        {
          text: '运维文档',
          children: [
            '/ops/',
          ]
        }
      ]
    },

    // 仓库配置
    repo: 'cuihairu/croupier',
    repoLabel: 'GitHub',
    editLink: true,
    editLinkText: '编辑此页',
    lastUpdated: true,
    lastUpdatedText: '上次更新',

    // 贡献者
    contributors: true,
    contributorsText: '贡献者',

    // 页面元信息
    tip: '提示',
    warning: '注意',
    danger: '警告'
  }),

  // Markdown 配置
  markdown: {
    code: {
      lineNumbers: true
    }
  },

  // 插件配置
  plugins: []
})