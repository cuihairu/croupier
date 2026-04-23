import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/sdk/specification.html.vue"
const data = JSON.parse("{\"path\":\"/sdk/specification.html\",\"title\":\"Croupier SDK 行为规范\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1767696273000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":3,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"986dda3b67466847e718c966feff17ff1e4fdbbd\",\"time\":1767696273000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: add file transfer security requirements to SDK specification\"},{\"hash\":\"51fb71d5f992f9d4ff830e08501a793f037ca546\",\"time\":1767694167000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: enhance SDK specification with retry and masking requirements\"},{\"hash\":\"22ef1eb81c2d700eb197d613b64e1d9160c5ac02\",\"time\":1767644581000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"chore: ignore generated binaries http and protoc-gen-croupier\"}]},\"filePathRelative\":\"sdk/specification.md\"}")
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
