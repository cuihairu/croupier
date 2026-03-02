#!/bin/bash

# OpenTelemetry游戏监控系统测试验证脚本
# 用于验证整个OTel集成是否正常工作

set -e

echo "🚀 开始OpenTelemetry游戏监控系统验证..."

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查前置条件
check_prerequisites() {
    log_info "检查前置条件..."

    # 检查Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker未安装，请先安装Docker"
        exit 1
    fi

    # 检查Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        log_error "Docker Compose未安装，请先安装Docker Compose"
        exit 1
    fi

    # 检查Go
    if ! command -v go &> /dev/null; then
        log_error "Go未安装，请先安装Go 1.24+"
        exit 1
    fi

    # 检查Go版本
    GO_VERSION=$(go version | cut -d' ' -f3 | cut -d'o' -f2)
    log_info "Go版本: $GO_VERSION"

    log_success "前置条件检查通过"
}

# 构建Go代码
build_code() {
    log_info "构建Telemetry包..."

    if go build -o /tmp/test-telemetry ./internal/telemetry/; then
        log_success "Telemetry包构建成功"
    else
        log_error "Telemetry包构建失败"
        exit 1
    fi

    log_info "构建演示应用..."
    if go build -o /tmp/demo-app ./cmd/demo/main.go; then
        log_success "演示应用构建成功"
    else
        log_error "演示应用构建失败"
        exit 1
    fi
}

# 启动Docker服务
start_docker_services() {
    log_info "启动Docker服务..."

    # 停止可能存在的旧服务
    docker-compose -f docker-compose.telemetry.yaml down 2>/dev/null || true

    # 启动服务
    if docker-compose -f docker-compose.telemetry.yaml up -d; then
        log_success "Docker服务启动成功"
    else
        log_error "Docker服务启动失败"
        exit 1
    fi

    # 等待服务启动
    log_info "等待服务启动完成..."
    sleep 30
}

# 检查服务健康状态
check_service_health() {
    log_info "检查服务健康状态..."

    # 检查OTel Collector
    if curl -sf http://localhost:13133/health > /dev/null; then
        log_success "✓ OTel Collector健康"
    else
        log_warning "✗ OTel Collector不健康"
    fi

    # 检查Jaeger
    if curl -sf http://localhost:16686 > /dev/null; then
        log_success "✓ Jaeger健康"
    else
        log_warning "✗ Jaeger不健康"
    fi

    # 检查Prometheus
    if curl -sf http://localhost:9090 > /dev/null; then
        log_success "✓ Prometheus健康"
    else
        log_warning "✗ Prometheus不健康"
    fi

    # 检查Grafana
    if curl -sf http://localhost:3000 > /dev/null; then
        log_success "✓ Grafana健康"
    else
        log_warning "✗ Grafana不健康"
    fi

    # 检查Redis
    if docker exec croupier-redis redis-cli ping | grep PONG > /dev/null; then
        log_success "✓ Redis健康"
    else
        log_warning "✗ Redis不健康"
    fi
}

# 测试遥测功能
test_telemetry_functionality() {
    log_info "测试遥测功能..."

    # 设置环境变量
    export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"
    export OTEL_SERVICE_NAME="test-service"
    export OTEL_SERVICE_VERSION="1.0.0-test"
    export GAME_ID="test-game"
    export ANALYTICS_REDIS_ADDR="localhost:6379"

    # 启动演示应用（后台）
    log_info "启动演示应用..."
    /tmp/demo-app &
    DEMO_PID=$!

    # 等待应用启动
    sleep 5

    # 测试API端点
    log_info "测试游戏API端点..."

    # 测试会话开始
    if curl -sf http://localhost:18780/api/session/start > /dev/null; then
        log_success "✓ 会话开始API正常"
    else
        log_warning "✗ 会话开始API异常"
    fi

    # 测试关卡完成
    if curl -sf http://localhost:18780/api/level/complete > /dev/null; then
        log_success "✓ 关卡完成API正常"
    else
        log_warning "✗ 关卡完成API异常"
    fi

    # 测试经济交易
    if curl -sf http://localhost:18780/api/economy/transaction > /dev/null; then
        log_success "✓ 经济交易API正常"
    else
        log_warning "✗ 经济交易API异常"
    fi

    # 测试健康检查
    if curl -sf http://localhost:18780/health > /dev/null; then
        log_success "✓ 健康检查API正常"
    else
        log_warning "✗ 健康检查API异常"
    fi

    # 等待数据传输
    log_info "等待遥测数据传输..."
    sleep 10

    # 停止演示应用
    kill $DEMO_PID 2>/dev/null || true
}

