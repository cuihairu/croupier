import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/campaign-implementation.html.vue"
const data = JSON.parse("{\"path\":\"/campaign-implementation.html\",\"title\":\"活动系统核心实现详解\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1767849468000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"b61be6f69bd51c0a4b0b580bb46da9c6cb9158c1\",\"time\":1767849468000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: add campaign system and C# SDK design documents\"}]},\"filePathRelative\":\"campaign-implementation.md\"}")
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
