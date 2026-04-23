import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

const config = defineConfig({
  lang: 'zh-CN',
  title: 'Croupier Python SDK',
  description: 'Python SDK for Croupier',
  head: [
    ['meta', { name: 'viewport', content: 'width=device-width,initial-scale=1' }],
    ['meta', { name: 'keywords', content: 'croupier,python,sdk,gRPC' }],
    ['meta', { name: 'theme-color', content: '#3776AB' }],
  ],
  base: '/croupier-sdk-python/',

  themeConfig: {
    logo: '/logo.png',

    nav: [
      { text: '指南', link: '/guide/' },
      { text: 'API 参考', link: '/api/' },
      {
        text: '其他 SDK',
        items: [
          { text: 'C++', link: 'https://cuihairu.github.io/croupier-sdk-cpp/' },
          { text: 'Go', link: 'https://cuihairu.github.io/croupier-sdk-go/' },
          { text: 'Java', link: 'https://cuihairu.github.io/croupier-sdk-java/' },
          { text: 'JavaScript', link: 'https://cuihairu.github.io/croupier-sdk-js/' },
        ],
      },
      { text: 'Croupier 主项目', link: 'https://cuihairu.github.io/croupier/' },
    ],

    sidebar: {
      '/': [
        { text: '简介', link: '/README.md' },
        {
          text: '指南',
          items: [
            { text: '概述', link: '/guide/README.md' },
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
      pattern: 'https://github.com/cuihairu/croupier-sdk-python/edit/main/docs/:path',
      text: '在 GitHub 上编辑此页'
    },

    lastUpdated: {
      text: '最后更新'
    },

    social: [
      { icon: 'github', link: 'https://github.com/cuihairu/croupier-sdk-python' }
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
