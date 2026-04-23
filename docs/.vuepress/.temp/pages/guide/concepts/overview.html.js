import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/guide/concepts/overview.html.vue"
const data = JSON.parse("{\"path\":\"/guide/concepts/overview.html\",\"title\":\"系统概览\",\"lang\":\"zh-CN\",\"frontmatter\":{\"title\":\"系统概览\",\"icon\":\"circle-info\",\"order\":1,\"category\":[\"核心概念\"],\"tag\":[\"架构\",\"概述\"]},\"git\":{\"updatedTime\":1767536295000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"29ee0287ebb01e25ffb4d491d8292d82e874389c\",\"time\":1767536295000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: update documentation to VuePress style\"}]},\"filePathRelative\":\"guide/concepts/overview.md\"}")
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
