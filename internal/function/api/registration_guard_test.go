package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/stretchr/testify/assert"
)

func postFunction(router *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/v1/metadata/functions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestRegisterFunction_RejectsPresentationExtensionKeys(t *testing.T) {
	router, _ := setupTestRouter()

	for name, extension := range map[string]string{
		"menu":  "x-menu",
		"title": "title",
	} {
		t.Run(name, func(t *testing.T) {
			w := postFunction(router, `{
				"id": "player.ban",
				"name": "player.ban",
				"version": "1.0.0",
				"resource": "player",
				"extensions": {"`+extension+`": "Players"}
			}`)

			Equal(t, http.StatusBadRequest, w.Code)
			var resp map[string]interface{}
			Nil(t, json.Unmarshal(w.Body.Bytes(), &resp))
			Equal(t, "bad_request", resp["error"])
			details, ok := resp["details"].(map[string]interface{})
			True(t, ok, "expected structured details, got %v", resp)
			Equal(t, extension, details["field"])
			Equal(t, "extensions."+extension, details["location"])
		})
	}
}

func TestRegisterFunction_RejectsPresentationFieldInSchema(t *testing.T) {
	router, _ := setupTestRouter()

	w := postFunction(router, `{
		"id": "player.list",
		"name": "player.list",
		"version": "1.0.0",
		"resource": "player",
		"inputSchema": "{\"type\":\"object\",\"properties\":{\"keyword\":{\"type\":\"string\",\"x-pagination\":{\"pageSize\":20}}}}"
	}`)

	Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	Nil(t, json.Unmarshal(w.Body.Bytes(), &resp))
	Equal(t, "bad_request", resp["error"])
	details, ok := resp["details"].(map[string]interface{})
	True(t, ok, "expected structured details, got %v", resp)
	Equal(t, "x-pagination", details["field"])
	Equal(t, "inputSchema.properties.keyword.x-pagination", details["location"])
}

func TestRegisterFunction_AcceptsCapabilityContractFields(t *testing.T) {
	router, _ := setupTestRouter()

	w := postFunction(router, `{
		"id": "player.ban",
		"name": "player.ban",
		"version": "1.0.0",
		"resource": "player",
		"tags": ["moderation"],
		"description": "Ban a player",
		"inputSchema": "{\"type\":\"object\",\"properties\":{\"player_id\":{\"type\":\"string\"}},\"required\":[\"player_id\"]}",
		"outputSchema": "{\"type\":\"object\",\"properties\":{\"ok\":{\"type\":\"boolean\"}}}",
		"behavior": {"mode": "command", "idempotent": false, "timeoutMs": 3000, "routeStrategy": "lb", "cacheable": false},
		"security": {"riskLevel": "high", "permission": "player.ban.invoke", "requiresApproval": true, "approvalType": "two_person", "auditLog": true, "maskSensitiveData": false},
		"extensions": {"x-vendor-note": "ops-only"}
	}`)

	Equal(t, http.StatusCreated, w.Code)
}

func TestUpdateFunction_RejectsPresentationFields(t *testing.T) {
	router, service := setupTestRouter()

	created := postFunction(router, `{"id": "player.get", "name": "player.get", "version": "1.0.0", "resource": "player"}`)
	Equal(t, http.StatusCreated, created.Code)

	req := httptest.NewRequest("PUT", "/api/v1/metadata/functions/player.get", strings.NewReader(`{
		"outputSchema": "{\"type\":\"object\",\"x-route\":\"/console/players\"}"
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	Nil(t, json.Unmarshal(w.Body.Bytes(), &resp))
	Equal(t, "bad_request", resp["error"])
	details, ok := resp["details"].(map[string]interface{})
	True(t, ok, "expected structured details, got %v", resp)
	Equal(t, "x-route", details["field"])

	// The stored function must remain untouched.
	stored, err := service.Get(testCtx(), "player.get")
	Nil(t, err)
	Empty(t, stored.OutputSchema)
}
