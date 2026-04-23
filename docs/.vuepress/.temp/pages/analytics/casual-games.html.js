import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/analytics/casual-games.html.vue"
const data = JSON.parse("{\"path\":\"/analytics/casual-games.html\",\"title\":\"休闲游戏指标\",\"lang\":\"zh-CN\",\"frontmatter\":{\"title\":\"休闲游戏指标\"},\"git\":{\"updatedTime\":1763325968000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":2,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"dc4340ed295c31ffe3f163d1cf26344cc33bff37\",\"time\":1763325968000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(analytics): flesh out current-system-analysis, enhancement plan, and per-genre pages\"},{\"hash\":\"0ac3783c0cafbb7e76dd3fb0484cad90c77386b8\",\"time\":1763325744000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(analytics): fix unresolved links; add missing pages; harden MDX; CI link check\\\\n\\\\nfeat(analytics): add public ingestion service (cmd/analytics-ingest), Makefile targets, Dockerfiles; compose service; fix worker image\\\\n\\\\nchore(docs): adjust config to ignore node_modules in docs plugin; remove duplicate index page\\\\n\\\\nrefactor(docs): escape MDX-sensitive chars and fence YAML blocks\"}]},\"filePathRelative\":\"analytics/casual-games.md\"}")
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
