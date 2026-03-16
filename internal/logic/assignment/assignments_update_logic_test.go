package assignment

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
)

func TestDiffFunctions_EmptyBefore(t *testing.T) {
	added, removed := diffFunctions(
		[]string{},
		[]string{"a", "b"},
	)
	assert.Equal(t, []string{"a", "b"}, added)
	assert.Empty(t, removed)
}

func TestDiffFunctions_EmptyAfter(t *testing.T) {
	added, removed := diffFunctions(
		[]string{"a", "b"},
		[]string{},
	)
	assert.Empty(t, added)
	assert.Equal(t, []string{"a", "b"}, removed)
}

func TestDiffFunctions_SameLists(t *testing.T) {
	added, removed := diffFunctions(
		[]string{"a", "b", "c"},
		[]string{"a", "b", "c"},
	)
	assert.Empty(t, added)
	assert.Empty(t, removed)
}

func TestDiffFunctions_NoOverlap(t *testing.T) {
	added, removed := diffFunctions(
		[]string{"a", "b"},
		[]string{"c", "d"},
	)
	assert.Equal(t, []string{"c", "d"}, added)
	assert.Equal(t, []string{"a", "b"}, removed)
}

func TestCollectKnownFunctions_NilContext(t *testing.T) {
	result := collectKnownFunctions(nil)
	assert.Nil(t, result)
}

func TestCollectKnownFunctions_NilRegistryStore(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	result := collectKnownFunctions(svcCtx)
	assert.Nil(t, result)
}

func TestNewAssignmentsUpdateLogic(t *testing.T) {
	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}

	logic := NewAssignmentsUpdateLogic(ctx, svcCtx)

	assert.NotNil(t, logic)
	assert.Equal(t, ctx, logic.ctx)
	assert.Same(t, svcCtx, logic.svcCtx)
}

func TestSplitKnownAndUnknown_NilKnown(t *testing.T) {
	input := []string{"func1", "func2"}

	// With nil/empty known map, all functions should be accepted (not unknown)
	accepted, unknown := splitKnownAndUnknown(input, nil)

	assert.Equal(t, input, accepted)
	assert.Nil(t, unknown)
}

func TestSplitKnownAndUnknown_EmptyInput(t *testing.T) {
	known := map[string]struct{}{
		"func1": {},
	}

	input := []string{}

	accepted, unknown := splitKnownAndUnknown(input, known)

	assert.Empty(t, accepted)
	assert.Empty(t, unknown)
}

// TestAssignmentsUpdate_EmptyGameId 测试空游戏ID验证
func TestAssignmentsUpdate_EmptyGameId(t *testing.T) {
	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	logic := NewAssignmentsUpdateLogic(ctx, svcCtx)

	// 测试空游戏ID - 权限检查会先失败
	req := &AssignmentsUpdateRequest{
		GameId:    "",
		Env:       "dev",
		Functions: []string{"func1"},
	}

	resp, err := logic.AssignmentsUpdate(req)
	assert.Nil(t, resp)
	assert.Error(t, err) // 权限检查失败
}

// TestAssignmentsUpdate_WhitespaceGameId 测试纯空白游戏ID
func TestAssignmentsUpdate_WhitespaceGameId(t *testing.T) {
	ctx := context.Background()
	svcCtx := &svc.ServiceContext{}
	logic := NewAssignmentsUpdateLogic(ctx, svcCtx)

	req := &AssignmentsUpdateRequest{
		GameId:    "   ",
		Env:       "dev",
		Functions: []string{"func1"},
	}

	resp, err := logic.AssignmentsUpdate(req)
	assert.Nil(t, resp)
	assert.Error(t, err) // 权限检查会先失败
}

// TestAssignmentsUpdate_NormalizeFunctions 测试函数名规范化
func TestAssignmentsUpdate_NormalizeFunctions(t *testing.T) {
	// 测试 normalizeFunctions 函数
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "空列表",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "正常函数名",
			input:    []string{"func1", "func2"},
			expected: []string{"func1", "func2"},
		},
		{
			name:     "带空格的函数名",
			input:    []string{" func1 ", " func2 "},
			expected: []string{"func1", "func2"},
		},
		{
			name:     "空字符串和空白",
			input:    []string{"", "  ", "func1", ""},
			expected: []string{"func1"},
		},
		{
			name:     "重复函数名",
			input:    []string{"func1", "func1", "func2"},
			expected: []string{"func1", "func2"}, // normalize会排重
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeFunctions(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSplitKnownAndUnknown_Mixed 测试混合已知和未知函数
func TestSplitKnownAndUnknown_Mixed(t *testing.T) {
	known := map[string]struct{}{
		"func1": {},
		"func2": {},
		"func3": {},
	}

	input := []string{"func1", "unknown1", "func2", "unknown2"}

	accepted, unknown := splitKnownAndUnknown(input, known)

	assert.Equal(t, []string{"func1", "func2"}, accepted)
	assert.Equal(t, []string{"unknown1", "unknown2"}, unknown)
}

// TestSplitKnownAndUnknown_AllUnknown 测试全部未知函数
func TestSplitKnownAndUnknown_AllUnknown(t *testing.T) {
	known := map[string]struct{}{
		"func1": {},
	}

	input := []string{"unknown1", "unknown2", "unknown3"}

	accepted, unknown := splitKnownAndUnknown(input, known)

	assert.Empty(t, accepted)
	assert.Equal(t, input, unknown)
}

// TestSplitKnownAndUnknown_AllKnown 测试全部已知函数
func TestSplitKnownAndUnknown_AllKnown(t *testing.T) {
	known := map[string]struct{}{
		"func1": {},
		"func2": {},
	}

	input := []string{"func1", "func2"}

	accepted, unknown := splitKnownAndUnknown(input, known)

	assert.Equal(t, input, accepted)
	assert.Nil(t, unknown)
}

// TestDiffFunctions_PartialOverlap 测试部分重叠
func TestDiffFunctions_PartialOverlap(t *testing.T) {
	added, removed := diffFunctions(
		[]string{"a", "b", "c"},
		[]string{"b", "c", "d"},
	)
	assert.Equal(t, []string{"d"}, added)
	assert.Equal(t, []string{"a"}, removed)
}

// TestDiffFunctions_Duplicates 测试重复函数
func TestDiffFunctions_Duplicates(t *testing.T) {
	// 测试有重复的情况
	added, removed := diffFunctions(
		[]string{"a", "a", "b"},
		[]string{"a", "b", "b"},
	)
	// 注意：当前实现使用 set，所以重复会被处理
	assert.Empty(t, added)
	assert.Empty(t, removed)
}

// TestCloneAssignments_Empty 测试克隆空分配
func TestCloneAssignments_Empty(t *testing.T) {
	original := map[string][]string{}

	cloned := cloneAssignments(original)

	assert.NotNil(t, cloned)
	assert.Empty(t, cloned)
}
