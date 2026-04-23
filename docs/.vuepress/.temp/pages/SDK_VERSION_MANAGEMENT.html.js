import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/SDK_VERSION_MANAGEMENT.html.vue"
const data = JSON.parse("{\"path\":\"/SDK_VERSION_MANAGEMENT.html\",\"title\":\"SDK 版本管理\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1763646401000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"9a3777ef43ec80d04e01c09371a31b389686f8a2\",\"time\":1763646401000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"fix(sdk): fix nightly release build and unify SDK versions\"}]},\"filePathRelative\":\"SDK_VERSION_MANAGEMENT.md\"}")
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
