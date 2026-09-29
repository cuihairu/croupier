package service

// 覆盖率巡检第十六轮（wt-api）：contract_service.go canonicalJSONBytes
// 残余翼收口——非法 JSON 原样透传翼（ proposal 摘要对存量坏行不炸不吞）。
// Marshal 失败翼（1752）登记不可达：v 来自 json.Unmarshal 的合法输出
// （map/slice/string/float64/bool/nil），json.Marshal 对这些类型无失败
// 路径（Unmarshal 不产出 NaN/Inf 与信道外类型），不造假用例。

import (
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestCanonicalJSONBytes_InvalidRawPassthrough(t *testing.T) {
	// 空 raw → nil（既有口径）
	assert.Nil(t, canonicalJSONBytes(nil))

	// 非法 JSON → 原样透传（不炸不吞）
	raw := model.JSON(`{"broken":`)
	assert.Equal(t, raw, canonicalJSONBytes(raw))
}
