package release

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Covers the happy paths and error branches of List/Create/Transition (they
// were previously at 0% -- only partially covered indirectly through the handler).

func TestServiceList_FiltersAndPaging(t *testing.T) {
	f := newFixture(t)
	f.seedRelease(t, "full", "1.0.0", 100)
	f.seedRelease(t, "gray", "1.1.0", 30)

	// All rows
	all, err := f.svc.List(context.Background(), &ReleaseListRequest{GameID: "demo", Env: "prod", Channel: "official", Platform: "android", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, all.Items, 2)
	assert.Equal(t, int64(2), all.Total)

	// Status filter
	grayOnly, err := f.svc.List(context.Background(), &ReleaseListRequest{GameID: "demo", Env: "prod", Channel: "official", Platform: "android", Status: "gray", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, grayOnly.Items, 1)

	// Paging
	p1, err := f.svc.List(context.Background(), &ReleaseListRequest{GameID: "demo", Env: "prod", Channel: "official", Platform: "android", Page: 1, PageSize: 1})
	require.NoError(t, err)
	assert.Len(t, p1.Items, 1)
	assert.Equal(t, int64(2), p1.Total)
}

func TestServiceCreate_ValidationAndSuccess(t *testing.T) {
	f := newFixture(t)

	// Invalid type
	_, err := f.svc.Create(context.Background(), &ReleaseCreateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		Version: "1.0.0", Type: "bogus",
	})
	assert.Error(t, err)

	// Successfully create a full release
	res, err := f.svc.Create(context.Background(), &ReleaseCreateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		Version: "2.0.0", Type: "full",
	})
	require.NoError(t, err)
	assert.NotZero(t, res.Id)
	assert.Equal(t, "draft", res.Status)

	// Channel validation
	_, err = f.svc.Create(context.Background(), &ReleaseCreateRequest{
		GameID: "demo", Env: "prod", Channel: "", Platform: "android",
		Version: "3.0.0", Type: "full",
	})
	assert.Error(t, err)
}

func TestServiceTransition_ErrorsAndHappy(t *testing.T) {
	f := newFixture(t)

	// Invalid action / invalid ID / nonexistent ID
	_, err := f.svc.Transition(context.Background(), &ReleaseTransitionRequest{ID: "1", Action: "bogus"})
	assert.Error(t, err)
	_, err = f.svc.Transition(context.Background(), &ReleaseTransitionRequest{ID: "abc", Action: "full"})
	assert.Error(t, err)
	_, err = f.svc.Transition(context.Background(), &ReleaseTransitionRequest{ID: "999", Action: "testing"})
	assert.Error(t, err)

	// Illegal transition: draft -> testing (the state machine requires
	// draft -> uploading -> testing)
	draft := f.seedRelease(t, "draft", "1.0.0", 0)
	_, err = f.svc.Transition(context.Background(), &ReleaseTransitionRequest{ID: uitoa(draft.ID), Action: "testing"})
	assert.Error(t, err)

	// Legal chain: testing -> gray (canary) -> full -> rollback
	// (starting from testing needs the artifact already uploaded -- covered by
	// the existing UploadArtifact case, so seed directly here)
	rel := f.seedRelease(t, "testing", "1.0.0", 0)

	res, err := f.svc.Transition(context.Background(), &ReleaseTransitionRequest{ID: uitoa(rel.ID), Action: "gray", GrayPercent: intPtr(30)})
	require.NoError(t, err)
	assert.Equal(t, "gray", res.Status)

	res, err = f.svc.Transition(context.Background(), &ReleaseTransitionRequest{ID: uitoa(rel.ID), Action: "full"})
	require.NoError(t, err)
	assert.Equal(t, "full", res.Status)

	res, err = f.svc.Transition(context.Background(), &ReleaseTransitionRequest{ID: uitoa(rel.ID), Action: "rollback"})
	require.NoError(t, err)
	assert.Equal(t, "rolled_back", res.Status)
}

func intPtr(v int) *int { return &v }

func uitoa(v uint) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
