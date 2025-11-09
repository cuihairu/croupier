Control 能力注册（RegisterCapabilities）扩展草案

目标
- 以不破坏现有 Register 的方式，新增一个用于能力清单（Manifest）上传的 RPC，便于 Provider 在启动时一次性上报 provider/functions/entities 的完整描述。

方案
1) 新增 RPC（推荐）

proto/croupier/control/v1/control.proto：

```
message ProviderMeta {
  string id = 1;
  string version = 2;
  string lang = 3;
  string sdk = 4;
}

message RegisterCapabilitiesRequest {
  ProviderMeta provider = 1;
  bytes manifest_json_gz = 2; // gzip 压缩后的 manifest.json
  // 预留：bytes fds = 10; // 可选，FileDescriptorSet（当使用 Proto FQN 映射时）
}

message RegisterCapabilitiesResponse {}

service ControlService {
  rpc RegisterCapabilities(RegisterCapabilitiesRequest) returns (RegisterCapabilitiesResponse);
}
```

2) 向后兼容
- 现有 `Register`/`Heartbeat` 保持不变（仅 functions 列表 + agent 基本信息），旧版 Agent 不受影响。
- 新版 Provider SDK 调用 `RegisterCapabilities` 上报 Manifest；Server 解析 manifest，合并为 descriptors 并暴露 `/api/descriptors`。

Server 端处理
- 解压 `manifest_json_gz`，校验符合 `docs/providers-manifest.schema.json`。
- 存储/缓存 清单；合并多 Provider 的 functions/entities；生成统一的 Descriptors 给 HTTP/前端。
- 可记录 provider 版本/语言/SDK，以便兼容与灰度发布。

注意
- Manifest 可能较大，建议 gzip；必要时可支持分段上传或对象存储托管（此处暂不做）。
- 后续可扩展 Provider 的增量/撤销（unregister）协议。

实施步骤
- 修改 proto，`buf generate` 生成代码（保持向后兼容）。
- Server 增加 RegisterCapabilities 的 handler（不影响现有 Register 用途）。
- 增加单元测试：小/中/大 清单，含 JSON‑Schema 与 Proto FQN 映射的组合。

