// 覆盖目标：workflow 引擎与 SQL 各 store 的错误分支、delegation 服务分支、
// notification sender marshal 失败、MemStore 过滤 continue、validateDefinition
// 边界、超时升级路径与条件求值分支。
package approvals

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- 测试替身 ----

// v10WFStore 在 MockWorkflowStore 上叠加错误注入。
type v10WFStore struct {
	*MockWorkflowStore
	getStepApprovalsErr error
	addStepApprovalErr  error
	listInstancesErr    error
}

func (s *v10WFStore) GetStepApprovals(instanceID string) ([]StepApproval, error) {
	if s.getStepApprovalsErr != nil {
		return nil, s.getStepApprovalsErr
	}
	return s.MockWorkflowStore.GetStepApprovals(instanceID)
}

func (s *v10WFStore) AddStepApproval(instanceID string, approval *StepApproval) error {
	if s.addStepApprovalErr != nil {
		return s.addStepApprovalErr
	}
	return s.MockWorkflowStore.AddStepApproval(instanceID, approval)
}

func (s *v10WFStore) ListInstances(filter WorkflowInstanceFilter, page Page) ([]*WorkflowInstance, int, error) {
	if s.listInstancesErr != nil {
		return nil, 0, s.listInstancesErr
	}
	return s.MockWorkflowStore.ListInstances(filter, page)
}

// v10DelgStore 在 stubDelegationStore 上叠加错误注入。
type v10DelgStore struct {
	stubDelegationStore
	activeBy    map[string][]*Delegation
	activeByErr map[string]error
	activeForFn func(userID string) ([]*Delegation, error)
}

func (s *v10DelgStore) GetActiveDelegationsForUser(userID string) ([]*Delegation, error) {
	if s.activeForFn != nil {
		return s.activeForFn(userID)
	}
	return s.stubDelegationStore.GetActiveDelegationsForUser(userID)
}

func (s *v10DelgStore) GetActiveDelegationsByUser(userID string) ([]*Delegation, error) {
	if err, ok := s.activeByErr[userID]; ok {
		return nil, err
	}
	if d, ok := s.activeBy[userID]; ok {
		return d, nil
	}
	return nil, nil
}

// v10QueryFailAfter 返回一个 gorm query 回调：前 passCount 次放行，之后全部报错。
func v10QueryFailAfter(t *testing.T, db *gorm.DB, table string, passCount int32) {
	t.Helper()
	var calls int32
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("v10:qfail", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == table && atomic.AddInt32(&calls, 1) > passCount {
			_ = tx.AddError(errors.New("injected query failure"))
		}
	}))
}

func v10UpdateFail(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("v10:ufail", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == table {
			_ = tx.AddError(errors.New("injected update failure"))
		}
	}))
}

func v10CreateFail(t *testing.T, db *gorm.DB) {
	t.Helper()
	// AutoMigrate 的 CREATE TABLE 走 tx.Exec（Raw callback），不走 create callback。
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register("v10:cfail", func(tx *gorm.DB) {
		_ = tx.AddError(errors.New("injected create failure"))
	}))
}

func v10NewEngine(store WorkflowStore) *WorkflowEngine {
	return NewWorkflowEngine(store, NewMemStore(), NewMockNotifier())
}

// ---- validateDefinition ----

