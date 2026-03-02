#!/bin/bash
# 打包 Player 函数 Pack (OpenAPI 3.0.3 格式)

set -e

PACK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACK_NAME="player.pack.tgz"
PACK_FILE="${PACK_DIR}/${PACK_NAME}"
OPENAPI_FILE="${PACK_DIR}/openapi.yaml"

echo "📦 打包 Player 函数 Pack (OpenAPI 3.0.3)..."
cd "${PACK_DIR}"

# 检查 openapi.yaml 是否存在
if [ ! -f "$OPENAPI_FILE" ]; then
  echo "❌ 错误: openapi.yaml 不存在"
  exit 1
fi

# 验证 openapi.yaml (可选，需要 swagger-cli)
if command -v swagger-cli &> /dev/null; then
  echo "🔍 验证 OpenAPI 规范..."
  if swagger-cli validate "$OPENAPI_FILE" 2>&1 | grep -q "No errors"; then
    echo "✅ OpenAPI 规范验证通过"
  else
    echo "⚠️  OpenAPI 规范有错误，继续打包..."
  fi
fi

# 创建临时目录
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# 复制 openapi.yaml
cp "$OPENAPI_FILE" "$TEMP_DIR/openapi.yaml"

# 创建 pack
echo "📦 创建 pack 文件..."
tar -czf "${PACK_FILE}" -C "$TEMP_DIR" openapi.yaml

if [ $? -eq 0 ]; then
  echo "✅ 打包成功: ${PACK_FILE}"
  echo "📋 文件大小: $(du -h "${PACK_FILE}" | cut -f1)"
  echo ""
  echo "🚀 导入命令 (Pack API):"
  echo "curl -X POST http://localhost:18780/api/v1/packs/import \\"
  echo "  -F \"pack=@${PACK_FILE}\" \\"
  echo "  -H \"X-Game-ID: your-game-id\""
  echo ""
  echo "🚀 导入命令 (OpenAPI Import API):"
  echo "curl -X POST http://localhost:18780/api/v1/functions/_import \\"
  echo "  -H \"Content-Type: application/json\" \\"
  echo "  -H \"X-Game-ID: your-game-id\" \\"
  echo "  --data-binary @openapi.yaml"
else
  echo "❌ 打包失败"
  exit 1
fi
