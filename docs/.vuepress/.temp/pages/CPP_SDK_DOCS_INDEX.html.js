import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/CPP_SDK_DOCS_INDEX.html.vue"
const data = JSON.parse("{\"path\":\"/CPP_SDK_DOCS_INDEX.html\",\"title\":\"C++ SDK 文档索引\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1763386371000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":4,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"0a84b1a377b6995c9dcb1803bbdd55771989e804\",\"time\":1763386371000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"fix(cpp-sdk): qualify win32 dynamic loader calls\"},{\"hash\":\"c2e7bcaace99d2bd1bc87a0b6d2a7fc8d1d9db84\",\"time\":1763344542000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(docs): fix C++ SDK links to ../sdks/cpp to satisfy local link checker\"},{\"hash\":\"0ac3783c0cafbb7e76dd3fb0484cad90c77386b8\",\"time\":1763325744000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(analytics): fix unresolved links; add missing pages; harden MDX; CI link check\\\\n\\\\nfeat(analytics): add public ingestion service (cmd/analytics-ingest), Makefile targets, Dockerfiles; compose service; fix worker image\\\\n\\\\nchore(docs): adjust config to ignore node_modules in docs plugin; remove duplicate index page\\\\n\\\\nrefactor(docs): escape MDX-sensitive chars and fence YAML blocks\"},{\"hash\":\"faee2a3c8d1b9048fc593232ff61b9029e670933\",\"time\":1763217465000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"chore: migrate from submodule to monorepo architecture\"}]},\"filePathRelative\":\"CPP_SDK_DOCS_INDEX.md\"}")
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
