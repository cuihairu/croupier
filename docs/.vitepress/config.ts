import { defineConfig } from 'vitepress'

export default defineConfig({
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
  lastUpdated: true,

  theme: {
    logo: '/logo.png',

    nav: [
      { text: '指南', link: '/guide/' },
      { text: '架构', link: '/architecture/' },
      { text: 'API 参考', link: '/api/' },
      { text: '开发', link: '/development/' },
      {
        text: 'SDK',
        items: [
          { text: 'SDK 概览', link: '/sdks/' },
          { text: 'C++ SDK', link: '/sdks/cpp/' },
          { text: 'Go SDK', link: 'https://github.com/cuihairu/croupier/tree/main/sdks/go' },
          { text: 'Java SDK', link: 'https://github.com/cuihairu/croupier/tree/main/sdks/java' },
          { text: 'JavaScript SDK', link: 'https://github.com/cuihairu/croupier/tree/main/sdks/js' },
          { text: 'Python SDK', link: 'https://github.com/cuihairu/croupier/tree/main/sdks/python' },
          { text: 'C# SDK', link: 'https://github.com/cuihairu/croupier/tree/main/sdks/csharp' },
          { text: '功能矩阵', link: '/sdks/sdk-parity-matrix' },
        ],
      },
      { text: '分析', link: '/analytics/' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: '入门指南',
          collapsed: false,
          items: [
            { text: '概述', link: '/guide/' },
            { text: '快速开始', link: '/guide/quick-start' },
            { text: '安装', link: '/guide/installation' },
            { text: '配置', link: '/guide/configuration' },
            { text: '部署', link: '/guide/deployment' },
          ],
        },
        {
          text: '核心概念',
          collapsed: true,
          items: [
            { text: '概览', link: '/guide/concepts/overview' },
            { text: '虚拟对象', link: '/guide/concepts/virtual-objects' },
            { text: '函数管理', link: '/guide/concepts/function-management' },
            { text: '权限', link: '/guide/concepts/permissions' },
          ],
        },
        {
          text: '第三方集成',
          collapsed: true,
          items: [
            { text: '第三方平台', link: '/guide/integrations/third-party-platforms' },
            { text: 'OpenAPI 注册', link: '/guide/integrations/openapi-registration' },
          ],
        },
        {
          text: '运维指南',
          collapsed: true,
          items: [
            { text: '监控', link: '/guide/operations/monitoring' },
            { text: '安全', link: '/guide/operations/security' },
            { text: '故障排查', link: '/guide/operations/troubleshooting' },
          ],
        },
      ],

      '/architecture/': [
        {
          text: '系统架构',
          items: [
            { text: '概述', link: '/architecture/' },
            { text: '分层', link: '/architecture/layers' },
            { text: '数据流', link: '/architecture/data-flow' },
          ],
        },
      ],

      '/api/': [
        {
          text: 'API 参考',
          collapsed: false,
          items: [
            { text: '概述', link: '/api/' },
            { text: 'gRPC', link: '/api/grpc' },
            { text: 'REST', link: '/api/rest' },
          ],
        },
        {
          text: '运维管理',
          collapsed: true,
          items: [
            { text: '运维', link: '/api/ops' },
            { text: '运维核心', link: '/api/ops_core' },
            { text: '简化运维', link: '/api/ops-simple' },
            { text: '管理', link: '/api/admin' },
            { text: '备份', link: '/api/backup' },
            { text: '配置', link: '/api/config' },
            { text: '迁移', link: '/api/migrate' },
            { text: '监控', link: '/api/monitoring' },
            { text: '节点', link: '/api/node' },
          ],
        },
        {
          text: 'Agent & 函数',
          collapsed: true,
          items: [
            { text: 'Agent', link: '/api/agent' },
            { text: '函数', link: '/api/function' },
            { text: '作业', link: '/api/job' },
            { text: '包', link: '/api/pack' },
            { text: 'Schema', link: '/api/schema' },
          ],
        },
        {
          text: '认证与权限',
          collapsed: true,
          items: [
            { text: '认证', link: '/api/auth' },
            { text: '审批', link: '/api/approval' },
            { text: '审计', link: '/api/audit' },
            { text: '限流', link: '/api/rate_limit' },
          ],
        },
        {
          text: '游戏管理',
          collapsed: true,
          items: [
            { text: '游戏', link: '/api/game' },
            { text: '玩家', link: '/api/player' },
            { text: '实体', link: '/api/entity' },
            { text: '组件', link: '/api/component' },
            { text: '注册', link: '/api/registry' },
          ],
        },
        {
          text: '消息通知',
          collapsed: true,
          items: [
            { text: '消息', link: '/api/message' },
            { text: '告警', link: '/api/alert' },
            { text: '支持', link: '/api/support' },
            { text: '工单', link: '/api/ticket' },
            { text: '反馈', link: '/api/feedback' },
          ],
        },
        {
          text: '平台与集成',
          collapsed: true,
          items: [
            { text: '平台', link: '/api/platform' },
            { text: '提供商', link: '/api/provider' },
            { text: '证书', link: '/api/certificate' },
            { text: '存储', link: '/api/storage' },
          ],
        },
        {
          text: '数据分析',
          collapsed: true,
          items: [
            { text: '分析', link: '/api/analytics' },
            { text: '行为分析', link: '/api/analytics_behavior' },
            { text: '概览', link: '/api/analytics_overview' },
            { text: '支付分析', link: '/api/analytics_payments' },
            { text: '留存分析', link: '/api/analytics_retention' },
          ],
        },
        {
          text: '其他',
          collapsed: true,
          items: [
            { text: '任务', link: '/api/assignment' },
            { text: 'FAQ', link: '/api/faq' },
            { text: '元数据', link: '/api/meta' },
            { text: '配置文件', link: '/api/profile' },
            { text: '工作空间', link: '/api/workspace' },
          ],
        },
      ],

      '/sdk/cpp/': [
        { text: 'C++ SDK', link: '/sdk/cpp/' },
      ],

      '/analytics/': [
        {
          text: '分析系统',
          items: [
            { text: '概述', link: '/analytics/' },
            { text: '快速开始', link: '/analytics/quick-start' },
          ],
        },
      ],

      '/development/': [
        {
          text: '开发指南',
          collapsed: false,
          items: [
            { text: '概述', link: '/development/' },
            { text: '仓库指南', link: '/development/repository-guidelines' },
            { text: '仓库布局', link: '/development/repository-layout' },
          ],
        },
      ],

      '/': [
        {
          text: '概览',
          collapsed: false,
          items: [
            { text: '首页', link: '/' },
            { text: '指南', link: '/guide/' },
            { text: '架构', link: '/architecture/' },
            { text: 'API', link: '/api/' },
            { text: '分析', link: '/analytics/' },
            { text: '开发', link: '/development/' },
          ],
        },
      ],
    },

    editLink: {
      pattern: 'https://github.com/cuihairu/croupier/edit/main/docs/:path',
      text: '在 GitHub 上编辑此页',
    },

    lastUpdatedText: '最后更新',

    socialLinks: [
      { icon: 'github', link: 'https://github.com/cuihairu/croupier' },
    ],
  },

  markdown: {
    // Mermaid charts are supported natively in VitePress
    // Math KaTeX support - configure if needed
  },

  vite: {
    build: {
      chunkSizeWarningLimit: 1200,
    },
  },
})
