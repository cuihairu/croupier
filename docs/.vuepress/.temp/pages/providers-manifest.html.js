import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/providers-manifest.html.vue"
const data = JSON.parse("{\"path\":\"/providers-manifest.html\",\"title\":\"\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1766533527000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":2,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"8d5bb6849c1d80a3c68c5de32f59f854cc324a13\",\"time\":1766533527000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"P0/P1: proto-first generator, manifest, jobs routing, TLS\"},{\"hash\":\"131875167feba215d595f088f163e7468dc20883\",\"time\":1762699203000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"chore: sync\"}]},\"filePathRelative\":\"providers-manifest.md\"}")
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
