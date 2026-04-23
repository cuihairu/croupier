import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/analytics/sdk-reference.html.vue"
const data = JSON.parse("{\"path\":\"/analytics/sdk-reference.html\",\"title\":\"SDK 参考\",\"lang\":\"zh-CN\",\"frontmatter\":{\"title\":\"SDK 参考\"},\"git\":{\"updatedTime\":1763326585000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":2,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"4651abc2dc4ea79d9d7986013ce873f113d7383f\",\"time\":1763326585000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(analytics): add ClickHouse schema + example queries; link from overview\\\\ndocs(metrics,ops): publish pages and escape MDX-sensitive tokens\\\\ndocs(sdk): add JS signature example\"},{\"hash\":\"0ac3783c0cafbb7e76dd3fb0484cad90c77386b8\",\"time\":1763325744000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(analytics): fix unresolved links; add missing pages; harden MDX; CI link check\\\\n\\\\nfeat(analytics): add public ingestion service (cmd/analytics-ingest), Makefile targets, Dockerfiles; compose service; fix worker image\\\\n\\\\nchore(docs): adjust config to ignore node_modules in docs plugin; remove duplicate index page\\\\n\\\\nrefactor(docs): escape MDX-sensitive chars and fence YAML blocks\"}]},\"filePathRelative\":\"analytics/sdk-reference.md\"}")
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
