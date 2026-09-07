package versioning

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingValidator makes every binding validation fail. gin's ShouldBindUri
// and ShouldBindQuery both call binding.Validator.ValidateStruct after mapping
// values, so swapping the global validator is the only way to exercise the
// uri/query binding error branches of the versioning handlers (their request
// DTOs only contain string uri fields, so real requests never fail binding).
type failingValidator struct{}

func (failingValidator) ValidateStruct(any) error {
	return errors.New("injected binding validation failure")
}

func (failingValidator) Engine() any { return nil }

// delayedFailingValidator passes the first validation call and fails every
// subsequent one. For Diff the first call is the uri binding and the second
// is the query binding, so the uri bind succeeds while BindQueryCompat fails.
type delayedFailingValidator struct {
	calls int
}

func (v *delayedFailingValidator) ValidateStruct(any) error {
	v.calls++
	if v.calls >= 2 {
		return errors.New("injected binding validation failure")
	}
	return nil
}

func (v *delayedFailingValidator) Engine() any { return nil }

func withBindingValidator(t *testing.T, v binding.StructValidator) {
	t.Helper()
	orig := binding.Validator
	binding.Validator = v
	t.Cleanup(func() { binding.Validator = orig })
}

func TestHandler_BindURIValidationFailures(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"GetChangeChain", http.MethodGet, "/api/versioning/pages/some--page/chain", ""},
		{"Diff", http.MethodGet, "/api/versioning/pages/some--page/diff", ""},
		{"Merge", http.MethodPost, "/api/versioning/pages/some--page/merge", `{"strategy":"auto"}`},
		{"RollbackDraft", http.MethodPost, "/api/versioning/pages/some--page/rollback-draft", `{"expectedDraftRevision":1,"version":1}`},
		{"RollbackPublish", http.MethodPost, "/api/versioning/pages/some--page/rollback-publish", `{"expectedDraftRevision":1,"version":1}`},
		{"RegenerateProposal", http.MethodPost, "/api/versioning/pages/some--page/regenerate", `{"force":false}`},
		{"Republish", http.MethodPost, "/api/versioning/pages/some--page/republish", `{"reason":"x"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupTestDB(t)
			router := newVersioningHandlerRouter(t, db)
			withBindingValidator(t, failingValidator{})

			rec := doVersioningRequest(router, tc.method, tc.path, tc.body)
			require.Equal(t, http.StatusInternalServerError, rec.Code)
			assert.Contains(t, rec.Body.String(), "internal_error")
		})
	}
}

func TestHandler_DiffBindQueryValidationFailure(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouter(t, db)
	withBindingValidator(t, &delayedFailingValidator{})

	rec := doVersioningRequest(router, http.MethodGet, "/api/versioning/pages/some--page/diff?fromVersion=1&toVersion=2", "")
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal_error")
}
