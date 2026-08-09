package registry

import (
	"strings"
)

// operationCapabilityMap maps common operation name suffixes to capability kinds.
var operationCapabilityMap = map[string]string{
	// collection_query
	"list":   "collection_query",
	"query":  "collection_query",
	"search": "collection_query",
	"find":   "collection_query",
	"getall": "collection_query",
	"fetch":  "collection_query",
	// item_query
	"get":      "item_query",
	"detail":   "item_query",
	"findbyid": "item_query",
	"getbyid":  "item_query",
	// create
	"create": "create",
	"add":    "create",
	"insert": "create",
	"new":    "create",
	// update
	"update": "update",
	"modify": "update",
	"edit":   "update",
	"patch":  "update",
	// delete
	"delete":  "delete",
	"remove":  "delete",
	"destroy": "delete",
}

// InferResourceAndCapability infers Resource, Operation, and Capability from
// a function ID when they are not explicitly provided.
//
// Function ID format: "resource.operation" (e.g., "player.list", "order.create").
// If the ID does not contain a dot, no inference is performed.
// Only empty fields are filled; explicit values are never overwritten.
func InferResourceAndCapability(functionID string, meta *FunctionMeta) {
	if meta == nil {
		return
	}
	// Only infer when capability is empty — explicit values take precedence.
	if strings.TrimSpace(meta.Capability) != "" {
		return
	}

	functionID = strings.TrimSpace(functionID)
	dotIdx := strings.LastIndex(functionID, ".")
	if dotIdx <= 0 || dotIdx >= len(functionID)-1 {
		return
	}

	resource := functionID[:dotIdx]
	operation := functionID[dotIdx+1:]

	if strings.TrimSpace(meta.Resource) == "" {
		meta.Resource = resource
	}
	if strings.TrimSpace(meta.Operation) == "" {
		meta.Operation = operation
	}

	capability, ok := operationCapabilityMap[strings.ToLower(operation)]
	if !ok {
		capability = "action"
	}
	meta.Capability = capability
}
