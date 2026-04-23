import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

const config = defineConfig({
  lang: 'zh-CN',
  title: 'Croupier Go SDK',
  description: '高性能 Go SDK，用于 Croupier 游戏函数注册与执行系统',
  head: [
    ['meta', { name: 'viewport', content: 'width=device-width,initial-scale=1' }],
    ['meta', { name: 'keywords', content: 'croupier,go,sdk,gRPC,游戏开发' }],
    ['meta', { name: 'theme-color', content: '#00ADD8' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:locale', content: 'zh-CN' }],
    ['meta', { property: 'og:title', content: 'Croupier Go SDK' }],
    ['meta', { property: 'og:site_name', content: 'Croupier Go SDK' }],
  ],
  base: '/croupier-sdk-go/',

  themeConfig: {
    logo: '/logo.png',

    nav: [
      { text: '指南', link: '/guide/' },
      { text: 'API 参考', link: '/api/' },
      { text: '示例', link: '/examples/' },
      { text: 'Croupier 主项目', link: 'https://cuihairu.github.io/croupier/' },
      {
        text: '其他 SDK',
        items: [
          { text: 'C++ SDK', link: 'https://cuihairu.github.io/croupier-sdk-cpp/' },
          { text: 'Java SDK', link: 'https://cuihairu.github.io/croupier-sdk-java/' },
          { text: 'JavaScript SDK', link: 'https://cuihairu.github.io/croupier-sdk-js/' },
          { text: 'Python SDK', link: 'https://cuihairu.github.io/croupier-sdk-python/' },
        ],
      },
    ],

    sidebar: {
      '/guide/': [
        {
          text: '入门指南',
          collapsed: false,
          items: [
            { text: '简介', link: '/guide/README.md' },
            { text: '安装', link: '/guide/installation.md' },
            { text: '快速开始', link: '/guide/quick-start.md' },
          ],
        },
        {
          text: '核心概念',
          items: [
            { text: '架构', link: '/guide/architecture.md' },
            { text: '函数描述符', link: '/guide/function-descriptor.md' },
            { text: '构建模式', link: '/guide/build-modes.md' },
          ],
        },
      ],

      '/api/': [
        {
          text: 'API 参考',
          collapsed: false,
          items: [
            { text: '概述', link: '/api/README.md' },
            { text: '客户端', link: '/api/client.md' },
            { text: '配置', link: '/api/config.md' },
          ],
        },
      ],

      '/examples/': [
        {
          text: '使用示例',
          collapsed: false,
          items: [
            { text: '概述', link: '/examples/README.md' },
            { text: '基础示例', link: '/examples/basic.md' },
            { text: '综合示例', link: '/examples/comprehensive.md' },
          ],
        },
      ],

      '/': [
        {
          text: '概览',
          collapsed: false,
          items: [
            { text: '简介', link: '/README.md' },
          ],
        },
        {
          text: '指南',
          collapsed: false,
          items: [
            { text: '简介', link: '/guide/README.md' },
            { text: '安装', link: '/guide/installation.md' },
          ],
        },
        {
          text: 'API',
          collapsed: false,
          items: [
            { text: '概述', link: '/api/README.md' },
          ],
        },
      ],
    },

    editLink: {
      pattern: 'https://github.com/cuihairu/croupier-sdk-go/edit/main/docs/:path',
      text: '在 GitHub 上编辑此页'
    },

    lastUpdated: {
      text: '最后更新'
    },

    social: [
      { icon: 'github', link: 'https://github.com/cuihairu/croupier-sdk-go' }
    ],

    search: {
      provider: 'local'
    },

    docFooter: {
      prev: '上一页',
      next: '下一页'
    }
  }
})

export default withMermaid(config)
