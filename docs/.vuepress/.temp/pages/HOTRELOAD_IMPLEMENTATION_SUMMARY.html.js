import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/HOTRELOAD_IMPLEMENTATION_SUMMARY.html.vue"
const data = JSON.parse("{\"path\":\"/HOTRELOAD_IMPLEMENTATION_SUMMARY.html\",\"title\":\"🔥 Croupier 热更新系统总结\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1763159063000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"14a8ebc6f9de8e0e670629f75052837fbf8bb3e0\",\"time\":1763159063000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: add comprehensive hot reload documentation and examples\"}]},\"filePathRelative\":\"HOTRELOAD_IMPLEMENTATION_SUMMARY.md\"}")
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
