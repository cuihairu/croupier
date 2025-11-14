#!/bin/bash

# OpenTelemetry 集成演示脚本

set -e

echo "🎭 OpenTelemetry 集成功能演示"
echo "============================"

SERVER_URL="http://localhost:8080"

# 检查服务可用性
check_services() {
    echo "🔍 检查服务可用性..."

    services=(
        "游戏服务器:$SERVER_URL/health"
        "Grafana:http://localhost:3000/api/health"
        "Jaeger:http://localhost:16686/"
        "Prometheus:http://localhost:9090/-/healthy"
    )

    for service_info in "${services[@]}"; do
        IFS=':' read -r name url <<< "$service_info"
        if curl -f -s "$url" > /dev/null; then
            echo "  ✅ $name 可用"
        else
            echo "  ❌ $name 不可用"
            echo "请先启动服务: ./scripts/start.sh start"
            exit 1
        fi
    done

    echo ""
}

# 演示基础功能
demo_basic_functionality() {
    echo "🎮 演示 1: 基础游戏功能"
    echo "======================"

    echo "📝 场景: 正常用户游戏流程"
    echo "  - 用户登录"
    echo "  - 开始关卡"
    echo "  - 记录客户端指标"
    echo ""

    USER_ID="demo_user_$(date +%s)"
    echo "👤 用户ID: $USER_ID"

    # 1. 用户登录
    echo "🔑 步骤 1: 用户登录..."
    login_response=$(curl -s "$SERVER_URL/api/login?user_id=$USER_ID&platform=ios&region=cn-north")
    echo "   响应: $login_response"

    if echo "$login_response" | grep -q "success"; then
        echo "   ✅ 登录成功"
    else
        echo "   ❌ 登录失败"
        return 1
    fi

    # 2. 开始关卡
    echo "🎯 步骤 2: 开始关卡..."
    level_response=$(curl -s "$SERVER_URL/api/level/start?session_id=demo_session&level_id=demo_level_1")
    echo "   响应: $level_response"

    if echo "$level_response" | grep -q "started"; then
        echo "   ✅ 关卡开始成功"
    else
        echo "   ⚠️ 关卡开始可能失败（这是正常的，因为session_id可能无效）"
    fi

    echo "   💡 查看追踪数据: http://localhost:16686"
    echo ""
}

# 演示观测性功能
demo_observability() {
    echo "📊 演示 2: 观测性功能"
    echo "==================="

    echo "📝 场景: 观测性数据收集和查询"
    echo ""

    # 检查 Prometheus 指标
    echo "📈 步骤 1: 检查 Prometheus 指标..."
    if curl -f -s "http://localhost:9090/api/v1/query" > /dev/null; then
        # 获取一些示例指标
        echo "   查询游戏会话总数..."
        sessions=$(curl -s "http://localhost:9090/api/v1/query?query=sum(game_session_total)" | \
            jq -r '.data.result[0].value[1]' 2>/dev/null || echo "0")
        echo "   🎮 当前会话总数: $sessions"

        echo "   查询请求总数..."
        requests=$(curl -s "http://localhost:9090/api/v1/query?query=sum(game_request_total)" | \
            jq -r '.data.result[0].value[1]' 2>/dev/null || echo "0")
        echo "   📊 当前请求总数: $requests"

        echo "   ✅ Prometheus 指标可用"
    else
        echo "   ❌ Prometheus 不可用"
    fi

    echo ""

    # 检查 Jaeger 追踪
    echo "🔍 步骤 2: 检查 Jaeger 追踪..."
    if curl -f -s "http://localhost:16686/api/services" > /dev/null; then
        services=$(curl -s "http://localhost:16686/api/services" | jq -r '.data[]' 2>/dev/null | head -5)
        if [ -n "$services" ]; then
            echo "   📋 可用的追踪服务:"
            echo "$services" | while read service; do
                echo "     - $service"
            done
        fi
        echo "   ✅ Jaeger 追踪可用"
    else
        echo "   ❌ Jaeger 不可用"
    fi

    echo ""

    # 显示访问链接
    echo "🔗 观测性访问链接:"
    echo "   📊 Grafana 仪表板: http://localhost:3000"
    echo "   🔍 Jaeger 追踪界面: http://localhost:16686"
    echo "   📈 Prometheus: http://localhost:9090"
    echo "   🚨 AlertManager: http://localhost:9093"
    echo ""
}

