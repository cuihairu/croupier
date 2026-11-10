package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

// Schema constants keep the list output in the gameops shape
// ({items,total,page,pageSize}) so the dashboard renderer selectors work.
const (
	playerSchema    = `{"type":"object","properties":{"id":{"type":"string"},"nickname":{"type":"string"},"level":{"type":"integer"},"xp":{"type":"integer"},"gold":{"type":"integer"},"diamonds":{"type":"integer"},"inventory":{"type":"object"},"status":{"type":"string"},"vip":{"type":"integer"},"lastSeenAt":{"type":"string","format":"date-time"},"createdAt":{"type":"string","format":"date-time"}}}`
	itemSchema      = `{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"type":{"type":"string"},"tradable":{"type":"boolean"}}}`
	inventorySchema = `{"type":"object","properties":{"playerId":{"type":"string"},"nickname":{"type":"string"},"items":{"type":"array","items":{"type":"object","properties":{"itemId":{"type":"string"},"name":{"type":"string"},"type":{"type":"string"},"count":{"type":"integer"}}}},"totalItems":{"type":"integer"}}}`
	mailSchema      = `{"type":"object","properties":{"id":{"type":"string"},"playerId":{"type":"string"},"title":{"type":"string"},"content":{"type":"string"},"attachments":{"type":"array","items":{"type":"object","properties":{"itemId":{"type":"string"},"count":{"type":"integer"}}}},"expireAt":{"type":"string","format":"date-time"},"expired":{"type":"boolean"},"read":{"type":"boolean"},"claimed":{"type":"boolean"},"sender":{"type":"string"},"createdAt":{"type":"string","format":"date-time"}}}`
	questSchema     = `{"type":"object","properties":{"id":{"type":"string"},"title":{"type":"string"},"goalType":{"type":"string"},"goalCount":{"type":"integer"},"rewardGold":{"type":"integer"},"rewardXp":{"type":"integer"},"rewardItemId":{"type":"string"},"rewardItemCount":{"type":"integer"},"daily":{"type":"boolean"},"progress":{"type":"object","properties":{"playerId":{"type":"string"},"questId":{"type":"string"},"progress":{"type":"integer"},"status":{"type":"string"},"day":{"type":"string"},"completedAt":{"type":"string","format":"date-time"}}}}}`
	orderSchema     = `{"type":"object","properties":{"orderNo":{"type":"string"},"playerId":{"type":"string"},"productId":{"type":"string"},"productName":{"type":"string"},"amount":{"type":"integer"},"currency":{"type":"string"},"diamonds":{"type":"integer"},"status":{"type":"string"},"shipKey":{"type":"string"},"shippedAt":{"type":"string","format":"date-time"},"createdAt":{"type":"string","format":"date-time"}}}`
	banSchema       = `{"type":"object","properties":{"id":{"type":"string"},"playerId":{"type":"string"},"reason":{"type":"string"},"startAt":{"type":"string","format":"date-time"},"endAt":{"type":"string","format":"date-time"},"operator":{"type":"string"},"active":{"type":"boolean"}}}`
	auditSchema     = `{"type":"object","properties":{"id":{"type":"string"},"at":{"type":"string","format":"date-time"},"actor":{"type":"string"},"action":{"type":"string"},"target":{"type":"string"},"result":{"type":"string"},"detail":{"type":"string"}}}`
	snapshotSchema  = `{"type":"object","properties":{"players":{"type":"integer"},"online":{"type":"integer"},"banned":{"type":"integer"},"gold":{"type":"integer"},"diamonds":{"type":"integer"},"orders":{"type":"object"},"mailsUnclaimed":{"type":"integer"},"items":{"type":"object"},"questsClaimable":{"type":"integer"}}}`
	successSchema   = `{"type":"object","properties":{"success":{"type":"boolean"},"id":{"type":"string"},"mailId":{"type":"string"},"orderNo":{"type":"string"},"playerId":{"type":"string"},"questId":{"type":"string"},"items":{"type":"array"},"gold":{"type":"integer"},"xp":{"type":"integer"},"diamonds":{"type":"integer"},"diamondsDeducted":{"type":"integer"},"level":{"type":"integer"},"rewardItem":{"type":"string"},"rewardCount":{"type":"integer"},"refreshed":{"type":"integer"}}}`
	reportSchema    = `{"type":"object","properties":{"success":{"type":"boolean"},"reportId":{"type":"string"},"runSeq":{"type":"integer"},"players":{"type":"integer"},"gold":{"type":"integer"},"diamonds":{"type":"integer"},"range":{"type":"string"}},"required":["success","reportId","runSeq"]}`
)

