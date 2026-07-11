package user

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: the user package currently exposes only the UserInfo DTO. There is no
// handler.go or service.go in this package, so there is nothing to exercise at
// the HTTP layer. The actual authenticated profile handler lives in
// internal/api/auth. These tests lock down the DTO contract (JSON shape and
// field casing) so downstream consumers keep compiling.

func TestUserInfo_JSONRoundTrip(t *testing.T) {
	original := UserInfo{
		Username: "admin",
		Roles:    []string{"admin", "operator"},
		Nickname: "Administrator",
		Email:    "admin@example.com",
		Phone:    "13800000000",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded UserInfo
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, original, decoded)
}

func TestUserInfo_OmitemptyFields(t *testing.T) {
	// Nickname/Email/Phone are tagged omitempty and must be absent when empty.
	minimal := UserInfo{
		Username: "someone",
		Roles:    []string{"viewer"},
	}

	data, err := json.Marshal(minimal)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.Contains(t, raw, "username")
	assert.Contains(t, raw, "roles")
	assert.NotContains(t, raw, "nickname", "nickname should be omitted when empty")
	assert.NotContains(t, raw, "email", "email should be omitted when empty")
	assert.NotContains(t, raw, "phone", "phone should be omitted when empty")
}

func TestUserInfo_FieldCasing(t *testing.T) {
	info := UserInfo{
		Username: "admin",
		Nickname: "Admin",
		Email:    "a@b.c",
		Phone:    "123",
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	for _, key := range []string{"username", "roles", "nickname", "email", "phone"} {
		assert.Contains(t, raw, key, "expected lowerCamelCase JSON key %q", key)
	}
}

func TestUserInfo_EmptyRoles(t *testing.T) {
	info := UserInfo{Username: "admin"}
	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded UserInfo
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Nil(t, decoded.Roles)
}
