package resourcecatalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// V9 helpers
// ---------------------------------------------------------------------------

// brokenTableDBV9 returns an in-memory DB where the named table exists but
// lacks every column, so scoped queries fail with "no such column" instead of
// gorm.ErrRecordNotFound / missing-table errors.
func brokenTableDBV9(t *testing.T, table string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE "+table+" (id INTEGER PRIMARY KEY)").Error)
	return db
}

func seedV9Capability(t *testing.T, db *gorm.DB, resourceKey string) {
	t.Helper()
	require.NoError(t, model.NewResourceCapabilityModel(db).UpsertCapability(context.Background(), &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: resourceKey,
	}))
}

func seedV9Contract(t *testing.T, db *gorm.DB, functionID string, capability dbenum.Capability, input, output string) {
	t.Helper()
	require.NoError(t, model.NewFunctionContractModel(db).UpsertContract(context.Background(), &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: functionID, Enabled: true,
		ResourceKey: "player", Capability: capability, Execution: "sync", Risk: dbenum.RiskSafe,
		InputSchema:  model.JSON(input),
		OutputSchema: model.JSON(output),
	}))
}

// seedV9TaskContracts seeds a start function plus optional companion task
// functions sharing the "player" resource key.
func seedV9TaskContracts(t *testing.T, db *gorm.DB, companions ...string) {
	t.Helper()
	seedV9Capability(t, db, "player")
	seedV9Contract(t, db, "player.startTask", dbenum.CapabilityTask,
		`{"type":"object","properties":{"reason":{"type":"string"}}}`,
		`{"type":"object","properties":{"taskId":{"type":"string"}}}`)
	taskInput := `{"type":"object","properties":{"taskId":{"type":"string"}},"required":["taskId"]}`
	taskOutput := `{"type":"object","properties":{"state":{"type":"string"},"events":{"type":"array"},"result":{"type":"object"}}}`
	for _, fn := range companions {
		seedV9Contract(t, db, fn, dbenum.CapabilityTask, taskInput, taskOutput)
	}
}

func validV9Task() spec.TaskSemantic {
	return spec.TaskSemantic{
		Start:  spec.FunctionRef{FunctionID: "player.startTask"},
		TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
		Status: spec.TaskStatusSemantic{
			Function:    spec.FunctionRef{FunctionID: "player.taskStatus"},
			TaskIDInput: "/taskId",
			StatePath:   "/state",
		},
	}
}

// ---------------------------------------------------------------------------
// List error paths and sorting
// ---------------------------------------------------------------------------

func TestServiceListContractModelErrorV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	svc := NewService(db, nil)
	svc.contractModel = model.NewFunctionContractModel(brokenTableDBV9(t, "function_contracts"))

	_, err := svc.List(context.Background(), &ListRequest{GameID: "g1", Env: "e1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list contracts")
}

func TestServiceListSemanticsModelErrorV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	svc := NewService(db, nil)
	svc.semanticsModel = model.NewCapabilitySemanticsModel(brokenTableDBV9(t, "capability_semantics"))

	_, err := svc.List(context.Background(), &ListRequest{GameID: "g1", Env: "e1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find semantics")
}

func TestServiceListSortsByCategoryThenKeyV9(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	capModel := model.NewResourceCapabilityModel(db)
	require.NoError(t, capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "zeta", CategoryKey: "alpha",
	}))
	require.NoError(t, capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "beta", CategoryKey: "alpha",
	}))
	require.NoError(t, capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "aaa", CategoryKey: "zzz",
	}))

	resp, err := NewService(db, nil).List(ctx, &ListRequest{GameID: "g1", Env: "e1"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 3)
	// alpha category sorts before zzz; inside alpha, beta < zeta.
	assert.Equal(t, "beta", resp.Items[0].ResourceKey)
	assert.Equal(t, "zeta", resp.Items[1].ResourceKey)
	assert.Equal(t, "aaa", resp.Items[2].ResourceKey)
}

// ---------------------------------------------------------------------------
// Detail error paths
// ---------------------------------------------------------------------------

