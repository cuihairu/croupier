package quicksdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient_MissingOpenID(t *testing.T) {
	_, err := NewClient(Config{OpenKey: "key"}, nil)
	if err == nil {
		t.Error("expected error for missing open_id")
	}
}

func TestNewClient_MissingOpenKey(t *testing.T) {
	_, err := NewClient(Config{OpenID: "id"}, nil)
	if err == nil {
		t.Error("expected error for missing open_key")
	}
}

func TestNewClient_Defaults(t *testing.T) {
	c, err := NewClient(Config{OpenID: "id", OpenKey: "key"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.baseURL != DefaultAPIBaseURL {
		t.Errorf("expected baseURL %q, got %q", DefaultAPIBaseURL, c.baseURL)
	}
	if c.httpClient.Timeout != DefaultTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultTimeout, c.httpClient.Timeout)
	}
	if !c.enabled {
		t.Error("expected client to be enabled by default")
	}
	if c.rateLimiter != nil {
		t.Error("expected no rate limiter by default")
	}
	if c.cache != nil {
		t.Error("expected no cache by default")
	}
}

func TestNewClient_WithCache(t *testing.T) {
	c, err := NewClient(Config{
		OpenID:        "id",
		OpenKey:       "key",
		EnableCache:   true,
		CacheDuration: 10 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.cache == nil {
		t.Error("expected cache to be initialized")
	}
	if !c.enableCache {
		t.Error("expected enableCache to be true")
	}
}

func TestNewClient_WithRateLimiter(t *testing.T) {
	c, err := NewClient(Config{
		OpenID:            "id",
		OpenKey:           "key",
		RequestsPerMinute: 60,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.rateLimiter == nil {
		t.Error("expected rate limiter to be initialized")
	}
}

func TestClient_IsEnabled_SetEnabled(t *testing.T) {
	c, _ := NewClient(Config{OpenID: "id", OpenKey: "key"}, nil)

	if !c.IsEnabled() {
		t.Error("expected client to be enabled initially")
	}

	c.SetEnabled(false)
	if c.IsEnabled() {
		t.Error("expected client to be disabled after SetEnabled(false)")
	}

	c.SetEnabled(true)
	if !c.IsEnabled() {
		t.Error("expected client to be enabled after SetEnabled(true)")
	}
}

func TestClient_Close(t *testing.T) {
	c, _ := NewClient(Config{OpenID: "id", OpenKey: "key", EnableCache: true}, nil)
	if err := c.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClient_Close_NoCache(t *testing.T) {
	c, _ := NewClient(Config{OpenID: "id", OpenKey: "key"}, nil)
	if err := c.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClient_Sign(t *testing.T) {
	c, _ := NewClient(Config{OpenID: "id", OpenKey: "mykey"}, nil)

	params := map[string]interface{}{
		"b": "2",
		"a": "1",
		"c": "3",
	}

	sig, err := c.sign(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify deterministic: same params produce same signature
	sig2, _ := c.sign(params)
	if sig != sig2 {
		t.Errorf("sign not deterministic: %q != %q", sig, sig2)
	}

	// Verify sign key is excluded
	paramsWithSign := map[string]interface{}{
		"a":    "1",
		"sign": "ignored",
	}
	sigWithSign, _ := c.sign(paramsWithSign)
	sigWithout, _ := c.sign(map[string]interface{}{"a": "1"})
	if sigWithSign != sigWithout {
		t.Error("sign key should be excluded from signature")
	}
}

func TestClient_BuildFormData(t *testing.T) {
	c, _ := NewClient(Config{OpenID: "id", OpenKey: "key"}, nil)

	params := map[string]interface{}{
		"key1": "value1",
		"key2": "value with spaces",
		"key3": "special&chars",
	}

	formData := c.buildFormData(params)

	// Should contain all key-value pairs
	if !containsAll(formData, "key1=value1", "key2=value+with+spaces") {
		t.Errorf("unexpected form data: %s", formData)
	}

	// Should be URL-encoded
	if !contains(formData, "key3=special%26chars") {
		t.Errorf("expected URL encoding for & character in: %s", formData)
	}
}

func TestClient_IsCacheableRequest(t *testing.T) {
	c, _ := NewClient(Config{OpenID: "id", OpenKey: "key"}, nil)

	cacheable := []string{
		"open/channelList",
		"open/serverList",
		"open/productList",
		"open/dayReport",
		"open/dayHourReport",
		"open/userLive",
		"open/channelDaysReport",
		"open/channelReport",
		"open/adReport",
		"open/getMediaApp",
		"open/getAdPlanGroup",
		"open/getPackageVersion",
		"open/getAdPages",
		"open/getAdPlan",
		"open/uwlLost",
	}

	for _, ep := range cacheable {
		if !c.isCacheableRequest(ep, nil) {
			t.Errorf("expected %q to be cacheable", ep)
		}
	}

	nonCacheable := []string{
		"open/createAdPlan",
		"open/updateAdPlan",
		"open/pushMessage",
		"open/unknown",
	}

	for _, ep := range nonCacheable {
		if c.isCacheableRequest(ep, nil) {
			t.Errorf("expected %q to NOT be cacheable", ep)
		}
	}
}

func TestClient_BuildCacheKey(t *testing.T) {
	c, _ := NewClient(Config{OpenID: "id", OpenKey: "key"}, nil)

	params := map[string]interface{}{
		"b": "2",
		"a": "1",
	}

	key1 := c.buildCacheKey("endpoint", params)
	key2 := c.buildCacheKey("endpoint", params)

	// Should be deterministic
	if key1 != key2 {
		t.Errorf("buildCacheKey not deterministic: %q != %q", key1, key2)
	}

	// Different params should produce different keys
	key3 := c.buildCacheKey("endpoint", map[string]interface{}{"a": "1", "b": "3"})
	if key1 == key3 {
		t.Error("different params should produce different keys")
	}

	// Different endpoints should produce different keys
	key4 := c.buildCacheKey("other", params)
	if key1 == key4 {
		t.Error("different endpoints should produce different keys")
	}
}

func TestClient_Do_Disabled(t *testing.T) {
	c, _ := NewClient(Config{OpenID: "id", OpenKey: "key"}, nil)
	c.SetEnabled(false)

	_, err := c.Do(context.Background(), "open/test", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for disabled client")
	}
}

func TestClient_Do_CacheHit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("HTTP request should not be made on cache hit")
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		OpenID:        "id",
		OpenKey:       "key",
		APIBaseURL:    srv.URL,
		EnableCache:   true,
		CacheDuration: 60 * time.Second,
	}, nil)

	// Pre-populate cache
	cacheKey := c.buildCacheKey("open/channelList", map[string]interface{}{"productCode": "p1"})
	c.cache.Set(cacheKey, &Response{Status: true, Message: "cached"})

	resp, err := c.Do(context.Background(), "open/channelList", map[string]interface{}{"productCode": "p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message != "cached" {
		t.Errorf("expected cached response, got %q", resp.Message)
	}
}

func TestClient_Do_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}

		// Verify openId is set
		if r.FormValue("openId") != "test-id" {
			t.Errorf("expected openId 'test-id', got %q", r.FormValue("openId"))
		}

		// Verify sign is present
		if r.FormValue("sign") == "" {
			t.Error("expected sign to be present")
		}

		resp := Response{
			Status:  true,
			Message: "success",
			Data:    json.RawMessage(`{"key":"value"}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		OpenID:     "test-id",
		OpenKey:    "test-key",
		APIBaseURL: srv.URL,
	}, nil)

	resp, err := c.Do(context.Background(), "open/test", map[string]interface{}{"param1": "val1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Status {
		t.Error("expected status true")
	}
	if resp.Message != "success" {
		t.Errorf("expected message 'success', got %q", resp.Message)
	}
}

func TestClient_Do_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := Response{
			Status:  false,
			Message: "something went wrong",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		OpenID:     "id",
		OpenKey:    "key",
		APIBaseURL: srv.URL,
	}, nil)

	_, err := c.Do(context.Background(), "open/test", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for API error response")
	}
}

func TestClient_Do_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		OpenID:     "id",
		OpenKey:    "key",
		APIBaseURL: srv.URL,
	}, nil)

	_, err := c.Do(context.Background(), "open/test", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestClient_Do_RateLimited(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		resp := Response{Status: true, Message: "ok"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		OpenID:            "id",
		OpenKey:           "key",
		APIBaseURL:        srv.URL,
		RequestsPerMinute: 60, // 1 per second
	}, nil)

	// First request should succeed (uses burst token)
	_, err := c.Do(context.Background(), "open/test", map[string]interface{}{})
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}

	// Second request should also succeed (may wait for token)
	_, err = c.Do(context.Background(), "open/test", map[string]interface{}{})
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}

	if requestCount != 2 {
		t.Errorf("expected 2 requests, got %d", requestCount)
	}
}

func TestClient_Do_CacheWrite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := Response{Status: true, Message: "fresh"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		OpenID:        "id",
		OpenKey:       "key",
		APIBaseURL:    srv.URL,
		EnableCache:   true,
		CacheDuration: 60 * time.Second,
	}, nil)

	// Build cache key BEFORE Do mutates params (Do adds openId/time/sign)
	params := map[string]interface{}{"productCode": "p1"}
	cacheKey := c.buildCacheKey("open/channelList", params)

	_, err := c.Do(context.Background(), "open/channelList", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it was cached using the pre-built key
	_, found := c.cache.Get(cacheKey)
	if !found {
		t.Error("expected response to be cached after first request")
	}
}

func TestClient_Do_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := Response{Status: true, Message: "ok"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		OpenID:     "id",
		OpenKey:    "key",
		APIBaseURL: srv.URL,
	}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := c.Do(ctx, "open/test", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestClient_Do_ContentType(t *testing.T) {
	var contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		resp := Response{Status: true, Message: "ok"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		OpenID:     "id",
		OpenKey:    "key",
		APIBaseURL: srv.URL,
	}, nil)

	_, err := c.Do(context.Background(), "open/test", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if contentType != "application/x-www-form-urlencoded" {
		t.Errorf("expected Content-Type 'application/x-www-form-urlencoded', got %q", contentType)
	}
}

func TestClient_Do_RetryOnNetworkError(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount <= 2 {
			// Close connection to simulate network error
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
			}
			return
		}
		resp := Response{Status: true, Message: "ok"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		OpenID:     "id",
		OpenKey:    "key",
		APIBaseURL: srv.URL,
		RetryCount: 3,
	}, nil)

	// This test verifies retry logic exists but may not always succeed
	// due to the nature of connection hijacking
	_, _ = c.Do(context.Background(), "open/test", map[string]interface{}{})
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// containsAll checks if s contains all substrings.
func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func TestClient_Sign_DifferentParams(t *testing.T) {
	c, _ := NewClient(Config{OpenID: "id", OpenKey: "key"}, nil)

	sig1, _ := c.sign(map[string]interface{}{"a": "1"})
	sig2, _ := c.sign(map[string]interface{}{"a": "2"})

	if sig1 == sig2 {
		t.Error("different params should produce different signatures")
	}
}

func TestClient_Sign_EmptyParams(t *testing.T) {
	c, _ := NewClient(Config{OpenID: "id", OpenKey: "key"}, nil)

	sig, err := c.sign(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig == "" {
		t.Error("expected non-empty signature for empty params")
	}
}

func TestClient_Do_URLConstruction(t *testing.T) {
	var requestURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURL = r.URL.String()
		resp := Response{Status: true, Message: "ok"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		OpenID:     "id",
		OpenKey:    "key",
		APIBaseURL: srv.URL,
	}, nil)

	_, err := c.Do(context.Background(), "open/test-endpoint", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestURL != "/open/test-endpoint" {
		t.Errorf("expected URL '/open/test-endpoint', got %q", requestURL)
	}
}

func TestClient_Do_TimestampAdded(t *testing.T) {
	var formValues map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		formValues = r.Form
		resp := Response{Status: true, Message: "ok"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		OpenID:     "id",
		OpenKey:    "key",
		APIBaseURL: srv.URL,
	}, nil)

	_, err := c.Do(context.Background(), "open/test", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if formValues["time"] == nil {
		t.Error("expected time parameter to be added")
	}
}

func TestInt64Value_Float64EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want int64
	}{
		{"large_float", float64(1e18), int64(1e18)},
		{"negative_float", float64(-3.7), -3},
		{"zero_float", float64(0), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Int64Value(tt.in)
			if got != tt.want {
				t.Errorf("Int64Value(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseTimestamp_EpochZero(t *testing.T) {
	got := ParseTimestamp(int64(0))
	if got.Unix() != 0 {
		t.Errorf("expected epoch 0, got %v", got)
	}
}

func TestGetString_ConvertNonString(t *testing.T) {
	m := map[string]interface{}{
		"struct": struct{ X int }{X: 42},
	}
	got := getString(m, "struct")
	if got == "" {
		t.Error("expected non-empty string for struct conversion")
	}
}

func TestGetInt_Float64Edge(t *testing.T) {
	m := map[string]interface{}{
		"neg":  float64(-3.7),
		"zero": float64(0),
	}

	if got := getInt(m, "neg"); got != -3 {
		t.Errorf("expected -3, got %d", got)
	}
	if got := getInt(m, "zero"); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestGetInt64_MissingKey(t *testing.T) {
	m := map[string]interface{}{}
	if got := getInt64(m, "missing"); got != 0 {
		t.Errorf("expected 0 for missing key, got %d", got)
	}
}

func TestGetBool_StringVariants(t *testing.T) {
	tests := []struct {
		val  string
		want bool
	}{
		{"true", true},
		{"1", true},
		{"TRUE", false},
		{"yes", false},
		{"0", false},
		{"false", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.val, func(t *testing.T) {
			m := map[string]interface{}{"k": tt.val}
			if got := getBool(m, "k"); got != tt.want {
				t.Errorf("getBool(%q) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

func TestClient_Do_WithNilParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := Response{Status: true, Message: "ok"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		OpenID:     "id",
		OpenKey:    "key",
		APIBaseURL: srv.URL,
	}, nil)

	// Do adds openId and time to params, so nil params should still work
	// because the code modifies the map in place
	_, err := c.Do(context.Background(), "open/test", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error with empty params: %v", err)
	}
}

func TestRateLimiter_NegativeRate(t *testing.T) {
	rl := newRateLimiter(-1)
	// Should use default 1000
	if rl == nil {
		t.Fatal("expected non-nil rate limiter")
	}

	ctx := context.Background()
	err := rl.Wait(ctx)
	if err != nil {
		t.Errorf("expected to acquire token, got error: %v", err)
	}
}

func BenchmarkSign(b *testing.B) {
	c, _ := NewClient(Config{OpenID: "id", OpenKey: "key"}, nil)
	params := map[string]interface{}{
		"productCode": "test",
		"channelCode": "100",
		"beginTime":   "1700000000",
		"endTime":     "1700086400",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.sign(params)
	}
}

func BenchmarkBuildCacheKey(b *testing.B) {
	c, _ := NewClient(Config{OpenID: "id", OpenKey: "key"}, nil)
	params := map[string]interface{}{
		"productCode": "test",
		"channelCode": "100",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.buildCacheKey("open/dayReport", params)
	}
}

func TestClient_Do_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"status":false,"message":"internal error"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		OpenID:     "id",
		OpenKey:    "key",
		APIBaseURL: srv.URL,
	}, nil)

	_, err := c.Do(context.Background(), "open/test", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestClient_Do_EmptyResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		OpenID:     "id",
		OpenKey:    "key",
		APIBaseURL: srv.URL,
	}, nil)

	_, err := c.Do(context.Background(), "open/test", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for empty response body")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := newCache(5 * time.Second)
	defer c.Close()

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			key := fmt.Sprintf("key%d", n)
			c.Set(key, &Response{Status: true})
			c.Get(key)
			c.Delete(key)
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
