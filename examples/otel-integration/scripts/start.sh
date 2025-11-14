#!/bin/bash

# OpenTelemetry 集成示例启动脚本

set -e

echo "🚀 启动 Croupier OpenTelemetry 集成示例"
echo "========================================="

# 检查依赖
check_dependencies() {
    echo "🔍 检查依赖..."

    if ! command -v docker &> /dev/null; then
        echo "❌ Docker 未安装，请先安装 Docker"
        exit 1
    fi

    if ! command -v docker-compose &> /dev/null; then
        echo "❌ Docker Compose 未安装，请先安装 Docker Compose"
        exit 1
    fi

    if ! command -v go &> /dev/null; then
        echo "❌ Go 未安装，请先安装 Go"
        exit 1
    fi

    echo "✅ 依赖检查通过"
}

# 构建应用程序
build_apps() {
    echo "🔨 构建示例应用程序..."

    mkdir -p bin

    echo "  - 构建游戏服务器..."
    go build -o bin/server cmd/server/main.go

    echo "  - 构建游戏客户端..."
    go build -o bin/client cmd/client/main.go

    echo "  - 构建游戏模拟器..."
    go build -o bin/game-simulator cmd/game-simulator/main.go

    echo "✅ 应用程序构建完成"
}

# 启动观测性基础设施
start_infrastructure() {
    echo "🏗️ 启动观测性基础设施..."

    # 停止可能存在的容器
    docker-compose down --remove-orphans

    # 启动服务
    docker-compose up -d

    # 等待服务启动
    echo "⏳ 等待服务启动..."
    sleep 30

    # 检查服务状态
    check_services

    echo "✅ 基础设施启动完成"
}

# 检查服务状态
check_services() {
    echo "🔍 检查服务状态..."

    services=(
        "otel-collector:13133/health_check"
        "prometheus:9090/-/healthy"
        "grafana:3000/api/health"
        "jaeger:14269/"
    )

    for service in "${services[@]}"; do
        IFS=':' read -r name endpoint <<< "$service"
        echo "  - 检查 $name..."

        max_attempts=30
        attempt=1

        while [ $attempt -le $max_attempts ]; do
            if curl -f -s "http://localhost:$endpoint" > /dev/null 2>&1; then
                echo "    ✅ $name 健康"
                break
            fi

            if [ $attempt -eq $max_attempts ]; then
                echo "    ❌ $name 不健康"
                return 1
            fi

            sleep 2
            ((attempt++))
        done
    done

    echo "✅ 所有服务健康"
}

# 显示访问信息
show_access_info() {
    echo ""
    echo "🌐 服务访问信息："
    echo "================================"
    echo "📊 Grafana 仪表板:     http://localhost:3000 (admin/admin)"
    echo "🔍 Jaeger 追踪界面:    http://localhost:16686"
    echo "📈 Prometheus:         http://localhost:9090"
    echo "🚨 AlertManager:       http://localhost:9093"
    echo "🔧 OTel Collector 调试: http://localhost:55679"
    echo "💾 Redis 缓存:         localhost:6379"
    echo ""
    echo "📋 API 端点:"
    echo "================================"
    echo "🎮 游戏服务器:         http://localhost:8080"
    echo "📊 健康检查:           http://localhost:8080/health"
    echo "🔑 玩家登录:           http://localhost:8080/api/login"
    echo "🎯 关卡开始:           http://localhost:8080/api/level/start"
    echo ""
}

# 启动示例应用程序
start_demo_apps() {
    echo "🎮 启动示例应用程序..."

    # 设置环境变量
    export OTEL_SERVICE_NAME="game-server"
    export OTEL_SERVICE_VERSION="1.0.0"
    export OTEL_ENVIRONMENT="demo"
    export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"
    export GAME_ID="croupier-demo"
    export OTEL_SAMPLING_RATIO="1.0"
    export ANALYTICS_REDIS_ADDR="localhost:6379"

    echo "  - 启动游戏服务器（后台）..."
    nohup ./bin/server > logs/server.log 2>&1 &
    SERVER_PID=$!
    echo "    游戏服务器 PID: $SERVER_PID"

    # 等待服务器启动
    sleep 5

    echo "  - 启动游戏模拟器（后台）..."
    nohup ./bin/game-simulator > logs/simulator.log 2>&1 &
    SIMULATOR_PID=$!
    echo "    游戏模拟器 PID: $SIMULATOR_PID"

    # 保存 PID 用于后续停止
    echo $SERVER_PID > .server.pid
    echo $SIMULATOR_PID > .simulator.pid

    echo "✅ 示例应用程序启动完成"
}

# 运行演示
run_demo() {
    echo "🎭 运行演示..."

    echo "  - 等待数据生成（60秒）..."
    sleep 60

    echo "  - 运行客户端示例..."
    ./bin/client

    echo "✅ 演示完成"
}

# 主函数
main() {
    case "${1:-start}" in
        "start")
            check_dependencies
            build_apps
            mkdir -p logs
            start_infrastructure
            show_access_info
            start_demo_apps

            echo "🎉 OpenTelemetry 集成示例启动完成！"
            echo ""
            echo "💡 提示："
            echo "  - 查看日志: tail -f logs/server.log 或 tail -f logs/simulator.log"
            echo "  - 停止示例: ./scripts/start.sh stop"
            echo "  - 运行演示: ./scripts/start.sh demo"
            echo ""
            echo "🔗 开始探索："
            echo "  1. 访问 Grafana 查看指标仪表板"
            echo "  2. 访问 Jaeger 查看分布式追踪"
            echo "  3. 访问 Prometheus 查看原始指标"
            echo ""
            ;;

        "stop")
            echo "🛑 停止 OpenTelemetry 集成示例..."

            # 停止应用程序
            if [ -f .server.pid ]; then
                SERVER_PID=$(cat .server.pid)
                echo "  - 停止游戏服务器 (PID: $SERVER_PID)..."
                kill $SERVER_PID 2>/dev/null || true
                rm .server.pid
            fi

            if [ -f .simulator.pid ]; then
                SIMULATOR_PID=$(cat .simulator.pid)
                echo "  - 停止游戏模拟器 (PID: $SIMULATOR_PID)..."
                kill $SIMULATOR_PID 2>/dev/null || true
                rm .simulator.pid
            fi

            # 停止 Docker 服务
            echo "  - 停止 Docker 服务..."
            docker-compose down

            echo "✅ 示例已停止"
            ;;

        "demo")
            run_demo
            ;;

        "status")
            echo "📊 服务状态："
            docker-compose ps
            ;;

        "logs")
            service=${2:-"all"}
            if [ "$service" = "all" ]; then
                docker-compose logs -f
            else
                docker-compose logs -f $service
            fi
            ;;

        *)
            echo "使用方法: $0 {start|stop|demo|status|logs [service]}"
            echo ""
            echo "命令说明："
            echo "  start  - 启动完整的 OpenTelemetry 示例"
            echo "  stop   - 停止所有服务"
            echo "  demo   - 运行演示客户端"
            echo "  status - 显示服务状态"
            echo "  logs   - 显示服务日志"
            exit 1
            ;;
    esac
}

main "$@"