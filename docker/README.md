# Docker 容器化部署

本目录包含 Croupier 所有服务的 Docker 配置文件。

## 目录结构

```
docker/
├── README.md                          # 本文件
├── docker-compose.yml                 # 主服务编排配置
├── docker-compose.telemetry.yaml      # 遥测监控栈配置
├── Dockerfile.server                  # Server 服务镜像
├── Dockerfile.agent                   # Agent 服务镜像
├── Dockerfile.edge                    # Edge 服务镜像
├── Dockerfile.web                     # Web UI 镜像
├── Dockerfile.analytics-worker        # 分析工作器镜像
├── Dockerfile.ingest                  # 数据摄取服务镜像
└── Dockerfile.demo                    # 演示应用镜像
```

## 快速开始

### 启动所有核心服务

```bash
cd docker
docker-compose up -d
```

这将启动：
- PostgreSQL (端口 5432)
- Redis (端口 6379)
- ClickHouse (端口 8123/9000)
- Edge 服务 (端口 9443/9080)
- Server 服务 (端口 8443/18080)
- Agent 服务 (端口 19090/19091)
- Web UI (端口 8000)
- Analytics Ingestion (端口 18081)

### 启动遥测监控栈

```bash
cd docker
docker-compose -f docker-compose.telemetry.yaml up -d
```

这将启动：
- OpenTelemetry Collector (端口 4317/4318)
- Jaeger (端口 16686)
- Prometheus (端口 9090)
- Grafana (端口 3000)
- Redis (端口 6379)
- ClickHouse (端口 8123/9000)
- Demo 应用 (端口 8080)

## 服务访问

### 核心服务

| 服务 | 访问地址 | 说明 |
|-----|---------|------|
| Web UI | http://localhost:8000 | 管理控制台 |
| Server HTTP API | http://localhost:18080 | REST API |
| Server gRPC | localhost:8443 | gRPC 服务 |
| Edge gRPC | localhost:9443 | Edge 代理 |
| Agent | localhost:19090 | 本地 Agent |
| Analytics Ingestion | http://localhost:18081 | 公网/DMZ 摄取入口（开发） |

### 数据库

| 服务 | 访问地址 | 凭据 |
|-----|---------|------|
| PostgreSQL | localhost:5432 | croupier/croupier_dev_password |
| Redis | localhost:6379 | 无密码 |
| ClickHouse | localhost:8123 | default/无密码 |

### 管理工具（需要 `--profile tools` 启动）

```bash
docker-compose --profile tools up -d
```

| 工具 | 访问地址 | 凭据 |
|-----|---------|------|
| pgAdmin | http://localhost:8082 | admin@croupier.local/admin123 |
| Redis Commander | http://localhost:8083 | admin/admin123 |

### 遥测监控

| 服务 | 访问地址 | 凭据 |
|-----|---------|------|
| Grafana | http://localhost:13000 | admin/admin |
| Prometheus | http://localhost:19092 | 无需认证 |
| Jaeger | http://localhost:17686 | 无需认证 |

## 常用命令

```bash
# 查看所有服务状态
docker-compose ps

# 查看日志
docker-compose logs -f [service_name]

# 停止所有服务
docker-compose down

# 停止并删除卷数据
docker-compose down -v

# 重启特定服务
docker-compose restart server

# 仅启动特定服务及其依赖
docker-compose up -d server

# 重新构建镜像
docker-compose build [service_name]

# 启动流式计算服务（Kafka + Flink）
docker-compose --profile stream up -d
```

## 环境变量

可以通过 `.env` 文件或环境变量覆盖默认配置：

```bash
# 数据库
DATABASE_URL=postgres://user:pass@host:5432/dbname
REDIS_URL=redis://host:6379/0
CLICKHOUSE_DSN=clickhouse://host:9000/analytics

# 镜像版本
GO_IMAGE=golang:1.25
POSTGRES_IMAGE=postgres:15-alpine
REDIS_IMAGE=redis:7-alpine

# 服务配置
SERVER_BUILD_TAGS="pg sqlite ip2location"
ANALYTICS_MQ_TYPE=redis
```

## 开发模式

开发模式下，建议使用以下配置：

```bash
# 仅启动数据库服务
docker-compose up -d postgres redis clickhouse

# 本地运行服务（更快的开发迭代）
cd ../services/server
go run . -f etc/croupier-api.yaml
```

## 生产部署建议

1. **修改默认密码**：所有数据库和管理工具的密码
2. **配置 TLS**：为所有服务启用 HTTPS/TLS
3. **持久化存储**：使用外部卷或云存储
4. **资源限制**：在 docker-compose.yml 中添加 CPU/内存限制
5. **监控告警**：配置 Grafana 告警规则
6. **备份策略**：定期备份 PostgreSQL 和 ClickHouse 数据

## 故障排查

### 服务无法启动

```bash
# 查看详细日志
docker-compose logs service_name

# 检查健康状态
docker-compose ps
```

### 端口冲突

修改 `docker-compose.yml` 中的端口映射：

```yaml
ports:
  - "新端口:容器端口"
```

### 数据库连接失败

确保服务健康检查通过：

```bash
docker-compose ps
# 查看 STATUS 列是否显示 healthy
```

## 更多信息

- [Croupier 文档](../README.md)
- [配置说明](../configs/)
- [开发指南](../DEVELOPMENT.md)
