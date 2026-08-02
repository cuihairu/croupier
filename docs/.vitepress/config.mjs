import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

const config = defineConfig({
  lang: 'zh-CN',
  title: 'Croupier',
  description: '分布式游戏管理系统 - 统一 session 架构的游戏运营控制面',
  srcExclude: ['archive/**'],
  head: [
    ['meta', { name: 'viewport', content: 'width=device-width,initial-scale=1' }],
    ['meta', { name: 'keywords', content: 'croupier,游戏管理,gm系统,分布式系统,session,agent,sdk' }],
    ['meta', { name: 'theme-color', content: '#3eaf7c' }],
  ],
  base: '/croupier/',

  themeConfig: {
    logo: '/logo.png',

    nav: [
      { text: '指南', link: '/guide/' },
      { text: '架构', link: '/architecture/' },
      { text: 'API 参考', link: '/api/' },
      { text: '数据分析', link: '/analytics/' },
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
      { text: '开发', link: '/development/' },
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
            { text: 'Page Studio', link: '/guide/concepts/function-registration-ui' },
            { text: '权限控制', link: '/guide/concepts/permissions' },
            { text: '资源与页面', link: '/architecture/dashboard-page-model' },
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
          text: '当前规范',
          collapsed: false,
          items: [
            { text: '概述', link: '/architecture/' },
            { text: '分层', link: '/architecture/layers' },
            { text: '术语', link: '/architecture/terms-and-layering' },
            { text: '数据流', link: '/architecture/data-flow' },
            { text: '游戏与环境作用域', link: '/architecture/game-environment-scope' },
            { text: 'Session 生命周期', link: '/architecture/session-lifecycle' },
            { text: 'SDK Wire 协议', link: '/architecture/sdk-wire-protocol' },
          ],
        },
        {
          text: 'Dashboard 页面模型',
          collapsed: false,
          items: [
            { text: 'Resource/Page 模型', link: '/architecture/dashboard-page-model' },
            { text: 'Descriptor v2 注册契约', link: '/architecture/openapi-sdk-descriptor-v2' },
            { text: 'UI Schema 与 PageSpec 规范', link: '/architecture/ui-schema-spec' },
            { text: '页面生成与运行时', link: '/architecture/ui-generation' },
            { text: 'Console 动态菜单', link: '/architecture/console-dynamic-menu' },
            { text: 'Dashboard 术语表', link: '/architecture/dashboard-glossary' },
          ],
        },
        {
          text: '决策与边界',
          collapsed: true,
          items: [
            { text: '传输层决策(不使用 gRPC)', link: '/architecture/transport-no-grpc' },
            { text: '扩展安装模型', link: '/architecture/extension-installation-model' },
            { text: '核心扩展映射', link: '/architecture/core-extension-mapping' },
            { text: '扩展 API 契约基线', link: '/architecture/extensions-api-contract-baseline' },
          ],
        },
        {
          text: '提案与迁移设计',
          collapsed: true,
          items: [
            { text: 'SDK-Agent 传输重构', link: '/architecture/sdk-agent-transport-redesign' },
            { text: 'Agent-Server Session', link: '/architecture/agent-server-session-transport-redesign' },
            { text: '扩展统一模式', link: '/architecture/official-extension-unified-pattern' },
          ],
        },
        {
          text: '参考资料',
          collapsed: true,
          items: [
            { text: 'Session 运行时调研', link: '/architecture/session-runtime-landscape' },
            { text: '前端 Adapter 模板', link: '/architecture/frontend-adapter-layer-template' },
          ],
        },
      ],

      '/api/': [
        {
          text: '基础',
          collapsed: false,
          items: [
            { text: '概述', link: '/api/' },
            { text: 'REST API', link: '/api/rest' },
            { text: '认证', link: '/api/auth' },
            { text: 'Schema', link: '/api/schema' },
            { text: '元数据', link: '/api/meta' },
          ],
        },
        {
          text: '核心业务',
          collapsed: false,
          items: [
            { text: '游戏', link: '/api/game' },
            { text: '玩家', link: '/api/player' },
            { text: '函数管理', link: '/api/function' },
            { text: '函数调用兼容视图', link: '/api/function_call' },
            { text: '任务', link: '/api/task' },
            { text: '消息', link: '/api/message' },
            { text: '配置', link: '/api/config' },
            { text: '审批', link: '/api/approval' },
            { text: '审计', link: '/api/audit' },
          ],
        },
        {
          text: '运维与平台',
          collapsed: true,
          items: [
            { text: '运维', link: '/api/ops' },
            { text: '运维核心', link: '/api/ops_core' },
            { text: '运维简化', link: '/api/ops-simple' },
            { text: 'Agent', link: '/api/agent' },
            { text: '节点', link: '/api/node' },
            { text: '注册中心', link: '/api/registry' },
            { text: '平台', link: '/api/platform' },
            { text: 'Provider', link: '/api/provider' },
            { text: '存储', link: '/api/storage' },
            { text: '备份', link: '/api/backup' },
            { text: '迁移', link: '/api/migrate' },
            { text: '监控', link: '/api/monitoring' },
            { text: '证书', link: '/api/certificate' },
            { text: '限流', link: '/api/rate_limit' },
            { text: '告警', link: '/api/alert' },
          ],
        },
        {
          text: '数据分析 API',
          collapsed: true,
          items: [
            { text: '分析 API', link: '/api/analytics' },
            { text: '分析概览', link: '/api/analytics_overview' },
            { text: '行为分析', link: '/api/analytics_behavior' },
            { text: '留存分析', link: '/api/analytics_retention' },
            { text: '支付分析', link: '/api/analytics_payments' },
          ],
        },
        {
          text: '控制台域',
          collapsed: true,
          items: [
            { text: '管理员', link: '/api/admin' },
            { text: '页面管理', link: '/api/page' },
            { text: '资源管理', link: '/api/resource' },
            { text: 'Profile', link: '/api/profile' },
          ],
        },
        {
          text: '运营支持',
          collapsed: true,
          items: [
            { text: '分配', link: '/api/assignment' },
            { text: '工单', link: '/api/ticket' },
            { text: '反馈', link: '/api/feedback' },
            { text: '客服', link: '/api/support' },
            { text: 'FAQ', link: '/api/faq' },
          ],
        },
      ],

      '/analytics/': [
        {
          text: '分析系统',
          collapsed: false,
          items: [
            { text: '概述', link: '/analytics/' },
            { text: '快速开始', link: '/analytics/quick-start' },
            { text: '指标全景图', link: '/analytics/game-metrics-overview' },
            { text: '指标词典', link: '/analytics/metrics-dictionary' },
          ],
        },
        {
          text: '采集与存储',
          collapsed: true,
          items: [
            { text: '数据采集架构', link: '/analytics/data-collection-architecture' },
            { text: 'OpenTelemetry 集成', link: '/analytics/opentelemetry-integration' },
            { text: 'ClickHouse 表结构', link: '/analytics/clickhouse-schema' },
            { text: 'API 参考', link: '/analytics/api-reference' },
            { text: 'SDK 参考', link: '/analytics/sdk-reference' },
          ],
        },
        {
          text: '游戏类型',
          collapsed: true,
          items: [
            { text: '游戏类型适配', link: '/analytics/game-type-adaptation' },
            { text: '休闲游戏', link: '/analytics/casual-games' },
            { text: '竞技游戏', link: '/analytics/competitive-games' },
            { text: 'RPG 游戏', link: '/analytics/rpg-games' },
            { text: '策略游戏', link: '/analytics/strategy-games' },
            { text: '棋牌桌游 Playbook', link: '/analytics/playbooks/board-table-cn' },
            { text: '卡牌 CCG Playbook', link: '/analytics/playbooks/card-ccg-cn' },
            { text: '放置游戏 Playbook', link: '/analytics/playbooks/idle-cn' },
            { text: '塔防游戏 Playbook', link: '/analytics/playbooks/tower-defense-cn' },
          ],
        },
        {
          text: '运维',
          collapsed: true,
          items: [
            { text: '最佳实践', link: '/analytics/best-practices' },
            { text: '故障排除', link: '/analytics/troubleshooting' },
            { text: '增强方案', link: '/analytics/enhancement-plan' },
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
            { text: '文档治理', link: '/development/documentation-governance' },
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
            { text: '资源与操作', link: '/sdks/cpp/guide/resources-and-operations' },
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
        { text: '数据分析', link: '/analytics/' },
        { text: '开发', link: '/development/' },
        { text: 'SDK', link: '/sdks/' },
        { text: '安全', link: '/security' },
      ],
    },

    editLink: {
      pattern: 'https://github.com/cuihairu/croupier/edit/main/docs/:path',
      text: '在 GitHub 上编辑此页'
    },

    lastUpdated: { text: '最后更新' },

    socialLinks: [
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
