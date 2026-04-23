import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/game-roles-design.html.vue"
const data = JSON.parse("{\"path\":\"/game-roles-design.html\",\"title\":\"游戏后台角色权限体系设计\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1762011989000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"9e9b1d50af7be12b54be13430bd67f7394f3ea3d\",\"time\":1762011989000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"feat: add comprehensive game team role-based access control system\"}]},\"filePathRelative\":\"game-roles-design.md\"}")
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
