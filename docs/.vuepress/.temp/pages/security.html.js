import comp from "/Users/cui/Workspaces/croupier/server/docs/.vuepress/.temp/pages/security.html.vue"
const data = JSON.parse("{\"path\":\"/security.html\",\"title\":\"安全配置\",\"lang\":\"zh-CN\",\"frontmatter\":{\"title\":\"安全配置\",\"icon\":\"shield\",\"order\":6,\"category\":[\"入门指南\"],\"tag\":[\"安全\",\"权限\"]},\"git\":{\"updatedTime\":1767536295000,\"contributors\":[{\"name\":\"cuihairu\",\"username\":\"cuihairu\",\"email\":\"chuihairu@gmail.com\",\"commits\":5,\"url\":\"https://github.com/cuihairu\"}],\"changelog\":[{\"hash\":\"29ee0287ebb01e25ffb4d491d8292d82e874389c\",\"time\":1767536295000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: update documentation to VuePress style\"},{\"hash\":\"228147054e4f1a15cada6bb3a8af42c4128040fb\",\"time\":1762002341000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"chore: commit local changes\"},{\"hash\":\"d23b283e871df1362d50a3dabf5bd4e8d8d10ef1\",\"time\":1761968750000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"server(http): approvals persistence (Postgres via -tags pg, fallback memory) + pagination/filter/detail APIs; start_job transport encode; audit masking; pack extract; docs &#x26; schema.\"},{\"hash\":\"2850102635c30770039d04ddc769f91e4129703e\",\"time\":1761962609000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"docs: rename Core concept to Server across README and docs\"},{\"hash\":\"8e304eb04907065b7a86a2e28431a17d03e2e4e2\",\"time\":1761174667000,\"email\":\"chuihairu@gmail.com\",\"author\":\"cuihairu\",\"message\":\"feat(core,agent,ui): introduce gRPC+mTLS vNext skeleton with Core↔Agent↔SDK E2E, HTTP API, dynamic UI, jobs and RBAC\\\\n\\\\n- Protocol/IDL: add Function/Control proto (with dev stubs); buf configs\\\\n- Core: gRPC server, ControlService, FunctionService routing, HTTP API (/api/descriptors,/api/invoke,/api/start_job,/api/stream_job,/api/cancel_job)\\\\n- Agent: local FunctionService routing, LocalControlService for SDK registration, job executor with idempotency + cancel\\\\n- SDK: local handler hosting, schema validation, idempotency helper; example game server\\\\n- UI: static page with descriptor-driven form, invoke + start job + cancel, SSE progress\\\\n- Security: mTLS wiring, audit chain logger, basic RBAC (policy loader, HTTP enforcement)\\\\n- Tooling: Makefile, dev cert script, CI workflow, docs skeleton and descriptors\"}]},\"filePathRelative\":\"security.md\"}")
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