func TestServiceDetailContractModelErrorV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	svc := NewService(db, nil)
	svc.contractModel = model.NewFunctionContractModel(brokenTableDBV9(t, "function_contracts"))

	_, err := svc.Detail(context.Background(), &DetailRequest{GameID: "g1", Env: "e1", ResourceKey: "player"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list contracts")
}

func TestServiceDetailSemanticsModelErrorV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	svc := NewService(db, nil)
	svc.semanticsModel = model.NewCapabilitySemanticsModel(brokenTableDBV9(t, "capability_semantics"))

	_, err := svc.Detail(context.Background(), &DetailRequest{GameID: "g1", Env: "e1", ResourceKey: "player"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find semantics")
}

func TestServiceDetailAffectedPagesErrorV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	// page_specs exists but is unusable: ListByScope fails with a non
	// missing-table error that must surface to the caller.
	require.NoError(t, db.Exec("CREATE TABLE page_specs (id INTEGER PRIMARY KEY)").Error)
	svc := NewService(db, nil)

	_, err := svc.Detail(context.Background(), &DetailRequest{GameID: "g1", Env: "e1", ResourceKey: "player"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list page drafts")
}

// ---------------------------------------------------------------------------
// UpdateSemantics error paths
// ---------------------------------------------------------------------------

func TestUpdateSemanticsFindSemanticsErrorV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	svc := NewService(db, nil)
	svc.semanticsModel = model.NewCapabilitySemanticsModel(brokenTableDBV9(t, "capability_semantics"))

	_, err := svc.UpdateSemantics(context.Background(), &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", IdentityField: "id",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find semantics")
}

func TestUpdateSemanticsInvalidBindingIDsV9(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	seedV9Capability(t, db, "player")
	seedV9Contract(t, db, "player.list", dbenum.CapabilityCollectionQuery,
		`{"type":"object","properties":{"page":{"type":"integer"}}}`,
		`{"type":"object","properties":{"items":{"type":"array"}}}`)
	svc := NewService(db, nil)

	list, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "g1", "e1", "player.list")
	require.NoError(t, err)

	_, err = svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", ItemQueryID: list.ID + 100,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid itemQueryId")

	_, err = svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", CreateID: list.ID + 100,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid createId")
}

func TestUpdateSemanticsUpdateIDBindsV9(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	seedV9Capability(t, db, "player")
	seedV9Contract(t, db, "player.update", dbenum.CapabilityUpdate,
		`{"type":"object","properties":{"id":{"type":"string"}}}`,
		`{"type":"object"}`)
	svc := NewService(db, nil)

	contract, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "g1", "e1", "player.update")
	require.NoError(t, err)

	resp, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", UpdateID: contract.ID,
		ChangeReason: "bind update",
	})
	require.NoError(t, err)
	assert.Equal(t, "platform_review", resp.Source)
}

func TestUpdateSemanticsUpsertErrorV9(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	seedV9Capability(t, db, "player")
	// Read-only DB: selects succeed, the semantics upsert fails.
	require.NoError(t, db.Exec("PRAGMA query_only = true").Error)

	_, err := NewService(db, nil).UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", IdentityField: "id",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upsert semantics")
}

func TestUpdateSemanticsCreateVersionErrorV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	svc := NewService(db, nil)
	svc.versionModel = model.NewCapabilitySemanticVersionModel(brokenTableDBV9(t, "capability_semantic_versions"))

	_, err := svc.UpdateSemantics(context.Background(), &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", IdentityField: "id",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create semantic version")
}

func TestUpdateSemanticsRebuildProposalsErrorV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	svc := NewService(db, nil)
	svc.contractService = contractsvc.NewContractService(brokenTableDBV9(t, "capability_semantics"))

	_, err := svc.UpdateSemantics(context.Background(), &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", IdentityField: "id",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rebuild proposals")
}

func TestUpdateSemanticsWritesAuditEventV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	auditSvc := audit.NewAuditService(audit.NewInMemoryAuditStore(), nil)
	svc := NewService(db, auditSvc)

	resp, err := svc.UpdateSemantics(context.Background(), &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", IdentityField: "id", ChangeReason: "audit me",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Message, "semantics updated")
}

// ---------------------------------------------------------------------------
// ListSemanticVersions error path
// ---------------------------------------------------------------------------

func TestListSemanticVersionsModelErrorV9(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	require.NoError(t, model.NewCapabilitySemanticsModel(db).UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID: "g1", Env: "e1", ResourceKey: "player", Source: "sdk",
	}))
	svc := NewService(db, nil)
	svc.versionModel = model.NewCapabilitySemanticVersionModel(brokenTableDBV9(t, "capability_semantic_versions"))

	_, err := svc.ListSemanticVersions(ctx, &ListSemanticVersionsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list semantic versions")
}

