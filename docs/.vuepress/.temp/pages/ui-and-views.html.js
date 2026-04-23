import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/ui-and-views.html.vue"
const data = JSON.parse("{\"path\":\"/ui-and-views.html\",\"title\":\"UI Schema & Views (Web)\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1763325744000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":2,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"0ac3783c0cafbb7e76dd3fb0484cad90c77386b8\",\"time\":1763325744000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(analytics): fix unresolved links; add missing pages; harden MDX; CI link check\\\\n\\\\nfeat(analytics): add public ingestion service (cmd/analytics-ingest), Makefile targets, Dockerfiles; compose service; fix worker image\\\\n\\\\nchore(docs): adjust config to ignore node_modules in docs plugin; remove duplicate index page\\\\n\\\\nrefactor(docs): escape MDX-sensitive chars and fence YAML blocks\"},{\"hash\":\"228147054e4f1a15cada6bb3a8af42c4128040fb\",\"time\":1762002341000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"chore: commit local changes\"}]},\"filePathRelative\":\"ui-and-views.md\"}")
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