# 演示游戏业务指标
demo_game_metrics() {
    echo "🎮 演示 3: 游戏业务指标"
    echo "====================="

    echo "📝 场景: 游戏业务指标收集和分析"
    echo ""

    echo "🎯 生成游戏业务事件..."

    # 模拟多个用户会话
    for i in {1..5}; do
        user_id="metrics_demo_user_$i"
        echo "  - 模拟用户 $user_id 的游戏会话..."

        # 登录
        curl -s "$SERVER_URL/api/login?user_id=$user_id&platform=android&region=us-west" > /dev/null

        # 开始不同关卡
        for level in 1 2 3; do
            curl -s "$SERVER_URL/api/level/start?session_id=demo_session_$i&level_id=level_$level" > /dev/null
            sleep 1
        done
    done

    echo "✅ 业务事件生成完成"
    echo ""

    # 等待指标更新
    echo "⏳ 等待指标更新（30秒）..."
    sleep 30

    # 查询业务指标
    echo "📊 查询最新业务指标..."
    if curl -f -s "http://localhost:9090/api/v1/query" > /dev/null; then
        echo "   🎮 会话指标:"

        # 会话总数
        sessions=$(curl -s "http://localhost:9090/api/v1/query?query=sum(game_session_total)" | \
            jq -r '.data.result[0].value[1]' 2>/dev/null || echo "N/A")
        echo "     - 总会话数: $sessions"

        # 关卡开始总数
        levels=$(curl -s "http://localhost:9090/api/v1/query?query=sum(game_level_start_total)" | \
            jq -r '.data.result[0].value[1]' 2>/dev/null || echo "N/A")
        echo "     - 关卡开始总数: $levels"

        # API 请求总数
        api_requests=$(curl -s "http://localhost:9090/api/v1/query?query=sum(game_request_total)" | \
            jq -r '.data.result[0].value[1]' 2>/dev/null || echo "N/A")
        echo "     - API 请求总数: $api_requests"

        echo "   ✅ 指标查询成功"
    else
        echo "   ❌ 无法查询指标"
    fi

    echo ""
}

# 演示告警功能
demo_alerts() {
    echo "🚨 演示 4: 告警功能"
    echo "=================="

    echo "📝 场景: 告警规则和通知"
    echo ""

    echo "🔍 检查告警规则..."
    if curl -f -s "http://localhost:9090/api/v1/rules" > /dev/null; then
        rules=$(curl -s "http://localhost:9090/api/v1/rules" | \
            jq -r '.data.groups[].rules[].name' 2>/dev/null | head -5)

        if [ -n "$rules" ]; then
            echo "   📋 已配置的告警规则:"
            echo "$rules" | while read rule; do
                echo "     - $rule"
            done
        fi

        echo "   ✅ 告警规则加载成功"
    else
        echo "   ❌ 无法检查告警规则"
    fi

    echo ""

    echo "🚨 检查活跃告警..."
    if curl -f -s "http://localhost:9090/api/v1/alerts" > /dev/null; then
        alerts=$(curl -s "http://localhost:9090/api/v1/alerts" | \
            jq -r '.data.alerts[] | .labels.alertname' 2>/dev/null)

        if [ -n "$alerts" ]; then
            echo "   ⚠️ 活跃告警:"
            echo "$alerts" | while read alert; do
                echo "     - $alert"
            done
        else
            echo "   ✅ 当前没有活跃告警"
        fi
    else
        echo "   ❌ 无法检查告警状态"
    fi

    echo ""
    echo "🔗 告警相关链接:"
    echo "   📊 Prometheus 告警: http://localhost:9090/alerts"
    echo "   🚨 AlertManager: http://localhost:9093"
    echo ""
}

