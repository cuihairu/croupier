package quicksdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/provider"
)

func TestNewProvider(t *testing.T) {
	p := NewProvider(nil)
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.Name() != "quicksdk" {
		t.Errorf("expected name 'quicksdk', got %q", p.Name())
	}
	if p.IsEnabled() {
		t.Error("expected provider to be disabled initially")
	}
}

func TestProvider_Init_MissingOpenID(t *testing.T) {
	p := NewProvider(nil)
	err := p.Init(context.Background(), provider.ProviderConfig{
		Config: map[string]interface{}{
			"open_key": "key",
		},
	})
	if err == nil {
		t.Error("expected error for missing open_id")
	}
}

func TestProvider_Init_MissingOpenKey(t *testing.T) {
	p := NewProvider(nil)
	err := p.Init(context.Background(), provider.ProviderConfig{
		Config: map[string]interface{}{
			"open_id": "id",
		},
	})
	if err == nil {
		t.Error("expected error for missing open_key")
	}
}

func TestProvider_Init_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewProvider(nil)
	err := p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"open_id":             "test-id",
			"open_key":            "test-key",
			"api_base_url":        srv.URL,
			"enable_cache":        true,
			"cache_duration":      float64(60),
			"requests_per_minute": float64(100),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.IsEnabled() {
		t.Error("expected provider to be enabled")
	}
	if p.client == nil {
		t.Error("expected client to be initialized")
	}
}

func TestProvider_Init_WithRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewProvider(nil)
	err := p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"open_id":      "test-id",
			"open_key":     "test-key",
			"api_base_url": srv.URL,
		},
		RateLimit: &provider.RateLimitConfig{
			RequestsPerMinute: 60,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.client.rateLimiter == nil {
		t.Error("expected rate limiter to be set")
	}
}

func TestProvider_SupportedMethods(t *testing.T) {
	p := NewProvider(nil)
	methods := p.SupportedMethods()

	expected := map[string]bool{
		"channel_list":         true,
		"server_list":          true,
		"product_list":         true,
		"role_info":            true,
		"order_list":           true,
		"day_report":           true,
		"day_hour_report":      true,
		"user_live":            true,
		"channel_days_report":  true,
		"channel_report":       true,
		"ad_report":            true,
		"media_app_list":       true,
		"ad_plan_group_list":   true,
		"package_version_list": true,
		"ad_pages_list":        true,
		"create_ad_plan":       true,
		"update_ad_plan":       true,
		"ad_plan_list":         true,
		"user_lost_list":       true,
		"push_message":         true,
	}

	if len(methods) != len(expected) {
		t.Errorf("expected %d methods, got %d", len(expected), len(methods))
	}

	for _, m := range methods {
		if !expected[m] {
			t.Errorf("unexpected method: %q", m)
		}
	}
}

func TestProvider_Call_NotInitialized(t *testing.T) {
	p := NewProvider(nil)
	_, err := p.Call(context.Background(), "channel_list", nil)
	if err == nil {
		t.Error("expected error for uninitialized client")
	}
}

func TestProvider_Call_UnknownMethod(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewProvider(nil)
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"open_id":      "test-id",
			"open_key":     "test-key",
			"api_base_url": srv.URL,
		},
	})

	_, err := p.Call(context.Background(), "nonexistent_method", nil)
	if err == nil {
		t.Error("expected error for unknown method")
	}

	var mnse *provider.MethodNotSupportedError
	if ok := isErrorAs(err, &mnse); !ok {
		t.Errorf("expected MethodNotSupportedError, got %T: %v", err, err)
	}
}

