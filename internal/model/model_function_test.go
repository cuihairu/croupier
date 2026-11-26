package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewFunctionModel(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	assert.NotNil(t, model)
}

func TestFunctionModel_Create(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	fn := &Function{
		FunctionID: "testfunction",
		GameID:     "game1",
		Name:       "Test Function",
		Resource:   "test",
		Status:     1,
	}

	err := model.Create(ctx, fn)
	require.NoError(t, err)
	assert.NotZero(t, fn.ID)
}

func TestFunctionModel_FindByID(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	fn := &Function{
		FunctionID: "findbyid",
		GameID:     "game1",
		Name:       "Find By ID",
		Resource:   "test",
		Status:     1,
	}
	err := model.Create(ctx, fn)
	require.NoError(t, err)

	found, err := model.FindByID(ctx, fn.ID)
	require.NoError(t, err)
	assert.Equal(t, fn.FunctionID, found.FunctionID)
}

func TestFunctionModel_FindByFunctionID(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	fn := &Function{
		FunctionID: "findbyfuncid",
		GameID:     "game1",
		Name:       "Find By Function ID",
		Resource:   "test",
		Status:     1,
	}
	err := model.Create(ctx, fn)
	require.NoError(t, err)

	found, err := model.FindByFunctionID(ctx, "findbyfuncid")
	require.NoError(t, err)
	assert.Equal(t, fn.ID, found.ID)
}

