import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/sdk-development.html.vue"
const data = JSON.parse("{\"path\":\"/sdk-development.html\",\"title\":\"SDK Development\",\"lang\":\"zh-CN\",\"frontmatter\":{},\"git\":{\"updatedTime\":1761174667000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":1,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"8e304eb04907065b7a86a2e28431a17d03e2e4e2\",\"time\":1761174667000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"feat(core,agent,ui): introduce gRPC+mTLS vNext skeleton with Core↔Agent↔SDK E2E, HTTP API, dynamic UI, jobs and RBAC\\\\n\\\\n- Protocol/IDL: add Function/Control proto (with dev stubs); buf configs\\\\n- Core: gRPC server, ControlService, FunctionService routing, HTTP API (/api/descriptors,/api/invoke,/api/start_job,/api/stream_job,/api/cancel_job)\\\\n- Agent: local FunctionService routing, LocalControlService for SDK registration, job executor with idempotency + cancel\\\\n- SDK: local handler hosting, schema validation, idempotency helper; example game server\\\\n- UI: static page with descriptor-driven form, invoke + start job + cancel, SSE progress\\\\n- Security: mTLS wiring, audit chain logger, basic RBAC (policy loader, HTTP enforcement)\\\\n- Tooling: Makefile, dev cert script, CI workflow, docs skeleton and descriptors\"}]},\"filePathRelative\":\"sdk-development.md\"}")
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
