import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

const config = defineConfig({
  lang: 'zh-CN',
  title: 'Croupier',
  description: '分布式游戏管理系统 - 统一的游戏运营控制面',
  head: [
    ['meta', { name: 'viewport', content: 'width=device-width,initial-scale=1' }],
    ['meta', { name: 'keywords', content: 'croupier,游戏管理,gm系统,分布式系统,gRPC' }],
    ['meta', { name: 'theme-color', content: '#3eaf7c' }],
  ],
  base: '/croupier/',

  themeConfig: {
    logo: '/logo.png',

    nav: [
      { text: '指南', link: '/guide/' },
      { text: '架构', link: '/architecture/' },
      { text: 'API 参考', link: '/api/' },
      {
        text: 'SDK',
        items: [
          { text: 'SDK 概览', link: '/sdks/' },
          { text: 'C++ SDK', link: '/sdks/cpp/' },
          { text: 'Go SDK', link: '/sdks/go/' },
          { text: 'Java SDK', link: '/sdks/java/' },
          { text: 'JavaScript SDK', link: '/sdks/js/' },
          { text: 'Python SDK', link: '/sdks/python/' },
          { text: 'C# SDK', link: '/sdks/csharp/' },
        ],
      },
    ],

    sidebar: {
      '/guide/': [
        {
          text: '入门',
          collapsed: false,
          items: [
            { text: '简介', link: '/guide/' },
            { text: '快速开始', link: '/guide/quick-start' },
            { text: '安装', link: '/guide/installation' },
            { text: '配置', link: '/guide/configuration' },
            { text: '部署', link: '/guide/deployment' },
          ],
        },
        {
          text: '核心概念',
          collapsed: false,
          items: [
            { text: '系统概述', link: '/guide/concepts/overview' },
            { text: '函数管理', link: '/guide/concepts/function-management' },
            { text: '权限控制', link: '/guide/concepts/permissions' },
            { text: '虚拟对象', link: '/guide/concepts/virtual-objects' },
          ],
        },
        {
          text: '运维',
          collapsed: true,
          items: [
            { text: '监控', link: '/guide/operations/monitoring' },
            { text: '安全', link: '/guide/operations/security' },
            { text: '故障排除', link: '/guide/operations/troubleshooting' },
          ],
        },
        {
          text: '集成',
          collapsed: true,
          items: [
            { text: 'OpenAPI 注册', link: '/guide/integrations/openapi-registration' },
            { text: '第三方平台', link: '/guide/integrations/third-party-platforms' },
          ],
        },
      ],

      '/architecture/': [
        {
          text: '架构文档',
          collapsed: false,
          items: [
            { text: '概述', link: '/architecture/' },
            { text: '分层', link: '/architecture/layers' },
            { text: '术语', link: '/architecture/terms-and-layering' },
            { text: '数据流', link: '/architecture/data-flow' },
          ],
        },
        {
          text: '传输协议',
          collapsed: true,
          items: [
            { text: 'SDK Wire 协议', link: '/architecture/sdk-wire-protocol' },
            { text: 'SDK-Agent 传输重构', link: '/architecture/sdk-agent-transport-redesign' },
            { text: 'Agent-Server Session', link: '/architecture/agent-server-session-transport-redesign' },
            { text: 'Session 生命周期', link: '/architecture/session-lifecycle' },
            { text: 'Session 运行时', link: '/architecture/session-runtime-landscape' },
          ],
        },
        {
          text: '扩展系统',
          collapsed: true,
          items: [
            { text: '扩展安装模型', link: '/architecture/extension-installation-model' },
            { text: '核心扩展映射', link: '/architecture/core-extension-mapping' },
            { text: '扩展统一模式', link: '/architecture/official-extension-unified-pattern' },
          ],
        },
      ],

      '/api/': [
        {
          text: 'REST API',
          collapsed: false,
          items: [
            { text: '概述', link: '/api/' },
            { text: '认证', link: '/api/auth' },
            { text: '用户管理', link: '/api/admin' },
          ],
        },
        {
          text: '函数 API',
          collapsed: false,
          items: [
            { text: '函数管理', link: '/api/function' },
            { text: '审批', link: '/api/approval' },
            { text: '审计', link: '/api/audit' },
          ],
        },
        {
          text: '游戏管理',
          collapsed: true,
          items: [
            { text: '游戏', link: '/api/game' },
            { text: '玩家', link: '/api/player' },
            { text: '消息', link: '/api/message' },
          ],
        },
        {
          text: '运营工具',
          collapsed: true,
          items: [
            { text: '工单', link: '/api/ticket' },
            { text: '反馈', link: '/api/feedback' },
            { text: '公告', link: '/api/assignment' },
          ],
        },
        {
          text: '系统 API',
          collapsed: true,
          items: [
            { text: 'Agent', link: '/api/agent' },
            { text: '节点', link: '/api/node' },
            { text: '注册中心', link: '/api/registry' },
            { text: '配置', link: '/api/config' },
            { text: '监控', link: '/api/monitoring' },
            { text: '证书', link: '/api/certificate' },
            { text: '备份', link: '/api/backup' },
            { text: '迁移', link: '/api/migrate' },
            { text: '存储', link: '/api/storage' },
          ],
        },
        {
          text: '其他 API',
          collapsed: true,
          items: [
            { text: '平台', link: '/api/platform' },
            { text: '工作空间', link: '/api/workspace' },
            { text: '服务商', link: '/api/provider' },
            { text: '实体', link: '/api/entity' },
            { text: '限流', link: '/api/rate_limit' },
            { text: '告警', link: '/api/alert' },
            { text: '任务', link: '/api/task' },
            { text: '客服', link: '/api/support' },
            { text: 'FAQ', link: '/api/faq' },
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
            { text: '发布约定', link: '/development/release-conventions' },
            { text: '业务扩展策略', link: '/development/new-business-extension-policy' },
          ],
        },
      ],

      '/sdks/': [
        { text: 'SDK 概览', link: '/sdks/' },
        { text: '能力矩阵', link: '/sdks/sdk-parity-matrix' },
        {
          text: '语言文档',
          collapsed: false,
          items: [
            { text: 'C++', link: '/sdks/cpp/' },
            { text: 'Go', link: '/sdks/go/' },
            { text: 'Java', link: '/sdks/java/' },
            { text: 'JavaScript', link: '/sdks/js/' },
            { text: 'Python', link: '/sdks/python/' },
            { text: 'C#', link: '/sdks/csharp/' },
          ],
        },
      ],

      '/sdks/cpp/': [
        { text: '概述', link: '/sdks/cpp/' },
        {
          text: '指南',
          collapsed: true,
          items: [
            { text: '安装', link: '/sdks/cpp/guide/installation' },
            { text: '构建', link: '/sdks/cpp/guide/building' },
            { text: '快速开始', link: '/sdks/cpp/guide/quick-start' },
            { text: '函数', link: '/sdks/cpp/guide/functions' },
            { text: '虚拟对象', link: '/sdks/cpp/guide/virtual-objects' },
            { text: '插件', link: '/sdks/cpp/guide/plugins' },
            { text: '配置', link: '/sdks/cpp/configuration/' },
          ],
        },
        {
          text: '进阶',
          collapsed: true,
          items: [
            { text: '架构', link: '/sdks/cpp/guide/architecture' },
            { text: '线程', link: '/sdks/cpp/guide/threading' },
            { text: '测试', link: '/sdks/cpp/guide/testing' },
            { text: '部署', link: '/sdks/cpp/guide/deployment' },
          ],
        },
        {
          text: '参考',
          collapsed: true,
          items: [
            { text: '约定', link: '/sdks/cpp/conventions' },
            { text: '集成', link: '/sdks/cpp/integration' },
            { text: 'API', link: '/sdks/cpp/api/' },
            { text: '示例', link: '/sdks/cpp/examples/' },
          ],
        },
      ],

      '/sdks/go/': [
        { text: '概述', link: '/sdks/go/' },
        { text: '指南', link: '/sdks/go/guide/' },
        { text: 'API', link: '/sdks/go/api/' },
        { text: '示例', link: '/sdks/go/examples/' },
        { text: '端到端流程', link: '/sdks/go/e2e-flow' },
      ],

      '/sdks/java/': [
        { text: '概述', link: '/sdks/java/' },
        { text: '指南', link: '/sdks/java/guide/' },
        { text: 'API', link: '/sdks/java/api/' },
      ],

      '/sdks/js/': [
        { text: '概述', link: '/sdks/js/' },
        { text: '指南', link: '/sdks/js/guide/' },
        { text: 'API', link: '/sdks/js/api/' },
      ],

      '/sdks/python/': [
        { text: '概述', link: '/sdks/python/' },
        { text: '指南', link: '/sdks/python/guide/' },
        { text: 'API', link: '/sdks/python/api/' },
      ],

      '/sdks/csharp/': [
        { text: '概述', link: '/sdks/csharp/' },
        { text: '指南', link: '/sdks/csharp/guide/' },
        { text: 'API', link: '/sdks/csharp/api/' },
      ],

      '/': [
        { text: '指南', link: '/guide/' },
        { text: '架构', link: '/architecture/' },
        { text: 'API 参考', link: '/api/' },
        { text: '开发', link: '/development/' },
        { text: 'SDK', link: '/sdks/' },
      ],
    },

    editLink: {
      pattern: 'https://github.com/cuihairu/croupier/edit/main/docs/:path',
      text: '在 GitHub 上编辑此页'
    },

    lastUpdated: { text: '最后更新' },

    social: [
      { icon: 'github', link: 'https://github.com/cuihairu/croupier' }
    ],

    search: { provider: 'local' },

    docFooter: {
      prev: '上一页',
      next: '下一页'
    }
  },

  vite: {
    build: {
      chunkSizeWarningLimit: 1200,
    },
  },
})

export default withMermaid(config)
