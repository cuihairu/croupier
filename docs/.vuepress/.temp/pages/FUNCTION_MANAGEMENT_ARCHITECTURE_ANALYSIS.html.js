import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/FUNCTION_MANAGEMENT_ARCHITECTURE_ANALYSIS.html.vue"
const data = JSON.parse("{\"path\":\"/FUNCTION_MANAGEMENT_ARCHITECTURE_ANALYSIS.html\",\"title\":\"Croupier 系统函数管理架构分析报告\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1763333725000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":2,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"8b0922f53f213665046dbb2bb952bfaf0ff479f5\",\"time\":1763333725000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"proto(ui,rbac): add common I18nText/Menu/PermissionSpec and wire into FunctionOptions + server FunctionDescriptor; add ListFunctionsSummary RPC (breaking change)\\\\n\\\\nsdk/cpp(windows): guard dlfcn include and use windows.h on _WIN32 in config_driven_loader.cpp to fix MSYS/MinGW build\\\\n\\\\ndocs: minor i18n/code fence update\"},{\"hash\":\"d015f85af868febe5fc90a28c7fbab7bd4691325\",\"time\":1763084302000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"feat: update all SDK submodules to proto-aligned versions and add comprehensive documentation\"}]},\"filePathRelative\":\"FUNCTION_MANAGEMENT_ARCHITECTURE_ANALYSIS.md\"}")
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
