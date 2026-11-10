package main

import (
	"encoding/json"
	"testing"
)

func TestDescriptorsValid(t *testing.T) {
	defs := rpgFunctionDefinitions(testStore(t))
	if len(defs) != 17 {
		t.Fatalf("defs=%d want 17", len(defs))
	}
	seen := map[string]bool{}
	for _, def := range defs {
		desc := enrichRPGDescriptor(def.desc)
		id := desc.ID
		if id == "" {
			t.Fatal("empty function id")
		}
		if seen[id] {
			t.Fatalf("duplicate id %s", id)
		}
		seen[id] = true
		if err := validateRPGDescriptor(desc); err != nil {
			t.Fatalf("descriptor %s: %v", id, err)
		}
		if desc.Capability == "" || desc.Risk == "" || desc.Execution == "" {
			t.Fatalf("descriptor %s incomplete: %s/%s/%s", id, desc.Capability, desc.Risk, desc.Execution)
		}
		for _, domain := range []string{"player", "mail", "task", "order", "economy", "report"} {
			if id == domain+".list" || id == domain+".generate" || id == domain+".send" {
				if desc.Resource != domain {
					t.Fatalf("%s resource=%s want %s", id, desc.Resource, domain)
				}
			}
		}
	}
}

func TestReportGenerateIsApprovalGatedTask(t *testing.T) {
	for _, def := range rpgFunctionDefinitions(testStore(t)) {
		if def.desc.ID != "report.generate" {
			continue
		}
		if def.desc.Execution != "task" || def.desc.Capability != "task" {
			t.Fatalf("report.generate execution=%s capability=%s", def.desc.Execution, def.desc.Capability)
		}
		if def.desc.Risk != "high" {
			t.Fatalf("report.generate risk=%s want high (approval gate)", def.desc.Risk)
		}
		return
	}
	t.Fatal("report.generate missing")
}

func TestShipAndBanAreHighRisk(t *testing.T) {
	high := map[string]bool{"order.ship": false, "order.refund": false, "player.ban": false, "player.unban": false}
	for _, def := range rpgFunctionDefinitions(testStore(t)) {
		if _, ok := high[def.desc.ID]; ok {
			if def.desc.Risk != "high" {
				t.Fatalf("%s risk=%s want high", def.desc.ID, def.desc.Risk)
			}
			high[def.desc.ID] = true
		}
	}
	for id, seen := range high {
		if !seen {
			t.Fatalf("%s missing", id)
		}
	}
}

func TestListSchemasKeepGameopsShape(t *testing.T) {
	for _, def := range rpgFunctionDefinitions(testStore(t)) {
		if def.desc.Capability != "collection_query" {
			continue
		}
		var out struct {
			Properties map[string]json.RawMessage `json:"properties"`
			Required   []string                   `json:"required"`
		}
		if err := json.Unmarshal([]byte(def.desc.OutputSchema), &out); err != nil {
			t.Fatalf("%s output schema: %v", def.desc.ID, err)
		}
		for _, key := range []string{"items", "total", "page", "pageSize"} {
			if _, ok := out.Properties[key]; !ok {
				t.Fatalf("%s output missing %s", def.desc.ID, key)
			}
		}
	}
}