func TestFunctionModel_Update(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	fn := &Function{
		FunctionID: "updatefunction",
		GameID:     "game1",
		Name:       "Update Function",
		Resource:   "test",
		Status:     1,
	}
	err := model.Create(ctx, fn)
	require.NoError(t, err)

	updates := map[string]interface{}{
		"name": "Updated Function Name",
	}
	err = model.Update(ctx, fn.ID, updates)
	require.NoError(t, err)

	found, err := model.FindByID(ctx, fn.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Function Name", found.Name)
}

func TestFunctionModel_Delete(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	fn := &Function{
		FunctionID: "deletefunction",
		GameID:     "game1",
		Name:       "Delete Function",
		Resource:   "test",
		Status:     1,
	}
	err := model.Create(ctx, fn)
	require.NoError(t, err)

	err = model.Delete(ctx, fn.ID)
	require.NoError(t, err)

	_, err = model.FindByID(ctx, fn.ID)
	assert.Error(t, err)
}

func TestFunctionModel_List(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		fn := &Function{
			FunctionID: "listfunction" + string(rune('a'+i)),
			GameID:     "game1",
			Name:       "List Function " + string(rune('a'+i)),
			Resource:   "test",
			Status:     1,
		}
		err := model.Create(ctx, fn)
		require.NoError(t, err)
	}

	functions, total, err := model.List(ctx, ListFunctionsOptions{
		PaginationOptions: PaginationOptions{Page: 1, PageSize: 10},
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(5))
	assert.GreaterOrEqual(t, len(functions), 5)
}

func TestFunctionModel_List_GameIDFilter(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	fn1 := &Function{FunctionID: "game1fn", GameID: "game1", Name: "Game1 Func", Resource: "test", Status: 1}
	fn2 := &Function{FunctionID: "game2fn", GameID: "game2", Name: "Game2 Func", Resource: "test", Status: 1}
	err := model.Create(ctx, fn1)
	require.NoError(t, err)
	err = model.Create(ctx, fn2)
	require.NoError(t, err)

	functions, total, err := model.List(ctx, ListFunctionsOptions{
		PaginationOptions: PaginationOptions{Page: 1, PageSize: 10},
		GameID:            "game1",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	for _, f := range functions {
		assert.Equal(t, "game1", f.GameID)
	}
}

func TestFunctionModel_List_ResourceFilter(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	fn1 := &Function{FunctionID: "cat1fn", GameID: "game1", Name: "Cat1", Resource: "admin", Status: 1}
	fn2 := &Function{FunctionID: "cat2fn", GameID: "game1", Name: "Cat2", Resource: "user", Status: 1}
	err := model.Create(ctx, fn1)
	require.NoError(t, err)
	err = model.Create(ctx, fn2)
	require.NoError(t, err)

	functions, total, err := model.List(ctx, ListFunctionsOptions{
		PaginationOptions: PaginationOptions{Page: 1, PageSize: 10},
		Resource:          "admin",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	for _, f := range functions {
		assert.Equal(t, "admin", f.Resource)
	}
}

func TestFunctionModel_List_StatusFilter(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	status1 := 1
	status0 := 0
	fn1 := &Function{FunctionID: "status1fn", GameID: "game1", Name: "Status1", Resource: "test", Status: 1}
	fn2 := &Function{FunctionID: "status0fn", GameID: "game1", Name: "Status0", Resource: "test", Status: 0}
	err := model.Create(ctx, fn1)
	require.NoError(t, err)
	err = model.Create(ctx, fn2)
	require.NoError(t, err)

	functions, total, err := model.List(ctx, ListFunctionsOptions{
		PaginationOptions: PaginationOptions{Page: 1, PageSize: 10},
		Status:            &status1,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	for _, f := range functions {
		assert.Equal(t, 1, f.Status)
	}

	functions, total, err = model.List(ctx, ListFunctionsOptions{
		PaginationOptions: PaginationOptions{Page: 1, PageSize: 10},
		Status:            &status0,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	for _, f := range functions {
		assert.Equal(t, 0, f.Status)
	}
}

func TestFunctionModel_List_Search(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	fn1 := &Function{FunctionID: "searchfn", GameID: "game1", Name: "Searchable Function", Resource: "test", Status: 1}
	fn2 := &Function{FunctionID: "otherfn", GameID: "game1", Name: "Other Function", Resource: "test", Status: 1}
	err := model.Create(ctx, fn1)
	require.NoError(t, err)
	err = model.Create(ctx, fn2)
	require.NoError(t, err)

	functions, total, err := model.List(ctx, ListFunctionsOptions{
		PaginationOptions: PaginationOptions{Page: 1, PageSize: 10},
		Search:            "searchable",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	found := false
	for _, f := range functions {
		if f.FunctionID == "searchfn" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestFunctionModel_UpsertDescriptor(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	desc := &FunctionDescriptor{
		FunctionID: "upsertdesc",
		Version:    "1.0.0",
		Schema:     map[string]interface{}{"type": "object"},
	}

	err := model.UpsertDescriptor(ctx, desc)
	require.NoError(t, err)

	// Update (upsert)
	desc.Schema = map[string]interface{}{"type": "object", "updated": true}
	err = model.UpsertDescriptor(ctx, desc)
	require.NoError(t, err)
}

func TestFunctionModel_ListDescriptors(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	desc1 := &FunctionDescriptor{FunctionID: "listdesc", Version: "1.0.0"}
	desc2 := &FunctionDescriptor{FunctionID: "listdesc", Version: "2.0.0"}
	err := model.UpsertDescriptor(ctx, desc1)
	require.NoError(t, err)
	err = model.UpsertDescriptor(ctx, desc2)
	require.NoError(t, err)

	descs, err := model.ListDescriptors(ctx, "listdesc")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(descs), 2)
}

func TestFunctionModel_RegisterInstance(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	instance := &FunctionInstance{
		FunctionID: "reginstance",
		AgentID:    "agent1",
		Status:     "running",
	}

	err := model.RegisterInstance(ctx, instance)
	require.NoError(t, err)

	// Upsert again
	instance.Status = "stopped"
	err = model.RegisterInstance(ctx, instance)
	require.NoError(t, err)
}

func TestFunctionModel_ListInstances(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	inst1 := &FunctionInstance{FunctionID: "listinstance", AgentID: "agent1", Status: "running"}
	inst2 := &FunctionInstance{FunctionID: "listinstance", AgentID: "agent2", Status: "running"}
	err := model.RegisterInstance(ctx, inst1)
	require.NoError(t, err)
	err = model.RegisterInstance(ctx, inst2)
	require.NoError(t, err)

	instances, err := model.ListInstances(ctx, "listinstance")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(instances), 2)
}

func TestFunctionModel_ReplacePermissions(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	perms := []FunctionPermission{
		{GameID: "game1", Env: "dev", Resource: "test"},
		{GameID: "game1", Env: "prod", Resource: "test"},
	}

	err := model.ReplacePermissions(ctx, "replacepermfn", perms)
	require.NoError(t, err)

	// Verify
	list, err := model.ListPermissions(ctx, "replacepermfn")
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Replace with different set
	newPerms := []FunctionPermission{{GameID: "game1", Env: "all", Resource: "all"}}
	err = model.ReplacePermissions(ctx, "replacepermfn", newPerms)
	require.NoError(t, err)

	list, err = model.ListPermissions(ctx, "replacepermfn")
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestFunctionModel_ListPermissions(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	perms := []FunctionPermission{
		{Env: "dev", Resource: "test1"},
		{Env: "prod", Resource: "test2"},
	}
	err := model.ReplacePermissions(ctx, "listpermfn", perms)
	require.NoError(t, err)

	list, err := model.ListPermissions(ctx, "listpermfn")
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestFunctionModel_SavePendingFunction(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	pending := &PendingFunction{
		FunctionID: "savependingfn",
		Payload:    map[string]interface{}{"test": true},
	}

	err := model.SavePendingFunction(ctx, pending)
	require.NoError(t, err)

	// Test missing function ID
	invalidPending := &PendingFunction{}
	err = model.SavePendingFunction(ctx, invalidPending)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "function id required")
}

func TestFunctionModel_DeletePending(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	pending := &PendingFunction{
		FunctionID: "deletependingfn",
		Payload:    map[string]interface{}{},
	}
	err := model.SavePendingFunction(ctx, pending)
	require.NoError(t, err)

	err = model.DeletePending(ctx, "deletependingfn")
	require.NoError(t, err)

	list, err := model.ListPending(ctx)
	require.NoError(t, err)
	for _, p := range list {
		assert.NotEqual(t, "deletependingfn", p.FunctionID)
	}
}

func TestFunctionModel_ListPending(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	pending1 := &PendingFunction{FunctionID: "listpending1", Payload: map[string]interface{}{}}
	pending2 := &PendingFunction{FunctionID: "listpending2", Payload: map[string]interface{}{}}
	err := model.SavePendingFunction(ctx, pending1)
	require.NoError(t, err)
	err = model.SavePendingFunction(ctx, pending2)
	require.NoError(t, err)

	list, err := model.ListPending(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
}

func TestFunctionModel_DeleteFunction(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	fn := &Function{
		FunctionID: "deletebyfuncid",
		GameID:     "game1",
		Name:       "Delete By Function ID",
		Resource:   "test",
		Status:     1,
	}
	err := model.Create(ctx, fn)
	require.NoError(t, err)

	err = model.DeleteFunction(ctx, "deletebyfuncid")
	require.NoError(t, err)

	_, err = model.FindByFunctionID(ctx, "deletebyfuncid")
	assert.Error(t, err)
}

func TestFunctionModel_CopyFunction(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	fn := &Function{
		FunctionID: "copyoriginal",
		GameID:     "game1",
		Name:       "Copy Original",
		Resource:   "test",
		Status:     1,
	}
	err := model.Create(ctx, fn)
	require.NoError(t, err)

	newID, err := model.CopyFunction(ctx, "copyoriginal")
	require.NoError(t, err)
	assert.Contains(t, newID, "copyoriginal_copy")

	// Verify the copy exists
	copied, err := model.FindByFunctionID(ctx, newID)
	require.NoError(t, err)
	assert.Equal(t, "Copy Original", copied.Name)
	assert.NotEqual(t, fn.ID, copied.ID)
}

func TestFunctionModel_CopyFunction_NotFound(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	_, err := model.CopyFunction(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestFunctionModel_BatchUpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	fn1 := &Function{FunctionID: "batchstatus1", GameID: "game1", Name: "Batch1", Resource: "test", Status: 1}
	fn2 := &Function{FunctionID: "batchstatus2", GameID: "game1", Name: "Batch2", Resource: "test", Status: 1}
	err := model.Create(ctx, fn1)
	require.NoError(t, err)
	err = model.Create(ctx, fn2)
	require.NoError(t, err)

	affected, failed, err := model.BatchUpdateStatus(ctx, []string{"batchstatus1", "batchstatus2"}, false)
	require.NoError(t, err)
	assert.Equal(t, 2, affected)
	assert.Nil(t, failed)

	updated1, err := model.FindByFunctionID(ctx, "batchstatus1")
	require.NoError(t, err)
	assert.Equal(t, StatusDisabled, updated1.Status)

	updated2, err := model.FindByFunctionID(ctx, "batchstatus2")
	require.NoError(t, err)
	assert.Equal(t, StatusDisabled, updated2.Status)

	affected, failed, err = model.BatchUpdateStatus(ctx, []string{"batchstatus1"}, true)
	require.NoError(t, err)
	assert.Equal(t, 1, affected)
	assert.Nil(t, failed)

	updated1, err = model.FindByFunctionID(ctx, "batchstatus1")
	require.NoError(t, err)
	assert.Equal(t, StatusEnabled, updated1.Status)
}

func TestFunctionModel_BatchUpdateStatus_Empty(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	affected, _, err := model.BatchUpdateStatus(ctx, []string{}, true)
	require.NoError(t, err)
	assert.Equal(t, 0, affected)
}

func TestFunctionModel_BatchDeleteFunctions(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	fn1 := &Function{FunctionID: "batchdel1", GameID: "game1", Name: "BatchDel1", Resource: "test", Status: 1}
	fn2 := &Function{FunctionID: "batchdel2", GameID: "game1", Name: "BatchDel2", Resource: "test", Status: 1}
	err := model.Create(ctx, fn1)
	require.NoError(t, err)
	err = model.Create(ctx, fn2)
	require.NoError(t, err)

	affected, _, err := model.BatchDeleteFunctions(ctx, []string{"batchdel1", "batchdel2"})
	require.NoError(t, err)
	assert.Equal(t, 2, affected)

	// Verify deletion
	_, err = model.FindByFunctionID(ctx, "batchdel1")
	assert.Error(t, err)
	_, err = model.FindByFunctionID(ctx, "batchdel2")
	assert.Error(t, err)
}

func TestFunctionModel_BatchDeleteFunctions_Empty(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	affected, _, err := model.BatchDeleteFunctions(ctx, []string{})
	require.NoError(t, err)
	assert.Equal(t, 0, affected)
}

func TestFunctionModel_BatchCopyFunctions(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	fn1 := &Function{FunctionID: "batchcopy1", GameID: "game1", Name: "BatchCopy1", Resource: "test", Status: 1}
	fn2 := &Function{FunctionID: "batchcopy2", GameID: "game1", Name: "BatchCopy2", Resource: "test", Status: 1}
	err := model.Create(ctx, fn1)
	require.NoError(t, err)
	err = model.Create(ctx, fn2)
	require.NoError(t, err)

	copied, failed, newIDs := model.BatchCopyFunctions(ctx, []string{"batchcopy1", "batchcopy2"})
	assert.Equal(t, 2, copied)
	assert.Len(t, failed, 0)
	assert.Len(t, newIDs, 2)

	// Verify copies exist
	for _, id := range newIDs {
		_, err := model.FindByFunctionID(ctx, id)
		assert.NoError(t, err)
	}
}

func TestFunctionModel_BatchCopyFunctions_Empty(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	copied, failed, newIDs := model.BatchCopyFunctions(ctx, []string{})
	assert.Equal(t, 0, copied)
	assert.Len(t, failed, 0)
	assert.Len(t, newIDs, 0)
}

// 设计债回归（BatchCopyFunctions 恒返 nil error 已删除）：部分 id 失败时
// 进 failedIDs 记录、其余继续复制——失败项在前也不断批次，且不再有
// error 返回值可检查。

// 设计债回归（BatchCopyFunctions 恒返 nil error 已删除）：部分 id 失败时
// 进 failedIDs 记录、其余继续复制——失败项在前也不断批次，且不再有
// error 返回值可检查。
func TestFunctionModel_BatchCopyFunctions_PartialFailure(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	require.NoError(t, model.Create(ctx, &Function{
		FunctionID: "batchpart1", GameID: "game1", Name: "BatchPart1", Resource: "test", Status: 1,
	}))

	// 失败项（不存在的 id）放在最前，证明后续项仍被继续复制。
	count, failed, copied := model.BatchCopyFunctions(ctx, []string{"missing-id", "batchpart1"})
	assert.Equal(t, 1, count, "仅一条成功复制")
	assert.Equal(t, []string{"missing-id"}, failed, "失败 id 进 failedIDs")
	require.Len(t, copied, 1)
	assert.Contains(t, copied[0], "batchpart1_copy_", "成功项产生 _copy_ 新 id")

	// 复制产物确实落库、可按新 id 查回。
	got, err := model.FindByFunctionID(ctx, copied[0])
	require.NoError(t, err)
	assert.Equal(t, copied[0], got.FunctionID)
}

// ===== Helper Functions Tests =====

func TestFunctionModel_GetPolicy(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	// Create a function first
	fn := &Function{
		FunctionID: "test.getPolicy",
		Name:       "Test Function",
		GameID:     "game1",
		Status:     1,
	}
	err := model.Create(ctx, fn)
	require.NoError(t, err)

	// Create policy
	policy := &FunctionPolicy{
		FunctionID: "test.getPolicy",
		Source:     "manual",
	}
	err = model.UpsertPolicy(ctx, policy)
	require.NoError(t, err)

	// Get policy
	got, err := model.GetPolicy(ctx, "test.getPolicy")
	require.NoError(t, err)
	assert.Equal(t, "test.getPolicy", got.FunctionID)
	assert.Equal(t, "manual", got.Source)
}

func TestFunctionModel_GetPolicy_NotFound(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	_, err := model.GetPolicy(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestFunctionModel_UpsertPolicy(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	policy := &FunctionPolicy{
		FunctionID: "test.upsertPolicy",
		Source:     "auto",
	}
	err := model.UpsertPolicy(ctx, policy)
	require.NoError(t, err)

	// Verify created
	got, err := model.GetPolicy(ctx, "test.upsertPolicy")
	require.NoError(t, err)
	assert.Equal(t, "auto", got.Source)

	// Update policy
	policy.Source = "manual"
	err = model.UpsertPolicy(ctx, policy)
	require.NoError(t, err)

	// Verify updated
	got, err = model.GetPolicy(ctx, "test.upsertPolicy")
	require.NoError(t, err)
	assert.Equal(t, "manual", got.Source)
}

func TestFunctionModel_DeletePolicy(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	// Create policy
	policy := &FunctionPolicy{
		FunctionID: "test.deletePolicy",
		Source:     "manual",
	}
	err := model.UpsertPolicy(ctx, policy)
	require.NoError(t, err)

	// Delete policy
	err = model.DeletePolicy(ctx, "test.deletePolicy")
	require.NoError(t, err)

	// Verify deleted
	_, err = model.GetPolicy(ctx, "test.deletePolicy")
	assert.Error(t, err)
}

func TestFunctionModel_ListPolicies(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	// Create multiple policies
	policies := []*FunctionPolicy{
		{FunctionID: "test.listPolicies1", Source: "manual"},
		{FunctionID: "test.listPolicies2", Source: "auto"},
		{FunctionID: "test.listPolicies3", Source: "manual"},
	}
	for _, p := range policies {
		err := model.UpsertPolicy(ctx, p)
		require.NoError(t, err)
	}

	// List all policies
	all, err := model.ListPolicies(ctx, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(all), 3)

	// List by source
	manual, err := model.ListPolicies(ctx, "manual")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(manual), 2)

	auto, err := model.ListPolicies(ctx, "auto")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(auto), 1)
}

func TestFunctionModel_ListDescriptorTemplates(t *testing.T) {
	db := setupTestDB(t)
	model := NewFunctionModel(db)
	ctx := context.Background()

	// Create descriptors
	descs := []*Descriptor{
		{DescriptorID: "desc1", Name: "Form Template", Category: "form"},
		{DescriptorID: "desc2", Name: "Table Template", Category: "table"},
		{DescriptorID: "desc3", Name: "Another Form", Category: "form"},
	}
	for _, d := range descs {
		err := db.Create(d).Error
		require.NoError(t, err)
	}

	// List all descriptor templates
	templates, err := model.ListDescriptorTemplates(ctx, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(templates), 3)

	// List by category
	formTemplates, err := model.ListDescriptorTemplates(ctx, "form")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(formTemplates), 2)

	tableTemplates, err := model.ListDescriptorTemplates(ctx, "table")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(tableTemplates), 1)
}

// ===== OpenAPISource Tests =====
