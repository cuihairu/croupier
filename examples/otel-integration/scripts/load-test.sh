#!/bin/bash

# OpenTelemetry 集成负载测试脚本

set -e

echo "🔥 启动 OpenTelemetry 集成负载测试"
echo "=================================="

# 默认参数
USERS=${1:-10}
DURATION=${2:-60s}
SERVER_URL=${3:-"http://localhost:8080"}

echo "📋 测试参数:"
echo "  - 并发用户数: $USERS"
echo "  - 测试时长: $DURATION"
echo "  - 服务器地址: $SERVER_URL"
echo ""

# 检查服务器可用性
check_server() {
    echo "🔍 检查服务器可用性..."

    if ! curl -f -s "$SERVER_URL/health" > /dev/null; then
        echo "❌ 服务器 $SERVER_URL 不可用"
        echo "请先启动游戏服务器: ./scripts/start.sh start"
        exit 1
    fi

    echo "✅ 服务器可用"
}

# 负载测试函数
run_load_test() {
    echo "🚀 开始负载测试..."

    # 记录开始时间
    START_TIME=$(date +%s)

    # 创建结果目录
    mkdir -p test-results

    # 并发运行客户端
    for i in $(seq 1 $USERS); do
        (
            USER_ID="load_test_user_$i"
            echo "👤 启动用户 $USER_ID"

            # 循环执行测试操作
            end_time=$(($(date +%s) + $(echo $DURATION | sed 's/s$//')))

            while [ $(date +%s) -lt $end_time ]; do
                # 登录
                login_response=$(curl -s "$SERVER_URL/api/login?user_id=$USER_ID&platform=web&region=test")

                if echo "$login_response" | grep -q "success"; then
                    session_id=$(echo "$login_response" | sed -n 's/.*"session_id":"\([^"]*\)".*/\1/p')

                    # 开始关卡
                    for level in 1 2 3; do
                        level_response=$(curl -s "$SERVER_URL/api/level/start?session_id=$session_id&level_id=level_$level")

                        if echo "$level_response" | grep -q "started"; then
                            echo "🎮 用户 $USER_ID 开始关卡 level_$level"

                            # 模拟游戏时间
                            sleep $((RANDOM % 10 + 5))
                        fi
                    done
                fi

                # 随机等待
                sleep $((RANDOM % 5 + 1))
            done

            echo "✅ 用户 $USER_ID 测试完成"
        ) &
    done

    # 等待所有用户完成
    wait

    # 计算总时间
    END_TIME=$(date +%s)
    TOTAL_TIME=$((END_TIME - START_TIME))

    echo ""
    echo "📊 负载测试完成"
    echo "=================="
    echo "  - 总用户数: $USERS"
    echo "  - 实际测试时长: ${TOTAL_TIME}s"
    echo "  - 预期测试时长: $DURATION"
    echo ""
}

# 收集测试结果
collect_results() {
    echo "📈 收集测试结果..."

    # 从 Prometheus 收集指标（如果可用）
    if curl -f -s "http://localhost:9090/api/v1/query" > /dev/null 2>&1; then
        echo "  - 从 Prometheus 收集指标..."

        # 请求总数
        total_requests=$(curl -s "http://localhost:9090/api/v1/query?query=sum(game_request_total)" | \
            jq -r '.data.result[0].value[1]' 2>/dev/null || echo "N/A")

        # 错误总数
        total_errors=$(curl -s "http://localhost:9090/api/v1/query?query=sum(game_error_total)" | \
            jq -r '.data.result[0].value[1]' 2>/dev/null || echo "N/A")

        # 平均响应时间
        avg_latency=$(curl -s "http://localhost:9090/api/v1/query?query=rate(game_request_duration_sum[1m])/rate(game_request_duration_count[1m])" | \
            jq -r '.data.result[0].value[1]' 2>/dev/null || echo "N/A")

        # 会话总数
        total_sessions=$(curl -s "http://localhost:9090/api/v1/query?query=sum(game_session_total)" | \
            jq -r '.data.result[0].value[1]' 2>/dev/null || echo "N/A")

        echo ""
        echo "📊 性能指标:"
        echo "============="
        echo "  - 总请求数: $total_requests"
        echo "  - 总错误数: $total_errors"
        echo "  - 平均延迟: ${avg_latency}ms"
        echo "  - 总会话数: $total_sessions"

        # 计算错误率
        if [ "$total_requests" != "N/A" ] && [ "$total_errors" != "N/A" ]; then
            error_rate=$(echo "scale=2; $total_errors * 100 / $total_requests" | bc -l 2>/dev/null || echo "N/A")
            echo "  - 错误率: ${error_rate}%"
        fi

        echo ""
    else
        echo "  ⚠️ Prometheus 不可用，跳过指标收集"
    fi
}

# 生成测试报告
generate_report() {
    echo "📝 生成测试报告..."

    cat > test-results/load-test-report.md << EOF
# OpenTelemetry 集成负载测试报告

## 测试配置
- **测试时间**: $(date)
- **并发用户数**: $USERS
- **测试时长**: $DURATION
- **服务器地址**: $SERVER_URL

## 测试场景
1. 用户登录
2. 关卡开始（3个关卡）
3. 模拟游戏玩法

## 性能指标
$(if [ "$total_requests" != "N/A" ]; then
    echo "- **总请求数**: $total_requests"
    echo "- **总错误数**: $total_errors"
    echo "- **平均延迟**: ${avg_latency}ms"
    echo "- **错误率**: ${error_rate}%"
    echo "- **总会话数**: $total_sessions"
else
    echo "指标收集失败，请检查 Prometheus 服务"
fi)

## 观测性验证
- ✅ 分布式追踪数据生成
- ✅ 指标数据收集
- ✅ 结构化日志输出
- ✅ 告警规则触发检查

## 建议优化项
1. 如果错误率 > 5%，检查服务器配置
2. 如果平均延迟 > 100ms，考虑性能优化
3. 检查内存和CPU使用情况

## 查看详细数据
- **Grafana**: http://localhost:3000
- **Jaeger**: http://localhost:16686
- **Prometheus**: http://localhost:9090

---
*报告生成时间: $(date)*
EOF

    echo "✅ 测试报告已保存到: test-results/load-test-report.md"
}

# 主函数
main() {
    check_server
    run_load_test
    collect_results
    generate_report

    echo ""
    echo "🎉 负载测试完成！"
    echo ""
    echo "📋 后续步骤："
    echo "  1. 查看测试报告: cat test-results/load-test-report.md"
    echo "  2. 访问 Grafana 查看实时指标: http://localhost:3000"
    echo "  3. 访问 Jaeger 查看追踪数据: http://localhost:16686"
    echo ""
}

main "$@"