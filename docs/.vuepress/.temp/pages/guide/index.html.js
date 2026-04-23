import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/guide/index.html.vue"
const data = JSON.parse("{\"path\":\"/guide/\",\"title\":\"首页\",\"lang\":\"zh-CN\",\"frontmatter\":{\"home\":true,\"title\":\"首页\",\"heroImage\":\"/logo.png\",\"heroText\":\"Croupier\",\"tagline\":\"分布式游戏管理系统\",\"actions\":[{\"text\":\"快速开始 →\",\"link\":\"/guide/quick-start.html\",\"type\":\"primary\"},{\"text\":\"项目介绍\",\"link\":\"/guide/concepts/overview.html\",\"type\":\"secondary\"}],\"features\":[{\"title\":\"🔐 零信任安全\",\"details\":\"gRPC+mTLS、细粒度 RBAC/ABAC、操作审批与审计日志，确保游戏运营安全。\"},{\"title\":\"🎮 函数注册控制\",\"details\":\"游戏服务器通过 Agent 注册函数，控制面统一调用、可视化进度与日志。\"},{\"title\":\"📊 Schema 驱动 UI\",\"details\":\"X-Render + JSON Schema 自动生成表单、风控提示、参数校验。\"},{\"title\":\"🔄 可观测性解耦\",\"details\":\"控制面与遥测面分离，支持实时事件处理与多维度监控。\"},{\"title\":\"📦 多语言 SDK\",\"details\":\"Go / C++ / Java / JS / Python 全覆盖，保持 Nightly 构建。\"},{\"title\":\"🚀 协议优先开发\",\"details\":\"所有 API 通过 Protocol Buffers 定义，使用 Buf 工具链管理。\"}],\"footer\":\"Apache-2.0 License | Copyright © 2024-present Croupier\"},\"git\":{\"updatedTime\":1767536295000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"29ee0287ebb01e25ffb4d491d8292d82e874389c\",\"time\":1767536295000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: update documentation to VuePress style\"}]},\"filePathRelative\":\"guide/README.md\"}")
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
