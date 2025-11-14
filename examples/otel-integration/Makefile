# OpenTelemetry 集成示例 Makefile

.PHONY: help build start stop demo test clean logs status

# 默认目标
.DEFAULT_GOAL := help

# 变量定义
GO_VERSION := 1.24
BINARY_DIR := bin
SERVER_BINARY := $(BINARY_DIR)/server
CLIENT_BINARY := $(BINARY_DIR)/client
SIMULATOR_BINARY := $(BINARY_DIR)/game-simulator

# 帮助信息
help: ## 显示帮助信息
	@echo "OpenTelemetry 集成示例 - 可用命令："
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "快速开始："
	@echo "  1. make build    # 构建应用程序"
	@echo "  2. make start    # 启动完整环境"
	@echo "  3. make demo     # 运行演示"
	@echo "  4. make test     # 运行测试"
	@echo "  5. make stop     # 停止所有服务"

# 构建相关
deps: ## 下载依赖
	@echo "🔄 下载 Go 依赖..."
	go mod download
	go mod tidy

build: deps $(SERVER_BINARY) $(CLIENT_BINARY) $(SIMULATOR_BINARY) build-enhanced ## 构建所有二进制文件
	@echo "✅ 所有应用程序构建完成"

$(BINARY_DIR):
	@mkdir -p $(BINARY_DIR)

