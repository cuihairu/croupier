package approval

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
)

type approvalSummary struct {
	ID              string `json:"id"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	Actor           string `json:"actor"`
	FunctionID      string `json:"function_id"`
	GameID          string `json:"game_id"`
	Env             string `json:"env"`
	State           string `json:"state"`
	Mode            string `json:"mode"`
	Route           string `json:"route,omitempty"`
	IdempotencyKey  string `json:"idempotency_key,omitempty"`
	TargetServiceID string `json:"target_service_id,omitempty"`
	HashKey         string `json:"hash_key,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

type approvalDetail struct {
	approvalSummary
	Payload        map[string]interface{} `json:"payload,omitempty"`
	PayloadPreview string                 `json:"payload_preview,omitempty"`
}

func buildApprovalSummary(a *approvals.Approval) approvalSummary {
	if a == nil {
		return approvalSummary{}
	}
	return approvalSummary{
		ID:              a.ID,
		CreatedAt:       utils.FormatTimestamp(a.CreatedAt),
		UpdatedAt:       utils.FormatTimestamp(a.UpdatedAt),
		Actor:           a.Actor,
		FunctionID:      a.FunctionID,
		GameID:          a.GameID,
		Env:             a.Env,
		State:           strings.ToLower(strings.TrimSpace(a.State)),
		Mode:            defaultString(a.Mode, "invoke"),
		Route:           a.Route,
		IdempotencyKey:  a.IdempotencyKey,
		TargetServiceID: a.TargetServiceID,
		HashKey:         a.HashKey,
		Reason:          a.Reason,
	}
}

func buildApprovalDetail(a *approvals.Approval) approvalDetail {
	summary := buildApprovalSummary(a)
	payload, preview := decodeApprovalPayload(a)
	return approvalDetail{
		approvalSummary: summary,
		Payload:         payload,
		PayloadPreview:  preview,
	}
}

func decodeApprovalPayload(a *approvals.Approval) (map[string]interface{}, string) {
	if a == nil || len(a.Payload) == 0 {
		return nil, ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(a.Payload, &payload); err != nil {
		return nil, string(a.Payload)
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, a.Payload, "", "  "); err != nil {
		return payload, string(a.Payload)
	}
	return payload, buf.String()
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
