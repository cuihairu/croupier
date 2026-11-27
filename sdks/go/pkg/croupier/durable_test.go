package croupier

// GAME-010 slice 5：SDK 信封便捷构造（GAME-009 D3）单测。身份锚点语义：
// 无 operationId 的调用拒绝出信封（非 durable 调用不该使用信封协议）；
// operationId 必须原样回传（平台侧信封归属校验的唯一依据）。

import (
	"context"
	"encoding/json"
	"testing"
)

const testOpID = "demo:prod:player:018f3b27-5c2e-7b10-8000-000000000009"

func durableCtx() context.Context {
	return WithOperationID(context.Background(), testOpID)
}

func TestOperationIDFromContext(t *testing.T) {
	if got := OperationIDFromContext(durableCtx()); got != testOpID {
		t.Fatalf("want %s, got %q", testOpID, got)
	}
	if got := OperationIDFromContext(context.Background()); got != "" {
		t.Fatalf("plain ctx should have no operationId, got %q", got)
	}
	if got := OperationIDFromContext(nil); got != "" {
		t.Fatalf("nil ctx should have no operationId, got %q", got)
	}
	// 空 operationId 不注入（保持原 ctx）。
	if got := OperationIDFromContext(WithOperationID(context.Background(), "  ")); got != "" {
		t.Fatalf("blank operationId should not be stored, got %q", got)
	}
}

func TestNewOperationEnvelopeRequiresOperationID(t *testing.T) {
	if _, err := NewOperationEnvelope(context.Background(), EnvelopeOutcomeSucceeded); err != ErrOperationIDMissing {
		t.Fatalf("plain ctx should reject envelope, got %v", err)
	}
	envelope, err := NewOperationEnvelope(durableCtx(), EnvelopeOutcomeSucceeded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if envelope.OperationID != testOpID || envelope.Outcome != EnvelopeOutcomeSucceeded {
		t.Fatalf("envelope mismatch: %+v", envelope)
	}
}

func TestEnvelopeMarshalWireShape(t *testing.T) {
	raw, err := SucceededWith(durableCtx(), "receipt-1", json.RawMessage(`{"ok":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("marshal invalid: %v", err)
	}
	if string(decoded["operationId"]) != `"`+testOpID+`"` {
		t.Fatalf("operationId not round-tripped: %s", raw)
	}
	if string(decoded["outcome"]) != `"succeeded"` {
		t.Fatalf("outcome wrong: %s", raw)
	}
	if _, ok := decoded["result"]; !ok {
		t.Fatalf("result missing: %s", raw)
	}
	// 可省略字段不得出现在线上（omitempty）。
	if _, ok := decoded["requestFingerprint"]; ok {
		t.Fatalf("requestFingerprint should be omitted: %s", raw)
	}
}

func TestEnvelopeHelpers(t *testing.T) {
	raw, err := FailedWith(durableCtx(), "quota exhausted")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var envelope GameOperationResult
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("invalid envelope: %v", err)
	}
	if envelope.Outcome != EnvelopeOutcomeFailed || envelope.Result == nil {
		t.Fatalf("failed envelope mismatch: %+v", envelope)
	}

	raw, err = RetryableFailedWith(durableCtx(), "dependency down", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("invalid envelope: %v", err)
	}
	if envelope.Outcome != EnvelopeOutcomeRetryableFailed || envelope.RetryableAfter != 30 {
		t.Fatalf("retryable envelope mismatch: %+v", envelope)
	}

	raw, err = CanceledWith(durableCtx(), "cancel-ok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("invalid envelope: %v", err)
	}
	if envelope.Outcome != EnvelopeOutcomeCanceled || envelope.GameReceipt != "cancel-ok" {
		t.Fatalf("canceled envelope mismatch: %+v", envelope)
	}

	raw, err = CancelFailedWith(durableCtx(), "side effects already applied")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("invalid envelope: %v", err)
	}
	if envelope.Outcome != EnvelopeOutcomeCancelFailed {
		t.Fatalf("cancel_failed envelope mismatch: %+v", envelope)
	}

	if _, err := SucceededWith(context.Background(), "r", nil); err != ErrOperationIDMissing {
		t.Fatalf("plain ctx should reject helpers, got %v", err)
	}
}

func TestEnvelopeMarshalNil(t *testing.T) {
	var envelope *GameOperationResult
	if _, err := envelope.Marshal(); err == nil {
		t.Fatal("nil envelope should not marshal")
	}
}
