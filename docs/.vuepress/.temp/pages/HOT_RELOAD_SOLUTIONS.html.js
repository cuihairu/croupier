import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/HOT_RELOAD_SOLUTIONS.html.vue"
const data = JSON.parse("{\"path\":\"/HOT_RELOAD_SOLUTIONS.html\",\"title\":\"🔥 游戏开发热更新方案总览\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1763159063000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"14a8ebc6f9de8e0e670629f75052837fbf8bb3e0\",\"time\":1763159063000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: add comprehensive hot reload documentation and examples\"}]},\"filePathRelative\":\"HOT_RELOAD_SOLUTIONS.md\"}")
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
