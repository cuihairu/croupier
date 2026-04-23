import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/directory-structure.html.vue"
const data = JSON.parse("{\"path\":\"/directory-structure.html\",\"title\":\"目录结构\",\"lang\":\"zh-CN\",\"frontmatter\":{\"title\":\"目录结构\",\"icon\":\"folder-tree\",\"order\":2,\"category\":[\"入门指南\"],\"tag\":[\"项目结构\",\"go-zero\"]},\"git\":{\"updatedTime\":1767536295000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":3,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"29ee0287ebb01e25ffb4d491d8292d82e874389c\",\"time\":1767536295000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: update documentation to VuePress style\"},{\"hash\":\"e6266911e0ddcbfaa25d62258510439c8f483a0d\",\"time\":1764281186000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"feat(server): add migration tooling and profile center\"},{\"hash\":\"131875167feba215d595f088f163e7468dc20883\",\"time\":1762699203000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"chore: sync\"}]},\"filePathRelative\":\"directory-structure.md\"}")
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
