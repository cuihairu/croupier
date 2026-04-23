import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/sdks/cpp/index.html.vue"
const data = JSON.parse("{\"path\":\"/sdks/cpp/\",\"title\":\"C++ SDK\",\"lang\":\"zh-CN\",\"frontmatter\":{\"title\":\"C++ SDK\",\"icon\":\"file-code\",\"order\":1,\"category\":[\"SDK 文档\"],\"tag\":[\"C++\",\"SDK\"]},\"git\":{\"updatedTime\":1767536295000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":4,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"29ee0287ebb01e25ffb4d491d8292d82e874389c\",\"time\":1767536295000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: update documentation to VuePress style\"},{\"hash\":\"024de7ad8de9ccd17965bf20d20682e42dc80496\",\"time\":1766583769000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"chore: align toolchain, docs, and build\"},{\"hash\":\"0ac3783c0cafbb7e76dd3fb0484cad90c77386b8\",\"time\":1763325744000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(analytics): fix unresolved links; add missing pages; harden MDX; CI link check\\\\n\\\\nfeat(analytics): add public ingestion service (cmd/analytics-ingest), Makefile targets, Dockerfiles; compose service; fix worker image\\\\n\\\\nchore(docs): adjust config to ignore node_modules in docs plugin; remove duplicate index page\\\\n\\\\nrefactor(docs): escape MDX-sensitive chars and fence YAML blocks\"},{\"hash\":\"faee2a3c8d1b9048fc593232ff61b9029e670933\",\"time\":1763217465000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"chore: migrate from submodule to monorepo architecture\"}]},\"filePathRelative\":\"sdks/cpp/README.md\"}")
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
