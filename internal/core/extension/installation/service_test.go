package installation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewService(t *testing.T) {
	svc := NewService(nil, nil, nil)
	assert.NotNil(t, svc)
}

func TestService_Install_NilRepo(t *testing.T) {
	svc := &Service{}
	item, err := svc.Install(context.Background(), InstallRequest{
		ExtensionID: "ext-1",
	})
	assert.Error(t, err)
	assert.Nil(t, item)
}

func TestService_List_NilRepo(t *testing.T) {
	svc := &Service{}
	items, total, err := svc.List(context.Background(), ListQuery{})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

func TestService_Get_NilRepo(t *testing.T) {
	svc := &Service{}
	item, err := svc.Get(context.Background(), 1)
	assert.Error(t, err)
	assert.Nil(t, item)
}

func TestService_Enable_NilRepo(t *testing.T) {
	svc := &Service{}
	err := svc.Enable(context.Background(), 1, "admin")
	assert.Error(t, err)
}

func TestService_Disable_NilRepo(t *testing.T) {
	svc := &Service{}
	err := svc.Disable(context.Background(), 1, "admin")
	assert.Error(t, err)
}

func TestService_Upgrade_NilRepo(t *testing.T) {
	svc := &Service{}
	err := svc.Upgrade(context.Background(), 1, "2.0", "admin")
	assert.Error(t, err)
}

func TestService_Uninstall_NilRepo(t *testing.T) {
	svc := &Service{}
	err := svc.Uninstall(context.Background(), 1, "admin")
	assert.Error(t, err)
}

func TestService_UpdateConfig_NilRepo(t *testing.T) {
	svc := &Service{}
	err := svc.UpdateConfig(context.Background(), 1, map[string]any{"key": "value"}, nil, "admin")
	assert.Error(t, err)
}

func TestService_AppendEvent_NilRepo(t *testing.T) {
	svc := &Service{}
	err := svc.appendEvent(context.Background(), 1, "test", "info", "test message", "admin", "")
	assert.NoError(t, err)
}

func TestService_ListEvents_NilRepo(t *testing.T) {
	svc := &Service{}
	events, total, err := svc.ListEvents(context.Background(), 1, EventListQuery{})
	assert.NoError(t, err)
	assert.Empty(t, events)
	assert.Equal(t, int64(0), total)
}

func TestService_ListBindings_NilRepo(t *testing.T) {
	svc := &Service{}
	bindings, err := svc.ListBindings(context.Background(), 1)
	assert.NoError(t, err)
	assert.Empty(t, bindings)
}

func TestService_RecordEvent_NilRepo(t *testing.T) {
	svc := &Service{}
	err := svc.RecordEvent(context.Background(), 1, "test", "info", "test message", "admin", "")
	assert.NoError(t, err)
}

func TestMarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		v    any
	}{
		{"nil", nil},
		{"empty map", map[string]any{}},
		{"with values", map[string]any{"key": "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := marshalJSON(tt.v)
			assert.NoError(t, err)
			// marshalJSON always returns a valid JSON object
			assert.NotNil(t, result)
			assert.NotEmpty(t, result)
		})
	}
}

func TestBuildInstallationKey(t *testing.T) {
	tests := []struct {
		name string
		req  InstallRequest
		want string
	}{
		{
			name: "basic",
			req:  InstallRequest{ExtensionID: "ext-1", ScopeType: "game", ScopeID: "game1", TargetType: "server", TargetID: "server1", ReleaseVersion: "1.0"},
			want: "ext-1:game:game1:server:server1:1.0",
		},
		{
			name: "empty values",
			req:  InstallRequest{},
			want: ":::::",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildInstallationKey(tt.req)
			assert.Equal(t, tt.want, got)
		})
	}
}