$(SERVER_BINARY): $(BINARY_DIR) cmd/server/main.go internal/telemetry/*.go internal/game/*.go
	@echo "🔨 构建游戏服务器..."
	go build -o $(SERVER_BINARY) cmd/server/main.go

$(CLIENT_BINARY): $(BINARY_DIR) cmd/client/main.go internal/telemetry/*.go
	@echo "🔨 构建游戏客户端..."
	go build -o $(CLIENT_BINARY) cmd/client/main.go

$(SIMULATOR_BINARY): $(BINARY_DIR) cmd/game-simulator/main.go internal/telemetry/*.go internal/game/*.go
	@echo "🔨 构建游戏模拟器..."
	go build -o $(SIMULATOR_BINARY) cmd/game-simulator/main.go

build-enhanced: $(BINARY_DIR) cmd/enhanced-client/main.go internal/telemetry/*.go ## 构建增强客户端
	@echo "🔨 构建增强客户端（含完整分析指标）..."
	go build -o $(BINARY_DIR)/enhanced-client cmd/enhanced-client/main.go

# 开发相关
dev-deps: ## 安装开发依赖
	@echo "🔧 安装开发依赖..."
	@which docker > /dev/null || (echo "❌ Docker 未安装" && exit 1)
	@which docker-compose > /dev/null || (echo "❌ Docker Compose 未安装" && exit 1)
	@which curl > /dev/null || (echo "❌ curl 未安装" && exit 1)
	@echo "✅ 开发依赖检查完成"

fmt: ## 格式化代码
	@echo "🎨 格式化 Go 代码..."
	go fmt ./...

lint: ## 代码检查
	@echo "🔍 运行代码检查..."
	@which golangci-lint > /dev/null || (echo "安装 golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

# 运行相关
start: build dev-deps ## 启动完整的 OpenTelemetry 示例环境
	@echo "🚀 启动 OpenTelemetry 集成示例..."
	./scripts/start.sh start

stop: ## 停止所有服务
	@echo "🛑 停止所有服务..."
	./scripts/start.sh stop

restart: stop start ## 重启所有服务

demo: ## 运行交互式演示
	@echo "🎭 运行演示..."
	./scripts/demo.sh

status: ## 显示服务状态
	@echo "📊 服务状态："
	./scripts/start.sh status

logs: ## 显示服务日志
	@echo "📋 服务日志："
	./scripts/start.sh logs

# 测试相关
test: build ## 运行单元测试
	@echo "🧪 运行单元测试..."
	go test -race -coverprofile=coverage.out ./...

test-integration: ## 运行集成测试
	@echo "🔗 运行集成测试..."
	@echo "请先启动服务: make start"
	go test -tags=integration ./test/...

test-client-analytics: ## 测试客户端分析指标完整性
	@echo "🔬 测试客户端分析指标..."
	./scripts/test-client-analytics.sh 60 3

test-client-analytics-extended: ## 扩展客户端分析测试（5分钟，10客户端）
	@echo "🔬 扩展客户端分析测试..."
	./scripts/test-client-analytics.sh 300 10

load-test: ## 运行负载测试（默认10用户60秒）
	@echo "🔥 运行负载测试..."
	./scripts/load-test.sh 10 60s

load-test-heavy: ## 运行重负载测试（50用户5分钟）
	@echo "💥 运行重负载测试..."
	./scripts/load-test.sh 50 300s

coverage: test ## 生成测试覆盖率报告
	@echo "📊 生成测试覆盖率报告..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "📋 覆盖率报告: coverage.html"

# 运维相关
docker-build: ## 构建 Docker 镜像
	@echo "🐳 构建 Docker 镜像..."
	docker build -t croupier-otel-server:latest -f docker/Dockerfile.server .
	docker build -t croupier-otel-client:latest -f docker/Dockerfile.client .
	docker build -t croupier-otel-simulator:latest -f docker/Dockerfile.simulator .

docker-push: docker-build ## 推送 Docker 镜像
	@echo "📤 推送 Docker 镜像..."
	@echo "请配置 Docker registry 地址"

backup: ## 备份配置和数据
	@echo "💾 备份配置和数据..."
	@mkdir -p backups/$(shell date +%Y%m%d_%H%M%S)
	@tar -czf backups/$(shell date +%Y%m%d_%H%M%S)/configs.tar.gz configs/
	@echo "✅ 备份完成: backups/$(shell date +%Y%m%d_%H%M%S)/"

# 清理相关
clean: ## 清理构建产物
	@echo "🧹 清理构建产物..."
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html
	rm -rf test-results

clean-docker: ## 清理 Docker 资源
	@echo "🐳 清理 Docker 资源..."
	docker-compose down -v --remove-orphans
	docker system prune -f

clean-all: clean clean-docker ## 完全清理

# 监控相关
monitor: ## 打开监控面板
	@echo "📊 打开监控面板..."
	@echo "Grafana: http://localhost:3000 (admin/admin)"
	@echo "Jaeger: http://localhost:16686"
	@echo "Prometheus: http://localhost:9090"
	@echo "AlertManager: http://localhost:9093"
	@if command -v open >/dev/null 2>&1; then \
		open http://localhost:3000; \
	elif command -v xdg-open >/dev/null 2>&1; then \
		xdg-open http://localhost:3000; \
	fi

health-check: ## 检查所有服务健康状态
	@echo "🔍 检查服务健康状态..."
	@curl -f -s http://localhost:8080/health > /dev/null && echo "✅ 游戏服务器健康" || echo "❌ 游戏服务器不健康"
	@curl -f -s http://localhost:3000/api/health > /dev/null && echo "✅ Grafana 健康" || echo "❌ Grafana 不健康"
	@curl -f -s http://localhost:16686/ > /dev/null && echo "✅ Jaeger 健康" || echo "❌ Jaeger 不健康"
	@curl -f -s http://localhost:9090/-/healthy > /dev/null && echo "✅ Prometheus 健康" || echo "❌ Prometheus 不健康"

# 文档相关
docs: ## 生成文档
	@echo "📚 生成文档..."
	@echo "当前文档位置："
	@echo "  - README.md: 主要文档"
	@echo "  - configs/: 配置文件文档"
	@echo "  - scripts/: 脚本使用说明"

# 极简集成相关
simple-game: build-simple ## 运行极简游戏服务器示例（演示5分钟集成）
	@echo "🎮 运行极简游戏服务器示例..."
	./bin/simple-game-server

build-simple: $(BINARY_DIR) cmd/simple-game-server/main.go simple_analytics.go ## 构建极简游戏服务器
	@echo "🔨 构建极简游戏服务器..."
	go build -o $(BINARY_DIR)/simple-game-server cmd/simple-game-server/main.go

demo-simple: ## 完整演示：启动服务+运行极简游戏（一键体验）
	@echo "🚀 启动完整极简演示..."
	@echo "1. 检查Croupier Server是否运行..."
	@curl -f -s http://localhost:8080/health > /dev/null || (echo "❌ 请先启动Croupier Server" && exit 1)
	@echo "2. 启动极简游戏服务器..."
	$(MAKE) simple-game
	@echo "3. 查看Analytics数据..."
	@echo "访问 http://localhost:3000 查看Grafana仪表板"

# 开发辅助
dev-server: build ## 启动开发模式服务器（仅服务器，无容器）
	@echo "🔧 启动开发模式服务器..."
	@export OTEL_SERVICE_NAME=game-server && \
	export OTEL_SERVICE_VERSION=dev && \
	export OTEL_ENVIRONMENT=development && \
	export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 && \
	export GAME_ID=dev-game && \
	export OTEL_SAMPLING_RATIO=1.0 && \
	./$(SERVER_BINARY)

dev-client: build ## 运行开发模式客户端
	@echo "🔧 运行开发模式客户端..."
	@export OTEL_SERVICE_NAME=game-client && \
	export OTEL_SERVICE_VERSION=dev && \
	export OTEL_ENVIRONMENT=development && \
	export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 && \
	./$(CLIENT_BINARY)

watch: ## 监控文件变化并重新构建
	@echo "👀 监控文件变化..."
	@which fswatch > /dev/null || (echo "请安装 fswatch: brew install fswatch" && exit 1)
	fswatch -o cmd/ internal/ | xargs -n1 -I{} make build

# 性能分析
profile: ## 运行性能分析
	@echo "📈 运行性能分析..."
	go tool pprof -http=:8081 http://localhost:1777/debug/pprof/profile
	@echo "性能分析界面: http://localhost:8081"

bench: ## 运行基准测试
	@echo "⚡ 运行基准测试..."
	go test -bench=. -benchmem ./...

# 便捷别名
up: start     ## 别名：启动服务
down: stop    ## 别名：停止服务
ps: status    ## 别名：查看状态
tail: logs    ## 别名：查看日志

# 验证环境
verify: ## 验证环境和配置
	@echo "✅ 验证环境..."
	@go version | grep -q "go1.24" || (echo "⚠️ 建议使用 Go 1.24" && go version)
	@which docker > /dev/null && echo "✅ Docker 可用" || echo "❌ Docker 不可用"
	@which docker-compose > /dev/null && echo "✅ Docker Compose 可用" || echo "❌ Docker Compose 不可用"
	@which curl > /dev/null && echo "✅ curl 可用" || echo "❌ curl 不可用"
	@[ -f configs/otel-collector.yaml ] && echo "✅ OTel Collector 配置存在" || echo "❌ OTel Collector 配置缺失"
	@[ -f docker-compose.yml ] && echo "✅ Docker Compose 配置存在" || echo "❌ Docker Compose 配置缺失"