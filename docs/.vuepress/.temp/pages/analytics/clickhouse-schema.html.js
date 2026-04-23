import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/analytics/clickhouse-schema.html.vue"
const data = JSON.parse("{\"path\":\"/analytics/clickhouse-schema.html\",\"title\":\"ClickHouse 表结构与物化聚合\",\"lang\":\"zh-CN\",\"frontmatter\":{\"title\":\"ClickHouse 表结构与物化聚合\"},\"git\":{\"updatedTime\":1763326585000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"4651abc2dc4ea79d9d7986013ce873f113d7383f\",\"time\":1763326585000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs(analytics): add ClickHouse schema + example queries; link from overview\\\\ndocs(metrics,ops): publish pages and escape MDX-sensitive tokens\\\\ndocs(sdk): add JS signature example\"}]},\"filePathRelative\":\"analytics/clickhouse-schema.md\"}")
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
