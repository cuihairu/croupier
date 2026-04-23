import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/guide/tutorial.html.vue"
const data = JSON.parse("{\"path\":\"/guide/tutorial.html\",\"title\":\"新手教程\",\"lang\":\"zh-CN\",\"frontmatter\":{\"title\":\"新手教程\",\"icon\":\"graduation-cap\",\"order\":6,\"category\":[\"入门指南\"],\"tag\":[\"教程\",\"新手\",\"实战\"]},\"git\":{\"updatedTime\":1767568282000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"5e3a2a3b50477178a32af9cb14fbb151489dbf9f\",\"time\":1767568282000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"chore: complete todo.md verification and mark all tasks as done\"}]},\"filePathRelative\":\"guide/tutorial.md\"}")
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
