import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/http-adapter.html.vue"
const data = JSON.parse("{\"path\":\"/http-adapter.html\",\"title\":\"HTTP Adapter (Generic)\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1762002341000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"228147054e4f1a15cada6bb3a8af42c4128040fb\",\"time\":1762002341000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"chore: commit local changes\"}]},\"filePathRelative\":\"http-adapter.md\"}")
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
