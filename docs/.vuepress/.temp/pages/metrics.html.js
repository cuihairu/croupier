import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/metrics.html.vue"
const data = JSON.parse("{\"path\":\"/metrics.html\",\"title\":\"Metrics & Observability\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1763326585000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":5,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"4651abc2dc4ea79d9d7986013ce873f113d7383f\",\"time\":1763326585000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(analytics): add ClickHouse schema + example queries; link from overview\\\\ndocs(metrics,ops): publish pages and escape MDX-sensitive tokens\\\\ndocs(sdk): add JS signature example\"},{\"hash\":\"0ac3783c0cafbb7e76dd3fb0484cad90c77386b8\",\"time\":1763325744000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(analytics): fix unresolved links; add missing pages; harden MDX; CI link check\\\\n\\\\nfeat(analytics): add public ingestion service (cmd/analytics-ingest), Makefile targets, Dockerfiles; compose service; fix worker image\\\\n\\\\nchore(docs): adjust config to ignore node_modules in docs plugin; remove duplicate index page\\\\n\\\\nrefactor(docs): escape MDX-sensitive chars and fence YAML blocks\"},{\"hash\":\"8644ff904a38aea858fa94f2bae89da4948f527b\",\"time\":1761967259000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"feat(packs): add type registry and transport-aware encode/decode (proto JSON&#x3C;->pb-bin) in HTTP; preload FDS from descriptors; add /api/packs/import and CLI 'croupier packs import'\"},{\"hash\":\"de61c861f1d3bf6f672de59a5a952b7e6f56e500\",\"time\":1761966883000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"feat(metrics): add per-function histograms with approx p50/p95/p99, toggles (--metrics.per_function, --metrics.per_game_denies); expose in JSON and when enabled in Prom text; wire toggles in CLI\"},{\"hash\":\"9f207cce9eb1f95a1378152046c1bc159d54c783\",\"time\":1761966594000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: add metrics.md (JSON+Prometheus endpoints and series) and config.md (includes/profiles precedence)\"}]},\"filePathRelative\":\"metrics.md\"}")
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
