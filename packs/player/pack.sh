#!/bin/bash
# 打包 Player 函数 Pack

PACK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACK_NAME="player.tgz"
PACK_FILE="${PACK_DIR}/${PACK_NAME}"

echo "📦 打包 Player 函数 Pack..."
cd "${PACK_DIR}"
tar -czf "${PACK_NAME}" descriptors/ ui/

if [ $? -eq 0 ]; then
  echo "✅ 打包成功: ${PACK_FILE}"
  echo "📋 文件大小: $(du -h "${PACK_FILE}" | cut -f1)"
  echo ""
  echo "🚀 导入命令:"
  echo "curl -X POST http://localhost:8080/api/v1/packs/import \\"
  echo "  -F \"pack=@${PACK_FILE}\" \\"
  echo "  -H \"X-Game-ID: your-game-id\""
else
  echo "❌ 打包失败"
  exit 1
fi