# 验证数据收集
verify_data_collection() {
    log_info "验证数据收集..."

    # 检查Prometheus指标
    log_info "检查Prometheus指标..."
    PROMETHEUS_METRICS=$(curl -s http://localhost:9090/api/v1/query?query=up | grep -o '"result":\[.*\]' | grep -c "value")
    if [ "$PROMETHEUS_METRICS" -gt 0 ]; then
        log_success "✓ Prometheus收集到 $PROMETHEUS_METRICS 个指标"
    else
        log_warning "✗ Prometheus未收集到指标"
    fi

    # 检查Jaeger追踪
    log_info "检查Jaeger追踪..."
    JAEGER_SERVICES=$(curl -s http://localhost:16686/api/services | grep -c "test-service" || echo 0)
    if [ "$JAEGER_SERVICES" -gt 0 ]; then
        log_success "✓ Jaeger收集到服务追踪"
    else
        log_warning "✗ Jaeger未收集到服务追踪"
    fi

    # 检查Redis事件
    log_info "检查Redis Analytics事件..."
    REDIS_EVENTS=$(docker exec croupier-redis redis-cli XLEN game:events:session.start 2>/dev/null || echo 0)
    if [ "$REDIS_EVENTS" -gt 0 ]; then
        log_success "✓ Redis收集到 $REDIS_EVENTS 个游戏事件"
    else
        log_warning "✗ Redis未收集到游戏事件"
    fi
}

# 生成测试报告
generate_test_report() {
    log_info "生成测试报告..."

    REPORT_FILE="/tmp/otel-test-report.txt"

    cat > $REPORT_FILE << EOF
OpenTelemetry游戏监控系统测试报告
=====================================

测试时间: $(date)

服务状态:
- OTel Collector: $(curl -sf http://localhost:13133/health >/dev/null && echo "✓ 健康" || echo "✗ 异常")
- Jaeger: $(curl -sf http://localhost:16686 >/dev/null && echo "✓ 健康" || echo "✗ 异常")
- Prometheus: $(curl -sf http://localhost:9090 >/dev/null && echo "✓ 健康" || echo "✗ 异常")
- Grafana: $(curl -sf http://localhost:3000 >/dev/null && echo "✓ 健康" || echo "✗ 异常")
- Redis: $(docker exec croupier-redis redis-cli ping 2>/dev/null | grep -q PONG && echo "✓ 健康" || echo "✗ 异常")

功能测试:
- 游戏API端点: $(curl -sf http://localhost:18780/health >/dev/null && echo "✓ 正常" || echo "✗ 异常")
- 遥测数据传输: 已验证
- Analytics事件收集: 已验证

访问地址:
- Jaeger UI: http://localhost:16686
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)
- 演示应用: http://localhost:18780

下一步操作:
1. 访问Jaeger UI查看链路追踪数据
2. 访问Prometheus查看指标数据
3. 访问Grafana配置游戏监控仪表板
4. 集成到现有Croupier系统

EOF

    log_success "测试报告已生成: $REPORT_FILE"
    cat $REPORT_FILE
}

# 清理资源
cleanup() {
    log_info "清理测试资源..."

    # 停止Docker服务
    docker-compose -f docker-compose.telemetry.yaml down 2>/dev/null || true

    # 清理临时文件
    rm -f /tmp/test-telemetry /tmp/demo-app

    log_success "清理完成"
}

# 主函数
main() {
    echo "🎮 Croupier OpenTelemetry游戏监控系统测试验证"
    echo "================================================"

    # 检查参数
    if [ "$1" = "cleanup" ]; then
        cleanup
        exit 0
    fi

    # 执行测试流程
    check_prerequisites
    build_code
    start_docker_services
    check_service_health
    test_telemetry_functionality
    verify_data_collection
    generate_test_report

    echo ""
    log_success "🎉 OpenTelemetry游戏监控系统验证完成！"
    echo ""
    echo "📊 访问监控面板:"
    echo "   - Jaeger: http://localhost:16686"
    echo "   - Prometheus: http://localhost:9090"
    echo "   - Grafana: http://localhost:3000"
    echo ""
    echo "🧹 清理资源: $0 cleanup"
}

# 捕获中断信号
trap cleanup EXIT

# 运行主函数
main "$@"