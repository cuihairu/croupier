import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/sdk/missing-features.html.vue"
const data = JSON.parse("{\"path\":\"/sdk/missing-features.html\",\"title\":\"Croupier SDK 缺失功能清单\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1767711801000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":4,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"0debd4408cd5d474d8d1e861781deddc09aca3fc\",\"time\":1767711801000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: update missing-features.md - retry mechanism completed\"},{\"hash\":\"a89f319b91b7df96190b625d26acbdeb4c82c488\",\"time\":1767694246000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: update C++ SDK status - token masking implemented\"},{\"hash\":\"2e92f1c9417f5896bc1f084d52701288217bcbdd\",\"time\":1767686716000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: update C++ SDK status - logging and retry features implemented\"},{\"hash\":\"22ef1eb81c2d700eb197d613b64e1d9160c5ac02\",\"time\":1767644581000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"chore: ignore generated binaries http and protoc-gen-croupier\"}]},\"filePathRelative\":\"sdk/missing-features.md\"}")
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
