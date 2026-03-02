#!/bin/bash
# Croupier 快速启动脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目根目录
PROJECT_ROOT="/Users/cui/Workspaces/croupier/croupier"
DASHBOARD_ROOT="/Users/cui/Workspaces/croupier/croupier-dashboard"
PACK_DIR="${PROJECT_ROOT}/packs/player"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Croupier 快速启动脚本${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 1. 检查后端服务
echo -e "${YELLOW}[1/4] 检查后端服务...${NC}"
if curl -s http://localhost:18780/healthz > /dev/null 2>&1; then
    echo -e "${GREEN}✅ 后端服务已运行 (http://localhost:18780)${NC}"
else
    echo -e "${RED}❌ 后端服务未运行${NC}"
    echo -e "${YELLOW}请先启动后端服务:${NC}"
    echo "  cd ${PROJECT_ROOT}/services/server"
    echo "  make build"
    echo "  ./bin/croupier-server --config services/server/etc/server.yaml"
    echo ""
    read -p "按回车键继续（假设您会手动启动后端）..."
fi

# 2. 检查前端服务
echo -e "${YELLOW}[2/4] 检查前端服务...${NC}"
if curl -s http://localhost:8000 > /dev/null 2>&1; then
    echo -e "${GREEN}✅ 前端服务已运行 (http://localhost:8000)${NC}"
else
    echo -e "${RED}❌ 前端服务未运行${NC}"
    echo -e "${YELLOW}请先启动前端服务:${NC}"
    echo "  cd ${DASHBOARD_ROOT}"
    echo "  pnpm dev"
    echo ""
    read -p "按回车键继续（假设您会手动启动前端）..."
fi

# 3. 打包示例 Pack
echo ""
echo -e "${YELLOW}[3/4] 打包示例函数 Pack...${NC}"
if [ -f "${PACK_DIR}/pack.sh" ]; then
    cd "${PACK_DIR}"
    chmod +x pack.sh
    ./pack.sh
    echo -e "${GREEN}✅ Pack 打包完成: ${PACK_DIR}/player.tgz${NC}"
else
    echo -e "${RED}❌ 找不到打包脚本: ${PACK_DIR}/pack.sh${NC}"
fi

# 4. 显示操作指南
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}🎉 启动准备完成！${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "${YELLOW}📖 接下来的操作步骤:${NC}"
echo ""
echo -e "${GREEN}1. 访问前端界面:${NC}"
echo "   http://localhost:8000"
echo ""
echo -e "${GREEN}2. 登录系统${NC}"
echo "   （使用配置文件中的账号密码）"
echo ""
echo -e "${GREEN}3. 打开 Pack 管理页面:${NC}"
echo "   http://localhost:8000/game/functions/packs"
echo "   或在菜单中: Game Management → Function Packs"
echo ""
echo -e "${GREEN}4. 上传示例 Pack:${NC}"
echo "   点击「上传 Pack」按钮"
echo "   选择文件: ${PACK_DIR}/player.tgz"
echo "   选择游戏环境后点击确认"
echo ""
echo -e "${GREEN}5. 查看函数目录:${NC}"
echo "   http://localhost:8000/game/functions/catalog"
echo "   找到「获取玩家信息」函数"
echo ""
echo -e "${GREEN}6. 调用函数:${NC}"
echo "   点击「▶️ 调用函数」按钮"
echo "   自动跳转到工作台: /game/player/get"
echo "   填写参数并调用"
echo ""
echo -e "${YELLOW}📚 完整文档:${NC}"
echo "   ${PROJECT_ROOT}/OPERATION_GUIDE.md"
echo "   ${PACK_DIR}/README.md"
echo ""
echo -e "${YELLOW}🔧 常用命令:${NC}"
echo "   查看已导入函数: curl http://localhost:18780/api/v1/functions/descriptors"
echo "   查看UI配置: curl http://localhost:18780/api/v1/functions/player.get/ui"
echo "   查看权限: curl http://localhost:18780/api/v1/functions/player.get/permissions"
echo ""
echo -e "${BLUE}========================================${NC}"
