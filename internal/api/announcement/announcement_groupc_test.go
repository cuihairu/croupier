// Fill out the remaining coverable branches of the announcement package
// (group C): the service validation failure branch of handler.Create
// (audience=role without a role).
package announcement

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// After the request body is bound from JSON, the service's validateAudience
// fails -> response.Error maps it to 400, covering the service error branch
// of handler.Create.
func TestAnnouncementHandler_Create_ServiceValidationFailure(t *testing.T) {
	_, r := newHandlerFixture(t)

	w := doJSON2(r, http.MethodPost, "/admin/announcements",
		`{"title":"维护","contentMd":"c","audience":"role"}`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "audience=role")
}
