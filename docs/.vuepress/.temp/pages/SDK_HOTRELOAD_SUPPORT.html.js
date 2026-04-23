import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/SDK_HOTRELOAD_SUPPORT.html.vue"
const data = JSON.parse("{\"path\":\"/SDK_HOTRELOAD_SUPPORT.html\",\"title\":\"🔥 Croupier SDK 热更新方案支持策略\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1763159063000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"14a8ebc6f9de8e0e670629f75052837fbf8bb3e0\",\"time\":1763159063000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: add comprehensive hot reload documentation and examples\"}]},\"filePathRelative\":\"SDK_HOTRELOAD_SUPPORT.md\"}")
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
