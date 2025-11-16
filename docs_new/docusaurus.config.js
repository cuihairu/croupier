// @ts-check
// Note: type annotations allow type checking and IDEs autocompletion

const config = {
  title: 'Croupier',
  tagline: '分布式游戏管理系统文档',
  url: 'https://cuihairu.github.io',
  baseUrl: '/croupier/',
  onBrokenLinks: 'throw',
  favicon: 'img/favicon.ico',
  i18n: {
    defaultLocale: 'zh-Hans',
    locales: ['zh-Hans'],
  },
  presets: [
    [
      '@docusaurus/preset-classic',
      {
        docs: {
          path: './docs',
          routeBasePath: '/',
          sidebarPath: './sidebars.js',
          editUrl: 'https://github.com/cuihairu/croupier/edit/main/docs/',
          showLastUpdateAuthor: true,
          showLastUpdateTime: true,
          exclude: [
            '**/analytics/**',
            '**/ops/**',
            '**/README.md',
          ],
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      },
    ],
  ],
  themeConfig: {
    navbar: {
      title: 'Croupier',
      items: [
        {
          href: 'https://github.com/cuihairu/croupier',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      copyright: `Copyright © ${new Date().getFullYear()} Croupier`,
    },
  },
  plugins: [
    [
      '@docusaurus/plugin-content-docs',
      {
        id: 'analytics',
        path: './docs/analytics',
        routeBasePath: '/analytics',
        sidebarPath: './sidebars.js',
        exclude: ['**/README.md'],
      },
    ],
    [
      '@docusaurus/plugin-content-docs',
      {
        id: 'ops',
        path: './docs/ops',
        routeBasePath: '/ops',
        sidebarPath: './sidebars.js',
        exclude: ['**/README.md'],
      },
    ],
  ],
};

module.exports = config;