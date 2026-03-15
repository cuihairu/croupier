package agent

import (
	"context"
	"errors"
	"testing"

	externalv1 "github.com/cuihairu/croupier/pkg/pb/croupier/external/v1"
	"google.golang.org/protobuf/proto"
)

func TestParseExternalFunctionID(t *testing.T) {
	provider, method, ok := parseExternalFunctionID("external.onepanel.install_app")
	if !ok {
		t.Fatalf("expected parse success")
	}
	if provider != "onepanel" || method != "install_app" {
		t.Fatalf("unexpected parse result provider=%s method=%s", provider, method)
	}
}

func TestInvokeExternalPlatformFunctionRawMode(t *testing.T) {
	resp, handled, err := invokeExternalPlatformFunction(context.Background(), "external.onepanel.list_apps", []byte(`{"k":"v"}`),
		func(ctx context.Context, provider, method string, request []byte) ([]byte, error) {
			if provider != "onepanel" || method != "list_apps" {
				t.Fatalf("unexpected provider/method: %s/%s", provider, method)
			}
			if string(request) != `{"k":"v"}` {
				t.Fatalf("unexpected request payload: %s", string(request))
			}
			return []byte(`{"ok":true}`), nil
		},
	)
	if !handled {
		t.Fatalf("expected handled=true")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != `{"ok":true}` {
		t.Fatalf("unexpected response: %s", string(resp))
	}
}

func TestInvokeExternalPlatformFunctionProtoMode(t *testing.T) {
	raw, err := proto.Marshal(&externalv1.CallPlatformRequest{
		Platform: "quicksdk",
		Method:   "day_report",
		Request:  []byte(`{"date":"2026-03-15"}`),
	})
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}
	respRaw, handled, err := invokeExternalPlatformFunction(context.Background(), "external.onepanel.placeholder", raw,
		func(ctx context.Context, provider, method string, request []byte) ([]byte, error) {
			if provider != "quicksdk" || method != "day_report" {
				t.Fatalf("unexpected proto override provider/method: %s/%s", provider, method)
			}
			return []byte(`{"status":"ok"}`), nil
		},
	)
	if !handled {
		t.Fatalf("expected handled=true")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := &externalv1.CallPlatformResponse{}
	if err := proto.Unmarshal(respRaw, resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.GetError() != "" {
		t.Fatalf("unexpected proto response error: %s", resp.GetError())
	}
	if string(resp.GetResponse()) != `{"status":"ok"}` {
		t.Fatalf("unexpected proto response payload: %s", string(resp.GetResponse()))
	}
}

func TestInvokeExternalPlatformFunctionProtoModeErrorWrapped(t *testing.T) {
	raw, err := proto.Marshal(&externalv1.CallPlatformRequest{
		Platform: "quicksdk",
		Method:   "day_report",
		Request:  []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}
	respRaw, handled, err := invokeExternalPlatformFunction(context.Background(), "external.onepanel.placeholder", raw,
		func(ctx context.Context, provider, method string, request []byte) ([]byte, error) {
			return nil, errors.New("platform timeout")
		},
	)
	if !handled {
		t.Fatalf("expected handled=true")
	}
	if err != nil {
		t.Fatalf("unexpected error, proto mode should wrap error into response: %v", err)
	}
	resp := &externalv1.CallPlatformResponse{}
	if err := proto.Unmarshal(respRaw, resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.GetError() == "" {
		t.Fatalf("expected wrapped error in proto response")
	}
}
