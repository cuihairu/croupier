#!/usr/bin/env bash
# 本地 protobuf 生成（脱离 buf registry）：
#   - Go:    protoc-gen-go v1.36.11（go install，对齐 protobuf 4.25.x API）
#     两份输出：主仓库 pkg/pb（go_package 原样）与 Go SDK 独立 module
#     sdks/go/pkg/pb（go_package 路径前缀改写到 sdks/go，;别名 后缀保留）。
#     漏掉后者会让 sdks/go 与 proto 漂移，CI「Go SDK」在 unknown field 上构建失败。
#   - Python/C++: protoc v34.1 内置生成器（python 运行时 6.33.5 / cpp 运行时
#     由 CMake find_package 决定，均向后兼容 gencode）
# 依赖：protoc(34.x)、protoc-gen-go 在 PATH；缺失时给出安装指引并退出。
set -euo pipefail
cd "$(dirname "$0")/.."

command -v protoc >/dev/null || { echo "缺少 protoc：从 https://github.com/protocolbuffers/protobuf/releases 安装 34.x"; exit 1; }
# Python/C++ gencode 内嵌生成器版本（头部 "Protobuf C++/Python Version"），
# 必须是 34.x——旧 protoc（如 3.21.x）产出的 gencode 对新运行时是跨大版本
# 降级，CI 编译/运行即炸。版本不符直接退出，宁可不生成也不产出坏 gencode。
protoc --version | grep -qE "libprotoc 34\." || {
  echo "protoc 必须是 34.x（当前 $(protoc --version)）：PYTHON/CPP gencode 与运行时跨大版本不兼容" >&2
  exit 1
}
# WKT include（google/protobuf/*.proto）：protoc 发行包 include/ 目录。
# 解压到系统路径或设置 PROTOC_INCLUDE 后此处自动生效。
PROTOC_INC=""
if [ -n "${PROTOC_INCLUDE:-}" ]; then
  PROTOC_INC="-I${PROTOC_INCLUDE}"
elif [ -d /usr/local/include/google/protobuf ]; then
  PROTOC_INC="-I/usr/local/include"
fi
command -v protoc-gen-go >/dev/null || { echo "缺少 protoc-gen-go：go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11"; exit 1; }

PROTO_VERSION="$(protoc --version)"
echo "[gen] using ${PROTO_VERSION}"

# proto/ 是 buf module 根（proto/buf.yaml），proto 文件内部引用 croupier/... 前缀
cd proto

find . -name '*.proto' | sort > /tmp/gen_proto_files.txt
echo "[gen] $(wc -l < /tmp/gen_proto_files.txt) proto files"

# Go（主仓库 pkg/pb）
protoc -I. ${PROTOC_INC} \
  --plugin=protoc-gen-go="$(command -v protoc-gen-go)" \
  --go_out=../pkg/pb --go_opt=paths=source_relative \
  $(cat /tmp/gen_proto_files.txt)

# Go SDK（独立 module）
#   副本 proto 树里改写 go_package 前缀（保留 ;包名 后缀）——M flag 只影响跨文件
#   import，不会改写嵌入 descriptor 的 go_package，故必须改 option 本身。
GO_PB_PREFIX="github.com/cuihairu/croupier/pkg/pb"
GO_SDK_PB_PREFIX="github.com/cuihairu/croupier/sdks/go/pkg/pb"
GO_SDK_TREE="$(mktemp -d)"
trap 'rm -rf "$GO_SDK_TREE"' EXIT
cp -r . "$GO_SDK_TREE/proto"
while read -r f; do
  rel="${f#./}"
  grep -q "option go_package = \"$GO_PB_PREFIX" "$rel" || {
    echo "[gen] $rel 的 option go_package 前缀不是 $GO_PB_PREFIX，无法生成 Go SDK 副本" >&2
    exit 1
  }
  sed -i "s#option go_package = \"$GO_PB_PREFIX#option go_package = \"$GO_SDK_PB_PREFIX#" \
    "$GO_SDK_TREE/proto/$rel"
done < /tmp/gen_proto_files.txt
protoc -I"$GO_SDK_TREE/proto" ${PROTOC_INC} \
  --plugin=protoc-gen-go="$(command -v protoc-gen-go)" \
  --go_out=../sdks/go/pkg/pb --go_opt=paths=source_relative \
  $(sed 's|^\./||' /tmp/gen_proto_files.txt)

# Python SDK
protoc -I. --python_out=../sdks/python/generated $(cat /tmp/gen_proto_files.txt)

# C++ SDK
protoc -I. --cpp_out=../sdks/cpp/generated $(cat /tmp/gen_proto_files.txt)

echo "[gen] done: pkg/pb, sdks/go/pkg/pb, sdks/python/generated, sdks/cpp/generated"
