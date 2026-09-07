package installation

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errSecretMarshalBoom = errors.New("secret marshal boom")

// failSecondMarshal 使 marshalJSON 的第二次调用（即 SecretRefs 序列化）失败，
// 第一次调用（Config 序列化）保持真实行为。
func failSecondMarshal(t *testing.T) {
	t.Helper()
	orig := marshalJSON
	calls := 0
	marshalJSON = func(v any) (model.JSON, error) {
		calls++
		if calls >= 2 {
			return nil, errSecretMarshalBoom
		}
		return orig(v)
	}
	t.Cleanup(func() { marshalJSON = orig })
}

func TestService_Install_SecretRefsMarshalError(t *testing.T) {
	svc, _ := newInstallService(t)
	failSecondMarshal(t)

	item, err := svc.Install(context.Background(), InstallRequest{
		ExtensionID: "ext-secret-marshal",
		Config:      map[string]any{"k": "v"},
		SecretRefs:  map[string]string{"secret": "ref"},
		Operator:    "admin",
	})

	require.Error(t, err)
	assert.Nil(t, item)
	assert.ErrorIs(t, err, errSecretMarshalBoom)
}

func TestService_UpdateConfig_SecretRefsMarshalError(t *testing.T) {
	svc, _ := newInstallService(t)
	id := mustInstall(t, svc)
	failSecondMarshal(t)

	err := svc.UpdateConfig(context.Background(), id, map[string]any{"k": "v"}, map[string]string{"secret": "ref"}, "admin")

	require.Error(t, err)
	assert.ErrorIs(t, err, errSecretMarshalBoom)
}
