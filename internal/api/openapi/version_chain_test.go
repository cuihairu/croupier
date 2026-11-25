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

	// The runtime (SDK-registered) version wins, and it must be valid semver.
	in := s.functionMetaInputForBinding(source("9.9.9"), OpenAPISourceOperation{}, "player.ban",
		reg.FunctionMeta{Version: "1.2.3"})
	assert.Equal(t, "1.2.3", in.Version)

	// When the runtime version is not semver, don't let the downstream gate
	// discard the whole function; fall back to info.version.
	in = s.functionMetaInputForBinding(source("9.9.9"), OpenAPISourceOperation{}, "player.ban",
		reg.FunctionMeta{Version: "nightly"})
	assert.Equal(t, "9.9.9", in.Version)

	// Neither side is semver -> default 1.0.0.
	in = s.functionMetaInputForBinding(source("2026-09"), OpenAPISourceOperation{}, "player.ban",
		reg.FunctionMeta{Version: ""})
	assert.Equal(t, "1.0.0", in.Version)

	// The unbound assembly path normalizes the same way.
	in = s.functionMetaInputForOperation(source("2026-09"), OpenAPISourceOperation{}, nil, "player.ban")
	assert.Equal(t, "1.0.0", in.Version)
	in = s.functionMetaInputForOperation(source("v3.2.1"), OpenAPISourceOperation{}, nil, "player.ban")
	assert.Equal(t, "v3.2.1", in.Version)
}
