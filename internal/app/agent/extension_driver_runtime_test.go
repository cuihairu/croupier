package agent

import (
	"context"
	"fmt"
	"testing"

	externalv1 "github.com/cuihairu/croupier/pkg/pb/croupier/external/v1"
	"google.golang.org/protobuf/proto"
)

type counterDriver struct {
	name       string
	initN      int
	reloadN    int
	stopN      int
	invokeN    int
	failOnInit bool
}

func (d *counterDriver) Name() string { return d.name }

func (d *counterDriver) Init(ctx context.Context, installation RuntimeInstallation) error {
	d.initN++
	if d.failOnInit {
		return fmt.Errorf("init failed")
	}
	return nil
}

func (d *counterDriver) Reload(ctx context.Context, installation RuntimeInstallation) error {
	d.reloadN++
	return nil
}

func (d *counterDriver) Stop(ctx context.Context, installationID uint) error {
	d.stopN++
	return nil
}

func (d *counterDriver) Invoke(ctx context.Context, functionID string, payload []byte) ([]byte, error) {
	d.invokeN++
	return append([]byte(nil), payload...), nil
}

func TestExtensionDriverRuntimeSyncLifecycle(t *testing.T) {
	rt := NewExtensionDriverRuntime()
	fake := &counterDriver{name: "workflow-driver"}
	rt.RegisterDriver(fake)

	snap1 := ExtensionRuntimeSnapshot{
		Installations: []RuntimeInstallation{
			{
				InstallationID: 1,
				Bindings: []RuntimeBinding{
					{BindingType: "function", BindingKey: "f.a"},
				},
			},
		},
	}
	res1, err := rt.Sync(context.Background(), snap1)
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}
	if fake.initN != 1 || fake.reloadN != 0 || fake.stopN != 0 {
		t.Fatalf("unexpected lifecycle counters after first sync: init=%d reload=%d stop=%d", fake.initN, fake.reloadN, fake.stopN)
	}
	if res1.Initialized != 1 || res1.Reloaded != 0 || res1.Stopped != 0 || res1.Failed != 0 {
		t.Fatalf("unexpected first sync result: %+v", res1)
	}

	res2, err := rt.Sync(context.Background(), snap1)
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}
	if fake.initN != 1 || fake.reloadN != 1 || fake.stopN != 0 {
		t.Fatalf("unexpected lifecycle counters after second sync: init=%d reload=%d stop=%d", fake.initN, fake.reloadN, fake.stopN)
	}
	if res2.Initialized != 0 || res2.Reloaded != 1 || res2.Stopped != 0 || res2.Failed != 0 {
		t.Fatalf("unexpected second sync result: %+v", res2)
	}

	res3, err := rt.Sync(context.Background(), ExtensionRuntimeSnapshot{Installations: []RuntimeInstallation{}})
	if err != nil {
		t.Fatalf("third sync failed: %v", err)
	}
	if fake.stopN != 1 {
		t.Fatalf("expected one stop call, got %d", fake.stopN)
	}
	if res3.Stopped != 1 {
		t.Fatalf("unexpected third sync result: %+v", res3)
	}
}

func TestExtensionDriverRuntimeSyncUnknownDriver(t *testing.T) {
	rt := NewExtensionDriverRuntime()
	_, err := rt.Sync(context.Background(), ExtensionRuntimeSnapshot{
		Installations: []RuntimeInstallation{
			{
				InstallationID: 9,
				Bindings: []RuntimeBinding{
					{BindingType: "function", Spec: map[string]any{"driver": "unknown-driver"}},
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected unknown driver sync error")
	}
}

func TestResolveDriverNames(t *testing.T) {
	names := resolveDriverNames(RuntimeInstallation{
		Bindings: []RuntimeBinding{
			{BindingType: "openapi"},
			{BindingType: "capability"},
			{BindingType: "webhook"},
		},
	})
	seen := map[string]bool{}
	for _, name := range names {
		seen[name] = true
	}
	if !seen["openapi-driver"] || !seen["workflow-driver"] || !seen["webhook-driver"] {
		t.Fatalf("unexpected driver names: %+v", names)
	}
}

func TestExtensionDriverRuntimeInvoke(t *testing.T) {
	rt := NewExtensionDriverRuntime()
	fake := &counterDriver{name: "workflow-driver"}
	rt.RegisterDriver(fake)

	out, err := rt.Invoke(context.Background(), "workflow-driver", "f.a", []byte(`{"ok":1}`))
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}
	if string(out) != `{"ok":1}` {
		t.Fatalf("unexpected invoke output: %s", string(out))
	}
	if fake.invokeN != 1 {
		t.Fatalf("expected invoke count=1, got %d", fake.invokeN)
	}
}

func TestOpenAPIDriverInvokeRaw(t *testing.T) {
	rt := NewExtensionDriverRuntime()
	rt.SetOpenAPICaller(func(ctx context.Context, provider, method string, request []byte) ([]byte, error) {
		if provider != "onepanel" || method != "list_apps" {
			t.Fatalf("unexpected provider/method: %s/%s", provider, method)
		}
		if string(request) != `{"k":"v"}` {
			t.Fatalf("unexpected request: %s", string(request))
		}
		return []byte(`{"ok":true}`), nil
	})
	out, err := rt.Invoke(context.Background(), "openapi-driver", "external.onepanel.list_apps", []byte(`{"k":"v"}`))
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}
	if string(out) != `{"ok":true}` {
		t.Fatalf("unexpected response: %s", string(out))
	}
}

func TestOpenAPIDriverInvokeProto(t *testing.T) {
	rt := NewExtensionDriverRuntime()
	rt.SetOpenAPICaller(func(ctx context.Context, provider, method string, request []byte) ([]byte, error) {
		if provider != "quicksdk" || method != "sync" {
			t.Fatalf("unexpected provider/method: %s/%s", provider, method)
		}
		return []byte(`{"status":"ok"}`), nil
	})
	raw, err := proto.Marshal(&externalv1.CallPlatformRequest{
		Platform: "quicksdk",
		Method:   "sync",
		Request:  []byte(`{"scope":"full"}`),
	})
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}
	out, err := rt.Invoke(context.Background(), "openapi-driver", "external.onepanel.placeholder", raw)
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}
	resp := &externalv1.CallPlatformResponse{}
	if err := proto.Unmarshal(out, resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.GetError() != "" {
		t.Fatalf("unexpected wrapped error: %s", resp.GetError())
	}
	if string(resp.GetResponse()) != `{"status":"ok"}` {
		t.Fatalf("unexpected response payload: %s", string(resp.GetResponse()))
	}
}
