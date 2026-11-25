package openapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-012 acceptance: a form-urlencoded request body is sent with the real
// form Content-Type, one key per repeated value, and every value escaped the
// way url.Values.Encode does it.

func TestBuildBody_FormType_EscapesAndMapsFields(t *testing.T) {
	p := NewProvider()
	body, ct := p.buildBody(&RequestBodyMapping{
		Type:   "form",
		Fields: map[string]string{"account": "name", "token": "token"},
	}, map[string]interface{}{
		"name":  "acc one",
		"token": "a&b=c",
		"extra": "dropped",
	})
	assert.Equal(t, "application/x-www-form-urlencoded", ct)
	assert.Equal(t, "account=acc+one&token=a%26b%3Dc", string(body))
}

func TestBuildBody_FormType_MultiValueField(t *testing.T) {
	p := NewProvider()
	body, ct := p.buildBody(&RequestBodyMapping{
		Type:   "form",
		Fields: map[string]string{"tag": "tags"},
	}, map[string]interface{}{
		"tags": []interface{}{"alpha", "beta gamma"},
	})
	assert.Equal(t, "application/x-www-form-urlencoded", ct)
	assert.Equal(t, "tag=alpha&tag=beta+gamma", string(body))
}

func TestBuildBody_FormType_WithoutFieldsSendsAllData(t *testing.T) {
	p := NewProvider()
	body, ct := p.buildBody(&RequestBodyMapping{Type: "form"}, map[string]interface{}{
		"b": "two",
		"a": "one",
	})
	assert.Equal(t, "application/x-www-form-urlencoded", ct)
	assert.Equal(t, "a=one&b=two", string(body))
}

func TestBuildBody_FormType_RendersScalars(t *testing.T) {
	p := NewProvider()
	body, _ := p.buildBody(&RequestBodyMapping{Type: "form"}, map[string]interface{}{
		"enabled":  true,
		"count":    float64(3),
		"ratio":    float64(2.5),
		"empty":    map[string]interface{}{},
		"settings": map[string]interface{}{"b": float64(2), "a": "x"},
	})
	assert.Equal(t,
		"count=3&empty=%7B%7D&enabled=true&ratio=2.5&settings%5Ba%5D=x&settings%5Bb%5D=2",
		string(body))
}

func TestBuildBody_FormType_IsDeterministic(t *testing.T) {
	p := NewProvider()
	data := map[string]interface{}{"z": "1", "a": "2", "m": "3"}
	first, _ := p.buildBody(&RequestBodyMapping{Type: "form"}, data)
	second, ct := p.buildBody(&RequestBodyMapping{Type: "form"}, data)
	assert.Equal(t, first, second)
	assert.Equal(t, "application/x-www-form-urlencoded", ct)
}

func TestBuildBody_FormType_SkipsAbsentAndNilFields(t *testing.T) {
	p := NewProvider()
	body, ct := p.buildBody(&RequestBodyMapping{
		Type:   "form",
		Fields: map[string]string{"name": "name", "age": "age"},
	}, map[string]interface{}{"name": "alice", "age": nil})
	assert.Equal(t, "application/x-www-form-urlencoded", ct)
	assert.Equal(t, "name=alice", string(body))
}

// The executor must actually ship the form body: the Content-Type header and the
// escaped payload are what the game side parses, so both are asserted on a real
// HTTP round trip rather than on the helper's return values.
func TestCall_FormRequestBody_SendsEscapedFormBody(t *testing.T) {
	var gotContentType string
	var gotBody []byte
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})
	defer server.Close()

	p := NewProvider()
	require.NoError(t, p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"baseUrl": server.URL,
			"methods": []interface{}{
				map[string]interface{}{
					"name":   "login",
					"path":   "/login",
					"method": "POST",
					"requestBody": map[string]interface{}{
						"type":   "form",
						"fields": map[string]interface{}{"account": "account", "tag": "tags"},
					},
				},
			},
		},
	}))

	payload, err := json.Marshal(map[string]interface{}{
		"account": "acc one",
		"tags":    []interface{}{"alpha", "b&c"},
	})
	require.NoError(t, err)
	data, err := p.Call(context.Background(), "login", payload)
	require.NoError(t, err)
	require.NotNil(t, data)

	assert.Equal(t, "application/x-www-form-urlencoded", gotContentType)
	assert.Equal(t, "account=acc+one&tag=alpha&tag=b%26c", string(gotBody))

	values, err := url.ParseQuery(string(gotBody))
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "b&c"}, values["tag"])
	assert.Equal(t, "acc one", values.Get("account"))
}
