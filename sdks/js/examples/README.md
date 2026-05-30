# 📡 Croupier Node.js SDK 文件传输示例

这个示例展示了如何使用Croupier Node.js SDK进行文件传输，为服务器端热重载提供基础支持。

## 🚀 快速开始

### 1. 安装依赖

```bash
cd examples/js-file-transfer
npm install
```

### 2. 启动Croupier Agent

```bash
# 在另一个终端启动Agent
cd ../../
make build
./bin/croupier-agent --config configs/agent.example.yaml
```

### 3. 运行示例

```bash
# 编译TypeScript
npm run build

# 运行示例
npm start

# 开发模式
npm run dev
```

## 📡 文件传输功能

### 基础文件上传

```javascript
// 计划中的文件上传 API
await client.uploadFile({
  filePath: "./functions/playerBan.js",
  content: fileContent,
  onProgress: (progress) => {
    console.log("validated", progress.filePath, progress.percent);
  },
  metadata: {
    version: "1.0.0",
    author: "game-team",
    description: "Player ban functionality",
  },
});
```

### 批量文件传输

```javascript
const files = [
  {
    filePath: "functions/playerBan.js",
    content: banCode,
    metadata: { version: "1.0.0" },
  },
  {
    filePath: "functions/walletTransfer.js",
    content: transferCode,
    metadata: { version: "1.0.0" },
  },
];

const batch = await client.uploadFiles(files);
console.log(batch.succeeded, batch.total);
console.log(batch.items);
```

### 流式文件上传

```javascript
// 计划中的流式上传大文件
const fs = require("fs");
const readStream = fs.createReadStream("./large-file.zip");

await client.uploadFileStream({
  filePath: "./assets/large-file.zip",
  stream: readStream,
  metadata: {
    size: String(fileStats.size),
    checksum: "sha256-hash",
  },
  onProgress: (progress) => {
    console.log(progress.loaded, progress.total, progress.percent);
  },
});
```

## 🛠️ 开发状态

当前SDK文件传输功能正在开发中：

- ✅ 接口定义完成
- ✅ TypeScript类型支持
- ✅ 客户端侧文件预检与元数据封装
- ✅ 流式内容预检支持
- ✅ 批量预检支持
- ✅ 上传进度回调
- ✅ 批量失败明细

## 🎯 功能演示

当前示例展示：

1. **基础架构**
   - TypeScript客户端配置
   - 接口定义展示
   - 错误处理示例

2. **文件处理**
   - 文件读取示例
   - 元数据处理
   - 基础文件操作

## 🔧 配置选项

### 客户端配置

```typescript
interface FileTransferConfig {
  agentAddr?: string;
  timeout?: number;
  retryAttempts?: number;
  chunkSize?: number; // 文件块大小（字节）
  maxFileSize?: number; // 最大文件大小（字节）
  compression?: boolean; // 启用压缩
  checksumVerification?: boolean; // 启用校验和验证
  parallelUploads?: number; // 并发上传数量
}
```

### 文件传输配置

```typescript
const config: FileTransferConfig = {
  agentAddr: "127.0.0.1:19090",
  timeout: 30000,
  retryAttempts: 3,
  chunkSize: 1024 * 1024, // 1MB chunks
  maxFileSize: 100 * 1024 * 1024, // 100MB max
  compression: true,
  checksumVerification: true,
  retryFailedUploads: true,
  parallelUploads: 4,
};
```

## 📊 示例函数处理器

### 玩家封禁处理器

```typescript
const playerBanHandler: FunctionHandler = async (
  context: string,
  payload: string,
): Promise<string> => {
  console.log(`🚫 Player ban requested: ${payload}`);

  try {
    const data = JSON.parse(payload);
    const playerId = data.player_id;
    const reason = data.reason || "No reason provided";

    // 模拟玩家封禁逻辑
    await new Promise((resolve) => setTimeout(resolve, 100));

    return JSON.stringify({
      status: "success",
      player_id: playerId,
      action: "banned",
      reason: reason,
      banned_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(), // 24小时后解封
    });
  } catch (error) {
    return JSON.stringify({
      status: "error",
      message: "Invalid payload format",
      error: error instanceof Error ? error.message : "Unknown error",
    });
  }
};
```

### 钱包转账处理器

```typescript
const walletTransferHandler: FunctionHandler = async (
  context: string,
  payload: string,
): Promise<string> => {
  console.log(`💰 Wallet transfer requested: ${payload}`);

  try {
    const data = JSON.parse(payload);
    const { from_player_id, to_player_id, amount, currency = "gold" } = data;

    // 模拟转账逻辑
    await new Promise((resolve) => setTimeout(resolve, 200));

    return JSON.stringify({
      status: "success",
      transaction_id: `tx_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
      from_player_id,
      to_player_id,
      amount: parseFloat(amount),
      currency,
      fee: parseFloat(amount) * 0.02, // 2% 手续费
      net_amount: parseFloat(amount) * 0.98,
      processed_at: new Date().toISOString(),
    });
  } catch (error) {
    return JSON.stringify({
      status: "error",
      message: "Transfer failed",
      error: error instanceof Error ? error.message : "Unknown error",
    });
  }
};
```

## 🚨 故障排除

### 常见问题

1. **连接问题**

   ```
   Error: connect ECONNREFUSED 127.0.0.1:19090
   ```

   - 确保Croupier Agent正在运行
   - 检查网络连接和端口配置
   - 验证防火墙设置

2. **文件权限问题**

   ```
   Error: EACCES: permission denied, open 'functions/test.js'
   ```

   - 检查文件路径权限
   - 确保有读写权限
   - 验证文件路径正确性

3. **TypeScript编译错误**
   ```
   error TS2304: Cannot find name 'FileTransferConfig'
   ```

   - 确保类型定义正确导入
   - 检查tsconfig.json配置
   - 重新编译：`npm run build`

### 最佳实践

1. **文件组织**
   - 将功能文件放在专门的目录
   - 使用版本控制管理代码
   - 保持文件结构清晰
   - 使用有意义的文件名

2. **错误处理**
   - 实现重试机制
   - 添加详细错误日志
   - 优雅处理网络错误
   - 验证文件完整性

3. **性能优化**
   - 使用适当的文件块大小
   - 实现并发上传
   - 监控传输进度
   - 启用压缩减少带宽使用

## 📚 相关文档

- [Croupier 主文档](https://docs.croupier.io)
- [gRPC API 参考](https://docs.croupier.io/api/grpc)
- [Node.js 最佳实践](https://nodejs.org/en/docs/guides/)
- [TypeScript 手册](https://www.typescriptlang.org/docs/)

---

_📡 为服务器热重载提供强大的文件传输支持！_
