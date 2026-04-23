import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/ARCHITECTURE.html.vue"
const data = JSON.parse("{\"path\":\"/ARCHITECTURE.html\",\"title\":\"架构设计\",\"lang\":\"zh-CN\",\"frontmatter\":{\"title\":\"架构设计\",\"icon\":\"sitemap\",\"order\":1,\"category\":[\"架构设计\"],\"tag\":[\"架构\",\"对象驱动\"]},\"git\":{\"updatedTime\":1767536295000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":3,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"29ee0287ebb01e25ffb4d491d8292d82e874389c\",\"time\":1767536295000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: update documentation to VuePress style\"},{\"hash\":\"8d5bb6849c1d80a3c68c5de32f59f854cc324a13\",\"time\":1766533527000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"P0/P1: proto-first generator, manifest, jobs routing, TLS\"},{\"hash\":\"faee2a3c8d1b9048fc593232ff61b9029e670933\",\"time\":1763217465000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"chore: migrate from submodule to monorepo architecture\"}]},\"filePathRelative\":\"ARCHITECTURE.md\"}")
export { comp, data }

if (import.meta.webpackHot) {
  import.meta.webpackHot.accept()
  if (__VUE_HMR_RUNTIME__.updatePageData) {
    __VUE_HMR_RUNTIME__.updatePageData(data)
  }
}

if (import.meta.hot) {
  import.meta.hot.accept(({ data }) => {
    __VUE_HMR_RUNTIME__.updatePageData(data)
  })
}
