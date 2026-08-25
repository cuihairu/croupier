package resourcecatalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Handler router helper
// ---------------------------------------------------------------------------

func newResourceCatalogRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewService(db, nil))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = svc.WithGameScope(ctx, svc.GameScope{
			GameID: c.GetHeader("X-Game-ID"),
			Env:    c.GetHeader("X-Env"),
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	api := router.Group("/api/resource-catalog")
	api.GET("", handler.List)
	api.GET("/:resourceKey", handler.Detail)
	api.PUT("/:resourceKey/semantics", handler.UpdateSemantics)
	api.GET("/:resourceKey/semantics/versions", handler.ListSemanticVersions)
	api.GET("/:resourceKey/conflicts", handler.ListConflicts)
	api.POST("/:resourceKey/conflicts/:field/resolve", handler.ResolveConflict)
	return router
}

func doCatalogRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("X-Game-ID", "g1")
	req.Header.Set("X-Env", "e1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func seedCatalogPlayer(t *testing.T, db *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, model.NewResourceCapabilityModel(db).UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Labels: map[string]interface{}{"zh-CN": "玩家"},
	}))
}

// ---------------------------------------------------------------------------
// Handler.List
// ---------------------------------------------------------------------------

func TestHandler_List_Success(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)
	seedCatalogPlayer(t, db)

	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog?query=player", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var resp ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)
	assert.Equal(t, "player", resp.Items[0].ResourceKey)
}

func TestHandler_List_InvalidQuery(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)

	// ListRequest has no form-bound fields, so binding cannot fail in a
	// meaningful way; assert an empty result instead.
	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var resp ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Items)
}

// ---------------------------------------------------------------------------
// Handler.Detail
// ---------------------------------------------------------------------------

func TestHandler_Detail_Success(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)
	seedCatalogPlayer(t, db)

	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog/player", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var item ResourceCatalogItem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &item))
	assert.Equal(t, "player", item.ResourceKey)
	assert.Equal(t, "玩家", item.Labels["zh-CN"])
}

func TestHandler_Detail_NotFound(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)

	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog/ghost", "")
	// The wrapped gorm.ErrRecordNotFound maps to 404.
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// ---------------------------------------------------------------------------
// Handler.UpdateSemantics
// ---------------------------------------------------------------------------

