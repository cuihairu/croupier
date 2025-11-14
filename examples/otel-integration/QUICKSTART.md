# 🚀 快速开始指南

这个指南将帮助您在5分钟内启动并运行 OpenTelemetry 集成示例。

## 📋 前置要求

确保您的系统安装了以下软件：

- **Go 1.24+**: [下载安装](https://golang.org/dl/)
- **Docker**: [下载安装](https://docs.docker.com/get-docker/)
- **Docker Compose**: [下载安装](https://docs.docker.com/compose/install/)
- **curl**: 大多数系统自带

### 快速检查
```bash
go version    # 应该显示 go1.24 或更高版本
docker --version
docker-compose --version
curl --version
```

## 🏃‍♂️ 一键启动

```bash
# 1. 进入示例目录
cd examples/otel-integration

# 2. 验证环境
make verify

# 3. 启动完整环境（这将需要几分钟下载Docker镜像）
make start
```

就这么简单！脚本会自动：
- ✅ 构建所有Go应用程序
- ✅ 启动 OpenTelemetry 观测性栈
- ✅ 启动游戏服务器和模拟器
- ✅ 开始生成演示数据

## 🌐 访问服务

启动完成后，访问以下地址：

| 服务 | 地址 | 用途 |
|------|------|------|
| **Grafana** | http://localhost:3000 | 可视化仪表板（admin/admin）|
| **Jaeger** | http://localhost:16686 | 分布式追踪界面 |
| **Prometheus** | http://localhost:9090 | 指标查询界面 |
| **游戏服务器** | http://localhost:8080 | API端点 |
| **健康检查** | http://localhost:8080/health | 服务状态 |

## 🎭 运行演示

```bash
# 运行交互式演示
make demo
```

这将带您了解：
- 🎮 基础游戏功能
- 📊 指标收集和查询
- 🔍 分布式追踪
- 🚨 告警功能
- ⚡ 性能分析

## 🔥 负载测试

```bash
# 轻量负载测试（10用户，60秒）
make load-test

# 重负载测试（50用户，5分钟）
make load-test-heavy
```

## 🔬 客户端分析指标测试

测试完整的客户端性能和行为指标收集：

```bash
# 基础客户端分析测试（3个客户端，60秒）
make test-client-analytics

# 扩展客户端分析测试（10个客户端，5分钟）
make test-client-analytics-extended
```

客户端分析测试包括：
- **性能指标**: FPS、内存、CPU、电池、温度
- **网络指标**: 延迟、抖动、丢包、带宽、重连
- **稳定性指标**: 崩溃、ANR、卡顿、内存不足
- **用户体验**: 触控精度、输入延迟、手势识别、UI响应
- **加载性能**: 启动时间、关卡加载、资源下载
- **渲染性能**: 帧时间、渲染调用、几何复杂度

## 📊 查看数据

### 1. Grafana 仪表板
访问 http://localhost:3000，使用 `admin/admin` 登录，查看：
- 游戏业务指标
- 系统性能指标
- 用户行为分析

### 2. Jaeger 追踪
访问 http://localhost:16686，查看：
- 完整的请求链路
- 性能瓶颈分析
- 错误追踪

### 3. Prometheus 指标
访问 http://localhost:9090，查询：
- `game_session_total`: 总会话数
- `game_level_start_total`: 关卡开始总数
- `game_request_duration_bucket`: 请求延迟分布

## 🛠️ 常用命令

```bash
# 查看服务状态
make status

# 查看服务日志
make logs

# 停止所有服务
make stop

# 重启服务
make restart

# 清理所有资源
make clean-all
```

## 🐛 故障排除

### 问题：服务启动失败
**解决方案：**
```bash
# 检查端口占用
netstat -tulpn | grep -E "(3000|4317|4318|8080|9090|16686)"

# 重置环境
make clean-all
make start
```

### 问题：没有数据显示
**解决方案：**
```bash
# 检查服务健康状态
make health-check

# 手动生成测试数据
curl "http://localhost:8080/api/login?user_id=test_user"
```

### 问题：Docker 内存不足
**解决方案：**
```bash
# 增加 Docker 内存限制到至少 4GB
# 或者使用轻量配置
docker-compose -f docker-compose.lite.yml up -d
```

## 📚 下一步

1. **自定义配置**: 编辑 `configs/` 下的配置文件
2. **添加指标**: 在 `internal/telemetry/` 中扩展指标定义
3. **集成到项目**: 参考 `internal/` 代码集成到您的项目
4. **生产部署**: 参考 `README.md` 中的生产配置建议

## 🆘 获取帮助

```bash
# 查看所有可用命令
make help

# 查看详细文档
cat README.md
```

有问题？请查看主 README.md 文档或提交 Issue！