// ---------------------------------------------------------------------------
// validateActionSemantics duplicate dedupe
// ---------------------------------------------------------------------------

func TestValidateActionSemanticsDeduplicatesV9(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	seedV9Capability(t, db, "player")
	seedV9Contract(t, db, "player.ban", dbenum.CapabilityAction,
		`{"type":"object","properties":{"id":{"type":"string"}}}`, `{"type":"object"}`)
	svc := NewService(db, nil)

	resp, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Actions: []ActionSemanticInfo{
			{FunctionID: "player.ban", Subject: "resource_item", IdentityInput: "/id"},
			{FunctionID: "player.ban", Subject: "resource_item", IdentityInput: "/id"},
		},
	})
	require.NoError(t, err)

	sem, err := model.NewCapabilitySemanticsModel(db).FindByScopeAndResourceKey(ctx, "g1", "e1", "player")
	require.NoError(t, err)
	var actions []ActionSemanticInfo
	require.NoError(t, json.Unmarshal(sem.Actions, &actions))
	assert.Len(t, actions, 1)
	_ = resp
}

// ---------------------------------------------------------------------------
// validateTaskSemantics error branches
// ---------------------------------------------------------------------------

func TestValidateTaskSemanticsStatusErrorsV9(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	seedV9TaskContracts(t, db, "player.taskStatus")

	cases := []struct {
		name string
		task spec.TaskSemantic
		want string
	}{
		{"status function missing", spec.TaskSemantic{
			Start:  spec.FunctionRef{FunctionID: "player.startTask"},
			TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
			Status: spec.TaskStatusSemantic{Function: spec.FunctionRef{FunctionID: "ghost"}},
		}, "tasks[0].status.function"},
		{"status taskIdInput invalid", spec.TaskSemantic{
			Start:  spec.FunctionRef{FunctionID: "player.startTask"},
			TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
			Status: spec.TaskStatusSemantic{
				Function:    spec.FunctionRef{FunctionID: "player.taskStatus"},
				TaskIDInput: "taskId",
				StatePath:   "/state",
			},
		}, "tasks[0].status.taskIdInput"},
		{"status taskIdInput not in schema", spec.TaskSemantic{
			Start:  spec.FunctionRef{FunctionID: "player.startTask"},
			TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
			Status: spec.TaskStatusSemantic{
				Function:    spec.FunctionRef{FunctionID: "player.taskStatus"},
				TaskIDInput: "/ghost",
				StatePath:   "/state",
			},
		}, "tasks[0].status.taskIdInput"},
		{"status statePath not pointer", spec.TaskSemantic{
			Start:  spec.FunctionRef{FunctionID: "player.startTask"},
			TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
			Status: spec.TaskStatusSemantic{
				Function:    spec.FunctionRef{FunctionID: "player.taskStatus"},
				TaskIDInput: "/taskId",
				StatePath:   "state",
			},
		}, "tasks[0].status.statePath"},
		{"status statePath not in schema", spec.TaskSemantic{
			Start:  spec.FunctionRef{FunctionID: "player.startTask"},
			TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
			Status: spec.TaskStatusSemantic{
				Function:    spec.FunctionRef{FunctionID: "player.taskStatus"},
				TaskIDInput: "/taskId",
				StatePath:   "/ghost",
			},
		}, "tasks[0].status.statePath"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateSemantics(context.Background(), &UpdateSemanticsRequest{
				GameID: "g1", Env: "e1", ResourceKey: "player", Tasks: []spec.TaskSemantic{tc.task},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestValidateTaskSemanticsEventsErrorsV9(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	seedV9TaskContracts(t, db, "player.taskStatus", "player.taskEvents")

	cases := []struct {
		name string
		task spec.TaskSemantic
		want string
	}{
		{"events function missing", func() spec.TaskSemantic {
			task := validV9Task()
			task.Events = &spec.TaskEventsSemantic{Function: spec.FunctionRef{FunctionID: "ghost"}}
			return task
		}(), "tasks[0].events.function"},
		{"events taskIdInput invalid", func() spec.TaskSemantic {
			task := validV9Task()
			task.Events = &spec.TaskEventsSemantic{
				Function:    spec.FunctionRef{FunctionID: "player.taskEvents"},
				TaskIDInput: "taskId",
				EventsPath:  "/events",
			}
			return task
		}(), "tasks[0].events.taskIdInput"},
		{"events taskIdInput not in schema", func() spec.TaskSemantic {
			task := validV9Task()
			task.Events = &spec.TaskEventsSemantic{
				Function:    spec.FunctionRef{FunctionID: "player.taskEvents"},
				TaskIDInput: "/ghost",
				EventsPath:  "/events",
			}
			return task
		}(), "tasks[0].events.taskIdInput"},
		{"events path not pointer", func() spec.TaskSemantic {
			task := validV9Task()
			task.Events = &spec.TaskEventsSemantic{
				Function:    spec.FunctionRef{FunctionID: "player.taskEvents"},
				TaskIDInput: "/taskId",
				EventsPath:  "events",
			}
			return task
		}(), "tasks[0].events.eventsPath"},
		{"events path not in schema", func() spec.TaskSemantic {
			task := validV9Task()
			task.Events = &spec.TaskEventsSemantic{
				Function:    spec.FunctionRef{FunctionID: "player.taskEvents"},
				TaskIDInput: "/taskId",
				EventsPath:  "/ghost",
			}
			return task
		}(), "tasks[0].events.eventsPath"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateSemantics(context.Background(), &UpdateSemanticsRequest{
				GameID: "g1", Env: "e1", ResourceKey: "player", Tasks: []spec.TaskSemantic{tc.task},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestValidateTaskSemanticsResultErrorsV9(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	seedV9TaskContracts(t, db, "player.taskStatus", "player.taskResult")

	cases := []struct {
		name string
		task spec.TaskSemantic
		want string
	}{
		{"result function missing", func() spec.TaskSemantic {
			task := validV9Task()
			task.Result = &spec.TaskResultSemantic{Function: spec.FunctionRef{FunctionID: "ghost"}}
			return task
		}(), "tasks[0].result.function"},
		{"result taskIdInput invalid", func() spec.TaskSemantic {
			task := validV9Task()
			task.Result = &spec.TaskResultSemantic{
				Function:    spec.FunctionRef{FunctionID: "player.taskResult"},
				TaskIDInput: "taskId",
				ResultPath:  "/result",
			}
			return task
		}(), "tasks[0].result.taskIdInput"},
		{"result path not pointer", func() spec.TaskSemantic {
			task := validV9Task()
			task.Result = &spec.TaskResultSemantic{
				Function:    spec.FunctionRef{FunctionID: "player.taskResult"},
				TaskIDInput: "/taskId",
				ResultPath:  "result",
			}
			return task
		}(), "tasks[0].result.resultPath"},
		{"result path not in schema", func() spec.TaskSemantic {
			task := validV9Task()
			task.Result = &spec.TaskResultSemantic{
				Function:    spec.FunctionRef{FunctionID: "player.taskResult"},
				TaskIDInput: "/taskId",
				ResultPath:  "/ghost",
			}
			return task
		}(), "tasks[0].result.resultPath"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateSemantics(context.Background(), &UpdateSemanticsRequest{
				GameID: "g1", Env: "e1", ResourceKey: "player", Tasks: []spec.TaskSemantic{tc.task},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestValidateTaskSemanticsCancelErrorsV9(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	seedV9TaskContracts(t, db, "player.taskStatus", "player.taskCancel")

	cases := []struct {
		name string
		task spec.TaskSemantic
		want string
	}{
		{"cancel function missing", func() spec.TaskSemantic {
			task := validV9Task()
			task.Cancel = &spec.TaskCommandSemantic{Function: spec.FunctionRef{FunctionID: "ghost"}}
			return task
		}(), "tasks[0].cancel.function"},
		{"cancel taskIdInput invalid", func() spec.TaskSemantic {
			task := validV9Task()
			task.Cancel = &spec.TaskCommandSemantic{
				Function:    spec.FunctionRef{FunctionID: "player.taskCancel"},
				TaskIDInput: "taskId",
			}
			return task
		}(), "tasks[0].cancel.taskIdInput"},
		{"cancel taskIdInput not in schema", func() spec.TaskSemantic {
			task := validV9Task()
			task.Cancel = &spec.TaskCommandSemantic{
				Function:    spec.FunctionRef{FunctionID: "player.taskCancel"},
				TaskIDInput: "/ghost",
			}
			return task
		}(), "tasks[0].cancel.taskIdInput"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateSemantics(context.Background(), &UpdateSemanticsRequest{
				GameID: "g1", Env: "e1", ResourceKey: "player", Tasks: []spec.TaskSemantic{tc.task},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestValidateTaskSemanticsDeduplicatesV9(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db, nil)
	seedV9TaskContracts(t, db, "player.taskStatus")

	resp, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Tasks: []spec.TaskSemantic{validV9Task(), validV9Task()},
	})
	require.NoError(t, err)

	sem, err := model.NewCapabilitySemanticsModel(db).FindByScopeAndResourceKey(ctx, "g1", "e1", "player")
	require.NoError(t, err)
	var tasks []spec.TaskSemantic
	require.NoError(t, json.Unmarshal(sem.Tasks, &tasks))
	assert.Len(t, tasks, 1)
	_ = resp
}

// ---------------------------------------------------------------------------
// validateReportSemantics error branches
// ---------------------------------------------------------------------------

func seedV9ReportContract(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedV9Capability(t, db, "player")
	seedV9Contract(t, db, "player.report", dbenum.CapabilityReport,
		`{"type":"object","properties":{"date":{"type":"string"}}}`,
		`{"type":"object","properties":{"rows":{"type":"array","items":{"type":"object","properties":{"date":{"type":"string"},"count":{"type":"integer"}}}}}}`)
}

func validV9Report() spec.ReportSemantic {
	return spec.ReportSemantic{
		Query:       spec.FunctionRef{FunctionID: "player.report"},
		DatasetPath: "/rows",
		Dimensions:  []string{"/date"},
		Metrics:     []string{"/count"},
	}
}

func TestValidateReportSemanticsPointerListErrorsV9(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	seedV9ReportContract(t, db)

	cases := []struct {
		name   string
		report spec.ReportSemantic
		want   string
	}{
		{"dimensions not pointers", func() spec.ReportSemantic {
			report := validV9Report()
			report.Dimensions = []string{"date"}
			return report
		}(), "reports[0].dimensions"},
		{"metrics not pointers", func() spec.ReportSemantic {
			report := validV9Report()
			report.Metrics = []string{"count"}
			return report
		}(), "reports[0].metrics"},
		{"dimension pointer missing in item schema", func() spec.ReportSemantic {
			report := validV9Report()
			report.Dimensions = []string{"/ghost"}
			return report
		}(), "reports[0].dimensions"},
		{"metric pointer missing in item schema", func() spec.ReportSemantic {
			report := validV9Report()
			report.Metrics = []string{"/ghost"}
			return report
		}(), "reports[0].metrics"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateSemantics(context.Background(), &UpdateSemanticsRequest{
				GameID: "g1", Env: "e1", ResourceKey: "player", Reports: []spec.ReportSemantic{tc.report},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestValidateReportSemanticsDeduplicatesV9(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db, nil)
	seedV9ReportContract(t, db)

	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Reports: []spec.ReportSemantic{validV9Report(), validV9Report()},
	})
	require.NoError(t, err)

	sem, err := model.NewCapabilitySemanticsModel(db).FindByScopeAndResourceKey(ctx, "g1", "e1", "player")
	require.NoError(t, err)
	var reports []spec.ReportSemantic
	require.NoError(t, json.Unmarshal(sem.Reports, &reports))
	assert.Len(t, reports, 1)
}

// ---------------------------------------------------------------------------
// validateSemanticFunctionRef lookup error
// ---------------------------------------------------------------------------

func TestValidateSemanticFunctionRefLookupErrorV9(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	svc.contractModel = model.NewFunctionContractModel(brokenTableDBV9(t, "function_contracts"))

	_, err := svc.validateSemanticFunctionRef(context.Background(), "g1", "e1", "player",
		spec.FunctionRef{FunctionID: "player.any"}, spec.CapabilityTask, "tasks[0].start")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tasks[0].start.functionId")
}

// ---------------------------------------------------------------------------
// schema pointer helpers
// ---------------------------------------------------------------------------

func TestSchemaObjectHasPointerNoPropertiesV9(t *testing.T) {
	// Root schema without properties: any pointer fails to resolve.
	root := map[string]json.RawMessage{"type": json.RawMessage(`"string"`)}
	assert.False(t, schemaObjectHasPointer(root, "/a"))
	// Intermediate node without properties also fails.
	nested := []byte(`{"type":"object","properties":{"a":{"type":"string"}}}`)
	assert.False(t, schemaHasPointer(nested, "/a/b"))
}

func TestArrayItemSchemaAtPointerEdgeCasesV9(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"data":{"type":"array","items":{"type":"object"}}}}`)

	_, ok := arrayItemSchemaAtPointer([]byte(`not-json`), "/data")
	assert.False(t, ok)

	// Token walk hits a missing property before reaching the array.
	_, ok = arrayItemSchemaAtPointer(schema, "/ghost/data")
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// buildAffectedPages error paths and same-kind ordering
// ---------------------------------------------------------------------------

func TestBuildAffectedPagesDraftModelErrorV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	require.NoError(t, db.Exec("CREATE TABLE page_specs (id INTEGER PRIMARY KEY)").Error)

	_, err := NewService(db, nil).buildAffectedPages(context.Background(), "g1", "e1", "player")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list page drafts")
}

// createV9PageDraftsStub creates a page_specs table that satisfies the scoped
// list query (WHERE/ORDER columns only) so later lookups can be broken
// selectively.
func createV9PageDraftsStub(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`CREATE TABLE page_specs (
		game_id TEXT, env TEXT, category_order INTEGER, "order" INTEGER,
		page_key TEXT, deleted_at DATETIME)`).Error)
}

func TestBuildAffectedPagesPublishedModelErrorV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	createV9PageDraftsStub(t, db)
	require.NoError(t, db.Exec("CREATE TABLE published_page_specs (id INTEGER PRIMARY KEY)").Error)

	_, err := NewService(db, nil).buildAffectedPages(context.Background(), "g1", "e1", "player")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list published pages")
}

func TestBuildAffectedPagesProposalModelErrorV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	createV9PageDraftsStub(t, db)
	require.NoError(t, db.Exec("CREATE TABLE published_page_specs (game_id TEXT, env TEXT, page_key TEXT, version INTEGER)").Error)
	// setupTestDB already migrated the proposals table; replace it with a
	// stub that cannot serve the scoped query.
	require.NoError(t, db.Migrator().DropTable("page_proposals"))
	require.NoError(t, db.Exec("CREATE TABLE page_proposals (id INTEGER PRIMARY KEY)").Error)

	_, err := NewService(db, nil).buildAffectedPages(context.Background(), "g1", "e1", "player")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list proposals")
}

func TestBuildAffectedPagesOrdersSameKindByPageKeyV9(t *testing.T) {
	db := setupTestDBWithPages(t)
	ctx := context.Background()
	pageModel := model.NewPageSpecModel(db)
	require.NoError(t, pageModel.Upsert(ctx, &model.PageSpec{
		GameID: "g1", Env: "e1", PageKey: "resource--zeta", Type: "resource",
		ResourceKey: "player", Status: "draft", DraftRevision: 1,
	}))
	require.NoError(t, pageModel.Upsert(ctx, &model.PageSpec{
		GameID: "g1", Env: "e1", PageKey: "resource--alpha", Type: "resource",
		ResourceKey: "player", Status: "draft", DraftRevision: 1,
	}))

	items, err := NewService(db, nil).buildAffectedPages(ctx, "g1", "e1", "player")
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "resource--alpha", items[0].PageKey)
	assert.Equal(t, "resource--zeta", items[1].PageKey)
}

// ---------------------------------------------------------------------------
// functionSpecsByID / semanticSourceDigest / labelsForResource
// ---------------------------------------------------------------------------

func TestFunctionSpecsByIDModelErrorV9(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	svc.contractModel = model.NewFunctionContractModel(brokenTableDBV9(t, "function_contracts"))

	out := svc.functionSpecsByID(context.Background(), "g1", "e1")
	assert.Empty(t, out)
}

func TestSemanticSourceDigestListErrorV9(t *testing.T) {
	broken := model.NewFunctionContractModel(brokenTableDBV9(t, "function_contracts"))
	assert.Empty(t, semanticSourceDigest(context.Background(), broken, "g1", "e1", "player"))
}

func TestLabelsForResourceNilOnUnusableKeyV9(t *testing.T) {
	assert.Nil(t, labelsForResource(nil, "___"))
	assert.Nil(t, labelsForResource(nil, " . "))
}

// ---------------------------------------------------------------------------
// ResolveConflict error branches
// ---------------------------------------------------------------------------

func seedV9ConflictSemantics(t *testing.T, db *gorm.DB, conflicts []byte) {
	t.Helper()
	require.NoError(t, model.NewCapabilitySemanticsModel(db).UpsertSemantics(context.Background(), &model.CapabilitySemantics{
		GameID: "g1", Env: "e1", ResourceKey: "player", Source: "sdk", Conflicts: conflicts,
	}))
}

func TestResolveConflictInvalidConflictsPayloadV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9ConflictSemantics(t, db, []byte(`{not-json`))

	_, err := NewService(db, nil).ResolveConflict(context.Background(), &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Field: "identityField", ChosenSource: "sdk_explicit",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse conflicts")
}

func TestResolveConflictUnsupportedFieldV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9ConflictSemantics(t, db, []byte(`[{"field":"mysteryField","values":{"sdk_explicit":"v"}}]`))

	_, err := NewService(db, nil).ResolveConflict(context.Background(), &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Field: "mysteryField", ChosenSource: "sdk_explicit",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported semantic conflict field")
}

func TestResolveConflictUpsertErrorV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9ConflictSemantics(t, db, []byte(`[{"field":"identityField","values":{"sdk_explicit":"id"}}]`))
	require.NoError(t, db.Exec("PRAGMA query_only = true").Error)

	_, err := NewService(db, nil).ResolveConflict(context.Background(), &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Field: "identityField", ChosenSource: "sdk_explicit",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update semantics")
}

func TestResolveConflictCreateVersionErrorV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9ConflictSemantics(t, db, []byte(`[{"field":"identityField","values":{"sdk_explicit":"id"}}]`))
	svc := NewService(db, nil)
	svc.versionModel = model.NewCapabilitySemanticVersionModel(brokenTableDBV9(t, "capability_semantic_versions"))

	_, err := svc.ResolveConflict(context.Background(), &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Field: "identityField", ChosenSource: "sdk_explicit",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create semantic version")
}

func TestResolveConflictWritesAuditEventV9(t *testing.T) {
	db := setupTestDB(t)
	seedV9ConflictSemantics(t, db, []byte(`[{"field":"identityField","values":{"sdk_explicit":"id"}}]`))
	svc := NewService(db, audit.NewAuditService(audit.NewInMemoryAuditStore(), nil))

	resp, err := svc.ResolveConflict(context.Background(), &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Field: "identityField", ChosenSource: "sdk_explicit", Reason: "v9",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Message, "Conflict resolved")
}

// ---------------------------------------------------------------------------
// Handler URI binding failures (called without route params)
// ---------------------------------------------------------------------------

func newBareGinContextV9(t *testing.T, method, target string, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, target, http.NoBody)
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	ctx.Request = req
	return ctx
}

func TestHandlerBindURIErrorsV9(t *testing.T) {
	db := setupTestDB(t)
	handler := NewHandler(NewService(db, nil))

	// No URI params are registered on the context, so every URI bind fails.
	assert.NotPanics(t, func() { handler.Detail(newBareGinContextV9(t, http.MethodGet, "/", "")) })
	assert.NotPanics(t, func() {
		handler.UpdateSemantics(newBareGinContextV9(t, http.MethodPut, "/", ""))
	})
	assert.NotPanics(t, func() {
		handler.ListSemanticVersions(newBareGinContextV9(t, http.MethodGet, "/", ""))
	})
	assert.NotPanics(t, func() { handler.ListConflicts(newBareGinContextV9(t, http.MethodGet, "/", "")) })
	assert.NotPanics(t, func() {
		handler.ResolveConflict(newBareGinContextV9(t, http.MethodPost, "/", ""))
	})
}

// ---------------------------------------------------------------------------
// Scope propagation sanity (game scope from context)
// ---------------------------------------------------------------------------

func TestGetScopeFromRequestContextV9(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(svc.WithGameScope(req.Context(), svc.GameScope{GameID: "", Env: ""}))
	ctx.Request = req

	gameID, env := getScope(ctx)
	assert.Empty(t, gameID)
	assert.Empty(t, env)
}