func TestHandler_UpdateSemantics_Success(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)
	seedCatalogPlayer(t, db)

	rec := doCatalogRequest(router, http.MethodPut, "/api/resource-catalog/player/semantics",
		`{"identityField":"player_id","changeReason":"handler test"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp UpdateSemanticsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Contains(t, resp.Message, "semantics updated")
	assert.Equal(t, "platform_review", resp.Source)
}

func TestHandler_UpdateSemantics_InvalidBody(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)
	seedCatalogPlayer(t, db)

	rec := doCatalogRequest(router, http.MethodPut, "/api/resource-catalog/player/semantics", `{bad json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_UpdateSemantics_CapabilityMissing(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)

	rec := doCatalogRequest(router, http.MethodPut, "/api/resource-catalog/ghost/semantics",
		`{"identityField":"id"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_found")
}

// ---------------------------------------------------------------------------
// Handler.ListSemanticVersions
// ---------------------------------------------------------------------------

func TestHandler_ListSemanticVersions_Success(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)
	ctx := context.Background()

	semModel := model.NewCapabilitySemanticsModel(db)
	require.NoError(t, semModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID: "g1", Env: "e1", ResourceKey: "player", Source: "sdk",
	}))
	sem, err := semModel.FindByScopeAndResourceKey(ctx, "g1", "e1", "player")
	require.NoError(t, err)
	require.NoError(t, model.NewCapabilitySemanticVersionModel(db).CreateVersion(ctx, &model.CapabilitySemanticVersion{
		SemanticsID: sem.ID, Version: 1, ChangeReason: "seed", CreatedBy: "tester",
	}))

	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog/player/semantics/versions?limit=10&offset=0", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var resp ListSemanticVersionsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	assert.Equal(t, 1, resp.Items[0].Version)
}

func TestHandler_ListSemanticVersions_NoSemantics(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)

	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog/player/semantics/versions", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var resp ListSemanticVersionsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Items)
	assert.Zero(t, resp.Total)
}

// ---------------------------------------------------------------------------
// Handler.ListConflicts
// ---------------------------------------------------------------------------

func TestHandler_ListConflicts_Success(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)
	ctx := context.Background()

	conflicts := []spec.SemanticConflict{
		{Field: "identityField", Values: map[spec.SemanticSource]json.RawMessage{
			spec.SemanticSourceSDKExplicit: json.RawMessage(`"id"`),
		}},
	}
	raw, err := json.Marshal(conflicts)
	require.NoError(t, err)
	require.NoError(t, model.NewCapabilitySemanticsModel(db).UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID: "g1", Env: "e1", ResourceKey: "player", Source: "sdk", Conflicts: raw,
	}))

	rec := doCatalogRequest(router, http.MethodGet, "/api/resource-catalog/player/conflicts", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var resp ListConflictsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Conflicts, 1)
	assert.Equal(t, "identityField", resp.Conflicts[0].Field)
}

// ---------------------------------------------------------------------------
// Handler.ResolveConflict
// ---------------------------------------------------------------------------

func TestHandler_ResolveConflict_Success(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)
	ctx := context.Background()

	conflicts := []spec.SemanticConflict{
		{Field: "identityField", Values: map[spec.SemanticSource]json.RawMessage{
			spec.SemanticSourceSDKExplicit:    json.RawMessage(`"sdk_id"`),
			spec.SemanticSourcePlatformReview: json.RawMessage(`"review_id"`),
		}},
	}
	raw, err := json.Marshal(conflicts)
	require.NoError(t, err)
	require.NoError(t, model.NewCapabilitySemanticsModel(db).UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID: "g1", Env: "e1", ResourceKey: "player", Source: "sdk", Conflicts: raw,
	}))

	rec := doCatalogRequest(router, http.MethodPost, "/api/resource-catalog/player/conflicts/identityField/resolve",
		`{"chosenSource":"platform_review","reason":"handler"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp ResolveConflictResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Contains(t, resp.Message, "Conflict resolved")
}

func TestHandler_ResolveConflict_InvalidBody(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)

	rec := doCatalogRequest(router, http.MethodPost, "/api/resource-catalog/player/conflicts/identityField/resolve", `oops`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ResolveConflict_InvalidSource(t *testing.T) {
	db := setupTestDB(t)
	router := newResourceCatalogRouter(t, db)

	rec := doCatalogRequest(router, http.MethodPost, "/api/resource-catalog/player/conflicts/identityField/resolve",
		`{"chosenSource":"bogus"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid chosenSource")
}

// ---------------------------------------------------------------------------
// getScope with values
// ---------------------------------------------------------------------------

func TestGetScope_WithValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(svc.WithGameScope(req.Context(), svc.GameScope{GameID: "game-a", Env: "env-b"}))
	ctx.Request = req

	gameID, env := getScope(ctx)
	assert.Equal(t, "game-a", gameID)
	assert.Equal(t, "env-b", env)
}

// ---------------------------------------------------------------------------
// humanizeResourceKey / labelsForResource / categoryKeyForResource
// ---------------------------------------------------------------------------

func TestHumanizeResourceKey(t *testing.T) {
	assert.Equal(t, "Inventory", humanizeResourceKey("inventory"))
	assert.Equal(t, "Player Item", humanizeResourceKey("player_item"))
	assert.Equal(t, "Mail Template", humanizeResourceKey("mail.template"))
	assert.Equal(t, "Guild Member", humanizeResourceKey("-guild_member-"))
	assert.Equal(t, "", humanizeResourceKey("  "))
	assert.Equal(t, "", humanizeResourceKey("__"))
}

func TestLabelsForResource(t *testing.T) {
	// Reviewed labels win.
	reviewed := labelsForResource(map[string]interface{}{"zh-CN": "玩家"}, "player")
	require.NotNil(t, reviewed)
	assert.Equal(t, "玩家", reviewed["zh-CN"])

	// Humanized fallback when no labels exist.
	fallback := labelsForResource(nil, "player_item")
	require.NotNil(t, fallback)
	assert.Equal(t, "Player Item", fallback["zh-CN"])
	assert.Equal(t, "Player Item", fallback["en-US"])

	// Non-string label values are dropped, so fallback kicks in.
	mixed := labelsForResource(map[string]interface{}{"zh-CN": 42}, "player")
	require.NotNil(t, mixed)
	assert.Equal(t, "Player", mixed["zh-CN"])
}

func TestCategoryKeyForResource(t *testing.T) {
	assert.Equal(t, "reviewed", categoryKeyForResource("player", "reviewed"))
	assert.Equal(t, "mail", categoryKeyForResource("mail.template", ""))
	assert.Equal(t, "player", categoryKeyForResource("player", ""))
	assert.Equal(t, "", categoryKeyForResource("  ", ""))
}

// ---------------------------------------------------------------------------
// buildAffectedPages deeper branches
// ---------------------------------------------------------------------------

func TestBuildAffectedPages_DraftAndPublished(t *testing.T) {
	db := setupTestDBWithPages(t)
	ctx := context.Background()
	service := NewService(db, nil)

	require.NoError(t, model.NewPageSpecModel(db).Upsert(ctx, &model.PageSpec{
		GameID: "g1", Env: "e1", PageKey: "resource--player", Type: "resource",
		ResourceKey: "player", Status: "draft", DraftRevision: 1, UpdatedAt: time.Now(),
	}))
	require.NoError(t, model.NewPageSpecModel(db).Upsert(ctx, &model.PageSpec{
		GameID: "g1", Env: "e1", PageKey: "resource--guild", Type: "resource",
		ResourceKey: "guild", Status: "draft", DraftRevision: 1, UpdatedAt: time.Now(),
	}))

	require.NoError(t, model.NewPublishedPageSpecModel(db).Create(ctx, &model.PublishedPageSpec{
		GameID: "g1", Env: "e1", PageKey: "resource--player", Version: 1,
		SpecJSON:    `{"pageKey":"resource--player","type":"resource","resourceKey":"player","bindings":[{"id":"query","functionId":"player.list"}]}`,
		Active:      true,
		PublishedAt: time.Now(), PublishedBy: "tester",
	}))
	require.NoError(t, model.NewPublishedPageSpecModel(db).Create(ctx, &model.PublishedPageSpec{
		GameID: "g1", Env: "e1", PageKey: "resource--guild", Version: 1,
		SpecJSON:    `{"pageKey":"resource--guild","type":"resource","resourceKey":"guild"}`,
		Active:      false,
		PublishedAt: time.Now(), PublishedBy: "tester",
	}))

	items, err := service.buildAffectedPages(ctx, "g1", "e1", "player")
	require.NoError(t, err)
	require.Len(t, items, 2)

	kinds := map[string]AffectedPageInfo{}
	for _, item := range items {
		kinds[item.Kind] = item
	}
	draft, ok := kinds["draft"]
	require.True(t, ok)
	assert.Equal(t, "resource--player", draft.PageKey)
	assert.Equal(t, 1, draft.DraftRevision)

	published, ok := kinds["published"]
	require.True(t, ok)
	assert.Equal(t, "active", published.Status)
	// Binding has no frozen contract snapshot, so freshness flags it stale.
	assert.True(t, published.Stale)
	assert.NotEmpty(t, published.BindingFreshness)
}

func TestBuildAffectedPages_MissingTablesReturnsNil(t *testing.T) {
	// setupTestDB does not migrate PageSpec/PublishedPageSpec tables, so the
	// helpers hit missing-table errors and return nil without failing.
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	require.NoError(t, model.NewResourceCapabilityModel(db).UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	}))
	items, err := service.buildAffectedPages(ctx, "g1", "e1", "player")
	require.NoError(t, err)
	assert.Nil(t, items)
}

// ---------------------------------------------------------------------------
// List / Detail error paths
// ---------------------------------------------------------------------------

func TestList_WithCategoryFilterAndQuery(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	capModel := model.NewResourceCapabilityModel(db)
	require.NoError(t, capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player", CategoryKey: "core",
	}))
	require.NoError(t, capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "guild", CategoryKey: "social",
	}))

	resp, err := service.List(ctx, &ListRequest{GameID: "g1", Env: "e1", Category: "core"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "player", resp.Items[0].ResourceKey)

	resp, err = service.List(ctx, &ListRequest{GameID: "g1", Env: "e1", Query: "guild"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "guild", resp.Items[0].ResourceKey)
}

func TestDetail_LabelsFallbackAndStatus(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	require.NoError(t, model.NewResourceCapabilityModel(db).UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player_item",
	}))
	require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player_item.list",
		Version: "1.0.0", Enabled: true, ResourceKey: "player_item",
		Capability: dbenum.CapabilityCollectionQuery, UpdatedAt: time.Now(),
	}))

	item, err := service.Detail(ctx, &DetailRequest{GameID: "g1", Env: "e1", ResourceKey: "player_item"})
	require.NoError(t, err)
	assert.Equal(t, "Player Item", item.Labels["zh-CN"])
	assert.Equal(t, "pending", item.Status)
	assert.Equal(t, "player_item", item.CategoryKey)
}

func TestDetail_FindSemanticsError(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)

	// Drop the semantics table to force a non-record-not-found error.
	require.NoError(t, db.Migrator().DropTable("capability_semantics"))
	_, err := service.Detail(context.Background(), &DetailRequest{GameID: "g1", Env: "e1", ResourceKey: "ghost"})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// validateFunctionBinding branches
// ---------------------------------------------------------------------------

func TestValidateFunctionBinding_Branches(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	contractModel := model.NewFunctionContractModel(db)
	require.NoError(t, contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.list",
		Version: "1.0.0", Enabled: true, ResourceKey: "player",
		Capability: dbenum.CapabilityCollectionQuery, UpdatedAt: time.Now(),
	}))
	require.NoError(t, contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "guild.list",
		Version: "1.0.0", Enabled: true, ResourceKey: "guild",
		Capability: dbenum.CapabilityCollectionQuery, UpdatedAt: time.Now(),
	}))
	require.NoError(t, contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.get",
		Version: "1.0.0", Enabled: true, ResourceKey: "player",
		Capability: dbenum.CapabilityItemQuery, UpdatedAt: time.Now(),
	}))
	require.NoError(t, contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.search",
		Version: "1.0.0", Enabled: false, ResourceKey: "player",
		Capability: dbenum.CapabilityCollectionQuery, UpdatedAt: time.Now(),
	}))

	listContract, err := contractModel.FindByScopeAndFunctionID(ctx, "g1", "e1", "player.list")
	require.NoError(t, err)
	guildContract, err := contractModel.FindByScopeAndFunctionID(ctx, "g1", "e1", "guild.list")
	require.NoError(t, err)
	getContract, err := contractModel.FindByScopeAndFunctionID(ctx, "g1", "e1", "player.get")
	require.NoError(t, err)
	searchContract, err := contractModel.FindByScopeAndFunctionID(ctx, "g1", "e1", "player.search")
	require.NoError(t, err)

	_, err = service.validateFunctionBinding(ctx, "g1", "e1", "player", listContract.ID, "collection_query")
	require.NoError(t, err)

	_, err = service.validateFunctionBinding(ctx, "g1", "e1", "player", guildContract.ID, "collection_query")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "belongs to resource")

	_, err = service.validateFunctionBinding(ctx, "g1", "e1", "player", getContract.ID, "collection_query")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "capability is")

	_, err = service.validateFunctionBinding(ctx, "g1", "e1", "player", searchContract.ID, "collection_query")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")

	_, err = service.validateFunctionBinding(ctx, "g1", "e1", "player", 99999, "collection_query")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// ResolveConflict branches
// ---------------------------------------------------------------------------

func seedConflictSemantics(t *testing.T, db *gorm.DB) {
	t.Helper()
	ctx := context.Background()
	conflicts := []spec.SemanticConflict{
		{Field: "identityField", Values: map[spec.SemanticSource]json.RawMessage{
			spec.SemanticSourceSDKExplicit: json.RawMessage(`"sdk_id"`),
		}},
	}
	raw, err := json.Marshal(conflicts)
	require.NoError(t, err)
	require.NoError(t, model.NewCapabilitySemanticsModel(db).UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID: "g1", Env: "e1", ResourceKey: "player", Source: "sdk", Conflicts: raw,
	}))
}

func TestResolveConflict_SourceNotInValues(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	seedConflictSemantics(t, db)

	_, err := service.ResolveConflict(context.Background(), &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Field: "identityField", ChosenSource: "openapi_rest",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in conflict values")
}

func TestResolveConflict_FieldNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	seedConflictSemantics(t, db)

	_, err := service.ResolveConflict(context.Background(), &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Field: "unknownField", ChosenSource: "sdk_explicit",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict not found")
}

func TestResolveConflict_SemanticsNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)

	_, err := service.ResolveConflict(context.Background(), &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "ghost",
		Field: "identityField", ChosenSource: "sdk_explicit",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find semantics")
}

func TestResolveConflict_UpdatesExistingProvenance(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)
	seedConflictSemantics(t, db)

	semModel := model.NewCapabilitySemanticsModel(db)
	sem, err := semModel.FindByScopeAndResourceKey(ctx, "g1", "e1", "player")
	require.NoError(t, err)
	existing := map[string]*spec.SemanticProvenance{
		"identityField": {Field: "identityField", Source: spec.SemanticSourceOpenAPIRest, Confidence: "low", Status: "stale"},
	}
	raw, err := json.Marshal(existing)
	require.NoError(t, err)
	sem.Provenance = raw
	require.NoError(t, semModel.UpsertSemantics(ctx, sem))

	resp, err := service.ResolveConflict(ctx, &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Field: "identityField", ChosenSource: "sdk_explicit", Reason: "pick sdk",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Message, "Conflict resolved")

	updated, err := semModel.FindByScopeAndResourceKey(ctx, "g1", "e1", "player")
	require.NoError(t, err)
	assert.Equal(t, "sdk_id", updated.IdentityField)

	var provenance map[string]*spec.SemanticProvenance
	require.NoError(t, json.Unmarshal(updated.Provenance, &provenance))
	require.NotNil(t, provenance["identityField"])
	assert.Equal(t, spec.SemanticSourceSDKExplicit, provenance["identityField"].Source)
	assert.Equal(t, "high", provenance["identityField"].Confidence)
	assert.Equal(t, "effective", provenance["identityField"].Status)
}

// ---------------------------------------------------------------------------
// ListSemanticVersions limit/offset branches
// ---------------------------------------------------------------------------

func TestListSemanticVersions_LimitClamping(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	semModel := model.NewCapabilitySemanticsModel(db)
	require.NoError(t, semModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID: "g1", Env: "e1", ResourceKey: "player", Source: "sdk",
	}))

	// Negative limit falls back to the default; offset below zero is clamped.
	resp, err := service.ListSemanticVersions(ctx, &ListSemanticVersionsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", Limit: -1, Offset: -5,
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
	assert.Zero(t, resp.Total)

	// Limit above maximum is clamped to 100 without error.
	resp, err = service.ListSemanticVersions(ctx, &ListSemanticVersionsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", Limit: 5000,
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

// ---------------------------------------------------------------------------
// semanticSourceDigest nil model
// ---------------------------------------------------------------------------

func TestSemanticSourceDigest_NilModel(t *testing.T) {
	assert.Empty(t, semanticSourceDigest(context.Background(), nil, "g1", "e1", "player"))
}

// ---------------------------------------------------------------------------
// parseProvenance nil-result branch
// ---------------------------------------------------------------------------

func TestParseProvenance_NullPayload(t *testing.T) {
	result := parseProvenance([]byte(`null`))
	require.NotNil(t, result)
	assert.Empty(t, result)
}
