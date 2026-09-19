package openapi

import (
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
)

func TestFunctionMetaInputVersionChain(t *testing.T) {
	s := &Service{}
	source := func(infoVersion string) *model.OpenAPISource {
		return &model.OpenAPISource{InfoVersion: infoVersion}
	}

	// 运行时（SDK 注册）版本优先，且必须 semver 合法。
	in := s.functionMetaInputForBinding(source("9.9.9"), OpenAPISourceOperation{}, "player.ban",
		reg.FunctionMeta{Version: "1.2.3"})
	assert.Equal(t, "1.2.3", in.Version)

	// 运行时版本非 semver 时不让整函数被下游门槛丢弃，回退 info.version。
	in = s.functionMetaInputForBinding(source("9.9.9"), OpenAPISourceOperation{}, "player.ban",
		reg.FunctionMeta{Version: "nightly"})
	assert.Equal(t, "9.9.9", in.Version)

	// 双非 semver → 默认 1.0.0。
	in = s.functionMetaInputForBinding(source("2026-09"), OpenAPISourceOperation{}, "player.ban",
		reg.FunctionMeta{Version: ""})
	assert.Equal(t, "1.0.0", in.Version)

	// unbound 组装路径同样归一。
	in = s.functionMetaInputForOperation(source("2026-09"), OpenAPISourceOperation{}, nil, "player.ban")
	assert.Equal(t, "1.0.0", in.Version)
	in = s.functionMetaInputForOperation(source("v3.2.1"), OpenAPISourceOperation{}, nil, "player.ban")
	assert.Equal(t, "v3.2.1", in.Version)
}
