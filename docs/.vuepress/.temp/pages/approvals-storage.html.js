import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/approvals-storage.html.vue"
const data = JSON.parse("{\"path\":\"/approvals-storage.html\",\"title\":\"Approvals 存储配置\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1765671134000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"c3894d255f976603b6d7964b3ca4250c06c7c565\",\"time\":1765671134000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"feat: 添加审批系统和相关功能\"}]},\"filePathRelative\":\"approvals-storage.md\"}")
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
