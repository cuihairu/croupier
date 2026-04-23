import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

const config = defineConfig({
  lang: 'zh-CN',
  title: 'Croupier C++ SDK',
  description: '高性能 C++ SDK，用于 Croupier 游戏函数注册与虚拟对象管理',
  head: [
    ['meta', { name: 'viewport', content: 'width=device-width,initial-scale=1' }],
    ['meta', { name: 'keywords', content: 'croupier,cpp,sdk,gRPC,游戏开发,虚拟对象' }],
    ['meta', { name: 'theme-color', content: '#3eaf7c' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:locale', content: 'zh-CN' }],
    ['meta', { property: 'og:title', content: 'Croupier C++ SDK' }],
    ['meta', { property: 'og:site_name', content: 'Croupier C++ SDK' }],
  ],
  base: '/croupier-sdk-cpp/',

  themeConfig: {
    logo: '/logo.png',

    nav: [
      { text: '指南', link: '/guide/' },
      { text: 'API 参考', link: '/api/' },
      { text: '示例', link: '/examples/' },
      { text: '配置', link: '/configuration/' },
      { text: 'Croupier 主项目', link: 'https://cuihairu.github.io/croupier/' },
      {
        text: '其他 SDK',
        items: [
          { text: 'Go SDK', link: 'https://cuihairu.github.io/croupier-sdk-go/' },
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
            { text: '构建', link: '/guide/building.md' },
          ],
        },
        {
          text: '核心概念',
          items: [
            { text: '架构', link: '/guide/architecture.md' },
            { text: '虚拟对象', link: '/guide/virtual-objects.md' },
            { text: '函数', link: '/guide/functions.md' },
          ],
        },
        {
          text: '高级主题',
          items: [
            { text: '插件', link: '/guide/plugins.md' },
            { text: '部署', link: '/guide/deployment.md' },
            { text: '测试', link: '/guide/testing.md' },
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
            { text: '函数', link: '/api/functions.md' },
            { text: '虚拟对象', link: '/api/virtual-objects.md' },
          ],
        },
      ],

      '/examples/': [
        {
          text: '使用示例',
          collapsed: false,
          items: [
            { text: '概述', link: '/examples/README.md' },
            { text: '基础函数', link: '/examples/basic-function.md' },
            { text: '虚拟对象', link: '/examples/virtual-object.md' },
            { text: '插件', link: '/examples/plugin.md' },
            { text: '综合示例', link: '/examples/comprehensive.md' },
          ],
        },
      ],

      '/configuration/': [
        {
          text: '配置指南',
          collapsed: false,
          items: [
            { text: '概述', link: '/configuration/README.md' },
            { text: '客户端配置', link: '/configuration/client-config.md' },
            { text: '环境变量', link: '/configuration/environments.md' },
            { text: '安全', link: '/configuration/security.md' },
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
            { text: '快速开始', link: '/guide/quick-start.md' },
            { text: '构建', link: '/guide/building.md' },
          ],
        },
        {
          text: 'API 参考',
          collapsed: false,
          items: [
            { text: '概述', link: '/api/README.md' },
            { text: '客户端', link: '/api/client.md' },
          ],
        },
        {
          text: '示例',
          collapsed: false,
          items: [
            { text: '概述', link: '/examples/README.md' },
          ],
        },
        {
          text: '配置',
          collapsed: false,
          items: [
            { text: '概述', link: '/configuration/README.md' },
          ],
        },
      ],
    },

    editLink: {
      pattern: 'https://github.com/cuihairu/croupier-sdk-cpp/edit/main/docs/:path',
      text: '在 GitHub 上编辑此页'
    },

    lastUpdated: {
      text: '最后更新',
      formatOptions: {
        dateStyle: 'full',
        timeStyle: 'short'
      }
    },

    social: [
      { icon: 'github', link: 'https://github.com/cuihairu/croupier-sdk-cpp' }
    ],

    search: {
      provider: 'local',
      options: {
        locales: {
          zh: {
            translations: {
              button: {
                buttonText: '搜索文档',
                buttonAriaLabel: '搜索文档'
              },
              modal: {
                noResultsText: '无法找到相关结果',
                resetButtonTitle: '清除查询条件',
                footer: {
                  selectText: '选择',
                  navigateText: '切换'
                }
              }
            }
          }
        }
      }
    },

    outline: {
      label: '页面导航',
      level: [2, 3]
    },

    docFooter: {
      prev: '上一页',
      next: '下一页'
    },

    returnToTopLabel: '回到顶部'
  },

  vite: {
    build: {
      chunkSizeWarningLimit: 1200,
    },
  },
})

export default withMermaid(config)
