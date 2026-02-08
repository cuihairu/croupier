#!/bin/bash

# ============================================================================
# Croupier 函数注册诊断脚本
# ============================================================================

echo "=========================================="
echo "🔍 Croupier 函数注册诊断"
echo "=========================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# ============================================================================
# 检查 1: Server 进程
# ============================================================================
echo "📋 检查 1: Server 进程"
echo "----------------------------------------"

if pgrep -f "croupier-server" > /dev/null; then
    echo -e "${GREEN}✅ Server 进程正在运行${NC}"
    pgrep -f "croupier-server" | head -1
else
    echo -e "${RED}❌ Server 进程未运行${NC}"
    echo ""
    echo "请先启动 Server："
    echo "  方式 1 (VS Code): 按 F5 → 选择 'Server (dev sqlite)'"
    echo "  方式 2 (命令行):"
    echo "    cd /Users/cui/Workspaces/croupier/croupier"
    echo "    ./bin/croupier-server -f services/server/etc/server.yaml"
    echo ""
    exit 1
fi
echo ""

# ============================================================================
# 检查 2: Agent 进程
# ============================================================================
echo "📋 检查 2: Agent 进程"
echo "----------------------------------------"

if pgrep -f "croupier-agent" > /dev/null; then
    echo -e "${GREEN}✅ Agent 进程正在运行${NC}"
    pgrep -f "croupier-agent" | head -1
else
    echo -e "${RED}❌ Agent 进程未运行${NC}"
    echo ""
    echo "请启动 Agent："
    echo "  方式 1 (VS Code): 按 F5 → 选择 'Agent (多文件示例)'"
    echo "  方式 2 (命令行):"
    echo "    cd /Users/cui/Workspaces/croupier/croupier/services/agent"
    echo "    ../../bin/croupier-agent -f etc/agent.yaml"
    echo ""
    exit 1
fi
echo ""

# ============================================================================
# 检查 3: 端口监听
# ============================================================================
echo "📋 检查 3: 端口监听状态"
echo "----------------------------------------"

# Server HTTP 端口 (18780)
if lsof -i :18780 > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Server HTTP (18780) - 正在监听${NC}"
else
    echo -e "${RED}❌ Server HTTP (18780) - 未监听${NC}"
fi

# Server Control 端口 (19090)
if lsof -i :19090 > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Server Control (19090) - 正在监听${NC}"
else
    echo -e "${YELLOW}⚠️  Server Control (19090) - 未监听${NC}"
fi

# Agent HTTP 端口 (18888)
if lsof -i :18888 > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Agent HTTP (18888) - 正在监听${NC}"
else
    echo -e "${RED}❌ Agent HTTP (18888) - 未监听${NC}"
fi

# Dashboard 端口 (8000)
if lsof -i :8000 > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Dashboard (8000) - 正在监听${NC}"
else
    echo -e "${RED}❌ Dashboard (8000) - 未监听${NC}"
fi

echo ""

# ============================================================================
# 检查 4: 配置文件
# ============================================================================
echo "📋 检查 4: 配置文件"
echo "----------------------------------------"

PLATFORMS_YAML="/Users/cui/Workspaces/croupier/croupier/services/agent/etc/platforms.yaml"
if [ -f "$PLATFORMS_YAML" ]; then
    echo -e "${GREEN}✅ platforms.yaml 存在${NC}"

    # 验证 YAML 语法
    if python3 -c "import yaml; yaml.safe_load(open('$PLATFORMS_YAML'))" 2>/dev/null; then
        echo -e "${GREEN}✅ platforms.yaml 语法正确${NC}"
    else
        echo -e "${RED}❌ platforms.yaml 语法错误${NC}"
    fi
else
    echo -e "${RED}❌ platforms.yaml 不存在${NC}"
fi

OPENAPI_EXAMPLE="/Users/cui/Workspaces/croupier/croupier/services/agent/etc/openapi.example.yaml"
if [ -f "$OPENAPI_EXAMPLE" ]; then
    echo -e "${GREEN}✅ openapi.example.yaml 存在${NC}"
else
    echo -e "${YELLOW}⚠️  openapi.example.yaml 不存在${NC}"
fi

echo ""

# ============================================================================
# 检查 5: Server API 测试
# ============================================================================
echo "📋 检查 5: Server API 测试"
echo "----------------------------------------"

# 测试 Server 健康检查
echo "测试 Server: curl http://localhost:18780/health"
HEALTH_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:18780/health 2>/dev/null)
HTTP_CODE=$(echo "$HEALTH_RESPONSE" | tail -1)
BODY=$(echo "$HEALTH_RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    echo -e "${GREEN}✅ Server API 响应正常${NC}"
    echo "   响应: $BODY"
else
    echo -e "${RED}❌ Server API 返回 $HTTP_CODE${NC}"
fi

echo ""

# ============================================================================
# 检查 6: 函数注册测试
# ============================================================================
echo "📋 检查 6: 函数注册状态"
echo "----------------------------------------"

# 尝试获取函数列表（不需要认证的端点）
echo "测试函数列表 API..."
FUNCTIONS_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:18780/api/v1/functions/list 2>&1)
HTTP_CODE=$(echo "$FUNCTIONS_RESPONSE" | tail -1)

