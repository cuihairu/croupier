// Coverage target: the error and success branches of List/Update/Vote/Categories
// in handler.go, the full Vote flow; the model-layer error branches of
// service.go (List/Vote/Create/Update/Categories failures); and the
// invalid-input branches of decodeTags/normalizeTags.
package faq

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- handler layer ----

func TestHandler_List_QueryBindError(t *testing.T) {
	handler := newFAQHandler(newFAQTestDB(t))

	ctx, rec := newFAQRequest(http.MethodGet, "/api/v1/faqs?page=abc", "")
	handler.List(ctx)

	// A failed form numeric conversion falls back to response.Error (non-200
	// status with the unified error body).
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertFAQErrorShape(t, rec)
}

func TestHandler_List_ServiceError(t *testing.T) {
	db := newFAQTestDB(t)
	handler := newFAQHandler(db)
	require.NoError(t, db.Migrator().DropTable(&model.FAQ{}))

	ctx, rec := newFAQRequest(http.MethodGet, "/api/v1/faqs?page=1", "")
	handler.List(ctx)

	assert.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
}

func TestHandler_Update_InvalidJSON(t *testing.T) {
	handler := newFAQHandler(newFAQTestDB(t))

	ctx, rec := newFAQRequest(http.MethodPut, "/api/v1/faqs/1", `{not-json`)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	handler.Update(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestHandler_Update_Success(t *testing.T) {
	db := newFAQTestDB(t)
	handler := newFAQHandler(db)
	created, err := newFAQService(db).
		Create(context.Background(), &FAQCreateRequest{Question: "q", Answer: "a", Category: "c", Visible: true})
	require.NoError(t, err)

	ctx, rec := newFAQRequest(http.MethodPut, fmt.Sprintf("/api/v1/faqs/%d", created.Id),
		`{"question":"updated question"}`)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(created.Id)}}
	handler.Update(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "updated question")
}

func TestHandler_Vote_Success(t *testing.T) {
	db := newFAQTestDB(t)
	handler := newFAQHandler(db)
	created, err := newFAQService(db).
		Create(context.Background(), &FAQCreateRequest{Question: "q", Answer: "a", Category: "c", Visible: true})
	require.NoError(t, err)

	ctx, rec := newFAQRequest(http.MethodPost, fmt.Sprintf("/api/v1/faqs/%d/vote", created.Id),
		`{"helpful":true}`)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(created.Id)}}
	handler.Vote(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"helpfulCount":1`)
}

func TestHandler_Vote_InvalidJSON(t *testing.T) {
	handler := newFAQHandler(newFAQTestDB(t))

	ctx, rec := newFAQRequest(http.MethodPost, "/api/v1/faqs/1/vote", `{not-json`)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	handler.Vote(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestHandler_Vote_MissingFAQ(t *testing.T) {
	handler := newFAQHandler(newFAQTestDB(t))

	ctx, rec := newFAQRequest(http.MethodPost, "/api/v1/faqs/999/vote", `{"helpful":true}`)
	ctx.Params = gin.Params{{Key: "id", Value: "999"}}
	handler.Vote(ctx)

	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertFAQErrorShape(t, rec)
}

func TestHandler_Vote_InvalidID(t *testing.T) {
	handler := newFAQHandler(newFAQTestDB(t))

	ctx, rec := newFAQRequest(http.MethodPost, "/api/v1/faqs/abc/vote", `{"helpful":true}`)
	ctx.Params = gin.Params{{Key: "id", Value: "abc"}}
	handler.Vote(ctx)

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestHandler_Categories_ServiceError(t *testing.T) {
	db := newFAQTestDB(t)
	handler := newFAQHandler(db)
	require.NoError(t, db.Migrator().DropTable(&model.FAQ{}))

	ctx, rec := newFAQRequest(http.MethodGet, "/api/v1/faqs/categories", "")
	handler.Categories(ctx)

	assert.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
}

// ---- service-layer error branches ----

func TestService_List_ModelError(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	require.NoError(t, db.Migrator().DropTable(&model.FAQ{}))

	_, err := svc.List(context.Background(), &FAQListRequest{Page: 1, PageSize: 10})
	require.Error(t, err)
}

func TestService_Vote_InvalidID(t *testing.T) {
	svc := newFAQService(newFAQServiceTestDB(t))

	_, err := svc.Vote(context.Background(), &FAQVoteRequest{ID: "abc", Helpful: true})
	require.Error(t, err)
}

func TestService_Create_SlugExistsError(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	require.NoError(t, db.Migrator().DropTable(&model.FAQ{}))

	_, err := svc.Create(context.Background(), &FAQCreateRequest{
		Question: "q", Answer: "a", Category: "c", Slug: "some-slug",
	})
	require.Error(t, err)
}

func TestService_Create_ModelError(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	require.NoError(t, db.Migrator().DropTable(&model.FAQ{}))

	_, err := svc.Create(context.Background(), &FAQCreateRequest{
		Question: "q", Answer: "a", Category: "c",
	})
	require.Error(t, err)
}

func TestService_Update_SlugExistsError(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	created, err := svc.Create(context.Background(), &FAQCreateRequest{Question: "q", Answer: "a", Category: "c"})
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable(&model.FAQ{}))

	slug := "fresh-slug"
	_, err = svc.Update(context.Background(), &FAQUpdateRequest{ID: fmt.Sprint(created.Id), Slug: &slug})
	require.Error(t, err)
}

func TestService_Update_ModelError(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	created, err := svc.Create(context.Background(), &FAQCreateRequest{Question: "q", Answer: "a", Category: "c"})
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable(&model.FAQ{}))

	_, err = svc.Update(context.Background(), &FAQUpdateRequest{ID: fmt.Sprint(created.Id), Question: "q2"})
	require.Error(t, err)
}

func TestService_Categories_ModelError(t *testing.T) {
	db := newFAQServiceTestDB(t)
	svc := newFAQService(db)
	require.NoError(t, db.Migrator().DropTable(&model.FAQ{}))

	_, err := svc.Categories(context.Background(), &FAQCategoriesRequest{})
	require.Error(t, err)
}

// ---- pure function branches ----

func TestDecodeTags_InvalidJSON(t *testing.T) {
	assert.Nil(t, decodeTags(model.JSON([]byte(`not-json`))))
	assert.Nil(t, decodeTags(model.JSON(nil)))
	assert.Equal(t, []string{"a"}, decodeTags(model.JSON([]byte(`["a"]`))))
}

func TestNormalizeTags_EmptyAndDuplicate(t *testing.T) {
	assert.Empty(t, normalizeTags([]string{"  ", ""}))
	assert.Equal(t, []string{"Pay", "LOGIN"}, normalizeTags([]string{" Pay ", "pay", "LOGIN", "login"}))
}

func TestEncodeTags_Empty(t *testing.T) {
	assert.NotNil(t, encodeTags(nil))
	assert.Empty(t, encodeTags(nil))
}
