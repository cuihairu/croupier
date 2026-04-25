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
      { text: '开发', link: '/development/' },
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
      { text: '分析', link: '/analytics/' },
    ],

    sidebar: {
      '/guide/': [
        { text: '入门指南', collapsed: false, items: [
          { text: '简介', link: '/guide/' },
          { text: '快速开始', link: '/guide/quick-start' },
          { text: '安装', link: '/guide/installation' },
          { text: '配置', link: '/guide/configuration' },
          { text: '部署', link: '/guide/deployment' },
        ]},
      ],
      '/architecture/': [
        { text: '系统架构', items: [
          { text: '概述', link: '/architecture/' },
          { text: '分层', link: '/architecture/layers' },
          { text: '数据流', link: '/architecture/data-flow' },
        ]},
      ],
      '/api/': [
        { text: 'API 参考', collapsed: false, items: [
          { text: '概述', link: '/api/' },
          { text: 'gRPC', link: '/api/grpc' },
          { text: 'REST', link: '/api/rest' },
        ]},
      ],
      '/development/': [
        { text: '开发指南', collapsed: false, items: [
          { text: '概述', link: '/development/' },
          { text: '仓库指南', link: '/development/repository-guidelines' },
          { text: '仓库布局', link: '/development/repository-layout' },
        ]},
      ],
      '/analytics/': [
        { text: '分析系统', items: [
          { text: '概述', link: '/analytics/' },
          { text: '快速开始', link: '/analytics/quick-start' },
        ]},
      ],
      '/sdks/': [
        { text: 'SDK', collapsed: false, items: [
          { text: '概览', link: '/sdks/' },
          { text: '能力矩阵', link: '/sdks/sdk-parity-matrix' },
          { text: 'C++ SDK', link: '/sdks/cpp/' },
          { text: 'C++ 指南', link: '/sdks/cpp/guide/' },
          { text: 'C++ API', link: '/sdks/cpp/api/' },
          { text: 'C++ 配置', link: '/sdks/cpp/configuration/' },
          { text: 'C++ 示例', link: '/sdks/cpp/examples/' },
          { text: 'Go SDK', link: '/sdks/go/' },
          { text: 'Go 指南', link: '/sdks/go/guide/' },
          { text: 'Go API', link: '/sdks/go/api/' },
          { text: 'Java SDK', link: '/sdks/java/' },
          { text: 'Java 指南', link: '/sdks/java/guide/' },
          { text: 'Java API', link: '/sdks/java/api/' },
          { text: 'JavaScript SDK', link: '/sdks/js/' },
          { text: 'JavaScript 指南', link: '/sdks/js/guide/' },
          { text: 'JavaScript API', link: '/sdks/js/api/' },
          { text: 'Python SDK', link: '/sdks/python/' },
          { text: 'Python 指南', link: '/sdks/python/guide/' },
          { text: 'Python API', link: '/sdks/python/api/' },
          { text: 'C# SDK', link: '/sdks/csharp/' },
          { text: 'C# 指南', link: '/sdks/csharp/guide/' },
          { text: 'C# API', link: '/sdks/csharp/api/' },
        ]},
      ],
      '/': [
        { text: '概览', collapsed: false, items: [
          { text: '简介', link: '/' },
          { text: '指南', link: '/guide/' },
          { text: '架构', link: '/architecture/' },
          { text: 'API', link: '/api/' },
          { text: '开发', link: '/development/' },
          { text: 'SDK', link: '/sdks/' },
        ]},
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
