package merge

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
)

func TestThreeWayMerge_NoChanges(t *testing.T) {
	base := spec.PageSpec{
		Title:   spec.LocalizedText{"zh-CN": "测试页面"},
		PageKey: "test.page",
		Type:    spec.PageTypeOperation,
	}

	result := ThreeWayMerge(base, base, base)

	assert.False(t, result.HasConflicts)
	assert.Empty(t, result.AutoMerge)
	assert.Empty(t, result.Conflicts)
}

func TestThreeWayMerge_AutoMergeDisplayField(t *testing.T) {
	base := spec.PageSpec{
		Title:   spec.LocalizedText{"zh-CN": "原始标题"},
		PageKey: "test.page",
		Type:    spec.PageTypeOperation,
	}

	draft := base
	draft.Title = spec.LocalizedText{"zh-CN": "用户修改的标题"}

	latest := base
	latest.Title = spec.LocalizedText{"zh-CN": "生成器的新标题"}

	result := ThreeWayMerge(base, draft, latest)

	assert.False(t, result.HasConflicts)
	assert.Len(t, result.AutoMerge, 1)
	assert.Equal(t, "title", result.AutoMerge[0].Field)
	// MergedValue is JSON encoded, so it should contain the user's changes
	assert.Contains(t, string(result.AutoMerge[0].MergedValue), "用户修改的标题")
}

func TestThreeWayMerge_ConflictField(t *testing.T) {
	base := spec.PageSpec{
		Title:   spec.LocalizedText{"zh-CN": "测试"},
		PageKey: "test.page",
		Type:    spec.PageTypeOperation,
		Bindings: []spec.PageFunctionBinding{
			{ID: "main", FunctionID: "func1"},
		},
	}

	draft := base
	draft.Bindings = []spec.PageFunctionBinding{
		{ID: "main", FunctionID: "func2"}, // User changed function
	}

	latest := base
	latest.Bindings = []spec.PageFunctionBinding{
		{ID: "main", FunctionID: "func3"}, // Generator changed function
	}

	result := ThreeWayMerge(base, draft, latest)

	assert.True(t, result.HasConflicts)
	assert.NotEmpty(t, result.Conflicts)
	assert.Equal(t, "bindings", result.Conflicts[0].Field)
}

func TestThreeWayMerge_MixedChanges(t *testing.T) {
	base := spec.PageSpec{
		Title:   spec.LocalizedText{"zh-CN": "原始"},
		Icon:    "default",
		PageKey: "test.page",
		Type:    spec.PageTypeOperation,
	}

	draft := base
	draft.Title = spec.LocalizedText{"zh-CN": "用户修改"} // Auto-merge field
	draft.Icon = "user-icon"                          // Auto-merge field

	latest := base
	latest.Title = spec.LocalizedText{"zh-CN": "生成器修改"} // Auto-merge field
	latest.Icon = "generator-icon"                      // Auto-merge field

	result := ThreeWayMerge(base, draft, latest)

	assert.False(t, result.HasConflicts)
	assert.Len(t, result.AutoMerge, 2) // title and icon
}

func TestAutoMergeFields_ContainsExpectedFields(t *testing.T) {
	assert.True(t, AutoMergeFields["title"])
	assert.True(t, AutoMergeFields["description"])
	assert.True(t, AutoMergeFields["icon"])
	assert.True(t, AutoMergeFields["category.labels"])
	assert.True(t, AutoMergeFields["navigation.title"])
	assert.True(t, AutoMergeFields["resource.listView.columns[].title"])
	assert.True(t, AutoMergeFields["resource.listView.defaultSort"])
	assert.True(t, AutoMergeFields["operation.form.fields[].label"])
	assert.True(t, AutoMergeFields["report.charts[].title"])
}

func TestConflictFields_ContainsExpectedFields(t *testing.T) {
	assert.True(t, ConflictFields["bindings"])
	assert.True(t, ConflictFields["bindings[].functionId"])
	assert.True(t, ConflictFields["bindings[].selectors"])
	assert.True(t, ConflictFields["resource.createForm"])
	assert.True(t, ConflictFields["resource.deleteAction"])
	assert.True(t, ConflictFields["operation.confirm"])
	assert.True(t, ConflictFields["task.taskView"])
	assert.True(t, ConflictFields["report.dataset"])
}

func TestThreeWayMergeAutoMergesSortButConflictsExecution(t *testing.T) {
	base := spec.PageSpec{
		Type: spec.PageTypeResource,
		Resource: &spec.ResourcePageSpec{ListView: &spec.ListViewSpec{
			DefaultSort: &spec.SortSpec{Field: "name", Order: "asc"},
		}},
		Bindings: []spec.PageFunctionBinding{{ID: "list", FunctionID: "player.list"}},
	}
	draft := base
	draft.Resource = &spec.ResourcePageSpec{ListView: &spec.ListViewSpec{
		DefaultSort: &spec.SortSpec{Field: "level", Order: "desc"},
	}}
	draft.Bindings = []spec.PageFunctionBinding{{ID: "list", FunctionID: "player.list.v2"}}
	latest := base
	latest.Resource = &spec.ResourcePageSpec{ListView: &spec.ListViewSpec{
		DefaultSort: &spec.SortSpec{Field: "createdAt", Order: "desc"},
	}}
	latest.Bindings = []spec.PageFunctionBinding{{ID: "list", FunctionID: "player.list.v3"}}

	result := ThreeWayMerge(base, draft, latest)
	assert.Len(t, result.AutoMerge, 1)
	assert.Equal(t, "resource.listView.defaultSort", result.AutoMerge[0].Field)
	assert.Len(t, result.Conflicts, 1)
	assert.Equal(t, "bindings", result.Conflicts[0].Field)
}
