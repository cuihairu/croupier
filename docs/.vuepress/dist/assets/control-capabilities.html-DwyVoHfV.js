import{_ as i,c as s,a as n,o as a}from"./app-C_dHcy8Q.js";const l={};function r(t,e){return a(),s("div",null,[...e[0]||(e[0]=[n(`<p>Control 能力注册（RegisterCapabilities）扩展草案</p><p>目标</p><ul><li>以不破坏现有 Register 的方式，新增一个用于能力清单（Manifest）上传的 RPC，便于 Provider 在启动时一次性上报 provider/functions/entities 的完整描述。</li></ul><p>方案</p><ol><li>新增 RPC（推荐）</li></ol><p>proto/croupier/control/v1/control.proto：</p><div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre><code class="language-text"><span class="line">message ProviderMeta {</span>
<span class="line">  string id = 1;</span>
<span class="line">  string version = 2;</span>
<span class="line">  string lang = 3;</span>
<span class="line">  string sdk = 4;</span>
<span class="line">}</span>
<span class="line"></span>
<span class="line">message RegisterCapabilitiesRequest {</span>
<span class="line">  ProviderMeta provider = 1;</span>
<span class="line">  bytes manifest_json_gz = 2; // gzip 压缩后的 manifest.json</span>
<span class="line">  // 预留：bytes fds = 10; // 可选，FileDescriptorSet（当使用 Proto FQN 映射时）</span>
<span class="line">}</span>
<span class="line"></span>
<span class="line">message RegisterCapabilitiesResponse {}</span>
<span class="line"></span>
<span class="line">service ControlService {</span>
<span class="line">  rpc RegisterCapabilities(RegisterCapabilitiesRequest) returns (RegisterCapabilitiesResponse);</span>
<span class="line">}</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><ol start="2"><li>向后兼容</li></ol><ul><li>现有 <code>Register</code>/<code>Heartbeat</code> 保持不变（仅 functions 列表 + agent 基本信息），旧版 Agent 不受影响。</li><li>新版 Provider SDK 调用 <code>RegisterCapabilities</code> 上报 Manifest；Server 解析 manifest，合并为 descriptors 并暴露 <code>/api/descriptors</code>。</li></ul><p>Server 端处理</p><ul><li>解压 <code>manifest_json_gz</code>，校验符合 <code>docs/providers-manifest.schema.json</code>。</li><li>存储/缓存 清单；合并多 Provider 的 functions/entities；生成统一的 Descriptors 给 HTTP/前端。</li><li>可记录 provider 版本/语言/SDK，以便兼容与灰度发布。</li></ul><p>注意</p><ul><li>Manifest 可能较大，建议 gzip；必要时可支持分段上传或对象存储托管（此处暂不做）。</li><li>后续可扩展 Provider 的增量/撤销（unregister）协议。</li></ul><p>实施步骤</p><ul><li>修改 proto，<code>buf generate</code> 生成代码（保持向后兼容）。</li><li>Server 增加 RegisterCapabilities 的 handler（不影响现有 Register 用途）。</li><li>增加单元测试：小/中/大 清单，含 JSON‑Schema 与 Proto FQN 映射的组合。</li></ul>`,15)])])}const o=i(l,[["render",r]]),p=JSON.parse('{"path":"/control-capabilities.html","title":"","lang":"zh-CN","frontmatter":{},"git":{"updatedTime":1762699203000,"contributors":[{"name":"cuihairu","username":"cuihairu","email":"chuihairu@gmail.com","commits":1,"url":"https://github.com/cuihairu"}],"changelog":[{"hash":"131875167feba215d595f088f163e7468dc20883","time":1762699203000,"email":"chuihairu@gmail.com","author":"cuihairu","message":"chore: sync"}]},"filePathRelative":"control-capabilities.md"}');export{o as comp,p as data};
