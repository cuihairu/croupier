package openapi

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

const (
	FlatInputCollision  = "openapi_flat_input_collision"
	FlatInputConstraint = "openapi_flat_input_constraint"
)

// InputProjection is the caller-facing view of a request contract.
type InputProjection struct {
	Schema      string
	Grouped     bool
	Diagnostics []spec.Diagnostic
}

// ProjectInput keeps compatible object fields flat and groups fields when
// flattening would conflate inputs or drop constraints. Schema references
// that cannot be inlined into the standalone projection are recorded as
// diagnostics (SUI-008).
func ProjectInput(operationID string, request *RequestContract, root map[string]any) InputProjection {
	var projection InputProjection
	var issues []refIssue
	diagnose := func(code, field, message string) {
		projection.Diagnostics = append(projection.Diagnostics, spec.Diagnostic{
			Code: code, Severity: spec.SeverityWarning, FunctionID: operationID, Field: field, Message: message,
		})
	}
	fields := make(map[string]string)
	for _, p := range request.Parameters {
		if previous, ok := fields[p.Name]; ok {
			diagnose(FlatInputCollision, p.Origin.Pointer+"/name",
				fmt.Sprintf("Input %q also appears at %s", p.Name, previous))
		}
		fields[p.Name] = p.Origin.Pointer + "/name"
	}

	var body map[string]any
	if request.Body != nil {
		media, hasJSON := request.Body.Content["application/json"]
		body, _ = resolvedRequestSchema(media.Schema, root, &issues)
		bodyPath := request.Body.Origin.Pointer
		schemaPath := bodyPath + "/content/application~1json/schema"
		if !hasJSON {
			diagnose(FlatInputConstraint, bodyPath+"/content", "No application/json body is declared")
		} else if body == nil || body["type"] != "object" {
			diagnose(FlatInputConstraint, schemaPath, "Request body requires its own JSON value")
		} else if len(fields) > 0 {
			if !request.Body.Required {
				diagnose(FlatInputConstraint, bodyPath+"/required", "Flat input cannot identify whether the optional body was supplied")
			}
			for _, name := range slices.Sorted(maps.Keys(body)) {
				switch name {
				case "type", "properties", "required", "title", "description", "example", "examples", "externalDocs":
					continue
				}
				diagnose(FlatInputConstraint, schemaPath+"/"+pointerToken(name),
					fmt.Sprintf("Flat input cannot preserve the body's %q keyword", name))
			}
			properties, _ := body["properties"].(map[string]any)
			for _, name := range slices.Sorted(maps.Keys(properties)) {
				if previous, ok := fields[name]; ok {
					diagnose(FlatInputCollision, schemaPath+"/properties/"+pointerToken(name),
						fmt.Sprintf("Body property %q also appears at %s", name, previous))
				}
			}
			required, _ := body["required"].([]any)
			for _, field := range required {
				name, _ := field.(string)
				if previous, ok := fields[name]; ok {
					if _, declared := properties[name]; !declared {
						diagnose(FlatInputCollision, schemaPath+"/required",
							fmt.Sprintf("Required body property %q also appears at %s", name, previous))
					}
				}
			}
		}
	}

	switch {
	case len(projection.Diagnostics) > 0:
		projection.Grouped = true
		projection.Schema = groupedInputSchema(request, root, body, &issues)
	case len(fields) == 0 && body != nil:
		raw, _ := json.Marshal(body)
		projection.Schema = string(raw)
	default:
		projection.Schema = legacyInputSchema(request, root, body, &issues)
	}
	// Unresolved references (SUI-008) precede the projection diagnostics so a
	// stable import order keeps the stored diagnostic list deterministic.
	projection.Diagnostics = append(refIssuesToDiagnostics(issues), projection.Diagnostics...)
	for i := range projection.Diagnostics {
		projection.Diagnostics[i].FunctionID = operationID
	}
	return projection
}

func groupedInputSchema(request *RequestContract, root map[string]any, body map[string]any, issues *[]refIssue) string {
	parameters := make(map[string]map[string]any)
	requiredParams := make(map[string][]string)
	for _, p := range request.Parameters {
		if parameters[p.In] == nil {
			parameters[p.In] = make(map[string]any)
		}
		parameters[p.In][p.Name] = parameterInputSchema(p, root, issues)
		if p.Required {
			requiredParams[p.In] = append(requiredParams[p.In], p.Name)
		}
	}
	fields := make(map[string]any)
	var required []string
	for _, in := range []string{"path", "query", "header", "cookie"} {
		if properties, ok := parameters[in]; ok {
			fields[in] = groupedInputObject(properties, requiredParams[in])
			if len(requiredParams[in]) > 0 {
				required = append(required, in)
			}
		}
	}
	if request.Body != nil {
		if body == nil {
			body = map[string]any{}
		}
		fields["body"] = body
		if request.Body.Required {
			required = append(required, "body")
		}
	}
	raw, _ := json.Marshal(groupedInputObject(fields, required))
	return string(raw)
}

func groupedInputObject(properties map[string]any, required []string) map[string]any {
	object := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		object["required"] = required
	}
	return object
}
