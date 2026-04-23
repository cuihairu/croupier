import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/analytics/playbooks/card-ccg-cn.html.vue"
const data = JSON.parse("{\"path\":\"/analytics/playbooks/card-ccg-cn.html\",\"title\":\"卡牌（Card/CCG）数据采集与分析指引\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1762770891000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"cuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"a1d1ca920cf0e77376028821ac9fb81d7a1951dd\",\"time\":1762770891000,\"email\":\"cuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"feat(analytics): add standardized game types, events, metrics + CN docs; backend fields + admin UI hooks\\\\n\\\\nConfigs:\\\\n- configs/analytics/{events,metrics,game_types,taxonomy}.yaml\\\\n- metrics zh_name/zh_desc; playbooks for TD/Idle/Card/Board\\\\n\\\\nBackend:\\\\n- add Game.GameType &#x26; Game.GenreCode (domain/model/repo)\\\\n- extend /api/games to read/write game_type/genre_code\\\\n- export analytics spec to web/public via cmd/analytics-export + Makefile target\\\\n\\\\nWeb (submodule pointer):\\\\n- add GameTypeInfo/SelectCard/Tag, MetricsCatalogModal, dev demo pages\\\\n- web/public/analytics-spec.json for UI consumption\\\\n\\\\nDocs:\\\\n- WEB_DOCUMENTATION_GUIDE.md: frontend usage &#x26; admin integration\\\\n\"}]},\"filePathRelative\":\"analytics/playbooks/card-ccg-cn.md\"}")
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
