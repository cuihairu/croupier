import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/api/grpc.html.vue"
const data = JSON.parse("{\"path\":\"/api/grpc.html\",\"title\":\"gRPC API\",\"lang\":\"zh-CN\",\"frontmatter\":{\"title\":\"gRPC API\",\"icon\":\"network-wired\",\"order\":2,\"category\":[\"API 参考\"],\"tag\":[\"gRPC\",\"API\",\"Protobuf\"]},\"git\":{\"updatedTime\":1767536295000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"29ee0287ebb01e25ffb4d491d8292d82e874389c\",\"time\":1767536295000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: update documentation to VuePress style\"}]},\"filePathRelative\":\"api/grpc.md\"}")
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
