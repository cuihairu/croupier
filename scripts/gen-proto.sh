#!/usr/bin/env bash
# 本地 protobuf 生成（脱离 buf registry）：
#   - Go:    protoc-gen-go v1.36.11（go install，对齐 protobuf 4.25.x API）
#     两份输出：主仓库 pkg/pb（go_package 原样）与 Go SDK 独立 module
#     sdks/go/pkg/pb（go_package 路径前缀改写到 sdks/go，;别名 后缀保留）。
#     漏掉后者会让 sdks/go 与 proto 漂移，CI「Go SDK」在 unknown field 上构建失败。
#   - C++:      protoc v34.1 内置生成器（gencode 头 "Protobuf C++ Version: 7.34.1"，
#     与 C++ 运行时 7.34.x 对齐；C++ 运行时支持旧一级 gencode，但新 gencode
#     不能配旧运行时）
#   - Python:   protoc v25.1 内置生成器（pyproject 锁死 protobuf==6.33.5 运行时；
#     Python 运行时**拒绝比自身新的 gencode**——gencode 必须 ≤ 运行时，
#     历史基线即 4.25.1，用 34.x 生成会直接 VersionError）
# 依赖：PROTOC_CPP_BIN / PROTOC_PYTHON_BIN（默认取 PATH 上的 protoc）、
# protoc-gen-go 在 PATH；版本不符直接退出，宁可不生成也不产出坏 gencode。
set -euo pipefail
cd "$(dirname "$0")/.."

PROTOC_CPP_BIN="${PROTOC_CPP_BIN:-$(command -v protoc || true)}"
PROTOC_PYTHON_BIN="${PROTOC_PYTHON_BIN:-$(command -v protoc || true)}"

[ -n "$PROTOC_CPP_BIN" ] || { echo "缺少 protoc(34.x)：https://github.com/protocolbuffers/protobuf/releases"; exit 1; }
[ -n "$PROTOC_PYTHON_BIN" ] || { echo "缺少 protoc(25.x)：https://github.com/protocolbuffers/protobuf/releases"; exit 1; }

# C++ gencode 必须是 34.x
"$PROTOC_CPP_BIN" --version | grep -qE "libprotoc 34\." || {
  echo "C++ 生成器必须是 protoc 34.x（当前 $("$PROTOC_CPP_BIN" --version)）" >&2
  exit 1
}
# Python gencode 必须 25.x（对应 pyproject protobuf==6.33.5 运行时；34.x 产出的
# 7.34.x gencode 会被 6.33.5 运行时 VersionError 拒载，3.21.x 属旧格式漂移）
"$PROTOC_PYTHON_BIN" --version | grep -qE "libprotoc 25\." || {
  echo "Python 生成器必须是 protoc 25.x（当前 $("$PROTOC_PYTHON_BIN" --version)）" >&2
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

echo "[gen] cpp generator: $("$PROTOC_CPP_BIN" --version)"
echo "[gen] python generator: $("$PROTOC_PYTHON_BIN" --version)"

# proto/ 是 buf module 根（proto/buf.yaml），proto 文件内部引用 croupier/... 前缀
cd proto

find . -name '*.proto' | sort > /tmp/gen_proto_files.txt
echo "[gen] $(wc -l < /tmp/gen_proto_files.txt) proto files"

# Go（主仓库 pkg/pb）——gencode 由 protoc-gen-go 决定，protoc 版本只管解析
protoc_bin="$PROTOC_CPP_BIN"
$protoc_bin -I. ${PROTOC_INC} \
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
$protoc_bin -I"$GO_SDK_TREE/proto" ${PROTOC_INC} \
  --plugin=protoc-gen-go="$(command -v protoc-gen-go)" \
  --go_out=../sdks/go/pkg/pb --go_opt=paths=source_relative \
  $(sed 's|^\./||' /tmp/gen_proto_files.txt)

# Python SDK（必须 25.x 生成器，见文件头说明）
"$PROTOC_PYTHON_BIN" -I. ${PROTOC_INC} --python_out=../sdks/python/generated \
  $(cat /tmp/gen_proto_files.txt)

# C++ SDK（必须 34.x 生成器，见文件头说明）
"$PROTOC_CPP_BIN" -I. ${PROTOC_INC} --cpp_out=../sdks/cpp/generated \
  $(cat /tmp/gen_proto_files.txt)

echo "[gen] done: pkg/pb, sdks/go/pkg/pb, sdks/python/generated, sdks/cpp/generated"
