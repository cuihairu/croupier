import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/analytics/game-metrics-overview.html.vue"
const data = JSON.parse("{\"path\":\"/analytics/game-metrics-overview.html\",\"title\":\"游戏指标全景图\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1763325744000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":2,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"0ac3783c0cafbb7e76dd3fb0484cad90c77386b8\",\"time\":1763325744000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(analytics): fix unresolved links; add missing pages; harden MDX; CI link check\\\\n\\\\nfeat(analytics): add public ingestion service (cmd/analytics-ingest), Makefile targets, Dockerfiles; compose service; fix worker image\\\\n\\\\nchore(docs): adjust config to ignore node_modules in docs plugin; remove duplicate index page\\\\n\\\\nrefactor(docs): escape MDX-sensitive chars and fence YAML blocks\"},{\"hash\":\"4a34fc6eff2c8bb89788947fb513d1f35cdb4db0\",\"time\":1762731549000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"feat: 完成游戏数据分析系统全面增强\"}]},\"filePathRelative\":\"analytics/game-metrics-overview.md\"}")
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