func TestV10ValidateDefinition_MissingStepID(t *testing.T) {
	e := v10NewEngine(NewMockWorkflowStore())
	err := e.validateDefinition(&WorkflowDefinition{ID: "w", Name: "n", Steps: []ApprovalStep{{ID: "ok"}, {ID: ""}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ID is required")
}

func TestV10ValidateDefinition_BadEscalateTo(t *testing.T) {
	e := v10NewEngine(NewMockWorkflowStore())
	err := e.validateDefinition(&WorkflowDefinition{
		ID:   "w",
		Name: "n",
		Steps: []ApprovalStep{
			{ID: "s1", EscalateTo: "ghost"},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid escalate_to reference")
}

// ---- store.go ----

func TestV10MemStoreList_ActorFilterMismatch(t *testing.T) {
	s := NewMemStore()
	_, err := s.Create(&Approval{ID: "a1", Actor: "alice", Mode: "invoke"})
	require.NoError(t, err)
	items, total, err := s.List(Filter{Actor: "bob", Mode: "invoke"}, Page{})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, 0, total)
}

func TestV10NewPGStore_UnreachableDSN(t *testing.T) {
	_, err := NewPGStore("postgres://u:p@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	require.Error(t, err)
}

func TestV10NewSQLiteStore_Success(t *testing.T) {
	store, err := NewSQLiteStore(t.TempDir() + "/v10.db")
	require.NoError(t, err)
	assert.NotNil(t, store)
}

// ---- sql_store.go ----

func TestV10SQLStore_NewStoreMigrateError(t *testing.T) {
	db := openTestDB(t)
	v10CreateFail(t, db)
	_, err := NewSQLStore(db)
	require.Error(t, err)
}

func TestV10SQLStore_ListFindErrorAfterCount(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)
	_, err = store.Create(&Approval{ID: "x1", State: "pending"})
	require.NoError(t, err)

	// Count(第1次 query) 放行，Find(第2次 query) 报错。
	v10QueryFailAfter(t, db, "approvals", 1)
	_, _, err = store.List(Filter{}, Page{})
	require.Error(t, err)
}

func TestV10SQLStore_ApproveRejectSaveError(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLStore(db)
	require.NoError(t, err)
	created, err := store.Create(&Approval{ID: "sv1", State: "pending"})
	require.NoError(t, err)

	v10UpdateFail(t, db, "approvals")
	_, err = store.Approve(created.ID, "op")
	require.Error(t, err)
	_, err = store.Reject(created.ID, "reason", "op")
	require.Error(t, err)
}

// ---- workflow_store.go ----

func TestV10WorkflowStore_NewStoreMigrateError(t *testing.T) {
	db := openTestDB(t)
	v10CreateFail(t, db)
	_, err := NewSQLWorkflowStore(db)
	require.Error(t, err)
}

func v10BadChanCondition() []ConditionGroup {
	return []ConditionGroup{{Conditions: []Condition{{Field: "f", Value: make(chan int)}}}}
}

func TestV10WorkflowStore_CreateUpdateDefinitionMarshalError(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)

	_, err = store.CreateDefinition(&WorkflowDefinition{
		ID: "w1", Name: "n", Steps: []ApprovalStep{{ID: "s1", Conditions: v10BadChanCondition()}},
	})
	require.Error(t, err)

	_, err = store.UpdateDefinition(&WorkflowDefinition{
		ID: "w2", Name: "n", Steps: []ApprovalStep{{ID: "s1", Conditions: v10BadChanCondition()}},
	})
	require.Error(t, err)
}

func TestV10WorkflowStore_ListDefinitionsBadRow(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	require.NoError(t, db.Create(&WorkflowDefinitionModel{ID: "bad", Name: "n", StepsJSON: []byte("not-json")}).Error)

	_, err = store.ListDefinitions(false)
	require.Error(t, err)
}

func TestV10WorkflowStore_CreateInstanceMarshalError(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	_, err = store.CreateInstance(&WorkflowInstance{
		ID: "i1", DefinitionID: "d", Context: map[string]interface{}{"bad": make(chan int)},
	})
	require.Error(t, err)
}

func TestV10WorkflowStore_UpdateInstanceMarshalError(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	_, err = store.UpdateInstance(&WorkflowInstance{
		ID: "i9", DefinitionID: "d", Context: map[string]interface{}{"bad": make(chan int)},
	})
	require.Error(t, err)
}

func TestV10WorkflowStore_UpdateInstanceSaveError(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	inst, err := store.CreateInstance(&WorkflowInstance{ID: "i2", DefinitionID: "d", ApprovalID: "a"})
	require.NoError(t, err)

	v10UpdateFail(t, db, "workflow_instances")
	_, err = store.UpdateInstance(inst)
	require.Error(t, err)
}

func TestV10WorkflowStore_ListInstancesFindError(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	_, err = store.CreateInstance(&WorkflowInstance{ID: "i3", DefinitionID: "d", ApprovalID: "a"})
	require.NoError(t, err)

	v10QueryFailAfter(t, db, "workflow_instances", 1)
	_, _, err = store.ListInstances(WorkflowInstanceFilter{}, Page{})
	require.Error(t, err)
}

func TestV10WorkflowStore_ListInstancesBadRow(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	require.NoError(t, db.Create(&WorkflowInstanceModel{ID: "bad", DefinitionID: "d", ContextJSON: []byte("not-json")}).Error)

	_, _, err = store.ListInstances(WorkflowInstanceFilter{}, Page{})
	require.Error(t, err)
}

func v10BadConstraints() []DelegationConstraint {
	return []DelegationConstraint{{Type: "t", Value: make(chan int), Enforced: true}}
}

func TestV10DelegationStore_CreateUpdateMarshalError(t *testing.T) {
	db := openTestDB(t)
	_, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	store, err := NewSQLDelegationStore(db)
	require.NoError(t, err)

	_, err = store.Create(&Delegation{ID: "d1", Permissions: []DelegationPermission{PermApprove}, Constraints: v10BadConstraints()})
	require.Error(t, err)

	_, err = store.Update(&Delegation{ID: "d2", Permissions: []DelegationPermission{PermApprove}, Constraints: v10BadConstraints()})
	require.Error(t, err)
}

func TestV10DelegationStore_ListFindErrorAndBadRow(t *testing.T) {
	db := openTestDB(t)
	_, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	store, err := NewSQLDelegationStore(db)
	require.NoError(t, err)

	_, err = store.Create(&Delegation{ID: "d-ok", Delegator: "a", Delegate: "b", State: DelegationStateActive, Permissions: []DelegationPermission{PermApprove}})
	require.NoError(t, err)

	t.Run("find error after count", func(t *testing.T) {
		v10QueryFailAfter(t, db, "approval_delegations", 1)
		_, _, err = store.List(DelegationFilter{}, Page{})
		require.Error(t, err)
	})
}

func TestV10DelegationStore_BadRowsToDelegation(t *testing.T) {
	db := openTestDB(t)
	_, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	store, err := NewSQLDelegationStore(db)
	require.NoError(t, err)

	// 坏行：Permissions JSON 非法 → ToDelegation 失败。
	require.NoError(t, db.Create(&DelegationModel{
		ID: "d-bad", Delegator: "a", Delegate: "b",
		State: string(DelegationStateActive), Permissions: []byte("not-json"),
	}).Error)

	_, _, err = store.List(DelegationFilter{}, Page{})
	require.Error(t, err)

	_, err = store.GetActiveDelegationsForUser("b")
	require.Error(t, err)
	_, err = store.GetActiveDelegationsByUser("a")
	require.Error(t, err)
}

func TestV10NotificationStore_BadEventJSON(t *testing.T) {
	db := openTestDB(t)
	_, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	store, err := NewSQLNotificationStore(db)
	require.NoError(t, err)
	require.NoError(t, db.Create(&NotificationModel{Recipient: "u", Channel: "inapp", EventJSON: []byte("not-json")}).Error)

	records, err := store.GetNotifications("u", 0)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Nil(t, records[0]) // 损坏行 continue 后保持 nil 占位
}

// ---- notification senders ----

func TestV10WebhookSender_MarshalError(t *testing.T) {
	w := NewWebhookSender("http://127.0.0.1:1/hook", "secret", "ops")
	err := w.Send(context.Background(), "u", NotificationEvent{
		Title: "t",
		Data:  map[string]interface{}{"bad": make(chan int)},
	})
	require.Error(t, err)
}

// ---- workflow engine ----

func v10Ctx() context.Context { return context.Background() }

func TestV10Engine_StartWorkflow_CreateInstanceError(t *testing.T) {
	db := openTestDB(t)
	store, err := NewSQLWorkflowStore(db)
	require.NoError(t, err)
	_, err = store.CreateDefinition(&WorkflowDefinition{
		ID: "wf-x", Name: "x", Active: true, Steps: []ApprovalStep{{ID: "s1", Approvers: []string{"a"}}},
	})
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable("workflow_instances"))

	eng := NewWorkflowEngine(store, NewMemStore(), NewMockNotifier())
	_, err = eng.StartWorkflow(v10Ctx(), "wf-x", &Approval{ID: "ap-x", Actor: "u"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create workflow instance")
}

func TestV10Engine_ApproveStep_Errors(t *testing.T) {
	t.Run("instance missing", func(t *testing.T) {
		e := v10NewEngine(NewMockWorkflowStore())
		_, err := e.ApproveStep(v10Ctx(), "no-such", "a", "", "", "")
		require.Error(t, err)
	})

	t.Run("definition missing", func(t *testing.T) {
		store := NewMockWorkflowStore()
		store.instances["i1"] = &WorkflowInstance{ID: "i1", DefinitionID: "ghost", State: WorkflowStatePending}
		e := v10NewEngine(store)
		_, err := e.ApproveStep(v10Ctx(), "i1", "a", "", "", "")
		require.Error(t, err)
	})

	t.Run("get step approvals error", func(t *testing.T) {
		store := &v10WFStore{MockWorkflowStore: NewMockWorkflowStore(), getStepApprovalsErr: errors.New("boom")}
		store.instances["i2"] = &WorkflowInstance{
			ID: "i2", DefinitionID: "wf", State: WorkflowStatePending,
			Definition: &WorkflowDefinition{ID: "wf", Steps: []ApprovalStep{{ID: "s1", Approvers: []string{"a"}}}},
		}
		e := v10NewEngine(store)
		_, err := e.ApproveStep(v10Ctx(), "i2", "a", "", "", "")
		require.Error(t, err)
	})

	t.Run("add step approval error", func(t *testing.T) {
		store := &v10WFStore{MockWorkflowStore: NewMockWorkflowStore(), addStepApprovalErr: errors.New("boom")}
		store.instances["i3"] = &WorkflowInstance{
			ID: "i3", DefinitionID: "wf", State: WorkflowStatePending,
			Definition: &WorkflowDefinition{ID: "wf", Steps: []ApprovalStep{{ID: "s1", Approvers: []string{"a"}}}},
		}
		e := v10NewEngine(store)
		_, err := e.ApproveStep(v10Ctx(), "i3", "a", "", "", "")
		require.Error(t, err)
	})
}

func TestV10Engine_ApproveStep_AdvanceWithTimeout(t *testing.T) {
	store := NewMockWorkflowStore()
	def := &WorkflowDefinition{
		ID: "wf-two", Name: "two", Active: true,
		Steps: []ApprovalStep{
			{ID: "s1", Approvers: []string{"a"}},
			{ID: "s2", Approvers: []string{"b"}, Timeout: time.Hour},
		},
	}
	store.definitions["wf-two"] = def
	inst := &WorkflowInstance{ID: "i-tw", DefinitionID: "wf-two", State: WorkflowStatePending, ApprovalID: "ap-tw", Definition: def}
	store.instances["i-tw"] = inst

	e := v10NewEngine(store)
	got, err := e.ApproveStep(v10Ctx(), "i-tw", "a", "", "", "")
	require.NoError(t, err)
	assert.Equal(t, 1, got.CurrentStep)
	require.NotNil(t, got.ExpiresAt)
	assert.True(t, got.ExpiresAt.After(time.Now()))
}

func TestV10Engine_ApproveStep_ApprovalStoreErrorIgnored(t *testing.T) {
	store := NewMockWorkflowStore()
	def := &WorkflowDefinition{ID: "wf-one", Name: "one", Active: true, Steps: []ApprovalStep{{ID: "s1", Approvers: []string{"a"}}}}
	store.definitions["wf-one"] = def
	store.instances["i-one"] = &WorkflowInstance{ID: "i-one", DefinitionID: "wf-one", State: WorkflowStatePending, ApprovalID: "missing-approval", Definition: def}

	// approvalStore 是空 MemStore，Approve("missing-approval") 报错但被忽略。
	e := v10NewEngine(store)
	got, err := e.ApproveStep(v10Ctx(), "i-one", "a", "", "", "")
	require.NoError(t, err)
	assert.Equal(t, WorkflowStateApproved, got.State)
}

func TestV10Engine_RejectStep_Errors(t *testing.T) {
	t.Run("instance missing", func(t *testing.T) {
		e := v10NewEngine(NewMockWorkflowStore())
		_, err := e.RejectStep(v10Ctx(), "no-such", "a", "r", "", "")
		require.Error(t, err)
	})

	t.Run("not pending", func(t *testing.T) {
		store := NewMockWorkflowStore()
		store.instances["i4"] = &WorkflowInstance{ID: "i4", State: WorkflowStateApproved}
		e := v10NewEngine(store)
		_, err := e.RejectStep(v10Ctx(), "i4", "a", "r", "", "")
		require.ErrorIs(t, err, ErrInvalidTransition)
	})

	t.Run("definition missing", func(t *testing.T) {
		store := NewMockWorkflowStore()
		store.instances["i5"] = &WorkflowInstance{ID: "i5", DefinitionID: "ghost", State: WorkflowStatePending}
		e := v10NewEngine(store)
		_, err := e.RejectStep(v10Ctx(), "i5", "a", "r", "", "")
		require.Error(t, err)
	})

	t.Run("add step approval error", func(t *testing.T) {
		store := &v10WFStore{MockWorkflowStore: NewMockWorkflowStore(), addStepApprovalErr: errors.New("boom")}
		store.instances["i6"] = &WorkflowInstance{
			ID: "i6", DefinitionID: "wf", State: WorkflowStatePending,
			Definition: &WorkflowDefinition{ID: "wf", Steps: []ApprovalStep{{ID: "s1", Approvers: []string{"a"}}}},
		}
		e := v10NewEngine(store)
		_, err := e.RejectStep(v10Ctx(), "i6", "a", "r", "", "")
		require.Error(t, err)
	})

	t.Run("approval store error ignored", func(t *testing.T) {
		store := NewMockWorkflowStore()
		def := &WorkflowDefinition{ID: "wf", Steps: []ApprovalStep{{ID: "s1", Approvers: []string{"a"}}}}
		store.instances["i7"] = &WorkflowInstance{ID: "i7", DefinitionID: "wf", State: WorkflowStatePending, ApprovalID: "missing", Definition: def}
		e := v10NewEngine(store)
		got, err := e.RejectStep(v10Ctx(), "i7", "a", "r", "", "")
		require.NoError(t, err)
		assert.Equal(t, WorkflowStateRejected, got.State)
	})
}

func TestV10Engine_CancelWorkflow_InstanceMissing(t *testing.T) {
	e := v10NewEngine(NewMockWorkflowStore())
	_, err := e.CancelWorkflow(v10Ctx(), "no-such", "u", "r")
	require.Error(t, err)
}

func TestV10Engine_ProcessTimeouts_ListError(t *testing.T) {
	store := &v10WFStore{MockWorkflowStore: NewMockWorkflowStore(), listInstancesErr: errors.New("boom")}
	e := v10NewEngine(store)
	_, err := e.ProcessTimeouts(v10Ctx())
	require.Error(t, err)
}

func v10ExpiredInstance(def *WorkflowDefinition, action string, stepIdx int) *WorkflowInstance {
	exp := time.Now().Add(-time.Minute)
	return &WorkflowInstance{
		ID: "i-exp", DefinitionID: def.ID, State: WorkflowStatePending,
		CurrentStep: stepIdx, ApprovalID: "missing-approval", ExpiresAt: &exp, Definition: def,
	}
}

func TestV10Engine_ProcessTimeouts_ApproveActionApprovalError(t *testing.T) {
	store := NewMockWorkflowStore()
	def := &WorkflowDefinition{ID: "wf-t1", Steps: []ApprovalStep{{ID: "s1", Approvers: []string{"a"}, TimeoutAction: "approve"}}}
	store.definitions["wf-t1"] = def
	store.instances["i-exp"] = v10ExpiredInstance(def, "approve", 0)

	e := v10NewEngine(store)
	processed, err := e.ProcessTimeouts(v10Ctx())
	require.NoError(t, err)
	require.Len(t, processed, 1)
	assert.Equal(t, WorkflowStateApproved, processed[0].State)
}

func TestV10Engine_ProcessTimeouts_RejectActionApprovalError(t *testing.T) {
	store := NewMockWorkflowStore()
	def := &WorkflowDefinition{ID: "wf-t2", Steps: []ApprovalStep{{ID: "s1", Approvers: []string{"a"}, TimeoutAction: "reject"}}}
	store.definitions["wf-t2"] = def
	store.instances["i-exp"] = v10ExpiredInstance(def, "reject", 0)

	e := v10NewEngine(store)
	processed, err := e.ProcessTimeouts(v10Ctx())
	require.NoError(t, err)
	require.Len(t, processed, 1)
	assert.Equal(t, WorkflowStateExpired, processed[0].State)
}

func TestV10Engine_ProcessTimeouts_EscalateWithTimeout(t *testing.T) {
	store := NewMockWorkflowStore()
	def := &WorkflowDefinition{
		ID: "wf-t3",
		Steps: []ApprovalStep{
			{ID: "s1", Approvers: []string{"a"}, TimeoutAction: "escalate", EscalateTo: "s2"},
			{ID: "s2", Approvers: []string{"b"}, Timeout: time.Hour},
		},
	}
	store.definitions["wf-t3"] = def
	store.instances["i-exp"] = v10ExpiredInstance(def, "escalate", 0)

	e := v10NewEngine(store)
	processed, err := e.ProcessTimeouts(v10Ctx())
	require.NoError(t, err)
	require.Len(t, processed, 1)
	assert.Equal(t, 1, processed[0].CurrentStep)
	require.NotNil(t, processed[0].ExpiresAt)
}

func TestV10Engine_IsAuthorizedApprover_QueryError(t *testing.T) {
	store := &v10WFStore{MockWorkflowStore: NewMockWorkflowStore(), getStepApprovalsErr: errors.New("boom")}
	e := v10NewEngine(store)
	ok := e.isAuthorizedApprover(ApprovalStep{ID: "s1", Approvers: []string{"someone-else"}}, "u", &WorkflowInstance{ID: "ix"})
	assert.False(t, ok)
}

func TestV10Engine_IsStepComplete_QueryError(t *testing.T) {
	store := &v10WFStore{MockWorkflowStore: NewMockWorkflowStore(), getStepApprovalsErr: errors.New("boom")}
	e := v10NewEngine(store)
	assert.False(t, e.isStepComplete(ApprovalStep{ID: "s1"}, &WorkflowInstance{ID: "ix"}))
}

func TestV10Engine_IsStepComplete_PercentageMinimumOne(t *testing.T) {
	store := NewMockWorkflowStore()
	e := v10NewEngine(store)
	// 1 个审批人 × 50% = 0 → 向上保护为 1，未批准时不视为完成。
	step := ApprovalStep{ID: "s1", Type: StepTypePercentage, RequiredCount: 50, Approvers: []string{"a"}}
	assert.False(t, e.isStepComplete(step, &WorkflowInstance{ID: "iy"}))
}

func TestV10Engine_EvaluateCondition_NotEquals(t *testing.T) {
	e := v10NewEngine(NewMockWorkflowStore())
	data := map[string]interface{}{"env": "prod"}
	assert.True(t, e.evaluateCondition(Condition{Field: "env", Operator: CondOpNotEquals, Value: "dev"}, data))
	assert.False(t, e.evaluateCondition(Condition{Field: "env", Operator: CondOpNotEquals, Value: "prod"}, data))
}

// ---- delegation service ----

func TestV10DelegationService_CanDelegate_StoreError(t *testing.T) {
	approvalStore := NewMemStore()
	_, err := approvalStore.Create(&Approval{ID: "ap-1", FunctionID: "fn-1", State: "pending"})
	require.NoError(t, err)

	store := &v10DelgStore{activeForFn: func(string) ([]*Delegation, error) {
		return nil, errors.New("db down")
	}}
	s := NewDelegationService(store, nil, nil)
	s.SetApprovalStore(approvalStore)

	ok, _, err := s.CanDelegate("u", "ap-1", PermApprove)
	require.Error(t, err)
	assert.False(t, ok)
}

func TestV10DelegationService_CanDelegate_ConstraintBlocked(t *testing.T) {
	approvalStore := NewMemStore()
	_, err := approvalStore.Create(&Approval{ID: "ap-1", FunctionID: "fn-1", State: "pending"})
	require.NoError(t, err)

	// 时间限制：允许的星期与今天不同 → checkConstraints false → continue。
	tomorrow := (int(time.Now().Weekday()) + 1) % 7
	store := &v10DelgStore{}
	store.activeFor = map[string][]*Delegation{
		"u": {{
			ID:          "d1",
			Delegator:   "boss",
			Delegate:    "u",
			Scope:       ScopeAll,
			Permissions: []DelegationPermission{PermApprove},
			State:       DelegationStateActive,
			Constraints: []DelegationConstraint{{
				Type:     "time_restriction",
				Enforced: true,
				Value:    map[string]interface{}{"allowed_days": []interface{}{float64(tomorrow)}},
			}},
		}},
	}
	s := NewDelegationService(store, nil, nil)
	s.SetApprovalStore(approvalStore)

	ok, _, err := s.CanDelegate("u", "ap-1", PermApprove)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestV10DelegationService_CheckCircular_SubQueryErrorContinues(t *testing.T) {
	// delegate 已有下游委托；查询其下游时 store 报错 → continue → 无环成立。
	store := &v10DelgStore{
		activeBy: map[string][]*Delegation{
			"carol": {{ID: "d9", Delegator: "carol", Delegate: "dave", State: DelegationStateActive}},
		},
		activeByErr: map[string]error{"dave": errors.New("db down")},
	}
	s := NewDelegationService(store, nil, nil)

	err := s.checkCircularDelegation("alice", "carol")
	require.NoError(t, err)
}
