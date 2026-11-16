// Command demo-game is the persistent demo game: a simulated integrator that
// registers the exact game-ops contract surface through the SDK→Agent chain,
// backed by a durable exactly-once store (store package) so GAME-002~006 /
// UI-001 game-side acceptance flows can run without a real game server.
//
// The 34 function contracts below are a verbatim copy of
// examples/gameops-demo/contracts.go (IDs, schemas, envelope shapes). The
// three-way drift lock in contracts_drift_test.go fails the build if any
// surface diverges: gameops-demo == demo-game == template bindings(33) +
// report.generate.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/sdks/go/cmd/demo-game/store"
	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

// schema constants mirror the E1 web contract shapes (list output keeps
// {items,total,page,pageSize}, so the renderer's /items,/total selectors work).
const (
	playerSchema   = `{"type":"object","properties":{"id":{"type":"string"},"nickname":{"type":"string"},"character":{"type":"string"},"serverId":{"type":"string"},"level":{"type":"integer"},"vip":{"type":"integer"},"status":{"type":"string"},"balanceGold":{"type":"integer"},"lastLoginAt":{"type":"string","format":"date-time"},"createdAt":{"type":"string","format":"date-time"}}}`
	taskSchema     = `{"type":"object","properties":{"id":{"type":"string"},"type":{"type":"string"},"title":{"type":"string"},"detail":{"type":"string"},"receiverRange":{"type":"string"},"targetServerId":{"type":"string"},"targetPlayerId":{"type":"string"},"targetPlayerIds":{"type":"array","items":{"type":"string"}},"targetOrderNo":{"type":"string"},"targetCount":{"type":"integer"},"perAmount":{"type":"integer"},"totalAmount":{"type":"integer"},"currency":{"type":"string"},"status":{"type":"string"},"sentCount":{"type":"integer"},"failedCount":{"type":"integer"},"operator":{"type":"string"},"approvalId":{"type":"string"},"createdAt":{"type":"string","format":"date-time"}}}`
	banSchema      = `{"type":"object","properties":{"id":{"type":"string"},"playerId":{"type":"string"},"nickname":{"type":"string"},"serverId":{"type":"string"},"approvalId":{"type":"string"},"reason":{"type":"string"},"durationHours":{"type":"integer"},"bannedAt":{"type":"string","format":"date-time"},"expiresAt":{"type":"string","format":"date-time"},"status":{"type":"string"},"operator":{"type":"string"},"approver":{"type":"string"}}}`
	orderSchema    = `{"type":"object","properties":{"orderNo":{"type":"string"},"playerId":{"type":"string"},"nickname":{"type":"string"},"amount":{"type":"integer"},"currency":{"type":"string"},"channel":{"type":"string"},"status":{"type":"string"},"gameCredit":{"type":"string"},"reconciliation":{"type":"string"},"payTransactionId":{"type":"string"},"gameCreditRef":{"type":"string"},"paidAt":{"type":"string","format":"date-time"},"creditedAt":{"type":"string","format":"date-time"}}}`
	activitySchema = `{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"type":{"type":"string"},"timezone":{"type":"string"},"startAt":{"type":"string","format":"date-time"},"endAt":{"type":"string","format":"date-time"},"status":{"type":"string"},"currentVersion":{"type":"integer"},"versions":{"type":"array","items":{"type":"object","properties":{"version":{"type":"integer"},"note":{"type":"string"},"publishedAt":{"type":"string","format":"date-time"},"operator":{"type":"string"}}}},"versionHistory":{"type":"string"}}}`
	successSchema  = `{"type":"object","properties":{"success":{"type":"boolean"},"id":{"type":"string"},"orderNo":{"type":"string"},"version":{"type":"integer"},"amount":{"type":"integer"},"count":{"type":"integer"},"lifted":{"type":"integer"},"retried":{"type":"integer"},"extraHours":{"type":"integer"},"approvalId":{"type":"string"}}}`
	reportSchema   = `{"type":"object","properties":{"success":{"type":"boolean"},"reportId":{"type":"string"},"runSeq":{"type":"integer"},"rows":{"type":"integer"},"range":{"type":"string"}},"required":["success","reportId","runSeq"]}`
)

func collectionSchema(itemSchema string) string {
	return `{"type":"object","properties":{"items":{"type":"array","items":` + itemSchema + `},"total":{"type":"integer"},"page":{"type":"integer"},"pageSize":{"type":"integer"}},"required":["items","total","page","pageSize"]}`
}

