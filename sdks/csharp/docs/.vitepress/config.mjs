import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

const config = defineConfig({
  lang: 'zh-CN',
  title: 'Croupier C# SDK',
  description: 'Croupier SDK for .NET 8+',
  head: [
    ['meta', { name: 'viewport', content: 'width=device-width,initial-scale=1' }],
    ['meta', { name: 'keywords', content: 'croupier,csharp,.net,sdk,grpc' }],
    ['meta', { name: 'theme-color', content: '#3eaf7c' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:locale', content: 'zh-CN' }],
    ['meta', { property: 'og:title', content: 'Croupier C# SDK' }],
    ['meta', { property: 'og:site_name', content: 'Croupier' }],
  ],
  base: '/croupier-sdk-csharp/',

  themeConfig: {
    nav: [
      { text: '指南', link: '/guide/' },
      { text: 'API 参考', link: '/api/' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: '开始使用',
          collapsed: false,
          items: [
            { text: '简介', link: '/guide/README.md' },
            { text: '安装', link: '/guide/installation.md' },
            { text: '快速开始', link: '/guide/quick-start.md' },
            { text: '配置', link: '/guide/configuration.md' },
            { text: '依赖注入', link: '/guide/dependency-injection.md' },
          ],
        },
        {
          text: '高级用法',
          items: [
            { text: '异步处理器', link: '/guide/async-handlers.md' },
            { text: '错误处理', link: '/guide/error-handling.md' },
            { text: 'Unity 集成', link: '/guide/unity-integration.md' },
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
            { text: '调用器', link: '/api/invoker.md' },
            { text: '模型', link: '/api/models.md' },
          ],
        },
      ],

      '/': [
        {
          text: '指南',
          collapsed: false,
          items: [
            { text: '简介', link: '/guide/README.md' },
            { text: '快速开始', link: '/guide/quick-start.md' },
          ],
        },
        {
          text: 'API',
          items: [
            { text: '概述', link: '/api/README.md' },
          ],
        },
      ],
    },

    editLink: {
      pattern: 'https://github.com/cuihairu/croupier-sdk-csharp/edit/main/docs/:path',
      text: '在 GitHub 上编辑此页'
    },

    lastUpdated: {
      text: '最后更新'
    },

    social: [
      { icon: 'github', link: 'https://github.com/cuihairu/croupier-sdk-csharp' }
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
