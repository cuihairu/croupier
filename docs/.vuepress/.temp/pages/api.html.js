import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/api.html.vue"
const data = JSON.parse("{\"path\":\"/api.html\",\"title\":\"Croupier API 文档\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1767626703000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":3,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"184f77f948407f9ca5d9088a4d764e5e800ece02\",\"time\":1767626703000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"feat: complete documentation-code alignment tasks\"},{\"hash\":\"2850102635c30770039d04ddc769f91e4129703e\",\"time\":1761962609000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: rename Core concept to Server across README and docs\"},{\"hash\":\"8e304eb04907065b7a86a2e28431a17d03e2e4e2\",\"time\":1761174667000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"feat(core,agent,ui): introduce gRPC+mTLS vNext skeleton with Core↔Agent↔SDK E2E, HTTP API, dynamic UI, jobs and RBAC\\\\n\\\\n- Protocol/IDL: add Function/Control proto (with dev stubs); buf configs\\\\n- Core: gRPC server, ControlService, FunctionService routing, HTTP API (/api/descriptors,/api/invoke,/api/start_job,/api/stream_job,/api/cancel_job)\\\\n- Agent: local FunctionService routing, LocalControlService for SDK registration, job executor with idempotency + cancel\\\\n- SDK: local handler hosting, schema validation, idempotency helper; example game server\\\\n- UI: static page with descriptor-driven form, invoke + start job + cancel, SSE progress\\\\n- Security: mTLS wiring, audit chain logger, basic RBAC (policy loader, HTTP enforcement)\\\\n- Tooling: Makefile, dev cert script, CI workflow, docs skeleton and descriptors\"}]},\"filePathRelative\":\"api.md\"}")
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
