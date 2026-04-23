import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/wire-and-di.html.vue"
const data = JSON.parse("{\"path\":\"/wire-and-di.html\",\"title\":\"Wire DI & Providers\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1762699203000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"131875167feba215d595f088f163e7468dc20883\",\"time\":1762699203000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"chore: sync\"}]},\"filePathRelative\":\"wire-and-di.md\"}")
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
