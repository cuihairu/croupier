package tasks

import (
	"testing"
)

func TestNewStore(t *testing.T) {
	store := NewStore(nil, nil)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestJSONPayload_Nil(t *testing.T) {
	result := JSONPayload(nil)
	if string(result) != "null" {
		t.Errorf("expected null, got %s", string(result))
	}
}

func TestJSONPayload_Struct(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	s := TestStruct{Name: "test", Age: 42}
	result := JSONPayload(s)
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
	if string(result) == "null" {
		t.Error("expected non-null result")
	}
}

func TestJSONPayload_Slice(t *testing.T) {
	result := JSONPayload([]string{"a", "b", "c"})
	if string(result) != `["a","b","c"]` {
		t.Errorf("unexpected result: %s", string(result))
	}
}

func TestJSONPayload_Map(t *testing.T) {
	result := JSONPayload(map[string]int{"one": 1, "two": 2})
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestJSONPayload_Bool(t *testing.T) {
	result := JSONPayload(true)
	if string(result) != "true" {
		t.Errorf("expected true, got %s", string(result))
	}
}

func TestJSONPayload_Float(t *testing.T) {
	result := JSONPayload(3.14)
	if string(result) != "3.14" {
		t.Errorf("expected 3.14, got %s", string(result))
	}
}