echo "HTTP 状态码: $HTTP_CODE"

if [ "$HTTP_CODE" = "200" ]; then
    echo -e "${GREEN}✅ 函数列表 API 可访问${NC}"

    # 尝试解析 JSON
    FUNCTION_COUNT=$(echo "$FUNCTIONS_RESPONSE" | sed '$d' | jq '.items | length' 2>/dev/null)
    if [ -n "$FUNCTION_COUNT" ]; then
        if [ "$FUNCTION_COUNT" -gt 0 ]; then
            echo -e "${GREEN}✅ 已注册 $FUNCTION_COUNT 个函数${NC}"
            echo ""
            echo "函数列表（前 5 个）："
            echo "$FUNCTIONS_RESPONSE" | sed '$d' | jq '.items[:5].id' 2>/dev/null
        else
            echo -e "${YELLOW}⚠️  函数列表为空（0 个函数）${NC}"
        fi
    else
        echo -e "${YELLOW}⚠️  无法解析函数列表响应${NC}"
    fi
elif [ "$HTTP_CODE" = "401" ]; then
    echo -e "${YELLOW}⚠️  需要认证（这是正常的）${NC}"
    echo ""
    echo "尝试使用 descriptors 端点..."
    DESCRIPTORS_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:18780/api/v1/functions/descriptors 2>&1)
    DESCRIPTORS_CODE=$(echo "$DESCRIPTORS_RESPONSE" | tail -1)

    if [ "$DESCRIPTORS_CODE" = "200" ]; then
        echo -e "${GREEN}✅ Descriptors API 可访问${NC}"
        DESCRIPTORS_COUNT=$(echo "$DESCRIPTORS_RESPONSE" | sed '$d' | jq 'length' 2>/dev/null)
        echo -e "${GREEN}✅ 已注册 $DESCRIPTORS_COUNT 个函数${NC}"
        echo ""
        echo "函数列表（前 5 个）："
        echo "$DESCRIPTORS_RESPONSE" | sed '$d' | jq '.[:5].id' 2>/dev/null
    fi
else
    echo -e "${RED}❌ 函数列表 API 返回 $HTTP_CODE${NC}"
    echo "响应: $(echo "$FUNCTIONS_RESPONSE" | sed '$d')"
fi

echo ""

# ============================================================================
# 检查 7: Agent 日志
# ============================================================================
echo "📋 检查 7: Agent 日志（最后 20 行）"
echo "----------------------------------------"

if pgrep -f "croupier-agent" > /dev/null; then
    # 检查是否有日志文件
    if [ -f "/var/log/croupier-agent.log" ]; then
        echo "最近的 Agent 日志："
        echo "----------------------------------------"
        tail -20 /var/log/croupier-agent.log | grep -E "INFO|ERROR|WARN|platform loaded|registering" || echo "（无相关日志）"
    else
        echo -e "${YELLOW}⚠️  日志文件不存在 (/var/log/croupier-agent.log)${NC}"
        echo ""
        echo "提示：Agent 日志可能输出到标准输出或 VS Code DEBUG CONSOLE"
    fi
else
    echo -e "${YELLOW}⚠️  Agent 未运行，无法查看日志${NC}"
fi

echo ""

# ============================================================================
# 总结
# ============================================================================
echo "=========================================="
echo "📊 诊断总结"
echo "=========================================="

# 统计问题
ISSUES=0

if ! pgrep -f "croupier-server" > /dev/null; then
    echo -e "${RED}❌ Server 未运行${NC}"
    ISSUES=$((ISSUES + 1))
fi

if ! pgrep -f "croupier-agent" > /dev/null; then
    echo -e "${RED}❌ Agent 未运行${NC}"
    ISSUES=$((ISSUES + 1))
fi

if ! lsof -i :18780 > /dev/null 2>&1; then
    echo -e "${RED}❌ Server HTTP (18780) 未监听${NC}"
    ISSUES=$((ISSUES + 1))
fi

echo ""

if [ $ISSUES -eq 0 ]; then
    echo -e "${GREEN}✅ 所有关键组件正在运行${NC}"
    echo ""
    echo "📝 下一步："
    echo "1. 打开 Dashboard: http://localhost:8000"
    echo "2. 进入：游戏管理 → 函数管理 → 函数目录"
    echo "3. 查看已注册的函数"
else
    echo -e "${RED}发现 $ISSUES 个问题，需要先解决${NC}"
    echo ""
    echo "📝 建议操作："
    echo "1. 确保 Server 正在运行"
    echo "2. 确保 Agent 正在运行"
    echo "3. 检查配置文件是否存在"
fi

echo ""
echo "=========================================="
echo "🔗 快速链接"
echo "=========================================="
echo "Dashboard:  http://localhost:8000"
echo "函数目录:    http://localhost:8000/game/functions/catalog"
echo "Server API:   http://localhost:18780"
echo "Agent HTTP:   http://localhost:18888"
echo "=========================================="
