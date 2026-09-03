// 覆盖目标：SdkStats handler/service、aggregateSdkLanguages 聚合排序、
// deleteProviderCaps 删除失败分支、openAPIDocFunctions 扩展字段提取。
package provider

import (
	"context"
	"net/http"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

func newSdkStatsStoreV9() *reg.Store {
	store := reg.NewStore()
	if err := store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-v9",
		GameID:  "game-v9",
		Env:     "prod",
		Providers: []reg.ProviderSession{
			{
				ProviderID:   "p-go-1",
				Addr:         "10.0.0.1:7000",
				SDKLanguage:  "go",
				SDKVersion:   "1.2.0",
				SDKName:      "croupier-go-sdk",
				LastSeenUnix: 1700000000,
			},
			{
				ProviderID:   "p-go-2",
				SDKLanguage:  "go",
				SDKVersion:   "1.2.0",
				LastSeenUnix: 1700000001,
			},
			{
				ProviderID:  "p-go-3",
				SDKLanguage: "go",
				SDKVersion:  "1.3.0",
			},
			{
				ProviderID:  "p-js-1",
				SDKLanguage: "js",
				SDKVersion:  "0.9.0",
			},
			{
				ProviderID: "p-blank",
			},
			{
				ProviderID:  "p-ws",
				SDKLanguage: "  ",
				SDKVersion:  "  ",
			},
		},
	}); err != nil {
		panic(err)
	}
	return store
}

func TestAggregateSdkLanguagesEmptyV9(t *testing.T) {
	out := aggregateSdkLanguages(nil)
	if len(out) != 0 {
		t.Fatalf("expected 0 languages, got %d", len(out))
	}
}

func TestAggregateSdkLanguagesSortAndVersionsV9(t *testing.T) {
	out := aggregateSdkLanguages([]SdkInstanceItem{
		{SdkLanguage: "go", SdkVersion: "1.2.0"},
		{SdkLanguage: "go", SdkVersion: "1.2.0"},
		{SdkLanguage: "go", SdkVersion: "1.3.0"},
		{SdkLanguage: "js", SdkVersion: "0.9.0"},
	})
	if len(out) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(out))
	}
	if out[0].Language != "go" || out[0].Count != 3 {
		t.Fatalf("unexpected top language: %+v", out[0])
	}
	if len(out[0].Versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(out[0].Versions))
	}
	if out[0].Versions[0].Version != "1.2.0" || out[0].Versions[0].Count != 2 {
		t.Fatalf("version count sort wrong: %+v", out[0].Versions)
	}
	if out[1].Language != "js" || out[1].Count != 1 {
		t.Fatalf("unexpected second language: %+v", out[1])
	}
}

func TestAggregateSdkLanguagesTieStableSortV9(t *testing.T) {
	out := aggregateSdkLanguages([]SdkInstanceItem{
		{SdkLanguage: "alpha", SdkVersion: "1"},
		{SdkLanguage: "beta", SdkVersion: "1"},
	})
	if len(out) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(out))
	}
	if out[0].Language != "alpha" || out[1].Language != "beta" {
		t.Fatalf("tie should keep name order: %+v", out)
	}
}

func TestServiceSdkStatsNilStoreV9(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	if _, err := s.SdkStats(context.Background(), &SdkStatsRequest{}); err == nil {
		t.Fatal("expected error for nil registry store")
	}
}

func TestServiceSdkStatsAggregatesV9(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newSdkStatsStoreV9()})
	resp, err := s.SdkStats(context.Background(), &SdkStatsRequest{})
	if err != nil {
		t.Fatalf("SdkStats error = %v", err)
	}
	if resp.TotalInstances != 6 {
		t.Fatalf("totalInstances = %d, want 6", resp.TotalInstances)
	}
	if len(resp.Instances) != 6 {
		t.Fatalf("instances = %d, want 6", len(resp.Instances))
	}
	// go(3) > js(1) > unknown(2)? unknown 也是 2 条（p-blank 与 p-ws）。
	if len(resp.Languages) != 3 {
		t.Fatalf("languages = %d, want 3", len(resp.Languages))
	}
	if resp.Languages[0].Language != "go" || resp.Languages[0].Count != 3 {
		t.Fatalf("top language = %+v", resp.Languages[0])
	}
	// 空白语言/版本归入 unknown
	unknownSeen := false
	for _, inst := range resp.Instances {
		if inst.ProviderID == "p-blank" || inst.ProviderID == "p-ws" {
			unknownSeen = true
			if inst.SdkLanguage != "unknown" || inst.SdkVersion != "unknown" {
				t.Fatalf("expected unknown sdk fields, got %+v", inst)
			}
		}
	}
	if !unknownSeen {
		t.Fatal("expected blank-language instance normalized to unknown")
	}
	// 实例字段透传
	for _, inst := range resp.Instances {
		if inst.AgentID != "agent-v9" || inst.GameID != "game-v9" || inst.Env != "prod" {
			t.Fatalf("instance scope fields wrong: %+v", inst)
		}
	}
}

func TestServiceSdkStatsEmptyStoreV9(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: reg.NewStore()})
	resp, err := s.SdkStats(nil, nil)
	if err != nil {
		t.Fatalf("SdkStats error = %v", err)
	}
	if resp.TotalInstances != 0 || len(resp.Instances) != 0 || len(resp.Languages) != 0 {
		t.Fatalf("expected empty stats, got %+v", resp)
	}
}

func TestHandlerSdkStatsSuccessV9(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(&svc.ServiceContext{RegistryStore: newSdkStatsStoreV9()}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers/sdk-stats", "")
	h.SdkStats(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
}

func TestHandlerSdkStatsServiceErrorV9(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers/sdk-stats", "")
	h.SdkStats(ctx)
	if rec.Code == http.StatusOK {
		t.Fatalf("status = 200, want error for nil store")
	}
}

func TestDeleteProviderCapsDeleteFailureV9(t *testing.T) {
	store := reg.NewStore()
	if err := deleteProviderCaps(store, "ghost"); err == nil {
		t.Fatal("expected error when deleting non-existent provider")
	}
	if err := deleteProviderCaps(store, "   "); err == nil {
		t.Fatal("expected error when deleting empty id")
	}
}

func TestServiceDeleteNilStoreV9(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	if _, err := s.Delete(context.Background(), &ProviderActionRequest{ID: "x"}); err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestOpenAPIDocFunctionsAllExtensionsV9(t *testing.T) {
	doc, err := decodeOpenAPIDoc([]byte(`{"openapi":"3.0.3","info":{"title":"t","version":"1"},"paths":{"/x":{"put":{"operationId":"op.x","summary":"s","x-resource":"r","x-risk":"high","x-operation":"write"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	fns := openAPIDocFunctions(doc)
	if len(fns) != 1 {
		t.Fatalf("functions = %d, want 1", len(fns))
	}
	fn := fns[0]
	if fn["resource"] != "r" || fn["risk"] != "high" || fn["operation"] != "write" {
		t.Fatalf("extension fields missing: %+v", fn)
	}
	if fn["method"] != "PUT" || fn["summary"] != "s" {
		t.Fatalf("base fields wrong: %+v", fn)
	}
}

func TestServiceReloadNilStoreV9(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	if _, err := s.Reload(context.Background(), &ProviderActionRequest{ID: "x"}); err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestServiceDetailEmptyIDV9(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: reg.NewStore()})
	if _, err := s.Detail(context.Background(), &ProviderDetailRequest{ID: "  "}); err == nil {
		t.Fatal("expected error for blank id")
	}
}