# 演示性能分析
demo_performance_analysis() {
    echo "⚡ 演示 5: 性能分析"
    echo "=================="

    echo "📝 场景: 性能指标和分布式追踪分析"
    echo ""

    echo "🔧 生成性能测试负载..."

    # 快速负载测试
    echo "  - 运行轻量负载测试（5个并发用户，30秒）..."
    ./scripts/load-test.sh 5 30s > /dev/null 2>&1 &
    LOAD_TEST_PID=$!

    # 等待一些数据生成
    sleep 15

    echo "📊 分析性能指标..."

    if curl -f -s "http://localhost:9090/api/v1/query" > /dev/null; then
        # 查询延迟指标
        echo "   🕐 响应时间分析:"

        # P95 延迟
        p95_latency=$(curl -s "http://localhost:9090/api/v1/query?query=histogram_quantile(0.95,rate(game_request_duration_bucket[1m]))" | \
            jq -r '.data.result[0].value[1]' 2>/dev/null || echo "N/A")
        echo "     - P95 延迟: ${p95_latency}ms"

        # 错误率
        error_rate=$(curl -s "http://localhost:9090/api/v1/query?query=rate(game_error_total[1m])/rate(game_request_total[1m])" | \
            jq -r '.data.result[0].value[1]' 2>/dev/null || echo "N/A")
        echo "     - 错误率: $error_rate"

        # QPS
        qps=$(curl -s "http://localhost:9090/api/v1/query?query=rate(game_request_total[1m])" | \
            jq -r '.data.result[0].value[1]' 2>/dev/null || echo "N/A")
        echo "     - QPS: $qps"

        echo "   ✅ 性能指标收集成功"
    else
        echo "   ❌ 无法查询性能指标"
    fi

    # 等待负载测试完成
    wait $LOAD_TEST_PID 2>/dev/null || true

    echo ""
    echo "📈 性能分析建议:"
    echo "   1. 查看 Grafana 性能仪表板获取详细分析"
    echo "   2. 使用 Jaeger 分析慢请求的追踪信息"
    echo "   3. 查看告警规则以监控性能阈值"
    echo ""
}

# 显示总结
show_summary() {
    echo "🎉 演示完成总结"
    echo "=============="
    echo ""
    echo "✅ 已演示的功能:"
    echo "   🎮 基础游戏功能集成"
    echo "   📊 指标收集和查询"
    echo "   🔍 分布式追踪"
    echo "   🚨 告警规则和监控"
    echo "   ⚡ 性能分析"
    echo ""
    echo "🔗 继续探索:"
    echo "   📊 Grafana 仪表板: http://localhost:3000"
    echo "   🔍 Jaeger 追踪界面: http://localhost:16686"
    echo "   📈 Prometheus: http://localhost:9090"
    echo "   🚨 AlertManager: http://localhost:9093"
    echo ""
    echo "📚 更多操作:"
    echo "   - 运行负载测试: ./scripts/load-test.sh 10 60s"
    echo "   - 查看服务日志: ./scripts/start.sh logs"
    echo "   - 停止所有服务: ./scripts/start.sh stop"
    echo ""
}

# 主函数
main() {
    echo "🚀 开始 OpenTelemetry 集成演示"
    echo ""

    check_services

    echo "📋 演示流程:"
    echo "  1. 基础游戏功能"
    echo "  2. 观测性功能"
    echo "  3. 游戏业务指标"
    echo "  4. 告警功能"
    echo "  5. 性能分析"
    echo ""

    read -p "按回车键开始演示..." -r
    echo ""

    demo_basic_functionality
    demo_observability
    demo_game_metrics
    demo_alerts
    demo_performance_analysis

    show_summary
}

main "$@"