// listInputSchema returns the caller schema for one list contract. The
// common fields cover current/pageSize and the optional filter fields sent by
// the corresponding E1 template binding (all optional → zero-object requests
// remain valid).
func listInputSchema(extra string) string {
	base := `"current":{"type":"integer","minimum":1},"pageSize":{"type":"integer","minimum":1,"maximum":100},"orderBy":{"type":"string"},"orderDirection":{"type":"string","enum":["asc","desc"]}`
	if extra != "" {
		base += "," + extra
	}
	return `{"type":"object","properties":{` + base + `}}`
}

type gameDefinition struct {
	desc    croupier.FunctionDescriptor
	handler croupier.FunctionHandler
}

// gameFunctionDefinitions returns the full contract surface. Function IDs
// exactly match the five game-ops template PageSpec bindings so a published
// demo page resolves through FunctionContract → binding execution unchanged.
//
// operationId: every write additionally accepts an optional `operationId`
// input (not declared in the schemas — they allow additionalProperties, so
// the contract surface stays identical to gameops-demo). It keys the
// exactly-once protocol in the store; see docs/design/demo-game.md.
func gameFunctionDefinitions(st *store.Store) []gameDefinition {
	defs := []gameDefinition{
		// player
		{croupier.FunctionDescriptor{ID: "player.list", Version: "1.0.0", Resource: "player", Operation: "list", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: listInputSchema(`"nickname":{"type":"string"},"status":{"type":"string"}`), OutputSchema: collectionSchema(playerSchema)}, st.PlayerList},
		{croupier.FunctionDescriptor{ID: "player.get", Version: "1.0.0", Resource: "player", Operation: "get", Capability: "item_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: playerSchema}, st.PlayerGet},
		{croupier.FunctionDescriptor{ID: "player.create", Version: "1.0.0", Resource: "player", Operation: "create", Capability: "create", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"nickname":{"type":"string"},"character":{"type":"string"},"serverId":{"type":"string"},"level":{"type":"integer"},"vip":{"type":"integer"}}}`, OutputSchema: successSchema}, st.PlayerCreate},
		{croupier.FunctionDescriptor{ID: "player.update", Version: "1.0.0", Resource: "player", Operation: "update", Capability: "update", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"nickname":{"type":"string"},"level":{"type":"integer"},"vip":{"type":"integer"}},"required":["id"]}`, OutputSchema: successSchema}, st.PlayerUpdate},
		{croupier.FunctionDescriptor{ID: "player.delete", Version: "1.0.0", Resource: "player", Operation: "delete", Capability: "delete", Execution: "sync", Risk: "danger", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: successSchema}, st.PlayerDelete},
		{croupier.FunctionDescriptor{ID: "player.ban", Version: "1.0.0", Resource: "player", Operation: "ban", Capability: "action", Execution: "sync", Risk: "high", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"reason":{"type":"string"},"duration":{"type":"integer"}},"required":["playerId"]}`, OutputSchema: successSchema}, st.PlayerBan},
		{croupier.FunctionDescriptor{ID: "player.recharge", Version: "1.0.0", Resource: "player", Operation: "recharge", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"amount":{"type":"integer"},"channel":{"type":"string"}},"required":["playerId"]}`, OutputSchema: successSchema}, st.PlayerRecharge},
		{croupier.FunctionDescriptor{ID: "player.batchrecharge", Version: "1.0.0", Resource: "player", Operation: "batch_recharge", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"playerIds":{"type":"array","items":{"type":"string"}},"amount":{"type":"integer","minimum":0}},"required":["playerIds","amount"]}`, OutputSchema: successSchema}, st.PlayerBatchRecharge},
		{croupier.FunctionDescriptor{ID: "player.mail", Version: "1.0.0", Resource: "player", Operation: "mail", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"title":{"type":"string"},"content":{"type":"string"}},"required":["playerId"]}`, OutputSchema: successSchema}, st.PlayerMail},
		{croupier.FunctionDescriptor{ID: "player.batchban", Version: "1.0.0", Resource: "player", Operation: "batch_ban", Capability: "action", Execution: "sync", Risk: "high", Enabled: true, InputSchema: `{"type":"object","properties":{"playerIds":{"type":"array","items":{"type":"string"}}}}`, OutputSchema: successSchema}, st.PlayerBatchBan},
		{croupier.FunctionDescriptor{ID: "player.batchunban", Version: "1.0.0", Resource: "player", Operation: "batch_unban", Capability: "action", Execution: "sync", Risk: "high", Enabled: true, InputSchema: `{"type":"object","properties":{"playerIds":{"type":"array","items":{"type":"string"}}}}`, OutputSchema: successSchema}, st.PlayerBatchUnban},

		// mail_task
		{croupier.FunctionDescriptor{ID: "mail_task.list", Version: "1.0.0", Resource: "mail_task", Operation: "list", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: listInputSchema(`"type":{"type":"string"},"status":{"type":"string"}`), OutputSchema: collectionSchema(taskSchema)}, st.MailTaskList},
		{croupier.FunctionDescriptor{ID: "mail_task.get", Version: "1.0.0", Resource: "mail_task", Operation: "get", Capability: "item_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: taskSchema}, st.MailTaskGet},
		{croupier.FunctionDescriptor{ID: "mail_task.create", Version: "1.0.0", Resource: "mail_task", Operation: "create", Capability: "create", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"type":{"type":"string"},"title":{"type":"string"},"detail":{"type":"string"},"receiverRange":{"type":"string"},"targetServerId":{"type":"string"},"targetPlayerId":{"type":"string"},"targetPlayerIds":{"type":"array","items":{"type":"string"}},"targetOrderNo":{"type":"string"},"perAmount":{"type":"integer"}}}`, OutputSchema: successSchema}, st.MailTaskCreate},
		{croupier.FunctionDescriptor{ID: "mail_task.update", Version: "1.0.0", Resource: "mail_task", Operation: "update", Capability: "update", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"title":{"type":"string"},"detail":{"type":"string"}},"required":["id"]}`, OutputSchema: successSchema}, st.MailTaskUpdate},
		{croupier.FunctionDescriptor{ID: "mail_task.retry_failed", Version: "1.0.0", Resource: "mail_task", Operation: "retry_failed", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: successSchema}, st.MailTaskRetryFailed},
		{croupier.FunctionDescriptor{ID: "mail_task.cancel", Version: "1.0.0", Resource: "mail_task", Operation: "cancel", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: successSchema}, st.MailTaskCancel},

		// ban
		{croupier.FunctionDescriptor{ID: "ban.list", Version: "1.0.0", Resource: "ban", Operation: "list", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: listInputSchema(`"nickname":{"type":"string"},"status":{"type":"string"},"reason":{"type":"string"}`), OutputSchema: collectionSchema(banSchema)}, st.BanList},
		{croupier.FunctionDescriptor{ID: "ban.get", Version: "1.0.0", Resource: "ban", Operation: "get", Capability: "item_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: banSchema}, st.BanGet},
		{croupier.FunctionDescriptor{ID: "ban.create", Version: "1.0.0", Resource: "ban", Operation: "create", Capability: "create", Execution: "sync", Risk: "high", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"durationHours":{"type":"integer","minimum":1},"serverId":{"type":"string"},"reason":{"type":"string"}},"required":["playerId","durationHours"]}`, OutputSchema: successSchema}, st.BanCreate},
		{croupier.FunctionDescriptor{ID: "ban.unban", Version: "1.0.0", Resource: "ban", Operation: "unban", Capability: "action", Execution: "sync", Risk: "high", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"reason":{"type":"string"}},"required":["id"]}`, OutputSchema: successSchema}, st.BanUnban},
		{croupier.FunctionDescriptor{ID: "ban.extend", Version: "1.0.0", Resource: "ban", Operation: "extend", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"extraHours":{"type":"integer","minimum":1}},"required":["id","extraHours"]}`, OutputSchema: successSchema}, st.BanExtend},
		{croupier.FunctionDescriptor{ID: "ban.batch_unban", Version: "1.0.0", Resource: "ban", Operation: "batch_unban", Capability: "action", Execution: "sync", Risk: "high", Enabled: true, InputSchema: `{"type":"object","properties":{"ids":{"type":"array","items":{"type":"string"}}}}`, OutputSchema: successSchema}, st.BanBatchUnban},

		// order
		{croupier.FunctionDescriptor{ID: "order.list", Version: "1.0.0", Resource: "order", Operation: "list", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: listInputSchema(`"nickname":{"type":"string"},"channel":{"type":"string"},"reconciliation":{"type":"string"}`), OutputSchema: collectionSchema(orderSchema)}, st.OrderList},
		{croupier.FunctionDescriptor{ID: "order.get", Version: "1.0.0", Resource: "order", Operation: "get", Capability: "item_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{"orderNo":{"type":"string"}},"required":["orderNo"]}`, OutputSchema: orderSchema}, st.OrderGet},
		{croupier.FunctionDescriptor{ID: "order.recredit", Version: "1.0.0", Resource: "order", Operation: "recredit", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"orderNo":{"type":"string"},"reason":{"type":"string"}},"required":["orderNo"]}`, OutputSchema: successSchema}, st.OrderRecredit},

		// activity
		{croupier.FunctionDescriptor{ID: "activity.list", Version: "1.0.0", Resource: "activity", Operation: "list", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: listInputSchema(`"name":{"type":"string"},"status":{"type":"string"},"type":{"type":"string"}`), OutputSchema: collectionSchema(activitySchema)}, st.ActivityList},
		{croupier.FunctionDescriptor{ID: "activity.get", Version: "1.0.0", Resource: "activity", Operation: "get", Capability: "item_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: activitySchema}, st.ActivityGet},
		{croupier.FunctionDescriptor{ID: "activity.create", Version: "1.0.0", Resource: "activity", Operation: "create", Capability: "create", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"name":{"type":"string"},"type":{"type":"string"},"timezone":{"type":"string"}},"required":["name"]}`, OutputSchema: successSchema}, st.ActivityCreate},
		{croupier.FunctionDescriptor{ID: "activity.publish_version", Version: "1.0.0", Resource: "activity", Operation: "publish_version", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"note":{"type":"string"}},"required":["id"]}`, OutputSchema: successSchema}, st.ActivityPublish},
		{croupier.FunctionDescriptor{ID: "activity.activate", Version: "1.0.0", Resource: "activity", Operation: "activate", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: successSchema}, st.ActivityActivate},
		{croupier.FunctionDescriptor{ID: "activity.end", Version: "1.0.0", Resource: "activity", Operation: "end", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: successSchema}, st.ActivityEnd},
		{croupier.FunctionDescriptor{ID: "activity.rollback", Version: "1.0.0", Resource: "activity", Operation: "rollback", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"targetVersion":{"type":"integer","minimum":1}},"required":["id","targetVersion"]}`, OutputSchema: successSchema}, st.ActivityRollback},

		// report.generate is the single task-mode (Execution: "task") contract.
		// It reproduces the E2 async-task, delay/result-unknown and failure
		// scenarios: invoked with mode=task the server returns a task id
		// immediately (result unknown until completion), the handler sleeps
		// delayMs (default ~2s) then succeeds or fails on the `fail` flag.
		// runSeq counts actual executions and is persisted through the store's
		// meta row, so a re-delivered task after a restart does not re-hand
		// out a reportId (UI-002 restart evidence, now durable by default).
		{croupier.FunctionDescriptor{ID: "report.generate", Version: "1.0.0", Resource: "report", Operation: "generate", Capability: "task", Execution: "task", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"range":{"type":"string"},"fail":{"type":"boolean"},"delayMs":{"type":"integer","minimum":0}},"required":["range"]}`, OutputSchema: reportSchema}, st.ReportGenerate},
	}
	return defs
}

// registerGameFunctions registers every definition via the SDK client,
// enriching descriptors the same way the game demo SDK does.
func registerGameFunctions(client croupier.Client, st *store.Store) error {
	for _, def := range gameFunctionDefinitions(st) {
		desc := enrichDescriptor(def.desc)
		if err := validateDescriptor(desc); err != nil {
			return err
		}
		if err := client.RegisterFunction(desc, def.handler); err != nil {
			return fmt.Errorf("register %s failed: %w", desc.ID, err)
		}
	}
	return nil
}

func enrichDescriptor(desc croupier.FunctionDescriptor) croupier.FunctionDescriptor {
	if desc.Tags == nil {
		desc.Tags = []string{desc.Resource, desc.Operation}
	}
	if desc.Summary == "" {
		desc.Summary = fmt.Sprintf("%s %s", desc.Resource, desc.Operation)
	}
	if desc.Description == "" {
		desc.Description = fmt.Sprintf("Demo game function %s for %s %s operations.", desc.ID, desc.Resource, desc.Operation)
	}
	return desc
}

func validateDescriptor(desc croupier.FunctionDescriptor) error {
	if strings.TrimSpace(desc.Capability) == "" {
		return fmt.Errorf("demo-game descriptor %s is missing capability", desc.ID)
	}
	if strings.TrimSpace(desc.InputSchema) == "" || !json.Valid([]byte(desc.InputSchema)) {
		return fmt.Errorf("demo-game descriptor %s has invalid input schema", desc.ID)
	}
	if strings.TrimSpace(desc.OutputSchema) == "" || !json.Valid([]byte(desc.OutputSchema)) {
		return fmt.Errorf("demo-game descriptor %s has invalid output schema", desc.ID)
	}
	return nil
}
