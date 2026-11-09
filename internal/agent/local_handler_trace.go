package agent

import (
	"encoding/json"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

func agentTracer() trace.Tracer {
	return otel.Tracer("croupier.agent")
}

func recordSpanResult(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}
	span.SetStatus(codes.Ok, "")
}

func mustMarshal(msg proto.Message) []byte {
	if msg == nil {
		return nil
	}
	data, err := proto.Marshal(msg)
	if err == nil {
		return data
	}
	fallback, _ := json.Marshal(map[string]string{"error": err.Error()})
	return fallback
}
