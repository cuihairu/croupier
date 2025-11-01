#!/bin/bash

# 游戏后台角色权限配置应用脚本
# 用于快速部署预设的角色权限体系

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIGS_DIR="$SCRIPT_DIR/../configs"

echo "🎮 游戏后台角色权限配置工具"
echo "=================================="

# 检查配置文件是否存在
if [ ! -f "$CONFIGS_DIR/rbac.game-roles.json" ]; then
    echo "❌ 错误: 找不到 rbac.game-roles.json 配置文件"
    exit 1
fi

if [ ! -f "$CONFIGS_DIR/users.game-roles.json" ]; then
    echo "❌ 错误: 找不到 users.game-roles.json 配置文件"
    exit 1
fi

# 显示可用操作
echo "请选择操作:"
echo "1) 应用游戏角色权限配置 (备份原配置)"
echo "2) 仅查看角色权限配置"
echo "3) 恢复原始配置"
echo "4) 验证配置文件格式"
echo "5) 退出"

read -p "请输入选择 (1-5): " choice

case $choice in
    1)
        echo "📋 开始应用游戏角色权限配置..."

        # 备份原配置
        if [ -f "$CONFIGS_DIR/rbac.json" ]; then
            cp "$CONFIGS_DIR/rbac.json" "$CONFIGS_DIR/rbac.json.backup.$(date +%Y%m%d_%H%M%S)"
            echo "✅ 已备份原 rbac.json 配置"
        fi

        if [ -f "$CONFIGS_DIR/users.json" ]; then
            cp "$CONFIGS_DIR/users.json" "$CONFIGS_DIR/users.json.backup.$(date +%Y%m%d_%H%M%S)"
            echo "✅ 已备份原 users.json 配置"
        fi

        # 应用新配置
        cp "$CONFIGS_DIR/rbac.game-roles.json" "$CONFIGS_DIR/rbac.json"
        cp "$CONFIGS_DIR/users.game-roles.json" "$CONFIGS_DIR/users.json"

        echo "✅ 完整游戏团队角色权限配置已应用成功!"
        echo "📝 包含以下23个角色:"
        echo ""
        echo "🏢 管理层 (4个):"
        echo "   - super_admin (超级管理员)"
        echo "   - admin (系统管理员)"
        echo "   - project_manager (项目经理)"
        echo "   - producer (制作人)"
        echo ""
        echo "💻 技术团队 (5个):"
        echo "   - tech_lead (技术负责人)"
        echo "   - senior_developer (高级开发工程师)"
        echo "   - developer (开发工程师)"
        echo "   - tester (测试工程师)"
        echo "   - ops (运维工程师)"
        echo ""
        echo "🎨 设计团队 (5个):"
        echo "   - game_designer (游戏策划/设计师)"
        echo "   - level_designer (关卡策划)"
        echo "   - system_designer (系统策划)"
        echo "   - numerical_designer (数值策划)"
        echo "   - ui_designer (UI设计师)"
        echo ""
        echo "📈 运营团队 (4个):"
        echo "   - operator (游戏运营)"
        echo "   - marketing (市场营销)"
        echo "   - community (社区管理)"
        echo "   - content_manager (内容管理员)"
        echo ""
        echo "📊 数据分析团队 (3个):"
        echo "   - analyst (数据分析师)"
        echo "   - bi_analyst (商业智能分析师)"
        echo "   - user_researcher (用户研究员)"
        echo ""
        echo "🎧 客服团队 (3个):"
        echo "   - support_manager (客服主管)"
        echo "   - senior_support (高级客服)"
        echo "   - support (客服人员)"
        echo ""
        echo "⚡ 特殊角色 (4个):"
        echo "   - gm (游戏管理员)"
        echo "   - bot_operator (托/机器人操作员)"
        echo "   - security (安全专员)"
        echo "   - auditor (审计员)"
        echo ""
        echo "⚠️  请重启服务以使配置生效"
        ;;

    2)
        echo "📋 角色权限配置预览:"
        echo ""
        echo "=== RBAC权限配置 ==="
        cat "$CONFIGS_DIR/rbac.game-roles.json" | jq '.'
        echo ""
        echo "=== 用户角色配置 ==="
        cat "$CONFIGS_DIR/users.game-roles.json" | jq '[.[] | {username: .username, roles: .roles, description: .description}]'
        ;;

    3)
        echo "🔄 恢复原始配置..."
        RBAC_BACKUP=$(ls -t "$CONFIGS_DIR"/rbac.json.backup.* 2>/dev/null | head -1)
        USERS_BACKUP=$(ls -t "$CONFIGS_DIR"/users.json.backup.* 2>/dev/null | head -1)

        if [ -n "$RBAC_BACKUP" ]; then
            cp "$RBAC_BACKUP" "$CONFIGS_DIR/rbac.json"
            echo "✅ 已恢复 rbac.json 配置"
        fi

        if [ -n "$USERS_BACKUP" ]; then
            cp "$USERS_BACKUP" "$CONFIGS_DIR/users.json"
            echo "✅ 已恢复 users.json 配置"
        fi

        echo "✅ 配置恢复完成"
        ;;

    4)
        echo "🔍 验证配置文件格式..."

        # 验证JSON格式
        if jq empty "$CONFIGS_DIR/rbac.game-roles.json" 2>/dev/null; then
            echo "✅ rbac.game-roles.json 格式正确"
        else
            echo "❌ rbac.game-roles.json 格式错误"
        fi

        if jq empty "$CONFIGS_DIR/users.game-roles.json" 2>/dev/null; then
            echo "✅ users.game-roles.json 格式正确"
        else
            echo "❌ users.game-roles.json 格式错误"
        fi

        # 验证权限结构
        ROLES=$(jq -r '.allow | keys[]' "$CONFIGS_DIR/rbac.game-roles.json" | grep '^role:' | wc -l)
        USERS=$(jq -r '.[].roles[]' "$CONFIGS_DIR/users.game-roles.json" | sort -u | wc -l)

        echo "📊 配置统计:"
        echo "   - 定义角色数: $ROLES"
        echo "   - 用户角色数: $USERS"
        echo "   - 涵盖团队: 管理层、技术、设计、运营、数据分析、客服、特殊角色"
        echo "   - 权限域数: 15个 (system, user, game, player, function, job, audit, monitor, data, design, numerical, level, content, marketing, community, event, announcement, mail, ban, reward, gm, bot, security, economy, support)"
        ;;

    5)
        echo "👋 退出配置工具"
        exit 0
        ;;

    *)
        echo "❌ 无效选择，请重新运行脚本"
        exit 1
        ;;
esac

echo ""
echo "💡 更多信息请查看: docs/complete-game-roles-design.md"