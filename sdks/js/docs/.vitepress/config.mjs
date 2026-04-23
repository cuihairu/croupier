import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

const config = defineConfig({
  lang: 'zh-CN',
  title: 'Croupier JavaScript SDK',
  description: 'JavaScript/TypeScript SDK for Croupier',
  head: [
    ['meta', { name: 'viewport', content: 'width=device-width,initial-scale=1' }],
    ['meta', { name: 'keywords', content: 'croupier,javascript,typescript,sdk' }],
    ['meta', { name: 'theme-color', content: '#f7df1e' }],
  ],
  base: '/croupier-sdk-js/',

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
            { text: '概述', link: '/guide/README.md' },
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
      pattern: 'https://github.com/cuihairu/croupier-sdk-js/edit/main/docs/:path',
      text: '在 GitHub 上编辑此页'
    },

    lastUpdated: {
      text: '最后更新'
    },

    social: [
      { icon: 'github', link: 'https://github.com/cuihairu/croupier-sdk-js' }
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
