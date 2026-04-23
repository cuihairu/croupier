import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/CPP_SDK_QUICK_REFERENCE.html.vue"
const data = JSON.parse("{\"path\":\"/CPP_SDK_QUICK_REFERENCE.html\",\"title\":\"Croupier C++ SDK 快速参考\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1763084302000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"d015f85af868febe5fc90a28c7fbab7bd4691325\",\"time\":1763084302000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"feat: update all SDK submodules to proto-aligned versions and add comprehensive documentation\"}]},\"filePathRelative\":\"CPP_SDK_QUICK_REFERENCE.md\"}")
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