func collectionSchema(itemSchema string) string {
	return `{"type":"object","properties":{"items":{"type":"array","items":` + itemSchema + `},"total":{"type":"integer"},"page":{"type":"integer"},"pageSize":{"type":"integer"}},"required":["items","total","page","pageSize"]}`
}

// listInputSchema is the common list input (pagination plus optional filter
// fields; all optional so zero-object requests stay valid).
func listInputSchema(extra string) string {
	base := `"current":{"type":"integer","minimum":1},"pageSize":{"type":"integer","minimum":1,"maximum":100},"orderBy":{"type":"string"},"orderDirection":{"type":"string","enum":["asc","desc"]}`
	if extra != "" {
		base += "," + extra
	}
	return `{"type":"object","properties":{` + base + `}}`
}

type rpgDefinition struct {
	desc    croupier.FunctionDescriptor
	handler croupier.FunctionHandler
}

// rpgFunctionDefinitions returns the full contract surface grouped by domain:
// player.* (workbench), mail.* (compensation flow), task.* (quests),
// order.* (recharge), economy.* (observability) and report.generate (the
// single task-mode contract; risk high so the platform runs it through the
// approval chain).
func rpgFunctionDefinitions(store *rpgStore) []rpgDefinition {
	defs := []rpgDefinition{
		// player.*
		{croupier.FunctionDescriptor{ID: "player.list", Version: "1.0.0", Resource: "player", Operation: "list", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: listInputSchema(`"nickname":{"type":"string"},"status":{"type":"string"}`), OutputSchema: collectionSchema(playerSchema)}, store.playerList},
		{croupier.FunctionDescriptor{ID: "player.get", Version: "1.0.0", Resource: "player", Operation: "get", Capability: "item_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: playerSchema}, store.playerGet},
		{croupier.FunctionDescriptor{ID: "player.inventory", Version: "1.0.0", Resource: "player", Operation: "inventory", Capability: "item_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`, OutputSchema: inventorySchema}, store.playerInventory},
		{croupier.FunctionDescriptor{ID: "player.ban", Version: "1.0.0", Resource: "player", Operation: "ban", Capability: "action", Execution: "sync", Risk: "high", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"reason":{"type":"string"},"durationHours":{"type":"integer","minimum":1}},"required":["playerId"]}`, OutputSchema: successSchema}, store.playerBan},
		{croupier.FunctionDescriptor{ID: "player.unban", Version: "1.0.0", Resource: "player", Operation: "unban", Capability: "action", Execution: "sync", Risk: "high", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`, OutputSchema: successSchema}, store.playerUnban},

		// mail.* (attachment claim is the compensation flow core)
		{croupier.FunctionDescriptor{ID: "mail.send", Version: "1.0.0", Resource: "mail", Operation: "send", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"title":{"type":"string"},"content":{"type":"string"},"attachments":{"type":"array","items":{"type":"object","properties":{"itemId":{"type":"string"},"count":{"type":"integer"}}}},"expireHours":{"type":"integer","minimum":1}},"required":["playerId","title"]}`, OutputSchema: successSchema}, store.mailSend},
		{croupier.FunctionDescriptor{ID: "mail.list", Version: "1.0.0", Resource: "mail", Operation: "list", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: listInputSchema(`"playerId":{"type":"string"}`), OutputSchema: collectionSchema(mailSchema)}, store.mailList},
		{croupier.FunctionDescriptor{ID: "mail.claim", Version: "1.0.0", Resource: "mail", Operation: "claim", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"mailId":{"type":"string"}},"required":["mailId"]}`, OutputSchema: successSchema}, store.mailClaim},

		// task.* (quest system)
		{croupier.FunctionDescriptor{ID: "task.list", Version: "1.0.0", Resource: "task", Operation: "list", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: listInputSchema(`"playerId":{"type":"string"},"daily":{"type":"boolean"}`), OutputSchema: collectionSchema(questSchema)}, store.taskList},
		{croupier.FunctionDescriptor{ID: "task.claim", Version: "1.0.0", Resource: "task", Operation: "claim", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"questId":{"type":"string"}},"required":["playerId","questId"]}`, OutputSchema: successSchema}, store.taskClaim},
		{croupier.FunctionDescriptor{ID: "task.refresh", Version: "1.0.0", Resource: "task", Operation: "refresh", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{}}`, OutputSchema: successSchema}, store.taskRefresh},

		// order.*
		{croupier.FunctionDescriptor{ID: "order.list", Version: "1.0.0", Resource: "order", Operation: "list", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: listInputSchema(`"playerId":{"type":"string"},"status":{"type":"string"}`), OutputSchema: collectionSchema(orderSchema)}, store.orderList},
		{croupier.FunctionDescriptor{ID: "order.ship", Version: "1.0.0", Resource: "order", Operation: "ship", Capability: "action", Execution: "sync", Risk: "high", Enabled: true, InputSchema: `{"type":"object","properties":{"orderNo":{"type":"string"}},"required":["orderNo"]}`, OutputSchema: successSchema}, store.orderShip},
		{croupier.FunctionDescriptor{ID: "order.refund", Version: "1.0.0", Resource: "order", Operation: "refund", Capability: "action", Execution: "sync", Risk: "high", Enabled: true, InputSchema: `{"type":"object","properties":{"orderNo":{"type":"string"}},"required":["orderNo"]}`, OutputSchema: successSchema}, store.orderRefund},

		// economy.*
		{croupier.FunctionDescriptor{ID: "economy.snapshot", Version: "1.0.0", Resource: "economy", Operation: "snapshot", Capability: "item_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{}}`, OutputSchema: snapshotSchema}, store.economySnapshot},
		{croupier.FunctionDescriptor{ID: "economy.audit", Version: "1.0.0", Resource: "economy", Operation: "audit", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: listInputSchema(`"action":{"type":"string"},"actor":{"type":"string"}`), OutputSchema: collectionSchema(auditSchema)}, store.economyAudit},

		// report.generate: the single task-mode (Execution: "task") contract,
		// risk high so the platform requires approval before execution. The
		// handler sleeps delayMs (default ~2s), honours cancellation, and
		// counts actual runs (runSeq persists in the world snapshot, so a
		// platform retry cannot re-hand out a reportId across restarts).
		{croupier.FunctionDescriptor{ID: "report.generate", Version: "1.0.0", Resource: "report", Operation: "generate", Capability: "task", Execution: "task", Risk: "high", Enabled: true, InputSchema: `{"type":"object","properties":{"range":{"type":"string"},"delayMs":{"type":"integer","minimum":0}},"required":["range"]}`, OutputSchema: reportSchema}, store.reportGenerate},
	}
	return defs
}

