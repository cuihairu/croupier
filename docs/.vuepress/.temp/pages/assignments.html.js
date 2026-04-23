import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/assignments.html.vue"
const data = JSON.parse("{\"path\":\"/assignments.html\",\"title\":\"Assignments (Per Game/Env Function Sets)\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1767568282000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":2,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"5e3a2a3b50477178a32af9cb14fbb151489dbf9f\",\"time\":1767568282000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"chore: complete todo.md verification and mark all tasks as done\"},{\"hash\":\"514f1b167d7492e355d07c0f3c3fcbbf44319c2a\",\"time\":1761994660000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"feat(agent+server+gm): Adapter supervisor, downlink verify; registry health; assignments UX; HTTP tests; docs\"}]},\"filePathRelative\":\"assignments.md\"}")
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
