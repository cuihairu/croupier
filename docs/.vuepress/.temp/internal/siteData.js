export const siteData = JSON.parse("{\"base\":\"/croupier/\",\"lang\":\"zh-CN\",\"title\":\"Croupier\",\"description\":\"分布式游戏管理系统 - 统一的游戏运营控制面\",\"head\":[[\"meta\",{\"name\":\"viewport\",\"content\":\"width=device-width,initial-scale=1\"}],[\"meta\",{\"name\":\"keywords\",\"content\":\"croupier,游戏管理,gm系统,分布式系统,gRPC\"}],[\"meta\",{\"name\":\"theme-color\",\"content\":\"#3eaf7c\"}],[\"meta\",{\"property\":\"og:type\",\"content\":\"website\"}],[\"meta\",{\"property\":\"og:locale\",\"content\":\"zh-CN\"}],[\"meta\",{\"property\":\"og:title\",\"content\":\"Croupier | 分布式游戏管理系统\"}],[\"meta\",{\"property\":\"og:site_name\",\"content\":\"Croupier\"}]],\"locales\":{}}")

if (import.meta.webpackHot) {
  import.meta.webpackHot.accept()
  if (__VUE_HMR_RUNTIME__.updateSiteData) {
    __VUE_HMR_RUNTIME__.updateSiteData(siteData)
  }
}

if (import.meta.hot) {
  import.meta.hot.accept(({ siteData }) => {
    __VUE_HMR_RUNTIME__.updateSiteData(siteData)
  })
}
