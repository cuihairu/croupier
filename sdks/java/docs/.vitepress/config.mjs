import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

const config = defineConfig({
  lang: 'zh-CN',
  title: 'Croupier Java SDK',
  description: 'Java SDK for Croupier Game Backend Platform',
  head: [
    ['meta', { name: 'viewport', content: 'width=device-width,initial-scale=1' }],
    ['meta', { name: 'keywords', content: 'croupier,java,sdk,gRPC,游戏开发' }],
    ['meta', { name: 'theme-color', content: '#0074BD' }],
  ],
  base: '/croupier-sdk-java/',

  themeConfig: {
    logo: '/logo.png',

    nav: [
      { text: '指南', link: '/guide/' },
      { text: 'API 参考', link: '/api/' },
      { text: '示例', link: '/examples/' },
      {
        text: '其他 SDK',
        items: [
          { text: 'C++', link: 'https://cuihairu.github.io/croupier-sdk-cpp/' },
          { text: 'Go', link: 'https://cuihairu.github.io/croupier-sdk-go/' },
          { text: 'JavaScript', link: 'https://cuihairu.github.io/croupier-sdk-js/' },
          { text: 'Python', link: 'https://cuihairu.github.io/croupier-sdk-python/' },
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
      pattern: 'https://github.com/cuihairu/croupier-sdk-java/edit/main/docs/:path',
      text: '在 GitHub 上编辑此页'
    },

    lastUpdated: {
      text: '最后更新'
    },

    social: [
      { icon: 'github', link: 'https://github.com/cuihairu/croupier-sdk-java' }
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
