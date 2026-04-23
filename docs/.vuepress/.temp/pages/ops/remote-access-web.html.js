import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/ops/remote-access-web.html.vue"
const data = JSON.parse("{\"path\":\"/ops/remote-access-web.html\",\"title\":\"远程访问（网页 RDP/SSH）方案设计\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1763326585000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":3,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"4651abc2dc4ea79d9d7986013ce873f113d7383f\",\"time\":1763326585000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(analytics): add ClickHouse schema + example queries; link from overview\\\\ndocs(metrics,ops): publish pages and escape MDX-sensitive tokens\\\\ndocs(sdk): add JS signature example\"},{\"hash\":\"0ac3783c0cafbb7e76dd3fb0484cad90c77386b8\",\"time\":1763325744000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(analytics): fix unresolved links; add missing pages; harden MDX; CI link check\\\\n\\\\nfeat(analytics): add public ingestion service (cmd/analytics-ingest), Makefile targets, Dockerfiles; compose service; fix worker image\\\\n\\\\nchore(docs): adjust config to ignore node_modules in docs plugin; remove duplicate index page\\\\n\\\\nrefactor(docs): escape MDX-sensitive chars and fence YAML blocks\"},{\"hash\":\"9183a71d141f253f6ac217641420d573f5ab8bda\",\"time\":1763187722000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(ops): add web-based RDP/SSH design (Guacamole/MeshCentral, audit, integration plan)\"}]},\"filePathRelative\":\"ops/remote-access-web.md\"}")
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
