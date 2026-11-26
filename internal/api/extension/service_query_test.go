package extension

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// Split from service_test.go; test bodies moved verbatim.

func TestCatalogListQuery(t *testing.T) {
	req := ExtensionCatalogListRequest{
		Keyword:  "test",
		Kind:     "analytics",
		Status:   "active",
		Page:     2,
		PageSize: 50,
	}
	q := catalogListQuery(req)
	if q.Keyword != "test" {
		t.Fatalf("unexpected keyword: %s", q.Keyword)
	}
	if q.Kind != "analytics" {
		t.Fatalf("unexpected kind: %s", q.Kind)
	}
	if q.Status != "active" {
		t.Fatalf("unexpected status: %s", q.Status)
	}
	if q.Limit != 50 {
		t.Fatalf("unexpected limit: %d", q.Limit)
	}
	if q.Offset != 50 {
		t.Fatalf("unexpected offset: %d", q.Offset)
	}
	defaultReq := ExtensionCatalogListRequest{}
	defaultQ := catalogListQuery(defaultReq)
	if defaultQ.Limit != 20 {
		t.Fatalf("unexpected default limit: %d", defaultQ.Limit)
	}
	if defaultQ.Offset != 0 {
		t.Fatalf("unexpected default offset: %d", defaultQ.Offset)
	}
	maxReq := ExtensionCatalogListRequest{PageSize: 200}
	maxQ := catalogListQuery(maxReq)
	if maxQ.Limit != 100 {
		t.Fatalf("unexpected max limit: %d", maxQ.Limit)
	}
}

func TestInstallationListQuery(t *testing.T) {
	req := ExtensionInstallationListRequest{
		ExtensionID: "ext1",
		ScopeType:   "system",
		ScopeID:     "global",
		TargetType:  "agent",
		TargetID:    "agent1",
		Status:      "enabled",
		Page:        3,
		PageSize:    25,
	}
	q := installationListQuery(req)
	if q.ExtensionID != "ext1" {
		t.Fatalf("unexpected extension_id: %s", q.ExtensionID)
	}
	if q.Limit != 25 {
		t.Fatalf("unexpected limit: %d", q.Limit)
	}
	if q.Offset != 50 {
		t.Fatalf("unexpected offset: %d", q.Offset)
	}
	defaultReq := ExtensionInstallationListRequest{}
	defaultQ := installationListQuery(defaultReq)
	if defaultQ.Limit != 20 {
		t.Fatalf("unexpected default limit: %d", defaultQ.Limit)
	}
	if defaultQ.Offset != 0 {
		t.Fatalf("unexpected default offset: %d", defaultQ.Offset)
	}
}

func TestCatalogListQuery_EdgeCases(t *testing.T) {
	t.Run("negative page", func(t *testing.T) {
		req := ExtensionCatalogListRequest{Page: -1, PageSize: 10}
		q := catalogListQuery(req)
		if q.Offset != 0 {
			t.Fatalf("expected offset 0 for negative page, got: %d", q.Offset)
		}
	})

	t.Run("zero page size", func(t *testing.T) {
		req := ExtensionCatalogListRequest{Page: 1, PageSize: 0}
		q := catalogListQuery(req)
		if q.Limit != 20 {
			t.Fatalf("expected limit 20 for zero page_size, got: %d", q.Limit)
		}
	})

	t.Run("page size exceeds max", func(t *testing.T) {
		req := ExtensionCatalogListRequest{Page: 1, PageSize: 200}
		q := catalogListQuery(req)
		if q.Limit != 100 {
			t.Fatalf("expected limit 100 for page_size > 100, got: %d", q.Limit)
		}
	})

	t.Run("page size at max boundary", func(t *testing.T) {
		req := ExtensionCatalogListRequest{Page: 1, PageSize: 100}
		q := catalogListQuery(req)
		if q.Limit != 100 {
			t.Fatalf("expected limit 100 at boundary, got: %d", q.Limit)
		}
	})
}

func TestInstallationListQuery_EdgeCases(t *testing.T) {
	t.Run("negative page", func(t *testing.T) {
		req := ExtensionInstallationListRequest{Page: -1, PageSize: 10}
		q := installationListQuery(req)
		if q.Offset != 0 {
			t.Fatalf("expected offset 0 for negative page, got: %d", q.Offset)
		}
	})

	t.Run("zero page size", func(t *testing.T) {
		req := ExtensionInstallationListRequest{Page: 1, PageSize: 0}
		q := installationListQuery(req)
		if q.Limit != 20 {
			t.Fatalf("expected limit 20 for zero page_size, got: %d", q.Limit)
		}
	})

	t.Run("page size exceeds max", func(t *testing.T) {
		req := ExtensionInstallationListRequest{Page: 1, PageSize: 150}
		q := installationListQuery(req)
		if q.Limit != 100 {
			t.Fatalf("expected limit 100 for page_size > 100, got: %d", q.Limit)
		}
	})
}

func TestCatalogListQuery_AllParams(t *testing.T) {
	req := ExtensionCatalogListRequest{
		Keyword:  "search",
		Kind:     "provider",
		Status:   "beta",
		Page:     5,
		PageSize: 50,
	}
	q := catalogListQuery(req)
	assert.Equal(t, "search", q.Keyword)
	assert.Equal(t, "provider", q.Kind)
	assert.Equal(t, "beta", q.Status)
	assert.Equal(t, 50, q.Limit)
	assert.Equal(t, 200, q.Offset) // (5-1)*50
}

func TestInstallationListQuery_AllParams(t *testing.T) {
	req := ExtensionInstallationListRequest{
		ExtensionID: "test.ext",
		ScopeType:   "system",
		ScopeID:     "global",
		TargetType:  "agent",
		TargetID:    "agent1",
		Status:      "enabled",
		Page:        2,
		PageSize:    10,
		Enabled:     boolPtr(true),
	}
	q := installationListQuery(req)
	assert.Equal(t, "test.ext", q.ExtensionID)
	assert.Equal(t, "system", q.ScopeType)
	assert.Equal(t, "global", q.ScopeID)
	assert.Equal(t, "agent", q.TargetType)
	assert.Equal(t, "agent1", q.TargetID)
	assert.Equal(t, "enabled", q.Status)
	assert.Equal(t, 10, q.Limit)
	assert.Equal(t, 10, q.Offset)
	assert.True(t, q.Enabled != nil && *q.Enabled)
}

func TestInstallationListQuery_WithEnabled(t *testing.T) {
	f := false
	req := ExtensionInstallationListRequest{
		Enabled: &f,
	}
	q := installationListQuery(req)
	assert.True(t, q.Enabled != nil && *q.Enabled == false)
}