func TestProvider_Call_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewProvider(nil)
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"open_id":      "test-id",
			"open_key":     "test-key",
			"api_base_url": srv.URL,
		},
	})

	_, err := p.Call(context.Background(), "channel_list", []byte("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestProvider_Call_AllMethods(t *testing.T) {
	// Mock server that returns valid responses for all endpoints
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data json.RawMessage
		// role_info expects {"total":N,"list":[...]} format
		if r.URL.Path == "/open/roleInfo" {
			data = json.RawMessage(`{"total":0,"list":[]}`)
		} else {
			data = json.RawMessage(`[]`)
		}
		resp := Response{
			Status:  true,
			Message: "ok",
			Data:    data,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewProvider(nil)
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"open_id":      "test-id",
			"open_key":     "test-key",
			"api_base_url": srv.URL,
		},
	})

	methods := []struct {
		name string
		req  []byte
	}{
		{"channel_list", []byte(`{"product_code":"p1"}`)},
		{"server_list", []byte(`{"product_code":"p1"}`)},
		{"product_list", nil},
		{"role_info", []byte(`{"product_code":"p1","server_name":"s1"}`)},
		{"order_list", []byte(`{"product_code":"p1"}`)},
		{"day_report", []byte(`{"product_code":"p1"}`)},
		{"day_hour_report", []byte(`{"product_code":"p1"}`)},
		{"user_live", []byte(`{"product_code":"p1"}`)},
		{"channel_days_report", []byte(`{"product_code":"p1"}`)},
		{"channel_report", []byte(`{"product_code":"p1"}`)},
		{"ad_report", []byte(`{"product_code":"p1","start_date":"2024-01-01","end_date":"2024-01-31"}`)},
		{"media_app_list", []byte(`{"media_type":"Toutiao"}`)},
		{"ad_plan_group_list", []byte(`{"product_code":"p1"}`)},
		{"package_version_list", []byte(`{"product_code":"p1"}`)},
		{"ad_pages_list", []byte(`{"product_code":"p1"}`)},
		{"user_lost_list", []byte(`{"product_code":"p1"}`)},
	}

	for _, m := range methods {
		t.Run(m.name, func(t *testing.T) {
			result, err := p.Call(context.Background(), m.name, m.req)
			if err != nil {
				t.Errorf("unexpected error for %s: %v", m.name, err)
			}
			if result == nil {
				t.Errorf("expected non-nil result for %s", m.name)
			}
		})
	}
}

func TestProvider_Call_UpdateAdPlan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := Response{
			Status:  true,
			Message: "ok",
			Data:    json.RawMessage(`{}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewProvider(nil)
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"open_id":      "test-id",
			"open_key":     "test-key",
			"api_base_url": srv.URL,
		},
	})

	result, err := p.Call(context.Background(), "update_ad_plan", []byte(`{"product_code":"p1","action":"FROM_CODE","url_type":"URL"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]interface{}
	json.Unmarshal(result, &res)
	if res["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", res["status"])
	}
}

func TestProvider_Call_PushMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := Response{
			Status:  true,
			Message: "ok",
			Data:    json.RawMessage(`{}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewProvider(nil)
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"open_id":      "test-id",
			"open_key":     "test-key",
			"api_base_url": srv.URL,
		},
	})

	result, err := p.Call(context.Background(), "push_message", []byte(`{"product_code":"p1","channel_codes":"c1","gateway":"huawei","title":"t","body":"b"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]interface{}
	json.Unmarshal(result, &res)
	if res["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", res["status"])
	}
}

func TestProvider_Close(t *testing.T) {
	p := NewProvider(nil)
	// Close without init should be fine
	if err := p.Close(); err != nil {
		t.Errorf("unexpected error closing nil client: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	_ = p.Init(context.Background(), provider.ProviderConfig{
		Config: map[string]interface{}{
			"open_id":      "test-id",
			"open_key":     "test-key",
			"api_base_url": srv.URL,
			"enable_cache": true,
		},
	})

	if err := p.Close(); err != nil {
		t.Errorf("unexpected error closing provider: %v", err)
	}
}

func TestProvider_Call_MediaAppList_DefaultType(t *testing.T) {
	var capturedMediaType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		capturedMediaType = r.FormValue("mediaType")
		resp := Response{
			Status:  true,
			Message: "ok",
			Data:    json.RawMessage(`[]`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewProvider(nil)
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"open_id":      "test-id",
			"open_key":     "test-key",
			"api_base_url": srv.URL,
		},
	})

	// Call without media_type should default to "Toutiao"
	_, err := p.Call(context.Background(), "media_app_list", []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedMediaType != "Toutiao" {
		t.Errorf("expected default mediaType 'Toutiao', got %q", capturedMediaType)
	}
}

// isErrorAs is a helper to check if err can be cast to target type.
func isErrorAs(err error, target interface{}) bool {
	if err == nil {
		return false
	}
	switch t := target.(type) {
	case **provider.MethodNotSupportedError:
		if e, ok := err.(*provider.MethodNotSupportedError); ok {
			*t = e
			return true
		}
	}
	return false
}