// registerRPGFunctions registers every definition via the SDK client,
// enriching descriptors the same way the gameops demos do.
func registerRPGFunctions(client croupier.Client, store *rpgStore) error {
	for _, def := range rpgFunctionDefinitions(store) {
		desc := enrichRPGDescriptor(def.desc)
		if err := validateRPGDescriptor(desc); err != nil {
			return err
		}
		if err := client.RegisterFunction(desc, def.handler); err != nil {
			return fmt.Errorf("register %s failed: %w", desc.ID, err)
		}
	}
	return nil
}

func enrichRPGDescriptor(desc croupier.FunctionDescriptor) croupier.FunctionDescriptor {
	if desc.Tags == nil {
		desc.Tags = []string{desc.Resource, desc.Operation}
	}
	if desc.Summary == "" {
		desc.Summary = fmt.Sprintf("%s %s", desc.Resource, desc.Operation)
	}
	if desc.Description == "" {
		desc.Description = fmt.Sprintf("GameOps RPG demo function %s for %s %s operations.", desc.ID, desc.Resource, desc.Operation)
	}
	return desc
}

func validateRPGDescriptor(desc croupier.FunctionDescriptor) error {
	if strings.TrimSpace(desc.Capability) == "" {
		return fmt.Errorf("gameops-rpg descriptor %s is missing capability", desc.ID)
	}
	if strings.TrimSpace(desc.InputSchema) == "" || !json.Valid([]byte(desc.InputSchema)) {
		return fmt.Errorf("gameops-rpg descriptor %s has invalid input schema", desc.ID)
	}
	if strings.TrimSpace(desc.OutputSchema) == "" || !json.Valid([]byte(desc.OutputSchema)) {
		return fmt.Errorf("gameops-rpg descriptor %s has invalid output schema", desc.ID)
	}
	return nil
}
