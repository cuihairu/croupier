package openapi

import "testing"

func TestParseOpenAPISpecExtractsInfoVersion(t *testing.T) {
	p := NewProvider()
	spec := []byte(`{
		"openapi": "3.0.3",
		"info": {"title": "svc", "version": "2.5.1"},
		"paths": {"/a": {"get": {"operationId": "get_a", "x-version": "0.9.0"}}}
	}`)
	if err := p.parseOpenAPISpec(spec); err != nil {
		t.Fatalf("parseOpenAPISpec: %v", err)
	}
	if got := p.InfoVersion(); got != "2.5.1" {
		t.Errorf("InfoVersion() = %q, want 2.5.1", got)
	}
	details := p.GetMethodDetails()
	if got := details["get_a"].Version; got != "0.9.0" {
		t.Errorf("x-version = %q, want 0.9.0", got)
	}
}

func TestParseOpenAPISpecMultiDocFirstInfoVersionWins(t *testing.T) {
	p := NewProvider()
	first := []byte(`{"openapi":"3.0.3","info":{"title":"a","version":"1.1.0"},"paths":{"/x":{"get":{"operationId":"x"}}}}`)
	second := []byte(`{"openapi":"3.0.3","info":{"title":"b","version":"2.2.0"},"paths":{"/y":{"get":{"operationId":"y"}}}}`)
	if err := p.parseOpenAPISpec(first); err != nil {
		t.Fatal(err)
	}
	if err := p.parseOpenAPISpec(second); err != nil {
		t.Fatal(err)
	}
	if got := p.InfoVersion(); got != "1.1.0" {
		t.Errorf("first document info.version must win, got %q", got)
	}
	if len(p.GetMethodDetails()) != 2 {
		t.Errorf("both docs' operations must be merged, got %d", len(p.GetMethodDetails()))
	}
}

func TestParseOpenAPISpecNonSemverInfoVersionCarriedRaw(t *testing.T) {
	p := NewProvider()
	spec := []byte(`{"openapi":"3.0.3","info":{"title":"a","version":"2026-09-19"},"paths":{}}`)
	if err := p.parseOpenAPISpec(spec); err != nil {
		t.Fatal(err)
	}
	// 解析层只承载原值，semver 合法性由消费方（versionutil）决定。
	if got := p.InfoVersion(); got != "2026-09-19" {
		t.Errorf("raw info.version must be carried, got %q", got)
	}
}
